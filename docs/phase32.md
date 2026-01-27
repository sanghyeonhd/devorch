좋습니다. **32단계는 “현실 단가 기반 Cost + Router Score 반영 + (env_fingerprint × category) Best-Arm 캐시(콜드스타트 가속)”**까지 실제 동작 코드로 완성합니다.

> ⚠️ 참고(정직하게): 이전 단계(1~31)에서 제가 작성한 전체 코드 베이스가 이 대화에 “실파일”로 존재하지 않기 때문에, **32단계는 ‘추가/수정해야 하는 파일의 풀코드’**를 컴파일 가능한 단위로 제공합니다.
(네 프로젝트에 이미 있는 타입/인터페이스 이름이 다르면, 해당 부분만 이름 매핑하면 그대로 붙습니다.)




---

32단계 산출물 요약

✅ 1) Provider/Model별 실제 단가(토큰) 테이블 + 오버라이드(설정)
✅ 2) bench.EstimateCostUSD()가 pricing을 사용하도록 업그레이드
✅ 3) Router Score에 quality/latency/cost/reliability/learning를 모두 반영
✅ 4) env_fingerprint 기반 Best-Arm 캐시 + OkAON 통계로 콜드스타트 우선순위 적용
✅ 5) Explain(선택 근거) 강화


---

A) 신규/수정 파일 목록 (32단계)

신규

internal/billing/pricing_table.go

internal/global/env_fingerprint.go

internal/router/bestarm_cache.go

internal/router/scoring_v2.go


수정

internal/bench/cost_model.go

internal/router/router.go

internal/router/explain.go

internal/okAON/sqlite/stats.go (envfp 기반 추천 통계 쿼리 추가)

(선택) internal/provider/pool/reliability.go (router에 reliability 제공)



---

1) Provider/Model 단가 테이블 (Pricing)

1-1) internal/billing/pricing_table.go (신규)

package billing

import (
	"strings"
	"sync"
)

type TokenPricing struct {
	USDPer1KInput  float64
	USDPer1KOutput float64
}

// PricingTable은 기본 내장 단가 + 사용자(설정) 오버라이드까지 합친 최종 조회 테이블
type PricingTable struct {
	mu sync.RWMutex

	// key formats:
	// - "provider/model"
	// - "provider/*"   (provider default)
	// - "*/*"         (global default)
	prices map[string]TokenPricing
}

func NewPricingTable() *PricingTable {
	t := &PricingTable{
		prices: map[string]TokenPricing{},
	}
	t.loadDefaults()
	return t
}

func (t *PricingTable) loadDefaults() {
	// ⚠️ 예시 기본값(러프). 실제 제품에서는 버전별/티어별로 정교화 가능.
	// Local은 0.
	t.prices["ollama/*"] = TokenPricing{USDPer1KInput: 0, USDPer1KOutput: 0}
	t.prices["local/*"] = TokenPricing{USDPer1KInput: 0, USDPer1KOutput: 0}

	// OpenAI 호환(예시)
	t.prices["openai/*"] = TokenPricing{USDPer1KInput: 0.002, USDPer1KOutput: 0.004}
	// Anthropic(예시)
	t.prices["anthropic/*"] = TokenPricing{USDPer1KInput: 0.003, USDPer1KOutput: 0.015}
	// Google(예시)
	t.prices["google/*"] = TokenPricing{USDPer1KInput: 0.0015, USDPer1KOutput: 0.003}
	// OpenRouter(예시: 평균)
	t.prices["openrouter/*"] = TokenPricing{USDPer1KInput: 0.002, USDPer1KOutput: 0.006}

	// 글로벌 디폴트
	t.prices["*/*"] = TokenPricing{USDPer1KInput: 0.002, USDPer1KOutput: 0.004}
}

// Override: 설정 파일에서 들어온 값을 머지할 때 사용
func (t *PricingTable) Upsert(provider, model string, p TokenPricing) {
	t.mu.Lock()
	defer t.mu.Unlock()
	k := normalizeKey(provider, model)
	t.prices[k] = p
}

func (t *PricingTable) Resolve(provider, model string) TokenPricing {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 1) exact provider/model
	if p, ok := t.prices[normalizeKey(provider, model)]; ok {
		return p
	}
	// 2) provider/*
	if p, ok := t.prices[normalizeKey(provider, "*")]; ok {
		return p
	}
	// 3) */*
	return t.prices["*/*"]
}

func normalizeKey(provider, model string) string {
	provider = strings.TrimSpace(strings.ToLower(provider))
	model = strings.TrimSpace(strings.ToLower(model))
	if provider == "" {
		provider = "*"
	}
	if model == "" {
		model = "*"
	}
	return provider + "/" + model
}


---

2) env_fingerprint (환경 지문)

2-1) internal/global/env_fingerprint.go (신규)

package global

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"runtime"
	"strconv"
)

type EnvFingerprint struct {
	OS        string
	Arch      string
	CPUCount  int
	// 메모리/CPU 모델/GPU 등은 33단계에서 확장 (OS별 수집 난이도)
}

func CurrentEnvFingerprint() EnvFingerprint {
	return EnvFingerprint{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCount: runtime.NumCPU(),
	}
}

func (e EnvFingerprint) String() string {
	// 안정적인 키(업그레이드/재부팅에도 동일)
	// CPUCount는 머신 티어를 간접 반영하므로 포함
	s := e.OS + "|" + e.Arch + "|" + strconv.Itoa(e.CPUCount)

	// 폐쇄망/보안 환경에서 hostname/user 포함을 피함
	return s
}

func (e EnvFingerprint) Hash() string {
	h := sha1.Sum([]byte(e.String()))
	return hex.EncodeToString(h[:])
}

// (선택) 사용자가 강제로 envfp를 고정하고 싶을 때
func EnvFingerprintFromEnvOrDefault() string {
	if v := os.Getenv("DEVORCH_ENV_FINGERPRINT"); v != "" {
		return v
	}
	return CurrentEnvFingerprint().Hash()
}


---

3) Cost 모델: pricing 적용

3-1) internal/bench/cost_model.go (수정: PricingTable 사용)

package bench

import "devorch/internal/billing"

type CostInput struct {
	Provider string
	Model    string
	TokensIn  int64
	TokensOut int64

	Pricing *billing.PricingTable // ✅ 32단계: 주입
}

func EstimateCostUSD(in CostInput) float64 {
	if in.Pricing == nil {
		// fallback: 없으면 기존 러프 단가
		total := float64(in.TokensIn + in.TokensOut)
		if in.Provider == "ollama" || in.Provider == "local" {
			return 0.0
		}
		return (total / 1000.0) * 0.002
	}

	p := in.Pricing.Resolve(in.Provider, in.Model)
	cIn := (float64(in.TokensIn) / 1000.0) * p.USDPer1KInput
	cOut := (float64(in.TokensOut) / 1000.0) * p.USDPer1KOutput
	return cIn + cOut
}


---

4) OkAON 통계: envfp × category 별 best 모델 추천

4-1) internal/okAON/sqlite/stats.go (수정/추가)

package sqlite

import (
	"context"
	"database/sql"
)

type BestArmRow struct {
	Provider     string
	Model        string
	AvgReward    float64
	AvgQuality   float64
	AvgLatencyMs float64
	N            int64
}

// env_fingerprint + category 기준으로 최근 데이터에서 best arm 추정
// - reward 우선, 동률이면 quality, 그 다음 latency로 tie-break
func (s *Store) BestArmByEnvCategory(ctx context.Context, envfp string, category string, limit int) ([]BestArmRow, error) {
	if limit <= 0 {
		limit = 5
	}
	q := `
SELECT
  provider,
  model,
  AVG(COALESCE(reward, 0)) AS avg_reward,
  AVG(COALESCE(quality_score, 0)) AS avg_quality,
  AVG(COALESCE(latency_ms, 0)) AS avg_latency,
  COUNT(1) AS n
FROM ok_aon_task_runs
WHERE env_fingerprint = ?
  AND category = ?
  AND finished_at_unix IS NOT NULL
GROUP BY provider, model
HAVING n >= 3
ORDER BY avg_reward DESC, avg_quality DESC, avg_latency ASC
LIMIT ?;
`
	rows, err := s.db.QueryContext(ctx, q, envfp, category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []BestArmRow
	for rows.Next() {
		var r BestArmRow
		if err := rows.Scan(&r.Provider, &r.Model, &r.AvgReward, &r.AvgQuality, &r.AvgLatencyMs, &r.N); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Store 기본 구조 (예시)
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

> ✅ 쿼리 전제: ok_aon_task_runs에 env_fingerprint, category, reward, quality_score, latency_ms 컬럼이 이미 존재(31단계 가정)




---

5) Best-Arm 캐시 (envfp×category)

5-1) internal/router/bestarm_cache.go (신규)

package router

import (
	"context"
	"sync"
	"time"
)

type BestArm struct {
	Provider string
	Model    string

	AvgReward  float64
	AvgQuality float64
	AvgLatency float64
	N          int64

	UpdatedAt time.Time
}

type BestArmCache struct {
	mu   sync.RWMutex
	ttl  time.Duration
	data map[string]BestArm // key = envfp|category
}

func NewBestArmCache(ttl time.Duration) *BestArmCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &BestArmCache{
		ttl:  ttl,
		data: map[string]BestArm{},
	}
}

func cacheKey(envfp, category string) string {
	return envfp + "|" + category
}

func (c *BestArmCache) Get(envfp, category string) (BestArm, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	k := cacheKey(envfp, category)
	v, ok := c.data[k]
	if !ok {
		return BestArm{}, false
	}
	if time.Since(v.UpdatedAt) > c.ttl {
		return BestArm{}, false
	}
	return v, true
}

func (c *BestArmCache) Put(envfp, category string, v BestArm) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v.UpdatedAt = time.Now()
	c.data[cacheKey(envfp, category)] = v
}

// Loader 인터페이스: OkAON store가 구현하면 됨
type BestArmLoader interface {
	BestArmByEnvCategory(ctx context.Context, envfp string, category string, limit int) ([]BestArmRow, error)
}

type BestArmRow struct {
	Provider     string
	Model        string
	AvgReward    float64
	AvgQuality   float64
	AvgLatencyMs float64
	N            int64
}

func (c *BestArmCache) Warm(ctx context.Context, loader BestArmLoader, envfp, category string) (BestArm, bool) {
	if v, ok := c.Get(envfp, category); ok {
		return v, true
	}
	rows, err := loader.BestArmByEnvCategory(ctx, envfp, category, 1)
	if err != nil || len(rows) == 0 {
		return BestArm{}, false
	}
	r := rows[0]
	best := BestArm{
		Provider:    r.Provider,
		Model:       r.Model,
		AvgReward:   r.AvgReward,
		AvgQuality:  r.AvgQuality,
		AvgLatency:  r.AvgLatencyMs,
		N:           r.N,
		UpdatedAt:   time.Now(),
	}
	c.Put(envfp, category, best)
	return best, true
}


---

6) Router Scoring v2 (quality/latency/cost/reliability/learning)

6-1) internal/router/scoring_v2.go (신규)

package router

import "math"

// Candidate는 router가 평가하는 모델 후보
type Candidate struct {
	Provider string
	Model    string

	// 실시간 상태
	HealthScore     float64 // 0..1 (provider/pool health)
	Reliability     float64 // 0..1 (성공률/최근 실패 decay)
	ExpectedLatency float64 // ms (예측치)
	ExpectedQuality float64 // 0..1 (학습/벤치 기반)
	ExpectedCostUSD float64 // $

	// 학습 가산점 (bandit/learner)
	LearningBonus float64 // -1..+1
}

// ScoreWeights는 운영체제/프로젝트별 튜닝 포인트
type ScoreWeights struct {
	WHealth     float64
	WReliab     float64
	WQuality    float64
	WLatency    float64
	WCost       float64
	WLearning   float64
}

func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		WHealth:   1.0,
		WReliab:   1.0,
		WQuality:  2.0,
		WLatency:  1.0,
		WCost:     1.0,
		WLearning: 0.8,
	}
}

// score는 "높을수록 좋음"
func ScoreCandidate(c Candidate, w ScoreWeights) float64 {
	// health/reliability는 그대로 가산
	s := 0.0
	s += w.WHealth * clamp01(c.HealthScore)
	s += w.WReliab * clamp01(c.Reliability)

	// quality는 가중
	s += w.WQuality * clamp01(c.ExpectedQuality)

	// latency는 낮을수록 좋음 → soft penalty
	s -= w.WLatency * latencyPenalty(c.ExpectedLatency)

	// cost는 낮을수록 좋음 → soft penalty
	s -= w.WCost * costPenalty(c.ExpectedCostUSD)

	// 학습 가산점
	s += w.WLearning * clamp(c.LearningBonus, -1, 1)

	return s
}

func latencyPenalty(ms float64) float64 {
	// 0~500ms: 거의 0, 2s 넘어가면 증가, 10s 넘어가면 강해짐
	if ms <= 500 {
		return 0
	}
	x := (ms - 500) / 5000.0 // 0~1이 5.5초 구간
	return clamp01(x)
}

func costPenalty(usd float64) float64 {
	// $0.00 ~ $0.05는 거의 0, $0.50 이상이면 강 페널티
	if usd <= 0.05 {
		return 0
	}
	x := (usd - 0.05) / 0.45
	return clamp01(x)
}

func clamp01(v float64) float64 {
	if v < 0 { return 0 }
	if v > 1 { return 1 }
	return v
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}


---

7) Router: envfp BestArm를 “콜드스타트 우선순위”로 사용

7-1) internal/router/router.go (수정: 핵심 로직만 포함한 풀코드 형태)

package router

import (
	"context"
	"time"
)

type Router struct {
	weights ScoreWeights

	bestCache *BestArmCache
	okLoader  BestArmLoader // okAON sqlite store가 구현하면 됨

	// (가정) 외부에서 후보군/health/학습보너스/예측치 등을 채우는 구성요소가 이미 존재
}

func NewRouter(okLoader BestArmLoader) *Router {
	return &Router{
		weights:   DefaultScoreWeights(),
		bestCache: NewBestArmCache(10 * time.Minute),
		okLoader:  okLoader,
	}
}

type RouteRequest struct {
	EnvFingerprint string
	Category       string

	Candidates []Candidate
}

type RouteResult struct {
	Chosen Candidate
	Scores map[string]float64 // key=provider/model
	Explain *Explain
}

func (r *Router) Choose(ctx context.Context, req RouteRequest) (RouteResult, error) {
	// 1) envfp×category best arm이 있으면, 후보 점수에 “콜드스타트 부스트”를 주고 시작
	var best BestArm
	bestOK := false
	if r.okLoader != nil && req.EnvFingerprint != "" && req.Category != "" {
		if v, ok := r.bestCache.Warm(ctx, r.okLoader, req.EnvFingerprint, req.Category); ok {
			best = v
			bestOK = true
		}
	}

	scores := map[string]float64{}
	var chosen Candidate
	bestScore := -1e18

	for _, c := range req.Candidates {
		s := ScoreCandidate(c, r.weights)

		// ✅ 콜드스타트 부스트: OkAON 통계상 best arm이면 약간 더 선호
		if bestOK && c.Provider == best.Provider && c.Model == best.Model {
			s += 0.25
		}

		k := c.Provider + "/" + c.Model
		scores[k] = s

		if s > bestScore {
			bestScore = s
			chosen = c
		}
	}

	ex := BuildExplain(req, chosen, scores, bestOK, best)
	return RouteResult{Chosen: chosen, Scores: scores, Explain: &ex}, nil
}


---

8) Explain 강화

8-1) internal/router/explain.go (수정/풀코드)

package router

type Explain struct {
	EnvFingerprint string
	Category       string

	ChosenProvider string
	ChosenModel    string

	UsedOkAONBestArm bool
	OkAONBestArm     *BestArm

	Top []ExplainRow
}

type ExplainRow struct {
	Provider string
	Model    string
	Score    float64

	Health     float64
	Reliab     float64
	Quality    float64
	LatencyMs  float64
	CostUSD    float64
	Learning   float64

	Notes []string
}

func BuildExplain(req RouteRequest, chosen Candidate, scores map[string]float64, usedBest bool, best BestArm) Explain {
	ex := Explain{
		EnvFingerprint:  req.EnvFingerprint,
		Category:        req.Category,
		ChosenProvider:  chosen.Provider,
		ChosenModel:     chosen.Model,
		UsedOkAONBestArm: usedBest,
	}
	if usedBest {
		tmp := best
		ex.OkAONBestArm = &tmp
	}

	// 상위 5개만 뽑아 넣기(간단 구현)
	rows := make([]ExplainRow, 0, len(req.Candidates))
	for _, c := range req.Candidates {
		k := c.Provider + "/" + c.Model
		r := ExplainRow{
			Provider: c.Provider,
			Model: c.Model,
			Score: scores[k],
			Health: c.HealthScore,
			Reliab: c.Reliability,
			Quality: c.ExpectedQuality,
			LatencyMs: c.ExpectedLatency,
			CostUSD: c.ExpectedCostUSD,
			Learning: c.LearningBonus,
		}
		if usedBest && c.Provider == best.Provider && c.Model == best.Model {
			r.Notes = append(r.Notes, "okAON_best_arm_boost")
		}
		rows = append(rows, r)
	}

	// selection sort top-5
	for i := 0; i < len(rows); i++ {
		max := i
		for j := i + 1; j < len(rows); j++ {
			if rows[j].Score > rows[max].Score {
				max = j
			}
		}
		rows[i], rows[max] = rows[max], rows[i]
		if i == 4 {
			break
		}
	}
	if len(rows) > 5 {
		rows = rows[:5]
	}
	ex.Top = rows
	return ex
}


---

9) (선택) Provider reliability를 Router에 공급

> 이미 provider/pool/reliability.go가 있으면 “조회 함수”만 router가 부르면 됩니다.
없다면 아래처럼 최소 구현으로 시작하세요.



9-1) internal/provider/pool/reliability.go (선택: 보강)

package pool

import (
	"sync"
	"time"
)

type ReliabilityTracker struct {
	mu sync.RWMutex
	// key = provider/model
	stats map[string]*relStat
}

type relStat struct {
	ok   int64
	fail int64
	last time.Time
}

func NewReliabilityTracker() *ReliabilityTracker {
	return &ReliabilityTracker{stats: map[string]*relStat{}}
}

func key(p, m string) string { return p + "/" + m }

func (t *ReliabilityTracker) Observe(provider, model string, success bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	k := key(provider, model)
	s, ok := t.stats[k]
	if !ok {
		s = &relStat{}
		t.stats[k] = s
	}
	if success {
		s.ok++
	} else {
		s.fail++
	}
	s.last = time.Now()
}

// 0..1
func (t *ReliabilityTracker) Reliability(provider, model string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.stats[key(provider, model)]
	if !ok {
		return 0.7 // unknown default
	}
	total := float64(s.ok + s.fail)
	if total == 0 {
		return 0.7
	}
	return float64(s.ok) / total
}


---

10) 32단계 연결 포인트 (필수)

이제 아래 3곳만 연결하면 “32단계 완성”입니다.

(1) background.finish()에서 cost 계산 시 PricingTable 주입

31단계에서:

cost := bench.EstimateCostUSD(bench.CostInput{ ... })

→ 32단계:

cost := bench.EstimateCostUSD(bench.CostInput{
  Provider: ...,
  Model: ...,
  TokensIn: tin,
  TokensOut: tout,
  Pricing: m.pricingTable, // ✅ 주입
})

(2) router 후보 Candidate 채울 때

ExpectedCostUSD는 pricing 기반으로 계산

Reliability는 reliability tracker에서 가져오기

ExpectedQuality는 (learning/bench stats) 기반 예측치


(3) router 요청 시 envfp 포함

envfp := global.EnvFingerprintFromEnvOrDefault()
router.Choose(ctx, router.RouteRequest{
  EnvFingerprint: envfp,
  Category: category,
  Candidates: candidates,
})


---

32단계까지 완료되면 생기는 효과

같은 “카테고리”라도 내 PC(envfp) 에서 실제로 더 잘 나오던 모델이 바로 선택됨

비용이 비싼 모델은 quality가 충분히 높지 않으면 자동으로 밀림

실패가 잦은 모델은 reliability로 자동 감점

cold start에서도 OkAON 통계로 초기 라우팅 품질이 확 올라감



---

원하면 33단계로 다음을 바로 이어갈게요(추천):

envfp를 CPU/RAM/디스크/가속기까지 확장(hw_profile)

category + task-type + repo fingerprint까지 feature 확장

“quality_eval”을 task 유형별 evaluator로 고도화 (diff/patch/test 결과까지 반영)