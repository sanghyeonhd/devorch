//! The Devorch command line interface.
//!
//! This binary is the R2 vertical slice: config → SQLite → repository →
//! worktree → persisted record → CLI command, end to end on real git. It is
//! deliberately narrow. Mission execution, routing and the GUI arrive in later
//! waves and must reuse these same crates rather than reimplementing them.

use std::path::PathBuf;

use anyhow::{Context, Result};
use clap::{Parser, Subcommand};
use devorch_config::Config;
use devorch_protocol::AgentKind;
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
    /// Report configuration, database and agent status.
    Doctor,
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
