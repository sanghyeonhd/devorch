//! The adapter registry.
//!
//! Mission code asks for "an adapter for this agent" and gets back something
//! that speaks the normalized contract. Nothing above this crate knows that
//! Codex has items and Gemini has tool_use blocks.

use devorch_agent::adapter::AgentAdapter;
use devorch_protocol::AgentKind;

pub use devorch_agent_claude::ClaudeAdapter;
pub use devorch_agent_codex::CodexAdapter;
pub use devorch_agent_gemini::GeminiAdapter;
pub use devorch_agent_grok::GrokAdapter;

/// Build the adapter for `agent`.
///
/// [`AgentKind::Fake`] has no adapter here — the deterministic test agent lives
/// in the mission crate, where the fakes it needs to cooperate with also live.
pub fn adapter_for(agent: AgentKind) -> Option<Box<dyn AgentAdapter>> {
    match agent {
        AgentKind::Codex => Some(Box::new(CodexAdapter::new())),
        AgentKind::Claude => Some(Box::new(ClaudeAdapter::new())),
        AgentKind::Grok => Some(Box::new(GrokAdapter::new())),
        AgentKind::Gemini => Some(Box::new(GeminiAdapter::new())),
        AgentKind::Fake => None,
    }
}

/// Every agent that has a real adapter.
pub fn all() -> Vec<AgentKind> {
    AgentKind::ALL.to_vec()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn every_first_class_agent_has_an_adapter_that_identifies_itself() {
        for agent in AgentKind::ALL {
            let adapter = adapter_for(agent).expect("adapter exists");
            assert_eq!(adapter.kind(), agent);
        }
    }

    #[test]
    fn the_fake_agent_has_no_vendor_adapter() {
        assert!(adapter_for(AgentKind::Fake).is_none());
    }
}
