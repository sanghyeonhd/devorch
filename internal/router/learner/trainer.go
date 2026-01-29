package learner

import (
	"context"

	"devorch/internal/router"
)

type Trainer struct {
	PolicyStore *router.PolicyStore
}

func NewTrainer(ps *router.PolicyStore) *Trainer {
	return &Trainer{PolicyStore: ps}
}

func (t *Trainer) Train(ctx context.Context, aggs []Aggregate) error {
	for _, a := range aggs {
		successRate := 0.0
		if a.SampleCount > 0 {
			successRate = float64(a.SuccessCount) / float64(a.SampleCount)
		}

		weight := ComputeWeight(successRate, a.AvgLatency, a.AvgCost)

		err := t.PolicyStore.Upsert(ctx, router.Policy{
			ScopeType: a.ScopeType,
			ScopeID:   a.ScopeID,

			OS:       a.OS,
			Arch:     a.Arch,
			Scenario: a.Scenario,

			Provider: a.Provider,
			Model:    a.Model,

			Weight:       weight,
			SuccessRate:  successRate,
			AvgLatencyMs: a.AvgLatency,
			AvgCost:      a.AvgCost,
			SampleCount:  a.SampleCount,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
