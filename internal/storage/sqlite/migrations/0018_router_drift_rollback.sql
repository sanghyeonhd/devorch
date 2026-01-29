-- Step 15: Drift detection + rollback snapshot

CREATE TABLE IF NOT EXISTS router_policy_snapshots (
  id TEXT PRIMARY KEY,
  scope_type TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  os TEXT NOT NULL,
  arch TEXT NOT NULL,
  scenario TEXT NOT NULL,

  -- 정책 전체를 JSON으로 저장(정확한 복구를 위해)
  policies_json TEXT NOT NULL,

  reason TEXT NOT NULL,            -- e.g. "pre-train", "manual", "pre-import"
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_router_policy_snapshots_scope
  ON router_policy_snapshots(scope_type, scope_id, os, arch, scenario, created_at);

CREATE TABLE IF NOT EXISTS router_drift_events (
  id TEXT PRIMARY KEY,
  scope_type TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  os TEXT NOT NULL,
  arch TEXT NOT NULL,
  scenario TEXT NOT NULL,

  baseline_window INTEGER NOT NULL,      -- 기준선 샘플수
  recent_window INTEGER NOT NULL,         -- 최근 샘플수

  baseline_success REAL NOT NULL,
  recent_success REAL NOT NULL,

  baseline_latency_ms REAL NOT NULL,
  recent_latency_ms REAL NOT NULL,

  baseline_cost REAL NOT NULL,
  recent_cost REAL NOT NULL,

  drift_score REAL NOT NULL,             -- 간단 합성 점수(성공률 하락 + 지연/비용 상승)
  threshold REAL NOT NULL,               -- 감지 임계값

  action TEXT NOT NULL,                  -- "none" | "rollback"
  snapshot_id TEXT,                      -- rollback 시 사용한 snapshot id
  note TEXT,

  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_router_drift_events_scope
  ON router_drift_events(scope_type, scope_id, os, arch, scenario, created_at);
