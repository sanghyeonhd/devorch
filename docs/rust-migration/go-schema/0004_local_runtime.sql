CREATE TABLE IF NOT EXISTS local_runtime_inventory (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,

  os TEXT NOT NULL,
  arch TEXT NOT NULL,

  runtime TEXT NOT NULL,          -- ex: "ollama"
  runtime_version TEXT,

  runtime_path TEXT,              -- installed binary path
  host TEXT,                      -- if remote daemon used (ollama host)

  cpu_cores INTEGER NOT NULL DEFAULT 0,
  mem_mb INTEGER NOT NULL DEFAULT 0,

  gpu_hint TEXT                  -- optional future
);

CREATE INDEX IF NOT EXISTS idx_local_runtime_inventory_time
ON local_runtime_inventory(created_at);
