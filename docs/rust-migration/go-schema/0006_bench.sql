CREATE TABLE IF NOT EXISTS bench_runs (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,
  session_id TEXT,
  task_id TEXT,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at INTEGER NOT NULL,
  latency_ms INTEGER NOT NULL,
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  error TEXT
);

CREATE INDEX IF NOT EXISTS idx_bench_runs_ws_time ON bench_runs(workspace, started_at);
CREATE INDEX IF NOT EXISTS idx_bench_runs_provider_model ON bench_runs(provider, model, started_at);
