package sqlite

import (
	"context"
)

// ArmStatRow는 arm stats 조회 결과
type ArmStatRow struct {
	Fingerprint  string  `json:"fingerprint"`
	Category     string  `json:"category"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	CountRuns    int64   `json:"count_runs"`
	MeanReward01 float64 `json:"mean_reward01"`
	UpdatedTsUTC int64   `json:"updated_ts_utc"`
}

// QueryArmStats는 지정된 fingerprint/category의 arm 통계를 조회합니다
func (s *Store) QueryArmStats(ctx context.Context, fingerprint, category string, limit int) ([]ArmStatRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT fingerprint, category, provider, model, count_runs, mean_reward_01, updated_at
FROM okaon_arm_stats
WHERE fingerprint=? AND category=?
ORDER BY mean_reward_01 DESC, count_runs DESC
LIMIT ?
`, fingerprint, category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ArmStatRow, 0, limit)
	for rows.Next() {
		var r ArmStatRow
		if err := rows.Scan(&r.Fingerprint, &r.Category, &r.Provider, &r.Model, &r.CountRuns, &r.MeanReward01, &r.UpdatedTsUTC); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
