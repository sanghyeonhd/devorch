package router

import (
	"context"

	"devorch/internal/okaon"
)

type OkAONEnricher struct {
	Store *okaon.Store
}

func (e *OkAONEnricher) Enrich(ctx context.Context, policyKey string, c Candidate) (Candidate, string) {
	if e == nil || e.Store == nil {
		return c, "okaon:disabled"
	}
	agg, err := e.Store.AggregateRecent(ctx, policyKey, c.Provider, c.Model, 80)
	if err != nil || agg.Count == 0 {
		return c, "okaon:none"
	}

	// Step1 heuristic: replace estimates with real observed aggregates
	if agg.AvgLatMS > 0 {
		c.EstLatencyMS = int(agg.AvgLatMS + 0.5)
	}
	c.EstQuality = clamp01(agg.AvgQual)
	// success gets applied in scoring separately; cost stays 0 in step1

	why := "okaon:applied"
	return c, why
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
