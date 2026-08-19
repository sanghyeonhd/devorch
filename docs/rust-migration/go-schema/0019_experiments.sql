-- Step 17: Prompt/Policy Experiments (A/B) + Auto Winner

CREATE TABLE IF NOT EXISTS experiments (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,              -- "prompt" | "routing" | "policy"
  status TEXT NOT NULL,            -- "draft" | "running" | "stopped" | "completed"
  strategy TEXT NOT NULL,          -- "ab" | "epsilon_greedy"
  traffic REAL NOT NULL DEFAULT 1.0,  -- 0.0 ~ 1.0
  seed TEXT NOT NULL,              -- sticky assignment seed
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_experiments_status
  ON experiments(status);

CREATE TABLE IF NOT EXISTS experiment_variants (
  id TEXT PRIMARY KEY,
  experiment_id TEXT NOT NULL,

  key TEXT NOT NULL,               -- "A", "B", "C"...
  weight REAL NOT NULL DEFAULT 0.5,

  -- payload is JSON: prompt template / routing rules / policy knobs
  payload_json TEXT NOT NULL,

  is_winner INTEGER NOT NULL DEFAULT 0,

  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(experiment_id) REFERENCES experiments(id)
);

CREATE INDEX IF NOT EXISTS idx_variants_experiment
  ON experiment_variants(experiment_id);

CREATE TABLE IF NOT EXISTS experiment_assignments (
  id TEXT PRIMARY KEY,
  experiment_id TEXT NOT NULL,

  subject TEXT NOT NULL,          -- user_id | workspace_id | project_id | session_id
  variant_key TEXT NOT NULL,      -- "A"/"B"/...

  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

  UNIQUE(experiment_id, subject),

  FOREIGN KEY(experiment_id) REFERENCES experiments(id)
);

CREATE INDEX IF NOT EXISTS idx_assignments_experiment
  ON experiment_assignments(experiment_id);

-- Outcome metrics per "bench_run_id" or "session/message"
CREATE TABLE IF NOT EXISTS experiment_outcomes (
  id TEXT PRIMARY KEY,
  experiment_id TEXT NOT NULL,
  variant_key TEXT NOT NULL,

  bench_run_id TEXT,              -- 연결 가능한 경우
  session_id TEXT,
  message_id TEXT,

  -- core metrics
  success INTEGER NOT NULL,       -- 1=success, 0=failure
  latency_ms INTEGER NOT NULL,
  cost_microusd INTEGER NOT NULL,
  quality_score REAL NOT NULL,    -- 0~1 normalized
  tokens_in INTEGER NOT NULL,
  tokens_out INTEGER NOT NULL,

  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(experiment_id) REFERENCES experiments(id)
);

CREATE INDEX IF NOT EXISTS idx_outcomes_experiment_variant
  ON experiment_outcomes(experiment_id, variant_key);

CREATE TABLE IF NOT EXISTS experiment_decisions (
  id TEXT PRIMARY KEY,
  experiment_id TEXT NOT NULL,

  decided_variant_key TEXT NOT NULL,
  reason TEXT NOT NULL,           -- json or text
  decided_at DATETIME DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(experiment_id) REFERENCES experiments(id)
);
