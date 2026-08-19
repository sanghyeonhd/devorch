CREATE TABLE IF NOT EXISTS test_runs (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,
  project_id TEXT,
  session_id TEXT,
  task_id TEXT,

  runner TEXT NOT NULL,
  command TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at INTEGER NOT NULL,
  duration_ms INTEGER NOT NULL,

  success INTEGER NOT NULL DEFAULT 1,
  exit_code INTEGER NOT NULL DEFAULT 0,
  error TEXT,

  stdout_summary TEXT,
  stderr_summary TEXT,
  tags TEXT
);

CREATE INDEX IF NOT EXISTS idx_test_runs_ws_time ON test_runs(workspace, started_at);
CREATE INDEX IF NOT EXISTS idx_test_runs_runner ON test_runs(runner, started_at);
