//! The adapter contract.
//!
//! Each vendor speaks its own wire format. An adapter's whole job is to turn
//! that format into the shared [`AgentEvent`] vocabulary without letting vendor
//! shapes leak upward — mission code must never branch on which agent produced
//! an event.
//!
//! Normalization is separated from execution on purpose. `normalize` is a pure
//! function of a line plus the adapter's own bookkeeping, so every adapter can
//! be tested against recorded fixtures with no process, no network and no
//! subscription.

use std::path::PathBuf;
use std::time::Duration;

use devorch_process::ProcessSpec;
use devorch_protocol::{AgentEvent, AgentKind, AgentRuntimeMode};

/// What an agent is being asked to do.
#[derive(Debug, Clone)]
pub struct ExecuteRequest {
    /// The instruction given to the agent.
    pub prompt: String,
    /// Working directory — always a candidate's own worktree.
    pub workdir: PathBuf,
    /// Model override. `None` uses whatever the CLI defaults to.
    pub model: Option<String>,
    /// Wall-clock ceiling for the run.
    pub timeout: Option<Duration>,
}

impl ExecuteRequest {
    /// A request to run `prompt` inside `workdir`.
    pub fn new(prompt: impl Into<String>, workdir: impl Into<PathBuf>) -> Self {
        Self {
            prompt: prompt.into(),
            workdir: workdir.into(),
            model: None,
            timeout: None,
        }
    }

    /// Pin a specific model.
    pub fn model(mut self, model: impl Into<String>) -> Self {
        self.model = Some(model.into());
        self
    }

    /// Abandon the run after `timeout`.
    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.timeout = Some(timeout);
        self
    }
}

/// Turns one vendor's stream into normalized events.
///
/// Implementations hold whatever bookkeeping the vendor format requires — tool
/// call names that only appear in a start event, sequence state, partial
/// messages — which is why `normalize` takes `&mut self`.
pub trait AgentAdapter: Send {
    /// Which agent this adapter speaks for.
    fn kind(&self) -> AgentKind;

    /// The runtime mode this adapter drives.
    fn mode(&self) -> AgentRuntimeMode {
        AgentRuntimeMode::OneShotStructured
    }

    /// Build the command that runs `request`.
    fn build_spec(&self, request: &ExecuteRequest) -> ProcessSpec;

    /// Normalize one line of vendor output.
    ///
    /// Returns zero or more events: a line may carry several (a message with
    /// both text and a tool call), or none (a keepalive, a blank line, or a
    /// vendor event Devorch has no use for). An unparseable line yields no
    /// events rather than failing the run — a stray log line from a CLI must
    /// not abort a mission that is otherwise working.
    fn normalize(&mut self, line: &str) -> Vec<AgentEvent>;

    /// Events implied by the process exiting, given its status.
    ///
    /// Adapters that already emit a terminal event from the stream return
    /// nothing here. This exists so a crash with no final event still produces
    /// a `Failed` rather than a session that silently stops.
    fn finalize(&mut self, exit_code: Option<i32>) -> Vec<AgentEvent> {
        let _ = exit_code;
        Vec::new()
    }
}

/// Whether a terminal event has been seen, shared by every adapter.
///
/// Vendors disagree about whether a crashed run emits a final event, so each
/// adapter tracks this and reports a synthetic failure in `finalize` when the
/// stream ended without one.
#[derive(Debug, Default)]
pub struct TerminalTracker {
    seen: bool,
}

impl TerminalTracker {
    /// Record that the stream produced its own terminal event.
    pub fn mark(&mut self) {
        self.seen = true;
    }

    /// Whether the stream already ended itself.
    pub fn seen(&self) -> bool {
        self.seen
    }

    /// The event a process exit implies, if the stream did not end itself.
    ///
    /// A non-zero exit with no failure event is a crash, and a zero exit with no
    /// completion event is still not a success Devorch can attest to — both are
    /// reported rather than assumed. "The process disappeared" never converts
    /// directly into success.
    pub fn finalize(&mut self, exit_code: Option<i32>) -> Vec<AgentEvent> {
        if self.seen {
            return Vec::new();
        }
        self.seen = true;

        use devorch_protocol::event::Failed;
        use devorch_protocol::FailureClass;

        vec![AgentEvent::Failed(Failed {
            message: match exit_code {
                Some(code) => format!("agent exited with status {code} without reporting a result"),
                None => "agent was terminated by a signal without reporting a result".to_string(),
            },
            class: match exit_code {
                Some(0) => FailureClass::AgentProtocol,
                _ => FailureClass::AgentCrash,
            },
        })]
    }
}

/// Map a vendor error message onto the bounded failure taxonomy.
///
/// Every vendor words its errors differently, but the categories a mission has
/// to react to are the same: a rate limit is worth retrying later, an auth
/// failure is not worth retrying at all, and a protocol error means the adapter
/// or the CLI changed. Keeping the mapping in one place is what stops four
/// adapters from disagreeing about what "quota" means.
pub fn classify_failure(message: &str) -> devorch_protocol::FailureClass {
    use devorch_protocol::FailureClass;

    let text = message.to_ascii_lowercase();

    // Usage limits come first: "usage limit" messages often also mention an
    // upgrade link, which would otherwise read as an auth problem.
    if text.contains("rate limit")
        || text.contains("usage limit")
        || text.contains("quota")
        || text.contains("429")
        || text.contains("too many requests")
        || text.contains("try again at")
    {
        return FailureClass::ModelRateLimit;
    }
    if text.contains("unauthor")
        || text.contains("api key")
        || text.contains("not logged in")
        || text.contains("authentication")
        || text.contains("credential")
        || text.contains("sign in")
    {
        return FailureClass::AgentAuth;
    }
    if text.contains("permission") || text.contains("denied") || text.contains("sandbox") {
        return FailureClass::PermissionDenied;
    }
    if text.contains("network")
        || text.contains("connection")
        || text.contains("timed out")
        || text.contains("dns")
    {
        return FailureClass::NetworkFailure;
    }
    if text.contains("test") && text.contains("fail") {
        return FailureClass::TestFailure;
    }
    FailureClass::AgentProtocol
}

/// Parse a line as JSON, ignoring blanks and non-JSON noise.
///
/// CLIs interleave human-readable notices with their structured stream. Those
/// lines are skipped, not treated as protocol errors.
pub fn parse_json_line(line: &str) -> Option<serde_json::Value> {
    let trimmed = line.trim();
    if trimmed.is_empty() || !trimmed.starts_with('{') {
        return None;
    }
    serde_json::from_str(trimmed).ok()
}

#[cfg(test)]
mod tests {
    use super::*;
    use devorch_protocol::FailureClass;

    #[test]
    fn blank_and_non_json_lines_are_skipped() {
        assert!(parse_json_line("").is_none());
        assert!(parse_json_line("   ").is_none());
        assert!(parse_json_line("warning: update available").is_none());
        assert!(parse_json_line("{ not json }").is_none());
        assert!(parse_json_line(r#"{"type":"ok"}"#).is_some());
    }

    #[test]
    fn a_real_usage_limit_message_is_classified_as_a_rate_limit() {
        // Captured verbatim from a live `codex exec --json` run.
        let message = "You've hit your usage limit. Upgrade to Pro \
                       (https://chatgpt.com/explore/pro), visit \
                       https://chatgpt.com/codex/settings/usage to purchase more credits or \
                       try again at Aug 20th, 2026 7:34 PM.";
        assert_eq!(classify_failure(message), FailureClass::ModelRateLimit);
    }

    #[test]
    fn failure_classes_are_distinguished_by_wording() {
        assert_eq!(
            classify_failure("Quota exceeded for the current project"),
            FailureClass::ModelRateLimit
        );
        assert_eq!(
            classify_failure("Unauthorized: please sign in"),
            FailureClass::AgentAuth
        );
        assert_eq!(
            classify_failure("sandbox denied write outside the workspace"),
            FailureClass::PermissionDenied
        );
        assert_eq!(
            classify_failure("connection reset"),
            FailureClass::NetworkFailure
        );
        // Anything unrecognised is a protocol problem, not a guess.
        assert_eq!(
            classify_failure("something entirely new"),
            FailureClass::AgentProtocol
        );
    }

    #[test]
    fn a_stream_that_ended_itself_needs_no_synthetic_event() {
        let mut tracker = TerminalTracker::default();
        tracker.mark();
        assert!(tracker.finalize(Some(0)).is_empty());
    }

    #[test]
    fn a_crash_without_a_final_event_becomes_a_failure() {
        let mut tracker = TerminalTracker::default();
        let events = tracker.finalize(Some(1));
        match &events[..] {
            [AgentEvent::Failed(f)] => assert_eq!(f.class, FailureClass::AgentCrash),
            other => panic!("expected a crash failure, got {other:?}"),
        }
    }

    #[test]
    fn a_clean_exit_without_a_result_is_a_protocol_failure_not_a_success() {
        let mut tracker = TerminalTracker::default();
        let events = tracker.finalize(Some(0));
        match &events[..] {
            [AgentEvent::Failed(f)] => assert_eq!(f.class, FailureClass::AgentProtocol),
            other => panic!("expected a protocol failure, got {other:?}"),
        }
    }

    #[test]
    fn a_signal_kill_is_reported_as_a_crash() {
        let mut tracker = TerminalTracker::default();
        let events = tracker.finalize(None);
        match &events[..] {
            [AgentEvent::Failed(f)] => {
                assert_eq!(f.class, FailureClass::AgentCrash);
                assert!(f.message.contains("signal"));
            }
            other => panic!("expected a crash failure, got {other:?}"),
        }
    }
}
