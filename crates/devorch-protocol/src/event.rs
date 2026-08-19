//! Canonical normalized agent events.
//!
//! Every vendor adapter (Codex, Claude, Grok, Gemini) maps its own wire format
//! onto [`AgentEvent`]. Adapters must also retain the original payload in
//! [`EventEnvelope::vendor_payload`] so a mapping bug is diagnosable after the
//! fact without re-running the agent.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use crate::id::{EventId, MissionId, SessionId, TaskId, WorkspaceId};

/// Which agent product produced an event.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AgentKind {
    Codex,
    Claude,
    Grok,
    Gemini,
    /// A deterministic in-process agent used by tests and dry runs.
    Fake,
}

impl AgentKind {
    /// All agents Devorch treats as first-class executors.
    pub const ALL: [AgentKind; 4] = [
        AgentKind::Codex,
        AgentKind::Claude,
        AgentKind::Grok,
        AgentKind::Gemini,
    ];

    /// Stable lowercase identifier used in config, CLI flags and the database.
    pub fn as_str(&self) -> &'static str {
        match self {
            AgentKind::Codex => "codex",
            AgentKind::Claude => "claude",
            AgentKind::Grok => "grok",
            AgentKind::Gemini => "gemini",
            AgentKind::Fake => "fake",
        }
    }
}

impl std::fmt::Display for AgentKind {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

impl std::str::FromStr for AgentKind {
    type Err = UnknownAgent;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s.trim().to_ascii_lowercase().as_str() {
            "codex" => Ok(AgentKind::Codex),
            "claude" => Ok(AgentKind::Claude),
            "grok" => Ok(AgentKind::Grok),
            "gemini" => Ok(AgentKind::Gemini),
            "fake" => Ok(AgentKind::Fake),
            _ => Err(UnknownAgent(s.to_string())),
        }
    }
}

/// Error returned when a string does not name a known agent.
#[derive(Debug, thiserror::Error)]
#[error("unknown agent: {0}")]
pub struct UnknownAgent(pub String);

/// How Devorch drives an agent process.
///
/// The ordering here is the preference order: a structured one-shot invocation
/// leaves no idle process behind and is therefore the default.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AgentRuntimeMode {
    OneShotStructured,
    PersistentProtocol,
    InteractivePty,
}

/// The normalized event vocabulary shared by every adapter.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum AgentEvent {
    SessionStarted(SessionStarted),
    TurnStarted(TurnStarted),
    TextDelta(TextDelta),
    PlanUpdated(PlanUpdated),

    ToolStarted(ToolStarted),
    ToolUpdated(ToolUpdated),
    ToolCompleted(ToolCompleted),

    FileChanged(FileChanged),
    CommandStarted(CommandStarted),
    CommandCompleted(CommandCompleted),

    PermissionRequired(PermissionRequired),
    Usage(Usage),

    TurnCompleted(TurnCompleted),
    Completed(Completed),
    Failed(Failed),
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct SessionStarted {
    pub agent: AgentKind,
    pub mode: AgentRuntimeMode,
    /// The vendor's own session/thread identifier, when it exposes one.
    pub vendor_session_id: Option<String>,
    pub model: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct TurnStarted {
    pub turn: u32,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct TextDelta {
    pub text: String,
    /// True when the vendor labels this as internal reasoning rather than an
    /// answer. Devorch never persists reasoning text into shared surfaces.
    #[serde(default)]
    pub reasoning: bool,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct PlanUpdated {
    pub steps: Vec<PlanStep>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct PlanStep {
    pub description: String,
    pub status: PlanStepStatus,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum PlanStepStatus {
    Pending,
    InProgress,
    Completed,
    Failed,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ToolStarted {
    pub call_id: String,
    pub name: String,
    pub arguments: Option<serde_json::Value>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ToolUpdated {
    pub call_id: String,
    pub detail: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ToolCompleted {
    pub call_id: String,
    pub name: String,
    pub success: bool,
    pub output: Option<String>,
}

/// Reported by the agent. This is a claim, not evidence: the authoritative
/// change set always comes from a `ChangeInventory` computed by Devorch.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct FileChanged {
    pub path: String,
    pub kind: FileChangeKind,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum FileChangeKind {
    Added,
    Modified,
    Deleted,
    Renamed,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct CommandStarted {
    pub call_id: String,
    pub command: String,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct CommandCompleted {
    pub call_id: String,
    pub exit_code: Option<i32>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct PermissionRequired {
    pub call_id: String,
    pub action: String,
    pub detail: Option<String>,
}

/// Token/cost accounting. Cost is `None` when the vendor does not report it —
/// unknown cost is never recorded as zero.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Usage {
    pub input_tokens: Option<u64>,
    pub output_tokens: Option<u64>,
    pub cached_input_tokens: Option<u64>,
    pub cost_usd: Option<f64>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct TurnCompleted {
    pub turn: u32,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Completed {
    /// The agent's final message, if it produced one.
    pub summary: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Failed {
    pub message: String,
    pub class: FailureClass,
}

/// Bounded failure taxonomy. Every failure must map to one of these so recovery
/// is a lookup rather than an open-ended judgement call.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum FailureClass {
    AgentNotInstalled,
    AgentAuth,
    AgentProtocol,
    ModelRateLimit,
    AgentCrash,
    ProcessHang,
    ToolFailure,
    BuildFailure,
    TestFailure,
    WorktreeConflict,
    UntrackedFilePolicy,
    EnvironmentFailure,
    NetworkFailure,
    PermissionDenied,
    PolicyBlocked,
    VerificationFailed,
    UserAborted,
    BudgetExceeded,
    Unknown,
}

/// A normalized event plus its routing context and original vendor payload.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct EventEnvelope {
    pub id: EventId,
    /// Monotonic per-session sequence number. Adapters must not skip values.
    pub seq: u64,
    pub timestamp: DateTime<Utc>,

    pub mission_id: Option<MissionId>,
    pub task_id: Option<TaskId>,
    pub workspace_id: Option<WorkspaceId>,
    pub session_id: SessionId,

    pub event: AgentEvent,
    /// The untouched vendor payload this event was normalized from.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub vendor_payload: Option<serde_json::Value>,
}

impl EventEnvelope {
    /// Build an envelope for `event` within `session`, stamped now.
    pub fn new(session_id: SessionId, seq: u64, event: AgentEvent) -> Self {
        Self {
            id: EventId::new(),
            seq,
            timestamp: Utc::now(),
            mission_id: None,
            task_id: None,
            workspace_id: None,
            session_id,
            event,
            vendor_payload: None,
        }
    }

    /// Attach the original vendor payload this event was derived from.
    pub fn with_vendor_payload(mut self, payload: serde_json::Value) -> Self {
        self.vendor_payload = Some(payload);
        self
    }

    /// Attach the workspace this event belongs to.
    pub fn with_workspace(mut self, workspace_id: WorkspaceId) -> Self {
        self.workspace_id = Some(workspace_id);
        self
    }

    /// True when this event ends the session, successfully or not.
    pub fn is_terminal(&self) -> bool {
        matches!(self.event, AgentEvent::Completed(_) | AgentEvent::Failed(_))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn agent_kind_round_trips() {
        for kind in AgentKind::ALL {
            let parsed: AgentKind = kind.as_str().parse().expect("parse");
            assert_eq!(kind, parsed);
        }
    }

    #[test]
    fn agent_kind_parsing_is_case_insensitive_and_trimmed() {
        assert_eq!(" Codex ".parse::<AgentKind>().unwrap(), AgentKind::Codex);
        assert!("copilot".parse::<AgentKind>().is_err());
    }

    #[test]
    fn envelope_serializes_with_a_tagged_event() {
        let env = EventEnvelope::new(
            SessionId::new(),
            0,
            AgentEvent::TextDelta(TextDelta {
                text: "hello".into(),
                reasoning: false,
            }),
        );
        let json = serde_json::to_value(&env).expect("serialize");
        assert_eq!(json["event"]["type"], "text_delta");
        assert_eq!(json["event"]["text"], "hello");
        // vendor_payload is omitted rather than serialized as null.
        assert!(json.get("vendor_payload").is_none());
    }

    #[test]
    fn envelope_round_trips_through_json() {
        let env = EventEnvelope::new(
            SessionId::new(),
            7,
            AgentEvent::Failed(Failed {
                message: "boom".into(),
                class: FailureClass::TestFailure,
            }),
        )
        .with_vendor_payload(serde_json::json!({"raw": "boom"}));
        let json = serde_json::to_string(&env).expect("serialize");
        let back: EventEnvelope = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(env, back);
        assert!(back.is_terminal());
    }

    #[test]
    fn text_delta_is_not_terminal() {
        let env = EventEnvelope::new(
            SessionId::new(),
            0,
            AgentEvent::TextDelta(TextDelta {
                text: "x".into(),
                reasoning: true,
            }),
        );
        assert!(!env.is_terminal());
    }
}
