좋습니다. 23단계는 22단계(OkAON CRUD + Thompson + Router learning_adapter + Hook 2종) 위에 **“운영 가능한 학습/품질/라우팅/유지보수(정리) + 벤치 레코더 + 신뢰도(decay) + 혼잡 피드백”**을 얹는 단계입니다.

아래는 23단계 풀코드(파일별) 입니다. (이번 단계도 표준 Go + SQLite 전제, 외부 의존성 없이 작성)


---

A) OkAON: 통계/집계/보관/정리

A-1) internal/okAON/retention.go

package okAON

import "time"

type RetentionPolicy struct {
	// 예: 30일 지난 work/run/quality/reward 정리
	MaxAge time.Duration

	// 최신 arm_stats는 보관, 너무 오래된 건 정리
	ArmStatsMaxAge time.Duration

	// 삭제는 단계적으로(한 번에 너무 많은 row 삭제 방지)
	MaxDeleteRowsPerTable int
}

func DefaultRetention() RetentionPolicy {
	return RetentionPolicy{
		MaxAge:               30 * 24 * time.Hour,
		ArmStatsMaxAge:       90 * 24 * time.Hour,
		MaxDeleteRowsPerTable: 50_000,
	}
}

A-2) internal/okAON/aggregates.go

package okAON

// 집계 결과(대시보드/라우터 설명에 사용)
type ModelAggregate struct {
	Model        string
	Provider     string
	CountRuns    int64
	AvgLatencyMs float64
	AvgQuality   float64
	TotalCostMicroUSD int64
}

type CategoryAggregate struct {
	Category     string
	CountRuns    int64
	AvgLatencyMs float64
	AvgQuality   float64
}

A-3) internal/okAON/sqlite/stats.go

package sqlite

import (
	"context"
	"database/sql"

	"devorch/internal/okAON"
)

func (s Store) GetTopModels(ctx context.Context, sinceTsUTC int64, limit int) ([]okAON.ModelAggregate, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := s.DB.QueryContext(ctx, `
WITH latest_q AS (
  SELECT q.run_id, q.quality_score
  FROM okaon_quality q
  JOIN (
    SELECT run_id, MAX(ts_utc) AS max_ts
    FROM okaon_quality
    GROUP BY run_id
  ) t ON t.run_id=q.run_id AND t.max_ts=q.ts_utc
)
SELECT
  r.model,
  r.provider,
  COUNT(r.run_id) AS cnt,
  AVG(r.latency_ms) AS avg_latency,
  AVG(COALESCE(lq.quality_score, 0)) AS avg_quality,
  COALESCE(SUM(r.cost_microusd), 0) AS total_cost
FROM okaon_run r
LEFT JOIN latest_q lq ON lq.run_id=r.run_id
WHERE r.ts_utc>=?
GROUP BY r.model, r.provider
ORDER BY avg_quality DESC, avg_latency ASC
LIMIT ?
`, sinceTsUTC, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]okAON.ModelAggregate, 0, limit)
	for rows.Next() {
		var a okAON.ModelAggregate
		var cnt sql.NullInt64
		var avgLat sql.NullFloat64
		var avgQ sql.NullFloat64
		var totalCost sql.NullInt64

		if err := rows.Scan(&a.Model, &a.Provider, &cnt, &avgLat, &avgQ, &totalCost); err != nil {
			return nil, err
		}
		a.CountRuns = nzI64(cnt)
		a.AvgLatencyMs = nzF64(avgLat)
		a.AvgQuality = nzF64(avgQ)
		a.TotalCostMicroUSD = nzI64(totalCost)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s Store) GetTopCategories(ctx context.Context, sinceTsUTC int64, limit int) ([]okAON.CategoryAggregate, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := s.DB.QueryContext(ctx, `
WITH latest_q AS (
  SELECT q.run_id, q.quality_score
  FROM okaon_quality q
  JOIN (
    SELECT run_id, MAX(ts_utc) AS max_ts
    FROM okaon_quality
    GROUP BY run_id
  ) t ON t.run_id=q.run_id AND t.max_ts=q.ts_utc
),
work_run AS (
  SELECT w.category, r.run_id, r.latency_ms
  FROM okaon_work w
  JOIN okaon_run r ON r.work_id=w.work_id
  WHERE r.ts_utc>=?
)
SELECT
  wr.category,
  COUNT(wr.run_id) AS cnt,
  AVG(wr.latency_ms) AS avg_latency,
  AVG(COALESCE(lq.quality_score, 0)) AS avg_quality
FROM work_run wr
LEFT JOIN latest_q lq ON lq.run_id=wr.run_id
GROUP BY wr.category
ORDER BY avg_quality DESC, avg_latency ASC
LIMIT ?
`, sinceTsUTC, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]okAON.CategoryAggregate, 0, limit)
	for rows.Next() {
		var a okAON.CategoryAggregate
		var cnt sql.NullInt64
		var avgLat sql.NullFloat64
		var avgQ sql.NullFloat64
		if err := rows.Scan(&a.Category, &cnt, &avgLat, &avgQ); err != nil {
			return nil, err
		}
		a.CountRuns = nzI64(cnt)
		a.AvgLatencyMs = nzF64(avgLat)
		a.AvgQuality = nzF64(avgQ)
		out = append(out, a)
	}
	return out, rows.Err()
}

A-4) internal/okAON/sqlite/vacuum.go

package sqlite

import (
	"context"
)

func (s Store) Vacuum(ctx context.Context) error {
	// SQLite VACUUM은 잠시 DB를 잠글 수 있음(운영에서는 스케줄링 추천)
	_, err := s.DB.ExecContext(ctx, `VACUUM;`)
	return err
}

A-5) internal/okAON/sqlite/retention.go

package sqlite

import (
	"context"
	"fmt"
	"time"

	"devorch/internal/okAON"
)

func (s Store) ApplyRetention(ctx context.Context, pol okAON.RetentionPolicy) error {
	if pol.MaxAge <= 0 {
		return nil
	}

	now := time.Now().UTC().Unix()
	cut := now - int64(pol.MaxAge.Seconds())
	armCut := now - int64(pol.ArmStatsMaxAge.Seconds())
	limit := pol.MaxDeleteRowsPerTable
	if limit <= 0 {
		limit = 50_000
	}

	// child tables are ON DELETE CASCADE via work_id/run_id 관계
	// 1) okaon_work 삭제 -> run/quality/reward cascade
	if _, err := s.DB.ExecContext(ctx, fmt.Sprintf(`
DELETE FROM okaon_work
WHERE ts_utc < ?
LIMIT %d
`, limit), cut); err != nil {
		return err
	}

	// 2) arm stats 오래된 것 정리
	if pol.ArmStatsMaxAge > 0 {
		if _, err := s.DB.ExecContext(ctx, fmt.Sprintf(`
DELETE FROM okaon_arm_stats
WHERE updated_ts_utc < ?
LIMIT %d
`, limit), armCut); err != nil {
			return err
		}
	}

	return nil
}


---

B) Learning: 회귀(Regression) — “품질 예측/콜드스타트 보조”

> 밴딧이 “선택”이라면, 회귀는 “예측”입니다.
여기서는 온라인 선형회귀(SGD) 최소 구현으로 시작합니다(외부 ML 라이브러리 없이).



B-1) learning/regression/model.go

package regression

import "math"

// 아주 단순한 선형모델: y = w·x + b
// feature는 이미 정규화되어 들어온다고 가정(예: 0..1)
type LinearModel struct {
	W []float64
	B float64
}

func NewLinearModel(dim int) *LinearModel {
	if dim <= 0 {
		dim = 1
	}
	return &LinearModel{
		W: make([]float64, dim),
		B: 0,
	}
}

func (m *LinearModel) Predict(x []float64) float64 {
	if len(x) == 0 || len(m.W) == 0 {
		return clamp01(m.B)
	}
	n := len(m.W)
	if len(x) < n {
		n = len(x)
	}
	y := m.B
	for i := 0; i < n; i++ {
		y += m.W[i] * x[i]
	}
	return clamp01(y)
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

B-2) learning/regression/trainer.go

package regression

// Online SGD trainer (MSE loss)
type Trainer struct {
	LR      float64 // learning rate
	L2      float64 // weight decay
	ClipGrad float64
}

func DefaultTrainer() Trainer {
	return Trainer{
		LR:       0.05,
		L2:       0.0001,
		ClipGrad: 1.0,
	}
}

func (t Trainer) Update(m *LinearModel, x []float64, yTrue float64) {
	if m == nil || len(m.W) == 0 {
		return
	}

	yPred := m.Predict(x)
	err := (yPred - yTrue) // d/dy (1/2 (yPred-yTrue)^2)

	// bias update
	gb := err
	gb = clip(gb, t.ClipGrad)
	m.B -= t.LR * gb

	// weight update
	n := len(m.W)
	if len(x) < n {
		n = len(x)
	}
	for i := 0; i < n; i++ {
		gi := err*x[i] + t.L2*m.W[i]
		gi = clip(gi, t.ClipGrad)
		m.W[i] -= t.LR * gi
	}
}

func clip(v, c float64) float64 {
	if c <= 0 {
		return v
	}
	if v > c {
		return c
	}
	if v < -c {
		return -c
	}
	return v
}


---

C) Router: Coldstart + Explain + Reward Adjust(간단)

C-1) internal/router/coldstart.go

package router

import "time"

type ColdstartPolicy struct {
	// 데이터 없으면 기본 점수(중립)
	DefaultScore float64

	// 최근 데이터가 이 시간보다 오래되면 coldstart처럼 취급
	StaleAfter time.Duration
}

func DefaultColdstart() ColdstartPolicy {
	return ColdstartPolicy{
		DefaultScore: 0.5,
		StaleAfter:   21 * 24 * time.Hour,
	}
}

C-2) internal/router/explain.go

package router

type ExplainItem struct {
	Key   string
	Value string
	Score float64
}

type Explain struct {
	ChosenModel string
	ChosenProvider string
	BaseScore   float64
	FinalScore  float64
	Reasons     []ExplainItem
}

func (e *Explain) Add(key, value string, score float64) {
	e.Reasons = append(e.Reasons, ExplainItem{Key: key, Value: value, Score: score})
}

C-3) internal/router/reward_adjust.go

package router

import "math"

// quality(↑), latency(↓), cost(↓)를 하나의 reward01(0..1)로 합성하는 유틸
// - 지금은 단순 합성(추후 정책/가중치 학습 가능)
type RewardWeights struct {
	Quality float64
	Latency float64
	Cost    float64
}

func DefaultRewardWeights() RewardWeights {
	return RewardWeights{Quality: 0.7, Latency: 0.2, Cost: 0.1}
}

// latencyMsNorm: 0..1 (0 빠름, 1 느림)
// costNorm: 0..1 (0 저렴, 1 비쌈)
// quality: 0..1 (0 나쁨, 1 좋음)
func ComposeReward01(w RewardWeights, quality, latencyMsNorm, costNorm float64) float64 {
	q := clamp01(quality)
	l := clamp01(1.0 - latencyMsNorm)
	c := clamp01(1.0 - costNorm)

	sum := w.Quality + w.Latency + w.Cost
	if sum <= 0 {
		return q
	}
	r := (w.Quality*q + w.Latency*l + w.Cost*c) / sum
	return clamp01(r)
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}


---

D) Background: 피드백/혼잡 제어(최소)

D-1) internal/background/feedback.go

package background

import "sync/atomic"

// 라우터/스케줄러에 힌트를 주는 피드백 채널(최소)
type Feedback struct {
	// 최근 queue depth
	queueDepth int64
	// 최근 congestion score 0..100
	congestion int64
}

func (f *Feedback) SetQueueDepth(v int64) { atomic.StoreInt64(&f.queueDepth, v) }
func (f *Feedback) QueueDepth() int64     { return atomic.LoadInt64(&f.queueDepth) }

func (f *Feedback) SetCongestion(v int64) { atomic.StoreInt64(&f.congestion, v) }
func (f *Feedback) Congestion() int64     { return atomic.LoadInt64(&f.congestion) }

D-2) internal/background/congestion.go

package background

// queueDepth 기반 혼잡 점수 계산(0..100)
// 매우 단순: depth 0 => 0, depth 50 => 100
func CongestionScore(queueDepth int64) int64 {
	if queueDepth <= 0 {
		return 0
	}
	if queueDepth >= 50 {
		return 100
	}
	return queueDepth * 2
}


---

E) Provider Pool: 신뢰도/Decay(고성능 라우팅에 필수)

E-1) internal/provider/pool/reliability.go

package pool

import (
	"sync"
	"time"
)

// 모델/프로바이더별 “실측 신뢰도” (EMA)
type Reliability struct {
	mu sync.Mutex

	// 0..1
	SuccessEMA float64
	LatencyEMA float64 // ms (정규화 전 raw)
	LastTs     time.Time

	// EMA 계수(0..1)
	Alpha float64
}

func NewReliability(alpha float64) *Reliability {
	if alpha <= 0 || alpha >= 1 {
		alpha = 0.2
	}
	return &Reliability{Alpha: alpha}
}

// success: 1/0, latencyMs: 측정치
func (r *Reliability) Observe(success float64, latencyMs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.LastTs.IsZero() {
		r.SuccessEMA = success
		r.LatencyEMA = latencyMs
		r.LastTs = time.Now()
		return
	}

	a := r.Alpha
	r.SuccessEMA = a*success + (1-a)*r.SuccessEMA
	r.LatencyEMA = a*latencyMs + (1-a)*r.LatencyEMA
	r.LastTs = time.Now()
}

func (r *Reliability) Snapshot() (successEMA, latencyEMA float64, last time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.SuccessEMA, r.LatencyEMA, r.LastTs
}

E-2) internal/provider/pool/decay.go

package pool

import "time"

// 오래된 통계는 영향도를 줄이기 위한 decay factor
// - last가 오래될수록 0에 가까워짐
func DecayFactor(last time.Time, halfLife time.Duration) float64 {
	if last.IsZero() {
		return 0.0
	}
	if halfLife <= 0 {
		halfLife = 24 * time.Hour
	}
	age := time.Since(last)
	if age <= 0 {
		return 1.0
	}
	// exp(-ln2 * age/halfLife)
	return expNegLn2(float64(age) / float64(halfLife))
}

func expNegLn2(x float64) float64 {
	// 근사: e^(-0.6931*x)
	// 표준 라이브러리 사용
	// (여기서는 math import 허용)
	return powE(-0.6931471805599453 * x)
}

var powE = func(v float64) float64 {
	// local import pattern 없이 math를 직접 사용
	// => 단일 파일로는 math를 import해야 함
	// 그래서 아래처럼 작성하는 대신 그냥 math.Exp를 쓰는 버전이 더 안전
	return 0
}

⚠️ 위처럼 “math 회피”는 Go에서 의미가 없습니다. 정상 컴파일 버전으로 교체하세요:

✅ internal/provider/pool/decay.go (컴파일 보장)

package pool

import (
	"math"
	"time"
)

func DecayFactor(last time.Time, halfLife time.Duration) float64 {
	if last.IsZero() {
		return 0.0
	}
	if halfLife <= 0 {
		halfLife = 24 * time.Hour
	}
	age := time.Since(last)
	if age <= 0 {
		return 1.0
	}
	return math.Exp(-0.6931471805599453 * float64(age) / float64(halfLife))
}


---

F) Bench: 실측 레코더(라우팅 학습 데이터 생산 핵심)

F-1) internal/bench/bench.go

package bench

import (
	"context"
	"time"
)

type Workload struct {
	Name        string
	Description string
	// 실제론 prompt/template/task 스크립트 등이 들어감
}

type Result struct {
	Provider string
	Model    string

	LatencyMs    int64
	InputTokens  int64
	OutputTokens int64
	CostMicroUSD int64

	// 0..1
	QualityScore float64
	Correctness  float64
	Style        float64
	Safety       float64

	ErrorCode string
	ErrorMsg  string
}

type Runner interface {
	Run(ctx context.Context, wl Workload) (Result, error)
}

type Bench struct {
	Runner Runner
}

func (b Bench) Run(ctx context.Context, wl Workload) (Result, error) {
	start := time.Now()
	res, err := b.Runner.Run(ctx, wl)
	if err != nil {
		return res, err
	}
	if res.LatencyMs <= 0 {
		res.LatencyMs = time.Since(start).Milliseconds()
	}
	return res, nil
}

F-2) internal/bench/recorder.go

package bench

import (
	"context"
	"time"

	"devorch/internal/id"
	"devorch/internal/okAON"
)

type OkStore interface {
	InsertWork(ctx context.Context, w okAON.Work) error
	InsertRun(ctx context.Context, r okAON.Run) error
	InsertQuality(ctx context.Context, q okAON.Quality) error
	InsertReward(ctx context.Context, rw okAON.Reward) error
}

type RecordInput struct {
	ProjectID   string
	WorkspaceID string
	UserID      string
	TaskType    string
	Category    string
	Agent       string

	PromptHash    string
	ContextHash   string
	OS            string
	Arch          string
	HWFingerprint string
	TagsJSON      string

	RoutePolicy string
	WasFallback bool
	RetryCount  int
}

func RecordBench(ctx context.Context, store OkStore, meta RecordInput, res Result, reward01 float64) (workID, runID string, err error) {
	now := time.Now().UTC().Unix()
	workID = id.NewULID()
	runID = id.NewULID()

	w := okAON.Work{
		WorkID:        workID,
		TsUTC:         now,
		ProjectID:     meta.ProjectID,
		WorkspaceID:   meta.WorkspaceID,
		UserID:        meta.UserID,
		TaskType:      meta.TaskType,
		Category:      meta.Category,
		Agent:         meta.Agent,
		PromptHash:    meta.PromptHash,
		ContextHash:   meta.ContextHash,
		OS:            meta.OS,
		Arch:          meta.Arch,
		HWFingerprint: meta.HWFingerprint,
		TagsJSON:      meta.TagsJSON,
	}
	if err := store.InsertWork(ctx, w); err != nil {
		return "", "", err
	}

	r := okAON.Run{
		RunID:        runID,
		WorkID:       workID,
		TsUTC:        now,
		Provider:     res.Provider,
		Model:        res.Model,
		RoutePolicy:  meta.RoutePolicy,
		WasFallback:  meta.WasFallback,
		RetryCount:   meta.RetryCount,
		LatencyMs:    res.LatencyMs,
		InputTokens:  res.InputTokens,
		OutputTokens: res.OutputTokens,
		CostMicroUSD: res.CostMicroUSD,
		ErrorCode:    res.ErrorCode,
		ErrorMsg:     res.ErrorMsg,
	}
	if err := store.InsertRun(ctx, r); err != nil {
		return "", "", err
	}

	q := okAON.Quality{
		QualityID:    id.NewULID(),
		RunID:        runID,
		TsUTC:        now,
		QualityScore: clamp01(res.QualityScore),
		Correctness:  clamp01(res.Correctness),
		Style:        clamp01(res.Style),
		Safety:       clamp01(res.Safety),
		Evaluator:    "bench",
		Notes:        wlNote(meta, res),
	}
	_ = store.InsertQuality(ctx, q)

	rw := okAON.Reward{
		RewardID:   id.NewULID(),
		RunID:      runID,
		TsUTC:      now,
		Reward:     clamp01(reward01),
		RewardType: "composite",
		Weight:     1.0,
	}
	_ = store.InsertReward(ctx, rw)

	return workID, runID, nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func wlNote(meta RecordInput, res Result) string {
	// 너무 길게 남기지 않기(PII/비밀 금지)
	return "bench_record"
}


---

G) Hook: 모델 스위치/재시도 소진 이벤트(최소 구현)

G-1) internal/hook/builtins/on_model_switch.go

package builtins

import (
	"context"
	"time"

	"devorch/internal/id"
	"devorch/internal/okAON"
)

// 모델 스위치 이벤트를 “학습/분석용”으로 저장(선택)
// - 실패/지연으로 인해 fallback이 일어난 경우 원인 추적에 중요
type ModelSwitchInput struct {
	RunID       string
	FromModel   string
	ToModel     string
	Reason      string
	ContextHash string
}

type SwitchStore interface {
	InsertReward(ctx context.Context, rw okAON.Reward) error
}

func OnModelSwitch(ctx context.Context, store SwitchStore, in ModelSwitchInput) error {
	// 여기서는 reward를 마이너스로 찍어 “스위치가 발생한 모델(From)”에 페널티를 주는 식의 최소 정책
	// (정교화는 다음 단계에서)
	if store == nil {
		return nil
	}
	now := time.Now().UTC().Unix()
	rw := okAON.Reward{
		RewardID:   id.NewULID(),
		RunID:      in.RunID,
		TsUTC:      now,
		Reward:     0.0, // 중립(혹은 -값을 쓰고 싶으면 스키마/정책 정리 필요)
		RewardType: "model_switch",
		Weight:     0.3,
	}
	return store.InsertReward(ctx, rw)
}

G-2) internal/hook/builtins/on_retry_exhausted.go

package builtins

import (
	"context"
	"time"

	"devorch/internal/id"
	"devorch/internal/okAON"
)

type RetryExhaustedInput struct {
	RunID   string
	Message string
}

type RetryStore interface {
	InsertReward(ctx context.Context, rw okAON.Reward) error
}

func OnRetryExhausted(ctx context.Context, store RetryStore, in RetryExhaustedInput) error {
	if store == nil {
		return nil
	}
	now := time.Now().UTC().Unix()
	rw := okAON.Reward{
		RewardID:   id.NewULID(),
		RunID:      in.RunID,
		TsUTC:      now,
		Reward:     0.0,
		RewardType: "retry_exhausted",
		Weight:     0.5,
	}
	return store.InsertReward(ctx, rw)
}


---

23단계 “누락 점검” 재확인(이번 단계 완료 후)

이번 23단계로 아래 항목이 채워집니다.

✅ OkAON: retention.go, aggregates.go, sqlite/stats.go, sqlite/retention.go, sqlite/vacuum.go

✅ Learning: regression/model.go, regression/trainer.go

✅ Router: coldstart.go, explain.go, reward_adjust.go

✅ Background: feedback.go, congestion.go

✅ Provider pool: reliability.go, decay.go

✅ Bench: bench.go, recorder.go

✅ Hook events: on_model_switch.go, on_retry_exhausted.go



---

다음(24단계)에서 바로 이어갈 “진짜 연결 작업”

23단계는 “모듈 풀코드”를 내려줬고, 24단계는 실제 데몬 파이프라인에 꽂는 작업이 됩니다.

background/manager.go에서 queueDepth → Feedback 업데이트

router/router.go에서 LearningAdapter.AdjustScore() + DecayFactor() + Reliability 반영

hook 체인에 on_model_switch, on_retry_exhausted 연결

diagnostics/performance.go에서 OkAON stats 조회 출력


원하면 바로 24단계 풀코드도 이어서 작성할게요.