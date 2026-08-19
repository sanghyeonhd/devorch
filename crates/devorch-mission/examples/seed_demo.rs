//! Seed a demo database so the desktop UI can be looked at with real data.
//!
//! This runs a genuine mission — real worktrees, real git, real verification —
//! against a seeded repository, using scripted agents instead of the vendor
//! CLIs. Everything the UI then displays was produced by the same code path a
//! live mission uses; only the agents are substituted.
//!
//! ```bash
//! cargo run -p devorch-mission --example seed_demo -- /tmp/devorch-demo
//! DEVORCH_HOME=/tmp/devorch-demo cargo run -p devorch-ui-bin
//! ```

use std::path::{Path, PathBuf};

use devorch_config::{Config, PathsConfig, TaskRisk};
use devorch_mission::{FakeBehavior, FakeExecutor, MissionRequest, MissionRunner};
use devorch_process::{run, ProcessSpec};
use devorch_protocol::AgentKind;
use devorch_store::Store;
use devorch_verify::Check;

const CORRECT: &str = "def add(a, b):\n    return a + b\n";
const DOCUMENTED: &str = "def add(a, b):\n    \"\"\"Return the sum.\"\"\"\n    return a + b\n";
const WRONG: &str = "def add(a, b):\n    return a * b\n";

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let home: PathBuf = std::env::args()
        .nth(1)
        .unwrap_or_else(|| "/tmp/devorch-demo".to_string())
        .into();

    if home.exists() {
        std::fs::remove_dir_all(&home)?;
    }
    let repo = home.join("demo-repo");
    std::fs::create_dir_all(&repo)?;
    seed_repo(&repo).await?;

    let config = Config {
        paths: PathsConfig {
            home: Some(home.clone()),
        },
        ..Default::default()
    };
    let store = Store::open(config.database_path()?)?;

    // The four-candidate scenario from the development simulation: two correct,
    // one wrong, one correct but with an unrequested extra file.
    let executor = FakeExecutor::new()
        .with(
            AgentKind::Codex,
            FakeBehavior::write("calculator.py", CORRECT),
        )
        .with(
            AgentKind::Claude,
            FakeBehavior::write("calculator.py", DOCUMENTED),
        )
        .with(AgentKind::Grok, FakeBehavior::write("calculator.py", WRONG))
        .with(
            AgentKind::Gemini,
            FakeBehavior::Edit(vec![
                ("calculator.py".into(), CORRECT.into()),
                ("NOTES.md".into(), "an unnecessary file\n".into()),
            ]),
        );

    let runner = MissionRunner::with_executor(&store, config, Box::new(executor));
    let request = MissionRequest {
        checks: vec![Check::required(
            "tests",
            "python3",
            &["-m", "unittest", "-q"],
        )],
        ..MissionRequest::new(
            "The add() function in calculator.py subtracts instead of adding. Fix it.",
            &repo,
        )
        .risk(TaskRisk::ExplicitCompareAll)
        .agents(AgentKind::ALL.to_vec())
    };

    match runner.run(request, |_| {}).await {
        Ok(outcome) => {
            println!("mission {}", outcome.record.id);
            println!("winner:  {:?}", outcome.winner());
            for candidate in &outcome.record.candidates {
                println!(
                    "  {:<8} {:<8} {} files, {} churn{}",
                    candidate.agent.as_str(),
                    if candidate.passed {
                        "passed"
                    } else {
                        "rejected"
                    },
                    candidate.touched_paths,
                    candidate.churn,
                    candidate
                        .rejection
                        .as_ref()
                        .map(|r| format!(" — {r}"))
                        .unwrap_or_default()
                );
            }
        }
        Err(e) => println!("mission failed: {e}"),
    }

    println!("\nrun the UI against it:");
    println!(
        "  DEVORCH_HOME={} cargo run -p devorch-ui-bin",
        home.display()
    );
    Ok(())
}

/// Create the seeded repository: `add` subtracts, and one test says so.
async fn seed_repo(dir: &Path) -> Result<(), Box<dyn std::error::Error>> {
    for args in [
        vec!["init", "-b", "main"],
        vec!["config", "user.email", "demo@devorch.local"],
        vec!["config", "user.name", "Devorch Demo"],
    ] {
        let spec = ProcessSpec::new("git").args(args).cwd(dir);
        let out = run(&spec).await?;
        if !out.success() {
            return Err(format!("git setup failed: {}", out.stderr).into());
        }
    }

    std::fs::write(dir.join(".gitignore"), "__pycache__/\n*.pyc\n")?;
    std::fs::write(
        dir.join("calculator.py"),
        "def add(a, b):\n    return a - b\n",
    )?;
    std::fs::write(
        dir.join("test_calculator.py"),
        "import unittest\n\
         from calculator import add\n\
         \n\
         class T(unittest.TestCase):\n\
         \x20   def test_add(self): self.assertEqual(add(2, 3), 5)\n\
         \n\
         if __name__ == '__main__': unittest.main()\n",
    )?;

    for args in [vec!["add", "."], vec!["commit", "-m", "seed bug"]] {
        let spec = ProcessSpec::new("git").args(args).cwd(dir);
        let out = run(&spec).await?;
        if !out.success() {
            return Err(format!("git commit failed: {}", out.stderr).into());
        }
    }
    Ok(())
}
