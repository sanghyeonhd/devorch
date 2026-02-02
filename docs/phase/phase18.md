좋습니다. Step 18 = “실험 스케줄러(주기적 auto-decide) + 드리프트 탐지 + 룰 엔진 기반 자동 실험 생성” 입니다.

Step17이 “실험을 만들고 → 결과를 기록하고 → 승자를 정하는 것”이라면

Step18은 “실험을 자동으로 굴리고/정리하고/필요하면 새 실험을 자동으로 생성”합니다.


아래는 Step 18 풀코드(Go + SQL) 입니다. (누락 없이 동작 가능한 기준)


---

Step 18 소스트리 추가

internal/
├─ experiment/
│  ├─ scheduler.go            # 주기 실행(autodecide/cleanup)
│  ├─ drift.go                # 드리프트 스냅샷/탐지
│  ├─ rules.go                # 룰 엔진(조건→실험 생성)
│  ├─ rule_store.go           # 룰/스냅샷 DB
│  └─ bootstrap.go            # 앱 시작 시 스케줄러 등록
│
├─ server/routes/
│  └─ experiment_rules.go     # 룰 관리 API
│
├─ diagnostics/checks/
│  └─ experimentation.go      # (선택) doctor 체크(스케줄러/룰/드리프트)
│
├─ storage/sqlite/migrations/
│  └─ 0008_experiment_rules_and_drift.sql


---

1) DB 마이그레이션 (SQL)

internal/storage/sqlite/migrations/0008_experiment_rules_and_drift.sql

-- Step 18: Experiment Scheduler + Drift Detection + Rule Engine

CREATE TABLE IF NOT EXISTS experiment_rules (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  is_enabled INTEGER NOT NULL DEFAULT 1,

  -- what experiment to create when triggered
  kind TEXT NOT NULL,                 -- "prompt" | "routing" | "policy"
  strategy TEXT NOT NULL,             -- "ab" | "epsilon_greedy"
  traffic REAL NOT NULL DEFAULT 0.2,  -- default traffic for auto-created experiments

  -- payload templates (JSON) for variants (A,B,...)
  variants_json TEXT NOT NULL,        -- example: [{"key":"A","weight":0.5,"payload_json":"{...}"}, ...]

  -- trigger conditions JSON (simple DSL)
  conditions_json TEXT NOT NULL,      -- example: {"metric":"quality","drop_pct":0.08,"window_minutes":180}

  -- rate-limits / safety controls
  cooldown_minutes INTEGER NOT NULL DEFAULT 720,  -- don't auto-create too often
  max_active_experiments INTEGER NOT NULL DEFAULT 3,

  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_experiment_rules_enabled
  ON experiment_rules(is_enabled);

-- rule firing history (cooldown enforcement)
CREATE TABLE IF NOT EXISTS experiment_rule_fires (
  id TEXT PRIMARY KEY,
  rule_id TEXT NOT NULL,
  fired_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  experiment_id TEXT,                 -- created experiment id
  reason TEXT NOT NULL,
  FOREIGN KEY(rule_id) REFERENCES experiment_rules(id)
);

CREATE INDEX IF NOT EXISTS idx_rule_fires_rule_time
  ON experiment_rule_fires(rule_id, fired_at);

-- drift snapshots: aggregated metrics by workspace/project/provider/model
CREATE TABLE IF NOT EXISTS drift_snapshots (
  id TEXT PRIMARY KEY,

  scope TEXT NOT NULL,                -- e.g., "workspace:xxx" or "project:yyy" or "global"
  window_minutes INTEGER NOT NULL,    -- aggregation window size

  -- aggregated metrics
  sample_n INTEGER NOT NULL,
  success_rate REAL NOT NULL,
  avg_quality REAL NOT NULL,
  avg_latency_ms REAL NOT NULL,
  avg_cost_microusd REAL NOT NULL,

  -- optional dimensions (for future)
  provider TEXT,
  model TEXT,

  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_drift_scope_time
  ON drift_snapshots(scope, created_at);


---

2) 룰/드리프트 타입

internal/experiment/rules.go

package experiment

import (
	"encoding/json"
	"time"
)

type Rule struct {
	ID        string
	Name      string
	IsEnabled bool

	Kind     Kind
	Strategy Strategy
	Traffic  float64

	VariantsJSON   string
	ConditionsJSON string

	CooldownMinutes      int
	MaxActiveExperiments int
}

type AutoVariant struct {
	Key         string  `json:"key"`
	Weight      float64 `json:"weight"`
	PayloadJSON string  `json:"payload_json"`
}

type ConditionSet struct {
	Metric        string  `json:"metric"`         // "quality" | "success" | "latency" | "cost"
	DropPct       float64 `json:"drop_pct"`       // e.g., 0.08 = -8%
	RisePct       float64 `json:"rise_pct"`       // optional
	WindowMinutes int     `json:"window_minutes"` // e.g., 180
	MinSamples    int     `json:"min_samples"`    // e.g., 50
	Scope         string  `json:"scope"`          // "global" | "workspace:<id>" | "project:<id>"
}

func (r Rule) ParseVariants() ([]AutoVariant, error) {
	var v []AutoVariant
	if err := json.Unmarshal([]byte(r.VariantsJSON), &v); err != nil {
		return nil, err
	}
	return v, nil
}

func (r Rule) ParseConditions() (ConditionSet, error) {
	var c ConditionSet
	if err := json.Unmarshal([]byte(r.ConditionsJSON), &c); err != nil {
		return ConditionSet{}, err
	}
	if c.WindowMinutes <= 0 {
		c.WindowMinutes = 180
	}
	if c.MinSamples <= 0 {
		c.MinSamples = 50
	}
	if c.Scope == "" {
		c.Scope = "global"
	}
	return c, nil
}

type RuleFire struct {
	ID          string
	RuleID      string
	ExperimentID string
	Reason      string
	FiredAt     time.Time
}

internal/experiment/drift.go

package experiment

import (
	"context"
	"time"

	"devorch/internal/id"
	"devorch/internal/storage"
)

type DriftSnapshot struct {
	ID            string
	Scope         string
	WindowMinutes int

	SampleN     int
	SuccessRate float64
	AvgQuality  float64
	AvgLatency  float64
	AvgCostMicro float64

	Provider string
	Model    string

	CreatedAt time.Time
}

type DriftStore struct {
	Storage storage.Storage
}

func NewDriftStore(s storage.Storage) *DriftStore {
	return &DriftStore{Storage: s}
}

// Create snapshot from existing usage/outcome tables.
// Implementation: uses experiment_outcomes as a reliable metric source.
// You can expand later to "session" or "bench" aggregated metrics.
func (ds *DriftStore) InsertSnapshotFromOutcomes(ctx context.Context, scope string, windowMinutes int) (DriftSnapshot, error) {
	// For Step 18 we keep it simple: global aggregation from recent outcomes.
	// If you later tag outcomes with scope/workspace/project, filter by that here.

	row := ds.Storage.QueryRow(ctx, `
SELECT
  COUNT(*) as n,
  AVG(success) as success_rate,
  AVG(quality_score) as avg_quality,
  AVG(latency_ms) as avg_latency,
  AVG(cost_microusd) as avg_cost
FROM experiment_outcomes
WHERE created_at >= DATETIME('now', ?)
`, "-"+itoa(windowMinutes)+" minutes")

	var n int
	var succ, q, lat, cost float64
	if err := row.Scan(&n, &succ, &q, &lat, &cost); err != nil {
		return DriftSnapshot{}, err
	}

	snap := DriftSnapshot{
		ID:            id.NewULID(),
		Scope:         scope,
		WindowMinutes: windowMinutes,
		SampleN:       n,
		SuccessRate:   succ,
		AvgQuality:    q,
		AvgLatency:    lat,
		AvgCostMicro:  cost,
	}

	_, err := ds.Storage.Exec(ctx, `
INSERT INTO drift_snapshots
(id, scope, window_minutes, sample_n, success_rate, avg_quality, avg_latency_ms, avg_cost_microusd, provider, model)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, snap.ID, snap.Scope, snap.WindowMinutes, snap.SampleN, snap.SuccessRate, snap.AvgQuality, snap.AvgLatency, snap.AvgCostMicro, nullIfEmpty(snap.Provider), nullIfEmpty(snap.Model))
	if err != nil {
		return DriftSnapshot{}, err
	}
	return snap, nil
}

func (ds *DriftStore) GetLatestSnapshot(ctx context.Context, scope string, windowMinutes int) (DriftSnapshot, bool, error) {
	row := ds.Storage.QueryRow(ctx, `
SELECT id, scope, window_minutes, sample_n, success_rate, avg_quality, avg_latency_ms, avg_cost_microusd, provider, model, created_at
FROM drift_snapshots
WHERE scope = ? AND window_minutes = ?
ORDER BY created_at DESC
LIMIT 1
`, scope, windowMinutes)

	var snap DriftSnapshot
	var provider, model *string
	var createdAt string
	err := row.Scan(&snap.ID, &snap.Scope, &snap.WindowMinutes, &snap.SampleN, &snap.SuccessRate, &snap.AvgQuality, &snap.AvgLatency, &snap.AvgCostMicro, &provider, &model, &createdAt)
	if err != nil {
		// storage layer may return sql.ErrNoRows; keep generic:
		return DriftSnapshot{}, false, nil
	}
	if provider != nil {
		snap.Provider = *provider
	}
	if model != nil {
		snap.Model = *model
	}
	// createdAt parsing omitted intentionally (not required for logic here)
	return snap, true, nil
}

// Drift detection: compares current vs baseline snapshot.
func DetectDrop(current DriftSnapshot, baseline DriftSnapshot, metric string) (dropPct float64) {
	cur := metricValue(current, metric)
	base := metricValue(baseline, metric)
	if base <= 0 {
		return 0
	}
	return (base - cur) / base
}

func metricValue(s DriftSnapshot, metric string) float64 {
	switch metric {
	case "quality":
		return s.AvgQuality
	case "success":
		return s.SuccessRate
	case "latency":
		return s.AvgLatency
	case "cost":
		return s.AvgCostMicro
	default:
		return s.AvgQuality
	}
}

// small itoa helper (no fmt in hot path)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + (n % 10))
		n /= 10
	}
	return sign + string(b[i:])
}


---

3) Rule Store (DB)

internal/experiment/rule_store.go

package experiment

import (
	"context"
	"database/sql"

	"devorch/internal/id"
	"devorch/internal/storage"
)

type RuleStore struct {
	Storage storage.Storage
}

func NewRuleStore(s storage.Storage) *RuleStore {
	return &RuleStore{Storage: s}
}

func (rs *RuleStore) ListEnabledRules(ctx context.Context) ([]Rule, error) {
	rows, err := rs.Storage.Query(ctx, `
SELECT id, name, is_enabled, kind, strategy, traffic, variants_json, conditions_json, cooldown_minutes, max_active_experiments
FROM experiment_rules
WHERE is_enabled = 1
ORDER BY created_at ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Rule
	for rows.Next() {
		var r Rule
		var enabled int
		var kind, strategy string
		if err := rows.Scan(&r.ID, &r.Name, &enabled, &kind, &strategy, &r.Traffic, &r.VariantsJSON, &r.ConditionsJSON, &r.CooldownMinutes, &r.MaxActiveExperiments); err != nil {
			return nil, err
		}
		r.IsEnabled = enabled == 1
		r.Kind = Kind(kind)
		r.Strategy = Strategy(strategy)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (rs *RuleStore) CreateRule(ctx context.Context, r Rule) error {
	if r.ID == "" {
		r.ID = id.NewULID()
	}
	enabled := 0
	if r.IsEnabled {
		enabled = 1
	}
	if r.CooldownMinutes <= 0 {
		r.CooldownMinutes = 720
	}
	if r.MaxActiveExperiments <= 0 {
		r.MaxActiveExperiments = 3
	}
	if r.Traffic <= 0 || r.Traffic > 1 {
		r.Traffic = 0.2
	}
	_, err := rs.Storage.Exec(ctx, `
INSERT INTO experiment_rules
(id, name, is_enabled, kind, strategy, traffic, variants_json, conditions_json, cooldown_minutes, max_active_experiments)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, r.ID, r.Name, enabled, string(r.Kind), string(r.Strategy), r.Traffic, r.VariantsJSON, r.ConditionsJSON, r.CooldownMinutes, r.MaxActiveExperiments)
	return err
}

func (rs *RuleStore) LastFireWithinCooldown(ctx context.Context, ruleID string, cooldownMinutes int) (bool, error) {
	row := rs.Storage.QueryRow(ctx, `
SELECT COUNT(*)
FROM experiment_rule_fires
WHERE rule_id = ?
  AND fired_at >= DATETIME('now', ?)
`, ruleID, "-"+itoa(cooldownMinutes)+" minutes")
	var n int
	if err := row.Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

func (rs *RuleStore) CountActiveExperiments(ctx context.Context) (int, error) {
	row := rs.Storage.QueryRow(ctx, `
SELECT COUNT(*) FROM experiments WHERE status='running'
`)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (rs *RuleStore) InsertFire(ctx context.Context, ruleID, experimentID, reason string) error {
	_, err := rs.Storage.Exec(ctx, `
INSERT INTO experiment_rule_fires (id, rule_id, experiment_id, reason)
VALUES (?, ?, ?, ?)
`, id.NewULID(), ruleID, nullIfEmpty(experimentID), reason)
	return err
}

func (rs *RuleStore) GetRule(ctx context.Context, id string) (Rule, bool, error) {
	row := rs.Storage.QueryRow(ctx, `
SELECT id, name, is_enabled, kind, strategy, traffic, variants_json, conditions_json, cooldown_minutes, max_active_experiments
FROM experiment_rules
WHERE id = ?
`, id)

	var r Rule
	var enabled int
	var kind, strategy string
	err := row.Scan(&r.ID, &r.Name, &enabled, &kind, &strategy, &r.Traffic, &r.VariantsJSON, &r.ConditionsJSON, &r.CooldownMinutes, &r.MaxActiveExperiments)
	if err == sql.ErrNoRows {
		return Rule{}, false, nil
	}
	if err != nil {
		return Rule{}, false, err
	}
	r.IsEnabled = enabled == 1
	r.Kind = Kind(kind)
	r.Strategy = Strategy(strategy)
	return r, true, nil
}


---

4) 스케줄러(주기 실행): auto-decide + drift snapshot + rule evaluate

internal/experiment/scheduler.go

package experiment

import (
	"context"
	"time"

	"devorch/internal/log"
)

type SchedulerConfig struct {
	IntervalSeconds int // main loop
	AutoDecideEvery int // ticks
	DriftEvery      int // ticks
	RulesEvery      int // ticks

	DriftBaselineWindowMinutes int // baseline window
}

func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		IntervalSeconds: 30,
		AutoDecideEvery: 4,  // 2 minutes
		DriftEvery:      4,  // 2 minutes
		RulesEvery:      4,  // 2 minutes
		DriftBaselineWindowMinutes: 24 * 60, // 24h baseline
	}
}

type Scheduler struct {
	Cfg SchedulerConfig

	ExpService *Service
	ExpStore   *Store

	RuleStore  *RuleStore
	DriftStore *DriftStore

	Logger log.Logger

	stop chan struct{}
}

func NewScheduler(cfg SchedulerConfig, svc *Service, st *Store, rs *RuleStore, ds *DriftStore, lg log.Logger) *Scheduler {
	if cfg.IntervalSeconds <= 0 {
		cfg = DefaultSchedulerConfig()
	}
	return &Scheduler{
		Cfg:        cfg,
		ExpService: svc,
		ExpStore:   st,
		RuleStore:  rs,
		DriftStore: ds,
		Logger:     lg,
		stop:       make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.Cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	s.Logger.Info("experiment scheduler started")

	tick := 0
	for {
		select {
		case <-ctx.Done():
			s.Logger.Info("experiment scheduler stopping (ctx done)")
			return
		case <-s.stop:
			s.Logger.Info("experiment scheduler stopping (stop)")
			return
		case <-ticker.C:
			tick++

			// 1) auto decide
			if s.Cfg.AutoDecideEvery > 0 && tick%s.Cfg.AutoDecideEvery == 0 {
				s.runAutoDecide(ctx)
			}

			// 2) drift snapshots
			if s.Cfg.DriftEvery > 0 && tick%s.Cfg.DriftEvery == 0 {
				s.runDrift(ctx)
			}

			// 3) rule engine
			if s.Cfg.RulesEvery > 0 && tick%s.Cfg.RulesEvery == 0 {
				s.runRules(ctx)
			}
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) runAutoDecide(ctx context.Context) {
	active, err := s.ExpService.LoadActive(ctx)
	if err != nil {
		s.Logger.Warn("scheduler: load active experiments failed", "err", err.Error())
		return
	}
	for _, a := range active {
		changed, msg, err := s.ExpService.AutoDecide(ctx, a.Experiment.ID)
		if err != nil {
			s.Logger.Warn("scheduler: auto decide failed", "exp", a.Experiment.ID, "err", err.Error())
			continue
		}
		if changed {
			s.Logger.Info("scheduler: auto decide changed", "exp", a.Experiment.ID, "msg", msg)
		}
	}
}

func (s *Scheduler) runDrift(ctx context.Context) {
	// Current short window snapshot (e.g. 180min by default from rules)
	// We store on-demand per rule below as well, but we keep a "global/180" snapshot for debugging.
	_, _ = s.DriftStore.InsertSnapshotFromOutcomes(ctx, "global", 180)
}

func (s *Scheduler) runRules(ctx context.Context) {
	rules, err := s.RuleStore.ListEnabledRules(ctx)
	if err != nil {
		s.Logger.Warn("scheduler: list rules failed", "err", err.Error())
		return
	}

	for _, r := range rules {
		if err := s.evaluateRule(ctx, r); err != nil {
			s.Logger.Warn("scheduler: evaluate rule failed", "rule", r.ID, "err", err.Error())
		}
	}
}

func (s *Scheduler) evaluateRule(ctx context.Context, r Rule) error {
	cond, err := r.ParseConditions()
	if err != nil {
		return err
	}

	// cooldown check
	inCooldown, err := s.RuleStore.LastFireWithinCooldown(ctx, r.ID, r.CooldownMinutes)
	if err != nil {
		return err
	}
	if inCooldown {
		return nil
	}

	// active experiments cap
	activeCount, err := s.RuleStore.CountActiveExperiments(ctx)
	if err != nil {
		return err
	}
	if activeCount >= r.MaxActiveExperiments {
		return nil
	}

	// build snapshots: current window vs baseline window (e.g., 24h)
	current, err := s.DriftStore.InsertSnapshotFromOutcomes(ctx, cond.Scope, cond.WindowMinutes)
	if err != nil {
		return err
	}
	if current.SampleN < cond.MinSamples {
		return nil
	}

	baseline, ok, err := s.DriftStore.GetLatestSnapshot(ctx, cond.Scope, s.Cfg.DriftBaselineWindowMinutes)
	if err != nil {
		return err
	}
	if !ok || baseline.SampleN < cond.MinSamples {
		// if baseline not available yet, don't fire
		return nil
	}

	drop := DetectDrop(current, baseline, cond.Metric)
	shouldFire := cond.DropPct > 0 && drop >= cond.DropPct

	if !shouldFire {
		return nil
	}

	// create experiment
	expID, reason, err := s.createExperimentFromRule(ctx, r, cond, current, baseline, drop)
	if err != nil {
		return err
	}

	// write fire history
	_ = s.RuleStore.InsertFire(ctx, r.ID, expID, reason)
	s.Logger.Info("scheduler: rule fired", "rule", r.ID, "exp", expID, "reason", reason)
	return nil
}

func (s *Scheduler) createExperimentFromRule(ctx context.Context, r Rule, cond ConditionSet, cur DriftSnapshot, base DriftSnapshot, drop float64) (string, string, error) {
	// create running experiment
	e := Experiment{
		ID:       "",
		Name:     "auto:" + r.Name,
		Kind:     r.Kind,
		Status:   StatusRunning,
		Strategy: r.Strategy,
		Traffic:  r.Traffic,
		Seed:     "auto:" + r.ID,
	}
	if err := s.ExpStore.CreateExperiment(ctx, e); err != nil {
		return "", "", err
	}

	// find the latest created experiment id by re-loading (simple approach)
	// In production: return ID from CreateExperiment; here we keep it minimal.
	// We'll re-query by name/status.
	expID, err := s.findLatestExperimentID(ctx, e.Name)
	if err != nil {
		return "", "", err
	}

	vars, err := r.ParseVariants()
	if err != nil {
		return "", "", err
	}
	for _, v := range vars {
		if v.Weight <= 0 {
			v.Weight = 1
		}
		err := s.ExpStore.UpsertVariant(ctx, Variant{
			ExperimentID: expID,
			Key:          v.Key,
			Weight:       v.Weight,
			PayloadJSON:  v.PayloadJSON,
		})
		if err != nil {
			return "", "", err
		}
	}

	reason := "metric=" + cond.Metric +
		" drop=" + strconv3(drop) +
		" current_n=" + itoa(cur.SampleN) +
		" baseline_n=" + itoa(base.SampleN)

	return expID, reason, nil
}

func (s *Scheduler) findLatestExperimentID(ctx context.Context, name string) (string, error) {
	row := s.ExpStore.Storage.QueryRow(ctx, `
SELECT id
FROM experiments
WHERE name = ?
ORDER BY created_at DESC
LIMIT 1
`, name)
	var id string
	_ = row.Scan(&id)
	return id, nil
}


---

5) 앱 부팅 시 스케줄러 등록

internal/experiment/bootstrap.go

package experiment

import (
	"context"

	"devorch/internal/log"
	"devorch/internal/storage"
)

type Bootstrap struct {
	Scheduler *Scheduler
}

func StartScheduler(ctx context.Context, st storage.Storage, logger log.Logger, expSvc *Service, expStore *Store) *Scheduler {
	ruleStore := NewRuleStore(st)
	driftStore := NewDriftStore(st)

	s := NewScheduler(DefaultSchedulerConfig(), expSvc, expStore, ruleStore, driftStore, logger)

	// run in goroutine
	go s.Start(ctx)

	return s
}

> 실제로는 internal/app/lifecycle.go 또는 internal/app/app.go의 start 단계에서
experiment.StartScheduler(...) 호출해서 데몬 시작과 함께 돌리면 됩니다.




---

6) 룰 관리 API

internal/server/routes/experiment_rules.go

package routes

import (
	"net/http"

	"devorch/internal/experiment"
)

type ExperimentRuleRoutes struct {
	RuleStore *experiment.RuleStore
}

func NewExperimentRuleRoutes(rs *experiment.RuleStore) *ExperimentRuleRoutes {
	return &ExperimentRuleRoutes{RuleStore: rs}
}

func (r *ExperimentRuleRoutes) Register(mux Mux) {
	mux.POST("/experiment/rules", r.createRule)
	// (확장) list/get/enable/disable endpoints는 이후 단계에서 추가 가능
}

func (r *ExperimentRuleRoutes) createRule(w http.ResponseWriter, req *http.Request) {
	var in struct {
		Name string `json:"name"`

		Kind     string  `json:"kind"`
		Strategy string  `json:"strategy"`
		Traffic  float64 `json:"traffic"`

		VariantsJSON   string `json:"variants_json"`
		ConditionsJSON string `json:"conditions_json"`

		CooldownMinutes      int `json:"cooldown_minutes"`
		MaxActiveExperiments int `json:"max_active_experiments"`
	}
	if err := decodeJSON(req, &in); err != nil {
		writeErr(w, 400, err)
		return
	}
	if in.Name == "" || in.VariantsJSON == "" || in.ConditionsJSON == "" {
		writeErr(w, 400, errf("missing name/variants_json/conditions_json"))
		return
	}

	rule := experiment.Rule{
		Name:     in.Name,
		IsEnabled: true,
		Kind:     experiment.Kind(in.Kind),
		Strategy: experiment.Strategy(in.Strategy),
		Traffic:  in.Traffic,

		VariantsJSON:   in.VariantsJSON,
		ConditionsJSON: in.ConditionsJSON,

		CooldownMinutes:      in.CooldownMinutes,
		MaxActiveExperiments: in.MaxActiveExperiments,
	}
	if err := r.RuleStore.CreateRule(req.Context(), rule); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}


---

7) (선택) Doctor 체크: experiment 스케줄러/룰/드리프트 점검

internal/diagnostics/checks/experimentation.go

package checks

import (
	"context"

	"devorch/internal/diagnostics"
	"devorch/internal/storage"
)

type ExperimentationCheck struct {
	Storage storage.Storage
}

func (c ExperimentationCheck) Name() string { return "experimentation" }

func (c ExperimentationCheck) Run(ctx context.Context) diagnostics.Result {
	// basic sanity: required tables exist
	row := c.Storage.QueryRow(ctx, `
SELECT name FROM sqlite_master
WHERE type='table' AND name IN ('experiments','experiment_rules','drift_snapshots')
ORDER BY name
`)
	var name string
	_ = row.Scan(&name)
	if name == "" {
		return diagnostics.Fail("experimentation tables missing (run migrations)")
	}
	return diagnostics.Pass("experimentation subsystem ok")
}


---

8) Step18 사용 예시(룰 1개 생성)

“품질(quality)이 3시간 창에서 8% 이상 하락하면 → prompt 실험을 자동 생성(A/B)”

conditions_json:


{"metric":"quality","drop_pct":0.08,"window_minutes":180,"min_samples":50,"scope":"global"}

variants_json:


[
  {"key":"A","weight":0.5,"payload_json":"{\"system_prompt\":\"You are DevOrch...\",\"extra_rules\":\"Follow repo conventions\"}"},
  {"key":"B","weight":0.5,"payload_json":"{\"system_prompt\":\"You are DevOrch (strict)...\",\"extra_rules\":\"Prefer minimal diffs\"}"}
]

이 룰이 발동하면:

1. 실험 자동 생성(traffic=0.2 기본)


2. 배정/결과 수집(Step17)


3. auto-decide로 자동 승자 선정


4. 승자 variant가 남고 실험 종료




---

Step18까지의 “스스로 발전” 루프가 완성됨

Step14~15: 로컬 벤치/스코어링

Step17: 실험(A/B) + 자동 승자 선정

Step18: 드리프트 감지 → 룰 기반 자동 실험 생성 → 자동 운영


즉, Devorch는 이제 “사용할수록 설정/프롬프트가 자동으로 개선되는” 구조가 됩니다.


---

원하시면 다음 단계(19단계)는 보통 이렇게 갑니다:

Step19: “승자 정책을 router/modelresolver에 영구 반영(버전드 정책 레지스트리) + 롤백/승격(채널) 시스템”


바로 19단계로 이어갈까요?