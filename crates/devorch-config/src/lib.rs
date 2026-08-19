//! Devorch configuration.
//!
//! Two rules drive the defaults here. First, Devorch keeps **zero** idle agent
//! processes: an agent starts because a mission needs it and exits when the
//! mission is done. Second, having four agents installed is not a reason to run
//! four — `max_parallel_agents` defaults to 2, and candidate count is chosen by
//! task risk, not by availability.

use std::path::{Path, PathBuf};

use devorch_protocol::AgentKind;
use serde::{Deserialize, Serialize};

/// Failure modes of configuration loading.
#[derive(Debug, thiserror::Error)]
pub enum ConfigError {
    #[error("could not read {path}: {source}")]
    Read {
        path: PathBuf,
        #[source]
        source: std::io::Error,
    },

    #[error("invalid config at {path}: {source}")]
    Parse {
        path: PathBuf,
        #[source]
        source: Box<toml::de::Error>,
    },

    #[error("could not determine the home directory")]
    NoHome,

    #[error("{field} must be at least 1, got {value}")]
    OutOfRange { field: &'static str, value: usize },
}

/// How much scrutiny a task warrants, which decides how many candidates run.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TaskRisk {
    Low,
    Normal,
    High,
    /// The operator explicitly asked to compare every agent.
    ExplicitCompareAll,
}

/// Runtime process policy.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(default)]
pub struct RuntimeConfig {
    /// Hard ceiling on agent processes alive at once.
    pub max_parallel_agents: usize,
    /// How long a persistent agent session may idle before being closed.
    pub idle_session_ttl_seconds: u64,
    /// Whether persistent vendor protocols may be used when available.
    pub persistent_protocols: PersistentProtocols,
}

/// Whether to keep vendor protocol sessions alive between turns.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PersistentProtocols {
    /// Decide per task, based on whether repeated turns justify the process.
    Auto,
    Always,
    Never,
}

impl Default for RuntimeConfig {
    fn default() -> Self {
        Self {
            max_parallel_agents: 2,
            idle_session_ttl_seconds: 300,
            persistent_protocols: PersistentProtocols::Auto,
        }
    }
}

/// Which agents Devorch may select, and how many at a time.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(default)]
pub struct AgentsConfig {
    /// Agents eligible for selection. Empty means all installed agents.
    pub enabled: Vec<String>,
}

impl Default for AgentsConfig {
    fn default() -> Self {
        Self {
            enabled: AgentKind::ALL
                .iter()
                .map(|a| a.as_str().to_string())
                .collect(),
        }
    }
}

/// Where Devorch keeps its data.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(default)]
pub struct PathsConfig {
    /// Root of Devorch's own state. Defaults to `~/.devorch`.
    pub home: Option<PathBuf>,
}

/// The complete configuration.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(default)]
pub struct Config {
    pub runtime: RuntimeConfig,
    pub agents: AgentsConfig,
    pub paths: PathsConfig,
}

impl Config {
    /// Load configuration from `path`, or fall back to defaults when absent.
    ///
    /// A missing file is normal — Devorch runs on defaults. A malformed file is
    /// not: it is reported rather than silently ignored, because a typo that
    /// quietly reverts `max_parallel_agents` to the default would be an
    /// invisible resource decision.
    pub fn load(path: impl AsRef<Path>) -> Result<Self, ConfigError> {
        let path = path.as_ref();
        let text = match std::fs::read_to_string(path) {
            Ok(text) => text,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(Self::default()),
            Err(source) => {
                return Err(ConfigError::Read {
                    path: path.to_path_buf(),
                    source,
                })
            }
        };

        let config: Config = toml::from_str(&text).map_err(|source| ConfigError::Parse {
            path: path.to_path_buf(),
            source: Box::new(source),
        })?;
        config.validate()?;
        Ok(config)
    }

    /// Reject values that would make the scheduler misbehave.
    pub fn validate(&self) -> Result<(), ConfigError> {
        if self.runtime.max_parallel_agents == 0 {
            return Err(ConfigError::OutOfRange {
                field: "runtime.max_parallel_agents",
                value: 0,
            });
        }
        Ok(())
    }

    /// Root of Devorch state: `paths.home`, else `$DEVORCH_HOME`, else `~/.devorch`.
    pub fn home_dir(&self) -> Result<PathBuf, ConfigError> {
        if let Some(home) = &self.paths.home {
            return Ok(home.clone());
        }
        if let Ok(env) = std::env::var("DEVORCH_HOME") {
            if !env.is_empty() {
                return Ok(PathBuf::from(env));
            }
        }
        let home = std::env::var("HOME")
            .ok()
            .or_else(|| std::env::var("USERPROFILE").ok())
            .filter(|h| !h.is_empty())
            .ok_or(ConfigError::NoHome)?;
        Ok(PathBuf::from(home).join(".devorch"))
    }

    /// Path of the Rust core database.
    ///
    /// This is a separate file from the Go baseline's `devorch.db` so the
    /// migration cannot corrupt existing OkAON data while both runtimes exist.
    pub fn database_path(&self) -> Result<PathBuf, ConfigError> {
        Ok(self.home_dir()?.join("devorch3.db"))
    }

    /// Directory holding candidate worktrees.
    pub fn worktree_root(&self) -> Result<PathBuf, ConfigError> {
        Ok(self.home_dir()?.join("worktrees"))
    }

    /// The default config file location.
    pub fn default_path() -> Result<PathBuf, ConfigError> {
        Self::default().home_dir().map(|h| h.join("config.toml"))
    }

    /// Agents eligible for selection, ignoring names that are not recognised.
    pub fn enabled_agents(&self) -> Vec<AgentKind> {
        if self.agents.enabled.is_empty() {
            return AgentKind::ALL.to_vec();
        }
        self.agents
            .enabled
            .iter()
            .filter_map(|name| name.parse().ok())
            .collect()
    }

    /// How many candidates to run for a task of the given risk.
    ///
    /// This is the bootstrap policy used before OkAON has enough data. It is
    /// also the reason four installed agents do not imply four running agents.
    pub fn candidate_count(&self, risk: TaskRisk) -> usize {
        let available = self.enabled_agents().len().max(1);
        let wanted = match risk {
            TaskRisk::Low => 1,
            TaskRisk::Normal => 2,
            TaskRisk::High | TaskRisk::ExplicitCompareAll => 4,
        };
        wanted.min(available)
    }

    /// How many of those candidates may execute at once.
    ///
    /// High-risk work still evaluates up to four candidates, but in waves — the
    /// concurrency cap is what keeps peak process count bounded.
    pub fn parallelism(&self, risk: TaskRisk) -> usize {
        self.candidate_count(risk)
            .min(self.runtime.max_parallel_agents)
    }
}

// clippy::result_large_err fires at 128 bytes. On Windows, `toml::de::Error`
// plus a PathBuf lands on or over that threshold; boxing the parser error
// keeps the enum small on every target CI builds.
const _: () = assert!(std::mem::size_of::<ConfigError>() < 128);

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn defaults_keep_parallelism_at_two() {
        let config = Config::default();
        assert_eq!(config.runtime.max_parallel_agents, 2);
        assert_eq!(config.runtime.idle_session_ttl_seconds, 300);
        assert_eq!(config.enabled_agents().len(), 4);
    }

    #[test]
    fn a_missing_config_file_yields_defaults() {
        let dir = tempfile::tempdir().unwrap();
        let config = Config::load(dir.path().join("absent.toml")).unwrap();
        assert_eq!(config, Config::default());
    }

    #[test]
    fn config_error_fits_clippy_result_large_err_on_every_target() {
        // Windows CI failed here: toml::de::Error + PathBuf is >= 128 bytes
        // unboxed. The const assertion above is the real pin; this test makes
        // the size visible in the suite.
        let size = std::mem::size_of::<ConfigError>();
        assert!(
            size < 128,
            "ConfigError is {size} bytes; clippy::result_large_err fires at 128"
        );
    }

    #[test]
    fn a_malformed_config_file_is_an_error_not_a_silent_default() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("config.toml");
        std::fs::write(&path, "runtime = \"not a table\"\n").unwrap();
        assert!(matches!(
            Config::load(&path),
            Err(ConfigError::Parse { .. })
        ));
    }

    #[test]
    fn partial_config_merges_over_defaults() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("config.toml");
        std::fs::write(&path, "[runtime]\nmax_parallel_agents = 4\n").unwrap();

        let config = Config::load(&path).unwrap();
        assert_eq!(config.runtime.max_parallel_agents, 4);
        // Untouched fields keep their defaults.
        assert_eq!(config.runtime.idle_session_ttl_seconds, 300);
        assert_eq!(config.enabled_agents().len(), 4);
    }

    #[test]
    fn zero_parallelism_is_rejected() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("config.toml");
        std::fs::write(&path, "[runtime]\nmax_parallel_agents = 0\n").unwrap();
        assert!(matches!(
            Config::load(&path),
            Err(ConfigError::OutOfRange { .. })
        ));
    }

    #[test]
    fn candidate_count_scales_with_risk_but_concurrency_does_not() {
        let config = Config::default();

        assert_eq!(config.candidate_count(TaskRisk::Low), 1);
        assert_eq!(config.candidate_count(TaskRisk::Normal), 2);
        assert_eq!(config.candidate_count(TaskRisk::High), 4);
        assert_eq!(config.candidate_count(TaskRisk::ExplicitCompareAll), 4);

        // Four candidates, but never more than two processes at a time.
        assert_eq!(config.parallelism(TaskRisk::High), 2);
        assert_eq!(config.parallelism(TaskRisk::ExplicitCompareAll), 2);
        assert_eq!(config.parallelism(TaskRisk::Low), 1);
    }

    #[test]
    fn candidate_count_cannot_exceed_the_enabled_agents() {
        let config = Config {
            agents: AgentsConfig {
                enabled: vec!["codex".into()],
            },
            ..Default::default()
        };
        assert_eq!(config.candidate_count(TaskRisk::High), 1);
    }

    #[test]
    fn unknown_agent_names_are_ignored_rather_than_fatal() {
        let config = Config {
            agents: AgentsConfig {
                enabled: vec!["codex".into(), "copilot".into()],
            },
            ..Default::default()
        };
        assert_eq!(config.enabled_agents(), vec![AgentKind::Codex]);
    }

    #[test]
    fn explicit_home_wins_over_the_environment() {
        let config = Config {
            paths: PathsConfig {
                home: Some(PathBuf::from("/custom/devorch")),
            },
            ..Default::default()
        };
        assert_eq!(config.home_dir().unwrap(), PathBuf::from("/custom/devorch"));
        assert_eq!(
            config.database_path().unwrap(),
            PathBuf::from("/custom/devorch/devorch3.db")
        );
    }

    #[test]
    fn the_rust_database_is_separate_from_the_go_baseline() {
        let config = Config {
            paths: PathsConfig {
                home: Some(PathBuf::from("/custom/devorch")),
            },
            ..Default::default()
        };
        let db = config.database_path().unwrap();
        assert!(db.ends_with("devorch3.db"));
        assert!(!db.ends_with("devorch.db"));
    }
}
