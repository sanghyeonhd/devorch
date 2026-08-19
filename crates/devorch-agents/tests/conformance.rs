//! Adapter conformance.
//!
//! Gate R2 of the migration plan reads: "recorded streams normalize to the same
//! event contract." That is what this file checks. Each vendor fixture is a real
//! stream shape; the assertions are about the *contract*, not about any one
//! vendor's wording.
//!
//! No process, no network, no subscription: default CI must never fail because
//! a provider quota ran out.

use std::path::PathBuf;

use devorch_agents::adapter_for;
use devorch_protocol::{AgentEvent, AgentKind};

/// Read a recorded stream from `testdata/agents/<agent>/<name>.jsonl`.
fn fixture(agent: AgentKind, name: &str) -> Vec<String> {
    let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../../testdata/agents")
        .join(agent.as_str())
        .join(format!("{name}.jsonl"));
    let text =
        std::fs::read_to_string(&path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()));
    text.lines().map(str::to_string).collect()
}

/// Replay a fixture through the adapter, as the runner would.
fn replay(agent: AgentKind, name: &str, exit_code: Option<i32>) -> Vec<AgentEvent> {
    let mut adapter = adapter_for(agent).expect("adapter exists");
    let mut events = Vec::new();
    for line in fixture(agent, name) {
        events.extend(adapter.normalize(&line));
    }
    events.extend(adapter.finalize(exit_code));
    events
}

#[test]
fn every_adapter_opens_with_a_session_and_ends_with_a_terminal_event() {
    for agent in AgentKind::ALL {
        let events = replay(agent, "session", Some(0));

        assert!(
            matches!(events.first(), Some(AgentEvent::SessionStarted(s)) if s.agent == agent),
            "{agent}: first event must be SessionStarted for this agent, got {:?}",
            events.first()
        );
        assert!(
            matches!(events.last(), Some(AgentEvent::Completed(_))),
            "{agent}: successful session must end in Completed, got {:?}",
            events.last()
        );
    }
}

#[test]
fn every_adapter_reports_the_edit_and_the_command_it_ran() {
    // The vendors disagree about *how* a file edit is signalled — Codex has a
    // file_change item, the others go through tool calls — so the contract-level
    // assertion is that some tool or file activity was reported at all.
    for agent in AgentKind::ALL {
        let events = replay(agent, "session", Some(0));

        let touched = events.iter().any(|e| {
            matches!(
                e,
                AgentEvent::FileChanged(_)
                    | AgentEvent::ToolStarted(_)
                    | AgentEvent::CommandStarted(_)
            )
        });
        assert!(
            touched,
            "{agent}: no tool, command or file activity reported"
        );

        let finished_work = events.iter().any(|e| {
            matches!(
                e,
                AgentEvent::ToolCompleted(_) | AgentEvent::CommandCompleted(_)
            )
        });
        assert!(finished_work, "{agent}: nothing was reported as completing");
    }
}

#[test]
fn every_adapter_produces_assistant_text() {
    for agent in AgentKind::ALL {
        let events = replay(agent, "session", Some(0));
        let has_answer = events
            .iter()
            .any(|e| matches!(e, AgentEvent::TextDelta(t) if !t.reasoning && !t.text.is_empty()));
        assert!(has_answer, "{agent}: no assistant text was normalized");
    }
}

#[test]
fn every_adapter_reports_usage_without_inventing_a_cost() {
    for agent in AgentKind::ALL {
        let events = replay(agent, "session", Some(0));
        let usage = events
            .iter()
            .find_map(|e| match e {
                AgentEvent::Usage(u) => Some(u),
                _ => None,
            })
            .unwrap_or_else(|| panic!("{agent}: no usage event"));

        assert!(usage.input_tokens.is_some(), "{agent}: no input tokens");
        assert!(usage.output_tokens.is_some(), "{agent}: no output tokens");

        // Unknown cost must stay unknown. A vendor that does not report cost
        // must not be recorded as free.
        if let Some(cost) = usage.cost_usd {
            assert!(cost > 0.0, "{agent}: reported a zero cost instead of none");
        }
    }
}

#[test]
fn reasoning_text_is_flagged_separately_from_the_answer() {
    // Codex, Claude and Grok all expose internal reasoning. It must be
    // distinguishable so it is never shown as if it were the agent's answer.
    for agent in [AgentKind::Codex, AgentKind::Claude, AgentKind::Grok] {
        let events = replay(agent, "session", Some(0));
        assert!(
            events
                .iter()
                .any(|e| matches!(e, AgentEvent::TextDelta(t) if t.reasoning)),
            "{agent}: reasoning was not flagged"
        );
    }
}

#[test]
fn every_adapter_maps_a_vendor_failure_onto_the_shared_taxonomy() {
    for agent in AgentKind::ALL {
        let events = replay(agent, "failure", Some(1));
        assert!(
            matches!(events.last(), Some(AgentEvent::Failed(_))),
            "{agent}: failure fixture did not end in Failed, got {:?}",
            events.last()
        );
    }
}

#[test]
fn rate_limits_and_quota_errors_are_classified_as_such() {
    use devorch_protocol::FailureClass;

    for agent in [AgentKind::Grok, AgentKind::Gemini] {
        let events = replay(agent, "failure", Some(1));
        match events.last() {
            Some(AgentEvent::Failed(f)) => assert_eq!(
                f.class,
                FailureClass::ModelRateLimit,
                "{agent}: rate limit was misclassified"
            ),
            other => panic!("{agent}: expected a failure, got {other:?}"),
        }
    }
}

#[test]
fn a_stream_that_stops_without_a_result_is_never_reported_as_success() {
    // "The process disappeared" must not become success. Every adapter is fed a
    // truncated stream and a crash exit code.
    for agent in AgentKind::ALL {
        let mut adapter = adapter_for(agent).expect("adapter exists");
        let mut events = Vec::new();
        // Only the first two lines: the session opens and then the stream dies.
        for line in fixture(agent, "session").into_iter().take(2) {
            events.extend(adapter.normalize(&line));
        }
        events.extend(adapter.finalize(Some(137)));

        assert!(
            matches!(events.last(), Some(AgentEvent::Failed(_))),
            "{agent}: a truncated stream must fail, got {:?}",
            events.last()
        );
    }
}

#[test]
fn noise_between_structured_lines_is_ignored_rather_than_fatal() {
    // CLIs interleave update notices and warnings with their JSON stream.
    for agent in AgentKind::ALL {
        let mut adapter = adapter_for(agent).expect("adapter exists");
        let mut events = Vec::new();
        for line in fixture(agent, "session") {
            events.extend(adapter.normalize("npm notice: a new version is available"));
            events.extend(adapter.normalize(""));
            events.extend(adapter.normalize("{ this is not json }"));
            events.extend(adapter.normalize(&line));
        }
        events.extend(adapter.finalize(Some(0)));

        assert!(
            matches!(events.last(), Some(AgentEvent::Completed(_))),
            "{agent}: noise broke the stream"
        );
    }
}

#[test]
fn adapters_never_shell_out_and_always_run_inside_the_given_worktree() {
    use devorch_agent::adapter::ExecuteRequest;

    let workdir = PathBuf::from("/tmp/devorch-wt");
    let request = ExecuteRequest::new("fix the seeded regression; do not touch main", &workdir);

    for agent in AgentKind::ALL {
        let adapter = adapter_for(agent).expect("adapter exists");
        let spec = adapter.build_spec(&request);

        assert_eq!(
            spec.cwd.as_deref(),
            Some(workdir.as_path()),
            "{agent}: must run in the candidate's worktree"
        );
        // A prompt is an argument, never part of a shell command line.
        assert!(
            !spec.program.contains("sh"),
            "{agent}: must not invoke a shell"
        );
        assert!(
            spec.args.iter().any(|a| a == &request.prompt),
            "{agent}: the prompt must be passed as its own argument"
        );
        // Devorch owns the permission decision; no vendor bypass switch.
        for banned in ["--yolo", "--dangerously-bypass-approvals-and-sandbox"] {
            assert!(
                !spec.args.iter().any(|a| a == banned),
                "{agent}: must not pass {banned}"
            );
        }
    }
}

// ---------------------------------------------------------------------------
// Live-captured streams.
//
// The fixtures above were written from published schemas. These were captured
// by actually running the CLIs, and each one corrected a real defect that the
// documented schema alone did not reveal. They are kept verbatim so a future
// change to an adapter has to keep working against what the tools really emit.
// ---------------------------------------------------------------------------

/// Replay a named fixture, as the runner would.
fn replay_named(agent: AgentKind, name: &str, exit_code: Option<i32>) -> Vec<AgentEvent> {
    let mut adapter = adapter_for(agent).expect("adapter exists");
    let mut events = Vec::new();
    for line in fixture(agent, name) {
        events.extend(adapter.normalize(&line));
    }
    events.extend(adapter.finalize(exit_code));
    events
}

#[test]
fn a_real_grok_session_yields_its_answer_its_usage_and_its_cost() {
    // Captured from `grok -p ... --output-format streaming-json`. Grok puts
    // chunk text in a bare `data` string, which the documented `text` field
    // would have missed entirely — the adapter produced no answer at all before
    // this fixture existed.
    let events = replay_named(AgentKind::Grok, "live-simple", Some(0));

    let answer: String = events
        .iter()
        .filter_map(|e| match e {
            AgentEvent::TextDelta(t) if !t.reasoning => Some(t.text.as_str()),
            _ => None,
        })
        .collect();
    assert_eq!(answer, "ok", "the assistant's answer must be recovered");

    assert!(
        events
            .iter()
            .any(|e| matches!(e, AgentEvent::TextDelta(t) if t.reasoning)),
        "thought chunks must be flagged as reasoning"
    );

    let usage = events
        .iter()
        .filter_map(|e| match e {
            AgentEvent::Usage(u) => Some(u),
            _ => None,
        })
        .next_back()
        .expect("usage was reported");
    assert_eq!(usage.input_tokens, Some(9116));
    assert_eq!(usage.output_tokens, Some(36));
    assert_eq!(usage.cached_input_tokens, Some(9472));
    // Grok reports real cost on `end`; it must be carried, not dropped.
    assert_eq!(usage.cost_usd, Some(0.00394128));

    assert!(matches!(events.last(), Some(AgentEvent::Completed(_))));
}

#[test]
fn a_real_codex_usage_limit_is_classified_as_a_rate_limit_not_a_protocol_error() {
    // Captured from `codex exec --json` after the account hit its usage limit.
    use devorch_protocol::FailureClass;

    let events = replay_named(AgentKind::Codex, "live-usage-limit", Some(1));

    match events.last() {
        Some(AgentEvent::Failed(f)) => assert_eq!(
            f.class,
            FailureClass::ModelRateLimit,
            "a usage limit is retryable later; a protocol error is not"
        ),
        other => panic!("expected a failure, got {other:?}"),
    }
}

#[test]
fn an_advisory_item_error_does_not_end_a_codex_session() {
    // The same live capture carries an item-level error about shortened skill
    // descriptions — a warning, mid-run. If that ended the session, a healthy
    // run that happened to emit one would be reported as failed.
    let mut adapter = adapter_for(AgentKind::Codex).expect("adapter exists");

    let mut events = Vec::new();
    for line in [
        r#"{"type":"thread.started","thread_id":"th_1"}"#,
        r#"{"type":"turn.started"}"#,
        r#"{"type":"item.completed","item":{"id":"item_0","type":"error","message":"Skill descriptions were shortened to fit the context budget."}}"#,
        r#"{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"Done."}}"#,
    ] {
        events.extend(adapter.normalize(line));
    }
    events.extend(adapter.finalize(Some(0)));

    assert!(
        matches!(events.last(), Some(AgentEvent::Completed(_))),
        "an advisory item error must not end the session: {:?}",
        events.last()
    );
}

#[test]
fn a_real_claude_session_yields_its_answer_its_model_and_its_reported_cost() {
    // Captured from `claude -p ... --output-format stream-json --verbose`.
    // The live stream carries event types the published schema does not mention
    // — `rate_limit_event` and a `post_turn_summary` system line — and its
    // `result` line has no `subtype` at all. All three must pass through
    // harmlessly.
    let events = replay_named(AgentKind::Claude, "live-simple", Some(0));

    match events.first() {
        Some(AgentEvent::SessionStarted(s)) => {
            assert_eq!(s.agent, AgentKind::Claude);
            assert_eq!(s.model.as_deref(), Some("claude-sonnet-5"));
            assert!(s.vendor_session_id.is_some(), "the session id must be kept");
        }
        other => panic!("expected SessionStarted, got {other:?}"),
    }

    let answer: String = events
        .iter()
        .filter_map(|e| match e {
            AgentEvent::TextDelta(t) if !t.reasoning => Some(t.text.as_str()),
            _ => None,
        })
        .collect();
    assert_eq!(answer, "ok");

    let usage = events
        .iter()
        .filter_map(|e| match e {
            AgentEvent::Usage(u) => Some(u),
            _ => None,
        })
        .next_back()
        .expect("usage was reported");
    assert_eq!(usage.input_tokens, Some(2));
    assert_eq!(usage.output_tokens, Some(4));
    assert_eq!(usage.cached_input_tokens, Some(30069));
    assert!(usage.cost_usd.unwrap() > 0.0, "real cost must be carried");

    // A `result` line with is_error false and no subtype is a success.
    assert!(matches!(events.last(), Some(AgentEvent::Completed(_))));
}
