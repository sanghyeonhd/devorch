-- 0021_okAON_runs_arms.sql
-- OkAON: 실측 러닝/벤치 데이터

CREATE TABLE IF NOT EXISTS okaon_runs (
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

CREATE INDEX IF NOT EXISTS idx_okaon_runs_fp_cat_time
ON okaon_runs(fingerprint, category, ended_ts_utc);

CREATE INDEX IF NOT EXISTS idx_okaon_runs_provider_model_time
ON okaon_runs(provider, model, ended_ts_utc);

-- arm stats: fingerprint+category 기준에서 provider/model arm 성능 통계
CREATE TABLE IF NOT EXISTS okaon_arm_stats (
  fingerprint       TEXT NOT NULL,
  category          TEXT NOT NULL,
  provider          TEXT NOT NULL,
  model             TEXT NOT NULL,

  count_runs        INTEGER NOT NULL,
  mean_reward01     REAL NOT NULL,
  updated_ts_utc    INTEGER NOT NULL,

  PRIMARY KEY (fingerprint, category, provider, model)
);

CREATE INDEX IF NOT EXISTS idx_okaon_arm_stats_fp_cat
ON okaon_arm_stats(fingerprint, category);

-- (선택) reward 이벤트(모델스위치/리트라이소진 등) 저장용
CREATE TABLE IF NOT EXISTS okaon_rewards (
  id               TEXT PRIMARY KEY,
  run_id           TEXT NOT NULL,
  kind             TEXT NOT NULL,   -- model_switch | retry_exhausted | ...
  payload_json     TEXT NOT NULL,
  created_ts_utc   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_rewards_run
ON okaon_rewards(run_id, created_ts_utc);
