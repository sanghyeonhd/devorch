CREATE TABLE IF NOT EXISTS quality_runs (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,
  session_id TEXT,
  task_id TEXT,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  success INTEGER NOT NULL DEFAULT 1,
  score REAL NOT NULL DEFAULT 0.0,
  latency_ms INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  error TEXT,
  notes TEXT
);

CREATE INDEX IF NOT EXISTS idx_quality_runs_ws_time ON quality_runs(workspace, created_at);
CREATE INDEX IF NOT EXISTS idx_quality_runs_provider_model ON quality_runs(provider, model, created_at);
