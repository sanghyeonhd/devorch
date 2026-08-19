//! A deterministic agent for tests and dry runs.
//!
//! The plan requires fakes before real backends, and this is why: the mission
//! loop has paths that are hard to reach with live agents on purpose — a
//! candidate that lies about succeeding, one that drops a stray file, one that
//! conflicts on merge. Those paths still have to be correct.
//!
//! [`FakeAgent`] performs a scripted edit by writing files directly, so a test
//! can construct the exact candidate it needs without a subscription, a network
//! or a wall-clock dependency.

use std::path::PathBuf;

use devorch_protocol::event::{Completed, Failed, SessionStarted, TextDelta};
use devorch_protocol::{AgentEvent, AgentKind, AgentRuntimeMode, FailureClass};

/// What a fake agent does when it runs.
#[derive(Debug, Clone)]
pub enum FakeBehavior {
    /// Write these `(relative path, contents)` pairs, then report success.
    Edit(Vec<(PathBuf, String)>),
    /// Write the files but report failure.
    EditThenFail(Vec<(PathBuf, String)>),
    /// Change nothing and report success anyway — the agent that lies.
    ClaimSuccessWithoutWorking,
    /// Fail immediately.
    Fail(String),
}

impl FakeBehavior {
    /// Write one file and report success.
    pub fn write(path: impl Into<PathBuf>, contents: impl Into<String>) -> Self {
        FakeBehavior::Edit(vec![(path.into(), contents.into())])
    }
}

/// A scripted agent that stands in for a vendor CLI.
#[derive(Debug)]
pub struct FakeAgent {
    agent: AgentKind,
    behavior: FakeBehavior,
}

impl FakeAgent {
    /// A fake standing in for `agent`, doing `behavior`.
    pub fn new(agent: AgentKind, behavior: FakeBehavior) -> Self {
        Self { agent, behavior }
    }

    /// Perform the scripted edit in `workdir`.
    ///
    /// Failures to write are surfaced as agent failures rather than panics: a
    /// test that mis-specifies a path should see a failing candidate, which is
    /// a real state the mission loop must handle.
    fn perform(&self, workdir: &std::path::Path) -> Result<(), String> {
        let files = match &self.behavior {
            FakeBehavior::Edit(files) | FakeBehavior::EditThenFail(files) => files,
            _ => return Ok(()),
        };

        for (path, contents) in files {
            let target = workdir.join(path);
            if let Some(parent) = target.parent() {
                std::fs::create_dir_all(parent).map_err(|e| e.to_string())?;
            }
            std::fs::write(&target, contents).map_err(|e| e.to_string())?;
        }
        Ok(())
    }
}

/// A [`CandidateExecutor`](crate::CandidateExecutor) built from scripted fakes.
///
/// Agents with no script do nothing, which is itself a case the mission loop
/// has to handle: a candidate that changed nothing loses on the inventory gate.
#[derive(Debug, Default)]
pub struct FakeExecutor {
    behaviors: std::collections::BTreeMap<AgentKind, FakeBehavior>,
}

impl FakeExecutor {
    /// An executor with no scripted agents.
    pub fn new() -> Self {
        Self::default()
    }

    /// Script `agent` to do `behavior`.
    pub fn with(mut self, agent: AgentKind, behavior: FakeBehavior) -> Self {
        self.behaviors.insert(agent, behavior);
        self
    }
}

#[async_trait::async_trait]
impl crate::CandidateExecutor for FakeExecutor {
    async fn execute(
        &self,
        agent: AgentKind,
        worktree: &std::path::Path,
        _prompt: &str,
        _timeout: Option<std::time::Duration>,
    ) -> Vec<AgentEvent> {
        match self.behaviors.get(&agent) {
            Some(behavior) => run_fake(&FakeAgent::new(agent, behavior.clone()), worktree),
            None => Vec::new(),
        }
    }
}

/// Run a fake agent's behavior against `workdir` and return its events.
pub fn run_fake(agent: &FakeAgent, workdir: &std::path::Path) -> Vec<AgentEvent> {
    let mut events = vec![AgentEvent::SessionStarted(SessionStarted {
        agent: agent.agent,
        mode: AgentRuntimeMode::OneShotStructured,
        vendor_session_id: None,
        model: Some("fake".into()),
    })];

    if let Err(e) = agent.perform(workdir) {
        events.push(AgentEvent::Failed(Failed {
            message: e,
            class: FailureClass::EnvironmentFailure,
        }));
        return events;
    }

    match &agent.behavior {
        FakeBehavior::Edit(_) => {
            events.push(AgentEvent::TextDelta(TextDelta {
                text: "applied the scripted edit".into(),
                reasoning: false,
            }));
            events.push(AgentEvent::Completed(Completed {
                summary: Some("done".into()),
            }));
        }
        FakeBehavior::EditThenFail(_) => {
            events.push(AgentEvent::Failed(Failed {
                message: "scripted failure after editing".into(),
                class: FailureClass::TestFailure,
            }));
        }
        FakeBehavior::ClaimSuccessWithoutWorking => {
            events.push(AgentEvent::TextDelta(TextDelta {
                text: "I have fixed the issue.".into(),
                reasoning: false,
            }));
            events.push(AgentEvent::Completed(Completed {
                summary: Some("fixed everything".into()),
            }));
        }
        FakeBehavior::Fail(message) => {
            events.push(AgentEvent::Failed(Failed {
                message: message.clone(),
                class: FailureClass::AgentCrash,
            }));
        }
    }

    events
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn an_edit_writes_the_file_and_reports_completion() {
        let dir = tempfile::tempdir().unwrap();
        let agent = FakeAgent::new(
            AgentKind::Codex,
            FakeBehavior::write("calculator.py", "def add(a, b):\n    return a + b\n"),
        );

        let events = run_fake(&agent, dir.path());
        assert!(matches!(events.last(), Some(AgentEvent::Completed(_))));
        assert!(dir.path().join("calculator.py").exists());
    }

    #[test]
    fn nested_paths_are_created() {
        let dir = tempfile::tempdir().unwrap();
        let agent = FakeAgent::new(
            AgentKind::Claude,
            FakeBehavior::write("src/deep/mod.rs", "// ok\n"),
        );

        run_fake(&agent, dir.path());
        assert!(dir.path().join("src/deep/mod.rs").exists());
    }

    #[test]
    fn the_lying_agent_claims_success_while_changing_nothing() {
        let dir = tempfile::tempdir().unwrap();
        let agent = FakeAgent::new(AgentKind::Grok, FakeBehavior::ClaimSuccessWithoutWorking);

        let events = run_fake(&agent, dir.path());
        assert!(matches!(events.last(), Some(AgentEvent::Completed(_))));
        assert_eq!(std::fs::read_dir(dir.path()).unwrap().count(), 0);
    }

    #[test]
    fn an_edit_then_fail_leaves_the_file_but_reports_failure() {
        let dir = tempfile::tempdir().unwrap();
        let agent = FakeAgent::new(
            AgentKind::Gemini,
            FakeBehavior::EditThenFail(vec![("a.txt".into(), "x".into())]),
        );

        let events = run_fake(&agent, dir.path());
        assert!(matches!(events.last(), Some(AgentEvent::Failed(_))));
        assert!(dir.path().join("a.txt").exists());
    }
}
