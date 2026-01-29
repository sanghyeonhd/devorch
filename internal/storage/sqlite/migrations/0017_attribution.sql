-- Step 16: Attribution + Environment Signature

CREATE TABLE IF NOT EXISTS env_signatures (
  id TEXT PRIMARY KEY,

  os TEXT NOT NULL,
  arch TEXT NOT NULL,

  cpu_model TEXT,
  cpu_cores INTEGER,

  gpu_model TEXT,
  gpu_driver TEXT,
  gpu_backend TEXT,      -- cuda | metal | rocm | cpu

  ram_gb REAL,

  runtime_version TEXT, -- devorch version
  ollama_version TEXT,
  llama_cpp_version TEXT,

  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_env_signatures_fingerprint
  ON env_signatures(os, arch, cpu_model, gpu_model, gpu_driver, gpu_backend);

CREATE TABLE IF NOT EXISTS attribution_events (
  id TEXT PRIMARY KEY,

  bench_run_id TEXT NOT NULL,
  env_signature_id TEXT NOT NULL,

  model TEXT NOT NULL,
  provider TEXT NOT NULL,

  prompt_hash TEXT NOT NULL,
  prompt_version TEXT,

  toolchain_hash TEXT,      -- compiler/build/runtime hash
  runtime_flags TEXT,       -- JSON

  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(env_signature_id) REFERENCES env_signatures(id)
);

CREATE INDEX IF NOT EXISTS idx_attribution_bench
  ON attribution_events(bench_run_id);
