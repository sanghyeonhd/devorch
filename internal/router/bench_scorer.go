package router

import (
	"context"
	"math"
	"strconv"
	"time"

	sqlstore "devorch/internal/storage/sqlite"
)

// BenchScoreWeights는 프로젝트 설정(.devorch/routing.jsonc 등)으로 외부화 가능
type BenchScoreWeights struct {
	LatencyWeight   float64       // 낮을수록 좋음
	ErrorWeight     float64       // 낮을수록 좋음
	QualityWeight   float64       // 높을수록 좋음
	CostWeight      float64       // 낮을수록 좋음
	RecencyHalfLife time.Duration // 최근 데이터 가중치
}

func DefaultBenchWeights() BenchScoreWeights {
	return BenchScoreWeights{
		LatencyWeight:   0.45,
		ErrorWeight:     0.35,
		QualityWeight:   0.15,
		CostWeight:      0.05,
		RecencyHalfLife: 14 * 24 * time.Hour,
	}
}

// BenchQuery는 스코어링에 필요한 최근 벤치 조회를 캡슐화
type BenchQuery interface {
	RecentByModelAndMachine(ctx context.Context, provider, model, scenario, machineFP string, limit int) ([]sqlstore.BenchmarkRow, error)
	RecentByModel(ctx context.Context, provider, model, scenario string, limit int) ([]sqlstore.BenchmarkRow, error)
}

type BenchScorer struct {
	W BenchScoreWeights
	Q BenchQuery
}

func NewBenchScorer(q BenchQuery, w *BenchScoreWeights) *BenchScorer {
	ww := DefaultBenchWeights()
	if w != nil {
		ww = *w
	}
	return &BenchScorer{W: ww, Q: q}
}

// ScoreCandidate: 벤치 이력 기반으로 후보 점수를 계산
func (s *BenchScorer) ScoreCandidate(ctx context.Context, c Candidate, rc RequestCtx) (score float64, reasons []string) {
	// 벤치 데이터 없으면 "기본 중립값" (0.5)로 시작
	base := 0.5
	reasons = []string{}

	if s.Q == nil {
		return base, append(reasons, "no bench store wired; using neutral score")
	}

	// 1) 같은 머신의 최근 이력 우선
	same, _ := s.Q.RecentByModelAndMachine(ctx, c.Provider, c.Model, rc.Scenario, rc.MachineFingerprint, 30)
	// 2) 없으면 전체 이력
	all, _ := s.Q.RecentByModel(ctx, c.Provider, c.Model, rc.Scenario, 50)

	rows := same
	using := "machine"
	if len(rows) < 5 {
		rows = all
		using = "global"
	}
	if len(rows) == 0 {
		return base, append(reasons, "no benchmark history; using neutral score")
	}

	// 집계: 지연/에러/퀄리티/비용을 "최근일수록 더 크게" 가중합
	now := time.Now()
	var (
		wSum        float64
		latMsW      float64
		errRateW    float64
		qualityW    float64
		costW       float64
		qualitySeen bool
		costSeen    bool
	)

	for _, r := range rows {
		age := now.Sub(r.CreatedAt)
		decay := recencyWeight(age, s.W.RecencyHalfLife) // 0..1
		wSum += decay

		latMsW += decay * float64(r.LatencyMs)
		if !r.Success {
			errRateW += decay * 1.0
		}

		if r.QualityScore != nil {
			qualitySeen = true
			qualityW += decay * clamp01(*r.QualityScore)
		}
		if r.CostUSD != nil {
			costSeen = true
			costW += decay * math.Max(0, *r.CostUSD)
		}
	}

	if wSum <= 0 {
		return base, append(reasons, "benchmark weights collapsed; using neutral score")
	}

	latAvg := latMsW / wSum
	errRate := errRateW / wSum

	var qAvg float64 = 0.5
	if qualitySeen {
		qAvg = qualityW / wSum
	}
	var costAvg float64 = 0.0
	if costSeen {
		costAvg = costW / wSum
	}

	// 정규화:
	// - latency: 0ms~5000ms 범위를 1..0으로 맵핑
	// - error:   0..1을 1..0으로 맵핑
	// - quality: 0..1 그대로
	// - cost:    0~0.02 USD를 1..0으로 맵핑(대략, 프로젝트에서 튜닝)
	latNorm := invNormalize(latAvg, 0, 5000)
	errNorm := 1.0 - clamp01(errRate)
	qualNorm := clamp01(qAvg)
	costNorm := invNormalize(costAvg, 0, 0.02)

	// soft constraints penalty
	penalty := 0.0
	if rc.MaxLatency > 0 && time.Duration(latAvg)*time.Millisecond > rc.MaxLatency {
		penalty += 0.15
		reasons = append(reasons, "penalty: exceeds max latency")
	}
	if rc.MaxCostUSD > 0 && costAvg > rc.MaxCostUSD {
		penalty += 0.10
		reasons = append(reasons, "penalty: exceeds max cost")
	}
	if rc.PreferLocal && c.Tags != nil && c.Tags["tier"] == "remote" {
		penalty += 0.08
		reasons = append(reasons, "penalty: prefer local")
	}

	weighted :=
		s.W.LatencyWeight*latNorm +
			s.W.ErrorWeight*errNorm +
			s.W.QualityWeight*qualNorm +
			s.W.CostWeight*costNorm

	// same-machine boost
	if using == "machine" && len(same) >= 5 {
		weighted += 0.05
		reasons = append(reasons, "boost: same-machine history")
	}
	reasons = append(reasons,
		"using="+using,
		"latNorm="+fmt2(latNorm),
		"errNorm="+fmt2(errNorm),
		"qualNorm="+fmt2(qualNorm),
		"costNorm="+fmt2(costNorm),
	)

	out := clamp01(weighted - penalty)
	return out, reasons
}

func recencyWeight(age, halfLife time.Duration) float64 {
	if halfLife <= 0 {
		return 1
	}
	// exp2(-age/halfLife)
	return math.Pow(0.5, float64(age)/float64(halfLife))
}

func invNormalize(x, min, max float64) float64 {
	if max <= min {
		return 0.5
	}
	if x <= min {
		return 1
	}
	if x >= max {
		return 0
	}
	return 1 - (x-min)/(max-min)
}

func fmt2(x float64) string {
	// avoid fmt import heavy; tiny formatter
	// 0.1234 -> "0.12"
	y := math.Round(x*100) / 100
	return strconv.FormatFloat(y, 'f', 2, 64)
}
