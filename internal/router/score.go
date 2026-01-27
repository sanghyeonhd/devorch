package router

import (
	"context"
	"math"

	"devorch/internal/storage/sqlite"
)

type BenchReader interface {
	RecentAgg(ctx context.Context, workspace string, limit int) ([]sqlite.BenchAgg, error)
}

// Scorer scores models based on benchmark results
type Scorer struct {
	Bench BenchReader
}

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
