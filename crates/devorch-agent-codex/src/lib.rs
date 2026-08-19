//! Codex adapter.
//!
//! Drives `codex exec --json`, whose stream is a thread/turn/item hierarchy.
//! Items carry their own kind — `agent_message`, `reasoning`,
//! `command_execution`, `file_change`, `mcp_tool_call`, `web_search`,
//! `todo_list`, `error` — and each can appear as started, updated or completed.
//!
//! Devorch domain types are deliberately not coupled to this schema: everything
//! Codex-shaped stops at this file.

use devorch_agent::adapter::{parse_json_line, AgentAdapter, ExecuteRequest, TerminalTracker};
use devorch_process::ProcessSpec;
use devorch_protocol::event::{
    Completed, CommandCompleted, CommandStarted, Failed, FileChanged, PlanStep, PlanStepStatus,
    PlanUpdated, SessionStarted, TextDelta, ToolCompleted, ToolStarted, TurnCompleted, TurnStarted,
    Usage,
};
use devorch_protocol::{
    AgentEvent, AgentKind, AgentRuntimeMode, FailureClass, FileChangeKind,
};
use serde_json::Value;

/// Normalizes the `codex exec --json` stream.
#[derive(Debug, Default)]
pub struct CodexAdapter {
    terminal: TerminalTracker,
    turn: u32,
}

impl CodexAdapter {
    /// A fresh adapter for one Codex session.
    pub fn new() -> Self {
        Self::default()
    }
}

impl AgentAdapter for CodexAdapter {
    fn kind(&self) -> AgentKind {
        AgentKind::Codex
    }

    fn mode(&self) -> AgentRuntimeMode {
        AgentRuntimeMode::OneShotStructured
    }

    fn build_spec(&self, request: &ExecuteRequest) -> ProcessSpec {
        let mut spec = ProcessSpec::new("codex")
            .arg("exec")
            .arg("--json")
            .arg("--skip-git-repo-check");

        if let Some(model) = &request.model {
            spec = spec.arg("--model").arg(model);
        }

        // The prompt is passed as a positional argument, never interpolated
        // into a shell string.
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
            "thread.started" => vec![AgentEvent::SessionStarted(SessionStarted {
                agent: AgentKind::Codex,
                mode: AgentRuntimeMode::OneShotStructured,
                vendor_session_id: str_field(&value, "thread_id"),
                model: str_field(&value, "model"),
            })],

            "turn.started" => {
                self.turn += 1;
                vec![AgentEvent::TurnStarted(TurnStarted { turn: self.turn })]
            }

            "turn.completed" => {
                let mut events = vec![AgentEvent::TurnCompleted(TurnCompleted {
                    turn: self.turn,
                })];
                if let Some(usage) = value.get("usage") {
                    events.push(usage_event(usage));
                }
                events
            }

            "turn.failed" => {
                self.terminal.mark();
                let message = value
                    .get("error")
                    .and_then(|e| e.get("message"))
                    .and_then(Value::as_str)
                    .unwrap_or("turn failed")
                    .to_string();
                vec![AgentEvent::Failed(Failed {
                    message,
                    class: FailureClass::ToolFailure,
                })]
            }

            "error" => {
                self.terminal.mark();
                vec![AgentEvent::Failed(Failed {
                    message: str_field(&value, "message")
                        .unwrap_or_else(|| "codex reported an error".into()),
                    class: FailureClass::AgentProtocol,
                })]
            }

            "item.started" | "item.updated" | "item.completed" => {
                let Some(item) = value.get("item") else {
                    return Vec::new();
                };
                self.normalize_item(kind, item)
            }

            // Codex emits a final `thread.completed` on some versions; when it
            // does not, `finalize` supplies the terminal event.
            "thread.completed" => {
                self.terminal.mark();
                vec![AgentEvent::Completed(Completed {
                    summary: str_field(&value, "summary"),
                })]
            }

            _ => Vec::new(),
        }
    }

    fn finalize(&mut self, exit_code: Option<i32>) -> Vec<AgentEvent> {
        // A clean exit is Codex's normal end-of-run signal, so it becomes a
        // completion rather than the tracker's protocol failure.
        if !self.terminal.seen() && exit_code == Some(0) {
            self.terminal.mark();
            return vec![AgentEvent::Completed(Completed { summary: None })];
        }
        self.terminal.finalize(exit_code)
    }
}

impl CodexAdapter {
    /// Map one item payload, given whether it started, updated or completed.
    fn normalize_item(&mut self, stage: &str, item: &Value) -> Vec<AgentEvent> {
        let item_type = item.get("type").and_then(Value::as_str).unwrap_or_default();
        let id = str_field(item, "id").unwrap_or_else(|| "item".to_string());
        let completed = stage == "item.completed";

        match item_type {
            // Only completed messages carry final text; emitting on `started`
            // as well would duplicate the whole message.
            "agent_message" if completed => text_event(item, false),

            "reasoning" if completed => text_event(item, true),

            "command_execution" => {
                let command = str_field(item, "command").unwrap_or_default();
                if completed {
                    vec![AgentEvent::CommandCompleted(CommandCompleted {
                        call_id: id,
                        exit_code: item
                            .get("exit_code")
                            .and_then(Value::as_i64)
                            .map(|c| c as i32),
                    })]
                } else if stage == "item.started" {
                    vec![AgentEvent::CommandStarted(CommandStarted {
                        call_id: id,
                        command,
                    })]
                } else {
                    Vec::new()
                }
            }

            "file_change" if completed => item
                .get("changes")
                .and_then(Value::as_array)
                .map(|changes| {
                    changes
                        .iter()
                        .filter_map(|change| {
                            Some(AgentEvent::FileChanged(FileChanged {
                                path: str_field(change, "path")?,
                                kind: match change
                                    .get("kind")
                                    .and_then(Value::as_str)
                                    .unwrap_or("update")
                                {
                                    "add" | "added" | "create" => FileChangeKind::Added,
                                    "delete" | "deleted" | "remove" => FileChangeKind::Deleted,
                                    "rename" | "renamed" => FileChangeKind::Renamed,
                                    _ => FileChangeKind::Modified,
                                },
                            }))
                        })
                        .collect()
                })
                .unwrap_or_default(),

            "mcp_tool_call" | "web_search" | "collab_tool_call" => {
                let name = str_field(item, "tool")
                    .or_else(|| str_field(item, "name"))
                    .unwrap_or_else(|| item_type.to_string());
                if completed {
                    vec![AgentEvent::ToolCompleted(ToolCompleted {
                        call_id: id,
                        name,
                        success: item
                            .get("status")
                            .and_then(Value::as_str)
                            .map(|s| s != "failed")
                            .unwrap_or(true),
                        output: str_field(item, "output"),
                    })]
                } else if stage == "item.started" {
                    vec![AgentEvent::ToolStarted(ToolStarted {
                        call_id: id,
                        name,
                        arguments: item.get("arguments").cloned(),
                    })]
                } else {
                    Vec::new()
                }
            }

            "todo_list" => item
                .get("items")
                .and_then(Value::as_array)
                .map(|entries| {
                    vec![AgentEvent::PlanUpdated(PlanUpdated {
                        steps: entries
                            .iter()
                            .filter_map(|entry| {
                                Some(PlanStep {
                                    description: str_field(entry, "text")
                                        .or_else(|| str_field(entry, "content"))?,
                                    status: match entry.get("completed").and_then(Value::as_bool) {
                                        Some(true) => PlanStepStatus::Completed,
                                        _ => PlanStepStatus::Pending,
                                    },
                                })
                            })
                            .collect(),
                    })]
                })
                .unwrap_or_default(),

            "error" => {
                self.terminal.mark();
                vec![AgentEvent::Failed(Failed {
                    message: str_field(item, "message")
                        .unwrap_or_else(|| "codex item error".into()),
                    class: FailureClass::ToolFailure,
                })]
            }

            _ => Vec::new(),
        }
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

/// Build a text event from an item's `text` field.
fn text_event(item: &Value, reasoning: bool) -> Vec<AgentEvent> {
    match str_field(item, "text") {
        Some(text) => vec![AgentEvent::TextDelta(TextDelta { text, reasoning })],
        None => Vec::new(),
    }
}

/// Map Codex token accounting. Codex does not report cost, so it stays `None`
/// rather than becoming a misleading zero.
fn usage_event(usage: &Value) -> AgentEvent {
    let num = |key: &str| usage.get(key).and_then(Value::as_u64);
    AgentEvent::Usage(Usage {
        input_tokens: num("input_tokens"),
        output_tokens: num("output_tokens"),
        cached_input_tokens: num("cached_input_tokens"),
        cost_usd: None,
    })
}
