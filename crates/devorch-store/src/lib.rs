//! SQLite persistence.
//!
//! The store owns schema migration, workspace records and the event journal.
//! It deliberately exposes typed repositories rather than a raw connection, so
//! callers cannot invent ad-hoc SQL against the schema.

use std::path::{Path, PathBuf};

use rusqlite::Connection;

pub mod journal;
pub mod workspace;

pub use journal::EventJournal;
pub use workspace::{WorkspaceRecord, WorkspaceState, WorkspaceStore};

/// Failure modes of the persistence layer.
#[derive(Debug, thiserror::Error)]
pub enum StoreError {
    #[error("database error: {0}")]
    Sqlite(#[from] rusqlite::Error),

    #[error("could not create database directory {path}: {source}")]
    Directory {
        path: PathBuf,
        #[source]
        source: std::io::Error,
    },

    #[error("migration {name} failed: {source}")]
    Migration {
        name: &'static str,
        #[source]
        source: rusqlite::Error,
    },

    #[error("no {kind} with id {id}")]
    NotFound { kind: &'static str, id: String },

    #[error("a {kind} named {name} already exists")]
    Duplicate { kind: &'static str, name: String },

    #[error("could not decode stored json: {0}")]
    Json(#[from] serde_json::Error),
}

/// Embedded migrations, applied in order and recorded in `schema_migrations`.
const MIGRATIONS: &[(&str, &str)] = &[("0001_init", include_str!("../migrations/0001_init.sql"))];

/// An open Devorch database.
pub struct Store {
    conn: Connection,
}

impl Store {
    /// Open (creating if needed) the database at `path` and run migrations.
    pub fn open(path: impl AsRef<Path>) -> Result<Self, StoreError> {
        let path = path.as_ref();
        if let Some(parent) = path.parent() {
            if !parent.as_os_str().is_empty() {
                std::fs::create_dir_all(parent).map_err(|source| StoreError::Directory {
                    path: parent.to_path_buf(),
                    source,
                })?;
            }
        }
        Self::from_connection(Connection::open(path)?)
    }

    /// Open an in-memory database. Used by tests and dry runs.
    pub fn open_in_memory() -> Result<Self, StoreError> {
        Self::from_connection(Connection::open_in_memory()?)
    }

    fn from_connection(conn: Connection) -> Result<Self, StoreError> {
        // WAL keeps the daemon readable while a mission writes; foreign keys are
        // off by default in SQLite and must be enabled per connection.
        conn.pragma_update(None, "journal_mode", "WAL")?;
        conn.pragma_update(None, "foreign_keys", "ON")?;
        conn.pragma_update(None, "busy_timeout", 5_000)?;

        let store = Self { conn };
        store.migrate()?;
        Ok(store)
    }

    /// Apply any migrations this database has not seen yet.
    fn migrate(&self) -> Result<(), StoreError> {
        self.conn.execute_batch(
            "CREATE TABLE IF NOT EXISTS schema_migrations (
                 name       TEXT PRIMARY KEY,
                 applied_at TEXT NOT NULL
             );",
        )?;

        for (name, sql) in MIGRATIONS {
            let applied: i64 = self.conn.query_row(
                "SELECT COUNT(*) FROM schema_migrations WHERE name = ?1",
                [name],
                |row| row.get(0),
            )?;
            if applied > 0 {
                continue;
            }
            self.conn
                .execute_batch(sql)
                .map_err(|source| StoreError::Migration { name, source })?;
            self.conn.execute(
                "INSERT INTO schema_migrations (name, applied_at) VALUES (?1, ?2)",
                rusqlite::params![name, chrono::Utc::now().to_rfc3339()],
            )?;
            tracing::info!(migration = name, "applied migration");
        }
        Ok(())
    }

    /// Names of the migrations applied to this database, in order.
    pub fn applied_migrations(&self) -> Result<Vec<String>, StoreError> {
        let mut stmt = self
            .conn
            .prepare("SELECT name FROM schema_migrations ORDER BY name")?;
        let rows = stmt.query_map([], |row| row.get::<_, String>(0))?;
        Ok(rows.collect::<Result<_, _>>()?)
    }

    /// Workspace records.
    pub fn workspaces(&self) -> WorkspaceStore<'_> {
        WorkspaceStore::new(&self.conn)
    }

    /// The domain event journal.
    pub fn journal(&self) -> EventJournal<'_> {
        EventJournal::new(&self.conn)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn migrations_apply_once_and_are_idempotent() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("devorch3.db");

        let store = Store::open(&path).unwrap();
        assert_eq!(store.applied_migrations().unwrap(), vec!["0001_init"]);
        drop(store);

        // Reopening must not re-run migrations or fail.
        let store = Store::open(&path).unwrap();
        assert_eq!(store.applied_migrations().unwrap(), vec!["0001_init"]);
    }

    #[test]
    fn open_creates_missing_parent_directories() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("nested/deeper/devorch3.db");
        Store::open(&path).expect("open");
        assert!(path.exists());
    }
}
