package sqlite

import (
	"context"
	"time"

	"devorch/internal/okaon"
)

// ApplyRetention applies retention policy to the database
func (s *Store) ApplyRetention(ctx context.Context, policy okaon.RetentionPolicy) error {
	// Prune general data
	if policy.MaxAge > 0 {
		_, err := s.PruneOldData(ctx, policy.MaxAge)
		if err != nil {
			return err
		}
	}

	// Prune arm stats
	if policy.ArmStatsMaxAge > 0 {
		cutoff := time.Now().UTC().Add(-policy.ArmStatsMaxAge).Unix()
		_, err := s.db.ExecContext(ctx,
			"DELETE FROM okaon_arm_stats WHERE updated_at < ?",
			cutoff,
		)
		if err != nil {
			return err
		}
	}

	return nil
}
