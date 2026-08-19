//! Gemini CLI adapter.
//!
//! Drives `gemini -p --output-format stream-json`, whose event family is
//! `init`, `message`, `tool_use`, `tool_result`, `error`, `result`.
//!
//! The repository also ships an experimental A2A server. Because it is marked
//! experimental it cannot be the only integration, so this adapter always has
//! the stream-json path and A2A stays a later, optional persistent mode.

use std::collections::HashMap;

use devorch_agent::adapter::{
    classify_failure, parse_json_line, AgentAdapter, ExecuteRequest, TerminalTracker,
};
use devorch_process::ProcessSpec;
use devorch_protocol::event::{
    Completed, Failed, SessionStarted, TextDelta, ToolCompleted, ToolStarted, Usage,
};
use devorch_protocol::{AgentEvent, AgentKind, AgentRuntimeMode};
use serde_json::Value;

/// Normalizes the `gemini --output-format stream-json` stream.
#[derive(Debug, Default)]
pub struct GeminiAdapter {
    terminal: TerminalTracker,
    tool_names: HashMap<String, String>,
}

impl GeminiAdapter {
    /// A fresh adapter for one Gemini session.
    pub fn new() -> Self {
        Self::default()
    }
}

impl AgentAdapter for GeminiAdapter {
    fn kind(&self) -> AgentKind {
        AgentKind::Gemini
    }

    fn mode(&self) -> AgentRuntimeMode {
        AgentRuntimeMode::OneShotStructured
    }

    fn build_spec(&self, request: &ExecuteRequest) -> ProcessSpec {
        let mut spec = ProcessSpec::new("gemini")
            .arg("-p")
            .arg(&request.prompt)
            .arg("--output-format")
            .arg("stream-json");

        if let Some(model) = &request.model {
            spec = spec.arg("--model").arg(model);
        }

        spec = spec.cwd(&request.workdir);

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
            "init" => vec![AgentEvent::SessionStarted(SessionStarted {
                agent: AgentKind::Gemini,
                mode: AgentRuntimeMode::OneShotStructured,
                vendor_session_id: str_field(&value, "session_id"),
                model: str_field(&value, "model"),
            })],

            "message" => {
                // Only assistant output is surfaced; the echoed user turn is not
                // agent output.
                if matches!(value.get("role").and_then(Value::as_str), Some("user")) {
                    return Vec::new();
                }
                match content_text(&value) {
                    Some(text) => vec![AgentEvent::TextDelta(TextDelta {
                        text,
                        reasoning: matches!(
                            value.get("subtype").and_then(Value::as_str),
                            Some("thought")
                        ),
                    })],
                    None => Vec::new(),
                }
            }

            "tool_use" => {
                let call_id = call_id_of(&value);
                let name = str_field(&value, "name").unwrap_or_else(|| "tool".into());
                self.tool_names.insert(call_id.clone(), name.clone());
                vec![AgentEvent::ToolStarted(ToolStarted {
                    call_id,
                    name,
                    arguments: value
                        .get("input")
                        .or_else(|| value.get("args"))
                        .or_else(|| value.get("arguments"))
                        .cloned(),
                })]
            }

            "tool_result" => {
                let call_id = call_id_of(&value);
                let name = self
                    .tool_names
                    .get(&call_id)
                    .cloned()
                    .or_else(|| str_field(&value, "name"))
                    .unwrap_or_else(|| "tool".into());
                let error = value.get("error").filter(|e| !e.is_null());
                vec![AgentEvent::ToolCompleted(ToolCompleted {
                    call_id,
                    name,
                    success: error.is_none(),
                    output: str_field(&value, "output")
                        .or_else(|| str_field(&value, "result"))
                        .or_else(|| error.and_then(|e| e.as_str().map(str::to_string))),
                })]
            }

            "result" => {
                self.terminal.mark();
                let mut events = Vec::new();
                if let Some(stats) = value.get("stats").or_else(|| value.get("usage")) {
                    events.push(usage_event(stats));
                }
                events.push(AgentEvent::Completed(Completed {
                    summary: str_field(&value, "response")
                        .or_else(|| str_field(&value, "result"))
                        .or_else(|| str_field(&value, "text")),
                }));
                events
            }

            "error" => {
                self.terminal.mark();
                let message = str_field(&value, "message")
                    .or_else(|| value.get("error").and_then(|e| str_field(e, "message")))
                    .unwrap_or_else(|| "gemini reported an error".into());
                vec![AgentEvent::Failed(Failed {
                    class: classify_failure(&message),
                    message,
                })]
            }

            _ => Vec::new(),
        }
    }

    fn finalize(&mut self, exit_code: Option<i32>) -> Vec<AgentEvent> {
        self.terminal.finalize(exit_code)
    }
}

/// A message's text, whether it is a bare string or a parts array.
fn content_text(value: &Value) -> Option<String> {
    match value.get("content") {
        Some(Value::String(text)) if !text.is_empty() => Some(text.clone()),
        Some(Value::Array(parts)) => {
            let joined: Vec<_> = parts
                .iter()
                .filter_map(|p| match p {
                    Value::String(s) => Some(s.clone()),
                    other => str_field(other, "text"),
                })
                .collect();
            (!joined.is_empty()).then(|| joined.join(""))
        }
        _ => str_field(value, "text"),
    }
}

/// The tool call identifier, under whichever key this version uses.
fn call_id_of(value: &Value) -> String {
    str_field(value, "id")
        .or_else(|| str_field(value, "tool_use_id"))
        .or_else(|| str_field(value, "call_id"))
        .unwrap_or_else(|| "tool".into())
}

/// Read a string field, treating an empty string as absent.
fn str_field(value: &Value, key: &str) -> Option<String> {
    value
        .get(key)
        .and_then(Value::as_str)
        .filter(|s| !s.is_empty())
        .map(str::to_string)
}

/// Map Gemini token accounting.
fn usage_event(stats: &Value) -> AgentEvent {
    let num = |key: &str| stats.get(key).and_then(Value::as_u64);
    AgentEvent::Usage(Usage {
        input_tokens: num("input_tokens").or_else(|| num("promptTokenCount")),
        output_tokens: num("output_tokens").or_else(|| num("candidatesTokenCount")),
        cached_input_tokens: num("cached_input_tokens").or_else(|| num("cachedContentTokenCount")),
        cost_usd: stats.get("cost_usd").and_then(Value::as_f64),
    })
}
