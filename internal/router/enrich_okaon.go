package router

import (
	"context"

	"devorch/internal/okaon"
)

// OkAONAggregator는 OkAON 집계 기능을 제공하는 인터페이스
type OkAONAggregator interface {
	GetModelAggregate(ctx context.Context, since int64) ([]okaon.ModelAggregate, error)
}

type OkAONEnricher struct {
	Store OkAONAggregator
}

func (e *OkAONEnricher) Enrich(ctx context.Context, policyKey string, c Candidate) (Candidate, string) {
	if e == nil || e.Store == nil {
		return c, "okaon:disabled"
	}

	// 최근 집계 데이터 조회
	aggs, err := e.Store.GetModelAggregate(ctx, 0)
	if err != nil {
		return c, "okaon:error"
	}

	// 해당 모델의 집계 찾기
	for _, agg := range aggs {
		if agg.Provider == c.Provider && agg.Model == c.Model {
			// 집계 데이터로 추정치 업데이트
			if agg.AvgLatencyMs > 0 {
				c.EstLatencyMS = int(agg.AvgLatencyMs + 0.5)
			}
			c.EstQuality = clamp01Internal(agg.AvgReward01)
			return c, "okaon:applied"
		}
	}

	return c, "okaon:none"
}

func clamp01Internal(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
