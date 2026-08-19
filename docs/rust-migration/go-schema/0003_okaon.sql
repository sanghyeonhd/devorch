-- OkAON: Outcome-KPI-Aware Orchestration Network (local learning store)

CREATE TABLE IF NOT EXISTS okaon_runs (
  run_id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,

  policy_key TEXT NOT NULL,

  workspace_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  project_id TEXT NOT NULL,

  agent_type TEXT NOT NULL,
  category TEXT NOT NULL,
  task_type TEXT NOT NULL,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  endpoint TEXT NOT NULL,

  latency_ms INTEGER NOT NULL,
  ok INTEGER NOT NULL, -- 0/1

  -- quality estimation: 0~1
  quality REAL NOT NULL,

  -- minimal token/cost fields
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  cost_micro_usd INTEGER NOT NULL DEFAULT 0,

  -- trace references (optional)
  trace_id TEXT,
  error_code TEXT,
  error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_okaon_policy_time
ON okaon_runs(policy_key, created_at);

CREATE INDEX IF NOT EXISTS idx_okaon_provider_model_time
ON okaon_runs(provider, model, created_at);
