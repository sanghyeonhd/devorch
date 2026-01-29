package router

import (
	"context"
	"math"

	"devorch/internal/storage/sqlite"
)

type BenchReader interface {
	RecentAgg(ctx context.Context, workspace string, limit int) ([]sqlite.BenchAgg, error)
}

type QualityReader interface {
	RecentAgg(ctx context.Context, workspace string, limit int) ([]sqlite.QualityAgg, error)
}

// Scorer scores models based on benchmark and quality results
type Scorer struct {
	Bench   BenchReader
	Quality QualityReader
}

// 낮을수록 좋은 penalty
func (s *Scorer) BenchPenalty(ctx context.Context, workspace, providerName, model string) float64 {
	if s.Bench == nil {
		return 0
	}
	aggs, err := s.Bench.RecentAgg(ctx, workspace, 50)
	if err != nil {
		return 0
	}
	for _, a := range aggs {
		if a.Provider == providerName && a.Model == model {
			lat := float64(a.P50LatencyMs)
			errRate := a.ErrorRate
			// penalty는 낮을수록 좋음. 지수 형태로 완만하게.
			p := math.Log1p(lat/500.0) + (errRate * 2.0)
			return p
		}
	}
	return 0
}

// 높을수록 좋은 보너스(점수에서 빼서 penalty 감소)
func (s *Scorer) QualityBonus(ctx context.Context, workspace, providerName, model string) float64 {
	if s.Quality == nil {
		return 0
	}
	aggs, err := s.Quality.RecentAgg(ctx, workspace, 50)
	if err != nil {
		return 0
	}
	for _, a := range aggs {
		if a.Provider == providerName && a.Model == model {
			// avg_score(0~1), fail_rate(0~1)
			// bonus는 fail_rate에 크게 민감
			return (a.AvgScore * 0.8) - (a.FailRate * 1.2)
		}
	}
	return 0
}

// 최종: baseScore(기존 latency/cost/quality 추정) + bench penalty - quality bonus
func (s *Scorer) AdjustedScore(ctx context.Context, workspace, providerName, model string, baseScore float64) float64 {
	p := s.BenchPenalty(ctx, workspace, providerName, model)
	b := s.QualityBonus(ctx, workspace, providerName, model)
	return baseScore + p - b
}
