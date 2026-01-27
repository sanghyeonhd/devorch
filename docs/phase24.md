아래는 24단계(“진짜 연결” 단계) 풀코드입니다.
이번 단계는 23단계에서 만든 모듈들을 **실제로 데몬 파이프라인(Background → Router → Hook → Diagnostics)**에 꽂아서 동작하게 만드는 단계입니다.

> ✅ 이번 단계에서 “새로 추가/수정”되는 파일들만 파일별 전체 코드로 제공합니다.
(기존 1~23단계 파일은 그대로 두고, 아래 파일들을 추가/교체하면 됩니다.)




---

1) Background: queueDepth → Feedback 업데이트 연결

1-1) internal/background/manager.go (교체)

package background

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type Task struct {
	ID          string
	Description string
	Run         func(ctx context.Context) error
	CreatedAt   time.Time
}

type Manager struct {
	feedback *Feedback

	queue    chan Task
	wg       sync.WaitGroup
	started  int32
	stopping int32

	// 관측용
	queueDepth int64

	// 동시 실행 제한
	concurrency int
}

func NewManager(feedback *Feedback, concurrency int) *Manager {
	if concurrency <= 0 {
		concurrency = 4
	}
	if feedback == nil {
		feedback = &Feedback{}
	}
	return &Manager{
		feedback:    feedback,
		queue:       make(chan Task, 1024),
		concurrency: concurrency,
	}
}

func (m *Manager) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&m.started, 0, 1) {
		return nil
	}

	// 워커 실행
	for i := 0; i < m.concurrency; i++ {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.worker(ctx)
		}()
	}

	// 관측 루프(혼잡/큐 뎁스)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				d := atomic.LoadInt64(&m.queueDepth)
				m.feedback.SetQueueDepth(d)
				m.feedback.SetCongestion(CongestionScore(d))
			}
		}
	}()

	return nil
}

func (m *Manager) Stop() {
	if atomic.CompareAndSwapInt32(&m.stopping, 0, 1) {
		close(m.queue)
	}
	m.wg.Wait()
}

func (m *Manager) Submit(t Task) error {
	if atomic.LoadInt32(&m.stopping) == 1 {
		return errors.New("background manager stopping")
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}

	atomic.AddInt64(&m.queueDepth, 1)
	select {
	case m.queue <- t:
		return nil
	default:
		atomic.AddInt64(&m.queueDepth, -1)
		return errors.New("background queue full")
	}
}

func (m *Manager) worker(ctx context.Context) {
	for task := range m.queue {
		atomic.AddInt64(&m.queueDepth, -1)

		// 실행
		_ = safeRun(ctx, task.Run)
	}
}

func safeRun(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		_ = recover()
	}()
	if fn == nil {
		return nil
	}
	return fn(ctx)
}


---

2) Provider Pool: Reliability + Decay를 Router가 쓰도록 “조회 API” 제공

2-1) internal/provider/pool/pool.go (교체)

package pool

import (
	"sync"
	"time"
)

type Key struct {
	Provider string
	Model    string
}

type Pool struct {
	mu sync.RWMutex

	// (provider, model) -> reliability
	rel map[Key]*Reliability

	halfLife time.Duration
}

func NewPool(halfLife time.Duration) *Pool {
	if halfLife <= 0 {
		halfLife = 24 * time.Hour
	}
	return &Pool{
		rel:      make(map[Key]*Reliability),
		halfLife: halfLife,
	}
}

func (p *Pool) Observe(provider, model string, success bool, latencyMs int64) {
	if provider == "" || model == "" {
		return
	}
	k := Key{Provider: provider, Model: model}

	p.mu.Lock()
	r := p.rel[k]
	if r == nil {
		r = NewReliability(0.2)
		p.rel[k] = r
	}
	p.mu.Unlock()

	s := 0.0
	if success {
		s = 1.0
	}
	r.Observe(s, float64(latencyMs))
}

type ReliabilitySnapshot struct {
	SuccessEMA float64
	LatencyEMA float64
	Decay      float64
	Last       time.Time
}

func (p *Pool) Snapshot(provider, model string) ReliabilitySnapshot {
	k := Key{Provider: provider, Model: model}

	p.mu.RLock()
	r := p.rel[k]
	p.mu.RUnlock()
	if r == nil {
		return ReliabilitySnapshot{}
	}

	succ, lat, last := r.Snapshot()
	return ReliabilitySnapshot{
		SuccessEMA: succ,
		LatencyEMA: lat,
		Decay:      DecayFactor(last, p.halfLife),
		Last:       last,
	}
}


---

3) Router: LearningAdapter + Reliability + Congestion(Feedback) 반영

> Router가 “최종 스코어”를 만들 때

OkAON 기반 learning_adapter 보정

provider/model reliability(EMA) + decay

background congestion(혼잡) 페널티
를 합산해서 선택합니다.




3-1) internal/router/learning_adapter.go (없으면 추가 / 있으면 교체)

package router

import (
	"context"
	"time"

	"devorch/internal/okAON"
)

type OkStats interface {
	// 최근 arm별 통계 조회(모델 선택에 쓰는 최소 API)
	// - 22단계에서 만든 okaon_arm_stats 기반 구현이 있다면 그걸 사용
	GetArmStats(ctx context.Context, fingerprint, category string, provider string, model string) (okAON.ArmStat, bool, error)
}

type LearningAdapter struct {
	Store OkStats

	// 최근 데이터만 반영하도록
	StaleAfter time.Duration

	// 보정 최대 폭(±)
	MaxDelta float64
}

func DefaultLearningAdapter(store OkStats) *LearningAdapter {
	return &LearningAdapter{
		Store:     store,
		StaleAfter: 21 * 24 * time.Hour,
		MaxDelta:  0.15,
	}
}

// baseScore(0..1)에 학습 보정 적용
func (a *LearningAdapter) AdjustScore(ctx context.Context, baseScore float64, fingerprint, category, provider, model string) (final float64, reason string) {
	if a == nil || a.Store == nil {
		return baseScore, "no_learning_store"
	}
	stat, ok, err := a.Store.GetArmStats(ctx, fingerprint, category, provider, model)
	if err != nil || !ok {
		return baseScore, "no_arm_stats"
	}

	age := time.Since(time.Unix(stat.UpdatedTsUTC, 0).UTC())
	if a.StaleAfter > 0 && age > a.StaleAfter {
		return baseScore, "arm_stats_stale"
	}

	// stat.MeanReward01: 0..1 로 가정
	// baseScore와 meanReward를 섞어서 보정
	mean := clamp01(stat.MeanReward01)
	delta := (mean - 0.5) * 2 * a.MaxDelta // [-MaxDelta, +MaxDelta]

	return clamp01(baseScore + delta), "arm_reward_adjust"
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

> 위에서 쓰는 okAON.ArmStat는 22단계/23단계에 맞춰 아래처럼 정의되어 있어야 합니다(없으면 추가).



3-2) internal/okAON/models_arm.go (없으면 추가)

package okAON

// okaon_arm_stats 최소 표현
type ArmStat struct {
	Fingerprint   string
	Category      string
	Provider      string
	Model         string

	CountRuns     int64
	MeanReward01  float64
	UpdatedTsUTC  int64
}


---

3-3) internal/router/router.go (교체)

package router

import (
	"context"
	"math"
	"time"

	"devorch/internal/provider/pool"
)

type Candidate struct {
	Provider string
	Model    string
	// 정책/요구사항상 기본 점수(0..1)
	BaseScore float64
}

type Feedback interface {
	QueueDepth() int64
	Congestion() int64
}

type Router struct {
	Coldstart ColdstartPolicy

	Pool *pool.Pool

	Learning *LearningAdapter
	Feedback Feedback

	// 혼잡 페널티 최대치(0..1)
	MaxCongestionPenalty float64
}

func NewRouter(p *pool.Pool, la *LearningAdapter, fb Feedback) *Router {
	return &Router{
		Coldstart:             DefaultColdstart(),
		Pool:                  p,
		Learning:              la,
		Feedback:              fb,
		MaxCongestionPenalty:  0.15,
	}
}

type SelectInput struct {
	Fingerprint string
	Category    string

	// 후보군(모델리졸버/요구사항에서 생성)
	Candidates []Candidate
}

type SelectOutput struct {
	Chosen Candidate
	Explain Explain
}

func (r *Router) Select(ctx context.Context, in SelectInput) (SelectOutput, error) {
	exp := Explain{}
	bestScore := -1.0
	var best Candidate

	congPenalty := r.congestionPenalty01()
	now := time.Now()

	for _, c := range in.Candidates {
		base := c.BaseScore
		if base <= 0 {
			base = r.Coldstart.DefaultScore
		}
		score := clamp01(base)

		// 1) learning_adapter 보정
		if r.Learning != nil {
			s2, reason := r.Learning.AdjustScore(ctx, score, in.Fingerprint, in.Category, c.Provider, c.Model)
			if s2 != score {
				exp.Add("learning", reason, s2-score)
			}
			score = s2
		}

		// 2) reliability + decay
		if r.Pool != nil {
			snap := r.Pool.Snapshot(c.Provider, c.Model)
			if snap.Decay > 0 {
				// 성공률이 낮으면 페널티, 높으면 소폭 가산
				relDelta := (snap.SuccessEMA - 0.5) * 0.20 * snap.Decay // 최대 ±0.1 정도
				score = clamp01(score + relDelta)

				// latency는 느릴수록 페널티(EMA ms를 0..1로 약식 정규화)
				latNorm := normalizeLatency01(snap.LatencyEMA)
				latDelta := -(latNorm * 0.10 * snap.Decay)
				score = clamp01(score + latDelta)

				if !snap.Last.IsZero() {
					_ = now // (추후 stale 판단 강화에 사용)
				}
			}
		}

		// 3) 혼잡 페널티
		if congPenalty > 0 {
			score = clamp01(score - congPenalty)
		}

		if score > bestScore {
			bestScore = score
			best = c
		}
	}

	exp.ChosenProvider = best.Provider
	exp.ChosenModel = best.Model
	exp.BaseScore = clamp01(best.BaseScore)
	exp.FinalScore = clamp01(bestScore)
	return SelectOutput{Chosen: best, Explain: exp}, nil
}

func (r *Router) congestionPenalty01() float64 {
	if r.Feedback == nil {
		return 0
	}
	// 0..100
	c := float64(r.Feedback.Congestion())
	if c <= 0 {
		return 0
	}
	if c > 100 {
		c = 100
	}
	return (c / 100.0) * r.MaxCongestionPenalty
}

func normalizeLatency01(latencyMs float64) float64 {
	// 매우 단순: 0ms -> 0, 3000ms -> 1
	if math.IsNaN(latencyMs) || latencyMs <= 0 {
		return 0
	}
	if latencyMs >= 3000 {
		return 1
	}
	return latencyMs / 3000.0
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


---

4) Hook Chain: model switch / retry exhausted 이벤트를 체인에 연결

> 여기서는 Hook 이벤트가 발행되는 위치에서
builtins.OnModelSwitch, builtins.OnRetryExhausted를 호출하도록 연결합니다.



4-1) internal/hook/events.go (교체/정리)

package hook

type EventType string

const (
	EventOnModelSwitch   EventType = "on_model_switch"
	EventOnRetryExhausted EventType = "on_retry_exhausted"
)

4-2) internal/hook/chain.go (교체)

package hook

import (
	"context"

	"devorch/internal/hook/builtins"
)

type Store interface {
	InsertReward(ctx context.Context, rw any) error
}

// 실제 구현에서는 okAON.Reward를 직접 받도록 하는게 정석이지만,
// 여기서는 hook 모듈이 okAON에 강결합되지 않게 “어댑터”로 처리합니다.
type RewardStore interface {
	builtins.SwitchStore
	builtins.RetryStore
}

type Chain struct {
	Store RewardStore
}

type DispatchInput struct {
	Type EventType

	// 공통
	RunID string

	// 모델 스위치
	FromModel   string
	ToModel     string
	Reason      string
	ContextHash string

	// 재시도 소진
	Message string
}

func (c *Chain) Dispatch(ctx context.Context, in DispatchInput) error {
	switch in.Type {
	case EventOnModelSwitch:
		return builtins.OnModelSwitch(ctx, c.Store, builtins.ModelSwitchInput{
			RunID:       in.RunID,
			FromModel:   in.FromModel,
			ToModel:     in.ToModel,
			Reason:      in.Reason,
			ContextHash: in.ContextHash,
		})
	case EventOnRetryExhausted:
		return builtins.OnRetryExhausted(ctx, c.Store, builtins.RetryExhaustedInput{
			RunID:   in.RunID,
			Message: in.Message,
		})
	default:
		return nil
	}
}

> ✅ RewardStore는 실제로는 internal/okAON/sqlite.Store가 그대로 구현하게 됩니다.
(22단계에서 InsertReward(ctx, okAON.Reward) error가 이미 있으므로 충족)




---

5) Diagnostics: OkAON Stats를 실제 “doctor performance”에 연결

5-1) internal/diagnostics/checks/performance.go (신규)

package checks

import (
	"context"
	"fmt"
	"time"

	"devorch/internal/okAON/sqlite"
)

type PerformanceReport struct {
	TopModels     []string
	TopCategories []string
}

func Performance(ctx context.Context, store sqlite.Store) (PerformanceReport, error) {
	now := time.Now().UTC().Unix()
	since := now - int64((7 * 24 * time.Hour).Seconds()) // 최근 7일

	models, err := store.GetTopModels(ctx, since, 10)
	if err != nil {
		return PerformanceReport{}, err
	}
	cats, err := store.GetTopCategories(ctx, since, 10)
	if err != nil {
		return PerformanceReport{}, err
	}

	rep := PerformanceReport{}
	for _, m := range models {
		rep.TopModels = append(rep.TopModels,
			fmt.Sprintf("%s/%s runs=%d q=%.3f lat=%.0fms cost=$%.6f",
				m.Provider, m.Model, m.CountRuns, m.AvgQuality, m.AvgLatencyMs, float64(m.TotalCostMicroUSD)/1_000_000.0))
	}
	for _, c := range cats {
		rep.TopCategories = append(rep.TopCategories,
			fmt.Sprintf("%s runs=%d q=%.3f lat=%.0fms",
				c.Category, c.CountRuns, c.AvgQuality, c.AvgLatencyMs))
	}
	return rep, nil
}


---

6) Router + ProviderPool + HookChain을 “App wiring”에 연결

> 24단계에서 가장 중요한 포인트:
app lifecycle에서 객체들이 서로 연결되어야 합니다.



6-1) internal/app/app.go (교체/최소 버전)

package app

import (
	"context"
	"time"

	"devorch/internal/background"
	"devorch/internal/hook"
	"devorch/internal/okAON/sqlite"
	"devorch/internal/provider/pool"
	"devorch/internal/router"
)

type App struct {
	Ok sqlite.Store

	Pool   *pool.Pool
	BgFB   *background.Feedback
	BgMgr  *background.Manager
	Router *router.Router
	Hooks  *hook.Chain
}

func NewApp(ok sqlite.Store) *App {
	p := pool.NewPool(24 * time.Hour)

	fb := &background.Feedback{}
	bg := background.NewManager(fb, 4)

	// learning adapter는 okAON arm_stats 조회 API를 구현한 store가 필요
	// (22단계에서 sqlite.Store가 GetArmStats를 구현하도록 해두면 그대로 주입 가능)
	la := router.DefaultLearningAdapter(ok)

	rt := router.NewRouter(p, la, fb)

	hc := &hook.Chain{
		Store: ok, // sqlite.Store가 InsertReward 구현
	}

	return &App{
		Ok:     ok,
		Pool:   p,
		BgFB:   fb,
		BgMgr:  bg,
		Router: rt,
		Hooks:  hc,
	}
}

func (a *App) Start(ctx context.Context) error {
	return a.BgMgr.Start(ctx)
}

func (a *App) Stop() {
	a.BgMgr.Stop()
}


---

7) ✅ (중요) 24단계 누락 방지 체크: sqlite.Store에 GetArmStats 연결

Router의 LearningAdapter가 GetArmStats()를 호출하므로, sqlite.Store가 이를 제공해야 합니다.

7-1) internal/okAON/sqlite/arm_stats.go (신규)

package sqlite

import (
	"context"
	"database/sql"

	"devorch/internal/okAON"
)

// fingerprint+category+provider+model에 대한 최신 arm_stats 1건
func (s Store) GetArmStats(ctx context.Context, fingerprint, category, provider, model string) (okAON.ArmStat, bool, error) {
	row := s.DB.QueryRowContext(ctx, `
SELECT fingerprint, category, provider, model, count_runs, mean_reward01, updated_ts_utc
FROM okaon_arm_stats
WHERE fingerprint=? AND category=? AND provider=? AND model=?
`, fingerprint, category, provider, model)

	var st okAON.ArmStat
	var count sql.NullInt64
	var mean sql.NullFloat64
	var updated sql.NullInt64

	err := row.Scan(&st.Fingerprint, &st.Category, &st.Provider, &st.Model, &count, &mean, &updated)
	if err == sql.ErrNoRows {
		return okAON.ArmStat{}, false, nil
	}
	if err != nil {
		return okAON.ArmStat{}, false, err
	}
	st.CountRuns = nzI64(count)
	st.MeanReward01 = nzF64(mean)
	st.UpdatedTsUTC = nzI64(updated)
	return st, true, nil
}


---

24단계 완료 요약(“진짜로 연결됨”)

✅ Background Manager가 queueDepth/혼잡을 지속적으로 계산 → Feedback에 저장

✅ Router가 선택 시

LearningAdapter(OkAON arm_stats) 보정

ProviderPool reliability + decay 반영

혼잡 페널티 반영
→ 최종 점수로 모델 선택


✅ HookChain이 on_model_switch, on_retry_exhausted를 실제 InsertReward로 연결

✅ Diagnostics performance check가 OkAON 통계 조회(GetTopModels/GetTopCategories) 연결

✅ App wiring에서 Router/Pool/Background/Hooks가 실제로 묶임



---

다음 25단계(바로 이어서): “파이프라인 실제 호출 지점” 연결

24단계는 “객체 연결”까지 끝냈고, 25단계는 아래를 실제 요청 처리 흐름에 꽂습니다.

provider 호출 성공/실패 시 Pool.Observe(provider, model, success, latencyMs) 기록

fallback 발생 시 HookChain에 EventOnModelSwitch 발행

retry 모두 소진 시 HookChain에 EventOnRetryExhausted 발행

bench 결과가 RecordBench로 저장되면서 arm_stats 업데이트까지 자동 반영


원하면 25단계 풀코드 바로 이어서 작성하겠습니다.