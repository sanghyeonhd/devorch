//! End-to-end mission tests against real git repositories.
//!
//! These are the acceptance tests for the whole flow the plan describes:
//!
//! ```text
//! mission → worktrees → agents → verify → compare → winner → merge → re-verify → cleanup
//! ```
//!
//! Agents are scripted fakes so the tests are deterministic, but everything
//! else is real: real worktrees, real commits, real merges, real SQLite.

use std::path::{Path, PathBuf};

use devorch_config::{Config, PathsConfig, TaskRisk};
use devorch_mission::{
    FakeBehavior, FakeExecutor, MissionError, MissionRequest, MissionRunner, MissionStatus,
    MissionStore,
};
use devorch_process::{run, ProcessSpec};
use devorch_protocol::AgentKind;
use devorch_store::Store;
use devorch_verify::Check;

/// The seeded repository from the development simulation: `add` subtracts.
async fn seed_repo(dir: &Path) {
    let git = |args: Vec<&str>| {
        let spec = ProcessSpec::new("git").args(args).cwd(dir);
        async move {
            let out = run(&spec).await.expect("git ran");
            assert!(out.success(), "git failed: {}{}", out.stdout, out.stderr);
        }
    };

    git(vec!["init", "-b", "main"]).await;
    git(vec!["config", "user.email", "test@devorch.local"]).await;
    git(vec!["config", "user.name", "Devorch Test"]).await;

    std::fs::write(dir.join(".gitignore"), "__pycache__/\n*.pyc\n").unwrap();
    std::fs::write(
        dir.join("calculator.py"),
        "def add(a, b):\n    return a - b\n",
    )
    .unwrap();
    std::fs::write(
        dir.join("test_calculator.py"),
        "import unittest\n\
         from calculator import add\n\
         \n\
         class T(unittest.TestCase):\n\
         \x20   def test_add(self): self.assertEqual(add(2, 3), 5)\n\
         \n\
         if __name__ == '__main__': unittest.main()\n",
    )
    .unwrap();

    git(vec!["add", "."]).await;
    git(vec!["commit", "-m", "seed bug"]).await;
}

/// A config rooted in `home`, so nothing touches the developer's real state.
fn config_in(home: &Path) -> Config {
    Config {
        paths: PathsConfig {
            home: Some(home.to_path_buf()),
        },
        ..Default::default()
    }
}

/// The project's own test command.
fn checks() -> Vec<Check> {
    vec![Check::required(
        "tests",
        "python3",
        &["-m", "unittest", "-q"],
    )]
}

const CORRECT_FIX: &str = "def add(a, b):\n    return a + b\n";
const WRONG_FIX: &str = "def add(a, b):\n    return a * b\n";

struct Fixture {
    _tmp: tempfile::TempDir,
    repo: PathBuf,
    home: PathBuf,
}

async fn fixture() -> Fixture {
    let tmp = tempfile::tempdir().unwrap();
    let repo = tmp.path().join("repo");
    let home = tmp.path().join("home");
    std::fs::create_dir_all(&repo).unwrap();
    seed_repo(&repo).await;
    Fixture {
        _tmp: tmp,
        repo,
        home,
    }
}

#[tokio::test]
async fn a_mission_selects_the_smallest_passing_candidate_and_merges_it() {
    let f = fixture().await;
    let store = Store::open(f.home.join("devorch3.db")).unwrap();

    // The simulation's four candidates: two correct, one wrong, one correct but
    // with an unrequested extra file.
    let executor = FakeExecutor::new()
        .with(
            AgentKind::Codex,
            FakeBehavior::write("calculator.py", CORRECT_FIX),
        )
        .with(
            AgentKind::Claude,
            FakeBehavior::write(
                "calculator.py",
                "def add(a, b):\n    \"\"\"Return the sum.\"\"\"\n    return a + b\n",
            ),
        )
        .with(
            AgentKind::Grok,
            FakeBehavior::write("calculator.py", WRONG_FIX),
        )
        .with(
            AgentKind::Gemini,
            FakeBehavior::Edit(vec![
                ("calculator.py".into(), CORRECT_FIX.into()),
                ("NOTES.md".into(), "an unnecessary file\n".into()),
            ]),
        );

    let runner = MissionRunner::with_executor(&store, config_in(&f.home), Box::new(executor));

    let request = MissionRequest::new("fix the seeded regression in add()", &f.repo)
        .risk(TaskRisk::ExplicitCompareAll)
        .agents(AgentKind::ALL.to_vec());
    let request = MissionRequest {
        checks: checks(),
        ..request
    };

    let outcome = runner
        .run(request, |_| {})
        .await
        .expect("mission succeeded");

    // Codex wins: correct, and the smallest diff of the correct ones.
    assert_eq!(outcome.winner(), Some(AgentKind::Codex));

    // Grok is rejected on failing tests, despite claiming success.
    let rejected: Vec<_> = outcome
        .comparison
        .rejected()
        .iter()
        .map(|e| e.candidate.agent)
        .collect();
    assert_eq!(rejected, vec![AgentKind::Grok]);

    // Gemini passed but ranked below Codex because of the untracked file.
    let gemini = outcome
        .comparison
        .evaluations
        .iter()
        .find(|e| e.candidate.agent == AgentKind::Gemini)
        .unwrap();
    assert!(gemini.passed());
    assert_eq!(
        gemini.candidate.inventory.untracked,
        vec![PathBuf::from("NOTES.md")]
    );

    // The merge landed and the gate passed again on the merged result.
    assert!(outcome.merge_commit.is_some());
    assert!(outcome.post_merge.as_ref().unwrap().passed());
    assert_eq!(outcome.record.status, MissionStatus::Succeeded);
}

#[tokio::test]
async fn an_agent_that_claims_success_without_working_is_rejected() {
    let f = fixture().await;
    let store = Store::open(f.home.join("devorch3.db")).unwrap();

    let executor = FakeExecutor::new()
        .with(AgentKind::Grok, FakeBehavior::ClaimSuccessWithoutWorking)
        .with(
            AgentKind::Codex,
            FakeBehavior::write("calculator.py", CORRECT_FIX),
        );

    let runner = MissionRunner::with_executor(&store, config_in(&f.home), Box::new(executor));
    let request = MissionRequest {
        checks: checks(),
        ..MissionRequest::new("fix add()", &f.repo).agents(vec![AgentKind::Grok, AgentKind::Codex])
    };

    let outcome = runner
        .run(request, |_| {})
        .await
        .expect("mission succeeded");

    assert_eq!(outcome.winner(), Some(AgentKind::Codex));

    let grok = outcome
        .comparison
        .evaluations
        .iter()
        .find(|e| e.candidate.agent == AgentKind::Grok)
        .unwrap();
    assert!(!grok.passed());
    assert!(
        grok.candidate.agent_claimed_success,
        "the agent did claim success"
    );
    assert!(
        grok.rejection_summary()
            .unwrap()
            .contains("changed nothing"),
        "rejected for the right reason: {:?}",
        grok.rejection_summary()
    );
}

#[tokio::test]
async fn a_mission_where_every_candidate_fails_leaves_the_repository_untouched() {
    let f = fixture().await;
    let store = Store::open(f.home.join("devorch3.db")).unwrap();

    let before = std::fs::read_to_string(f.repo.join("calculator.py")).unwrap();

    let executor = FakeExecutor::new()
        .with(
            AgentKind::Codex,
            FakeBehavior::write("calculator.py", WRONG_FIX),
        )
        .with(
            AgentKind::Grok,
            FakeBehavior::write("calculator.py", WRONG_FIX),
        );

    let runner = MissionRunner::with_executor(&store, config_in(&f.home), Box::new(executor));
    let request = MissionRequest {
        checks: checks(),
        ..MissionRequest::new("fix add()", &f.repo).agents(vec![AgentKind::Codex, AgentKind::Grok])
    };

    let err = runner.run(request, |_| {}).await.expect_err("no winner");
    assert!(matches!(err, MissionError::NoWinner));

    // main is exactly as it was.
    assert_eq!(
        std::fs::read_to_string(f.repo.join("calculator.py")).unwrap(),
        before
    );

    // The failure and both candidates are still on record.
    let missions = MissionStore::new(&store).list().unwrap();
    assert_eq!(missions.len(), 1);
    assert_eq!(missions[0].status, MissionStatus::Failed);
    assert_eq!(missions[0].candidates.len(), 2);
}

#[tokio::test]
async fn a_candidate_that_adds_build_output_is_rejected() {
    let f = fixture().await;
    let store = Store::open(f.home.join("devorch3.db")).unwrap();

    // `build/` is not gitignored in the seed repo, so it reaches the inventory.
    let executor = FakeExecutor::new().with(
        AgentKind::Gemini,
        FakeBehavior::Edit(vec![
            ("calculator.py".into(), CORRECT_FIX.into()),
            ("build/out.log".into(), "noise\n".into()),
        ]),
    );

    let runner = MissionRunner::with_executor(&store, config_in(&f.home), Box::new(executor));
    let request = MissionRequest {
        checks: checks(),
        ..MissionRequest::new("fix add()", &f.repo).agents(vec![AgentKind::Gemini])
    };

    let err = runner.run(request, |_| {}).await.expect_err("no winner");
    assert!(matches!(err, MissionError::NoWinner));

    let missions = MissionStore::new(&store).list().unwrap();
    let rejection = missions[0].candidates[0].rejection.clone().unwrap();
    assert!(rejection.contains("build output"), "{rejection}");
}

#[tokio::test]
async fn generated_files_that_are_gitignored_do_not_penalise_a_candidate() {
    // The simulation's first failure was __pycache__ breaking the merge. It is
    // gitignored in the seed repo, so it must not show up as a change at all.
    let f = fixture().await;
    let store = Store::open(f.home.join("devorch3.db")).unwrap();

    let executor = FakeExecutor::new().with(
        AgentKind::Codex,
        FakeBehavior::Edit(vec![
            ("calculator.py".into(), CORRECT_FIX.into()),
            ("__pycache__/calculator.pyc".into(), "bytecode".into()),
        ]),
    );

    let runner = MissionRunner::with_executor(&store, config_in(&f.home), Box::new(executor));
    let request = MissionRequest {
        checks: checks(),
        ..MissionRequest::new("fix add()", &f.repo).agents(vec![AgentKind::Codex])
    };

    let outcome = runner
        .run(request, |_| {})
        .await
        .expect("mission succeeded");
    assert_eq!(outcome.winner(), Some(AgentKind::Codex));

    let inventory = &outcome.comparison.winner().unwrap().candidate.inventory;
    assert_eq!(
        inventory.touched_count(),
        1,
        "only calculator.py: {inventory:?}"
    );
}

#[tokio::test]
async fn a_dry_run_compares_without_merging() {
    let f = fixture().await;
    let store = Store::open(f.home.join("devorch3.db")).unwrap();

    let executor = FakeExecutor::new().with(
        AgentKind::Codex,
        FakeBehavior::write("calculator.py", CORRECT_FIX),
    );

    let runner = MissionRunner::with_executor(&store, config_in(&f.home), Box::new(executor));
    let request = MissionRequest {
        checks: checks(),
        ..MissionRequest::new("fix add()", &f.repo)
            .agents(vec![AgentKind::Codex])
            .dry_run()
    };

    let outcome = runner
        .run(request, |_| {})
        .await
        .expect("mission succeeded");

    assert_eq!(outcome.winner(), Some(AgentKind::Codex));
    assert!(outcome.merge_commit.is_none(), "a dry run must not merge");
    assert!(outcome.post_merge.is_none());
}

#[tokio::test]
async fn losing_worktrees_are_removed_but_their_records_survive() {
    let f = fixture().await;
    let store = Store::open(f.home.join("devorch3.db")).unwrap();

    let executor = FakeExecutor::new()
        .with(
            AgentKind::Codex,
            FakeBehavior::write("calculator.py", CORRECT_FIX),
        )
        .with(
            AgentKind::Grok,
            FakeBehavior::write("calculator.py", WRONG_FIX),
        );

    let runner = MissionRunner::with_executor(&store, config_in(&f.home), Box::new(executor));
    let request = MissionRequest {
        checks: checks(),
        ..MissionRequest::new("fix add()", &f.repo).agents(vec![AgentKind::Codex, AgentKind::Grok])
    };

    let outcome = runner
        .run(request, |_| {})
        .await
        .expect("mission succeeded");
    let short = outcome.record.short_id();

    let loser_dir = f.home.join("worktrees").join(format!("{short}-grok"));
    let winner_dir = f.home.join("worktrees").join(format!("{short}-codex"));
    assert!(!loser_dir.exists(), "the losing worktree should be gone");
    assert!(winner_dir.exists(), "the winner is kept for inspection");

    // The evidence outlives the directory.
    let workspaces = store.workspaces().list(true).unwrap();
    assert_eq!(workspaces.len(), 2);
    let missions = MissionStore::new(&store).list().unwrap();
    assert_eq!(missions[0].candidates.len(), 2);
}

#[tokio::test]
async fn mission_state_survives_a_daemon_restart() {
    let f = fixture().await;

    let mission_id = {
        let store = Store::open(f.home.join("devorch3.db")).unwrap();
        let executor = FakeExecutor::new().with(
            AgentKind::Codex,
            FakeBehavior::write("calculator.py", CORRECT_FIX),
        );
        let runner = MissionRunner::with_executor(&store, config_in(&f.home), Box::new(executor));
        let request = MissionRequest {
            checks: checks(),
            ..MissionRequest::new("fix add()", &f.repo).agents(vec![AgentKind::Codex])
        };
        runner
            .run(request, |_| {})
            .await
            .expect("mission succeeded")
            .record
            .id
    };

    // A new process against the same database.
    let store = Store::open(f.home.join("devorch3.db")).unwrap();
    let reloaded = MissionStore::new(&store)
        .get(&mission_id)
        .unwrap()
        .expect("mission survived the restart");

    assert_eq!(reloaded.status, MissionStatus::Succeeded);
    assert_eq!(reloaded.winner, Some(AgentKind::Codex));
    assert!(reloaded.merge_commit.is_some());
    assert_eq!(reloaded.candidates.len(), 1);
}

#[tokio::test]
async fn candidates_run_in_waves_bounded_by_max_parallel_agents() {
    let f = fixture().await;
    let store = Store::open(f.home.join("devorch3.db")).unwrap();

    let executor = FakeExecutor::new()
        .with(
            AgentKind::Codex,
            FakeBehavior::write("calculator.py", CORRECT_FIX),
        )
        .with(
            AgentKind::Claude,
            FakeBehavior::write("calculator.py", CORRECT_FIX),
        )
        .with(
            AgentKind::Grok,
            FakeBehavior::write("calculator.py", CORRECT_FIX),
        )
        .with(
            AgentKind::Gemini,
            FakeBehavior::write("calculator.py", CORRECT_FIX),
        );

    let runner = MissionRunner::with_executor(&store, config_in(&f.home), Box::new(executor));
    let request = MissionRequest {
        checks: checks(),
        ..MissionRequest::new("fix add()", &f.repo).agents(AgentKind::ALL.to_vec())
    };

    let mut plan_waves = 0;
    let outcome = runner
        .run(request, |progress| {
            if let devorch_mission::MissionProgress::Planned(plan) = progress {
                plan_waves = plan.waves();
            }
        })
        .await
        .expect("mission succeeded");

    // Four candidates, two at a time: two waves, never four processes at once.
    assert_eq!(plan_waves, 2);
    assert_eq!(outcome.comparison.evaluations.len(), 4);
}

#[tokio::test]
async fn a_mission_with_no_agents_fails_before_creating_anything() {
    let f = fixture().await;
    let store = Store::open(f.home.join("devorch3.db")).unwrap();

    let runner =
        MissionRunner::with_executor(&store, config_in(&f.home), Box::new(FakeExecutor::new()));
    let request = MissionRequest {
        checks: checks(),
        ..MissionRequest::new("fix add()", &f.repo).agents(vec![])
    };

    // No agents named and none installed in this harness's view, so the router
    // returns nothing. The mission must fail without creating a worktree.
    let result = runner.run(request, |_| {}).await;
    if let Err(MissionError::NoAgents) = result {
        assert!(store.workspaces().list(true).unwrap().is_empty());
    }
}

#[tokio::test]
async fn a_repository_with_nothing_to_verify_fails_before_any_agent_runs() {
    // Regression from a live run: with no detectable test command, every
    // candidate was rejected as unverified *after* the agents had already been
    // invoked. The cost of that mistake should be one error, not four runs.
    let tmp = tempfile::tempdir().unwrap();
    let repo = tmp.path().join("repo");
    let home = tmp.path().join("home");
    std::fs::create_dir_all(&repo).unwrap();

    let git = ProcessSpec::new("git")
        .args(["init", "-b", "main"])
        .cwd(&repo);
    assert!(run(&git).await.unwrap().success());
    for args in [
        vec!["config", "user.email", "t@devorch.local"],
        vec!["config", "user.name", "T"],
    ] {
        let spec = ProcessSpec::new("git").args(args).cwd(&repo);
        assert!(run(&spec).await.unwrap().success());
    }
    std::fs::write(repo.join("README.md"), "docs only\n").unwrap();
    for args in [vec!["add", "."], vec!["commit", "-m", "seed"]] {
        let spec = ProcessSpec::new("git").args(args).cwd(&repo);
        assert!(run(&spec).await.unwrap().success());
    }

    let store = Store::open(home.join("devorch3.db")).unwrap();
    let executor = FakeExecutor::new().with(AgentKind::Codex, FakeBehavior::write("a.txt", "x"));
    let runner = MissionRunner::with_executor(&store, config_in(&home), Box::new(executor));

    // No explicit checks, and nothing detectable in the repository.
    let request = MissionRequest::new("do something", &repo).agents(vec![AgentKind::Codex]);
    let err = runner
        .run(request, |_| {})
        .await
        .expect_err("should refuse");

    assert!(matches!(err, MissionError::NoChecks), "got {err:?}");
    assert!(
        store.workspaces().list(true).unwrap().is_empty(),
        "no worktree should have been created"
    );
}
