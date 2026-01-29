// Package sqlite provides OkAON statistics queries.
// Phase 32: envfp × category best arm query
package sqlite

import (
	"context"
	"database/sql"

	"devorch/internal/router"
)

// BestArmByEnvCategory returns the best performing arms for a given environment and category.
// Results are ordered by avg_reward DESC, avg_quality DESC, avg_latency ASC.
func (s *Store) BestArmByEnvCategory(ctx context.Context, envfp, category string, limit int) ([]router.BestArmRow, error) {
	if limit <= 0 {
		limit = 5
	}

	q := `
SELECT
  provider,
  model,
  AVG(COALESCE(reward_01, 0)) AS avg_reward,
  AVG(COALESCE(quality_score, 0)) AS avg_quality,
  AVG(COALESCE(latency_ms, 0)) AS avg_latency,
  COUNT(1) AS n
FROM okaon_runs
WHERE fingerprint = ?
  AND category = ?
  AND finished_at IS NOT NULL
GROUP BY provider, model
HAVING n >= 3
ORDER BY avg_reward DESC, avg_quality DESC, avg_latency ASC
LIMIT ?;
`

	rows, err := s.db.QueryContext(ctx, q, envfp, category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []router.BestArmRow
	for rows.Next() {
		var r router.BestArmRow
		if err := rows.Scan(&r.Provider, &r.Model, &r.AvgReward, &r.AvgQuality, &r.AvgLatencyMs, &r.N); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BestArmByFingerprint returns the best performing arms for a given fingerprint across all categories.
func (s *Store) BestArmByFingerprint(ctx context.Context, fingerprint string, limit int) ([]router.BestArmRow, error) {
	if limit <= 0 {
		limit = 10
	}

	q := `
SELECT
  provider,
  model,
  AVG(COALESCE(reward_01, 0)) AS avg_reward,
  AVG(COALESCE(quality_score, 0)) AS avg_quality,
  AVG(COALESCE(latency_ms, 0)) AS avg_latency,
  COUNT(1) AS n
FROM okaon_runs
WHERE fingerprint = ?
  AND finished_at IS NOT NULL
GROUP BY provider, model
HAVING n >= 3
ORDER BY avg_reward DESC, avg_quality DESC, avg_latency ASC
LIMIT ?;
`

	rows, err := s.db.QueryContext(ctx, q, fingerprint, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []router.BestArmRow
	for rows.Next() {
		var r router.BestArmRow
		if err := rows.Scan(&r.Provider, &r.Model, &r.AvgReward, &r.AvgQuality, &r.AvgLatencyMs, &r.N); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// TopModelsGlobal returns the top performing models across all environments.
func (s *Store) TopModelsGlobal(ctx context.Context, limit int) ([]router.BestArmRow, error) {
	if limit <= 0 {
		limit = 10
	}

	q := `
SELECT
  provider,
  model,
  AVG(COALESCE(reward_01, 0)) AS avg_reward,
  AVG(COALESCE(quality_score, 0)) AS avg_quality,
  AVG(COALESCE(latency_ms, 0)) AS avg_latency,
  COUNT(1) AS n
FROM okaon_runs
WHERE finished_at IS NOT NULL
GROUP BY provider, model
HAVING n >= 5
ORDER BY avg_reward DESC, avg_quality DESC, avg_latency ASC
LIMIT ?;
`

	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []router.BestArmRow
	for rows.Next() {
		var r router.BestArmRow
		if err := rows.Scan(&r.Provider, &r.Model, &r.AvgReward, &r.AvgQuality, &r.AvgLatencyMs, &r.N); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Ensure Store implements the BestArmLoader interface
var _ router.BestArmLoader = (*Store)(nil)

// Helper for nullable float64
func nullFloat64(f sql.NullFloat64) float64 {
	if f.Valid {
		return f.Float64
	}
	return 0
}
