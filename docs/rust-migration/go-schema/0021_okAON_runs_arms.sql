-- 0021_okAON_runs_arms.sql
-- OkAON: 실측 러닝/벤치 데이터 (Phase 21 확장)

-- okaon_runs_v2: 간단한 실행 기록 테이블 (기존 okaon_runs와 별도)
CREATE TABLE IF NOT EXISTS okaon_runs_v2 (
  run_id            TEXT PRIMARY KEY,
  fingerprint       TEXT NOT NULL,
  category          TEXT NOT NULL,
  provider          TEXT NOT NULL,
  model             TEXT NOT NULL,

  success           INTEGER NOT NULL,
  latency_ms        INTEGER NOT NULL,
  cost_micro_usd    INTEGER NOT NULL,
  quality01         REAL NOT NULL,
  error_msg         TEXT NOT NULL,

  started_ts_utc    INTEGER NOT NULL,
  ended_ts_utc      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_runs_v2_fp_cat_time
ON okaon_runs_v2(fingerprint, category, ended_ts_utc);

CREATE INDEX IF NOT EXISTS idx_okaon_runs_v2_provider_model_time
ON okaon_runs_v2(provider, model, ended_ts_utc);

-- okaon_arm_stats_v2: 간단한 arm 통계 (기존 테이블과 별도)
CREATE TABLE IF NOT EXISTS okaon_arm_stats_v2 (
  fingerprint       TEXT NOT NULL,
  category          TEXT NOT NULL,
  provider          TEXT NOT NULL,
  model             TEXT NOT NULL,

  count_runs        INTEGER NOT NULL,
  mean_reward01     REAL NOT NULL,
  updated_ts_utc    INTEGER NOT NULL,

  PRIMARY KEY (fingerprint, category, provider, model)
);

CREATE INDEX IF NOT EXISTS idx_okaon_arm_stats_v2_fp_cat
ON okaon_arm_stats_v2(fingerprint, category);

-- okaon_rewards_v2: reward 이벤트 (기존 테이블과 별도)
CREATE TABLE IF NOT EXISTS okaon_rewards_v2 (
  id               TEXT PRIMARY KEY,
  run_id           TEXT NOT NULL,
  kind             TEXT NOT NULL,
  payload_json     TEXT NOT NULL,
  created_ts_utc   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_rewards_v2_run
ON okaon_rewards_v2(run_id, created_ts_utc);
