-- 0020_okAON_learning.sql
-- OkAON: 로컬 실측 기반 데이터 레이크 + 학습 입력 데이터
-- 목표: (task -> model 선택 -> 결과 품질/지연/비용) 을 축적하여 router/learning이 반영

PRAGMA foreign_keys = ON;

-- 작업/요청 단위 (Work Item)
CREATE TABLE IF NOT EXISTS okaon_work (
  work_id            TEXT PRIMARY KEY,            -- ULID
  ts_utc             INTEGER NOT NULL,            -- unix epoch seconds
  project_id         TEXT NOT NULL,
  workspace_id       TEXT NOT NULL,
  user_id            TEXT NOT NULL,

  task_type          TEXT NOT NULL,               -- e.g. "code_edit", "search", "chat", "refactor"
  category           TEXT NOT NULL,               -- e.g. "ultrabrain", "quick", "visual-engineering"
  agent              TEXT NOT NULL,               -- e.g. "atlas", "oracle", ...
  prompt_hash        TEXT NOT NULL,               -- privacy-safe fingerprint
  context_hash       TEXT NOT NULL,               -- privacy-safe fingerprint
  os                 TEXT NOT NULL,               -- darwin/windows/linux
  arch               TEXT NOT NULL,               -- amd64/arm64
  hw_fingerprint     TEXT NOT NULL,               -- coarse fingerprint (no PII)
  tags_json          TEXT NOT NULL DEFAULT '{}'    -- JSON string
);

CREATE INDEX IF NOT EXISTS idx_okaon_work_ts
ON okaon_work(ts_utc);

CREATE INDEX IF NOT EXISTS idx_okaon_work_project
ON okaon_work(project_id, ts_utc);

-- 실제 실행(모델/프로바이더 선택 결과)
CREATE TABLE IF NOT EXISTS okaon_run (
  run_id             TEXT PRIMARY KEY,            -- ULID
  work_id            TEXT NOT NULL,
  ts_utc             INTEGER NOT NULL,

  provider           TEXT NOT NULL,               -- openai/anthropic/google/ollama/...
  model              TEXT NOT NULL,               -- full model id
  route_policy       TEXT NOT NULL,               -- router policy name/version
  was_fallback       INTEGER NOT NULL,            -- 0/1
  retry_count        INTEGER NOT NULL,

  latency_ms         INTEGER NOT NULL,
  input_tokens       INTEGER NOT NULL,
  output_tokens      INTEGER NOT NULL,
  cost_microusd      INTEGER NOT NULL,            -- 비용 없는 로컬은 0

  error_code         TEXT NOT NULL DEFAULT '',
  error_msg          TEXT NOT NULL DEFAULT '',

  FOREIGN KEY(work_id) REFERENCES okaon_work(work_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_okaon_run_work
ON okaon_run(work_id);

CREATE INDEX IF NOT EXISTS idx_okaon_run_model_ts
ON okaon_run(model, ts_utc);

-- 품질 평가(품질 게이트/유저 피드백/테스트 통과 등)
CREATE TABLE IF NOT EXISTS okaon_quality (
  quality_id         TEXT PRIMARY KEY,            -- ULID
  run_id             TEXT NOT NULL,
  ts_utc             INTEGER NOT NULL,

  -- score: 0~1
  quality_score      REAL NOT NULL,
  -- optional components
  correctness        REAL NOT NULL DEFAULT 0,
  style              REAL NOT NULL DEFAULT 0,
  safety             REAL NOT NULL DEFAULT 0,

  evaluator          TEXT NOT NULL,               -- "qa_gate", "unit_test", "human", "lint", ...
  notes              TEXT NOT NULL DEFAULT '',

  FOREIGN KEY(run_id) REFERENCES okaon_run(run_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_okaon_quality_run
ON okaon_quality(run_id);

CREATE INDEX IF NOT EXISTS idx_okaon_quality_ts
ON okaon_quality(ts_utc);

-- 학습용 reward 이벤트(밴딧/회귀 입력)
CREATE TABLE IF NOT EXISTS okaon_reward (
  reward_id          TEXT PRIMARY KEY,            -- ULID
  run_id             TEXT NOT NULL,
  ts_utc             INTEGER NOT NULL,

  reward             REAL NOT NULL,               -- signed reward, e.g. [-1, +1]
  reward_type        TEXT NOT NULL,               -- "quality", "latency", "cost", "composite"
  weight             REAL NOT NULL DEFAULT 1.0,   -- 가중치

  FOREIGN KEY(run_id) REFERENCES okaon_run(run_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_okaon_reward_run
ON okaon_reward(run_id);

-- 모델별/컨텍스트별 팔(arm) 통계(스냅샷)
CREATE TABLE IF NOT EXISTS okaon_arm_stats (
  arm_key            TEXT PRIMARY KEY,            -- e.g. hash(context)+model
  updated_ts_utc      INTEGER NOT NULL,

  alpha              REAL NOT NULL,               -- Beta distribution params
  beta               REAL NOT NULL,
  pulls              INTEGER NOT NULL,
  last_reward        REAL NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_arm_stats_updated
ON okaon_arm_stats(updated_ts_utc);
