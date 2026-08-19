CREATE TABLE IF NOT EXISTS auth_sessions (
  id TEXT PRIMARY KEY,              -- session id (ulid)
  user_id TEXT NOT NULL,
  workspace TEXT NOT NULL,

  access_token_hash TEXT NOT NULL,  -- bearer hash
  expires_at INTEGER NOT NULL,       -- unix ms

  email TEXT,
  provider TEXT,                    -- "github", "openai", "google", etc
  created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_token ON auth_sessions(access_token_hash);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_ws_user ON auth_sessions(workspace, user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_exp ON auth_sessions(expires_at);
