좋습니다. 36단계는 OkAON(실측 데이터 레이크) “최소 CRUD + 조회 + 통계”가 실제로 돌아가도록 아래 파일들을 풀코드로 제공합니다.

이번 단계에서 바로 되는 것:

실행(벤치/작업) 결과를 okaon_runs에 저장

리워드(품질/사용자피드백/자동평가) 를 okaon_rewards에 저장

최근 실행 이력 조회 / 모델별 통계 조회

(옵션) 모델 프로필(로컬 환경별) 업서트


또한 0003 마이그레이션 SQL도 누락 없이 포함합니다. (이게 없으면 CRUD가 돌아가지 않음)


---

36단계 포함 파일 목록 (풀코드)

internal/okAON/models.go

internal/okAON/store.go

internal/okAON/sqlite/insert.go

internal/okAON/sqlite/query.go

internal/okAON/sqlite/stats.go

internal/storage/sqlite/migrations/0003_okAON_learning.sql



---

1) internal/okAON/models.go

package okAON

import "time"

// RunRecord: "작업 1회"의 실측 기록(라우터/학습의 원천 데이터)
type RunRecord struct {
	ID string

	// Context keys
	OrgID       string
	WorkspaceID string
	ProjectID   string
	SessionID   string
	TaskID      string

	// Routing keys
	Category string // e.g. quick, ultrabrain, visual-engineering
	Agent    string // e.g. sisyphus, atlas, oracle ...

	// Model selection result
	Provider string // e.g. openai, anthropic, google, ollama
	Model    string // e.g. gpt-5.2, claude-opus-4-5, gemini-3-pro, llama3:8b
	Endpoint string // optional: base URL / local runtime tag

	// Environment fingerprint (per machine)
	OS          string // darwin/windows/linux
	Arch        string // amd64/arm64
	MachineFP   string // hashed fingerprint
	HWTier      string // low/mid/high
	Locality    string // local/remote/hybrid
	NetworkType string // offline/lan/wan/unknown

	// Input/Output metadata
	PromptHash  string // hash only (no raw prompt here)
	OutputHash  string // hash only (no raw output here)
	ToolingHint string // optional: "lsp|astgrep|mcp" etc

	// Measurements
	StartedAt    time.Time
	CompletedAt  time.Time
	LatencyMs    int64
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	CostMicros   int64 // cost in micro currency unit (e.g., USD * 1e6)

	// Result
	Success      bool
	ErrorClass   string // timeout, rate_limit, model_error, tool_error, ...
	ErrorMessage string // short (redacted)
	QualityScore float64 // 0~1 (auto-eval or proxy)
}

type RewardRecord struct {
	ID string

	RunID string

	// Who/What produced reward
	Source string // user|auto|bench|qa|policy
	Kind   string // preference|quality|latency|cost|safety|compliance

	// Reward values
	Reward     float64 // -1.0 ~ +1.0 (normalized)
	Confidence float64 // 0~1
	Notes      string  // short & redacted
	CreatedAt  time.Time
}

type ModelProfile struct {
	ID string

	Provider string
	Model    string

	// Environment fingerprint
	OS        string
	Arch      string
	MachineFP string

	// Rolling stats
	SampleCount    int64
	AvgLatencyMs   float64
	AvgCostMicros  float64
	AvgQuality     float64
	SuccessRate    float64
	UpdatedAt      time.Time
	FirstSeenAt    time.Time
}

type ModelStats struct {
	Provider string
	Model    string
	Samples  int64

	SuccessRate   float64
	P50LatencyMs  float64
	P90LatencyMs  float64
	AvgLatencyMs  float64
	AvgCostMicros float64
	AvgQuality    float64
}


---

2) internal/okAON/store.go

package okAON

import "context"

// Store is the OkAON persistence boundary.
// Router/Learning/Bench will write runs & rewards here.
type Store interface {
	InsertRun(ctx context.Context, r RunRecord) error
	InsertReward(ctx context.Context, rr RewardRecord) error

	// Read paths
	ListRuns(ctx context.Context, q ListRunsQuery) ([]RunRecord, error)
	GetModelStats(ctx context.Context, q ModelStatsQuery) ([]ModelStats, error)

	// Optional: keep rolling model profile per machine
	UpsertModelProfile(ctx context.Context, p ModelProfile) error
	GetModelProfile(ctx context.Context, q ModelProfileQuery) (*ModelProfile, error)
}

type ListRunsQuery struct {
	OrgID       string
	WorkspaceID string
	ProjectID   string
	SessionID   string
	TaskID      string

	Provider string
	Model    string

	Limit  int
	Offset int
	Desc   bool
}

type ModelStatsQuery struct {
	OrgID       string
	WorkspaceID string
	ProjectID   string

	OS        string
	Arch      string
	MachineFP string

	Category string
	Agent    string

	Provider string
	Model    string

	// last N days (0 = all)
	LastDays int
	Limit    int
}

type ModelProfileQuery struct {
	Provider  string
	Model     string
	OS        string
	Arch      string
	MachineFP string
}


---

3) internal/okAON/sqlite/insert.go

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"devorch/internal/okAON"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) InsertRun(ctx context.Context, r okAON.RunRecord) error {
	if r.ID == "" {
		return fmt.Errorf("okaon: run id is required")
	}
	if r.StartedAt.IsZero() {
		r.StartedAt = time.Now().UTC()
	}
	if r.CompletedAt.IsZero() {
		r.CompletedAt = r.StartedAt
	}
	if r.TotalTokens == 0 {
		r.TotalTokens = r.InputTokens + r.OutputTokens
	}

	const q = `
INSERT INTO okaon_runs (
  id,
  org_id, workspace_id, project_id, session_id, task_id,
  category, agent,
  provider, model, endpoint,
  os, arch, machine_fp, hw_tier, locality, network_type,
  prompt_hash, output_hash, tooling_hint,
  started_at, completed_at,
  latency_ms, input_tokens, output_tokens, total_tokens, cost_micros,
  success, error_class, error_message,
  quality_score
) VALUES (
  ?, ?, ?, ?, ?, ?,
  ?, ?,
  ?, ?, ?,
  ?, ?, ?, ?, ?, ?,
  ?, ?, ?,
  ?, ?,
  ?, ?, ?, ?, ?,
  ?, ?, ?,
  ?
);
`

	_, err := s.db.ExecContext(ctx, q,
		r.ID,
		nullStr(r.OrgID), nullStr(r.WorkspaceID), nullStr(r.ProjectID), nullStr(r.SessionID), nullStr(r.TaskID),
		nullStr(r.Category), nullStr(r.Agent),
		nullStr(r.Provider), nullStr(r.Model), nullStr(r.Endpoint),
		nullStr(r.OS), nullStr(r.Arch), nullStr(r.MachineFP), nullStr(r.HWTier), nullStr(r.Locality), nullStr(r.NetworkType),
		nullStr(r.PromptHash), nullStr(r.OutputHash), nullStr(r.ToolingHint),
		r.StartedAt.UTC().UnixMilli(), r.CompletedAt.UTC().UnixMilli(),
		r.LatencyMs, r.InputTokens, r.OutputTokens, r.TotalTokens, r.CostMicros,
		boolToInt(r.Success), nullStr(r.ErrorClass), nullStr(r.ErrorMessage),
		r.QualityScore,
	)
	if err != nil {
		return fmt.Errorf("okaon: insert run: %w", err)
	}
	return nil
}

func (s *Store) InsertReward(ctx context.Context, rr okAON.RewardRecord) error {
	if rr.ID == "" {
		return fmt.Errorf("okaon: reward id is required")
	}
	if rr.RunID == "" {
		return fmt.Errorf("okaon: reward run_id is required")
	}
	if rr.CreatedAt.IsZero() {
		rr.CreatedAt = time.Now().UTC()
	}

	const q = `
INSERT INTO okaon_rewards (
  id, run_id,
  source, kind,
  reward, confidence,
  notes,
  created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);
`

	_, err := s.db.ExecContext(ctx, q,
		rr.ID, rr.RunID,
		nullStr(rr.Source), nullStr(rr.Kind),
		rr.Reward, rr.Confidence,
		nullStr(rr.Notes),
		rr.CreatedAt.UTC().UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("okaon: insert reward: %w", err)
	}
	return nil
}

func (s *Store) UpsertModelProfile(ctx context.Context, p okAON.ModelProfile) error {
	if p.ID == "" {
		return fmt.Errorf("okaon: profile id is required")
	}
	if p.Provider == "" || p.Model == "" {
		return fmt.Errorf("okaon: profile provider/model required")
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = time.Now().UTC()
	}
	if p.FirstSeenAt.IsZero() {
		p.FirstSeenAt = p.UpdatedAt
	}

	const q = `
INSERT INTO okaon_model_profiles (
  id, provider, model,
  os, arch, machine_fp,
  sample_count, avg_latency_ms, avg_cost_micros, avg_quality, success_rate,
  first_seen_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(provider, model, os, arch, machine_fp) DO UPDATE SET
  sample_count   = excluded.sample_count,
  avg_latency_ms = excluded.avg_latency_ms,
  avg_cost_micros= excluded.avg_cost_micros,
  avg_quality    = excluded.avg_quality,
  success_rate   = excluded.success_rate,
  updated_at     = excluded.updated_at;
`

	_, err := s.db.ExecContext(ctx, q,
		p.ID, p.Provider, p.Model,
		nullStr(p.OS), nullStr(p.Arch), nullStr(p.MachineFP),
		p.SampleCount, p.AvgLatencyMs, p.AvgCostMicros, p.AvgQuality, p.SuccessRate,
		p.FirstSeenAt.UTC().UnixMilli(), p.UpdatedAt.UTC().UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("okaon: upsert model profile: %w", err)
	}
	return nil
}

// helpers
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}


---

4) internal/okAON/sqlite/query.go

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"devorch/internal/okAON"
)

func (s *Store) ListRuns(ctx context.Context, q okAON.ListRunsQuery) ([]okAON.RunRecord, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	var where []string
	var args []any

	// scoped filters
	addEq := func(col, v string) {
		if v == "" {
			return
		}
		where = append(where, col+" = ?")
		args = append(args, v)
	}

	addEq("org_id", q.OrgID)
	addEq("workspace_id", q.WorkspaceID)
	addEq("project_id", q.ProjectID)
	addEq("session_id", q.SessionID)
	addEq("task_id", q.TaskID)
	addEq("provider", q.Provider)
	addEq("model", q.Model)

	sb := strings.Builder{}
	sb.WriteString(`
SELECT
  id,
  org_id, workspace_id, project_id, session_id, task_id,
  category, agent,
  provider, model, endpoint,
  os, arch, machine_fp, hw_tier, locality, network_type,
  prompt_hash, output_hash, tooling_hint,
  started_at, completed_at,
  latency_ms, input_tokens, output_tokens, total_tokens, cost_micros,
  success, error_class, error_message,
  quality_score
FROM okaon_runs
`)

	if len(where) > 0 {
		sb.WriteString("WHERE ")
		sb.WriteString(strings.Join(where, " AND "))
		sb.WriteString("\n")
	}

	if q.Desc {
		sb.WriteString("ORDER BY started_at DESC\n")
	} else {
		sb.WriteString("ORDER BY started_at ASC\n")
	}

	sb.WriteString("LIMIT ? OFFSET ?;")
	args = append(args, limit, q.Offset)

	rows, err := s.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("okaon: list runs: %w", err)
	}
	defer rows.Close()

	var out []okAON.RunRecord
	for rows.Next() {
		var r okAON.RunRecord

		var startedMs, completedMs int64
		var successInt int

		// nullable cols
		var (
			orgID, wsID, projID, sessID, taskID sql.NullString
			category, agent                      sql.NullString
			provider, model, endpoint            sql.NullString
			osv, arch, mfp                       sql.NullString
			hwTier, locality, netType            sql.NullString
			pHash, oHash, toolHint               sql.NullString
			errClass, errMsg                     sql.NullString
		)

		if err := rows.Scan(
			&r.ID,
			&orgID, &wsID, &projID, &sessID, &taskID,
			&category, &agent,
			&provider, &model, &endpoint,
			&osv, &arch, &mfp, &hwTier, &locality, &netType,
			&pHash, &oHash, &toolHint,
			&startedMs, &completedMs,
			&r.LatencyMs, &r.InputTokens, &r.OutputTokens, &r.TotalTokens, &r.CostMicros,
			&successInt, &errClass, &errMsg,
			&r.QualityScore,
		); err != nil {
			return nil, fmt.Errorf("okaon: list runs scan: %w", err)
		}

		r.OrgID = orgID.String
		r.WorkspaceID = wsID.String
		r.ProjectID = projID.String
		r.SessionID = sessID.String
		r.TaskID = taskID.String
		r.Category = category.String
		r.Agent = agent.String
		r.Provider = provider.String
		r.Model = model.String
		r.Endpoint = endpoint.String
		r.OS = osv.String
		r.Arch = arch.String
		r.MachineFP = mfp.String
		r.HWTier = hwTier.String
		r.Locality = locality.String
		r.NetworkType = netType.String
		r.PromptHash = pHash.String
		r.OutputHash = oHash.String
		r.ToolingHint = toolHint.String
		r.ErrorClass = errClass.String
		r.ErrorMessage = errMsg.String
		r.Success = successInt == 1

		r.StartedAt = millisToTime(startedMs)
		r.CompletedAt = millisToTime(completedMs)

		out = append(out, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("okaon: list runs rows: %w", err)
	}

	return out, nil
}

func (s *Store) GetModelProfile(ctx context.Context, q okAON.ModelProfileQuery) (*okAON.ModelProfile, error) {
	if q.Provider == "" || q.Model == "" {
		return nil, fmt.Errorf("okaon: provider/model required")
	}

	const sqlq = `
SELECT
  id, provider, model,
  os, arch, machine_fp,
  sample_count, avg_latency_ms, avg_cost_micros, avg_quality, success_rate,
  first_seen_at, updated_at
FROM okaon_model_profiles
WHERE provider=? AND model=? AND os=? AND arch=? AND machine_fp=?
LIMIT 1;
`

	row := s.db.QueryRowContext(ctx, sqlq,
		q.Provider, q.Model,
		q.OS, q.Arch, q.MachineFP,
	)

	var p okAON.ModelProfile
	var firstMs, updMs int64

	var osV, archV, fpV sql.NullString
	if err := row.Scan(
		&p.ID, &p.Provider, &p.Model,
		&osV, &archV, &fpV,
		&p.SampleCount, &p.AvgLatencyMs, &p.AvgCostMicros, &p.AvgQuality, &p.SuccessRate,
		&firstMs, &updMs,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("okaon: get model profile: %w", err)
	}

	p.OS = osV.String
	p.Arch = archV.String
	p.MachineFP = fpV.String
	p.FirstSeenAt = millisToTime(firstMs)
	p.UpdatedAt = millisToTime(updMs)
	return &p, nil
}


---

5) internal/okAON/sqlite/stats.go

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"devorch/internal/okAON"
)

func (s *Store) GetModelStats(ctx context.Context, q okAON.ModelStatsQuery) ([]okAON.ModelStats, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	var where []string
	var args []any

	addEq := func(col, v string) {
		if v == "" {
			return
		}
		where = append(where, col+" = ?")
		args = append(args, v)
	}

	addEq("org_id", q.OrgID)
	addEq("workspace_id", q.WorkspaceID)
	addEq("project_id", q.ProjectID)

	addEq("os", q.OS)
	addEq("arch", q.Arch)
	addEq("machine_fp", q.MachineFP)

	addEq("category", q.Category)
	addEq("agent", q.Agent)

	addEq("provider", q.Provider)
	addEq("model", q.Model)

	if q.LastDays > 0 {
		cutoff := time.Now().UTC().Add(-time.Duration(q.LastDays) * 24 * time.Hour).UnixMilli()
		where = append(where, "started_at >= ?")
		args = append(args, cutoff)
	}

	// NOTE:
	// SQLite에서 정확한 p50/p90을 "정확히" 계산하려면 윈도우 함수 + 정렬이 필요합니다.
	// 여기서는 "근사(실무 충분)"로:
	// - avg latency
	// - success rate
	// - avg cost / quality
	// - p50/p90: 정렬 후 offset 기반(정확)
	//
	// 정확 p50/p90을 위해 2쿼리 수행:
	// 1) 그룹별 샘플 수 집계
	// 2) 각 그룹별 p50/p90 latency를 별도 subquery로 산출

	// 1) group aggregate
	sb := strings.Builder{}
	sb.WriteString(`
SELECT
  provider,
  model,
  COUNT(*) AS samples,
  AVG(latency_ms) AS avg_latency_ms,
  AVG(cost_micros) AS avg_cost_micros,
  AVG(quality_score) AS avg_quality,
  AVG(CASE WHEN success=1 THEN 1.0 ELSE 0.0 END) AS success_rate
FROM okaon_runs
`)
	if len(where) > 0 {
		sb.WriteString("WHERE " + strings.Join(where, " AND ") + "\n")
	}
	sb.WriteString("GROUP BY provider, model\n")
	sb.WriteString("ORDER BY samples DESC\n")
	sb.WriteString("LIMIT ?;")
	argsAgg := append(args, limit)

	rows, err := s.db.QueryContext(ctx, sb.String(), argsAgg...)
	if err != nil {
		return nil, fmt.Errorf("okaon: model stats agg: %w", err)
	}
	defer rows.Close()

	type aggRow struct {
		provider      string
		model         string
		samples       int64
		avgLatencyMs  float64
		avgCostMicros float64
		avgQuality    float64
		successRate   float64
	}

	var aggs []aggRow
	for rows.Next() {
		var a aggRow
		if err := rows.Scan(
			&a.provider,
			&a.model,
			&a.samples,
			&a.avgLatencyMs,
			&a.avgCostMicros,
			&a.avgQuality,
			&a.successRate,
		); err != nil {
			return nil, fmt.Errorf("okaon: model stats agg scan: %w", err)
		}
		aggs = append(aggs, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("okaon: model stats agg rows: %w", err)
	}

	// 2) p50/p90 per group (exact by ORDER+OFFSET)
	out := make([]okAON.ModelStats, 0, len(aggs))
	for _, a := range aggs {
		p50, p90, err := s.getP50P90LatencyMs(ctx, q, a.provider, a.model, a.samples)
		if err != nil {
			return nil, err
		}

		out = append(out, okAON.ModelStats{
			Provider: a.provider,
			Model:    a.model,
			Samples:  a.samples,

			SuccessRate:   a.successRate,
			P50LatencyMs:  p50,
			P90LatencyMs:  p90,
			AvgLatencyMs:  a.avgLatencyMs,
			AvgCostMicros: a.avgCostMicros,
			AvgQuality:    a.avgQuality,
		})
	}

	return out, nil
}

func (s *Store) getP50P90LatencyMs(ctx context.Context, q okAON.ModelStatsQuery, provider, model string, samples int64) (float64, float64, error) {
	if samples <= 0 {
		return 0, 0, nil
	}

	// rebuild same filters but with provider/model fixed
	var where []string
	var args []any

	addEq := func(col, v string) {
		if v == "" {
			return
		}
		where = append(where, col+" = ?")
		args = append(args, v)
	}

	addEq("org_id", q.OrgID)
	addEq("workspace_id", q.WorkspaceID)
	addEq("project_id", q.ProjectID)

	addEq("os", q.OS)
	addEq("arch", q.Arch)
	addEq("machine_fp", q.MachineFP)

	addEq("category", q.Category)
	addEq("agent", q.Agent)

	// fixed group
	where = append(where, "provider = ?")
	args = append(args, provider)
	where = append(where, "model = ?")
	args = append(args, model)

	if q.LastDays > 0 {
		cutoff := time.Now().UTC().Add(-time.Duration(q.LastDays) * 24 * time.Hour).UnixMilli()
		where = append(where, "started_at >= ?")
		args = append(args, cutoff)
	}

	// percentile indices
	// p50: median
	p50Idx := int64((samples - 1) * 50 / 100)
	p90Idx := int64((samples - 1) * 90 / 100)

	getAt := func(idx int64) (float64, error) {
		sb := strings.Builder{}
		sb.WriteString("SELECT latency_ms FROM okaon_runs ")
		sb.WriteString("WHERE " + strings.Join(where, " AND ") + " ")
		sb.WriteString("ORDER BY latency_ms ASC ")
		sb.WriteString("LIMIT 1 OFFSET ?;")

		row := s.db.QueryRowContext(ctx, sb.String(), append(args, idx)...)
		var v sql.NullFloat64
		if err := row.Scan(&v); err != nil {
			return 0, fmt.Errorf("okaon: percentile scan: %w", err)
		}
		if !v.Valid {
			return 0, nil
		}
		return v.Float64, nil
	}

	p50, err := getAt(p50Idx)
	if err != nil {
		return 0, 0, err
	}
	p90, err := getAt(p90Idx)
	if err != nil {
		return 0, 0, err
	}
	return p50, p90, nil
}

func millisToTime(ms int64) time.Time {
	// shared helper for query.go too (keep duplicated minimal)
	return time.UnixMilli(ms).UTC()
}

> 위 millisToTime()는 query.go에서도 쓰이므로 중복 없이 쓰려면 util.go로 빼도 되지만, 단계 분리상 지금은 그대로 둡니다.
컴파일 에러 없이 쓰려면 query.go에도 동일 함수가 있거나, 여기 함수만 두고 query.go에서는 이 파일의 함수를 쓰도록 정리하면 됩니다.



✅ 가장 깔끔한 방법은 아래 util.go를 추가하는 건데, 요청 트리에 없어서 이번 단계에서는 포함하지 않았습니다.
(원하면 36.1로 정리해드릴게요)


---

6) internal/storage/sqlite/migrations/0003_okAON_learning.sql

-- 0003_okAON_learning.sql
-- OkAON: local measured data lake for routing/learning
-- Keep only hashes / redacted fields. No raw prompt/output.

PRAGMA foreign_keys = ON;

-- runs: each execution measurement
CREATE TABLE IF NOT EXISTS okaon_runs (
  id TEXT PRIMARY KEY,

  org_id TEXT,
  workspace_id TEXT,
  project_id TEXT,
  session_id TEXT,
  task_id TEXT,

  category TEXT,
  agent TEXT,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  endpoint TEXT,

  os TEXT,
  arch TEXT,
  machine_fp TEXT,
  hw_tier TEXT,
  locality TEXT,
  network_type TEXT,

  prompt_hash TEXT,
  output_hash TEXT,
  tooling_hint TEXT,

  started_at INTEGER NOT NULL,
  completed_at INTEGER NOT NULL,

  latency_ms INTEGER NOT NULL DEFAULT 0,
  input_tokens INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  cost_micros INTEGER NOT NULL DEFAULT 0,

  success INTEGER NOT NULL DEFAULT 1,
  error_class TEXT,
  error_message TEXT,

  quality_score REAL NOT NULL DEFAULT 0.0
);

CREATE INDEX IF NOT EXISTS idx_okaon_runs_time
  ON okaon_runs(started_at);

CREATE INDEX IF NOT EXISTS idx_okaon_runs_scope
  ON okaon_runs(org_id, workspace_id, project_id, session_id, task_id);

CREATE INDEX IF NOT EXISTS idx_okaon_runs_model
  ON okaon_runs(provider, model);

CREATE INDEX IF NOT EXISTS idx_okaon_runs_env
  ON okaon_runs(os, arch, machine_fp);

CREATE INDEX IF NOT EXISTS idx_okaon_runs_cat_agent
  ON okaon_runs(category, agent);

-- rewards: feedback signal bound to a run
CREATE TABLE IF NOT EXISTS okaon_rewards (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,

  source TEXT,
  kind TEXT,

  reward REAL NOT NULL DEFAULT 0.0,
  confidence REAL NOT NULL DEFAULT 1.0,
  notes TEXT,

  created_at INTEGER NOT NULL,

  FOREIGN KEY(run_id) REFERENCES okaon_runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_okaon_rewards_run
  ON okaon_rewards(run_id);

CREATE INDEX IF NOT EXISTS idx_okaon_rewards_time
  ON okaon_rewards(created_at);

-- model profile: per machine rolling aggregate
CREATE TABLE IF NOT EXISTS okaon_model_profiles (
  id TEXT NOT NULL,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,

  os TEXT NOT NULL,
  arch TEXT NOT NULL,
  machine_fp TEXT NOT NULL,

  sample_count INTEGER NOT NULL DEFAULT 0,
  avg_latency_ms REAL NOT NULL DEFAULT 0.0,
  avg_cost_micros REAL NOT NULL DEFAULT 0.0,
  avg_quality REAL NOT NULL DEFAULT 0.0,
  success_rate REAL NOT NULL DEFAULT 0.0,

  first_seen_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,

  PRIMARY KEY(provider, model, os, arch, machine_fp)
);

CREATE INDEX IF NOT EXISTS idx_okaon_profiles_updated
  ON okaon_model_profiles(updated_at);


---

36단계 “연결 방법” (짧게)

다음 단계(37)에서 Router/Learning이 OkAON을 쓰게 하려면:

internal/app/app.go에서 DB 열린 뒤

okAONStore := okAONSqlite.New(app.DB())

이걸 router/bench/learning에 주입



---

원하시면 제가 37단계로 바로:

internal/bench/recorder.go에서 InsertRun() 실제 호출

internal/bench/reward_emit.go에서 InsertReward() 호출

그리고 router/learning_adapter.go가 GetModelStats() 기반으로 가산점 적용


까지 풀코드로 이어서 붙이겠습니다.