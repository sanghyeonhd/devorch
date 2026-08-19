//! Runtime capability discovery.
//!
//! Each agent is asked what it is (`--version`) and what it accepts (`--help`),
//! and the answer is parsed into [`AgentCapabilities`]. Nothing here asserts a
//! flag exists because a document said so — if the installed binary stops
//! advertising `--output-format`, the capability disappears and the router stops
//! choosing that mode, rather than the adapter failing at mission time.

use devorch_process::which;
use devorch_protocol::{AgentKind, AgentRuntimeMode};

use crate::{binary_name, probe, AgentCapabilities};

/// Inspect every first-class agent.
pub async fn inspect_all() -> Vec<AgentCapabilities> {
    let mut out = Vec::with_capacity(AgentKind::ALL.len());
    for agent in AgentKind::ALL {
        out.push(inspect(agent).await);
    }
    out
}

/// Inspect one agent. An uninstalled agent yields a `missing` record rather
/// than an error, because "not installed" is a normal state the UI displays.
pub async fn inspect(agent: AgentKind) -> AgentCapabilities {
    let binary = binary_name(agent);

    let Some(path) = which(binary).await else {
        return AgentCapabilities::missing(agent);
    };

    let version = probe(binary, &["--version"])
        .await
        .map(|v| {
            v.trim()
                .lines()
                .next()
                .unwrap_or_default()
                .trim()
                .to_string()
        })
        .filter(|v| !v.is_empty());

    let mut help = probe(binary, &["--help"]).await.unwrap_or_default();
    // Codex puts its headless surface behind a subcommand, so the top-level
    // help alone understates what it can do.
    if agent == AgentKind::Codex {
        if let Some(exec_help) = probe(binary, &["exec", "--help"]).await {
            help.push('\n');
            help.push_str(&exec_help);
        }
    }
    let help = help.to_ascii_lowercase();

    let structured_formats = detect_formats(agent, &help);
    let protocols = detect_protocols(&help);

    let mut runtime_modes = Vec::new();
    if !structured_formats.is_empty() {
        runtime_modes.push(AgentRuntimeMode::OneShotStructured);
    }
    if !protocols.is_empty() {
        runtime_modes.push(AgentRuntimeMode::PersistentProtocol);
    }
    // Every one of these CLIs starts an interactive session with no arguments,
    // so PTY is always available as the compatibility fallback.
    runtime_modes.push(AgentRuntimeMode::InteractivePty);

    AgentCapabilities {
        id: agent,
        installed: true,
        path: Some(path),
        version,
        runtime_modes,
        structured_formats,
        resume: help.contains("--resume") || help.contains("resume"),
        tool_permissions: detect_tool_permissions(&help),
        protocols,
    }
}

/// Structured output formats advertised by the installed binary.
fn detect_formats(agent: AgentKind, help: &str) -> Vec<String> {
    let mut formats = Vec::new();

    // Codex signals structured exec output with a bare `--json` rather than a
    // format enum.
    if agent == AgentKind::Codex && help.contains("--json") {
        formats.push("json".to_string());
    }

    if help.contains("--output-format") {
        for candidate in [
            "stream-json",
            "streaming-messages-json",
            "streaming-json",
            "json",
        ] {
            if help.contains(candidate) && !formats.iter().any(|f| f == candidate) {
                formats.push(candidate.to_string());
            }
        }
    }

    formats
}

/// Whether the installed binary lets Devorch constrain what the agent may do.
///
/// Each vendor spells this differently — an allow/deny tool list, a permission
/// mode, an approval policy, a sandbox policy — so all four spellings count.
/// Devorch still owns the final policy decision; this only records whether the
/// vendor surface exists to push a constraint down into.
fn detect_tool_permissions(help: &str) -> bool {
    [
        "--allowed-tools",
        "--allowedtools",
        "--disallowed-tools",
        "--permission-mode",
        "--ask-for-approval",
        "--sandbox",
        "--allow-",
        "--deny-",
    ]
    .iter()
    .any(|marker| help.contains(marker))
}

/// Persistent protocol surfaces advertised by the installed binary.
fn detect_protocols(help: &str) -> Vec<String> {
    let mut protocols = Vec::new();
    for (marker, name) in [
        ("acp", "acp"),
        ("a2a", "a2a"),
        ("app-server", "app-server"),
        ("mcp", "mcp"),
    ] {
        if help.contains(marker) {
            protocols.push(name.to_string());
        }
    }
    protocols
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn codex_json_flag_is_recognised_without_an_output_format_enum() {
        let help = "usage: codex exec [options]\n      --json  emit jsonl events";
        assert_eq!(detect_formats(AgentKind::Codex, help), vec!["json"]);
    }

    #[test]
    fn gemini_output_format_choices_are_parsed() {
        let help = r#"-o, --output-format  the format of the cli output. [choices: "text", "json", "stream-json"]"#;
        let formats = detect_formats(AgentKind::Gemini, help);
        assert!(formats.contains(&"stream-json".to_string()));
        assert!(formats.contains(&"json".to_string()));
    }

    #[test]
    fn grok_streaming_variants_are_parsed() {
        let help = "--output-format <output_format>\n  - streaming-json: ndjson of acp session updates\n  - streaming-messages-json: ndjson in the anthropic messages wire format";
        let formats = detect_formats(AgentKind::Grok, help);
        assert!(formats.contains(&"streaming-json".to_string()));
        assert!(formats.contains(&"streaming-messages-json".to_string()));
    }

    #[test]
    fn an_agent_with_no_structured_flags_reports_none() {
        assert!(detect_formats(AgentKind::Claude, "usage: claude").is_empty());
        // Absent --json, Codex must not be credited with structured output.
        assert!(detect_formats(AgentKind::Codex, "usage: codex").is_empty());
    }

    #[test]
    fn permission_surfaces_are_detected_across_vendor_spellings() {
        // Claude spells it as a tool allow-list, Codex as an approval and
        // sandbox policy. Both mean Devorch can constrain the agent.
        assert!(detect_tool_permissions("--allowed-tools <tools...>"));
        assert!(detect_tool_permissions(
            "-a, --ask-for-approval <approval_policy>"
        ));
        assert!(detect_tool_permissions("-s, --sandbox <sandbox_mode>"));
        assert!(!detect_tool_permissions("usage: agent [options]"));
    }

    #[test]
    fn protocol_surfaces_are_detected_from_help_text() {
        assert_eq!(detect_protocols("run grok agent over acp"), vec!["acp"]);
        assert!(detect_protocols("experimental a2a server").contains(&"a2a".to_string()));
        assert!(detect_protocols("plain help text").is_empty());
    }

    #[tokio::test]
    async fn an_uninstalled_agent_is_reported_as_missing_not_as_an_error() {
        // Fake maps to a binary that is deliberately never installed.
        let caps = inspect(AgentKind::Fake).await;
        assert!(!caps.installed);
        assert_eq!(caps.preferred_mode(), None);
    }

    #[tokio::test]
    async fn inspect_all_reports_every_first_class_agent() {
        let all = inspect_all().await;
        assert_eq!(all.len(), 4);
        for (caps, expected) in all.iter().zip(AgentKind::ALL) {
            assert_eq!(caps.id, expected);
            // Installed agents must expose at least the PTY fallback.
            if caps.installed {
                assert!(!caps.runtime_modes.is_empty());
                assert!(caps.preferred_mode().is_some());
            }
        }
    }
}
