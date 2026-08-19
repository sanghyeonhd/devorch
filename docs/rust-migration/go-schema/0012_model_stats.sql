CREATE TABLE IF NOT EXISTS model_stats (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,

  samples INTEGER NOT NULL DEFAULT 0,
  avg_score REAL NOT NULL DEFAULT 0.0,
  avg_latency_ms REAL NOT NULL DEFAULT 0.0,
  fail_rate REAL NOT NULL DEFAULT 0.0,

  updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_model_stats_ws_model
ON model_stats(workspace, provider, model);
