package sqlite

import (
	"context"

	"devorch/internal/okaon"
)

// GetModelAggregate returns model-level aggregates
func (s *Store) GetModelAggregate(ctx context.Context, since int64) ([]okaon.ModelAggregate, error) {
	const q = `
		SELECT 
			r.provider,
			r.model,
			COUNT(*) as total_runs,
			SUM(CASE WHEN r.error_code = '' THEN 1 ELSE 0 END) as success_runs,
			SUM(CASE WHEN r.error_code != '' THEN 1 ELSE 0 END) as failure_runs,
			COALESCE(AVG(r.latency_ms), 0) as avg_latency_ms,
			COALESCE(AVG(rw.reward_01), 0) as avg_reward,
			COALESCE(SUM(r.cost_micro_usd), 0) / 1000000.0 as total_cost_usd
		FROM okaon_runs r
		LEFT JOIN okaon_rewards rw ON r.id = rw.run_id
		WHERE r.created_at >= ?
		GROUP BY r.provider, r.model
		ORDER BY total_runs DESC
	`

	rows, err := s.db.QueryContext(ctx, q, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []okaon.ModelAggregate
	for rows.Next() {
		var m okaon.ModelAggregate
		if err := rows.Scan(
			&m.Provider, &m.Model,
			&m.TotalRuns, &m.SuccessRuns, &m.FailureRuns,
			&m.AvgLatencyMs, &m.AvgReward01, &m.TotalCostUSD,
		); err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

// GetCategoryAggregate returns category-level aggregates
func (s *Store) GetCategoryAggregate(ctx context.Context, since int64) ([]okaon.CategoryAggregate, error) {
	const q = `
		SELECT 
			w.category,
			COUNT(DISTINCT r.id) as total_runs,
			COUNT(DISTINCT r.provider || ':' || r.model) as unique_models,
			COALESCE(AVG(rw.reward_01), 0) as avg_reward
		FROM okaon_works w
		JOIN okaon_runs r ON w.id = r.work_id
		LEFT JOIN okaon_rewards rw ON r.id = rw.run_id
		WHERE w.created_at >= ?
		GROUP BY w.category
		ORDER BY total_runs DESC
	`

	rows, err := s.db.QueryContext(ctx, q, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []okaon.CategoryAggregate
	for rows.Next() {
		var c okaon.CategoryAggregate
		if err := rows.Scan(
			&c.Category, &c.TotalRuns, &c.UniqueModels, &c.AvgReward01,
		); err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, rows.Err()
}
