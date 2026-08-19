//! The domain event journal.
//!
//! Only low-volume domain events are persisted here, one row each. High-volume
//! material — terminal chunks, raw model streams, screenshots — must go to
//! bounded blob storage with a retention policy, never to a row per byte.
//!
//! The `(session_id, seq)` uniqueness constraint is what makes restart recovery
//! trustworthy: a replayed or duplicated adapter event cannot silently create a
//! second copy with a different id.

use chrono::{DateTime, Utc};
use devorch_protocol::{EventEnvelope, SessionId, WorkspaceId};
use rusqlite::Connection;

use crate::StoreError;

/// Append-only reads and writes over `event_journal`.
pub struct EventJournal<'a> {
    conn: &'a Connection,
}

impl<'a> EventJournal<'a> {
    pub(crate) fn new(conn: &'a Connection) -> Self {
        Self { conn }
    }

    /// Append one event. Re-appending the same `(session_id, seq)` is rejected.
    pub fn append(&self, envelope: &EventEnvelope) -> Result<(), StoreError> {
        let payload = serde_json::to_string(envelope)?;
        let event_type = event_type_of(envelope);

        self.conn.execute(
            "INSERT INTO event_journal
                 (id, session_id, seq, timestamp, mission_id, task_id, workspace_id, event_type, payload)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)",
            rusqlite::params![
                envelope.id.as_str(),
                envelope.session_id.as_str(),
                envelope.seq as i64,
                envelope.timestamp.to_rfc3339(),
                envelope.mission_id.as_ref().map(|m| m.to_string()),
                envelope.task_id.as_ref().map(|t| t.to_string()),
                envelope.workspace_id.as_ref().map(|w| w.to_string()),
                event_type,
                payload,
            ],
        )?;
        Ok(())
    }

    /// Every event for a session, in sequence order.
    pub fn session_events(&self, session: &SessionId) -> Result<Vec<EventEnvelope>, StoreError> {
        let mut stmt = self
            .conn
            .prepare("SELECT payload FROM event_journal WHERE session_id = ?1 ORDER BY seq")?;
        let rows = stmt.query_map([session.as_str()], |row| row.get::<_, String>(0))?;

        let mut events = Vec::new();
        for payload in rows {
            events.push(serde_json::from_str(&payload?)?);
        }
        Ok(events)
    }

    /// Every event recorded against a workspace, oldest first.
    pub fn workspace_events(
        &self,
        workspace: &WorkspaceId,
    ) -> Result<Vec<EventEnvelope>, StoreError> {
        let mut stmt = self.conn.prepare(
            "SELECT payload FROM event_journal WHERE workspace_id = ?1 ORDER BY timestamp, seq",
        )?;
        let rows = stmt.query_map([workspace.as_str()], |row| row.get::<_, String>(0))?;

        let mut events = Vec::new();
        for payload in rows {
            events.push(serde_json::from_str(&payload?)?);
        }
        Ok(events)
    }

    /// The highest sequence number recorded for a session, if any.
    ///
    /// After a daemon restart this is how a reattaching adapter learns where to
    /// resume numbering instead of restarting at zero.
    pub fn last_seq(&self, session: &SessionId) -> Result<Option<u64>, StoreError> {
        let value: Option<i64> = self.conn.query_row(
            "SELECT MAX(seq) FROM event_journal WHERE session_id = ?1",
            [session.as_str()],
            |row| row.get(0),
        )?;
        Ok(value.map(|v| v as u64))
    }

    /// Total number of journalled events.
    pub fn count(&self) -> Result<u64, StoreError> {
        let count: i64 = self
            .conn
            .query_row("SELECT COUNT(*) FROM event_journal", [], |row| row.get(0))?;
        Ok(count as u64)
    }

    /// Delete events older than `cutoff`, returning how many rows went.
    ///
    /// Retention is explicit; the journal never trims itself behind the
    /// operator's back.
    pub fn prune_before(&self, cutoff: DateTime<Utc>) -> Result<u64, StoreError> {
        let removed = self.conn.execute(
            "DELETE FROM event_journal WHERE timestamp < ?1",
            [cutoff.to_rfc3339()],
        )?;
        Ok(removed as u64)
    }
}

/// The discriminant string stored in `event_type`, for cheap filtering without
/// deserializing every payload.
fn event_type_of(envelope: &EventEnvelope) -> String {
    serde_json::to_value(&envelope.event)
        .ok()
        .and_then(|v| v.get("type").and_then(|t| t.as_str()).map(str::to_string))
        .unwrap_or_else(|| "unknown".to_string())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::Store;
    use devorch_protocol::event::{Completed, TextDelta};
    use devorch_protocol::AgentEvent;

    fn text(session: &SessionId, seq: u64, body: &str) -> EventEnvelope {
        EventEnvelope::new(
            session.clone(),
            seq,
            AgentEvent::TextDelta(TextDelta {
                text: body.to_string(),
                reasoning: false,
            }),
        )
    }

    #[test]
    fn events_replay_in_sequence_order() {
        let store = Store::open_in_memory().unwrap();
        let session = SessionId::new();

        // Append out of order; the journal must still replay by seq.
        store.journal().append(&text(&session, 2, "third")).unwrap();
        store.journal().append(&text(&session, 0, "first")).unwrap();
        store
            .journal()
            .append(&text(&session, 1, "second"))
            .unwrap();

        let events = store.journal().session_events(&session).unwrap();
        let bodies: Vec<_> = events
            .iter()
            .map(|e| match &e.event {
                AgentEvent::TextDelta(d) => d.text.clone(),
                other => panic!("unexpected event {other:?}"),
            })
            .collect();
        assert_eq!(bodies, vec!["first", "second", "third"]);
    }

    #[test]
    fn duplicate_sequence_numbers_are_rejected() {
        let store = Store::open_in_memory().unwrap();
        let session = SessionId::new();
        store.journal().append(&text(&session, 0, "a")).unwrap();
        assert!(store.journal().append(&text(&session, 0, "b")).is_err());
    }

    #[test]
    fn sessions_are_isolated_from_each_other() {
        let store = Store::open_in_memory().unwrap();
        let a = SessionId::new();
        let b = SessionId::new();
        store.journal().append(&text(&a, 0, "a0")).unwrap();
        store.journal().append(&text(&b, 0, "b0")).unwrap();

        assert_eq!(store.journal().session_events(&a).unwrap().len(), 1);
        assert_eq!(store.journal().session_events(&b).unwrap().len(), 1);
        assert_eq!(store.journal().count().unwrap(), 2);
    }

    #[test]
    fn last_seq_reports_where_to_resume_after_a_restart() {
        let store = Store::open_in_memory().unwrap();
        let session = SessionId::new();
        assert_eq!(store.journal().last_seq(&session).unwrap(), None);

        store.journal().append(&text(&session, 0, "a")).unwrap();
        store.journal().append(&text(&session, 7, "b")).unwrap();
        assert_eq!(store.journal().last_seq(&session).unwrap(), Some(7));
    }

    #[test]
    fn payloads_survive_a_full_round_trip() {
        let store = Store::open_in_memory().unwrap();
        let session = SessionId::new();
        let original = EventEnvelope::new(
            session.clone(),
            0,
            AgentEvent::Completed(Completed {
                summary: Some("fixed the regression".into()),
            }),
        )
        .with_vendor_payload(serde_json::json!({"raw": {"nested": [1, 2, 3]}}));

        store.journal().append(&original).unwrap();
        let loaded = store.journal().session_events(&session).unwrap();
        assert_eq!(loaded, vec![original]);
    }

    #[test]
    fn events_can_be_read_back_by_workspace() {
        let store = Store::open_in_memory().unwrap();
        let session = SessionId::new();
        let workspace = WorkspaceId::new();
        store
            .journal()
            .append(&text(&session, 0, "a").with_workspace(workspace.clone()))
            .unwrap();
        store.journal().append(&text(&session, 1, "b")).unwrap();

        assert_eq!(
            store.journal().workspace_events(&workspace).unwrap().len(),
            1
        );
    }

    #[test]
    fn pruning_removes_only_events_older_than_the_cutoff() {
        let store = Store::open_in_memory().unwrap();
        let session = SessionId::new();

        let mut old = text(&session, 0, "old");
        old.timestamp = Utc::now() - chrono::Duration::days(30);
        store.journal().append(&old).unwrap();
        store
            .journal()
            .append(&text(&session, 1, "recent"))
            .unwrap();

        let removed = store
            .journal()
            .prune_before(Utc::now() - chrono::Duration::days(7))
            .unwrap();
        assert_eq!(removed, 1);
        assert_eq!(store.journal().count().unwrap(), 1);
    }
}
