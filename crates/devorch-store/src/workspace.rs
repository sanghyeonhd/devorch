//! Workspace records.
//!
//! A workspace is the durable record of a git worktree Devorch created. The
//! record outlives the directory on purpose: after a losing candidate's worktree
//! is removed, the row remains so the mission history still explains what ran
//! where and against which base commit.

use std::path::{Path, PathBuf};

use chrono::{DateTime, Utc};
use devorch_protocol::{AgentKind, WorkspaceId};
use rusqlite::{Connection, OptionalExtension, Row};

use crate::StoreError;

/// Lifecycle of a workspace.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum WorkspaceState {
    /// The worktree exists on disk.
    Active,
    /// The worktree has been removed; the record is retained for history.
    Removed,
}

impl WorkspaceState {
    fn as_str(&self) -> &'static str {
        match self {
            WorkspaceState::Active => "active",
            WorkspaceState::Removed => "removed",
        }
    }

    fn parse(raw: &str) -> Self {
        match raw {
            "removed" => WorkspaceState::Removed,
            _ => WorkspaceState::Active,
        }
    }
}

/// A persisted workspace.
#[derive(Debug, Clone, PartialEq)]
pub struct WorkspaceRecord {
    pub id: WorkspaceId,
    pub name: String,
    pub repo_root: PathBuf,
    pub worktree_path: PathBuf,
    pub branch: String,
    pub base_commit: String,
    pub agent: Option<AgentKind>,
    pub state: WorkspaceState,
    pub created_at: DateTime<Utc>,
    pub removed_at: Option<DateTime<Utc>>,
}

/// Reads and writes over the `workspaces` table.
pub struct WorkspaceStore<'a> {
    conn: &'a Connection,
}

impl<'a> WorkspaceStore<'a> {
    pub(crate) fn new(conn: &'a Connection) -> Self {
        Self { conn }
    }

    /// Insert a new workspace record.
    ///
    /// Returns [`StoreError::Duplicate`] rather than a raw constraint error when
    /// the name is taken, so the CLI can report something actionable.
    pub fn insert(&self, record: &WorkspaceRecord) -> Result<(), StoreError> {
        let result = self.conn.execute(
            "INSERT INTO workspaces
                 (id, name, repo_root, worktree_path, branch, base_commit, agent, state, created_at, removed_at)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10)",
            rusqlite::params![
                record.id.as_str(),
                record.name,
                record.repo_root.to_string_lossy(),
                record.worktree_path.to_string_lossy(),
                record.branch,
                record.base_commit,
                record.agent.map(|a| a.as_str()),
                record.state.as_str(),
                record.created_at.to_rfc3339(),
                record.removed_at.map(|t| t.to_rfc3339()),
            ],
        );

        match result {
            Ok(_) => Ok(()),
            Err(rusqlite::Error::SqliteFailure(err, _))
                if err.code == rusqlite::ErrorCode::ConstraintViolation =>
            {
                Err(StoreError::Duplicate {
                    kind: "workspace",
                    name: record.name.clone(),
                })
            }
            Err(e) => Err(e.into()),
        }
    }

    /// Fetch a workspace by id.
    pub fn get(&self, id: &WorkspaceId) -> Result<WorkspaceRecord, StoreError> {
        self.find_by_id(id)?.ok_or_else(|| StoreError::NotFound {
            kind: "workspace",
            id: id.to_string(),
        })
    }

    /// Fetch a workspace by id, or `None`.
    pub fn find_by_id(&self, id: &WorkspaceId) -> Result<Option<WorkspaceRecord>, StoreError> {
        Ok(self
            .conn
            .query_row(
                "SELECT * FROM workspaces WHERE id = ?1",
                [id.as_str()],
                from_row,
            )
            .optional()?)
    }

    /// Fetch a workspace by its unique name, or `None`.
    pub fn find_by_name(&self, name: &str) -> Result<Option<WorkspaceRecord>, StoreError> {
        Ok(self
            .conn
            .query_row("SELECT * FROM workspaces WHERE name = ?1", [name], from_row)
            .optional()?)
    }

    /// List workspaces, newest first. `include_removed` controls whether
    /// historical records are returned alongside live ones.
    pub fn list(&self, include_removed: bool) -> Result<Vec<WorkspaceRecord>, StoreError> {
        let sql = if include_removed {
            "SELECT * FROM workspaces ORDER BY created_at DESC"
        } else {
            "SELECT * FROM workspaces WHERE state = 'active' ORDER BY created_at DESC"
        };
        let mut stmt = self.conn.prepare(sql)?;
        let rows = stmt.query_map([], from_row)?;
        Ok(rows.collect::<Result<_, _>>()?)
    }

    /// List active workspaces belonging to one repository.
    pub fn list_for_repo(&self, repo_root: &Path) -> Result<Vec<WorkspaceRecord>, StoreError> {
        let mut stmt = self.conn.prepare(
            "SELECT * FROM workspaces
             WHERE repo_root = ?1 AND state = 'active'
             ORDER BY created_at DESC",
        )?;
        let rows = stmt.query_map([repo_root.to_string_lossy()], from_row)?;
        Ok(rows.collect::<Result<_, _>>()?)
    }

    /// Mark a workspace removed, retaining the record for history.
    pub fn mark_removed(&self, id: &WorkspaceId) -> Result<(), StoreError> {
        let changed = self.conn.execute(
            "UPDATE workspaces SET state = 'removed', removed_at = ?2 WHERE id = ?1",
            rusqlite::params![id.as_str(), Utc::now().to_rfc3339()],
        )?;
        if changed == 0 {
            return Err(StoreError::NotFound {
                kind: "workspace",
                id: id.to_string(),
            });
        }
        Ok(())
    }
}

fn from_row(row: &Row<'_>) -> rusqlite::Result<WorkspaceRecord> {
    let parse_time = |raw: String| {
        DateTime::parse_from_rfc3339(&raw)
            .map(|t| t.with_timezone(&Utc))
            .unwrap_or_else(|_| Utc::now())
    };

    Ok(WorkspaceRecord {
        id: row
            .get::<_, String>("id")?
            .parse()
            .unwrap_or_else(|_| WorkspaceId::new()),
        name: row.get("name")?,
        repo_root: PathBuf::from(row.get::<_, String>("repo_root")?),
        worktree_path: PathBuf::from(row.get::<_, String>("worktree_path")?),
        branch: row.get("branch")?,
        base_commit: row.get("base_commit")?,
        agent: row
            .get::<_, Option<String>>("agent")?
            .and_then(|a| a.parse().ok()),
        state: WorkspaceState::parse(&row.get::<_, String>("state")?),
        created_at: parse_time(row.get("created_at")?),
        removed_at: row.get::<_, Option<String>>("removed_at")?.map(parse_time),
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::Store;

    fn record(name: &str) -> WorkspaceRecord {
        WorkspaceRecord {
            id: WorkspaceId::new(),
            name: name.to_string(),
            repo_root: PathBuf::from("/repo"),
            worktree_path: PathBuf::from(format!("/repo/.devorch/{name}")),
            branch: format!("devorch/{name}"),
            base_commit: "a".repeat(40),
            agent: Some(AgentKind::Codex),
            state: WorkspaceState::Active,
            created_at: Utc::now(),
            removed_at: None,
        }
    }

    #[test]
    fn insert_and_read_back_round_trips_every_field() {
        let store = Store::open_in_memory().unwrap();
        let rec = record("alpha");
        store.workspaces().insert(&rec).unwrap();

        let loaded = store.workspaces().get(&rec.id).unwrap();
        assert_eq!(loaded.name, rec.name);
        assert_eq!(loaded.branch, rec.branch);
        assert_eq!(loaded.base_commit, rec.base_commit);
        assert_eq!(loaded.agent, Some(AgentKind::Codex));
        assert_eq!(loaded.state, WorkspaceState::Active);
        assert_eq!(loaded.worktree_path, rec.worktree_path);
    }

    #[test]
    fn duplicate_names_are_reported_as_duplicates() {
        let store = Store::open_in_memory().unwrap();
        store.workspaces().insert(&record("alpha")).unwrap();
        let err = store.workspaces().insert(&record("alpha")).unwrap_err();
        assert!(matches!(err, StoreError::Duplicate { .. }), "got {err:?}");
    }

    #[test]
    fn missing_workspace_is_not_found() {
        let store = Store::open_in_memory().unwrap();
        let err = store.workspaces().get(&WorkspaceId::new()).unwrap_err();
        assert!(matches!(err, StoreError::NotFound { .. }));
    }

    #[test]
    fn removal_hides_the_record_from_the_default_listing_but_keeps_history() {
        let store = Store::open_in_memory().unwrap();
        let rec = record("alpha");
        store.workspaces().insert(&rec).unwrap();

        store.workspaces().mark_removed(&rec.id).unwrap();

        assert!(store.workspaces().list(false).unwrap().is_empty());
        let history = store.workspaces().list(true).unwrap();
        assert_eq!(history.len(), 1);
        assert_eq!(history[0].state, WorkspaceState::Removed);
        assert!(history[0].removed_at.is_some());
    }

    #[test]
    fn listing_filters_by_repository() {
        let store = Store::open_in_memory().unwrap();
        store.workspaces().insert(&record("alpha")).unwrap();
        let mut other = record("beta");
        other.repo_root = PathBuf::from("/elsewhere");
        store.workspaces().insert(&other).unwrap();

        let mine = store
            .workspaces()
            .list_for_repo(Path::new("/repo"))
            .unwrap();
        assert_eq!(mine.len(), 1);
        assert_eq!(mine[0].name, "alpha");
    }

    #[test]
    fn find_by_name_locates_a_workspace() {
        let store = Store::open_in_memory().unwrap();
        let rec = record("alpha");
        store.workspaces().insert(&rec).unwrap();
        assert_eq!(
            store
                .workspaces()
                .find_by_name("alpha")
                .unwrap()
                .unwrap()
                .id,
            rec.id
        );
        assert!(store.workspaces().find_by_name("nope").unwrap().is_none());
    }
}
