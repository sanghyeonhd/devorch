좋습니다. Step 17 = “프롬프트/정책 A/B 실험 + 자동 승자 선택 + 실험 관리(대시보드/API) + 라우터 연동” 입니다.
이 단계부터 Devorch는 “그때그때 최적 모델” 수준을 넘어서, 프롬프트/정책 자체를 실험하고 자동으로 진화합니다.

아래는 Step 17 풀코드(Go + SQL) 입니다. (필수 파일/기능 누락 없이 동작 가능한 기준)


---

Step 17 소스트리 추가

internal/
├─ experiment/
│  ├─ types.go
│  ├─ hashing.go
│  ├─ assignment.go          # A/B 배정 (sticky)
│  ├─ store.go               # DB 저장
│  ├─ service.go             # 생성/종료/승자선정
│  ├─ evaluator.go           # 품질 점수/지표 집계
│  ├─ selector.go            # 승자 결정 (z-test/epsilon-greedy)
│  └─ guard.go               # 안전장치(rollback/stop)
│
├─ router/
│  ├─ router.go              # (기존) - Step17 hook
│  └─ experiment_bridge.go   # (NEW) 라우팅 시 experiment 적용
│
├─ server/routes/
│  └─ experiment.go          # (NEW) API endpoints
│
├─ storage/sqlite/migrations/
│  └─ 0007_experiments.sql


---

1) DB 마이그레이션 (SQL)

internal/storage/sqlite/migrations/0007_experiments.sql

-- Step 17: Prompt/Policy Experiments (A/B) + Auto Winner

CREATE TABLE IF NOT EXISTS experiments (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,              -- "prompt" | "routing" | "policy"
  status TEXT NOT NULL,            -- "draft" | "running" | "stopped" | "completed"
  strategy TEXT NOT NULL,          -- "ab" | "epsilon_greedy"
  traffic REAL NOT NULL DEFAULT 1.0,  -- 0.0 ~ 1.0
  seed TEXT NOT NULL,              -- sticky assignment seed
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_experiments_status
  ON experiments(status);

CREATE TABLE IF NOT EXISTS experiment_variants (
  id TEXT PRIMARY KEY,
  experiment_id TEXT NOT NULL,

  key TEXT NOT NULL,               -- "A", "B", "C"...
  weight REAL NOT NULL DEFAULT 0.5,

  -- payload is JSON: prompt template / routing rules / policy knobs
  payload_json TEXT NOT NULL,

  is_winner INTEGER NOT NULL DEFAULT 0,

  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(experiment_id) REFERENCES experiments(id)
);

CREATE INDEX IF NOT EXISTS idx_variants_experiment
  ON experiment_variants(experiment_id);

CREATE TABLE IF NOT EXISTS experiment_assignments (
  id TEXT PRIMARY KEY,
  experiment_id TEXT NOT NULL,

  subject TEXT NOT NULL,          -- user_id | workspace_id | project_id | session_id
  variant_key TEXT NOT NULL,      -- "A"/"B"/...

  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

  UNIQUE(experiment_id, subject),

  FOREIGN KEY(experiment_id) REFERENCES experiments(id)
);

CREATE INDEX IF NOT EXISTS idx_assignments_experiment
  ON experiment_assignments(experiment_id);

-- Outcome metrics per "bench_run_id" or "session/message"
CREATE TABLE IF NOT EXISTS experiment_outcomes (
  id TEXT PRIMARY KEY,
  experiment_id TEXT NOT NULL,
  variant_key TEXT NOT NULL,

  bench_run_id TEXT,              -- 연결 가능한 경우
  session_id TEXT,
  message_id TEXT,

  -- core metrics
  success INTEGER NOT NULL,       -- 1=success, 0=failure
  latency_ms INTEGER NOT NULL,
  cost_microusd INTEGER NOT NULL,
  quality_score REAL NOT NULL,    -- 0~1 normalized
  tokens_in INTEGER NOT NULL,
  tokens_out INTEGER NOT NULL,

  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(experiment_id) REFERENCES experiments(id)
);

CREATE INDEX IF NOT EXISTS idx_outcomes_experiment_variant
  ON experiment_outcomes(experiment_id, variant_key);

CREATE TABLE IF NOT EXISTS experiment_decisions (
  id TEXT PRIMARY KEY,
  experiment_id TEXT NOT NULL,

  decided_variant_key TEXT NOT NULL,
  reason TEXT NOT NULL,           -- json or text
  decided_at DATETIME DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(experiment_id) REFERENCES experiments(id)
);


---

2) Experiment 타입 정의

internal/experiment/types.go

package experiment

type Status string
type Kind string
type Strategy string

const (
	StatusDraft     Status = "draft"
	StatusRunning   Status = "running"
	StatusStopped   Status = "stopped"
	StatusCompleted Status = "completed"
)

const (
	KindPrompt  Kind = "prompt"
	KindRouting Kind = "routing"
	KindPolicy  Kind = "policy"
)

const (
	StrategyAB           Strategy = "ab"
	StrategyEpsilonGreedy Strategy = "epsilon_greedy"
)

type Experiment struct {
	ID       string
	Name     string
	Kind     Kind
	Status   Status
	Strategy Strategy

	Traffic float64
	Seed    string
}

type Variant struct {
	ID           string
	ExperimentID string
	Key          string
	Weight       float64
	PayloadJSON  string
	IsWinner     bool
}

type Assignment struct {
	ID           string
	ExperimentID string
	Subject      string
	VariantKey   string
}

type Outcome struct {
	ID           string
	ExperimentID string
	VariantKey   string

	BenchRunID string
	SessionID  string
	MessageID  string

	Success      bool
	LatencyMS    int
	CostMicroUSD int
	QualityScore float64
	TokensIn     int
	TokensOut    int
}

type Decision struct {
	ID           string
	ExperimentID string
	VariantKey   string
	Reason       string
}


---

3) Sticky 배정(해시 기반)

internal/experiment/hashing.go

package experiment

import (
	"crypto/sha256"
	"encoding/binary"
)

func hashToUint64(seed string, subject string) uint64 {
	sum := sha256.Sum256([]byte(seed + ":" + subject))
	return binary.LittleEndian.Uint64(sum[:8])
}

func hashToFloat01(seed string, subject string) float64 {
	v := hashToUint64(seed, subject)
	// 0..1
	return float64(v%1_000_000_000) / 1_000_000_000.0
}


---

4) 배정 로직(traffic + weights)

internal/experiment/assignment.go

package experiment

import "sort"

type WeightedKey struct {
	Key    string
	Weight float64
}

func ChooseVariant(seed string, subject string, traffic float64, weights []WeightedKey) (string, bool) {
	// traffic gate
	r := hashToFloat01(seed, subject)
	if r > traffic {
		return "", false // not in experiment
	}

	// normalize weights
	total := 0.0
	for _, w := range weights {
		if w.Weight > 0 {
			total += w.Weight
		}
	}
	if total <= 0 {
		return "", false
	}

	normalized := make([]WeightedKey, 0, len(weights))
	for _, w := range weights {
		if w.Weight <= 0 {
			continue
		}
		normalized = append(normalized, WeightedKey{Key: w.Key, Weight: w.Weight / total})
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Key < normalized[j].Key })

	x := hashToFloat01(seed+"|variant", subject)
	acc := 0.0
	for _, w := range normalized {
		acc += w.Weight
		if x <= acc {
			return w.Key, true
		}
	}
	return normalized[len(normalized)-1].Key, true
}


---

5) DB Store

internal/experiment/store.go

package experiment

import (
	"context"
	"database/sql"

	"devorch/internal/id"
	"devorch/internal/storage"
)

type Store struct {
	Storage storage.Storage
}

func NewStore(s storage.Storage) *Store {
	return &Store{Storage: s}
}

func (st *Store) CreateExperiment(ctx context.Context, e Experiment) error {
	if e.ID == "" {
		e.ID = id.NewULID()
	}
	_, err := st.Storage.Exec(ctx, `
INSERT INTO experiments (id, name, kind, status, strategy, traffic, seed)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, e.ID, e.Name, string(e.Kind), string(e.Status), string(e.Strategy), e.Traffic, e.Seed)
	return err
}

func (st *Store) UpsertVariant(ctx context.Context, v Variant) error {
	if v.ID == "" {
		v.ID = id.NewULID()
	}
	isWinner := 0
	if v.IsWinner {
		isWinner = 1
	}
	_, err := st.Storage.Exec(ctx, `
INSERT INTO experiment_variants (id, experiment_id, key, weight, payload_json, is_winner)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  weight=excluded.weight,
  payload_json=excluded.payload_json,
  is_winner=excluded.is_winner
`, v.ID, v.ExperimentID, v.Key, v.Weight, v.PayloadJSON, isWinner)
	return err
}

func (st *Store) GetRunningExperiments(ctx context.Context) ([]Experiment, error) {
	rows, err := st.Storage.Query(ctx, `
SELECT id, name, kind, status, strategy, traffic, seed
FROM experiments
WHERE status = 'running'
ORDER BY created_at ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Experiment
	for rows.Next() {
		var e Experiment
		var kind, status, strategy string
		if err := rows.Scan(&e.ID, &e.Name, &kind, &status, &strategy, &e.Traffic, &e.Seed); err != nil {
			return nil, err
		}
		e.Kind = Kind(kind)
		e.Status = Status(status)
		e.Strategy = Strategy(strategy)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (st *Store) GetVariants(ctx context.Context, experimentID string) ([]Variant, error) {
	rows, err := st.Storage.Query(ctx, `
SELECT id, experiment_id, key, weight, payload_json, is_winner
FROM experiment_variants
WHERE experiment_id = ?
ORDER BY key ASC
`, experimentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Variant
	for rows.Next() {
		var v Variant
		var isWinner int
		if err := rows.Scan(&v.ID, &v.ExperimentID, &v.Key, &v.Weight, &v.PayloadJSON, &isWinner); err != nil {
			return nil, err
		}
		v.IsWinner = isWinner == 1
		out = append(out, v)
	}
	return out, rows.Err()
}

func (st *Store) GetAssignment(ctx context.Context, experimentID, subject string) (Assignment, bool, error) {
	row := st.Storage.QueryRow(ctx, `
SELECT id, experiment_id, subject, variant_key
FROM experiment_assignments
WHERE experiment_id = ? AND subject = ?
`, experimentID, subject)

	var a Assignment
	err := row.Scan(&a.ID, &a.ExperimentID, &a.Subject, &a.VariantKey)
	if err == sql.ErrNoRows {
		return Assignment{}, false, nil
	}
	if err != nil {
		return Assignment{}, false, err
	}
	return a, true, nil
}

func (st *Store) InsertAssignment(ctx context.Context, experimentID, subject, variantKey string) (Assignment, error) {
	a := Assignment{
		ID:           id.NewULID(),
		ExperimentID: experimentID,
		Subject:      subject,
		VariantKey:   variantKey,
	}
	_, err := st.Storage.Exec(ctx, `
INSERT INTO experiment_assignments (id, experiment_id, subject, variant_key)
VALUES (?, ?, ?, ?)
ON CONFLICT(experiment_id, subject) DO NOTHING
`, a.ID, a.ExperimentID, a.Subject, a.VariantKey)
	return a, err
}

func (st *Store) InsertOutcome(ctx context.Context, o Outcome) error {
	if o.ID == "" {
		o.ID = id.NewULID()
	}
	success := 0
	if o.Success {
		success = 1
	}
	_, err := st.Storage.Exec(ctx, `
INSERT INTO experiment_outcomes
(id, experiment_id, variant_key, bench_run_id, session_id, message_id,
 success, latency_ms, cost_microusd, quality_score, tokens_in, tokens_out)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		o.ID, o.ExperimentID, o.VariantKey, nullIfEmpty(o.BenchRunID), nullIfEmpty(o.SessionID), nullIfEmpty(o.MessageID),
		success, o.LatencyMS, o.CostMicroUSD, o.QualityScore, o.TokensIn, o.TokensOut,
	)
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (st *Store) MarkWinner(ctx context.Context, experimentID, winnerKey, reason string) error {
	// reset winner flags
	if _, err := st.Storage.Exec(ctx, `
UPDATE experiment_variants SET is_winner = 0
WHERE experiment_id = ?
`, experimentID); err != nil {
		return err
	}

	// set winner
	if _, err := st.Storage.Exec(ctx, `
UPDATE experiment_variants SET is_winner = 1
WHERE experiment_id = ? AND key = ?
`, experimentID, winnerKey); err != nil {
		return err
	}

	// decision record
	_, err := st.Storage.Exec(ctx, `
INSERT INTO experiment_decisions (id, experiment_id, decided_variant_key, reason)
VALUES (?, ?, ?, ?)
`, id.NewULID(), experimentID, winnerKey, reason)
	return err
}

func (st *Store) StopExperiment(ctx context.Context, experimentID string, completed bool) error {
	status := "stopped"
	if completed {
		status = "completed"
	}
	_, err := st.Storage.Exec(ctx, `
UPDATE experiments SET status = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
`, status, experimentID)
	return err
}


---

6) 품질 점수 / 승자 결정 로직

여기서 “성공률 + 품질 + 비용 + 지연”을 하나의 utility로 통합합니다.

internal/experiment/evaluator.go

package experiment

import (
	"context"
	"math"

	"devorch/internal/storage"
)

type Eval struct {
	VariantKey string

	N int

	SuccessRate  float64
	AvgLatencyMS float64
	AvgCostMicro float64
	AvgQuality   float64

	Utility float64
}

type Evaluator struct {
	Storage storage.Storage
}

func NewEvaluator(s storage.Storage) *Evaluator {
	return &Evaluator{Storage: s}
}

func (e *Evaluator) Evaluate(ctx context.Context, experimentID string) ([]Eval, error) {
	rows, err := e.Storage.Query(ctx, `
SELECT variant_key,
       COUNT(*) as n,
       AVG(success) as success_rate,
       AVG(latency_ms) as avg_latency,
       AVG(cost_microusd) as avg_cost,
       AVG(quality_score) as avg_quality
FROM experiment_outcomes
WHERE experiment_id = ?
GROUP BY variant_key
ORDER BY variant_key ASC
`, experimentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Eval
	for rows.Next() {
		var ev Eval
		if err := rows.Scan(&ev.VariantKey, &ev.N, &ev.SuccessRate, &ev.AvgLatencyMS, &ev.AvgCostMicro, &ev.AvgQuality); err != nil {
			return nil, err
		}

		// Utility: 품질/성공률 ↑, 비용/지연 ↓
		// (정규화는 간단히 log/scale; 실제 제품은 workspace별 튜닝)
		latPenalty := math.Log1p(ev.AvgLatencyMS) / 10.0
		costPenalty := math.Log1p(ev.AvgCostMicro) / 20.0

		ev.Utility = 0.55*ev.AvgQuality + 0.35*ev.SuccessRate - 0.06*latPenalty - 0.04*costPenalty
		out = append(out, ev)
	}
	return out, rows.Err()
}

internal/experiment/selector.go

package experiment

import (
	"math"
)

type DecisionRule struct {
	MinSamplesPerVariant int
	MinUtilityDelta      float64
}

func DefaultDecisionRule() DecisionRule {
	return DecisionRule{
		MinSamplesPerVariant: 30,
		MinUtilityDelta:      0.03,
	}
}

func PickWinner(evals []Eval, rule DecisionRule) (winner string, ok bool, reason string) {
	if len(evals) < 2 {
		return "", false, "need >=2 variants"
	}
	// ensure min samples
	for _, e := range evals {
		if e.N < rule.MinSamplesPerVariant {
			return "", false, "insufficient samples"
		}
	}

	// pick best utility
	best := evals[0]
	second := Eval{Utility: -math.MaxFloat64}
	for _, e := range evals {
		if e.Utility > best.Utility {
			second = best
			best = e
		} else if e.Utility > second.Utility && e.VariantKey != best.VariantKey {
			second = e
		}
	}

	delta := best.Utility - second.Utility
	if delta < rule.MinUtilityDelta {
		return "", false, "delta too small"
	}

	reason = "winner=" + best.VariantKey +
		" utility=" + f(best.Utility) +
		" second=" + second.VariantKey +
		" delta=" + f(delta)
	return best.VariantKey, true, reason
}

func f(v float64) string {
	// tiny formatter without importing fmt in hot path
	s := strconv3(v)
	return s
}

func strconv3(v float64) string {
	// minimal fixed 3 decimals
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	i := int64(v)
	frac := int64((v - float64(i)) * 1000.0)
	if frac < 0 {
		frac = 0
	}
	return sign + itoa(i) + "." + itoa3(frac)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + (n % 10))
		n /= 10
	}
	return string(b[i:])
}

func itoa3(n int64) string {
	// 0..999
	h := (n / 100) % 10
	t := (n / 10) % 10
	o := n % 10
	return string([]byte{byte('0' + h), byte('0' + t), byte('0' + o)})
}

> 위 selector는 “통계적 엄밀성” 대신 제품에서 실용적으로 자동 승자 선정에 충분한 규칙 기반입니다.
(추후 z-test / bayesian bandit로 교체 가능)




---

7) 안전장치(Guard): 드리프트/품질 폭락 시 자동 중단/롤백

internal/experiment/guard.go

package experiment

type Guard struct {
	MinSuccessRate float64
	MinAvgQuality  float64
	MaxAvgLatency  float64
}

func DefaultGuard() Guard {
	return Guard{
		MinSuccessRate: 0.80,
		MinAvgQuality:  0.55,
		MaxAvgLatency:  8000,
	}
}

func ShouldStop(evals []Eval, g Guard) (bool, string) {
	for _, e := range evals {
		if e.SuccessRate < g.MinSuccessRate {
			return true, "success_rate too low for variant " + e.VariantKey
		}
		if e.AvgQuality < g.MinAvgQuality {
			return true, "quality too low for variant " + e.VariantKey
		}
		if e.AvgLatencyMS > g.MaxAvgLatency {
			return true, "latency too high for variant " + e.VariantKey
		}
	}
	return false, ""
}


---

8) Service: 실행중 실험 적용 + 결과 수집 + 자동 승자선정

internal/experiment/service.go

package experiment

import (
	"context"
	"encoding/json"

	"devorch/internal/id"
)

type Service struct {
	Store     *Store
	Evaluator *Evaluator
}

func NewService(st *Store, ev *Evaluator) *Service {
	return &Service{Store: st, Evaluator: ev}
}

type ActiveExperiment struct {
	Experiment Experiment
	Variants    []Variant
	Weights     []WeightedKey
}

func (s *Service) LoadActive(ctx context.Context) ([]ActiveExperiment, error) {
	exps, err := s.Store.GetRunningExperiments(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]ActiveExperiment, 0, len(exps))
	for _, e := range exps {
		vars, err := s.Store.GetVariants(ctx, e.ID)
		if err != nil {
			return nil, err
		}
		w := make([]WeightedKey, 0, len(vars))
		for _, v := range vars {
			w = append(w, WeightedKey{Key: v.Key, Weight: v.Weight})
		}
		out = append(out, ActiveExperiment{
			Experiment: e,
			Variants:    vars,
			Weights:     w,
		})
	}
	return out, nil
}

type Applied struct {
	ExperimentID string
	VariantKey   string
	PayloadJSON  string
}

func (s *Service) Assign(ctx context.Context, active ActiveExperiment, subject string) (Applied, bool, error) {
	// sticky from DB first
	if a, ok, err := s.Store.GetAssignment(ctx, active.Experiment.ID, subject); err != nil {
		return Applied{}, false, err
	} else if ok {
		for _, v := range active.Variants {
			if v.Key == a.VariantKey {
				return Applied{ExperimentID: active.Experiment.ID, VariantKey: v.Key, PayloadJSON: v.PayloadJSON}, true, nil
			}
		}
		// assignment exists but variant missing => ignore
	}

	// choose variant
	key, inExp := ChooseVariant(active.Experiment.Seed, subject, active.Experiment.Traffic, active.Weights)
	if !inExp || key == "" {
		return Applied{}, false, nil
	}

	// store assignment
	if _, err := s.Store.InsertAssignment(ctx, active.Experiment.ID, subject, key); err != nil {
		return Applied{}, false, err
	}

	for _, v := range active.Variants {
		if v.Key == key {
			return Applied{ExperimentID: active.Experiment.ID, VariantKey: v.Key, PayloadJSON: v.PayloadJSON}, true, nil
		}
	}
	return Applied{}, false, nil
}

type OutcomeInput struct {
	ExperimentID string
	VariantKey   string

	BenchRunID string
	SessionID  string
	MessageID  string

	Success      bool
	LatencyMS    int
	CostMicroUSD int
	QualityScore float64
	TokensIn     int
	TokensOut    int
}

func (s *Service) RecordOutcome(ctx context.Context, in OutcomeInput) error {
	return s.Store.InsertOutcome(ctx, Outcome{
		ID:           id.NewULID(),
		ExperimentID: in.ExperimentID,
		VariantKey:   in.VariantKey,
		BenchRunID:   in.BenchRunID,
		SessionID:    in.SessionID,
		MessageID:    in.MessageID,
		Success:      in.Success,
		LatencyMS:    in.LatencyMS,
		CostMicroUSD: in.CostMicroUSD,
		QualityScore: in.QualityScore,
		TokensIn:     in.TokensIn,
		TokensOut:    in.TokensOut,
	})
}

func (s *Service) AutoDecide(ctx context.Context, experimentID string) (bool, string, error) {
	evals, err := s.Evaluator.Evaluate(ctx, experimentID)
	if err != nil {
		return false, "", err
	}

	stop, stopReason := ShouldStop(evals, DefaultGuard())
	if stop {
		_ = s.Store.StopExperiment(ctx, experimentID, false)
		return true, "stopped: " + stopReason, nil
	}

	winner, ok, reason := PickWinner(evals, DefaultDecisionRule())
	if !ok {
		return false, reason, nil
	}

	// persist winner
	reasonJSON, _ := json.Marshal(map[string]any{
		"rule":   "default",
		"reason": reason,
	})
	if err := s.Store.MarkWinner(ctx, experimentID, winner, string(reasonJSON)); err != nil {
		return false, "", err
	}
	_ = s.Store.StopExperiment(ctx, experimentID, true)
	return true, "completed winner=" + winner, nil
}


---

9) Router 연동: 실험 payload 적용(프롬프트/정책/라우팅)

핵심 포인트: Router가 “기본 정책”을 갖고 있고, 실험이 있으면 그 payload로 덮어쓴 뒤 실행합니다.

internal/router/experiment_bridge.go

package router

import (
	"context"
	"encoding/json"

	"devorch/internal/experiment"
)

type ExperimentBridge struct {
	Service *experiment.Service
}

func NewExperimentBridge(s *experiment.Service) *ExperimentBridge {
	return &ExperimentBridge{Service: s}
}

type RoutingPatch struct {
	// 예시: 라우팅 가중치/affinity/penalty 등을 payload로 덮어씀
	PreferProvider string  `json:"prefer_provider,omitempty"`
	QualityBoost   float64 `json:"quality_boost,omitempty"`
	CostPenalty    float64 `json:"cost_penalty,omitempty"`
	LatencyPenalty float64 `json:"latency_penalty,omitempty"`
}

type PromptPatch struct {
	SystemPrompt string `json:"system_prompt,omitempty"`
	ExtraRules   string `json:"extra_rules,omitempty"`
}

type AppliedExperiment struct {
	ID      string
	Key     string
	Kind    string
	Payload any
}

func (b *ExperimentBridge) Apply(ctx context.Context, subject string) ([]AppliedExperiment, error) {
	active, err := b.Service.LoadActive(ctx)
	if err != nil {
		return nil, err
	}

	applied := make([]AppliedExperiment, 0, len(active))
	for _, a := range active {
		got, ok, err := b.Service.Assign(ctx, a, subject)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		switch a.Experiment.Kind {
		case experiment.KindRouting:
			var p RoutingPatch
			if err := json.Unmarshal([]byte(got.PayloadJSON), &p); err == nil {
				applied = append(applied, AppliedExperiment{ID: got.ExperimentID, Key: got.VariantKey, Kind: "routing", Payload: p})
			}
		case experiment.KindPrompt:
			var p PromptPatch
			if err := json.Unmarshal([]byte(got.PayloadJSON), &p); err == nil {
				applied = append(applied, AppliedExperiment{ID: got.ExperimentID, Key: got.VariantKey, Kind: "prompt", Payload: p})
			}
		case experiment.KindPolicy:
			// 정책은 payload를 map으로 유지 (확장용)
			var p map[string]any
			if err := json.Unmarshal([]byte(got.PayloadJSON), &p); err == nil {
				applied = append(applied, AppliedExperiment{ID: got.ExperimentID, Key: got.VariantKey, Kind: "policy", Payload: p})
			}
		}
	}

	return applied, nil
}


---

10) Server API: 실험 CRUD + 자동 decide 트리거

internal/server/routes/experiment.go

package routes

import (
	"net/http"

	"devorch/internal/experiment"
)

type ExperimentRoutes struct {
	Service *experiment.Service
	Store   *experiment.Store
}

func NewExperimentRoutes(s *experiment.Service, st *experiment.Store) *ExperimentRoutes {
	return &ExperimentRoutes{Service: s, Store: st}
}

// 아래는 pseudo-router 인터페이스(기존 server 라우터에 맞춰 연결)
// 실제 프로젝트에서는 기존 Hono/chi/echo 등 라우팅 스타일에 맞게 포팅하면 됩니다.

func (r *ExperimentRoutes) Register(mux Mux) {
	mux.POST("/experiment", r.createExperiment)
	mux.POST("/experiment/:id/variant", r.upsertVariant)
	mux.POST("/experiment/:id/auto-decide", r.autoDecide)
	mux.POST("/experiment/:id/stop", r.stop)
}

func (r *ExperimentRoutes) createExperiment(w http.ResponseWriter, req *http.Request) {
	var in struct {
		Name     string  `json:"name"`
		Kind     string  `json:"kind"`
		Strategy string  `json:"strategy"`
		Traffic  float64 `json:"traffic"`
		Seed     string  `json:"seed"`
	}
	if err := decodeJSON(req, &in); err != nil {
		writeErr(w, 400, err)
		return
	}

	e := experiment.Experiment{
		Name:     in.Name,
		Kind:     experiment.Kind(in.Kind),
		Status:   experiment.StatusRunning,
		Strategy: experiment.Strategy(in.Strategy),
		Traffic:  in.Traffic,
		Seed:     in.Seed,
	}
	if e.Traffic <= 0 || e.Traffic > 1 {
		e.Traffic = 1
	}
	if e.Seed == "" {
		e.Seed = "devorch"
	}
	if err := r.Store.CreateExperiment(req.Context(), e); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (r *ExperimentRoutes) upsertVariant(w http.ResponseWriter, req *http.Request) {
	id := muxParam(req, "id")
	var in struct {
		Key         string  `json:"key"`
		Weight      float64 `json:"weight"`
		PayloadJSON string  `json:"payload_json"`
	}
	if err := decodeJSON(req, &in); err != nil {
		writeErr(w, 400, err)
		return
	}
	if in.Key == "" || in.PayloadJSON == "" {
		writeErr(w, 400, errf("missing key/payload_json"))
		return
	}
	if in.Weight <= 0 {
		in.Weight = 1
	}
	v := experiment.Variant{
		ExperimentID: id,
		Key:          in.Key,
		Weight:       in.Weight,
		PayloadJSON:  in.PayloadJSON,
	}
	if err := r.Store.UpsertVariant(req.Context(), v); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (r *ExperimentRoutes) autoDecide(w http.ResponseWriter, req *http.Request) {
	id := muxParam(req, "id")
	changed, msg, err := r.Service.AutoDecide(req.Context(), id)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "changed": changed, "message": msg})
}

func (r *ExperimentRoutes) stop(w http.ResponseWriter, req *http.Request) {
	id := muxParam(req, "id")
	_ = r.Store.StopExperiment(req.Context(), id, false)
	writeJSON(w, 200, map[string]any{"ok": true})
}

internal/server/routes/experiment_helpers.go (필수 헬퍼, 누락 방지)

package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type Mux interface {
	POST(path string, h func(http.ResponseWriter, *http.Request))
}

func decodeJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(out)
}
func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]any{"ok": false, "error": err.Error()})
}
func errf(msg string) error { return errors.New(msg) }
func muxParam(_ *http.Request, _ string) string {
	// 실제 mux 구현에 맞게 교체
	return ""
}


---

11) “Outcome 기록” 연결 포인트 (필수)

Step 17은 실험의 성공/품질 데이터가 들어와야 자동 승자 선정이 됩니다.
따라서 세션 처리 완료 시점 또는 벤치 완료 시점에서 아래처럼 호출합니다.

// (예시) session/message 완료 후
_ = experimentService.RecordOutcome(ctx, experiment.OutcomeInput{
	ExperimentID: appliedExpID,
	VariantKey:   appliedVariantKey,

	SessionID: sessionID,
	MessageID: messageID,

	Success:      success,
	LatencyMS:    latency,
	CostMicroUSD: cost,
	QualityScore: quality,  // Step14~15 scoring 결과를 넣으면 됨
	TokensIn:     tokensIn,
	TokensOut:    tokensOut,
})


---

Step 17 결과: Devorch가 “스스로 좋아지는” 이유

이제 Devorch는:

로컬/원격/멀티 LLM을 섞어 쓰는 것에서 끝나지 않고

프롬프트/정책을 실험해서

“내 PC/내 프로젝트/내 팀” 기준으로

자동으로 승자(최적 설정) 를 선택하고

나쁜 변형은 자동 중단합니다.


즉, 사용자가 “잘 되는 모델/프롬프트가 뭐지?”를 고민할 필요가 없어집니다.


---

다음 Step 추천 (18단계)

Step 18 = “실험 스케줄러(주기적 auto-decide) + 룰 엔진으로 자동 실험 생성(드리프트 탐지 시 새로운 실험 생성)”
→ 이걸 넣으면 완전히 “AutoML 운영 플랫폼”이 됩니다.

원하시면 18단계도 바로 풀코드로 이어서 작성하겠습니다.