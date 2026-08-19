CREATE TABLE IF NOT EXISTS usage_ledger (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,

  workspace_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  project_id TEXT NOT NULL,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,

  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,

  cost_micro_usd INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_usage_ledger_time
ON usage_ledger(created_at);
