좋습니다. **38단계(연결 완성)**은 아래 3가지를 “끊김 없이” 붙입니다.

1. Router가 후보 점수 계산 루프에서 LearningAdapter.Adjust()를 실제 적용


2. Bench 자동 품질평가(quality_eval.go)로 QualityScore(0..1) 산출


3. Hook on_task_complete.go가 “Run 기록 → 품질평가 → Reward 발행”까지 자동 처리



아래는 38단계 풀코드(파일별 전체) 입니다.


---

1) internal/router/router.go (LearningAdapter 실제 적용 포함)

package router

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"devorch/internal/modelresolver"
	"devorch/internal/provider"
)

// Router는 후보 모델(Provider+Model)들을 점수화하고 최적 후보를 선택한다.
// 38단계 핵심: 후보 루프에서 LearningAdapter.Adjust()를 적용하여 "내 PC/내 프로젝트 기준"으로 점수 가산/감산.
type Router struct {
	resolver *modelresolver.Resolver
	reg      provider.Registry

	adapter *LearningAdapter // optional
	clock   func() time.Time
}

func New(resolver *modelresolver.Resolver, reg provider.Registry, adapter *LearningAdapter) *Router {
	return &Router{
		resolver: resolver,
		reg:      reg,
		adapter:  adapter,
		clock: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// RouteInput: 라우팅 입력(요청 컨텍스트)
type RouteInput struct {
	OrgID       string
	WorkspaceID string
	ProjectID   string

	OS        string
	Arch      string
	MachineFP string

	Category string // quick/ultrabrain/visual-engineering 등
	Agent    string // oracle/explore 등 (선택)

	Intent string // "chat"|"tool"|"embed" etc

	// 사용자 정책/옵션
	PreferLocal bool
	MaxLatencyMs int64
	MaxCostMicros int64
	RequireOAuthOnly bool

	// 선택적으로: 특정 provider/model 고정
	OverrideProvider string
	OverrideModel    string
}

// Decision: 선택 결과
type Decision struct {
	Provider string
	Model    string
	Endpoint string

	ScoreBase     float64
	ScoreLearning float64
	ScoreFinal    float64

	// 디버그/설명용
	Ranked []CandidateScore
}

// CandidateScore: 후보별 점수 구성
type CandidateScore struct {
	Provider string
	Model    string
	Endpoint string

	BaseScore float64
	LearnDelta float64
	FinalScore float64

	Reason []string
}

// Route: 후보 수집 -> base scoring -> learning adjustment -> 최종 선택
func (r *Router) Route(ctx context.Context, in RouteInput) (Decision, error) {
	if r.resolver == nil {
		return Decision{}, errors.New("router: resolver is nil")
	}
	if r.reg == nil {
		return Decision{}, errors.New("router: provider registry is nil")
	}

	// 1) 후보 모델 목록 확보 (Resolver가 category/agent 정책 기반으로 후보 provider chain을 만든다고 가정)
	req := modelresolver.RequirementInput{
		Category: in.Category,
		Agent:    in.Agent,
		Intent:   in.Intent,
		OverrideProvider: in.OverrideProvider,
		OverrideModel:    in.OverrideModel,
	}
	candidates, err := r.resolver.ResolveCandidates(ctx, req)
	if err != nil {
		return Decision{}, err
	}
	if len(candidates) == 0 {
		return Decision{}, fmt.Errorf("router: no candidates resolved for category=%q agent=%q", in.Category, in.Agent)
	}

	// 2) 후보별 base score 계산 + LearningAdapter 적용
	scored := make([]CandidateScore, 0, len(candidates))
	for _, c := range candidates {
		// provider instance check
		p, ok := r.reg.Get(c.Provider)
		if !ok {
			continue
		}
		// optional: OAuth-only enforcement (remote API providers만 대상으로)
		if in.RequireOAuthOnly && !p.SupportsOAuthOnly() {
			continue
		}
		// optional: local preference
		if in.PreferLocal && p.Locality() == provider.LocalityRemote {
			// 로컬 우선이면 원격 후보에 약한 패널티를 준다(완전 제외는 아님)
		}

		base, reason := scoreBase(p, c, in)

		learnDelta := 0.0
		final := base
		if r.adapter != nil {
			adjIn := ScoreAdjustmentInput{
				OrgID:       in.OrgID,
				WorkspaceID: in.WorkspaceID,
				ProjectID:   in.ProjectID,

				OS:        in.OS,
				Arch:      in.Arch,
				MachineFP: in.MachineFP,

				Category: in.Category,
				Agent:    in.Agent,

				Provider: c.Provider,
				Model:    c.Model,
			}
			var delta float64
			final, delta, err = r.adapter.Adjust(ctx, adjIn, base)
			if err != nil {
				// learning 실패는 라우팅 실패로 치지 않음(안정성)
				final = base
				delta = 0
				reason = append(reason, "learning_adjust=skip(err)")
			} else {
				learnDelta = delta
				reason = append(reason, fmt.Sprintf("learning_delta=%+.3f", learnDelta))
			}
		}

		// preferLocal penalty/bonus (경미)
		if in.PreferLocal {
			if p.Locality() == provider.LocalityLocal {
				final += 0.03
				reason = append(reason, "prefer_local=bonus")
			} else {
				final -= 0.03
				reason = append(reason, "prefer_local=penalty")
			}
		}

		// latency/cost hard constraints (있으면 컷)
		if in.MaxLatencyMs > 0 && p.EstimatedP50LatencyMs() > float64(in.MaxLatencyMs) {
			reason = append(reason, "constraint=max_latency(exceeded)")
			continue
		}
		if in.MaxCostMicros > 0 && p.EstimatedAvgCostMicros() > float64(in.MaxCostMicros) {
			reason = append(reason, "constraint=max_cost(exceeded)")
			continue
		}

		scored = append(scored, CandidateScore{
			Provider: c.Provider,
			Model:    c.Model,
			Endpoint: c.Endpoint,

			BaseScore:  base,
			LearnDelta: learnDelta,
			FinalScore: final,
			Reason:     reason,
		})
	}

	if len(scored) == 0 {
		return Decision{}, fmt.Errorf("router: no viable candidates after filtering")
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].FinalScore > scored[j].FinalScore
	})

	best := scored[0]
	return Decision{
		Provider:      best.Provider,
		Model:         best.Model,
		Endpoint:      best.Endpoint,
		ScoreBase:     best.BaseScore,
		ScoreLearning: best.LearnDelta,
		ScoreFinal:    best.FinalScore,
		Ranked:        scored,
	}, nil
}

// --- scoring helpers ---

func scoreBase(p provider.Provider, c modelresolver.Candidate, in RouteInput) (float64, []string) {
	// baseScore는 "안정성 + 예상지연 + 예상비용 + 모델적합도"를 단순 가중치로 구성한다.
	// (정교한 버전은 internal/router/score.go로 분리 가능)
	reason := []string{}

	score := 0.0

	// reliability/health (0..1)
	h := p.HealthScore()
	score += 0.40 * h
	reason = append(reason, fmt.Sprintf("health=%.2f", h))

	// latency preference (낮을수록 좋음)
	lat := p.EstimatedP50LatencyMs()
	switch {
	case lat <= 600:
		score += 0.20
		reason = append(reason, "lat=good")
	case lat <= 2000:
		score += 0.10
		reason = append(reason, "lat=ok")
	case lat <= 8000:
		score -= 0.05
		reason = append(reason, "lat=slow")
	default:
		score -= 0.12
		reason = append(reason, "lat=very_slow")
	}

	// cost preference (낮을수록 좋음)
	cost := p.EstimatedAvgCostMicros()
	switch {
	case cost <= 0:
		score += 0.15
		reason = append(reason, "cost=zero")
	case cost <= 20000:
		score += 0.08
		reason = append(reason, "cost=low")
	case cost <= 150000:
		score -= 0.03
		reason = append(reason, "cost=mid")
	default:
		score -= 0.10
		reason = append(reason, "cost=high")
	}

	// category fit (간단)
	fit := p.CategoryFit(in.Category)
	score += 0.25 * fit
	reason = append(reason, fmt.Sprintf("fit[%s]=%.2f", in.Category, fit))

	// locality minor
	if p.Locality() == provider.LocalityLocal {
		score += 0.03
		reason = append(reason, "locality=local(+)")
	} else {
		score += 0.00
		reason = append(reason, "locality=remote")
	}

	return score, reason
}


---

2) internal/bench/quality_eval.go (자동 품질평가: 0..1)

package bench

import (
	"strings"
	"time"
)

// QualityEvaluator: 실행 결과(성공/지연/에러/간단 규칙)를 기반으로 0..1 품질점수 산출
// 목적: "내 PC에서 실제로 잘 동작하는 모델"이 학습/라우팅에 반영될 수 있게 하기 위함.
type QualityEvaluator struct {
	// thresholds
	goodLatencyMs int64
	okLatencyMs   int64

	// penalty/bonus knobs
	successBonus float64
	errorPenalty float64
	slowPenalty  float64

	// output checks
	minOutputChars int
}

func NewQualityEvaluator() *QualityEvaluator {
	return &QualityEvaluator{
		goodLatencyMs: 900,
		okLatencyMs:   2500,

		successBonus: 0.10,
		errorPenalty: 0.35,
		slowPenalty:  0.15,

		minOutputChars: 60,
	}
}

// Eval: RunMeasurement로부터 QualityScore(0..1) 산출
func (q *QualityEvaluator) Eval(m RunMeasurement) float64 {
	// base
	score := 0.55

	// success / error
	if m.Success {
		score += q.successBonus
	} else {
		score -= q.errorPenalty
	}

	// latency
	lat := m.LatencyMs
	if lat <= 0 && !m.StartedAt.IsZero() && !m.CompletedAt.IsZero() && !m.CompletedAt.Before(m.StartedAt) {
		lat = m.CompletedAt.Sub(m.StartedAt).Milliseconds()
	}
	if lat > 0 {
		switch {
		case lat <= q.goodLatencyMs:
			score += 0.10
		case lat <= q.okLatencyMs:
			score += 0.03
		default:
			score -= q.slowPenalty
		}
	}

	// output sanity (너무 짧거나 공백 위주면 패널티)
	out := strings.TrimSpace(m.OutputText)
	if len(out) < q.minOutputChars {
		score -= 0.10
	}

	// error class heuristics
	ec := strings.ToLower(strings.TrimSpace(m.ErrorClass))
	if ec != "" {
		switch {
		case strings.Contains(ec, "rate"):
			score -= 0.05
		case strings.Contains(ec, "timeout"):
			score -= 0.10
		case strings.Contains(ec, "auth"):
			score -= 0.12
		case strings.Contains(ec, "oom") || strings.Contains(ec, "out_of_memory"):
			score -= 0.12
		default:
			score -= 0.06
		}
	}

	// clamp 0..1
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}

// Helper for timestamps in callers
func nowUTC() time.Time { return time.Now().UTC() }


---

3) internal/hook/builtins/on_task_complete.go (Run 기록→품질평가→Reward 발행 자동 연결)

package builtins

import (
	"context"
	"fmt"
	"time"

	"devorch/internal/bench"
	"devorch/internal/id"
)

// 38단계 목표:
// - 작업(또는 요청) 완료 이벤트를 받으면
//   1) RunMeasurement 구성
//   2) QualityEvaluator로 QualityScore 계산
//   3) Bench(OkAONRecorder)로 InsertRun
//   4) RewardEmitter로 자동 Reward 발행(quality 기반)
// 을 수행한다.
//
// 이 훅은 "플랫폼/도구/세션 구조"와 느슨하게 결합되도록 이벤트 payload를 단순 struct로 정의한다.

// TaskCompleteEvent: 데몬 내부에서 "작업 완료" 시점에 생성되는 이벤트(권장 최소 스키마)
type TaskCompleteEvent struct {
	// ids
	OrgID       string
	WorkspaceID string
	ProjectID   string
	SessionID   string
	TaskID      string

	// route result
	Provider string
	Model    string
	Endpoint string

	// context
	Category string
	Agent    string

	// environment
	OS          string
	Arch        string
	MachineFP   string
	HWTier      string
	Locality    string
	NetworkType string

	// content (hook에서는 hash로만 저장되도록 bench recorder가 처리)
	Prompt string
	Output string

	// timing
	StartedAt   time.Time
	CompletedAt time.Time

	// usage
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	CostMicros   int64

	// result
	Success      bool
	ErrorClass   string
	ErrorMessage string

	ToolingHint string
}

// OnTaskCompleteHook: 실제 훅 구현체
type OnTaskCompleteHook struct {
	benchSvc *bench.Service
	qeval    *bench.QualityEvaluator
	emitter  *bench.RewardEmitter
	ulidGen  func() string
}

func NewOnTaskCompleteHook(benchSvc *bench.Service, qeval *bench.QualityEvaluator, emitter *bench.RewardEmitter) *OnTaskCompleteHook {
	return &OnTaskCompleteHook{
		benchSvc: benchSvc,
		qeval:    qeval,
		emitter:  emitter,
		ulidGen: func() string {
			return id.NewULID()
		},
	}
}

// Handle: 상위 hook 체인에서 호출되는 엔트리포인트(프로젝트의 hook 인터페이스에 맞게 연결)
func (h *OnTaskCompleteHook) Handle(ctx context.Context, ev TaskCompleteEvent) error {
	if h.benchSvc == nil || h.qeval == nil || h.emitter == nil {
		return fmt.Errorf("on_task_complete: dependencies missing")
	}

	runID := "run_" + h.ulidGen()
	rewardID := "rwd_" + h.ulidGen()

	rm := bench.RunMeasurement{
		RunID:       runID,
		OrgID:       ev.OrgID,
		WorkspaceID: ev.WorkspaceID,
		ProjectID:   ev.ProjectID,
		SessionID:   ev.SessionID,
		TaskID:      ev.TaskID,

		Category: ev.Category,
		Agent:    ev.Agent,

		Provider: ev.Provider,
		Model:    ev.Model,
		Endpoint: ev.Endpoint,

		OS:          ev.OS,
		Arch:        ev.Arch,
		MachineFP:   ev.MachineFP,
		HWTier:      ev.HWTier,
		Locality:    ev.Locality,
		NetworkType: ev.NetworkType,

		PromptText:  ev.Prompt,
		OutputText:  ev.Output,
		ToolingHint: ev.ToolingHint,

		StartedAt:    ev.StartedAt,
		CompletedAt:  ev.CompletedAt,
		LatencyMs:    calcLatencyMs(ev.StartedAt, ev.CompletedAt),

		InputTokens:  ev.InputTokens,
		OutputTokens: ev.OutputTokens,
		TotalTokens:  ev.TotalTokens,
		CostMicros:   ev.CostMicros,

		Success:      ev.Success,
		ErrorClass:   ev.ErrorClass,
		ErrorMessage: ev.ErrorMessage,
	}

	// 1) QualityScore 산출
	rm.QualityScore = h.qeval.Eval(rm)

	// 2) OkAON Run 저장
	if err := h.benchSvc.RecordRun(ctx, rm); err != nil {
		return err
	}

	// 3) Reward 발행 (quality 기반)
	// reward: (quality-0.5)*2 -> [-1, +1]
	reward := (rm.QualityScore - 0.5) * 2.0
	conf := 0.55
	if ev.Success {
		conf = 0.70
	}
	if !ev.Success {
		conf = 0.60
	}

	re := bench.RewardEvent{
		RewardID:    rewardID,
		RunID:       runID,
		Source:      "auto",
		Kind:        "quality",
		Reward:      reward,
		Confidence:  conf,
		Notes:       fmt.Sprintf("auto_quality=%.3f success=%v", rm.QualityScore, ev.Success),
		At:          time.Now().UTC(),
	}
	if err := h.emitter.Emit(ctx, re); err != nil {
		return err
	}

	return nil
}

func calcLatencyMs(start, end time.Time) int64 {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0
	}
	return end.Sub(start).Milliseconds()
}


---

✅ 38단계 완료 기준 (체크리스트)

[x] Router 후보 점수 루프에서 LearningAdapter.Adjust 적용

[x] Bench 자동 품질평가로 QualityScore(0..1) 산출

[x] Hook이 RecordRun → EmitReward까지 자동 연결

[x] Learning 실패/OkAON 통계 부족 시에도 라우팅이 깨지지 않도록(skip/fallback)



---

원하시면 39단계는 바로 이렇게 가면 가장 “체감”이 큽니다:

internal/router/router.go에 Decision.Ranked를 서버/웹UI로 노출(Explain)

internal/server/routes/router.go에서 /v1/router/trace 같은 API 추가

webui/vscode에 learning delta/최근 14일 stats 시각화


원하시는 39단계 방향(서버 API 먼저 vs UI 먼저) 없이도, 제가 서버 API부터 바로 이어서 풀코드로 진행하겠습니다.