package router

import (
	"context"
	"sort"

	sqlstore "devorch/internal/storage/sqlite"
)

// BenchRouter uses benchmark history for routing decisions
type BenchRouter struct {
	scorer *BenchScorer
}

func NewBenchRouter(q BenchQuery, w *BenchScoreWeights) *BenchRouter {
	return &BenchRouter{scorer: NewBenchScorer(q, w)}
}

func (r *BenchRouter) Choose(ctx context.Context, candidates []Candidate, rc RequestCtx) (Scored, []Scored) {
	scored := make([]Scored, 0, len(candidates))
	for _, c := range candidates {
		s, reasons := r.scorer.ScoreCandidate(ctx, c, rc)
		scored = append(scored, Scored{
			Candidate: c,
			Score:     s,
			Reasons:   reasons,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) == 0 {
		return Scored{}, scored
	}
	return scored[0], scored
}

// ---- BenchQuery implementation helpers (optional) ----

// SQLiteBenchQuery adapts sqlite.BenchmarksStore to BenchQuery
type SQLiteBenchQuery struct {
	S *sqlstore.BenchmarksStore
}

func (q SQLiteBenchQuery) RecentByModelAndMachine(ctx context.Context, provider, model, scenario, machineFP string, limit int) ([]sqlstore.BenchmarkRow, error) {
	return q.S.RecentByModelAndMachine(ctx, provider, model, scenario, machineFP, limit)
}

func (q SQLiteBenchQuery) RecentByModel(ctx context.Context, provider, model, scenario string, limit int) ([]sqlstore.BenchmarkRow, error) {
	return q.S.RecentByModel(ctx, provider, model, scenario, limit)
}
