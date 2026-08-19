CREATE TABLE IF NOT EXISTS router_policy (
  id INTEGER PRIMARY KEY AUTOINCREMENT,

  os TEXT NOT NULL,
  arch TEXT NOT NULL,
  scenario TEXT NOT NULL,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,

  weight REAL NOT NULL,         -- 선택 가중치
  success_rate REAL NOT NULL,
  avg_latency_ms REAL NOT NULL,
  avg_cost REAL NOT NULL,
  sample_count INTEGER NOT NULL,

  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  
  -- Added from 0016 migration
  scope_type TEXT NOT NULL DEFAULT 'global',
  scope_id   TEXT NOT NULL DEFAULT '*',

  UNIQUE(os, arch, scenario, provider, model)
);

CREATE INDEX IF NOT EXISTS idx_router_policy_scope_key
  ON router_policy(scope_type, scope_id, os, arch, scenario);
