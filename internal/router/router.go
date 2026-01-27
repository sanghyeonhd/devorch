package router

import (
	"context"
	"fmt"
	"math"
)

type Router struct {
	w        Weights
	bandit   Bandit
	enricher *OkAONEnricher
}

func New(w Weights, bandit Bandit, enricher *OkAONEnricher) *Router {
	return &Router{w: w, bandit: bandit, enricher: enricher}
}

func (r *Router) Select(ctx context.Context, wl Workload, candidates []Candidate) (Decision, error) {
	if len(candidates) == 0 {
		return Decision{}, fmt.Errorf("no candidates")
	}

	policyKey := BuildPolicyKey(wl)

	// Enrich using OkAON (real local history)
	enriched := make([]Candidate, 0, len(candidates))
	enrichWhy := make([]string, 0, len(candidates))
	for _, c := range candidates {
		ec, why := r.enricher.Enrich(ctx, policyKey, c)
		enriched = append(enriched, ec)
		enrichWhy = append(enrichWhy, why)
	}

	// Score (higher is better). Normalize latency cheaply: 1/(1+lat)
	bestIdx := 0
	bestScore := -math.MaxFloat64

	scores := make([]float64, len(enriched))
	for i, c := range enriched {
		qual := c.EstQuality
		lat := c.EstLatencyMS
		latScore := 1.0
		if lat > 0 {
			latScore = 1.0 / (1.0 + float64(lat)/200.0)
		}
		costScore := 1.0 // Step1 unknown => neutral

		score := r.w.Quality*qual + r.w.Latency*latScore + r.w.Cost*costScore
		scores[i] = score
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	chosen := bestIdx
	if r.bandit != nil {
		chosen = r.bandit.Choose(bestIdx, len(enriched))
		if chosen < 0 {
			chosen = bestIdx
		}
	}

	c := enriched[chosen]
	why := fmt.Sprintf("score=%.4f (enrich=%s)", scores[chosen], enrichWhy[chosen])

	return Decision{
		Provider: c.Provider,
		Model:    c.Model,
		Endpoint: c.Endpoint,
		Score:    scores[chosen],
		Why:      why,
	}, nil
}
