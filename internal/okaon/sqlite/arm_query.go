package sqlite

import (
	"context"

	"devorch/internal/okaon"
)

// GetArmStats returns arm statistics for a specific key
func (s *Store) GetArmStats(ctx context.Context, fingerprint, category, provider, model string) (okaon.ArmStat, bool, error) {
	const q = `
		SELECT fingerprint, category, provider, model, count_runs, mean_reward_01, updated_at
		FROM okaon_arm_stats
		WHERE fingerprint = ? AND category = ? AND provider = ? AND model = ?
	`

	var stat okaon.ArmStat
	err := s.db.QueryRowContext(ctx, q, fingerprint, category, provider, model).Scan(
		&stat.Fingerprint, &stat.Category, &stat.Provider, &stat.Model,
		&stat.CountRuns, &stat.MeanReward01, &stat.UpdatedTsUTC,
	)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return stat, false, nil
		}
		return stat, false, err
	}
	return stat, true, nil
}
