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

use devorch_agent::adapter::AgentAdapter;
use devorch_agents::adapter_for;
use devorch_protocol::{AgentEvent, AgentKind};

/// Read a recorded stream from `testdata/agents/<agent>/<name>.jsonl`.
fn fixture(agent: AgentKind, name: &str) -> Vec<String> {
    let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../../testdata/agents")
        .join(agent.as_str())
        .join(format!("{name}.jsonl"));
    let text = std::fs::read_to_string(&path)
        .unwrap_or_else(|e| panic!("read {}: {e}", path.display()));
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
        assert!(touched, "{agent}: no tool, command or file activity reported");

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
        let has_answer = events.iter().any(
            |e| matches!(e, AgentEvent::TextDelta(t) if !t.reasoning && !t.text.is_empty()),
        );
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
