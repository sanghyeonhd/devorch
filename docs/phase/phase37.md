좋습니다. 37단계 = Bench(실측/품질게이트) → OkAON 저장 + Reward 발행 + Router가 OkAON 통계로 “가산점/패널티”를 적용하는 연결을 풀코드로 드립니다.

이번 단계에서 제공하는 파일(풀코드):

internal/bench/bench.go

internal/bench/recorder.go

internal/bench/reward_emit.go

internal/bench/learner_hook.go

internal/router/learning_adapter.go


> 전제: 36단계의 internal/okAON/* 및 internal/okAON/sqlite/*, 0003_okAON_learning.sql 이 이미 존재합니다.




---

1) internal/bench/bench.go

package bench

import (
	"context"
	"time"

	"devorch/internal/okAON"
)

// RunMeasurement: 실행(한 번의 요청/작업)에 대한 "실측" 입력.
// 실제 raw prompt/output는 저장하지 않고, hash만 저장하는 것을 기본으로 한다.
type RunMeasurement struct {
	// IDs
	RunID       string
	OrgID       string
	WorkspaceID string
	ProjectID   string
	SessionID   string
	TaskID      string

	// Task context
	Category string
	Agent    string

	// Model selection
	Provider string
	Model    string
	Endpoint string

	// Environment
	OS          string
	Arch        string
	MachineFP   string
	HWTier      string
	Locality    string // local/remote/hybrid
	NetworkType string // offline/lan/wan/unknown

	// Content (will be hashed)
	PromptText string
	OutputText string

	// Optional hints
	ToolingHint string // "lsp|astgrep|mcp|..." or empty

	// Timing
	StartedAt   time.Time
	CompletedAt time.Time
	LatencyMs   int64

	// Usage
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	CostMicros   int64

	// Result
	Success      bool
	ErrorClass   string
	ErrorMessage string // short+redacted ideally

	// Proxy / auto eval (0..1)
	QualityScore float64
}

// RewardEvent: 실행 결과에 대한 보상(피드백/자동평가/벤치).
type RewardEvent struct {
	RewardID string
	RunID    string

	Source string // user|auto|bench|qa|policy
	Kind   string // preference|quality|latency|cost|safety|compliance

	Reward     float64 // -1..+1 normalized
	Confidence float64 // 0..1
	Notes      string  // short (redacted)
	At         time.Time
}

// Recorder: Bench 측에서 OkAON에 기록하는 경계.
type Recorder interface {
	RecordRun(ctx context.Context, m RunMeasurement) error
	EmitReward(ctx context.Context, r RewardEvent) error
}

// LearningSink: (선택) 학습 엔진에 이벤트를 전달하고 싶을 때 사용하는 경계.
// 37단계에서는 "연결 포인트"만 제공하고, 실제 학습 로직은 learning/ 패키지에서 처리.
type LearningSink interface {
	OnRunRecorded(ctx context.Context, run okAON.RunRecord) error
	OnRewardEmitted(ctx context.Context, reward okAON.RewardRecord) error
}

// Service: Bench -> OkAON 저장, (옵션) LearningSink로 이벤트 전달
type Service struct {
	rec   Recorder
	sink  LearningSink
	clock func() time.Time
}

func NewService(rec Recorder, sink LearningSink) *Service {
	return &Service{
		rec:   rec,
		sink: sink,
		clock: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) RecordRun(ctx context.Context, m RunMeasurement) error {
	return s.rec.RecordRun(ctx, m)
}

func (s *Service) EmitReward(ctx context.Context, r RewardEvent) error {
	return s.rec.EmitReward(ctx, r)
}


---

2) internal/bench/recorder.go

package bench

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"devorch/internal/okAON"
)

type OkAONRecorder struct {
	store okAON.Store
	// optional: rolling profile update on each run
	enableProfileUpsert bool
}

func NewOkAONRecorder(store okAON.Store) *OkAONRecorder {
	return &OkAONRecorder{store: store, enableProfileUpsert: true}
}

func (r *OkAONRecorder) RecordRun(ctx context.Context, m RunMeasurement) error {
	if m.RunID == "" {
		return fmt.Errorf("bench: RunID required")
	}
	if m.Provider == "" || m.Model == "" {
		return fmt.Errorf("bench: Provider/Model required")
	}

	started := m.StartedAt
	if started.IsZero() {
		started = time.Now().UTC()
	}
	completed := m.CompletedAt
	if completed.IsZero() {
		completed = started
	}

	lat := m.LatencyMs
	if lat <= 0 && !completed.Before(started) {
		lat = completed.Sub(started).Milliseconds()
	}

	// Hash prompt/output only (no raw content)
	pHash := hashText(m.PromptText)
	oHash := hashText(m.OutputText)

	errMsg := redactShort(m.ErrorMessage)

	run := okAON.RunRecord{
		ID:          m.RunID,
		OrgID:       m.OrgID,
		WorkspaceID: m.WorkspaceID,
		ProjectID:   m.ProjectID,
		SessionID:   m.SessionID,
		TaskID:      m.TaskID,

		Category: m.Category,
		Agent:    m.Agent,

		Provider: m.Provider,
		Model:    m.Model,
		Endpoint: m.Endpoint,

		OS:          m.OS,
		Arch:        m.Arch,
		MachineFP:   m.MachineFP,
		HWTier:      m.HWTier,
		Locality:    m.Locality,
		NetworkType: m.NetworkType,

		PromptHash:  pHash,
		OutputHash:  oHash,
		ToolingHint: m.ToolingHint,

		StartedAt:    started,
		CompletedAt:  completed,
		LatencyMs:    lat,
		InputTokens:  m.InputTokens,
		OutputTokens: m.OutputTokens,
		TotalTokens:  nonZero(m.TotalTokens, m.InputTokens+m.OutputTokens),
		CostMicros:   m.CostMicros,

		Success:      m.Success,
		ErrorClass:   m.ErrorClass,
		ErrorMessage: errMsg,

		QualityScore: clamp01(m.QualityScore),
	}

	if err := r.store.InsertRun(ctx, run); err != nil {
		return err
	}

	if r.enableProfileUpsert {
		_ = r.upsertRollingProfile(ctx, run)
	}
	return nil
}

func (r *OkAONRecorder) EmitReward(ctx context.Context, e RewardEvent) error {
	if e.RewardID == "" {
		return fmt.Errorf("bench: RewardID required")
	}
	if e.RunID == "" {
		return fmt.Errorf("bench: RunID required for reward")
	}
	at := e.At
	if at.IsZero() {
		at = time.Now().UTC()
	}

	rr := okAON.RewardRecord{
		ID:         e.RewardID,
		RunID:      e.RunID,
		Source:     e.Source,
		Kind:       e.Kind,
		Reward:     clamp(e.Reward, -1, 1),
		Confidence: clamp01(e.Confidence),
		Notes:      redactShort(e.Notes),
		CreatedAt:  at,
	}
	return r.store.InsertReward(ctx, rr)
}

// ---- rolling profile update (simple EMA) ----
// 실무에서는 별도 batch job로도 가능하지만, 37단계는 "붙어서 돈다"를 목표로 간단히 구현.

func (r *OkAONRecorder) upsertRollingProfile(ctx context.Context, run okAON.RunRecord) error {
	// 최소 조건: machine fingerprint가 있어야 의미가 큼
	if run.OS == "" || run.Arch == "" || run.MachineFP == "" {
		return nil
	}

	// 기존 프로필 조회
	existing, err := r.store.GetModelProfile(ctx, okAON.ModelProfileQuery{
		Provider:  run.Provider,
		Model:     run.Model,
		OS:        run.OS,
		Arch:      run.Arch,
		MachineFP: run.MachineFP,
	})
	if err != nil {
		return err
	}

	// EMA 업데이트
	const alpha = 0.12 // 최근 값에 조금 더 가중
	now := time.Now().UTC()

	sampleCount := int64(1)
	avgLat := float64(run.LatencyMs)
	avgCost := float64(run.CostMicros)
	avgQ := float64(run.QualityScore)
	sr := 0.0
	if run.Success {
		sr = 1.0
	}

	if existing != nil {
		sampleCount = existing.SampleCount + 1

		avgLat = ema(existing.AvgLatencyMs, float64(run.LatencyMs), alpha)
		avgCost = ema(existing.AvgCostMicros, float64(run.CostMicros), alpha)
		avgQ = ema(existing.AvgQuality, float64(run.QualityScore), alpha)
		sr = ema(existing.SuccessRate, boolToFloat(run.Success), alpha)
	}

	profile := okAON.ModelProfile{
		// id는 논리키가 PK라서 "아무 문자열"이어도 되지만, 추적용으로 deterministic하게 생성
		ID:        "prof_" + shortHash(run.Provider+"|"+run.Model+"|"+run.OS+"|"+run.Arch+"|"+run.MachineFP),
		Provider:  run.Provider,
		Model:     run.Model,
		OS:        run.OS,
		Arch:      run.Arch,
		MachineFP: run.MachineFP,

		SampleCount:    sampleCount,
		AvgLatencyMs:   avgLat,
		AvgCostMicros:  avgCost,
		AvgQuality:     avgQ,
		SuccessRate:    sr,
		UpdatedAt:      now,
		FirstSeenAt:    firstSeen(existing, now),
	}
	return r.store.UpsertModelProfile(ctx, profile)
}

// ---- helpers ----

func ema(prev, x, alpha float64) float64 {
	return (1-alpha)*prev + alpha*x
}

func firstSeen(existing *okAON.ModelProfile, now time.Time) time.Time {
	if existing == nil || existing.FirstSeenAt.IsZero() {
		return now
	}
	return existing.FirstSeenAt
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func nonZero(v, fallback int64) int64 {
	if v != 0 {
		return v
	}
	return fallback
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

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func redactShort(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// very simple redaction guard: limit length and remove line breaks
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 180 {
		return s[:180] + "…"
	}
	return s
}

func hashText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:6])
}

// Compile-time guard (optional) if store is backed by sqlite and you want to detect sql.ErrNoRows etc.
var _ = sql.ErrNoRows


---

3) internal/bench/reward_emit.go

package bench

import (
	"context"
	"fmt"
	"time"

	"devorch/internal/okAON"
)

// RewardEmitter: Router/Learning/QA 등에서 reward를 "표준 형태"로 발행하기 위한 유틸.
type RewardEmitter struct {
	store okAON.Store
}

func NewRewardEmitter(store okAON.Store) *RewardEmitter {
	return &RewardEmitter{store: store}
}

// Emit: RewardEvent를 OkAON에 저장한다.
func (e *RewardEmitter) Emit(ctx context.Context, ev RewardEvent) error {
	if ev.RewardID == "" {
		return fmt.Errorf("bench: RewardID required")
	}
	if ev.RunID == "" {
		return fmt.Errorf("bench: RunID required")
	}
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}

	rr := okAON.RewardRecord{
		ID:         ev.RewardID,
		RunID:      ev.RunID,
		Source:     ev.Source,
		Kind:       ev.Kind,
		Reward:     clamp(ev.Reward, -1, 1),
		Confidence: clamp01(ev.Confidence),
		Notes:      ev.Notes,
		CreatedAt:  ev.At,
	}
	return e.store.InsertReward(ctx, rr)
}


---

4) internal/bench/learner_hook.go

package bench

import (
	"context"

	"devorch/internal/okAON"
)

// LearnerHook: bench가 기록한 run/reward를 learning 엔진에 전달하는 연결부.
// 37단계에서는 "연결 포인트"를 제공하고, 실제 학습은 internal/learning/*에서 구현.
type LearnerHook struct {
	sink LearningSink
}

func NewLearnerHook(sink LearningSink) *LearnerHook {
	return &LearnerHook{sink: sink}
}

func (h *LearnerHook) OnRunRecorded(ctx context.Context, run okAON.RunRecord) error {
	if h.sink == nil {
		return nil
	}
	return h.sink.OnRunRecorded(ctx, run)
}

func (h *LearnerHook) OnRewardEmitted(ctx context.Context, reward okAON.RewardRecord) error {
	if h.sink == nil {
		return nil
	}
	return h.sink.OnRewardEmitted(ctx, reward)
}


---

5) internal/router/learning_adapter.go

package router

import (
	"context"
	"fmt"
	"math"
	"time"

	"devorch/internal/okAON"
)

// LearningAdapter: OkAON 통계를 읽어, 라우팅 점수에 "가산/감산"을 적용한다.
// 핵심 목표:
// - 같은 모델이라도 "내 PC / 내 네트워크 / 내 프로젝트"에서 성능 좋은 쪽을 더 선택하도록 유도
// - cold-start 시 과도한 편향을 막기 위해 샘플수 기반 감쇠 적용
type LearningAdapter struct {
	store okAON.Store

	// weighting knobs
	wLatency float64
	wCost    float64
	wQuality float64
	wSuccess float64

	// minimum samples to trust
	minSamples int64

	// stats window
	lastDays int
}

func NewLearningAdapter(store okAON.Store) *LearningAdapter {
	return &LearningAdapter{
		store:       store,
		wLatency:    0.45,
		wCost:       0.20,
		wQuality:    0.25,
		wSuccess:    0.10,
		minSamples:  12,
		lastDays:    14,
	}
}

// ScoreAdjustmentInput: 라우터가 후보 모델에 대해 "학습 기반 가산점"을 요청할 때 넣는 입력.
type ScoreAdjustmentInput struct {
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
}

// Adjust: baseScore에 delta를 더해 반환.
// delta는 [-0.25, +0.25] 범위로 제한(안정성).
func (a *LearningAdapter) Adjust(ctx context.Context, in ScoreAdjustmentInput, baseScore float64) (float64, float64, error) {
	if a.store == nil {
		return baseScore, 0, nil
	}
	if in.Provider == "" || in.Model == "" {
		return baseScore, 0, fmt.Errorf("router: learning adapter requires provider/model")
	}

	stats, err := a.store.GetModelStats(ctx, okAON.ModelStatsQuery{
		OrgID:       in.OrgID,
		WorkspaceID: in.WorkspaceID,
		ProjectID:   in.ProjectID,

		OS:        in.OS,
		Arch:      in.Arch,
		MachineFP: in.MachineFP,

		Category: in.Category,
		Agent:    in.Agent,

		Provider:  in.Provider,
		Model:     in.Model,
		LastDays:  a.lastDays,
		Limit:     1,
	})
	if err != nil {
		return baseScore, 0, err
	}
	if len(stats) == 0 {
		return baseScore, 0, nil // no data yet
	}

	s := stats[0]

	// 샘플수 기반 trust (cold-start 방지)
	trust := a.sampleTrust(s.Samples, a.minSamples)

	// score components (normalized heuristics)
	latScore := a.latencyScore(s.P50LatencyMs, s.P90LatencyMs, s.AvgLatencyMs)
	costScore := a.costScore(s.AvgCostMicros)
	qualScore := a.qualityScore(s.AvgQuality)
	succScore := a.successScore(s.SuccessRate)

	raw := a.wLatency*latScore + a.wCost*costScore + a.wQuality*qualScore + a.wSuccess*succScore
	delta := trust * raw

	// clamp delta to keep router stable
	delta = clamp(delta, -0.25, 0.25)

	return baseScore + delta, delta, nil
}

// ---- scoring heuristics ----

func (a *LearningAdapter) sampleTrust(samples, min int64) float64 {
	if samples <= 0 {
		return 0
	}
	if samples >= min {
		return 1
	}
	// smooth ramp: sqrt
	return math.Sqrt(float64(samples) / float64(min))
}

// latencyScore: 낮을수록 좋다. 값이 아주 큰 경우 점수는 음수로 내려간다.
func (a *LearningAdapter) latencyScore(p50, p90, avg float64) float64 {
	// if percentiles absent, fallback to avg
	v := avg
	if p50 > 0 {
		v = p50
	}
	// 기준(ms): 600ms(좋음), 2000ms(보통), 8000ms(나쁨)
	switch {
	case v <= 600:
		return +0.20
	case v <= 2000:
		return +0.08
	case v <= 8000:
		return -0.08
	default:
		return -0.18
	}
}

// costScore: 낮을수록 좋다(마이크로 단위).
func (a *LearningAdapter) costScore(avgCostMicros float64) float64 {
	// 기준: 0(로컬)~ 20000(저렴) ~ 150000(비쌈)
	switch {
	case avgCostMicros <= 0:
		return +0.18
	case avgCostMicros <= 20000:
		return +0.08
	case avgCostMicros <= 150000:
		return -0.05
	default:
		return -0.14
	}
}

// qualityScore: 높을수록 좋다 (0..1).
func (a *LearningAdapter) qualityScore(avgQ float64) float64 {
	avgQ = clamp(avgQ, 0, 1)
	// 0.85 이상: 매우 좋음
	if avgQ >= 0.85 {
		return +0.20
	}
	if avgQ >= 0.70 {
		return +0.10
	}
	if avgQ >= 0.55 {
		return 0.0
	}
	return -0.12
}

// successScore: 성공률 높을수록 좋다.
func (a *LearningAdapter) successScore(sr float64) float64 {
	sr = clamp(sr, 0, 1)
	if sr >= 0.98 {
		return +0.08
	}
	if sr >= 0.93 {
		return +0.03
	}
	if sr >= 0.85 {
		return -0.05
	}
	return -0.12
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---- optional: explain helper ----

// ExplainLine: 라우팅 UI에서 “왜 이 모델이 선택됐는지”를 보여줄 때 사용할 수 있는 구조.
type ExplainLine struct {
	Provider string
	Model    string
	Samples  int64
	Delta    float64
	Window   string
	At       time.Time
}

func WindowString(days int) string {
	if days <= 0 {
		return "all"
	}
	return fmt.Sprintf("last_%dd", days)
}


---

37단계 “실제 주입/연결” 최소 예시 (짧게)

internal/app/app.go (혹은 wire)에서 DB 준비 후:

okAONStore := okaon_sqlite.New(db)  (36단계 sqlite store)

rec := bench.NewOkAONRecorder(okAONStore)

benchSvc := bench.NewService(rec, nil)

learnAdapter := router.NewLearningAdapter(okAONStore)


그리고 Router 후보 점수 계산 끝나기 직전에:

base, delta, _ := learnAdapter.Adjust(ctx, in, baseScore)

finalScore = base



---

원하시면 38단계로 바로 이어서:

internal/router/router.go에 실제 후보 리스트 루프에서 learning_adapter 적용하는 풀코드

internal/bench/quality_eval.go(간단 자동평가: 규칙/형태/테스트 성공 여부 기반) 풀코드

hook/builtins/on_task_complete.go에서 RunID 생성→RecordRun→EmitReward(자동) 까지 “끊김 없는 연결”


을 전부 풀코드로 작성해드릴게요.