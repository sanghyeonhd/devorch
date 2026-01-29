package sqlite

import (
	"context"
	"time"
)

// Vacuum performs database maintenance
func (s *Store) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "VACUUM")
	return err
}

// Analyze updates query planner statistics
func (s *Store) Analyze(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "ANALYZE")
	return err
}

// PruneOldData removes data older than maxAge
func (s *Store) PruneOldData(ctx context.Context, maxAge time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-maxAge).Unix()

	var total int64

	// Delete old rewards
	res, err := s.db.ExecContext(ctx, "DELETE FROM okaon_rewards WHERE created_at < ?", cutoff)
	if err != nil {
		return total, err
	}
	n, _ := res.RowsAffected()
	total += n

	// Delete old qualities
	res, err = s.db.ExecContext(ctx, "DELETE FROM okaon_qualities WHERE evaluated_at < ?", cutoff)
	if err != nil {
		return total, err
	}
	n, _ = res.RowsAffected()
	total += n

	// Delete old runs
	res, err = s.db.ExecContext(ctx, "DELETE FROM okaon_runs WHERE created_at < ?", cutoff)
	if err != nil {
		return total, err
	}
	n, _ = res.RowsAffected()
	total += n

	// Delete old works
	res, err = s.db.ExecContext(ctx, "DELETE FROM okaon_works WHERE created_at < ?", cutoff)
	if err != nil {
		return total, err
	}
	n, _ = res.RowsAffected()
	total += n

	return total, nil
}
