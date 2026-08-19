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
const MIGRATIONS: &[(&str, &str)] = &[
    ("0001_init", include_str!("../migrations/0001_init.sql")),
    (
        "0002_documents",
        include_str!("../migrations/0002_documents.sql"),
    ),
];

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

    /// Insert or replace a JSON document of `kind`.
    ///
    /// `created_at` is preserved across updates so a record's age survives being
    /// rewritten as a mission progresses.
    pub fn put_document<T: serde::Serialize>(
        &self,
        kind: &str,
        id: &str,
        value: &T,
    ) -> Result<(), StoreError> {
        let payload = serde_json::to_string(value)?;
        let now = chrono::Utc::now().to_rfc3339();
        self.conn.execute(
            "INSERT INTO documents (kind, id, payload, created_at, updated_at)
             VALUES (?1, ?2, ?3, ?4, ?4)
             ON CONFLICT (kind, id) DO UPDATE SET payload = ?3, updated_at = ?4",
            rusqlite::params![kind, id, payload, now],
        )?;
        Ok(())
    }

    /// Read one document, or `None` if it is absent.
    pub fn get_document<T: serde::de::DeserializeOwned>(
        &self,
        kind: &str,
        id: &str,
    ) -> Result<Option<T>, StoreError> {
        use rusqlite::OptionalExtension;
        let payload: Option<String> = self
            .conn
            .query_row(
                "SELECT payload FROM documents WHERE kind = ?1 AND id = ?2",
                [kind, id],
                |row| row.get(0),
            )
            .optional()?;
        match payload {
            Some(payload) => Ok(Some(serde_json::from_str(&payload)?)),
            None => Ok(None),
        }
    }

    /// Every document of `kind`, newest first.
    pub fn list_documents<T: serde::de::DeserializeOwned>(
        &self,
        kind: &str,
    ) -> Result<Vec<T>, StoreError> {
        let mut stmt = self
            .conn
            .prepare("SELECT payload FROM documents WHERE kind = ?1 ORDER BY created_at DESC")?;
        let rows = stmt.query_map([kind], |row| row.get::<_, String>(0))?;

        let mut out = Vec::new();
        for payload in rows {
            out.push(serde_json::from_str(&payload?)?);
        }
        Ok(out)
    }

    /// Delete one document, reporting whether it existed.
    pub fn delete_document(&self, kind: &str, id: &str) -> Result<bool, StoreError> {
        let removed = self.conn.execute(
            "DELETE FROM documents WHERE kind = ?1 AND id = ?2",
            [kind, id],
        )?;
        Ok(removed > 0)
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
        assert_eq!(
            store.applied_migrations().unwrap(),
            vec!["0001_init", "0002_documents"]
        );
        drop(store);

        // Reopening must not re-run migrations or fail.
        let store = Store::open(&path).unwrap();
        assert_eq!(
            store.applied_migrations().unwrap(),
            vec!["0001_init", "0002_documents"]
        );
    }

    #[test]
    fn documents_round_trip_and_update_in_place() {
        #[derive(serde::Serialize, serde::Deserialize, PartialEq, Debug)]
        struct Doc {
            goal: String,
            status: String,
        }

        let store = Store::open_in_memory().unwrap();
        let first = Doc {
            goal: "fix login".into(),
            status: "running".into(),
        };
        store.put_document("mission", "m1", &first).unwrap();
        assert_eq!(
            store.get_document::<Doc>("mission", "m1").unwrap(),
            Some(first)
        );

        let updated = Doc {
            goal: "fix login".into(),
            status: "succeeded".into(),
        };
        store.put_document("mission", "m1", &updated).unwrap();

        let all = store.list_documents::<Doc>("mission").unwrap();
        assert_eq!(all.len(), 1, "an update must not create a second row");
        assert_eq!(all[0].status, "succeeded");
    }

    #[test]
    fn document_kinds_are_isolated_from_each_other() {
        let store = Store::open_in_memory().unwrap();
        store.put_document("mission", "x", &"a").unwrap();
        store.put_document("plan", "x", &"b").unwrap();

        assert_eq!(
            store.get_document::<String>("mission", "x").unwrap(),
            Some("a".to_string())
        );
        assert_eq!(store.list_documents::<String>("plan").unwrap().len(), 1);
    }

    #[test]
    fn a_missing_document_is_none_and_deleting_it_says_so() {
        let store = Store::open_in_memory().unwrap();
        assert_eq!(
            store.get_document::<String>("mission", "nope").unwrap(),
            None
        );
        assert!(!store.delete_document("mission", "nope").unwrap());

        store.put_document("mission", "x", &"a").unwrap();
        assert!(store.delete_document("mission", "x").unwrap());
    }

    #[test]
    fn documents_survive_reopening_the_database() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("devorch3.db");
        {
            let store = Store::open(&path).unwrap();
            store.put_document("mission", "m1", &"persisted").unwrap();
        }
        let store = Store::open(&path).unwrap();
        assert_eq!(
            store.get_document::<String>("mission", "m1").unwrap(),
            Some("persisted".to_string())
        );
    }

    #[test]
    fn open_creates_missing_parent_directories() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("nested/deeper/devorch3.db");
        Store::open(&path).expect("open");
        assert!(path.exists());
    }
}
