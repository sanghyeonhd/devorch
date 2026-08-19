-- Generic JSON document storage.
--
-- Missions, comparisons and routing plans are read as whole objects and never
-- queried field-by-field, so they are stored as documents rather than as a
-- table per aggregate. Anything that *is* queried by field — workspaces, the
-- event journal — keeps its own columns.

CREATE TABLE IF NOT EXISTS documents (
    kind       TEXT NOT NULL,
    id         TEXT NOT NULL,
    payload    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (kind, id)
);

CREATE INDEX IF NOT EXISTS idx_documents_kind_created
    ON documents (kind, created_at DESC);
