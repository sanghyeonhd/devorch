-- Benchmarks history (local machine oriented)
-- Stores "task-complete benchmark result" to improve future routing.

CREATE TABLE IF NOT EXISTS benchmarks (
  id TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  os TEXT NOT NULL,
  arch TEXT NOT NULL,
  machine_fingerprint TEXT NOT NULL,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,

  scenario TEXT NOT NULL,          -- e.g. "quick", "code-edit", "chat", "tool-call"
  input_tokens INTEGER NOT NULL,
  output_tokens INTEGER NOT NULL,

  latency_ms INTEGER NOT NULL,
  success INTEGER NOT NULL,         -- 1/0
  error TEXT,

  quality_score REAL,               -- optional 0..1
  cost_usd REAL,                    -- optional
  metadata_json TEXT                -- optional JSON
);

CREATE INDEX IF NOT EXISTS idx_benchmarks_created_at ON benchmarks(created_at);
CREATE INDEX IF NOT EXISTS idx_benchmarks_model ON benchmarks(provider, model);
CREATE INDEX IF NOT EXISTS idx_benchmarks_machine ON benchmarks(machine_fingerprint);
