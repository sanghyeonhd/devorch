//! Event sequencing and journalling.
//!
//! Adapters produce vendor events at their own pace; this crate assigns the
//! monotonic per-session sequence numbers that make the journal replayable and
//! writes them through to the store. Sequence assignment lives here rather than
//! in each adapter so four independently written adapters cannot each invent
//! their own numbering rule.

use devorch_protocol::{AgentEvent, EventEnvelope, SessionId, WorkspaceId};
use devorch_store::{Store, StoreError};

/// Failure modes of event recording.
#[derive(Debug, thiserror::Error)]
pub enum EventError {
    #[error(transparent)]
    Store(#[from] StoreError),
}

/// Assigns sequence numbers for one session and persists the events.
///
/// A recorder resumed after a daemon restart continues from the highest
/// sequence number already in the journal, so a reattached session never
/// overwrites its own history.
pub struct SessionRecorder<'a> {
    store: &'a Store,
    session_id: SessionId,
    workspace_id: Option<WorkspaceId>,
    next_seq: u64,
}

impl<'a> SessionRecorder<'a> {
    /// Start recording a fresh session at sequence 0.
    pub fn new(store: &'a Store, session_id: SessionId) -> Self {
        Self {
            store,
            session_id,
            workspace_id: None,
            next_seq: 0,
        }
    }

    /// Resume recording an existing session, continuing its numbering.
    pub fn resume(store: &'a Store, session_id: SessionId) -> Result<Self, EventError> {
        let next_seq = store
            .journal()
            .last_seq(&session_id)?
            .map(|seq| seq + 1)
            .unwrap_or(0);
        Ok(Self {
            store,
            session_id,
            workspace_id: None,
            next_seq,
        })
    }

    /// Tag every subsequent event with a workspace.
    pub fn with_workspace(mut self, workspace_id: WorkspaceId) -> Self {
        self.workspace_id = Some(workspace_id);
        self
    }

    /// The session being recorded.
    pub fn session_id(&self) -> &SessionId {
        &self.session_id
    }

    /// The sequence number the next event will receive.
    pub fn next_seq(&self) -> u64 {
        self.next_seq
    }

    /// Assign the next sequence number to `event`, persist it, and return the
    /// envelope that was written.
    pub fn record(&mut self, event: AgentEvent) -> Result<EventEnvelope, EventError> {
        let mut envelope = EventEnvelope::new(self.session_id.clone(), self.next_seq, event);
        envelope.workspace_id = self.workspace_id.clone();
        self.store.journal().append(&envelope)?;
        self.next_seq += 1;
        Ok(envelope)
    }

    /// Record `event` alongside the raw vendor payload it was normalized from.
    pub fn record_with_payload(
        &mut self,
        event: AgentEvent,
        vendor_payload: serde_json::Value,
    ) -> Result<EventEnvelope, EventError> {
        let mut envelope = EventEnvelope::new(self.session_id.clone(), self.next_seq, event)
            .with_vendor_payload(vendor_payload);
        envelope.workspace_id = self.workspace_id.clone();
        self.store.journal().append(&envelope)?;
        self.next_seq += 1;
        Ok(envelope)
    }

    /// Replay everything recorded for this session so far.
    pub fn replay(&self) -> Result<Vec<EventEnvelope>, EventError> {
        Ok(self.store.journal().session_events(&self.session_id)?)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use devorch_protocol::event::{Completed, TextDelta};

    fn delta(text: &str) -> AgentEvent {
        AgentEvent::TextDelta(TextDelta {
            text: text.to_string(),
            reasoning: false,
        })
    }

    #[test]
    fn sequence_numbers_are_monotonic_from_zero() {
        let store = Store::open_in_memory().unwrap();
        let mut rec = SessionRecorder::new(&store, SessionId::new());

        assert_eq!(rec.record(delta("a")).unwrap().seq, 0);
        assert_eq!(rec.record(delta("b")).unwrap().seq, 1);
        assert_eq!(rec.record(delta("c")).unwrap().seq, 2);
        assert_eq!(rec.next_seq(), 3);
    }

    #[test]
    fn a_resumed_session_continues_rather_than_restarting_numbering() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("devorch3.db");
        let session = SessionId::new();

        {
            let store = Store::open(&path).unwrap();
            let mut rec = SessionRecorder::new(&store, session.clone());
            rec.record(delta("before restart")).unwrap();
            rec.record(delta("also before")).unwrap();
        }

        // Simulate a daemon restart: new process, same database.
        let store = Store::open(&path).unwrap();
        let mut rec = SessionRecorder::resume(&store, session.clone()).unwrap();
        assert_eq!(rec.next_seq(), 2, "must resume after the persisted events");

        let envelope = rec.record(delta("after restart")).unwrap();
        assert_eq!(envelope.seq, 2);
        assert_eq!(rec.replay().unwrap().len(), 3);
    }

    #[test]
    fn resuming_an_unknown_session_starts_at_zero() {
        let store = Store::open_in_memory().unwrap();
        let rec = SessionRecorder::resume(&store, SessionId::new()).unwrap();
        assert_eq!(rec.next_seq(), 0);
    }

    #[test]
    fn recorded_events_carry_the_workspace_tag() {
        let store = Store::open_in_memory().unwrap();
        let workspace = WorkspaceId::new();
        let mut rec =
            SessionRecorder::new(&store, SessionId::new()).with_workspace(workspace.clone());

        let envelope = rec.record(delta("a")).unwrap();
        assert_eq!(envelope.workspace_id, Some(workspace.clone()));
        assert_eq!(
            store.journal().workspace_events(&workspace).unwrap().len(),
            1
        );
    }

    #[test]
    fn the_vendor_payload_is_preserved_next_to_the_normalized_event() {
        let store = Store::open_in_memory().unwrap();
        let mut rec = SessionRecorder::new(&store, SessionId::new());

        let raw = serde_json::json!({"type": "item.completed", "item": {"id": "x"}});
        rec.record_with_payload(
            AgentEvent::Completed(Completed { summary: None }),
            raw.clone(),
        )
        .unwrap();

        let replayed = rec.replay().unwrap();
        assert_eq!(replayed[0].vendor_payload, Some(raw));
    }
}
