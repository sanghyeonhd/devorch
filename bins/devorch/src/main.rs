//! The Devorch command line interface.
//!
//! This binary is the R2 vertical slice: config → SQLite → repository →
//! worktree → persisted record → CLI command, end to end on real git. It is
//! deliberately narrow. Mission execution, routing and the GUI arrive in later
//! waves and must reuse these same crates rather than reimplementing them.

use std::path::PathBuf;

use anyhow::{Context, Result};
use clap::{Parser, Subcommand};
use devorch_config::{Config, TaskRisk};
use devorch_mission::{MissionRequest, MissionRunner, MissionStore};
use devorch_protocol::{AgentEvent, AgentKind};
use devorch_store::Store;
use devorch_workspace::{CreateWorkspace, WorkspaceManager};

#[derive(Parser)]
#[command(
    name = "devorch",
    version,
    about = "Local-first multi-agent development orchestrator"
)]
struct Cli {
    /// Path to config.toml. Defaults to $DEVORCH_HOME/config.toml.
    #[arg(long, global = true)]
    config: Option<PathBuf>,

    /// Emit machine-readable JSON instead of human-readable text.
    #[arg(long, global = true)]
    json: bool,

    #[command(subcommand)]
    command: Command,
}

#[derive(Subcommand)]
enum Command {
    /// Manage isolated git worktrees for agent candidates.
    Workspace {
        #[command(subcommand)]
        command: WorkspaceCommand,
    },
    /// Inspect the agent CLIs installed on this machine.
    Agent {
        #[command(subcommand)]
        command: AgentCommand,
    },
    /// Run and inspect missions.
    Mission {
        #[command(subcommand)]
        command: MissionCommand,
    },
    /// Report configuration, database and agent status.
    Doctor,
}

#[derive(Subcommand)]
enum MissionCommand {
    /// Run a mission: compare candidates, merge the winner, verify the merge.
    Run {
        /// What the agents should accomplish.
        #[arg(long)]
        goal: String,
        /// Repository to work in.
        #[arg(long, default_value = ".")]
        repo: PathBuf,
        /// Agents to use, comma separated. Omit to let the router decide.
        #[arg(long, value_delimiter = ',')]
        agents: Vec<String>,
        /// How much scrutiny the task warrants.
        #[arg(long, default_value = "normal")]
        risk: String,
        /// Cap on agents running at once. Overrides the config default.
        #[arg(long)]
        max_parallel: Option<usize>,
        /// Paths the task is authorized to touch.
        #[arg(long, value_delimiter = ',')]
        owned_paths: Vec<PathBuf>,
        /// Per-agent wall-clock ceiling, in seconds.
        #[arg(long)]
        timeout: Option<u64>,
        /// Verification command, repeatable. Overrides project detection.
        ///
        /// Written as a plain command line: `--check "cargo test --workspace"`.
        /// A candidate that fails any of these cannot win.
        #[arg(long = "check")]
        checks: Vec<String>,
        /// Compare candidates but do not merge.
        #[arg(long)]
        dry_run: bool,
    },
    /// List missions, newest first.
    List,
    /// Show one mission in full.
    Show { id: String },
}

#[derive(Subcommand)]
enum WorkspaceCommand {
    /// Create a worktree and its persisted record.
    Create {
        /// Workspace name; also the branch suffix and directory name.
        name: String,
        /// Repository to branch from. Defaults to the current directory.
        #[arg(long, default_value = ".")]
        repo: PathBuf,
        /// Revision to branch from. Defaults to HEAD.
        #[arg(long)]
        base: Option<String>,
        /// Agent that will work in this workspace.
        #[arg(long)]
        agent: Option<String>,
    },
    /// List workspaces.
    List {
        /// Include workspaces that have already been removed.
        #[arg(long)]
        all: bool,
    },
    /// Remove a worktree and its branch, keeping the record as history.
    Remove {
        name: String,
        /// Discard uncommitted work in the worktree.
        #[arg(long)]
        force: bool,
    },
    /// Show the complete change inventory of a workspace.
    ///
    /// Includes untracked files, which `git diff` alone would miss.
    Inventory { name: String },
}

#[derive(Subcommand)]
enum AgentCommand {
    /// Discover what each installed agent CLI actually supports.
    Inspect {
        /// Inspect one agent instead of all four.
        #[arg(long)]
        agent: Option<String>,
    },
}

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_env("DEVORCH_LOG")
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("warn")),
        )
        .with_writer(std::io::stderr)
        .init();

    let cli = Cli::parse();

    let config_path = match &cli.config {
        Some(path) => path.clone(),
        None => Config::default_path().context("resolve default config path")?,
    };
    let config = Config::load(&config_path)
        .with_context(|| format!("load config from {}", config_path.display()))?;

    match cli.command {
        Command::Workspace { command } => workspace(command, &config, cli.json).await,
        Command::Agent { command } => agent(command, cli.json).await,
        Command::Mission { command } => mission(command, config, cli.json).await,
        Command::Doctor => doctor(&config, &config_path, cli.json).await,
    }
}

/// Open the Rust core database, creating it and its parent directory if needed.
fn open_store(config: &Config) -> Result<Store> {
    let path = config.database_path().context("resolve database path")?;
    Store::open(&path).with_context(|| format!("open database at {}", path.display()))
}

async fn workspace(command: WorkspaceCommand, config: &Config, json: bool) -> Result<()> {
    let store = open_store(config)?;
    let root = config.worktree_root().context("resolve worktree root")?;
    let manager = WorkspaceManager::new(&store, &root);

    match command {
        WorkspaceCommand::Create {
            name,
            repo,
            base,
            agent,
        } => {
            let agent = match agent {
                Some(raw) => Some(
                    raw.parse::<AgentKind>()
                        .with_context(|| format!("unknown agent `{raw}`"))?,
                ),
                None => None,
            };

            let record = manager
                .create(CreateWorkspace {
                    name,
                    repo_path: repo,
                    base,
                    agent,
                })
                .await
                .context("create workspace")?;

            if json {
                println!(
                    "{}",
                    serde_json::json!({
                        "id": record.id.as_str(),
                        "name": record.name,
                        "path": record.worktree_path,
                        "branch": record.branch,
                        "base_commit": record.base_commit,
                        "agent": record.agent.map(|a| a.as_str()),
                    })
                );
            } else {
                println!("created workspace {}", record.name);
                println!("  path   {}", record.worktree_path.display());
                println!("  branch {}", record.branch);
                println!(
                    "  base   {}",
                    &record.base_commit[..12.min(record.base_commit.len())]
                );
            }
        }

        WorkspaceCommand::List { all } => {
            let records = manager.list(all).context("list workspaces")?;
            if json {
                let rows: Vec<_> = records
                    .iter()
                    .map(|r| {
                        serde_json::json!({
                            "id": r.id.as_str(),
                            "name": r.name,
                            "path": r.worktree_path,
                            "branch": r.branch,
                            "base_commit": r.base_commit,
                            "agent": r.agent.map(|a| a.as_str()),
                            "state": format!("{:?}", r.state).to_lowercase(),
                            "created_at": r.created_at.to_rfc3339(),
                        })
                    })
                    .collect();
                println!("{}", serde_json::to_string_pretty(&rows)?);
            } else if records.is_empty() {
                println!("no workspaces");
            } else {
                for r in records {
                    let agent = r.agent.map(|a| a.as_str()).unwrap_or("-");
                    println!(
                        "{:<20} {:<10} {:<8} {}",
                        r.name,
                        format!("{:?}", r.state).to_lowercase(),
                        agent,
                        r.worktree_path.display()
                    );
                }
            }
        }

        WorkspaceCommand::Remove { name, force } => {
            let record = manager
                .remove(&name, force)
                .await
                .context("remove workspace")?;
            if json {
                println!(
                    "{}",
                    serde_json::json!({"id": record.id.as_str(), "name": record.name, "state": "removed"})
                );
            } else {
                println!("removed workspace {}", record.name);
            }
        }

        WorkspaceCommand::Inventory { name } => {
            let inv = manager.inventory(&name).await.context("read inventory")?;
            if json {
                println!("{}", serde_json::to_string_pretty(&inv)?);
            } else {
                println!("workspace {name}");
                println!("  touched paths   {}", inv.touched_count());
                println!("  churn           {}", inv.churn);
                print_paths("tracked modified", &inv.tracked_modified);
                print_paths("staged", &inv.staged);
                print_paths("untracked", &inv.untracked);
                print_paths("deleted", &inv.deleted);
                print_paths("binary", &inv.binary);
                print_paths("generated", &inv.generated);
            }
        }
    }
    Ok(())
}

fn print_paths(label: &str, paths: &[PathBuf]) {
    if paths.is_empty() {
        return;
    }
    println!("  {label}:");
    for path in paths {
        println!("    {}", path.display());
    }
}

async fn agent(command: AgentCommand, json: bool) -> Result<()> {
    let AgentCommand::Inspect { agent } = command;

    let capabilities = match agent {
        Some(raw) => {
            let kind = raw
                .parse::<AgentKind>()
                .with_context(|| format!("unknown agent `{raw}`"))?;
            vec![devorch_agent::inspect(kind).await]
        }
        None => devorch_agent::inspect_all().await,
    };

    if json {
        println!("{}", serde_json::to_string_pretty(&capabilities)?);
        return Ok(());
    }

    for caps in capabilities {
        if !caps.installed {
            println!("{:<8} not installed", caps.id.as_str());
            continue;
        }
        println!(
            "{:<8} {}",
            caps.id.as_str(),
            caps.version.as_deref().unwrap_or("unknown version")
        );
        println!(
            "         mode       {}",
            caps.preferred_mode()
                .map(|m| format!("{m:?}"))
                .unwrap_or_else(|| "none".into())
        );
        println!(
            "         formats    {}",
            if caps.structured_formats.is_empty() {
                "-".to_string()
            } else {
                caps.structured_formats.join(", ")
            }
        );
        println!(
            "         protocols  {}",
            if caps.protocols.is_empty() {
                "-".to_string()
            } else {
                caps.protocols.join(", ")
            }
        );
        println!(
            "         resume {}   tool permissions {}",
            caps.resume, caps.tool_permissions
        );
    }
    Ok(())
}

async fn doctor(config: &Config, config_path: &PathBuf, json: bool) -> Result<()> {
    let home = config.home_dir()?;
    let db_path = config.database_path()?;
    let store = open_store(config)?;
    let migrations = store.applied_migrations()?;
    let workspaces = store.workspaces().list(false)?.len();
    let capabilities = devorch_agent::inspect_all().await;
    let installed = capabilities.iter().filter(|c| c.installed).count();

    if json {
        println!(
            "{}",
            serde_json::to_string_pretty(&serde_json::json!({
                "config_path": config_path,
                "config_exists": config_path.exists(),
                "home": home,
                "database": db_path,
                "migrations": migrations,
                "active_workspaces": workspaces,
                "max_parallel_agents": config.runtime.max_parallel_agents,
                "agents_installed": installed,
                "agents": capabilities,
            }))?
        );
        return Ok(());
    }

    println!("devorch doctor");
    println!(
        "  config              {} ({})",
        config_path.display(),
        if config_path.exists() {
            "loaded"
        } else {
            "defaults"
        }
    );
    println!("  home                {}", home.display());
    println!("  database            {}", db_path.display());
    println!("  migrations applied  {}", migrations.len());
    println!("  active workspaces   {workspaces}");
    println!(
        "  max parallel agents {}",
        config.runtime.max_parallel_agents
    );
    println!("  agents installed    {installed}/4");
    for caps in &capabilities {
        println!(
            "    {:<8} {}",
            caps.id.as_str(),
            if caps.installed {
                caps.version.as_deref().unwrap_or("installed")
            } else {
                "not installed"
            }
        );
    }
    Ok(())
}

/// Parse a `--check "program args..."` flag into a required check.
///
/// Split on whitespace rather than run through a shell: a verification command
/// is executed directly, so a repository cannot smuggle a pipeline or a
/// redirect into the gate that judges it.
fn parse_check(raw: &str) -> Result<devorch_verify::Check> {
    let mut parts = raw.split_whitespace();
    let program = parts
        .next()
        .with_context(|| format!("empty --check value: {raw:?}"))?;
    let args: Vec<&str> = parts.collect();
    Ok(devorch_verify::Check::required(program, program, &args))
}

/// Parse the `--risk` flag.
fn parse_risk(raw: &str) -> Result<TaskRisk> {
    match raw.trim().to_ascii_lowercase().as_str() {
        "low" => Ok(TaskRisk::Low),
        "normal" => Ok(TaskRisk::Normal),
        "high" => Ok(TaskRisk::High),
        "all" | "compare-all" => Ok(TaskRisk::ExplicitCompareAll),
        other => anyhow::bail!("unknown risk `{other}`; expected low, normal, high or all"),
    }
}

async fn mission(command: MissionCommand, mut config: Config, json: bool) -> Result<()> {
    let store = open_store(&config)?;

    match command {
        MissionCommand::Run {
            goal,
            repo,
            agents,
            risk,
            max_parallel,
            owned_paths,
            timeout,
            checks,
            dry_run,
        } => {
            if let Some(max) = max_parallel {
                anyhow::ensure!(max >= 1, "--max-parallel must be at least 1");
                config.runtime.max_parallel_agents = max;
            }

            let agents = agents
                .iter()
                .map(|raw| {
                    raw.parse::<AgentKind>()
                        .with_context(|| format!("unknown agent `{raw}`"))
                })
                .collect::<Result<Vec<_>>>()?;

            let request = MissionRequest {
                agents,
                owned_paths,
                agent_timeout: timeout.map(std::time::Duration::from_secs),
                checks: checks
                    .iter()
                    .map(|raw| parse_check(raw))
                    .collect::<Result<Vec<_>>>()?,
                dry_run,
                ..MissionRequest::new(goal, repo).risk(parse_risk(&risk)?)
            };

            let runner = MissionRunner::new(&store, config);
            let mut text = TextBuffer::default();
            let outcome = runner
                .run(request, |progress| {
                    report_progress(&progress, &mut text, json)
                })
                .await;

            match outcome {
                Ok(outcome) => {
                    if json {
                        println!(
                            "{}",
                            serde_json::to_string_pretty(&serde_json::json!({
                                "mission": outcome.record,
                                "winner": outcome.winner().map(|a| a.as_str()),
                                "merge_commit": outcome.merge_commit,
                                "reasoning": outcome.plan.reasoning,
                            }))?
                        );
                    } else {
                        print_outcome(&outcome);
                    }
                    Ok(())
                }
                // A mission that finds no acceptable candidate is a real
                // answer, not a crash. It exits non-zero so scripts can react,
                // and says why.
                Err(e) => {
                    eprintln!("mission failed: {e}");
                    std::process::exit(1);
                }
            }
        }

        MissionCommand::List => {
            let missions = MissionStore::new(&store).list()?;
            if json {
                println!("{}", serde_json::to_string_pretty(&missions)?);
            } else if missions.is_empty() {
                println!("no missions");
            } else {
                for m in missions {
                    println!(
                        "{:<28} {:<10} {:<8} {}",
                        m.id.as_str(),
                        format!("{:?}", m.status).to_lowercase(),
                        m.winner.map(|a| a.as_str()).unwrap_or("-"),
                        m.goal
                    );
                }
            }
            Ok(())
        }

        MissionCommand::Show { id } => {
            let mission_id = id
                .parse()
                .map_err(|_| anyhow::anyhow!("invalid id `{id}`"))?;
            let record = MissionStore::new(&store)
                .get(&mission_id)?
                .with_context(|| format!("no mission with id {id}"))?;

            if json {
                println!("{}", serde_json::to_string_pretty(&record)?);
                return Ok(());
            }

            println!("mission {}", record.id);
            println!("  goal    {}", record.goal);
            println!("  status  {:?}", record.status);
            println!("  base    {}", short(&record.base_commit));
            if let Some(winner) = record.winner {
                println!("  winner  {winner}");
            }
            if let Some(commit) = &record.merge_commit {
                println!("  merged  {}", short(commit));
            }
            if let Some(failure) = &record.failure {
                println!("  failed  {failure}");
            }
            println!("  candidates:");
            for c in &record.candidates {
                println!(
                    "    {:<8} {:<8} {} files, {} churn, {}ms{}",
                    c.agent.as_str(),
                    if c.passed { "passed" } else { "rejected" },
                    c.touched_paths,
                    c.churn,
                    c.duration_ms,
                    c.rejection
                        .as_ref()
                        .map(|r| format!(" — {r}"))
                        .unwrap_or_default()
                );
            }
            Ok(())
        }
    }
}

/// First 12 characters of a commit SHA.
fn short(sha: &str) -> &str {
    &sha[..12.min(sha.len())]
}

/// Accumulates token-level text deltas so the log reads as sentences.
///
/// Grok streams a delta per word; printing one line each turns a two-sentence
/// answer into forty lines of noise. Deltas are buffered and flushed on a
/// sentence boundary or when the agent moves on to something else.
#[derive(Default)]
struct TextBuffer {
    agent: Option<AgentKind>,
    buffer: String,
}

impl TextBuffer {
    /// Add a delta, returning a line when one is ready to print.
    fn push(&mut self, agent: AgentKind, text: &str) -> Option<String> {
        let flushed = (self.agent != Some(agent)).then(|| self.flush()).flatten();
        self.agent = Some(agent);
        self.buffer.push_str(text);

        if flushed.is_some() {
            return flushed;
        }
        // Flush on a sentence end, or when the buffer grows past a comfortable
        // line, so a long answer still appears while it is being written.
        let ends_sentence = self.buffer.trim_end().ends_with(['.', '!', '?', '\n']);
        if ends_sentence || self.buffer.len() > 160 {
            return self.flush();
        }
        None
    }

    /// Emit whatever is buffered.
    fn flush(&mut self) -> Option<String> {
        let agent = self.agent?;
        let text = self.buffer.trim().to_string();
        self.buffer.clear();
        (!text.is_empty()).then(|| format!("  [{agent}] {text}"))
    }
}

/// Stream mission progress to the terminal.
///
/// Suppressed under `--json` so the structured result stays the only thing on
/// stdout and remains parseable.
fn report_progress(
    progress: &devorch_mission::MissionProgress<'_>,
    text: &mut TextBuffer,
    json: bool,
) {
    use devorch_mission::MissionProgress;

    if json {
        return;
    }

    // Anything that is not more assistant text ends the current sentence.
    if !matches!(
        progress,
        MissionProgress::AgentEvent(_, AgentEvent::TextDelta(_))
    ) {
        if let Some(line) = text.flush() {
            println!("{line}");
        }
    }

    match progress {
        MissionProgress::Planned(plan) => {
            let names: Vec<_> = plan.candidates.iter().map(|a| a.as_str()).collect();
            println!(
                "plan: {} candidate(s) [{}], {} at a time",
                plan.candidates.len(),
                names.join(", "),
                plan.parallelism
            );
            for reason in &plan.reasoning {
                println!("  - {reason}");
            }
        }
        MissionProgress::CandidateStarted(agent, workspace) => {
            println!("→ {agent} started in {workspace}");
        }
        MissionProgress::AgentEvent(agent, event) => {
            // Reasoning text is never echoed: it is the model's private
            // thinking, not its answer.
            if let AgentEvent::TextDelta(t) = event {
                if !t.reasoning {
                    if let Some(line) = text.push(*agent, &t.text) {
                        println!("{line}");
                    }
                }
            }
        }
        MissionProgress::CandidateFinished(candidate) => {
            println!(
                "← {} finished: {} file(s), {} churn, tests {}",
                candidate.agent,
                candidate.inventory.touched_count(),
                candidate.inventory.churn,
                if candidate.verification.passed() {
                    "passed"
                } else {
                    "failed"
                }
            );
        }
        MissionProgress::Compared(comparison) => {
            println!("comparison:");
            for evaluation in &comparison.evaluations {
                println!(
                    "  {:<8} {:<8} {} files, {} churn{}",
                    evaluation.candidate.agent.as_str(),
                    if evaluation.passed() {
                        "passed"
                    } else {
                        "rejected"
                    },
                    evaluation.score.touched_paths,
                    evaluation.score.churn,
                    evaluation
                        .rejection_summary()
                        .map(|r| format!(" — {r}"))
                        .unwrap_or_default()
                );
            }
        }
        MissionProgress::Merged(commit) => println!("merged as {}", short(commit)),
        MissionProgress::PostMergeVerified(report) => {
            println!(
                "post-merge gate: {}",
                if report.passed() { "passed" } else { "FAILED" }
            );
        }
    }
}

/// Print the final result of a mission.
fn print_outcome(outcome: &devorch_mission::MissionOutcome) {
    println!();
    match outcome.winner() {
        Some(winner) => println!("winner: {winner}"),
        None => println!("winner: none"),
    }
    if let Some(commit) = &outcome.merge_commit {
        println!("merged: {}", short(commit));
    } else {
        println!("merged: no (dry run)");
    }
    println!("mission: {}", outcome.record.id);
}
