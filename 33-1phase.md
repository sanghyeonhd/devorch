좋아요. 네트워크 끊긴 지점부터 **“Self-Improving(OkAON + Bandit + Router 연동)” 코어를 실제로 돌아가게 만드는 풀코드(핵심 파일 묶음)**를 이어서 드립니다.
아래는 **(33단계 기준) “OkAON SQLite 스키마 + CRUD + Bandit(Thompson) + Router 어댑터 + Hook(학습 피드백 주입)”**까지 한 번에 붙여넣어 개발 가능한 단위로 구성했습니다.
(전체 트리 전체를 한 번에 다 풀코드로 내리면 분량/리스크가 너무 커서, 지금 단계는 “학습 엔진이 실제로 돌아가는 최소 완성 세트”를 먼저 완성합니다.)
0) 추가/수정 파일 목록 (이번 단계)
internal/storage/sqlite/sqlite.go
internal/storage/sqlite/migrations/0003_okAON_learning.sql
internal/okAON/*.go
internal/okAON/sqlite/*.go
internal/learning/*.go
internal/learning/bandit/*.go
internal/router/learning_adapter.go
internal/router/reward_adjust.go
internal/router/coldstart.go
internal/router/explain.go
internal/hook/builtins/on_task_complete.go
internal/hook/builtins/on_quality_gate.go
internal/hook/builtins/on_model_switch.go
internal/hook/builtins/on_retry_exhausted.go
1) SQLite 공통 헬퍼
internal/storage/sqlite/sqlite.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	SQL *sql.DB
}

type OpenOptions struct {
	Path            string
	BusyTimeout     time.Duration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func Open(ctx context.Context, opt OpenOptions) (*DB, error) {
	if opt.Path == "" {
		return nil, errors.New("sqlite: Path is required")
	}
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", opt.Path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if opt.BusyTimeout <= 0 {
		opt.BusyTimeout = 10 * time.Second
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d;", opt.BusyTimeout.Milliseconds())); err != nil {
		_ = db.Close()
		return nil, err
	}

	if opt.MaxOpenConns <= 0 {
		opt.MaxOpenConns = 10
	}
	if opt.MaxIdleConns <= 0 {
		opt.MaxIdleConns = 10
	}
	db.SetMaxOpenConns(opt.MaxOpenConns)
	db.SetMaxIdleConns(opt.MaxIdleConns)
	if opt.ConnMaxLifetime <= 0 {
		opt.ConnMaxLifetime = 30 * time.Minute
	}
	db.SetConnMaxLifetime(opt.ConnMaxLifetime)

	// quick ping
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(cctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &DB{SQL: db}, nil
}

func (d *DB) Close() error {
	if d == nil || d.SQL == nil {
		return nil
	}
	return d.SQL.Close()
}
2) OkAON + Learning 스키마 마이그레이션
internal/storage/sqlite/migrations/0003_okAON_learning.sql
코드 복사
Sql
-- OkAON (Observed kAON) + Learning schema
-- Purpose:
--  - store real machine benchmark results, task outcomes, rewards, and router decisions
--  - enable bandit/regression based policy improvements over time

PRAGMA foreign_keys = ON;

-- 1) Environment fingerprint (machine + OS + runtime)
CREATE TABLE IF NOT EXISTS okaon_env (
  env_id TEXT PRIMARY KEY,              -- ULID/UUID string
  created_at INTEGER NOT NULL,           -- unix ms
  os TEXT NOT NULL,                      -- darwin/windows/linux
  arch TEXT NOT NULL,                    -- amd64/arm64
  cpu_brand TEXT,
  cpu_cores INTEGER,
  ram_mb INTEGER,
  gpu_brand TEXT,
  gpu_vram_mb INTEGER,
  disk_type TEXT,
  runtime_stack TEXT,                    -- e.g. "ollama+llamacpp"
  notes TEXT
);

CREATE INDEX IF NOT EXISTS idx_okaon_env_os_arch ON okaon_env(os, arch);

-- 2) Model inventory (local or remote)
CREATE TABLE IF NOT EXISTS okaon_model (
  model_id TEXT PRIMARY KEY,             -- canonical id e.g. "openai/gpt-5.2", "ollama/llama3.1:8b"
  provider TEXT NOT NULL,                -- openai/anthropic/google/ollama/openrouter/...
  family TEXT,                           -- gpt/claude/gemini/llama/qwen...
  context_window INTEGER,
  is_local INTEGER NOT NULL DEFAULT 0,   -- 0/1
  created_at INTEGER NOT NULL,
  meta_json TEXT                         -- json string
);

CREATE INDEX IF NOT EXISTS idx_okaon_model_provider ON okaon_model(provider);

-- 3) Workloads (task category + signature)
CREATE TABLE IF NOT EXISTS okaon_workload (
  workload_id TEXT PRIMARY KEY,          -- stable hash/ulid
  created_at INTEGER NOT NULL,
  category TEXT NOT NULL,                -- quick/ultrabrain/visual-engineering/...
  signature TEXT NOT NULL,               -- stable string (hash inputs, repo, toolset)
  repo_lang TEXT,
  toolset TEXT,
  notes TEXT
);

CREATE INDEX IF NOT EXISTS idx_okaon_workload_category ON okaon_workload(category);

-- 4) Runs (a single execution attempt on a model for a workload)
CREATE TABLE IF NOT EXISTS okaon_run (
  run_id TEXT PRIMARY KEY,               -- ULID
  created_at INTEGER NOT NULL,
  env_id TEXT NOT NULL,
  workload_id TEXT NOT NULL,
  model_id TEXT NOT NULL,
  route_reason TEXT,                     -- why selected
  latency_ms INTEGER NOT NULL,
  input_tokens INTEGER,
  output_tokens INTEGER,
  cost_micros INTEGER,                   -- monetary cost in micros (if remote)
  success INTEGER NOT NULL DEFAULT 0,     -- 0/1
  quality_score REAL,                    -- 0..1 or any scale
  reward REAL,                           -- final reward used for learning
  error_type TEXT,
  error_msg TEXT,
  FOREIGN KEY(env_id) REFERENCES okaon_env(env_id) ON DELETE CASCADE,
  FOREIGN KEY(workload_id) REFERENCES okaon_workload(workload_id) ON DELETE CASCADE,
  FOREIGN KEY(model_id) REFERENCES okaon_model(model_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_okaon_run_env ON okaon_run(env_id, created_at);
CREATE INDEX IF NOT EXISTS idx_okaon_run_workload ON okaon_run(workload_id, created_at);
CREATE INDEX IF NOT EXISTS idx_okaon_run_model ON okaon_run(model_id, created_at);

-- 5) Bandit arms state (per (env, workload, model))
CREATE TABLE IF NOT EXISTS okaon_bandit_arm (
  arm_key TEXT PRIMARY KEY,              -- env_id|workload_id|model_id
  env_id TEXT NOT NULL,
  workload_id TEXT NOT NULL,
  model_id TEXT NOT NULL,
  algo TEXT NOT NULL,                    -- thompson/ucb/eps
  n INTEGER NOT NULL DEFAULT 0,
  sum_reward REAL NOT NULL DEFAULT 0.0,
  sum_reward2 REAL NOT NULL DEFAULT 0.0, -- for variance
  alpha REAL NOT NULL DEFAULT 1.0,       -- beta prior alpha (if used)
  beta REAL NOT NULL DEFAULT 1.0,        -- beta prior beta  (if used)
  last_updated INTEGER NOT NULL,
  FOREIGN KEY(env_id) REFERENCES okaon_env(env_id) ON DELETE CASCADE,
  FOREIGN KEY(workload_id) REFERENCES okaon_workload(workload_id) ON DELETE CASCADE,
  FOREIGN KEY(model_id) REFERENCES okaon_model(model_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_okaon_arm_env_workload ON okaon_bandit_arm(env_id, workload_id);

-- 6) Policy snapshots (versioned, rollbackable)
CREATE TABLE IF NOT EXISTS okaon_policy_snapshot (
  snapshot_id TEXT PRIMARY KEY,          -- ULID
  created_at INTEGER NOT NULL,
  env_id TEXT NOT NULL,
  category TEXT NOT NULL,
  policy_json TEXT NOT NULL,             -- full policy / weights / router parameters
  notes TEXT,
  FOREIGN KEY(env_id) REFERENCES okaon_env(env_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_okaon_policy_env_cat ON okaon_policy_snapshot(env_id, category, created_at);

-- 7) Retention / vacuum markers
CREATE TABLE IF NOT EXISTS okaon_retention_state (
  key TEXT PRIMARY KEY,
  updated_at INTEGER NOT NULL,
  value TEXT
);
3) OkAON 도메인 모델 + Store 인터페이스
internal/okAON/models.go
코드 복사
Go
package okaon

import "time"

type Env struct {
	EnvID        string
	CreatedAt    time.Time
	OS           string
	Arch         string
	CPUBrand     string
	CPUCores     int
	RAMMB        int
	GPUBrand     string
	GPUVRAMMB    int
	DiskType     string
	RuntimeStack string
	Notes        string
}

type Model struct {
	ModelID        string
	Provider       string
	Family         string
	ContextWindow  int
	IsLocal        bool
	CreatedAt      time.Time
	MetaJSON       string
}

type Workload struct {
	WorkloadID string
	CreatedAt   time.Time
	Category    string
	Signature   string
	RepoLang    string
	Toolset     string
	Notes       string
}

type Run struct {
	RunID        string
	CreatedAt    time.Time
	EnvID        string
	WorkloadID   string
	ModelID      string
	RouteReason  string
	LatencyMS    int64
	InputTokens  int64
	OutputTokens int64
	CostMicros   int64
	Success      bool
	QualityScore float64
	Reward       float64
	ErrorType    string
	ErrorMsg     string
}

type BanditArm struct {
	ArmKey       string
	EnvID        string
	WorkloadID   string
	ModelID      string
	Algo         string
	N            int64
	SumReward    float64
	SumReward2   float64
	Alpha        float64
	Beta         float64
	LastUpdated  time.Time
}

type PolicySnapshot struct {
	SnapshotID string
	CreatedAt  time.Time
	EnvID      string
	Category   string
	PolicyJSON string
	Notes      string
}
internal/okAON/store.go
코드 복사
Go
package okaon

import "context"

type Store interface {
	UpsertEnv(ctx context.Context, e Env) error
	UpsertModel(ctx context.Context, m Model) error
	UpsertWorkload(ctx context.Context, w Workload) error

	InsertRun(ctx context.Context, r Run) error

	GetRecentRunsByWorkload(ctx context.Context, envID, workloadID string, limit int) ([]Run, error)
	GetArms(ctx context.Context, envID, workloadID string) ([]BanditArm, error)
	UpsertArm(ctx context.Context, arm BanditArm) error

	InsertPolicySnapshot(ctx context.Context, s PolicySnapshot) error
	GetLatestPolicySnapshot(ctx context.Context, envID, category string) (*PolicySnapshot, error)

	VacuumIfNeeded(ctx context.Context) error
}
internal/okAON/retention.go
코드 복사
Go
package okaon

import "time"

type RetentionPolicy struct {
	MaxRunsPerWorkload int
	MaxAge             time.Duration
}

func DefaultRetention() RetentionPolicy {
	return RetentionPolicy{
		MaxRunsPerWorkload: 5000,
		MaxAge:             90 * 24 * time.Hour,
	}
}
internal/okAON/rewards.go
코드 복사
Go
package okaon

import "math"

// Reward shaping helpers.
// Goal: unify multiple signals (success, quality, latency, cost) into a stable scalar reward.

type RewardInput struct {
	Success      bool
	QualityScore float64 // 0..1
	LatencyMS    float64
	CostMicros   float64
}

type RewardWeights struct {
	SuccessW  float64
	QualityW  float64
	LatencyW  float64
	CostW     float64
}

func DefaultRewardWeights() RewardWeights {
	return RewardWeights{
		SuccessW: 1.0,
		QualityW: 1.0,
		LatencyW: 0.3,
		CostW:    0.2,
	}
}

// Reward output range: roughly -1.5 .. +2.0
func ComputeReward(in RewardInput, w RewardWeights) float64 {
	success := 0.0
	if in.Success {
		success = 1.0
	}

	quality := clamp01(in.QualityScore)

	// latency penalty (log scale)
	lat := 0.0
	if in.LatencyMS > 0 {
		lat = -math.Log1p(in.LatencyMS/250.0) // ~ -log(1 + x/250)
	}

	// cost penalty (micros -> normalized)
	cost := 0.0
	if in.CostMicros > 0 {
		cost = -math.Log1p(in.CostMicros/1_000_000.0)
	}

	return w.SuccessW*success + w.QualityW*quality + w.LatencyW*lat + w.CostW*cost
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
internal/okAON/aggregates.go
코드 복사
Go
package okaon

import "math"

type Stats struct {
	N         int
	Mean      float64
	Var       float64
	Min       float64
	Max       float64
	Sum       float64
	Sum2      float64
}

func StatsFromSum(n int, sum, sum2 float64) Stats {
	if n <= 0 {
		return Stats{}
	}
	mean := sum / float64(n)
	variance := 0.0
	if n > 1 {
		variance = (sum2/float64(n) - mean*mean)
		if variance < 0 {
			variance = 0
		}
	}
	return Stats{
		N:    n,
		Mean: mean,
		Var:  variance,
		Sum:  sum,
		Sum2: sum2,
		Min:  math.NaN(),
		Max:  math.NaN(),
	}
}
internal/okAON/policies.go
코드 복사
Go
package okaon

// PolicyJSON is intentionally opaque at this layer.
// Router/Learning decide the schema; OkAON stores it versioned.

type Policy struct {
	JSON string
}
internal/okAON/profiles.go
코드 복사
Go
package okaon

type HWProfile struct {
	OS     string
	Arch   string
	RAMMB  int
	GPUVRAMMB int
	CPUCores int
	HasGPU bool
}

func TierFromProfile(p HWProfile) string {
	// simple tiering
	if p.HasGPU && p.GPUVRAMMB >= 16000 && p.RAMMB >= 64000 {
		return "high"
	}
	if p.RAMMB >= 32000 {
		return "mid"
	}
	return "low_end"
}
4) OkAON SQLite 구현 (CRUD + Stats + Vacuum)
internal/okAON/sqlite/migrate.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
)

type Migrator struct {
	DB *sql.DB
}

func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{DB: db}
}

// If you already have a migration runner, connect here.
// This helper just verifies core tables exist.
func (m *Migrator) Verify(ctx context.Context) error {
	if m.DB == nil {
		return errors.New("okaon/sqlite: nil db")
	}
	_, err := m.DB.ExecContext(ctx, `SELECT 1 FROM okaon_env LIMIT 1;`)
	return err
}
internal/okAON/sqlite/insert.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/okAON"
)

type Repo struct {
	DB *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{DB: db}
}

func toMS(t time.Time) int64 { return t.UnixMilli() }

func (r *Repo) UpsertEnv(ctx context.Context, e okaon.Env) error {
	_, err := r.DB.ExecContext(ctx, `
INSERT INTO okaon_env(env_id, created_at, os, arch, cpu_brand, cpu_cores, ram_mb, gpu_brand, gpu_vram_mb, disk_type, runtime_stack, notes)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(env_id) DO UPDATE SET
  os=excluded.os, arch=excluded.arch, cpu_brand=excluded.cpu_brand, cpu_cores=excluded.cpu_cores,
  ram_mb=excluded.ram_mb, gpu_brand=excluded.gpu_brand, gpu_vram_mb=excluded.gpu_vram_mb,
  disk_type=excluded.disk_type, runtime_stack=excluded.runtime_stack, notes=excluded.notes;
`, e.EnvID, toMS(e.CreatedAt), e.OS, e.Arch, e.CPUBrand, e.CPUCores, e.RAMMB, e.GPUBrand, e.GPUVRAMMB, e.DiskType, e.RuntimeStack, e.Notes)
	return err
}

func (r *Repo) UpsertModel(ctx context.Context, m okaon.Model) error {
	isLocal := 0
	if m.IsLocal {
		isLocal = 1
	}
	_, err := r.DB.ExecContext(ctx, `
INSERT INTO okaon_model(model_id, provider, family, context_window, is_local, created_at, meta_json)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(model_id) DO UPDATE SET
  provider=excluded.provider, family=excluded.family, context_window=excluded.context_window, is_local=excluded.is_local, meta_json=excluded.meta_json;
`, m.ModelID, m.Provider, m.Family, m.ContextWindow, isLocal, toMS(m.CreatedAt), m.MetaJSON)
	return err
}

func (r *Repo) UpsertWorkload(ctx context.Context, w okaon.Workload) error {
	_, err := r.DB.ExecContext(ctx, `
INSERT INTO okaon_workload(workload_id, created_at, category, signature, repo_lang, toolset, notes)
VALUES(?,?,?,?,?,?,?)
ON CONFLICT(workload_id) DO UPDATE SET
  category=excluded.category, signature=excluded.signature, repo_lang=excluded.repo_lang, toolset=excluded.toolset, notes=excluded.notes;
`, w.WorkloadID, toMS(w.CreatedAt), w.Category, w.Signature, w.RepoLang, w.Toolset, w.Notes)
	return err
}

func (r *Repo) InsertRun(ctx context.Context, run okaon.Run) error {
	success := 0
	if run.Success {
		success = 1
	}
	_, err := r.DB.ExecContext(ctx, `
INSERT INTO okaon_run(run_id, created_at, env_id, workload_id, model_id, route_reason, latency_ms, input_tokens, output_tokens, cost_micros, success, quality_score, reward, error_type, error_msg)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);
`, run.RunID, toMS(run.CreatedAt), run.EnvID, run.WorkloadID, run.ModelID, run.RouteReason, run.LatencyMS, run.InputTokens, run.OutputTokens, run.CostMicros, success, run.QualityScore, run.Reward, run.ErrorType, run.ErrorMsg)
	return err
}

func armKey(envID, workloadID, modelID string) string {
	return envID + "|" + workloadID + "|" + modelID
}

func (r *Repo) UpsertArm(ctx context.Context, a okaon.BanditArm) error {
	if a.ArmKey == "" {
		a.ArmKey = armKey(a.EnvID, a.WorkloadID, a.ModelID)
	}
	_, err := r.DB.ExecContext(ctx, `
INSERT INTO okaon_bandit_arm(arm_key, env_id, workload_id, model_id, algo, n, sum_reward, sum_reward2, alpha, beta, last_updated)
VALUES(?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(arm_key) DO UPDATE SET
  algo=excluded.algo, n=excluded.n, sum_reward=excluded.sum_reward, sum_reward2=excluded.sum_reward2,
  alpha=excluded.alpha, beta=excluded.beta, last_updated=excluded.last_updated;
`, a.ArmKey, a.EnvID, a.WorkloadID, a.ModelID, a.Algo, a.N, a.SumReward, a.SumReward2, a.Alpha, a.Beta, toMS(a.LastUpdated))
	return err
}

func (r *Repo) InsertPolicySnapshot(ctx context.Context, s okaon.PolicySnapshot) error {
	_, err := r.DB.ExecContext(ctx, `
INSERT INTO okaon_policy_snapshot(snapshot_id, created_at, env_id, category, policy_json, notes)
VALUES(?,?,?,?,?,?);
`, s.SnapshotID, toMS(s.CreatedAt), s.EnvID, s.Category, s.PolicyJSON, s.Notes)
	return err
}
internal/okAON/sqlite/query.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/okAON"
)

func fromMS(ms int64) time.Time { return time.UnixMilli(ms) }

func (r *Repo) GetRecentRunsByWorkload(ctx context.Context, envID, workloadID string, limit int) ([]okaon.Run, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.DB.QueryContext(ctx, `
SELECT run_id, created_at, env_id, workload_id, model_id, route_reason, latency_ms, input_tokens, output_tokens, cost_micros, success, quality_score, reward, error_type, error_msg
FROM okaon_run
WHERE env_id=? AND workload_id=?
ORDER BY created_at DESC
LIMIT ?;
`, envID, workloadID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []okaon.Run
	for rows.Next() {
		var rr okaon.Run
		var created int64
		var success int
		if err := rows.Scan(
			&rr.RunID, &created, &rr.EnvID, &rr.WorkloadID, &rr.ModelID, &rr.RouteReason,
			&rr.LatencyMS, &rr.InputTokens, &rr.OutputTokens, &rr.CostMicros,
			&success, &rr.QualityScore, &rr.Reward, &rr.ErrorType, &rr.ErrorMsg,
		); err != nil {
			return nil, err
		}
		rr.CreatedAt = fromMS(created)
		rr.Success = (success == 1)
		out = append(out, rr)
	}
	return out, rows.Err()
}

func (r *Repo) GetArms(ctx context.Context, envID, workloadID string) ([]okaon.BanditArm, error) {
	rows, err := r.DB.QueryContext(ctx, `
SELECT arm_key, env_id, workload_id, model_id, algo, n, sum_reward, sum_reward2, alpha, beta, last_updated
FROM okaon_bandit_arm
WHERE env_id=? AND workload_id=?;
`, envID, workloadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []okaon.BanditArm
	for rows.Next() {
		var a okaon.BanditArm
		var last int64
		if err := rows.Scan(&a.ArmKey, &a.EnvID, &a.WorkloadID, &a.ModelID, &a.Algo, &a.N, &a.SumReward, &a.SumReward2, &a.Alpha, &a.Beta, &last); err != nil {
			return nil, err
		}
		a.LastUpdated = fromMS(last)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repo) GetLatestPolicySnapshot(ctx context.Context, envID, category string) (*okaon.PolicySnapshot, error) {
	row := r.DB.QueryRowContext(ctx, `
SELECT snapshot_id, created_at, env_id, category, policy_json, notes
FROM okaon_policy_snapshot
WHERE env_id=? AND category=?
ORDER BY created_at DESC
LIMIT 1;
`, envID, category)

	var s okaon.PolicySnapshot
	var created int64
	if err := row.Scan(&s.SnapshotID, &created, &s.EnvID, &s.Category, &s.PolicyJSON, &s.Notes); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	s.CreatedAt = fromMS(created)
	return &s, nil
}
internal/okAON/sqlite/stats.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"

	"devorch/internal/okAON"
)

func (r *Repo) RewardStats(ctx context.Context, envID, workloadID, modelID string) (okaon.Stats, error) {
	row := r.DB.QueryRowContext(ctx, `
SELECT COUNT(1), COALESCE(SUM(reward),0), COALESCE(SUM(reward*reward),0)
FROM okaon_run
WHERE env_id=? AND workload_id=? AND model_id=?;
`, envID, workloadID, modelID)

	var n int
	var sum, sum2 float64
	if err := row.Scan(&n, &sum, &sum2); err != nil {
		return okaon.Stats{}, err
	}
	return okaon.StatsFromSum(n, sum, sum2), nil
}

func (r *Repo) EnsureModelExists(ctx context.Context, modelID string) (bool, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT 1 FROM okaon_model WHERE model_id=? LIMIT 1;`, modelID)
	var one int
	err := row.Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
internal/okAON/sqlite/vacuum.go
코드 복사
Go
package sqlite

import (
	"context"
	"time"
)

func (r *Repo) VacuumIfNeeded(ctx context.Context) error {
	// very lightweight: vacuum at most once per day
	now := time.Now().UnixMilli()
	_, _ = r.DB.ExecContext(ctx, `
INSERT INTO okaon_retention_state(key, updated_at, value)
VALUES('last_vacuum', ?, '')
ON CONFLICT(key) DO NOTHING;
`, now)

	var last int64
	row := r.DB.QueryRowContext(ctx, `SELECT updated_at FROM okaon_retention_state WHERE key='last_vacuum';`)
	if err := row.Scan(&last); err != nil {
		return nil // do not fail runtime
	}
	if time.UnixMilli(now).Sub(time.UnixMilli(last)) < 24*time.Hour {
		return nil
	}
	// update first to avoid repeated vacuum storms
	_, _ = r.DB.ExecContext(ctx, `UPDATE okaon_retention_state SET updated_at=? WHERE key='last_vacuum';`, now)

	_, err := r.DB.ExecContext(ctx, `VACUUM;`)
	return err
}
5) OkAON Store 어댑터(Repo → Store)
internal/okAON/sqlite/store.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"

	"devorch/internal/okAON"
)

type Store struct {
	repo *Repo
}

func NewStore(db *sql.DB) *Store {
	return &Store{repo: NewRepo(db)}
}

func (s *Store) UpsertEnv(ctx context.Context, e okaon.Env) error               { return s.repo.UpsertEnv(ctx, e) }
func (s *Store) UpsertModel(ctx context.Context, m okaon.Model) error           { return s.repo.UpsertModel(ctx, m) }
func (s *Store) UpsertWorkload(ctx context.Context, w okaon.Workload) error     { return s.repo.UpsertWorkload(ctx, w) }
func (s *Store) InsertRun(ctx context.Context, r okaon.Run) error               { return s.repo.InsertRun(ctx, r) }
func (s *Store) GetRecentRunsByWorkload(ctx context.Context, envID, workloadID string, limit int) ([]okaon.Run, error) {
	return s.repo.GetRecentRunsByWorkload(ctx, envID, workloadID, limit)
}
func (s *Store) GetArms(ctx context.Context, envID, workloadID string) ([]okaon.BanditArm, error) {
	return s.repo.GetArms(ctx, envID, workloadID)
}
func (s *Store) UpsertArm(ctx context.Context, arm okaon.BanditArm) error { return s.repo.UpsertArm(ctx, arm) }
func (s *Store) InsertPolicySnapshot(ctx context.Context, ps okaon.PolicySnapshot) error {
	return s.repo.InsertPolicySnapshot(ctx, ps)
}
func (s *Store) GetLatestPolicySnapshot(ctx context.Context, envID, category string) (*okaon.PolicySnapshot, error) {
	return s.repo.GetLatestPolicySnapshot(ctx, envID, category)
}
func (s *Store) VacuumIfNeeded(ctx context.Context) error { return s.repo.VacuumIfNeeded(ctx) }
6) Learning 엔진 (Contextual Bandit: Thompson 중심)
6.1 Bandit 공통
internal/learning/bandit/bandit.go
코드 복사
Go
package bandit

import "math/rand"

type Arm struct {
	ID string
}

type ArmState struct {
	ID         string
	N          int64
	SumReward  float64
	SumReward2 float64
	Alpha      float64
	Beta       float64
}

type ChooseInput struct {
	Arms  []ArmState
	Rand  *rand.Rand
}

type UpdateInput struct {
	ArmID  string
	Reward float64
}

type Bandit interface {
	Name() string
	Choose(in ChooseInput) (armID string, explain map[string]any)
	Update(state ArmState, upd UpdateInput) (ArmState, map[string]any)
}
internal/learning/bandit/dist.go
코드 복사
Go
package bandit

import (
	"math"
	"math/rand"
)

// Normal sampler (Box-Muller)
func SampleNormal(r *rand.Rand, mean, std float64) float64 {
	if std <= 0 {
		return mean
	}
	u1 := r.Float64()
	u2 := r.Float64()
	z := math.Sqrt(-2.0*math.Log(u1)) * math.Cos(2.0*math.Pi*u2)
	return mean + std*z
}

// Very simple Gamma sampler (Marsaglia-Tsang for k>=1)
func SampleGamma(r *rand.Rand, k, theta float64) float64 {
	if k <= 0 {
		return 0
	}
	if k < 1 {
		// Johnk's generator
		for {
			u := r.Float64()
			b := (math.E + k) / math.E
			p := b * u
			var x float64
			if p <= 1 {
				x = math.Pow(p, 1/k)
			} else {
				x = -math.Log((b - p) / k)
			}
			u2 := r.Float64()
			if p <= 1 {
				if u2 <= math.Exp(-x) {
					return theta * x
				}
			} else {
				if u2 <= math.Pow(x, k-1) {
					return theta * x
				}
			}
		}
	}

	d := k - 1.0/3.0
	c := 1.0 / math.Sqrt(9*d)
	for {
		x := SampleNormal(r, 0, 1)
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := r.Float64()
		if u < 1-0.0331*(x*x)*(x*x) {
			return theta * d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return theta * d * v
		}
	}
}

// Beta via Gamma
func SampleBeta(r *rand.Rand, alpha, beta float64) float64 {
	x := SampleGamma(r, alpha, 1)
	y := SampleGamma(r, beta, 1)
	if x+y == 0 {
		return 0.5
	}
	return x / (x + y)
}
6.2 Thompson Sampling (핵심)
internal/learning/bandit/thompson.go
코드 복사
Go
package bandit

import (
	"math"
)

type Thompson struct {
	// Reward is expected ~ [-2..+2]. We map to [0..1] for Beta updates.
}

func NewThompson() *Thompson { return &Thompson{} }
func (t *Thompson) Name() string { return "thompson" }

func (t *Thompson) Choose(in ChooseInput) (string, map[string]any) {
	r := in.Rand
	bestID := ""
	best := -1.0
	samples := make(map[string]float64, len(in.Arms))

	for _, a := range in.Arms {
		alpha := a.Alpha
		beta := a.Beta