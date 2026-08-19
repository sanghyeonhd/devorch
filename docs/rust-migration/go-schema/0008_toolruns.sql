CREATE TABLE IF NOT EXISTS tool_runs (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,
  session_id TEXT,
  task_id TEXT,

  tool_name TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at INTEGER NOT NULL,
  duration_ms INTEGER NOT NULL,

  success INTEGER NOT NULL DEFAULT 1,
  error TEXT,

  input_summary TEXT,
  output_summary TEXT,

  output_bytes INTEGER NOT NULL DEFAULT 0,
  tags TEXT,
  
  -- Added from 0011 migration
  exit_code INTEGER NOT NULL DEFAULT 0,
  meta_json TEXT
);

CREATE INDEX IF NOT EXISTS idx_tool_runs_ws_time ON tool_runs(workspace, started_at);
CREATE INDEX IF NOT EXISTS idx_tool_runs_tool ON tool_runs(tool_name, started_at);
