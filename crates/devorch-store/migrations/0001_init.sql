-- Devorch 3 Rust core schema.
--
-- This is a new database namespace (`devorch3.db` by default) rather than an
-- in-place edit of the Go schema. The 22 Go migrations remain untouched so the
-- existing OkAON data survives; the schema map records how they are folded in.

CREATE TABLE IF NOT EXISTS workspaces (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL UNIQUE,
    repo_root     TEXT NOT NULL,
    worktree_path TEXT NOT NULL,
    branch        TEXT NOT NULL,
    base_commit   TEXT NOT NULL,
    agent         TEXT,
    state         TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    removed_at    TEXT
);

CREATE INDEX IF NOT EXISTS idx_workspaces_state ON workspaces (state);
CREATE INDEX IF NOT EXISTS idx_workspaces_repo ON workspaces (repo_root);

-- Low-volume domain events only. High-volume material (terminal chunks, raw
-- model streams, screenshots) belongs in bounded blob storage, not in a row
-- per byte.
CREATE TABLE IF NOT EXISTS event_journal (
    id           TEXT PRIMARY KEY,
    session_id   TEXT NOT NULL,
    seq          INTEGER NOT NULL,
    timestamp    TEXT NOT NULL,
    mission_id   TEXT,
    task_id      TEXT,
    workspace_id TEXT,
    event_type   TEXT NOT NULL,
    payload      TEXT NOT NULL,
    UNIQUE (session_id, seq)
);

CREATE INDEX IF NOT EXISTS idx_events_session ON event_journal (session_id, seq);
CREATE INDEX IF NOT EXISTS idx_events_workspace ON event_journal (workspace_id);
