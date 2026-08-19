//! Grok Build adapter.
//!
//! Drives `grok -p --output-format streaming-json`, which emits NDJSON of the
//! agent's native ACP session updates: `text`, `thought`, `tool_call`,
//! `tool_call_update`, `plan`, `usage`, `available_commands`, `end`, `error`.
//!
//! The field shapes here were corrected against a real captured session
//! (`testdata/agents/grok/live-simple.jsonl`). Grok puts chunk text in a bare
//! `data` string rather than a `text` field, and reports its session id and
//! cost only on the final `end` event — both of which the documented schema
//! alone would have got wrong.

use std::collections::HashMap;

use devorch_agent::adapter::{
    classify_failure, parse_json_line, AgentAdapter, ExecuteRequest, TerminalTracker,
};
use devorch_process::ProcessSpec;
use devorch_protocol::event::{
    Completed, Failed, PlanStep, PlanStepStatus, PlanUpdated, SessionStarted, TextDelta,
    ToolCompleted, ToolStarted, ToolUpdated, Usage,
};
use devorch_protocol::{AgentEvent, AgentKind, AgentRuntimeMode};
use serde_json::Value;

/// Normalizes the `grok --output-format streaming-json` stream.
#[derive(Debug, Default)]
pub struct GrokAdapter {
    terminal: TerminalTracker,
    started: bool,
    /// tool call id → name, for updates that omit the name.
    tool_names: HashMap<String, String>,
}

impl GrokAdapter {
    /// A fresh adapter for one Grok session.
    pub fn new() -> Self {
        Self::default()
    }
}

impl AgentAdapter for GrokAdapter {
    fn kind(&self) -> AgentKind {
        AgentKind::Grok
    }

    fn mode(&self) -> AgentRuntimeMode {
        AgentRuntimeMode::OneShotStructured
    }

    fn build_spec(&self, request: &ExecuteRequest) -> ProcessSpec {
        // Grok exposes always-approve/yolo switches. They are deliberately not
        // used: Devorch owns the permission decision, not the vendor.
        let mut spec = ProcessSpec::new("grok")
            .arg("-p")
            .arg(&request.prompt)
            .arg("--output-format")
            .arg("streaming-json");

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

        // Grok has no explicit session-start event, so the first meaningful
        // line opens the session.
        let mut events = Vec::new();
        if !self.started && kind != "error" {
            self.started = true;
            events.push(AgentEvent::SessionStarted(SessionStarted {
                agent: AgentKind::Grok,
                mode: AgentRuntimeMode::OneShotStructured,
                vendor_session_id: str_field(&value, "session_id")
                    .or_else(|| str_field(&value, "sessionId")),
                model: str_field(&value, "model"),
            }));
        }

        match kind {
            "text" | "agent_message_chunk" => {
                if let Some(text) = text_of(&value) {
                    events.push(AgentEvent::TextDelta(TextDelta {
                        text,
                        reasoning: false,
                    }));
                }
            }

            "thought" | "agent_thought_chunk" => {
                if let Some(text) = text_of(&value) {
                    events.push(AgentEvent::TextDelta(TextDelta {
                        text,
                        reasoning: true,
                    }));
                }
            }

            "tool_call" => {
                let call_id = call_id_of(&value);
                let name = str_field(&value, "name")
                    .or_else(|| str_field(&value, "title"))
                    .unwrap_or_else(|| "tool".into());
                self.tool_names.insert(call_id.clone(), name.clone());
                events.push(AgentEvent::ToolStarted(ToolStarted {
                    call_id,
                    name,
                    arguments: value
                        .get("arguments")
                        .or_else(|| value.get("input"))
                        .cloned(),
                }));
            }

            "tool_call_update" => {
                let call_id = call_id_of(&value);
                let status = value.get("status").and_then(Value::as_str).unwrap_or("");
                match status {
                    "completed" | "failed" | "error" => {
                        let name = self
                            .tool_names
                            .get(&call_id)
                            .cloned()
                            .unwrap_or_else(|| "tool".into());
                        events.push(AgentEvent::ToolCompleted(ToolCompleted {
                            call_id,
                            name,
                            success: status == "completed",
                            output: str_field(&value, "output")
                                .or_else(|| str_field(&value, "content")),
                        }));
                    }
                    _ => events.push(AgentEvent::ToolUpdated(ToolUpdated {
                        call_id,
                        detail: str_field(&value, "content")
                            .or_else(|| str_field(&value, "status")),
                    })),
                }
            }

            "plan" => {
                if let Some(entries) = value
                    .get("entries")
                    .or_else(|| value.get("steps"))
                    .and_then(Value::as_array)
                {
                    events.push(AgentEvent::PlanUpdated(PlanUpdated {
                        steps: entries
                            .iter()
                            .filter_map(|entry| {
                                Some(PlanStep {
                                    description: str_field(entry, "content")
                                        .or_else(|| str_field(entry, "title"))?,
                                    status: match entry.get("status").and_then(Value::as_str) {
                                        Some("completed") => PlanStepStatus::Completed,
                                        Some("in_progress") => PlanStepStatus::InProgress,
                                        Some("failed") => PlanStepStatus::Failed,
                                        _ => PlanStepStatus::Pending,
                                    },
                                })
                            })
                            .collect(),
                    }));
                }
            }

            "usage" => events.push(usage_event(&value)),

            "end" => {
                self.terminal.mark();
                // `end` carries the authoritative usage and the only cost Grok
                // reports, so it is emitted even though a `usage` event may
                // already have appeared mid-stream.
                if value.get("usage").is_some() || value.get("total_cost_usd").is_some() {
                    events.push(usage_event(&value));
                }
                events.push(AgentEvent::Completed(Completed {
                    summary: str_field(&value, "result")
                        .or_else(|| str_field(&value, "text"))
                        .or_else(|| str_field(&value, "stopReason")),
                }));
            }

            "error" => {
                self.terminal.mark();
                let message = str_field(&value, "message")
                    .or_else(|| str_field(&value, "error"))
                    .or_else(|| str_field(&value, "data"))
                    .unwrap_or_else(|| "grok reported an error".into());
                events.push(AgentEvent::Failed(Failed {
                    class: classify_failure(&message),
                    message,
                }));
            }

            // available_commands and other session metadata carry nothing
            // Devorch acts on.
            _ => {}
        }

        events
    }

    fn finalize(&mut self, exit_code: Option<i32>) -> Vec<AgentEvent> {
        self.terminal.finalize(exit_code)
    }
}

/// The tool call identifier, under whichever key this version uses.
fn call_id_of(value: &Value) -> String {
    str_field(value, "id")
        .or_else(|| str_field(value, "tool_call_id"))
        .or_else(|| str_field(value, "toolCallId"))
        .unwrap_or_else(|| "tool".into())
}

/// The textual payload of a chunk.
///
/// A live session sends `{"type":"text","data":"ok"}` — the payload is a bare
/// string under `data`. The other spellings are kept as fallbacks so a future
/// version that nests it does not go silent.
fn text_of(value: &Value) -> Option<String> {
    value
        .get("data")
        .and_then(Value::as_str)
        .filter(|s| !s.is_empty())
        .map(str::to_string)
        .or_else(|| str_field(value, "text"))
        .or_else(|| str_field(value, "content"))
        .or_else(|| value.get("content").and_then(|c| str_field(c, "text")))
        .or_else(|| value.get("data").and_then(|d| str_field(d, "text")))
}

/// Read a string field, treating an empty string as absent.
fn str_field(value: &Value, key: &str) -> Option<String> {
    value
        .get(key)
        .and_then(Value::as_str)
        .filter(|s| !s.is_empty())
        .map(str::to_string)
}

/// Map Grok token accounting.
///
/// Cost lives on the outer event, tokens on the nested `usage` object; a live
/// `end` event carries both.
fn usage_event(value: &Value) -> AgentEvent {
    let usage = value.get("usage").unwrap_or(value);
    let num = |key: &str| usage.get(key).and_then(Value::as_u64);
    AgentEvent::Usage(Usage {
        input_tokens: num("input_tokens").or_else(|| num("prompt_tokens")),
        output_tokens: num("output_tokens").or_else(|| num("completion_tokens")),
        cached_input_tokens: num("cache_read_input_tokens").or_else(|| num("cached_input_tokens")),
        cost_usd: value
            .get("total_cost_usd")
            .or_else(|| usage.get("cost_usd"))
            .and_then(Value::as_f64),
    })
}
