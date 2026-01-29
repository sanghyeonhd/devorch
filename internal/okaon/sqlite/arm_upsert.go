package sqlite

import (
	"context"
	"math"
	"time"

	"devorch/internal/okaon"
)

// UpsertArmStats updates or inserts arm statistics
func (s *Store) UpsertArmStats(ctx context.Context, u okaon.ArmStatUpdate) error {
	now := time.Now().UTC().Unix()

	// SQLite UPSERT
	const q = `
		INSERT INTO okaon_arm_stats (
			fingerprint, category, provider, model,
			count_runs, sum_reward_01, sum_reward_sq, mean_reward_01, stddev_reward,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fingerprint, category, provider, model) DO UPDATE SET
			count_runs = count_runs + excluded.count_runs,
			sum_reward_01 = sum_reward_01 + excluded.sum_reward_01,
			sum_reward_sq = sum_reward_sq + excluded.sum_reward_sq,
			mean_reward_01 = (sum_reward_01 + excluded.sum_reward_01) / (count_runs + excluded.count_runs),
			stddev_reward = CASE 
				WHEN (count_runs + excluded.count_runs) < 2 THEN 0
				ELSE sqrt(
					((sum_reward_sq + excluded.sum_reward_sq) - 
					 ((sum_reward_01 + excluded.sum_reward_01) * (sum_reward_01 + excluded.sum_reward_01)) / 
					 (count_runs + excluded.count_runs)) / 
					((count_runs + excluded.count_runs) - 1)
				)
			END,
			updated_at = excluded.updated_at
	`

	// 초기 mean/stddev 계산
	var mean, stddev float64
	if u.DeltaCount > 0 {
		mean = u.DeltaSumReward / float64(u.DeltaCount)
		if u.DeltaCount > 1 {
			variance := (u.DeltaSumSq - (u.DeltaSumReward*u.DeltaSumReward)/float64(u.DeltaCount)) / float64(u.DeltaCount-1)
			if variance > 0 {
				stddev = math.Sqrt(variance)
			}
		}
	}

	_, err := s.db.ExecContext(ctx, q,
		u.Fingerprint, u.Category, u.Provider, u.Model,
		u.DeltaCount, u.DeltaSumReward, u.DeltaSumSq, mean, stddev,
		now,
	)
	return err
}
