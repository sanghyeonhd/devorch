CREATE TABLE IF NOT EXISTS auth_tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  workspace_id TEXT NOT NULL,
  provider TEXT NOT NULL,              -- github/google/openai/anthropic/custom_oidc...
  access_token TEXT NOT NULL,
  refresh_token TEXT,
  token_type TEXT,
  expiry_unix INTEGER,                 -- unix seconds (0 if unknown)
  scope TEXT,
  id_token TEXT,                       -- OIDC optional
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(workspace_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_workspace
ON auth_tokens(workspace_id);

CREATE TRIGGER IF NOT EXISTS trg_auth_tokens_updated
AFTER UPDATE ON auth_tokens
BEGIN
  UPDATE auth_tokens SET updated_at=CURRENT_TIMESTAMP WHERE id=NEW.id;
END;
