CREATE TABLE IF NOT EXISTS feedback (
  id TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,

  session_id TEXT NOT NULL,
  message_id TEXT NOT NULL,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  scenario TEXT NOT NULL,

  thumbs_up INTEGER NOT NULL,
  note TEXT,

  quality_score REAL NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_feedback_created_at ON feedback(created_at);
CREATE INDEX IF NOT EXISTS idx_feedback_model ON feedback(provider, model, scenario);
CREATE INDEX IF NOT EXISTS idx_feedback_session ON feedback(session_id);
