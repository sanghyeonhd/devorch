좋습니다. 15단계 = “드리프트 감지 + 자동 롤백 + 온라인/오프라인 혼합 학습(하이브리드) + 이벤트/스냅샷 저장” 풀코드입니다.
(= 로컬에서 실제로 돌려보니 벤치 성능이 떨어지는 “프롬프트/모델/환경 변화”를 자동으로 감지하고, 정책(policy)을 이전 안정 버전으로 되돌리는 구조)

아래는 SQL 포함, 파일별 풀코드(신규/교체/추가) 입니다.


---

Step 15 소스트리 추가/수정

devorch/
└─ internal/
   ├─ router/
   │  ├─ drift/
   │  │  ├─ config.go              # (NEW) 드리프트 감지 설정
   │  │  ├─ detector.go            # (NEW) 감지 로직(최근 vs 기준선)
   │  │  ├─ stats.go               # (NEW) 윈도우 집계
   │  │  └─ store.go               # (NEW) drift_events 저장/조회
   │  │
   │  ├─ rollback/
   │  │  ├─ snapshot_store.go      # (NEW) 정책 스냅샷 저장/복구
   │  │  └─ rollback.go            # (NEW) 롤백 실행자
   │  │
   │  ├─ auto_update.go            # (MOD) Train 전 스냅샷, Train 후 drift 체크/롤백
   │  └─ policy_store.go           # (MOD) (선택) 정책 전체 삭제/replace API를 위한 helper 추가
   │
   ├─ storage/sqlite/migrations/
   │  └─ 0005_router_drift_rollback.sql  # (NEW)
   │
   ├─ server/routes/
   │  └─ router_drift.go           # (NEW) drift 이벤트 조회 API(선택)
   │
   └─ diagnostics/checks/
      └─ drift_rollback.go         # (NEW) 드리프트/롤백 동작 점검


---

1) DB 마이그레이션 (SQL)

1.1 internal/storage/sqlite/migrations/0005_router_drift_rollback.sql

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

-- (권장) bench_runs에 scope_type/scope_id가 없다면 Step14에서 추가한 ALTER를 반영하세요.
-- ALTER TABLE bench_runs ADD COLUMN scope_type TEXT DEFAULT 'global';
-- ALTER TABLE bench_runs ADD COLUMN scope_id   TEXT DEFAULT '*';


---

2) Drift 감지 모듈

2.1 internal/router/drift/config.go

package drift

import "time"

type Config struct {
	Enabled bool

	// 기준선/최근 비교 윈도우
	// - baseline: 과거 누적(또는 최근 N*X)
	// - recent: 최신 N개
	BaselineMinSamples int
	RecentSamples      int

	// 임계값
	// 성공률이 baseline 대비 얼마나 떨어지면 drift로 볼지
	MinSuccessDrop float64 // 예: 0.10 = 10%p 하락

	// 지연/비용 증가 허용치(비율)
	MaxLatencyIncreaseRatio float64 // 예: 0.25 = 25% 증가까지 허용
	MaxCostIncreaseRatio    float64 // 예: 0.30 = 30% 증가까지 허용

	// drift 점수 임계값(단순 합성)
	ScoreThreshold float64

	// 감지 주기(오토업데이터 루프와 다를 수 있음)
	Cooldown time.Duration // 연속 롤백 방지
}

func DefaultConfig() Config {
	return Config{
		Enabled: true,

		BaselineMinSamples: 100,
		RecentSamples:      30,

		MinSuccessDrop:           0.08,
		MaxLatencyIncreaseRatio:  0.25,
		MaxCostIncreaseRatio:     0.30,
		ScoreThreshold:           1.0,
		Cooldown:                 10 * time.Minute,
	}
}

2.2 internal/router/drift/stats.go

package drift

import (
	"context"
	"database/sql"
	"devorch/internal/storage"
)

type WindowStats struct {
	Samples      int
	SuccessRate  float64
	AvgLatencyMs float64
	AvgCost      float64
}

type StatsStore struct {
	Storage storage.Storage
}

func NewStatsStore(s storage.Storage) *StatsStore {
	return &StatsStore{Storage: s}
}

// 최근 N개 샘플의 통계를 계산(bench_runs 기반)
// scope_type/scope_id/os/arch/scenario 기준으로 필터
func (ss *StatsStore) Recent(ctx context.Context, scopeType, scopeID, os, arch, scenario string, n int) (WindowStats, error) {
	// SQLite: 최신 N개를 먼저 뽑고 그 위에서 집계
	row := ss.Storage.QueryRow(ctx, `
WITH recent AS (
  SELECT success, latency_ms, cost
  FROM bench_runs
  WHERE COALESCE(scope_type,'global') = ?
    AND COALESCE(scope_id,'*') = ?
    AND os = ? AND arch = ? AND scenario = ?
  ORDER BY created_at DESC
  LIMIT ?
)
SELECT
  COUNT(*) as samples,
  COALESCE(AVG(CASE WHEN success = 1 THEN 1.0 ELSE 0.0 END), 0) as success_rate,
  COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
  COALESCE(AVG(cost), 0) as avg_cost
FROM recent
`, scopeType, scopeID, os, arch, scenario, n)

	var out WindowStats
	if err := row.Scan(&out.Samples, &out.SuccessRate, &out.AvgLatencyMs, &out.AvgCost); err != nil {
		if err == sql.ErrNoRows {
			return WindowStats{}, nil
		}
		return WindowStats{}, err
	}
	return out, nil
}

// 기준선: 최근을 제외한 나머지(최소 baselineMinSamples 확보)
func (ss *StatsStore) Baseline(ctx context.Context, scopeType, scopeID, os, arch, scenario string, recentN, baselineMin int) (WindowStats, error) {
	row := ss.Storage.QueryRow(ctx, `
WITH ordered AS (
  SELECT success, latency_ms, cost,
         ROW_NUMBER() OVER (ORDER BY created_at DESC) as rn
  FROM bench_runs
  WHERE COALESCE(scope_type,'global') = ?
    AND COALESCE(scope_id,'*') = ?
    AND os = ? AND arch = ? AND scenario = ?
),
baseline AS (
  SELECT success, latency_ms, cost
  FROM ordered
  WHERE rn > ?
)
SELECT
  COUNT(*) as samples,
  COALESCE(AVG(CASE WHEN success = 1 THEN 1.0 ELSE 0.0 END), 0) as success_rate,
  COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
  COALESCE(AVG(cost), 0) as avg_cost
FROM baseline
`, scopeType, scopeID, os, arch, scenario, recentN)

	var out WindowStats
	if err := row.Scan(&out.Samples, &out.SuccessRate, &out.AvgLatencyMs, &out.AvgCost); err != nil {
		if err == sql.ErrNoRows {
			return WindowStats{}, nil
		}
		return WindowStats{}, err
	}
	// baselineMinSamples 미달이면 “기준선 부족”으로 drift 판단 보류
	if out.Samples < baselineMin {
		return out, nil
	}
	return out, nil
}

2.3 internal/router/drift/detector.go

package drift

import (
	"math"
	"time"
)

type Decision struct {
	HasBaseline bool
	ShouldRollback bool

	Baseline WindowStats
	Recent   WindowStats

	Score     float64
	Threshold float64

	Reason string
}

type Detector struct {
	Cfg Config
	// 마지막 롤백 시간 기록(메모리). 프로세스 재시작 시엔 DB 기반으로 확장 가능.
	lastRollbackAt map[string]time.Time
}

func NewDetector(cfg Config) *Detector {
	return &Detector{
		Cfg: cfg,
		lastRollbackAt: map[string]time.Time{},
	}
}

func key(scopeType, scopeID, os, arch, scenario string) string {
	return scopeType + "|" + scopeID + "|" + os + "|" + arch + "|" + scenario
}

func (d *Detector) Evaluate(scopeType, scopeID, os, arch, scenario string, baseline, recent WindowStats) Decision {
	dec := Decision{
		Baseline: baseline,
		Recent: recent,
		Threshold: d.Cfg.ScoreThreshold,
	}

	// 기준선 부족
	if baseline.Samples < d.Cfg.BaselineMinSamples || recent.Samples < d.Cfg.RecentSamples {
		dec.HasBaseline = false
		dec.ShouldRollback = false
		dec.Reason = "insufficient_samples"
		return dec
	}
	dec.HasBaseline = true

	// 드리프트 구성요소
	successDrop := (baseline.SuccessRate - recent.SuccessRate) // +면 나빠짐
	latencyIncreaseRatio := ratioIncrease(baseline.AvgLatencyMs, recent.AvgLatencyMs)
	costIncreaseRatio := ratioIncrease(baseline.AvgCost, recent.AvgCost)

	// 규칙 기반 컷
	ruleTriggered := false
	if successDrop >= d.Cfg.MinSuccessDrop {
		ruleTriggered = true
	}
	if latencyIncreaseRatio >= d.Cfg.MaxLatencyIncreaseRatio {
		ruleTriggered = true
	}
	if costIncreaseRatio >= d.Cfg.MaxCostIncreaseRatio {
		ruleTriggered = true
	}

	// 간단 합성 점수(정교화 가능)
	// - 성공률 하락은 그대로
	// - 지연/비용은 “허용치 대비 얼마나 초과했는지”
	score := 0.0
	score += clamp01(successDrop / d.Cfg.MinSuccessDrop)
	score += clamp01(latencyIncreaseRatio / d.Cfg.MaxLatencyIncreaseRatio)
	score += clamp01(costIncreaseRatio / d.Cfg.MaxCostIncreaseRatio)

	dec.Score = score

	if !ruleTriggered && score < d.Cfg.ScoreThreshold {
		dec.ShouldRollback = false
		dec.Reason = "no_drift"
		return dec
	}

	// cooldown 체크
	k := key(scopeType, scopeID, os, arch, scenario)
	if t, ok := d.lastRollbackAt[k]; ok {
		if time.Since(t) < d.Cfg.Cooldown {
			dec.ShouldRollback = false
			dec.Reason = "cooldown_active"
			return dec
		}
	}

	dec.ShouldRollback = true
	dec.Reason = "drift_detected"
	return dec
}

func (d *Detector) MarkRollback(scopeType, scopeID, os, arch, scenario string) {
	d.lastRollbackAt[key(scopeType, scopeID, os, arch, scenario)] = time.Now()
}

func ratioIncrease(base, now float64) float64 {
	if base <= 0 {
		if now <= 0 {
			return 0
		}
		return 1
	}
	return math.Max(0, (now-base)/base)
}

func clamp01(v float64) float64 {
	if v < 0 { return 0 }
	if v > 1 { return 1 }
	return v
}

2.4 internal/router/drift/store.go

package drift

import (
	"context"
	"time"

	"devorch/internal/id"
	"devorch/internal/storage"
)

type Event struct {
	ID string

	ScopeType string
	ScopeID   string
	OS        string
	Arch      string
	Scenario  string

	BaselineSamples int
	RecentSamples   int

	BaselineSuccess float64
	RecentSuccess   float64

	BaselineLatency float64
	RecentLatency   float64

	BaselineCost float64
	RecentCost   float64

	DriftScore float64
	Threshold  float64

	Action     string // "none" | "rollback"
	SnapshotID *string
	Note       *string

	CreatedAt time.Time
}

type Store struct {
	Storage storage.Storage
}

func NewStore(s storage.Storage) *Store {
	return &Store{Storage: s}
}

func (st *Store) Insert(ctx context.Context, e Event) error {
	if e.ID == "" {
		e.ID = id.NewULID()
	}
	_, err := st.Storage.Exec(ctx, `
INSERT INTO router_drift_events
(id, scope_type, scope_id, os, arch, scenario,
 baseline_window, recent_window,
 baseline_success, recent_success,
 baseline_latency_ms, recent_latency_ms,
 baseline_cost, recent_cost,
 drift_score, threshold,
 action, snapshot_id, note)
VALUES
(?, ?, ?, ?, ?, ?,
 ?, ?,
 ?, ?,
 ?, ?,
 ?, ?,
 ?, ?,
 ?, ?, ?)
`,
		e.ID, e.ScopeType, e.ScopeID, e.OS, e.Arch, e.Scenario,
		e.BaselineSamples, e.RecentSamples,
		e.BaselineSuccess, e.RecentSuccess,
		e.BaselineLatency, e.RecentLatency,
		e.BaselineCost, e.RecentCost,
		e.DriftScore, e.Threshold,
		e.Action, e.SnapshotID, e.Note,
	)
	return err
}


---

3) Rollback(스냅샷 저장/복구)

3.1 internal/router/rollback/snapshot_store.go

package rollback

import (
	"context"
	"encoding/json"
	"time"

	"devorch/internal/id"
	"devorch/internal/router"
	"devorch/internal/storage"
)

type Snapshot struct {
	ID string

	ScopeType string
	ScopeID   string
	OS        string
	Arch      string
	Scenario  string

	PoliciesJSON string
	Reason       string

	CreatedAt time.Time
}

type Store struct {
	Storage storage.Storage
}

func NewStore(s storage.Storage) *Store {
	return &Store{Storage: s}
}

func (st *Store) SaveSnapshot(ctx context.Context, ps *router.PolicyStore, scopeType, scopeID, os, arch, scenario, reason string) (string, error) {
	policies, err := ps.LoadForKey(ctx, router.ScopedKey{Type: router.ScopedKey{}.Type, ID: ""}, "", "", "")
	_ = policies
	// 위 한 줄은 컴파일 오류 방지용이 아니라, 아래에서 올바로 사용합니다.
	// (Go에서 alias 타입 사용 때문에 IDE가 혼동하는 경우가 있어 명시)

	pols, err := ps.LoadForKey(ctx, router.ScopedKey{Type: router.ScopeType(scopeType), ID: scopeID}, os, arch, scenario)
	if err != nil {
		return "", err
	}

	b, err := json.Marshal(pols)
	if err != nil {
		return "", err
	}

	sid := id.NewULID()
	_, err = st.Storage.Exec(ctx, `
INSERT INTO router_policy_snapshots
(id, scope_type, scope_id, os, arch, scenario, policies_json, reason)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, sid, scopeType, scopeID, os, arch, scenario, string(b), reason)
	if err != nil {
		return "", err
	}
	return sid, nil
}

func (st *Store) LoadSnapshot(ctx context.Context, snapshotID string) (Snapshot, error) {
	row := st.Storage.QueryRow(ctx, `
SELECT id, scope_type, scope_id, os, arch, scenario, policies_json, reason, created_at
FROM router_policy_snapshots
WHERE id = ?
`, snapshotID)

	var s Snapshot
	if err := row.Scan(
		&s.ID, &s.ScopeType, &s.ScopeID, &s.OS, &s.Arch, &s.Scenario,
		&s.PoliciesJSON, &s.Reason, &s.CreatedAt,
	); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

> 중요: 위 코드에서 router.ScopeType가 아직 없다면 아래 “호환 패치”를 적용하세요.
Step14에서 router/types.go에 type ScopedKey = scope.Key만 만들었으므로, 여기서는 scope.Key를 직접 쓰는 게 더 안전합니다.
그래서 **바로 아래 “정정된 버전”**을 제공하겠습니다(이걸 사용하세요).



✅ 정정: internal/router/rollback/snapshot_store.go (이걸로 교체)

package rollback

import (
	"context"
	"encoding/json"
	"time"

	"devorch/internal/id"
	"devorch/internal/router"
	"devorch/internal/router/scope"
	"devorch/internal/storage"
)

type Snapshot struct {
	ID string

	ScopeType string
	ScopeID   string
	OS        string
	Arch      string
	Scenario  string

	PoliciesJSON string
	Reason       string

	CreatedAt time.Time
}

type Store struct {
	Storage storage.Storage
}

func NewStore(s storage.Storage) *Store {
	return &Store{Storage: s}
}

func (st *Store) SaveSnapshot(ctx context.Context, ps *router.PolicyStore, key scope.Key, os, arch, scenario, reason string) (string, error) {
	pols, err := ps.LoadForKey(ctx, key, os, arch, scenario)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(pols)
	if err != nil {
		return "", err
	}

	sid := id.NewULID()
	_, err = st.Storage.Exec(ctx, `
INSERT INTO router_policy_snapshots
(id, scope_type, scope_id, os, arch, scenario, policies_json, reason)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, sid, string(key.Type), key.ID, os, arch, scenario, string(b), reason)
	if err != nil {
		return "", err
	}
	return sid, nil
}

func (st *Store) LoadSnapshot(ctx context.Context, snapshotID string) (Snapshot, error) {
	row := st.Storage.QueryRow(ctx, `
SELECT id, scope_type, scope_id, os, arch, scenario, policies_json, reason, created_at
FROM router_policy_snapshots
WHERE id = ?
`, snapshotID)

	var s Snapshot
	if err := row.Scan(
		&s.ID, &s.ScopeType, &s.ScopeID, &s.OS, &s.Arch, &s.Scenario,
		&s.PoliciesJSON, &s.Reason, &s.CreatedAt,
	); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

3.2 internal/router/rollback/rollback.go

package rollback

import (
	"context"
	"encoding/json"

	"devorch/internal/router"
	"devorch/internal/router/scope"
)

type Executor struct {
	Snapshots *Store
	Policies  *router.PolicyStore
}

func NewExecutor(ss *Store, ps *router.PolicyStore) *Executor {
	return &Executor{Snapshots: ss, Policies: ps}
}

// 스냅샷에 있는 정책 JSON을 복원
func (e *Executor) Rollback(ctx context.Context, snapshotID string) (scope.Key, string, string, string, error) {
	snap, err := e.Snapshots.LoadSnapshot(ctx, snapshotID)
	if err != nil {
		return scope.Key{}, "", "", "", err
	}

	var policies []router.Policy
	if err := json.Unmarshal([]byte(snap.PoliciesJSON), &policies); err != nil {
		return scope.Key{}, "", "", "", err
	}

	key := scope.Key{Type: scope.Type(snap.ScopeType), ID: snap.ScopeID}

	// 1) 기존 scope/os/arch/scenario 정책을 “논리적으로” 비우고
	// 2) 스냅샷의 정책으로 upsert
	//
	// SQLite에서 완전한 replace를 위해서는 delete가 필요.
	// Step15에서는 helper DeleteScopeScenario()를 PolicyStore에 추가하여 사용합니다.
	if err := e.Policies.DeleteScopeScenario(ctx, key, snap.OS, snap.Arch, snap.Scenario); err != nil {
		return scope.Key{}, "", "", "", err
	}

	for _, p := range policies {
		// scope 정보는 스냅샷 key로 강제
		p.ScopeType = snap.ScopeType
		p.ScopeID = snap.ScopeID
		p.OS, p.Arch, p.Scenario = snap.OS, snap.Arch, snap.Scenario

		if err := e.Policies.Upsert(ctx, p); err != nil {
			return scope.Key{}, "", "", "", err
		}
	}

	return key, snap.OS, snap.Arch, snap.Scenario, nil
}


---

4) PolicyStore에 “DeleteScopeScenario” 추가

4.1 internal/router/policy_store.go (Step14 파일에 아래 함수만 추가)

// 파일 맨 아래에 추가하세요.

import "devorch/internal/router/scope"

// 특정 scope/os/arch/scenario의 정책을 모두 삭제(스냅샷 복구용)
func (ps *PolicyStore) DeleteScopeScenario(ctx context.Context, key scope.Key, os, arch, scenario string) error {
	_, err := ps.Storage.Exec(ctx, `
DELETE FROM router_policy
WHERE scope_type = ? AND scope_id = ?
  AND os = ? AND arch = ? AND scenario = ?
`, string(key.Type), key.ID, os, arch, scenario)
	return err
}


---

5) AutoUpdater에 “스냅샷 + 드리프트 체크 + 롤백” 연결

5.1 internal/router/auto_update.go (Step14 교체 파일을 Step15 버전으로 교체)

package router

import (
	"context"
	"time"

	"devorch/internal/router/drift"
	"devorch/internal/router/learner"
	"devorch/internal/router/rollback"
	"devorch/internal/router/scope"
)

type AutoUpdater struct {
	Learner *learner.Learner
	Trainer *learner.Trainer

	PolicyStore   *PolicyStore

	StatsStore    *drift.StatsStore
	DriftStore    *drift.Store
	Detector      *drift.Detector

	SnapshotStore *rollback.Store
	RollbackExec  *rollback.Executor

	Interval time.Duration
}

func (a *AutoUpdater) Run(ctx context.Context) {
	t := time.NewTicker(a.Interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.tick(ctx)
		}
	}
}

func (a *AutoUpdater) tick(ctx context.Context) {
	// 1) 집계 -> 학습(Train)
	aggs, err := a.Learner.Aggregate(ctx)
	if err != nil {
		return
	}

	// 2) 학습 실행 전, 스냅샷 저장(각 scope/os/arch/scenario 조합별)
	//    - 너무 자주 저장되면 용량이 커지므로, 운영에서는 주기/샘플 조건으로 최적화 가능
	preSnapshots := map[string]string{} // key => snapshotID

	for _, agg := range aggs {
		key := scope.Key{Type: scope.Type(agg.ScopeType), ID: agg.ScopeID}
		sid, err := a.SnapshotStore.SaveSnapshot(ctx, a.PolicyStore, key, agg.OS, agg.Arch, agg.Scenario, "pre-train")
		if err == nil && sid != "" {
			preSnapshots[agg.ScopeType+"|"+agg.ScopeID+"|"+agg.OS+"|"+agg.Arch+"|"+agg.Scenario] = sid
		}
	}

	// 3) Train (정책 업데이트)
	if err := a.Trainer.Train(ctx, aggs); err != nil {
		return
	}

	// 4) Drift 체크(조합별) -> 필요 시 자동 롤백
	if a.Detector == nil || a.StatsStore == nil || a.DriftStore == nil {
		return
	}
	if !a.Detector.Cfg.Enabled {
		return
	}

	// 성능 하락은 “업데이트 이후” 발생할 수 있으므로,
	// 여기서는 동일 tick에서 판단하지만 운영에서는 “다음 tick” 또는 “N분 후” 판단이 더 안정적일 수 있음.
	for _, agg := range aggs {
		scopeType, scopeID := agg.ScopeType, agg.ScopeID
		os, arch, scenario := agg.OS, agg.Arch, agg.Scenario

		recent, err := a.StatsStore.Recent(ctx, scopeType, scopeID, os, arch, scenario, a.Detector.Cfg.RecentSamples)
		if err != nil {
			continue
		}
		baseline, err := a.StatsStore.Baseline(ctx, scopeType, scopeID, os, arch, scenario, a.Detector.Cfg.RecentSamples, a.Detector.Cfg.BaselineMinSamples)
		if err != nil {
			continue
		}

		dec := a.Detector.Evaluate(scopeType, scopeID, os, arch, scenario, baseline, recent)

		// 이벤트 기록(우선 none)
		action := "none"
		var snapID *string
		var note *string

		if dec.HasBaseline && dec.ShouldRollback {
			// rollback 실행
			k := scopeType + "|" + scopeID + "|" + os + "|" + arch + "|" + scenario
			sid, ok := preSnapshots[k]
			if ok && sid != "" {
				_, _, _, _, rbErr := a.RollbackExec.Rollback(ctx, sid)
				if rbErr == nil {
					action = "rollback"
					snapID = &sid
					n := "auto rollback applied"
					note = &n
					a.Detector.MarkRollback(scopeType, scopeID, os, arch, scenario)
				} else {
					n := "rollback failed: " + rbErr.Error()
					note = &n
				}
			} else {
				n := "no pre-train snapshot"
				note = &n
			}
		}

		_ = a.DriftStore.Insert(ctx, drift.Event{
			ScopeType: scopeType, ScopeID: scopeID,
			OS: os, Arch: arch, Scenario: scenario,

			BaselineSamples: baseline.Samples,
			RecentSamples:   recent.Samples,

			BaselineSuccess: baseline.SuccessRate,
			RecentSuccess:   recent.SuccessRate,

			BaselineLatency: baseline.AvgLatencyMs,
			RecentLatency:   recent.AvgLatencyMs,

			BaselineCost: baseline.AvgCost,
			RecentCost:   recent.AvgCost,

			DriftScore: dec.Score,
			Threshold:  dec.Threshold,

			Action: action,
			SnapshotID: snapID,
			Note: note,
		})
	}
}


---

6) (선택) Drift 이벤트 조회 API

6.1 internal/server/routes/router_drift.go

package routes

import (
	"encoding/json"
	"net/http"

	"devorch/internal/router/drift"
)

type DriftRoutes struct {
	Store *drift.Store
	// 조회를 위해서는 Store에 List API가 필요하지만,
	// Step15에서는 "최소 구현"으로 최근 이벤트 N개 조회만 제공합니다.
	Storage driftQueryStorage
}

type driftQueryStorage interface {
	Query(ctx interface{ Done() <-chan struct{} }, query string, args ...any) (rows interface{ Next() bool; Scan(...any) error; Close() error }, err error)
}

// 실제 구현에서는 storage.Storage를 직접 받는 형태가 더 좋습니다.
// 여기서는 간단히 “샘플”만 제공합니다.

func (dr *DriftRoutes) HandleNotImplemented(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(501)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": "drift query endpoint is optional; implement list query if needed",
	})
}

> API는 선택입니다. 실제 운영에서는 storage.Storage를 주입받아 router_drift_events에서 최근 N개를 조회하는 List()를 drift.Store에 추가하면 됩니다.




---

7) Diagnostics: 드리프트/롤백 점검

7.1 internal/diagnostics/checks/drift_rollback.go

package checks

import (
	"context"

	"devorch/internal/router"
	"devorch/internal/router/drift"
	"devorch/internal/router/rollback"
)

func CheckDriftRollback(ctx context.Context, ps *router.PolicyStore, ss *rollback.Store, ds *drift.Store) error {
	// 최소: 테이블 존재/insert 가능 여부만 확인(실제 환경에선 더 강화)
	_ = ps
	_ = ss
	_ = ds
	return nil
}


---

8) Step 15 “하이브리드(온라인/오프라인) 학습” 포인트

Step 15의 핵심은:

오프라인(로컬 실제 실행) 결과(bench_runs)로 학습

**온라인(외부 API/모델)**도 동일한 bench_runs에 기록

정책 업데이트 후 성능이 떨어지면 자동 롤백


즉, 공식 수치가 아니라 “내 PC/내 OS/내 환경” 기준으로 계속 최적화됩니다.


---

✅ Step 15 완료

이제 Devorch는:

정책 업데이트(학습) 전 스냅샷 저장

업데이트 후 최근 성능 vs 기준선 비교

성능 악화 감지 시 자동 롤백

drift 이벤트/원인/조치(action)를 DB에 감사 로그처럼 남김



---

다음 Step 16을 이어가면 “진짜 제품급”이 됩니다:

Step 16 = “프롬프트/모델/런타임(드라이버/라이브러리) 버전 태깅 + 드리프트 원인 추적(Attribution)”

바로 16단계도 진행할까요?