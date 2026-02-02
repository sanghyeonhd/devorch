좋습니다. **(네트워크가 끊긴 지점 이후 = “40단계”)**로는, 지금 트리 기준에서 가장 먼저 “실제로 동작/학습이 시작되는” 핵심 축인 OkAON(SQLite) 저장소 + 마이그레이션(0003) + CRUD(Query/Stats) + 보존/정리(Vacuum/Retention) 을 풀코드로 넣는 게 맞습니다.
또, 참고로 주신 sanghyeonhd/BIFC2는 GitHub에서 확인해보니 phase1~phase39.md 같은 단계 문서 파일들만 있고 실제 Go 소스 트리는 올라와 있지 않았습니다. �
그래서 “현재 개발된 시점”을 레포에서 정확히 이어붙이기는 불가능했고, 당신이 제시한 최종 트리에서 40단계로 바로 실구현 가능한 OkAON 모듈 전체를 완성본으로 제공합니다.
GitHub +1
아래는 40단계: OkAON SQLite 전체 풀코드(+SQL) 입니다. (경로/파일명 그대로)
40단계 풀코드
1) SQL 마이그레이션
internal/storage/sqlite/migrations/0003_okAON_learning.sql
코드 복사
Sql
-- 0003_okAON_learning.sql
-- OkAON: local measured benchmark + reward + policy snapshot store

PRAGMA foreign_keys = ON;

-- 1) 실행(벤치/요청) 단위 기록
CREATE TABLE IF NOT EXISTS okaon_run (
  id               TEXT PRIMARY KEY,             -- ULID/UUID
  created_at_unix  INTEGER NOT NULL,             -- seconds
  project_id       TEXT NOT NULL,
  workspace_id     TEXT,
  user_id          TEXT,

  -- env / machine fingerprint
  os               TEXT NOT NULL,                -- windows/darwin/linux
  arch             TEXT NOT NULL,                -- amd64/arm64
  machine_fingerprint TEXT NOT NULL,             -- stable hash-like string
  hw_tier          TEXT NOT NULL,                -- low/mid/high/custom

  -- task meta
  category         TEXT NOT NULL,                -- quick/ultrabrain/visual-engineering/...
  agent_type       TEXT,                         -- oracle/sisyphus/...
  toolchain        TEXT,                         -- lsp/astgrep/git/...
  prompt_hash      TEXT NOT NULL,                -- sha256(base prompt)
  input_tokens     INTEGER NOT NULL DEFAULT 0,
  output_tokens    INTEGER NOT NULL DEFAULT 0,

  -- selected provider/model
  provider         TEXT NOT NULL,                -- openai/anthropic/google/ollama/...
  model            TEXT NOT NULL,                -- gpt-5.2/claude-.../llama3...
  endpoint_type    TEXT NOT NULL,                -- local/remote
  route_reason     TEXT,                         -- router explanation (short)

  -- performance (measured)
  latency_ms       INTEGER NOT NULL DEFAULT 0,
  ttft_ms          INTEGER NOT NULL DEFAULT 0,
  stream_duration_ms INTEGER NOT NULL DEFAULT 0,
  total_cost_microusd INTEGER NOT NULL DEFAULT 0, -- cost ledger micro USD
  success          INTEGER NOT NULL DEFAULT 1,     -- 1/0
  error_code       TEXT,
  error_message    TEXT,

  -- quality gate / evaluation
  quality_score    REAL NOT NULL DEFAULT 0.0,     -- 0~1 normalized
  quality_label    TEXT,                          -- pass/warn/fail
  reward           REAL NOT NULL DEFAULT 0.0       -- learned reward (can be same as quality_score adjusted)
);

CREATE INDEX IF NOT EXISTS idx_okaon_run_created_at ON okaon_run(created_at_unix);
CREATE INDEX IF NOT EXISTS idx_okaon_run_project ON okaon_run(project_id);
CREATE INDEX IF NOT EXISTS idx_okaon_run_machine ON okaon_run(machine_fingerprint);
CREATE INDEX IF NOT EXISTS idx_okaon_run_choice ON okaon_run(provider, model, endpoint_type);
CREATE INDEX IF NOT EXISTS idx_okaon_run_task ON okaon_run(category, agent_type);

-- 2) 리워드 이벤트 (명시적 피드백/자동 품질 게이트/재시도 페널티 등)
CREATE TABLE IF NOT EXISTS okaon_reward_event (
  id              TEXT PRIMARY KEY,
  created_at_unix INTEGER NOT NULL,
  run_id          TEXT NOT NULL,
  kind            TEXT NOT NULL,      -- quality_gate/user_feedback/retry_penalty/cost_penalty/latency_penalty/...
  value           REAL NOT NULL,      -- +/- reward
  meta_json       TEXT,               -- small json blob (optional)
  FOREIGN KEY(run_id) REFERENCES okaon_run(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_reward_run ON okaon_reward_event(run_id);
CREATE INDEX IF NOT EXISTS idx_reward_created_at ON okaon_reward_event(created_at_unix);

-- 3) 정책 스냅샷 (밴딧/회귀/라우터 정책 결과 저장)
CREATE TABLE IF NOT EXISTS okaon_policy_snapshot (
  id               TEXT PRIMARY KEY,
  created_at_unix  INTEGER NOT NULL,
  scope            TEXT NOT NULL,      -- global/workspace/project/machine
  scope_id         TEXT,               -- optional id for scope
  policy_type      TEXT NOT NULL,      -- bandit/thompson/ucb/epsilon/linear_regression/...
  version          INTEGER NOT NULL DEFAULT 1,
  payload_json     TEXT NOT NULL       -- policy params / arm priors / weights
);

CREATE INDEX IF NOT EXISTS idx_policy_scope ON okaon_policy_snapshot(scope, scope_id);
CREATE INDEX IF NOT EXISTS idx_policy_created_at ON okaon_policy_snapshot(created_at_unix);

-- 4) Arm(모델/프로바이더 선택지) 통계 (빠른 조회용 머터리얼라이즈드 성격)
CREATE TABLE IF NOT EXISTS okaon_arm_stats (
  id                 TEXT PRIMARY KEY,  -- scope+arm key hash
  updated_at_unix     INTEGER NOT NULL,

  scope              TEXT NOT NULL,      -- global/workspace/project/machine
  scope_id           TEXT,
  category           TEXT NOT NULL,

  provider           TEXT NOT NULL,
  model              TEXT NOT NULL,
  endpoint_type      TEXT NOT NULL,      -- local/remote

  pulls              INTEGER NOT NULL DEFAULT 0,
  successes          INTEGER NOT NULL DEFAULT 0,
  failures           INTEGER NOT NULL DEFAULT 0,

  avg_latency_ms     REAL NOT NULL DEFAULT 0.0,
  avg_quality        REAL NOT NULL DEFAULT 0.0,
  avg_cost_microusd  REAL NOT NULL DEFAULT 0.0,

  reward_sum         REAL NOT NULL DEFAULT 0.0,
  reward_sq_sum      REAL NOT NULL DEFAULT 0.0    -- variance helper
);

CREATE INDEX IF NOT EXISTS idx_arm_scope ON okaon_arm_stats(scope, scope_id, category);
CREATE INDEX IF NOT EXISTS idx_arm_choice ON okaon_arm_stats(provider, model, endpoint_type);
2) OkAON 모델/인터페이스
internal/okAON/models.go
코드 복사
Go
package okAON

import "time"

type Run struct {
	ID        string
	CreatedAt time.Time

	ProjectID   string
	WorkspaceID string
	UserID      string

	OS                string
	Arch              string
	MachineFingerprint string
	HwTier            string

	Category  string
	AgentType string
	Toolchain string
	PromptHash string

	InputTokens  int64
	OutputTokens int64

	Provider     string
	Model        string
	EndpointType string // local/remote
	RouteReason  string

	LatencyMs        int64
	TTFTMs           int64
	StreamDurationMs int64
	TotalCostMicroUSD int64

	Success      bool
	ErrorCode    string
	ErrorMessage string

	QualityScore float64
	QualityLabel string
	Reward       float64
}

type RewardEvent struct {
	ID        string
	CreatedAt time.Time
	RunID     string
	Kind      string
	Value     float64
	MetaJSON  string
}

type PolicySnapshot struct {
	ID        string
	CreatedAt time.Time
	Scope     string // global/workspace/project/machine
	ScopeID   string
	PolicyType string
	Version   int64
	PayloadJSON string
}

type ArmStats struct {
	ID        string
	UpdatedAt time.Time

	Scope    string
	ScopeID  string
	Category string

	Provider     string
	Model        string
	EndpointType string

	Pulls     int64
	Successes int64
	Failures  int64

	AvgLatencyMs    float64
	AvgQuality      float64
	AvgCostMicroUSD float64

	RewardSum   float64
	RewardSqSum float64
}

type RunQuery struct {
	ProjectID string
	Scope     string // optional, used by stats joins
	ScopeID   string
	Category  string
	Provider  string
	Model     string
	EndpointType string
	MachineFingerprint string

	Since time.Time
	Until time.Time
	Limit int
}

type Aggregates struct {
	Count        int64
	SuccessRate  float64
	AvgLatencyMs float64
	AvgQuality   float64
	AvgCostMicroUSD float64
	AvgReward    float64
}
internal/okAON/store.go
코드 복사
Go
package okAON

import "context"

type Store interface {
	Migrate(ctx context.Context) error

	InsertRun(ctx context.Context, run *Run) error
	InsertRewardEvent(ctx context.Context, ev *RewardEvent) error
	InsertPolicySnapshot(ctx context.Context, snap *PolicySnapshot) error

	ListRuns(ctx context.Context, q RunQuery) ([]Run, error)
	GetRun(ctx context.Context, id string) (*Run, error)

	UpsertArmStats(ctx context.Context, s *ArmStats) error
	GetArmStats(ctx context.Context, scope, scopeID, category string, limit int) ([]ArmStats, error)

	GetAggregates(ctx context.Context, q RunQuery) (*Aggregates, error)

	VacuumAndRetention(ctx context.Context, keepDays int) error
	Close() error
}
3) SQLite 구현
internal/okAON/sqlite/migrate.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Migrator struct {
	DB *sql.DB
}

func (m *Migrator) EnsureSchemaTable(ctx context.Context) error {
	_, err := m.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at_unix INTEGER NOT NULL
);`)
	return err
}

func (m *Migrator) AppliedVersions(ctx context.Context) (map[string]bool, error) {
	rows, err := m.DB.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func (m *Migrator) ApplyAll(ctx context.Context) error {
	if err := m.EnsureSchemaTable(ctx); err != nil {
		return err
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	applied, err := m.AppliedVersions(ctx)
	if err != nil {
		return err
	}

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		if applied[version] {
			continue
		}

		b, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, string(b)); err != nil {
			return fmt.Errorf("migration %s failed: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at_unix) VALUES(?, strftime('%s','now'))`, version); err != nil {
			return err
		}
	}

	return tx.Commit()
}
internal/okAON/sqlite/migrations/0003_okAON_learning.sql
위 SQL 파일을 그대로 이 경로로 복사해 두세요.
internal/okAON/sqlite/sqlite_store.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/mattn/go-sqlite3"

	ok "devorch/internal/okAON"
)

type Store struct {
	db *sql.DB
	m  *Migrator
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // sqlite 안정성 (daemon 단일 프로세스 가정)
	db.SetConnMaxLifetime(0)

	return &Store{db: db, m: &Migrator{DB: db}}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Migrate(ctx context.Context) error { return s.m.ApplyAll(ctx) }

func unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func fromUnix(sec int64) time.Time {
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}

var ErrNotFound = errors.New("not found")

// compile-time check
var _ ok.Store = (*Store)(nil)
internal/okAON/sqlite/insert.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	ok "devorch/internal/okAON"
)

func (s *Store) InsertRun(ctx context.Context, run *ok.Run) error {
	if run == nil {
		return fmt.Errorf("run is nil")
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO okaon_run(
  id, created_at_unix,
  project_id, workspace_id, user_id,
  os, arch, machine_fingerprint, hw_tier,
  category, agent_type, toolchain, prompt_hash,
  input_tokens, output_tokens,
  provider, model, endpoint_type, route_reason,
  latency_ms, ttft_ms, stream_duration_ms,
  total_cost_microusd,
  success, error_code, error_message,
  quality_score, quality_label, reward
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)`,
		run.ID, unix(run.CreatedAt),
		run.ProjectID, run.WorkspaceID, run.UserID,
		run.OS, run.Arch, run.MachineFingerprint, run.HwTier,
		run.Category, run.AgentType, run.Toolchain, run.PromptHash,
		run.InputTokens, run.OutputTokens,
		run.Provider, run.Model, run.EndpointType, run.RouteReason,
		run.LatencyMs, run.TTFTMs, run.StreamDurationMs,
		run.TotalCostMicroUSD,
		boolToInt(run.Success), nullStr(run.ErrorCode), nullStr(run.ErrorMessage),
		run.QualityScore, nullStr(run.QualityLabel), run.Reward,
	)
	return err
}

func (s *Store) InsertRewardEvent(ctx context.Context, ev *ok.RewardEvent) error {
	if ev == nil {
		return fmt.Errorf("reward event is nil")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO okaon_reward_event(
  id, created_at_unix, run_id, kind, value, meta_json
) VALUES(?, ?, ?, ?, ?, ?)`,
		ev.ID, unix(ev.CreatedAt), ev.RunID, ev.Kind, ev.Value, nullStr(ev.MetaJSON),
	)
	return err
}

func (s *Store) InsertPolicySnapshot(ctx context.Context, snap *ok.PolicySnapshot) error {
	if snap == nil {
		return fmt.Errorf("policy snapshot is nil")
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO okaon_policy_snapshot(
  id, created_at_unix, scope, scope_id, policy_type, version, payload_json
) VALUES(?, ?, ?, ?, ?, ?, ?)`,
		snap.ID, unix(snap.CreatedAt), snap.Scope, nullStr(snap.ScopeID), snap.PolicyType, snap.Version, snap.PayloadJSON,
	)
	return err
}

func (s *Store) UpsertArmStats(ctx context.Context, st *ok.ArmStats) error {
	if st == nil {
		return fmt.Errorf("arm stats is nil")
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO okaon_arm_stats(
  id, updated_at_unix,
  scope, scope_id, category,
  provider, model, endpoint_type,
  pulls, successes, failures,
  avg_latency_ms, avg_quality, avg_cost_microusd,
  reward_sum, reward_sq_sum
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  updated_at_unix=excluded.updated_at_unix,
  pulls=excluded.pulls,
  successes=excluded.successes,
  failures=excluded.failures,
  avg_latency_ms=excluded.avg_latency_ms,
  avg_quality=excluded.avg_quality,
  avg_cost_microusd=excluded.avg_cost_microusd,
  reward_sum=excluded.reward_sum,
  reward_sq_sum=excluded.reward_sq_sum
`,
		st.ID, unix(st.UpdatedAt),
		st.Scope, nullStr(st.ScopeID), st.Category,
		st.Provider, st.Model, st.EndpointType,
		st.Pulls, st.Successes, st.Failures,
		st.AvgLatencyMs, st.AvgQuality, st.AvgCostMicroUSD,
		st.RewardSum, st.RewardSqSum,
	)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
internal/okAON/sqlite/query.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	ok "devorch/internal/okAON"
)

func (s *Store) GetRun(ctx context.Context, id string) (*ok.Run, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT
  id, created_at_unix,
  project_id, workspace_id, user_id,
  os, arch, machine_fingerprint, hw_tier,
  category, agent_type, toolchain, prompt_hash,
  input_tokens, output_tokens,
  provider, model, endpoint_type, route_reason,
  latency_ms, ttft_ms, stream_duration_ms,
  total_cost_microusd,
  success, error_code, error_message,
  quality_score, quality_label, reward
FROM okaon_run
WHERE id=?`, id)

	r, err := scanRun(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Store) ListRuns(ctx context.Context, q ok.RunQuery) ([]ok.Run, error) {
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	where, args := buildRunWhere(q)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, `
SELECT
  id, created_at_unix,
  project_id, workspace_id, user_id,
  os, arch, machine_fingerprint, hw_tier,
  category, agent_type, toolchain, prompt_hash,
  input_tokens, output_tokens,
  provider, model, endpoint_type, route_reason,
  latency_ms, ttft_ms, stream_duration_ms,
  total_cost_microusd,
  success, error_code, error_message,
  quality_score, quality_label, reward
FROM okaon_run
`+where+`
ORDER BY created_at_unix DESC
LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ok.Run, 0, limit)
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

func (s *Store) GetAggregates(ctx context.Context, q ok.RunQuery) (*ok.Aggregates, error) {
	where, args := buildRunWhere(q)

	row := s.db.QueryRowContext(ctx, `
SELECT
  COUNT(1) as cnt,
  SUM(CASE WHEN success=1 THEN 1 ELSE 0 END) as succ,
  AVG(latency_ms) as avg_lat,
  AVG(quality_score) as avg_q,
  AVG(total_cost_microusd) as avg_cost,
  AVG(reward) as avg_reward
FROM okaon_run
`+where, args...)

	var cnt sql.NullInt64
	var succ sql.NullInt64
	var avgLat sql.NullFloat64
	var avgQ sql.NullFloat64
	var avgCost sql.NullFloat64
	var avgReward sql.NullFloat64

	if err := row.Scan(&cnt, &succ, &avgLat, &avgQ, &avgCost, &avgReward); err != nil {
		return nil, err
	}

	c := cnt.Int64
	if !cnt.Valid {
		c = 0
	}
	su := succ.Int64
	if !succ.Valid {
		su = 0
	}

	sr := 0.0
	if c > 0 {
		sr = float64(su) / float64(c)
	}

	return &ok.Aggregates{
		Count:        c,
		SuccessRate:  sr,
		AvgLatencyMs: nzf(avgLat),
		AvgQuality:   nzf(avgQ),
		AvgCostMicroUSD: nzf(avgCost),
		AvgReward:    nzf(avgReward),
	}, nil
}

func (s *Store) GetArmStats(ctx context.Context, scope, scopeID, category string, limit int) ([]ok.ArmStats, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT
  id, updated_at_unix,
  scope, scope_id, category,
  provider, model, endpoint_type,
  pulls, successes, failures,
  avg_latency_ms, avg_quality, avg_cost_microusd,
  reward_sum, reward_sq_sum
FROM okaon_arm_stats
WHERE scope=? AND IFNULL(scope_id,'')=? AND category=?
ORDER BY reward_sum DESC, successes DESC
LIMIT ?`,
		scope, scopeID, category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ok.ArmStats
	for rows.Next() {
		var a ok.ArmStats
		var updated int64
		var scopeIDNull sql.NullString

		if err := rows.Scan(
			&a.ID, &updated,
			&a.Scope, &scopeIDNull, &a.Category,
			&a.Provider, &a.Model, &a.EndpointType,
			&a.Pulls, &a.Successes, &a.Failures,
			&a.AvgLatencyMs, &a.AvgQuality, &a.AvgCostMicroUSD,
			&a.RewardSum, &a.RewardSqSum,
		); err != nil {
			return nil, err
		}
		a.UpdatedAt = fromUnix(updated)
		if scopeIDNull.Valid {
			a.ScopeID = scopeIDNull.String
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func buildRunWhere(q ok.RunQuery) (string, []any) {
	where := "WHERE 1=1"
	args := make([]any, 0, 12)

	add := func(cond string, v any, okv bool) {
		if !okv {
			return
		}
		where += " AND " + cond
		args = append(args, v)
	}

	add("project_id=?", q.ProjectID, q.ProjectID != "")
	add("category=?", q.Category, q.Category != "")
	add("provider=?", q.Provider, q.Provider != "")
	add("model=?", q.Model, q.Model != "")
	add("endpoint_type=?", q.EndpointType, q.EndpointType != "")
	add("machine_fingerprint=?", q.MachineFingerprint, q.MachineFingerprint != "")

	if !q.Since.IsZero() {
		add("created_at_unix>=?", q.Since.UTC().Unix(), true)
	}
	if !q.Until.IsZero() {
		add("created_at_unix<=?", q.Until.UTC().Unix(), true)
	}

	return where + " ", args
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRun(row scanner) (*ok.Run, error) {
	var r ok.Run
	var created int64
	var wsID, userID sql.NullString
	var agentType, toolchain sql.NullString
	var routeReason sql.NullString
	var errCode, errMsg sql.NullString
	var qLabel sql.NullString
	var success int

	if err := row.Scan(
		&r.ID, &created,
		&r.ProjectID, &wsID, &userID,
		&r.OS, &r.Arch, &r.MachineFingerprint, &r.HwTier,
		&r.Category, &agentType, &toolchain, &r.PromptHash,
		&r.InputTokens, &r.OutputTokens,
		&r.Provider, &r.Model, &r.EndpointType, &routeReason,
		&r.LatencyMs, &r.TTFTMs, &r.StreamDurationMs,
		&r.TotalCostMicroUSD,
		&success, &errCode, &errMsg,
		&r.QualityScore, &qLabel, &r.Reward,
	); err != nil {
		return nil, err
	}

	r.CreatedAt = fromUnix(created)
	r.Success = success == 1

	if wsID.Valid {
		r.WorkspaceID = wsID.String
	}
	if userID.Valid {
		r.UserID = userID.String
	}
	if agentType.Valid {
		r.AgentType = agentType.String
	}
	if toolchain.Valid {
		r.Toolchain = toolchain.String
	}
	if routeReason.Valid {
		r.RouteReason = routeReason.String
	}
	if errCode.Valid {
		r.ErrorCode = errCode.String
	}
	if errMsg.Valid {
		r.ErrorMessage = errMsg.String
	}
	if qLabel.Valid {
		r.QualityLabel = qLabel.String
	}
	return &r, nil
}

func nzf(v sql.NullFloat64) float64 {
	if v.Valid {
		return v.Float64
	}
	return 0
}

func (s *Store) timeRangeDefault(q *ok.RunQuery) {
	if q.Since.IsZero() && q.Until.IsZero() {
		q.Since = time.Now().Add(-7 * 24 * time.Hour)
	}
}

func (s *Store) MustProject(q ok.RunQuery) error {
	if q.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}
	return nil
}
internal/okAON/sqlite/stats.go
코드 복사
Go
package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"

	ok "devorch/internal/okAON"
)

// BuildArmStatsID: scope+scopeID+category+provider+model+endpoint_type 를 해시로 고정 ID 생성
func BuildArmStatsID(scope, scopeID, category, provider, model, endpointType string) string {
	h := sha256.Sum256([]byte(scope + "|" + scopeID + "|" + category + "|" + provider + "|" + model + "|" + endpointType))
	return hex.EncodeToString(h[:16]) // 32 hex chars (충분)
}

// RecomputeArmStatsFromRuns:
// - 최근 run들을 기반으로 arm_stats를 갱신 (간단/안전/빠름)
// - 성능 때문에 “최근 N개/최근 X일” 기준으로만 호출하는 것을 권장
func (s *Store) RecomputeArmStatsFromRuns(ctx context.Context, scope, scopeID, category string, q ok.RunQuery) error {
	q.Category = category
	rows, err := s.ListRuns(ctx, q)
	if err != nil {
		return err
	}

	type key struct{ p, m, e string }
	type agg struct {
		pulls, succ, fail int64
		latSum, qSum, costSum float64
		rewardSum, rewardSqSum float64
	}
	m := map[key]*agg{}

	for _, r := range rows {
		k := key{r.Provider, r.Model, r.EndpointType}
		a := m[k]
		if a == nil {
			a = &agg{}
			m[k] = a
		}

		a.pulls++
		if r.Success {
			a.succ++
		} else {
			a.fail++
		}

		a.latSum += float64(r.LatencyMs)
		a.qSum += r.QualityScore
		a.costSum += float64(r.TotalCostMicroUSD)
		a.rewardSum += r.Reward
		a.rewardSqSum += r.Reward * r.Reward
	}

	for k, a := range m {
		if a.pulls == 0 {
			continue
		}
		st := &ok.ArmStats{
			ID:           BuildArmStatsID(scope, scopeID, category, k.p, k.m, k.e),
			UpdatedAt:    nowUTC(),
			Scope:        scope,
			ScopeID:      scopeID,
			Category:     category,
			Provider:     k.p,
			Model:        k.m,
			EndpointType: k.e,
			Pulls:        a.pulls,
			Successes:    a.succ,
			Failures:     a.fail,
			AvgLatencyMs: a.latSum / float64(a.pulls),
			AvgQuality:   a.qSum / float64(a.pulls),
			AvgCostMicroUSD: a.costSum / float64(a.pulls),
			RewardSum:    a.rewardSum,
			RewardSqSum:  a.rewardSqSum,
		}
		if err := s.UpsertArmStats(ctx, st); err != nil {
			return fmt.Errorf("upsert arm stats failed: %w", err)
		}
	}
	return nil
}

// RewardVariance (optional helper)
func RewardVariance(pulls int64, sum, sq float64) float64 {
	if pulls <= 1 {
		return 0
	}
	mean := sum / float64(pulls)
	return math.Max(0, (sq/float64(pulls)) - (mean * mean))
}
internal/okAON/sqlite/vacuum.go
코드 복사
Go
package sqlite

import (
	"context"
	"fmt"
)

func (s *Store) VacuumAndRetention(ctx context.Context, keepDays int) error {
	if keepDays <= 0 {
		keepDays = 30
	}

	// 오래된 run/이벤트 삭제
	_, err := s.db.ExecContext(ctx, `
DELETE FROM okaon_run
WHERE created_at_unix < (strftime('%s','now') - (? * 86400));`, keepDays)
	if err != nil {
		return fmt.Errorf("retention delete failed: %w", err)
	}

	// sqlite 최적화 (과도하게 자주 호출 X)
	_, _ = s.db.ExecContext(ctx, `PRAGMA optimize;`)
	_, err = s.db.ExecContext(ctx, `VACUUM;`)
	if err != nil {
		// VACUUM은 잠금 때문에 실패할 수 있어서 치명적 오류로 보지 않을 수도 있음
		return fmt.Errorf("vacuum failed: %w", err)
	}
	return nil
}
internal/okAON/sqlite/time.go
코드 복사
Go
package sqlite

import "time"

func nowUTC() time.Time { return time.Now().UTC() }
40단계 적용 체크리스트
internal/okAON/sqlite/migrations/0003_okAON_learning.sql 파일을 정확히 위치시킵니다.
데몬 시작 시 okAONStore.Migrate(ctx) 호출
벤치/라우터/훅에서 아래 순서로 넣으면 “학습 데이터가 쌓이기 시작”
run 기록: InsertRun
품질/페널티: InsertRewardEvent (+ run.reward 업데이트는 41단계에서)
arm_stats 재계산(간단 버전): RecomputeArmStatsFromRuns (예: 최근 7일)
다음(41단계)에서 바로 이어갈 것
router/learning_adapter.go에서 **okaon_arm