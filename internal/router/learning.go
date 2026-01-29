package router

import (
	"context"
	"math"

	"devorch/internal/storage/sqlite"
)

type ModelStatsReader interface {
	Get(ctx context.Context, workspace, provider, model string) (*sqlite.ModelStats, error)
}

type LearningScorer struct {
	Stats ModelStatsReader
}

// 라우팅 점수에 "실제 운영 품질"을 넣는다.
// baseScore(기존 latency/cost/affinity) + learnedBonus
// learnedBonus = f(avg_score, fail_rate, avg_latency_ms)
func (l *LearningScorer) LearnedBonus(ctx context.Context, workspace, provider, model string) float64 {
	if l == nil || l.Stats == nil {
		return 0
	}
	ms, err := l.Stats.Get(ctx, workspace, provider, model)
	if err != nil || ms == nil || ms.Samples < 5 {
		return 0 // 샘플 적으면 영향 최소화
	}

	// avg_score(0~1) → [-0.10, +0.20]
	scoreBonus := (ms.AvgScore - 0.5) * 0.4

	// fail_rate(0~1) → [-0.25, 0]
	failPenalty := -0.25 * clamp01(ms.FailRate)

	// latency penalty: 0~30s는 완만, 그 이상은 가파르게
	lat := ms.AvgLatencyMs
	latPenalty := 0.0
	if lat > 3000 {
		latPenalty = -0.05 * math.Log1p((lat-3000)/3000)
	}

	return scoreBonus + failPenalty + latPenalty
}

// Recorder가 학습값 업데이트할 때 쓰는 헬퍼
type ModelStatsWriter interface {
	Get(ctx context.Context, workspace, provider, model string) (*sqlite.ModelStats, error)
	Upsert(ctx context.Context, ms sqlite.ModelStats) error
}
