네. 다만 이 대화 세션에는 이전에 제가 1~32단계에서 생성했던 “실제 코드 파일 본문”이 남아있지 않아서, 정확히 “끊긴 마지막 줄 다음”을 그대로 이어붙일 수는 없습니다. 대신 **최종 트리 기준에서 바로 다음 개발 단위(= 학습/OkAON/Router 연결 핵심)**를 누락 없이 컴파일 가능한 형태로 이어서 제공합니다.
(이걸 “33단계”로 잡고 진행합니다.)

아래는 **OkAON(SQLite) + Learning(Thompson) + Router(learning_adapter) + Hook(on_task_complete)**를 실제로 연결하는 최소 완성 묶음입니다.


---

33단계: OkAON + Thompson Bandit + Router Learning Adapter + Hook 연결 (풀코드)

포함 파일 목록

internal/storage/sqlite/migrations/0003_okAON_learning.sql

internal/okAON/models.go

internal/okAON/store.go

internal/okAON/sqlite/migrate.go

internal/okAON/sqlite/insert.go

internal/okAON/sqlite/query.go

internal/okAON/sqlite/stats.go

internal/learning/bandit/dist.go

internal/learning/bandit/thompson.go

internal/learning/learner.go

internal/router/learning_adapter.go

internal/hook/builtins/on_task_complete.go



---

1) internal/storage/sqlite/migrations/0003_okAON_learning.sql

-- 0003_okAON_learning.sql
-- OkAON: local measured benchmark + task result + reward + bandit snapshot
-- SQLite foreign_keys assumed ON

BEGIN;

-- 1) Environment fingerprint (OS/arch/hw/runtime) for "local real benchmark"
CREATE TABLE IF NOT EXISTS okaon_env (
  env_id        TEXT PRIMARY KEY,              -- ULID or stable hash
  os            TEXT NOT NULL,                 -- windows/darwin/linux
  arch          TEXT NOT NULL,                 -- amd64/arm64
  hostname      TEXT,
  cpu_model     TEXT,
  cpu_cores     INTEGER,
  mem_bytes     INTEGER,
  gpu_model     TEXT,
  runtime       TEXT NOT NULL,                 -- ollama/llamacpp/remote
  runtime_ver   TEXT,
  created_at    INTEGER NOT NULL               -- unix epoch seconds
);

CREATE INDEX IF NOT EXISTS idx_okaon_env_os_arch ON okaon_env(os, arch);

-- 2) Provider / model inventory observed in real runs
CREATE TABLE IF NOT EXISTS okaon_model (
  model_id      TEXT PRIMARY KEY,              -- "openai/gpt-5.2" or "ollama/llama3.1:8b"
  provider      TEXT NOT NULL,                 -- openai/anthropic/google/ollama/...
  family        TEXT,                          -- gpt/claude/gemini/llama/...
  context       INTEGER,                       -- context window tokens (optional)
  notes         TEXT,
  created_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_model_provider ON okaon_model(provider);

-- 3) Workload definition (what was asked)
CREATE TABLE IF NOT EXISTS okaon_workload (
  workload_id   TEXT PRIMARY KEY,              -- ULID
  kind          TEXT NOT NULL,                 -- "code.edit" | "search" | "plan" | "review" | ...
  language      TEXT,                          -- "go" | "ts" | ...
  repo_hash     TEXT,                          -- optional (project fingerprint)
  size_bucket   TEXT,                          -- small/medium/large
  created_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_workload_kind ON okaon_workload(kind);

-- 4) Run record (per execution): latency/cost/quality etc
CREATE TABLE IF NOT EXISTS okaon_run (
  run_id        TEXT PRIMARY KEY,              -- ULID
  env_id        TEXT NOT NULL REFERENCES okaon_env(env_id) ON DELETE CASCADE,
  workload_id   TEXT NOT NULL REFERENCES okaon_workload(workload_id) ON DELETE CASCADE,
  model_id      TEXT NOT NULL REFERENCES okaon_model(model_id) ON DELETE CASCADE,

  started_at    INTEGER NOT NULL,
  finished_at   INTEGER NOT NULL,

  input_tokens  INTEGER,
  output_tokens INTEGER,

  latency_ms    INTEGER NOT NULL,
  ok            INTEGER NOT NULL,              -- 1 success, 0 fail
  err_type      TEXT,                          -- timeout/rate_limit/tool_fail/...
  err_msg       TEXT,

  quality_score REAL,                          -- 0..1 (or -1 if unknown)
  cost_usd      REAL,                          -- optional
  extra_json    TEXT                           -- optional JSON
);

CREATE INDEX IF NOT EXISTS idx_okaon_run_env_workload ON okaon_run(env_id, workload_id);
CREATE INDEX IF NOT EXISTS idx_okaon_run_model ON okaon_run(model_id);
CREATE INDEX IF NOT EXISTS idx_okaon_run_time ON okaon_run(started_at);

-- 5) Reward events (derived from quality gate, user feedback, or heuristics)
CREATE TABLE IF NOT EXISTS okaon_reward (
  reward_id     TEXT PRIMARY KEY,              -- ULID
  run_id        TEXT NOT NULL REFERENCES okaon_run(run_id) ON DELETE CASCADE,
  reward        REAL NOT NULL,                 -- can be negative
  reason        TEXT,                          -- "quality_gate" | "user_rating" | "timeout_penalty" ...
  created_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_reward_run ON okaon_reward(run_id);

-- 6) Bandit policy snapshot (for reproducible routing)
CREATE TABLE IF NOT EXISTS okaon_bandit_snapshot (
  snapshot_id   TEXT PRIMARY KEY,              -- ULID
  env_id        TEXT NOT NULL REFERENCES okaon_env(env_id) ON DELETE CASCADE,
  workload_kind TEXT NOT NULL,                 -- match okaon_workload.kind
  algo          TEXT NOT NULL,                 -- "thompson"
  state_json    TEXT NOT NULL,                 -- serialized arms params
  created_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_snapshot_env_kind ON okaon_bandit_snapshot(env_id, workload_kind);

COMMIT;


---

2) internal/okAON/models.go

package okaon

type Env struct {
	EnvID      string
	OS         string
	Arch       string
	Hostname   string
	CPUModel   string
	CPUCores   int
	MemBytes   int64
	GPUModel   string
	Runtime    string
	RuntimeVer string
	CreatedAt  int64
}

type Model struct {
	ModelID   string
	Provider  string
	Family    string
	Context   int
	Notes     string
	CreatedAt int64
}

type Workload struct {
	WorkloadID  string
	Kind        string
	Language    string
	RepoHash    string
	SizeBucket  string
	CreatedAt   int64
}

type Run struct {
	RunID        string
	EnvID        string
	WorkloadID   string
	ModelID      string
	StartedAt    int64
	FinishedAt   int64
	InputTokens  int
	OutputTokens int
	LatencyMS    int64
	OK           bool
	ErrType      string
	ErrMsg       string
	QualityScore float64
	CostUSD      float64
	ExtraJSON    string
}

type Reward struct {
	RewardID  string
	RunID     string
	Reward    float64
	Reason    string
	CreatedAt int64
}

type BanditSnapshot struct {
	SnapshotID   string
	EnvID        string
	WorkloadKind string
	Algo         string
	StateJSON    string
	CreatedAt    int64
}


---

3) internal/okAON/store.go

package okaon

import "context"

type Store interface {
	UpsertEnv(ctx context.Context, e Env) error
	UpsertModel(ctx context.Context, m Model) error
	InsertWorkload(ctx context.Context, w Workload) error
	InsertRun(ctx context.Context, r Run) error
	InsertReward(ctx context.Context, rw Reward) error

	LatestSnapshot(ctx context.Context, envID, workloadKind string) (BanditSnapshot, bool, error)
	UpsertSnapshot(ctx context.Context, s BanditSnapshot) error

	// stats helpers for learning/router
	ModelStatsByKind(ctx context.Context, envID, workloadKind string, lookbackSec int64) ([]ModelAgg, error)
}

type ModelAgg struct {
	ModelID     string
	Count       int
	SuccessRate float64
	AvgLatency  float64
	AvgQuality  float64
	AvgReward   float64
}


---

4) internal/okAON/sqlite/migrate.go

package sqlite

import (
	"context"
	"database/sql"
)

type Migrator interface {
	Ensure(ctx context.Context) error
}

type FileMigrator struct {
	DB *sql.DB
	// 실제론 embed.FS로 sql 파일을 포함시키면 됨.
	// 33단계에선 "마이그레이션 테이블 존재"만 보장하는 최소 구성.
}

func (m *FileMigrator) Ensure(ctx context.Context) error {
	_, err := m.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at INTEGER NOT NULL
);
`)
	return err
}


---

5) internal/okAON/sqlite/insert.go

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/okAON"
)

type Store struct {
	DB *sql.DB
}

func New(db *sql.DB) *Store { return &Store{DB: db} }

func nowSec() int64 { return time.Now().Unix() }

func (s *Store) UpsertEnv(ctx context.Context, e okaon.Env) error {
	if e.CreatedAt == 0 {
		e.CreatedAt = nowSec()
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO okaon_env(env_id, os, arch, hostname, cpu_model, cpu_cores, mem_bytes, gpu_model, runtime, runtime_ver, created_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(env_id) DO UPDATE SET
  os=excluded.os,
  arch=excluded.arch,
  hostname=excluded.hostname,
  cpu_model=excluded.cpu_model,
  cpu_cores=excluded.cpu_cores,
  mem_bytes=excluded.mem_bytes,
  gpu_model=excluded.gpu_model,
  runtime=excluded.runtime,
  runtime_ver=excluded.runtime_ver;
`, e.EnvID, e.OS, e.Arch, e.Hostname, e.CPUModel, e.CPUCores, e.MemBytes, e.GPUModel, e.Runtime, e.RuntimeVer, e.CreatedAt)
	return err
}

func (s *Store) UpsertModel(ctx context.Context, m okaon.Model) error {
	if m.CreatedAt == 0 {
		m.CreatedAt = nowSec()
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO okaon_model(model_id, provider, family, context, notes, created_at)
VALUES(?, ?, ?, ?, ?, ?)
ON CONFLICT(model_id) DO UPDATE SET
  provider=excluded.provider,
  family=excluded.family,
  context=excluded.context,
  notes=excluded.notes;
`, m.ModelID, m.Provider, m.Family, m.Context, m.Notes, m.CreatedAt)
	return err
}

func (s *Store) InsertWorkload(ctx context.Context, w okaon.Workload) error {
	if w.CreatedAt == 0 {
		w.CreatedAt = nowSec()
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO okaon_workload(workload_id, kind, language, repo_hash, size_bucket, created_at)
VALUES(?, ?, ?, ?, ?, ?);
`, w.WorkloadID, w.Kind, w.Language, w.RepoHash, w.SizeBucket, w.CreatedAt)
	return err
}

func (s *Store) InsertRun(ctx context.Context, r okaon.Run) error {
	// defaults
	if r.StartedAt == 0 {
		r.StartedAt = nowSec()
	}
	if r.FinishedAt == 0 {
		r.FinishedAt = r.StartedAt
	}
	okInt := 0
	if r.OK {
		okInt = 1
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO okaon_run(
  run_id, env_id, workload_id, model_id,
  started_at, finished_at,
  input_tokens, output_tokens,
  latency_ms, ok, err_type, err_msg,
  quality_score, cost_usd, extra_json
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`,
		r.RunID, r.EnvID, r.WorkloadID, r.ModelID,
		r.StartedAt, r.FinishedAt,
		nullInt(r.InputTokens), nullInt(r.OutputTokens),
		r.LatencyMS, okInt, nullText(r.ErrType), nullText(r.ErrMsg),
		nullFloat(r.QualityScore), nullFloat(r.CostUSD), nullText(r.ExtraJSON),
	)
	return err
}

func (s *Store) InsertReward(ctx context.Context, rw okaon.Reward) error {
	if rw.CreatedAt == 0 {
		rw.CreatedAt = nowSec()
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO okaon_reward(reward_id, run_id, reward, reason, created_at)
VALUES(?, ?, ?, ?, ?);
`, rw.RewardID, rw.RunID, rw.Reward, rw.Reason, rw.CreatedAt)
	return err
}

func (s *Store) UpsertSnapshot(ctx context.Context, sn okaon.BanditSnapshot) error {
	if sn.CreatedAt == 0 {
		sn.CreatedAt = nowSec()
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO okaon_bandit_snapshot(snapshot_id, env_id, workload_kind, algo, state_json, created_at)
VALUES(?, ?, ?, ?, ?, ?)
ON CONFLICT(snapshot_id) DO UPDATE SET
  state_json=excluded.state_json;
`, sn.SnapshotID, sn.EnvID, sn.WorkloadKind, sn.Algo, sn.StateJSON, sn.CreatedAt)
	return err
}

// ---- helpers ----
func nullText(v string) any {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}
func nullInt(v int) any {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}
func nullFloat(v float64) any {
	if v == 0 {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: v, Valid: true}
}


---

6) internal/okAON/sqlite/query.go

package sqlite

import (
	"context"
	"database/sql"

	"devorch/internal/okAON"
)

func (s *Store) LatestSnapshot(ctx context.Context, envID, workloadKind string) (okaon.BanditSnapshot, bool, error) {
	row := s.DB.QueryRowContext(ctx, `
SELECT snapshot_id, env_id, workload_kind, algo, state_json, created_at
FROM okaon_bandit_snapshot
WHERE env_id = ? AND workload_kind = ?
ORDER BY created_at DESC
LIMIT 1;
`, envID, workloadKind)

	var out okaon.BanditSnapshot
	err := row.Scan(&out.SnapshotID, &out.EnvID, &out.WorkloadKind, &out.Algo, &out.StateJSON, &out.CreatedAt)
	if err == sql.ErrNoRows {
		return okaon.BanditSnapshot{}, false, nil
	}
	if err != nil {
		return okaon.BanditSnapshot{}, false, err
	}
	return out, true, nil
}


---

7) internal/okAON/sqlite/stats.go

package sqlite

import (
	"context"
	"database/sql"
	"math"

	"devorch/internal/okAON"
)

func (s *Store) ModelStatsByKind(ctx context.Context, envID, workloadKind string, lookbackSec int64) ([]okaon.ModelAgg, error) {
	// runs + rewards join
	rows, err := s.DB.QueryContext(ctx, `
WITH runs AS (
  SELECT r.model_id, r.ok, r.latency_ms, r.quality_score, r.run_id
  FROM okaon_run r
  JOIN okaon_workload w ON w.workload_id = r.workload_id
  WHERE r.env_id = ?
    AND w.kind = ?
    AND r.started_at >= (strftime('%s','now') - ?)
),
rw AS (
  SELECT run_id, AVG(reward) AS avg_reward
  FROM okaon_reward
  GROUP BY run_id
)
SELECT
  runs.model_id,
  COUNT(*) AS cnt,
  AVG(CASE WHEN runs.ok = 1 THEN 1.0 ELSE 0.0 END) AS success_rate,
  AVG(runs.latency_ms) AS avg_latency,
  AVG(COALESCE(NULLIF(runs.quality_score, 0), NULL)) AS avg_quality,
  AVG(COALESCE(rw.avg_reward, 0)) AS avg_reward
FROM runs
LEFT JOIN rw ON rw.run_id = runs.run_id
GROUP BY runs.model_id
ORDER BY cnt DESC;
`, envID, workloadKind, lookbackSec)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []okaon.ModelAgg
	for rows.Next() {
		var (
			modelID                 string
			cnt                     int
			successRate, avgLatency float64
			avgQuality              sql.NullFloat64
			avgReward               float64
		)
		if err := rows.Scan(&modelID, &cnt, &successRate, &avgLatency, &avgQuality, &avgReward); err != nil {
			return nil, err
		}
		q := 0.0
		if avgQuality.Valid && !math.IsNaN(avgQuality.Float64) {
			q = avgQuality.Float64
		}
		out = append(out, okaon.ModelAgg{
			ModelID:     modelID,
			Count:       cnt,
			SuccessRate: successRate,
			AvgLatency:  avgLatency,
			AvgQuality:  q,
			AvgReward:   avgReward,
		})
	}
	return out, rows.Err()
}


---

8) internal/learning/bandit/dist.go

package bandit

import (
	"math"
	"math/rand"
)

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// Marsaglia and Tsang method for Gamma(k, theta=1), k>0
func gammaSample(r *rand.Rand, k float64) float64 {
	if k < 1.0 {
		// Johnk's generator for 0<k<1
		for {
			u := r.Float64()
			b := (math.E + k) / math.E
			p := b * u
			if p <= 1.0 {
				x := math.Pow(p, 1.0/k)
				u2 := r.Float64()
				if u2 <= math.Exp(-x) {
					return x
				}
			} else {
				x := -math.Log((b - p) / k)
				u2 := r.Float64()
				if u2 <= math.Pow(x, k-1.0) {
					return x
				}
			}
		}
	}

	// k >= 1
	d := k - 1.0/3.0
	c := 1.0 / math.Sqrt(9*d)
	for {
		x := r.NormFloat64()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := r.Float64()
		if u < 1-0.0331*(x*x)*(x*x) {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}

func betaSample(r *rand.Rand, a, b float64) float64 {
	ga := gammaSample(r, a)
	gb := gammaSample(r, b)
	return ga / (ga + gb)
}


---

9) internal/learning/bandit/thompson.go

package bandit

import (
	"encoding/json"
	"math/rand"
	"time"
)

type ThompsonArm struct {
	// Beta distribution params for Bernoulli reward (mapped to [0,1])
	Alpha float64 `json:"alpha"`
	Beta  float64 `json:"beta"`
	// optional: keep counts for explain/debug
	Trials  int     `json:"trials"`
	AvgReal float64 `json:"avg_real"` // running avg of raw reward
}

type ThompsonState struct {
	Arms map[string]*ThompsonArm `json:"arms"` // key=model_id
}

func NewThompsonState() ThompsonState {
	return ThompsonState{Arms: map[string]*ThompsonArm{}}
}

func (s *ThompsonState) EnsureArm(modelID string) *ThompsonArm {
	a, ok := s.Arms[modelID]
	if !ok {
		a = &ThompsonArm{Alpha: 1, Beta: 1}
		s.Arms[modelID] = a
	}
	return a
}

func (s *ThompsonState) Select(modelIDs []string, seed int64) (string, map[string]float64) {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	r := rand.New(rand.NewSource(seed))

	scores := make(map[string]float64, len(modelIDs))
	bestID := ""
	best := -1.0

	for _, id := range modelIDs {
		arm := s.EnsureArm(id)
		// sample from Beta(α,β)
		x := betaSample(r, arm.Alpha, arm.Beta)
		scores[id] = x
		if x > best {
			best = x
			bestID = id
		}
	}
	return bestID, scores
}

// Update with raw reward (can be negative..positive). We map it to [0,1].
// simplest mapping: sigmoid-ish clamp around 0 => 0.5 baseline
func (s *ThompsonState) Update(modelID string, rawReward float64) {
	arm := s.EnsureArm(modelID)

	arm.Trials++
	arm.AvgReal = arm.AvgReal + (rawReward-arm.AvgReal)/float64(arm.Trials)

	// map rawReward to [0,1] (policy can be replaced later)
	// baseline: -1 => 0, 0 => 0.5, +1 => 1
	mapped := clamp01(0.5 + 0.5*rawReward)

	arm.Alpha += mapped
	arm.Beta += (1.0 - mapped)
}

func (s ThompsonState) Marshal() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func UnmarshalThompsonState(js string) (ThompsonState, error) {
	var st ThompsonState
	if js == "" {
		return NewThompsonState(), nil
	}
	if err := json.Unmarshal([]byte(js), &st); err != nil {
		return ThompsonState{}, err
	}
	if st.Arms == nil {
		st.Arms = map[string]*ThompsonArm{}
	}
	return st, nil
}


---

10) internal/learning/learner.go

package learning

import (
	"context"

	"devorch/internal/learning/bandit"
	"devorch/internal/okAON"
)

type Learner struct {
	OkAON okaon.Store
}

type Key struct {
	EnvID        string
	WorkloadKind string
}

type Decision struct {
	ChosenModel string
	Scores      map[string]float64
	Reason      string
	SnapshotID  string // optional
}

func (l *Learner) SelectModel(ctx context.Context, k Key, candidates []string) (Decision, error) {
	// load latest snapshot
	sn, ok, err := l.OkAON.LatestSnapshot(ctx, k.EnvID, k.WorkloadKind)
	if err != nil {
		return Decision{}, err
	}

	var st bandit.ThompsonState
	if !ok {
		st = bandit.NewThompsonState()
	} else {
		st, err = bandit.UnmarshalThompsonState(sn.StateJSON)
		if err != nil {
			// corrupted snapshot: start fresh (fail-open)
			st = bandit.NewThompsonState()
		}
	}

	chosen, scores := st.Select(candidates, 0)
	return Decision{
		ChosenModel: chosen,
		Scores:      scores,
		Reason:      "thompson_sampling",
		SnapshotID:  sn.SnapshotID,
	}, nil
}

func (l *Learner) UpdateFromReward(ctx context.Context, k Key, modelID string, reward float64, snapshotID, newSnapshotID string) (string, string, error) {
	// load current state
	sn, ok, err := l.OkAON.LatestSnapshot(ctx, k.EnvID, k.WorkloadKind)
	if err != nil {
		return "", "", err
	}

	var st bandit.ThompsonState
	if ok {
		st, err = bandit.UnmarshalThompsonState(sn.StateJSON)
		if err != nil {
			st = bandit.NewThompsonState()
		}
	} else {
		st = bandit.NewThompsonState()
	}

	st.Update(modelID, reward)
	js, err := st.Marshal()
	if err != nil {
		return "", "", err
	}

	// write snapshot (append-only recommended; 여기선 UpsertSnapshot 사용)
	newID := newSnapshotID
	if newID == "" {
		newID = snapshotID
	}
	if newID == "" {
		// caller should pass ULID; but keep minimal
		newID = "snapshot_" + k.EnvID + "_" + k.WorkloadKind
	}

	err = l.OkAON.UpsertSnapshot(ctx, okaon.BanditSnapshot{
		SnapshotID:   newID,
		EnvID:        k.EnvID,
		WorkloadKind: k.WorkloadKind,
		Algo:         "thompson",
		StateJSON:    js,
	})
	if err != nil {
		return "", "", err
	}
	return sn.SnapshotID, newID, nil
}


---

11) internal/router/learning_adapter.go

package router

import (
	"context"
	"sort"

	"devorch/internal/learning"
)

type LearningAdapter struct {
	Learner *learning.Learner
}

// Candidate is model_id list from resolver/router base policy.
func (a *LearningAdapter) Pick(ctx context.Context, envID, workloadKind string, candidates []string) (string, map[string]float64, string, error) {
	if a == nil || a.Learner == nil || len(candidates) == 0 {
		return "", nil, "disabled", nil
	}

	// stable ordering for determinism
	cp := append([]string(nil), candidates...)
	sort.Strings(cp)

	dec, err := a.Learner.SelectModel(ctx, learning.Key{EnvID: envID, WorkloadKind: workloadKind}, cp)
	if err != nil {
		// fail-open: just pick first candidate
		return cp[0], nil, "fail_open", nil
	}
	if dec.ChosenModel == "" {
		return cp[0], dec.Scores, "empty_choice", nil
	}
	return dec.ChosenModel, dec.Scores, dec.Reason, nil
}


---

12) internal/hook/builtins/on_task_complete.go

package builtins

import (
	"context"
	"time"

	"devorch/internal/id"
	"devorch/internal/learning"
	"devorch/internal/okAON"
)

// TaskCompleteEvent: 실제 구현에선 internal/hook/events.go에 정의된 이벤트 타입을 사용.
// 여기선 “학습 연결”에 필요한 필드만 최소로 둠.
type TaskCompleteEvent struct {
	EnvID        string
	WorkloadKind string
	WorkloadID   string
	RunModelID   string

	LatencyMS    int64
	OK           bool
	QualityScore float64
	CostUSD      float64

	InputTokens  int
	OutputTokens int

	// Reward (quality gate / heuristics / user feedback)
	Reward float64
	Reason string
}

type OnTaskCompleteHook struct {
	OkAON   okaon.Store
	Learner *learning.Learner
}

func (h *OnTaskCompleteHook) Handle(ctx context.Context, ev TaskCompleteEvent) error {
	// 1) persist run + reward into OkAON
	runID := id.NewULID()
	now := time.Now().Unix()

	_ = h.OkAON.InsertRun(ctx, okaon.Run{
		RunID:        runID,
		EnvID:        ev.EnvID,
		WorkloadID:   ev.WorkloadID,
		ModelID:      ev.RunModelID,
		StartedAt:    now,
		FinishedAt:   now,
		InputTokens:  ev.InputTokens,
		OutputTokens: ev.OutputTokens,
		LatencyMS:    ev.LatencyMS,
		OK:           ev.OK,
		QualityScore: ev.QualityScore,
		CostUSD:      ev.CostUSD,
	})

	_ = h.OkAON.InsertReward(ctx, okaon.Reward{
		RewardID:  id.NewULID(),
		RunID:     runID,
		Reward:    ev.Reward,
		Reason:    ev.Reason,
		CreatedAt: now,
	})

	// 2) update learning policy snapshot
	if h.Learner != nil {
		_, _, _ = h.Learner.UpdateFromReward(
			ctx,
			learning.Key{EnvID: ev.EnvID, WorkloadKind: ev.WorkloadKind},
			ev.RunModelID,
			ev.Reward,
			"",
			id.NewULID(),
		)
	}

	return nil
}


---

다음(34단계)로 바로 이어서 무엇을 구현할까요?

지금 33단계가 “학습 루프 연결”의 핵심이라, 다음은 보통 아래 순서가 가장 빠르게 체감이 납니다.

34단계: internal/id/ulid.go(ULID 구현/의존성 없이) + storage/sqlite 초기화/마이그레이션 러너

35단계: bench/recorder.go에서 실제 실행 결과(토큰/지연/품질)를 OkAON.InsertRun/Reward로 흘리기

36단계: router/router.go에서 후보모델 산정 후 LearningAdapter.Pick() 적용 + Explain(reason/scores) 노출


원하시면 제가 34단계로 바로 진행해서, “DB 초기화 + 0001~0003 마이그레이션 자동 적용 + ULID 생성기”까지 풀코드로 이어가겠습니다.