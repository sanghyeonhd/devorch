//! Claude Code adapter.
//!
//! Drives `claude -p --output-format stream-json`, whose lines wrap Anthropic
//! Messages content blocks: a `system`/`init` line, `assistant` lines carrying
//! `text` and `tool_use` blocks, `user` lines carrying `tool_result` blocks, and
//! a final `result` line.
//!
//! The validation report made a specific demand of this adapter: do not assume a
//! fixed forever-valid wire schema. So every field here is read defensively —
//! a renamed or missing field degrades one event rather than failing the run —
//! and the runtime mode comes from capability discovery, not from this file.

use std::collections::HashMap;

use devorch_agent::adapter::{parse_json_line, AgentAdapter, ExecuteRequest, TerminalTracker};
use devorch_process::ProcessSpec;
use devorch_protocol::event::{
    Completed, Failed, SessionStarted, TextDelta, ToolCompleted, ToolStarted, Usage,
};
use devorch_protocol::{AgentEvent, AgentKind, AgentRuntimeMode, FailureClass};
use serde_json::Value;

/// Normalizes the `claude -p --output-format stream-json` stream.
#[derive(Debug, Default)]
pub struct ClaudeAdapter {
    terminal: TerminalTracker,
    /// tool_use id → tool name. Results arrive on a later line that carries the
    /// id but not the name.
    tool_names: HashMap<String, String>,
}

impl ClaudeAdapter {
    /// A fresh adapter for one Claude session.
    pub fn new() -> Self {
        Self::default()
    }
}

impl AgentAdapter for ClaudeAdapter {
    fn kind(&self) -> AgentKind {
        AgentKind::Claude
    }

    fn mode(&self) -> AgentRuntimeMode {
        AgentRuntimeMode::OneShotStructured
    }

    fn build_spec(&self, request: &ExecuteRequest) -> ProcessSpec {
        // --verbose is required for stream-json to emit intermediate events
        // rather than only the final result.
        let mut spec = ProcessSpec::new("claude")
            .arg("-p")
            .arg("--output-format")
            .arg("stream-json")
            .arg("--verbose");

        if let Some(model) = &request.model {
            spec = spec.arg("--model").arg(model);
        }

        spec = spec.arg(&request.prompt).cwd(&request.workdir);

        if let Some(timeout) = request.timeout {
            spec = spec.timeout(timeout);
        }
        spec
    }

    fn normalize(&mut self, line: &str) -> Vec<AgentEvent> {
        let Some(value) = parse_json_line(line) else {
            return Vec::new();
        };
        let Some(kind) = value.get("type").and_then(Value::as_str) else {
            return Vec::new();
        };

        match kind {
            "system" => {
                if value.get("subtype").and_then(Value::as_str) != Some("init") {
                    return Vec::new();
                }
                vec![AgentEvent::SessionStarted(SessionStarted {
                    agent: AgentKind::Claude,
                    mode: AgentRuntimeMode::OneShotStructured,
                    vendor_session_id: str_field(&value, "session_id"),
                    model: str_field(&value, "model"),
                })]
            }

            "assistant" => self.normalize_assistant(&value),
            "user" => self.normalize_user(&value),

            "result" => {
                self.terminal.mark();
                let mut events = Vec::new();

                if let Some(usage) = value.get("usage") {
                    events.push(usage_event(usage, value.get("total_cost_usd")));
                }

                let is_error = value
                    .get("is_error")
                    .and_then(Value::as_bool)
                    .unwrap_or(false)
                    || value.get("subtype").and_then(Value::as_str) == Some("error");

                if is_error {
                    events.push(AgentEvent::Failed(Failed {
                        message: str_field(&value, "result")
                            .or_else(|| str_field(&value, "error"))
                            .unwrap_or_else(|| "claude reported an error".into()),
                        class: match value.get("subtype").and_then(Value::as_str) {
                            Some("error_max_turns") => FailureClass::BudgetExceeded,
                            _ => FailureClass::ToolFailure,
                        },
                    }));
                } else {
                    events.push(AgentEvent::Completed(Completed {
                        summary: str_field(&value, "result"),
                    }));
                }
                events
            }

            _ => Vec::new(),
        }
    }

    fn finalize(&mut self, exit_code: Option<i32>) -> Vec<AgentEvent> {
        self.terminal.finalize(exit_code)
    }
}

impl ClaudeAdapter {
    /// Assistant lines carry text and tool_use content blocks.
    fn normalize_assistant(&mut self, value: &Value) -> Vec<AgentEvent> {
        let mut events = Vec::new();

        for block in content_blocks(value) {
            match block.get("type").and_then(Value::as_str) {
                Some("text") => {
                    if let Some(text) = str_field(block, "text") {
                        events.push(AgentEvent::TextDelta(TextDelta {
                            text,
                            reasoning: false,
                        }));
                    }
                }
                Some("thinking") => {
                    if let Some(text) =
                        str_field(block, "thinking").or_else(|| str_field(block, "text"))
                    {
                        events.push(AgentEvent::TextDelta(TextDelta {
                            text,
                            reasoning: true,
                        }));
                    }
                }
                Some("tool_use") => {
                    let call_id = str_field(block, "id").unwrap_or_else(|| "tool".into());
                    let name = str_field(block, "name").unwrap_or_else(|| "tool".into());
                    self.tool_names.insert(call_id.clone(), name.clone());
                    events.push(AgentEvent::ToolStarted(ToolStarted {
                        call_id,
                        name,
                        arguments: block.get("input").cloned(),
                    }));
                }
                _ => {}
            }
        }

        if let Some(usage) = value.get("message").and_then(|m| m.get("usage")) {
            events.push(usage_event(usage, None));
        }
        events
    }

    /// User lines carry tool_result blocks referring back to a tool_use id.
    fn normalize_user(&mut self, value: &Value) -> Vec<AgentEvent> {
        content_blocks(value)
            .into_iter()
            .filter(|block| block.get("type").and_then(Value::as_str) == Some("tool_result"))
            .map(|block| {
                let call_id = str_field(block, "tool_use_id").unwrap_or_else(|| "tool".into());
                let name = self
                    .tool_names
                    .get(&call_id)
                    .cloned()
                    .unwrap_or_else(|| "tool".into());
                AgentEvent::ToolCompleted(ToolCompleted {
                    call_id,
                    name,
                    success: !block
                        .get("is_error")
                        .and_then(Value::as_bool)
                        .unwrap_or(false),
                    output: tool_result_text(block),
                })
            })
            .collect()
    }
}

/// The content blocks of a message line, whatever nesting the version uses.
fn content_blocks(value: &Value) -> Vec<&Value> {
    value
        .get("message")
        .and_then(|m| m.get("content"))
        .or_else(|| value.get("content"))
        .and_then(Value::as_array)
        .map(|blocks| blocks.iter().collect())
        .unwrap_or_default()
}

/// A tool result's content, which may be a bare string or a block array.
fn tool_result_text(block: &Value) -> Option<String> {
    match block.get("content") {
        Some(Value::String(text)) => Some(text.clone()),
        Some(Value::Array(parts)) => {
            let joined: Vec<_> = parts.iter().filter_map(|p| str_field(p, "text")).collect();
            (!joined.is_empty()).then(|| joined.join("\n"))
        }
        _ => None,
    }
}

/// Read a string field, treating an empty string as absent.
fn str_field(value: &Value, key: &str) -> Option<String> {
    value
        .get(key)
        .and_then(Value::as_str)
        .filter(|s| !s.is_empty())
        .map(str::to_string)
}

/// Map Claude token accounting, including cost when the CLI reports it.
fn usage_event(usage: &Value, cost: Option<&Value>) -> AgentEvent {
    let num = |key: &str| usage.get(key).and_then(Value::as_u64);
    AgentEvent::Usage(Usage {
        input_tokens: num("input_tokens"),
        output_tokens: num("output_tokens"),
        cached_input_tokens: num("cache_read_input_tokens").or_else(|| num("cached_input_tokens")),
        cost_usd: cost.and_then(Value::as_f64),
    })
}
