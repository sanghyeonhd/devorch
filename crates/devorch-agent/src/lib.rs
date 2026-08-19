//! Agent contracts and runtime capability discovery.
//!
//! Devorch never hardcodes what an installed agent CLI can do. Flags move
//! between versions, and four vendors move independently. Instead each agent is
//! *inspected* at runtime — version plus help text — and the adapter selects a
//! runtime mode from what the installed binary actually advertises.

use std::path::PathBuf;
use std::time::Duration;

use devorch_process::{run, ProcessError, ProcessSpec};
use devorch_protocol::{AgentKind, AgentRuntimeMode};
use serde::{Deserialize, Serialize};

pub mod adapter;
pub mod inspect;
pub mod runner;

pub use adapter::{classify_failure, AgentAdapter, ExecuteRequest, TerminalTracker};
pub use inspect::{inspect, inspect_all};
pub use runner::{run_agent, AgentRun};

/// Failure modes of agent inspection and execution.
#[derive(Debug, thiserror::Error)]
pub enum AgentError {
    #[error("{agent} is not installed")]
    NotInstalled { agent: AgentKind },

    #[error("{agent} did not respond to inspection: {detail}")]
    InspectionFailed { agent: AgentKind, detail: String },

    #[error(transparent)]
    Process(#[from] ProcessError),
}

/// What an installed agent CLI can actually do, as observed at runtime.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct AgentCapabilities {
    pub id: AgentKind,
    pub installed: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub path: Option<PathBuf>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    /// Runtime modes this binary supports, most preferred first.
    pub runtime_modes: Vec<AgentRuntimeMode>,
    /// Structured output formats detected in the help text.
    pub structured_formats: Vec<String>,
    /// Whether a previous session can be resumed.
    pub resume: bool,
    /// Whether tool permissions can be constrained from the command line.
    pub tool_permissions: bool,
    /// Additional protocols the binary advertises (`acp`, `a2a`, `stdio`, `mcp`).
    pub protocols: Vec<String>,
}

impl AgentCapabilities {
    /// A record for an agent that is not installed.
    pub fn missing(id: AgentKind) -> Self {
        Self {
            id,
            installed: false,
            path: None,
            version: None,
            runtime_modes: Vec::new(),
            structured_formats: Vec::new(),
            resume: false,
            tool_permissions: false,
            protocols: Vec::new(),
        }
    }

    /// The mode Devorch should use by default.
    ///
    /// A structured one-shot run is preferred because it leaves no idle process
    /// behind. PTY scraping is a last resort, never the default when a
    /// structured protocol exists.
    pub fn preferred_mode(&self) -> Option<AgentRuntimeMode> {
        [
            AgentRuntimeMode::OneShotStructured,
            AgentRuntimeMode::PersistentProtocol,
            AgentRuntimeMode::InteractivePty,
        ]
        .into_iter()
        .find(|mode| self.runtime_modes.contains(mode))
    }

    /// Whether this agent can be driven without scraping a terminal.
    pub fn has_structured_output(&self) -> bool {
        !self.structured_formats.is_empty()
    }
}

/// The command name Devorch invokes for an agent.
pub fn binary_name(agent: AgentKind) -> &'static str {
    match agent {
        AgentKind::Codex => "codex",
        AgentKind::Claude => "claude",
        AgentKind::Grok => "grok",
        AgentKind::Gemini => "gemini",
        AgentKind::Fake => "devorch-fake-agent",
    }
}

/// How long an inspection command may take before it is treated as a hang.
pub const INSPECT_TIMEOUT: Duration = Duration::from_secs(30);

/// Run `binary <args>` and return stdout+stderr, or `None` on any failure.
///
/// Help output arrives on stdout for some CLIs and stderr for others, so both
/// are searched. An agent that fails to answer is simply reported as
/// uninspectable rather than aborting inspection of the others.
pub(crate) async fn probe(binary: &str, args: &[&str]) -> Option<String> {
    let spec = ProcessSpec::new(binary).args(args).timeout(INSPECT_TIMEOUT);
    let out = run(&spec).await.ok()?;
    let combined = format!("{}\n{}", out.stdout, out.stderr);
    (!combined.trim().is_empty()).then_some(combined)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn a_missing_agent_has_no_usable_mode() {
        let caps = AgentCapabilities::missing(AgentKind::Codex);
        assert!(!caps.installed);
        assert_eq!(caps.preferred_mode(), None);
        assert!(!caps.has_structured_output());
    }

    #[test]
    fn structured_one_shot_is_preferred_over_pty() {
        let caps = AgentCapabilities {
            runtime_modes: vec![
                AgentRuntimeMode::InteractivePty,
                AgentRuntimeMode::OneShotStructured,
            ],
            ..AgentCapabilities::missing(AgentKind::Claude)
        };
        assert_eq!(
            caps.preferred_mode(),
            Some(AgentRuntimeMode::OneShotStructured)
        );
    }

    #[test]
    fn persistent_protocol_beats_pty_when_one_shot_is_unavailable() {
        let caps = AgentCapabilities {
            runtime_modes: vec![
                AgentRuntimeMode::InteractivePty,
                AgentRuntimeMode::PersistentProtocol,
            ],
            ..AgentCapabilities::missing(AgentKind::Grok)
        };
        assert_eq!(
            caps.preferred_mode(),
            Some(AgentRuntimeMode::PersistentProtocol)
        );
    }

    #[test]
    fn binary_names_match_the_installed_clis() {
        assert_eq!(binary_name(AgentKind::Codex), "codex");
        assert_eq!(binary_name(AgentKind::Claude), "claude");
        assert_eq!(binary_name(AgentKind::Grok), "grok");
        assert_eq!(binary_name(AgentKind::Gemini), "gemini");
    }
}
