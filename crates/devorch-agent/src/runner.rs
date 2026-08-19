//! Running an agent and normalizing its stream.
//!
//! This is the only place that couples an adapter to a live process. Everything
//! above it sees normalized events; everything below it is one vendor's
//! problem.

use devorch_process::{run_streaming, ProcessError};
use devorch_protocol::{AgentEvent, AgentKind};

use crate::adapter::{AgentAdapter, ExecuteRequest};
use crate::AgentError;

/// The result of one agent run.
#[derive(Debug, Clone)]
pub struct AgentRun {
    pub agent: AgentKind,
    pub exit_code: Option<i32>,
    /// Every normalized event, in the order it was produced.
    pub events: Vec<AgentEvent>,
    /// Whatever the CLI wrote to stderr, kept for diagnosis.
    pub stderr: String,
    pub duration: std::time::Duration,
}

impl AgentRun {
    /// Whether the run ended in a normalized `Completed` event.
    ///
    /// This reports what the agent *claimed*. It is never sufficient evidence
    /// that the work is correct — that requires the verification engine.
    pub fn claimed_success(&self) -> bool {
        matches!(self.events.last(), Some(AgentEvent::Completed(_)))
    }

    /// The failure the run ended with, if it failed.
    pub fn failure(&self) -> Option<&devorch_protocol::event::Failed> {
        match self.events.last() {
            Some(AgentEvent::Failed(f)) => Some(f),
            _ => None,
        }
    }
}

/// Run one agent to completion, normalizing its stream as it arrives.
///
/// `on_event` sees each event the moment it is parsed, so a caller can journal
/// and display progress during a run rather than after it.
pub async fn run_agent<A, F>(
    adapter: &mut A,
    request: &ExecuteRequest,
    mut on_event: F,
) -> Result<AgentRun, AgentError>
where
    A: AgentAdapter + ?Sized,
    F: FnMut(&AgentEvent),
{
    let spec = adapter.build_spec(request);
    let agent = adapter.kind();
    tracing::info!(agent = %agent, command = %spec.display(), "starting agent");

    let mut events = Vec::new();

    // The adapter is borrowed mutably by the line callback, so normalization
    // and the event sink are threaded through one closure.
    let output = {
        let events = &mut events;
        let on_event = &mut on_event;
        run_streaming(&spec, |line| {
            for event in adapter.normalize(line) {
                on_event(&event);
                events.push(event);
            }
        })
        .await
    };

    let output = match output {
        Ok(output) => output,
        Err(ProcessError::NotFound { .. }) => return Err(AgentError::NotInstalled { agent }),
        Err(e) => return Err(e.into()),
    };

    for event in adapter.finalize(output.exit_code) {
        on_event(&event);
        events.push(event);
    }

    Ok(AgentRun {
        agent,
        exit_code: output.exit_code,
        events,
        stderr: output.stderr,
        duration: output.duration,
    })
}
