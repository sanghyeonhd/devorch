package experiment

import (
	"context"
	"database/sql"
	"math"
)

type Eval struct {
	VariantKey string

	N int

	SuccessRate  float64
	AvgLatencyMS float64
	AvgCostMicro float64
	AvgQuality   float64

	Utility float64
}

type Evaluator struct {
	db *sql.DB
}

func NewEvaluator(db *sql.DB) *Evaluator {
	return &Evaluator{db: db}
}

func (e *Evaluator) Evaluate(ctx context.Context, experimentID string) ([]Eval, error) {
	rows, err := e.db.QueryContext(ctx, `
SELECT variant_key,
       COUNT(*) as n,
       AVG(success) as success_rate,
       AVG(latency_ms) as avg_latency,
       AVG(cost_microusd) as avg_cost,
       AVG(quality_score) as avg_quality
FROM experiment_outcomes
WHERE experiment_id = ?
GROUP BY variant_key
ORDER BY variant_key ASC
`, experimentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Eval
	for rows.Next() {
		var ev Eval
		if err := rows.Scan(&ev.VariantKey, &ev.N, &ev.SuccessRate, &ev.AvgLatencyMS, &ev.AvgCostMicro, &ev.AvgQuality); err != nil {
			return nil, err
		}

		// Utility: 품질/성공률 ↑, 비용/지연 ↓
		latPenalty := math.Log1p(ev.AvgLatencyMS) / 10.0
		costPenalty := math.Log1p(ev.AvgCostMicro) / 20.0

		ev.Utility = 0.55*ev.AvgQuality + 0.35*ev.SuccessRate - 0.06*latPenalty - 0.04*costPenalty
		out = append(out, ev)
	}
	return out, rows.Err()
}
