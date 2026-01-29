package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/id"
)

type ModelStats struct {
	ID        string
	Workspace string
	Provider  string
	Model     string

	Samples      int
	AvgScore     float64
	AvgLatencyMs float64
	FailRate     float64

	UpdatedAt time.Time
}

type ModelStatsStore struct {
	DB *sql.DB
}

func (s *ModelStatsStore) Upsert(ctx context.Context, ms ModelStats) error {
	// SQLite upsert
	_, err := s.DB.ExecContext(ctx, `
	  INSERT INTO model_stats (
	    id, workspace, provider, model,
	    samples, avg_score, avg_latency_ms, fail_rate, updated_at
	  ) VALUES (?,?,?,?,?,?,?,?,?)
	  ON CONFLICT(workspace, provider, model)
	  DO UPDATE SET
	    samples=excluded.samples,
	    avg_score=excluded.avg_score,
	    avg_latency_ms=excluded.avg_latency_ms,
	    fail_rate=excluded.fail_rate,
	    updated_at=excluded.updated_at
	`, ms.ID, ms.Workspace, ms.Provider, ms.Model,
		ms.Samples, ms.AvgScore, ms.AvgLatencyMs, ms.FailRate, ms.UpdatedAt.UnixMilli(),
	)
	return err
}

func (s *ModelStatsStore) Get(ctx context.Context, workspace, provider, model string) (*ModelStats, error) {
	row := s.DB.QueryRowContext(ctx, `
	  SELECT id, workspace, provider, model,
	         samples, avg_score, avg_latency_ms, fail_rate, updated_at
	  FROM model_stats
	  WHERE workspace=? AND provider=? AND model=?
	  LIMIT 1
	`, workspace, provider, model)

	var ms ModelStats
	var updatedMs int64
	if err := row.Scan(&ms.ID, &ms.Workspace, &ms.Provider, &ms.Model,
		&ms.Samples, &ms.AvgScore, &ms.AvgLatencyMs, &ms.FailRate, &updatedMs); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	ms.UpdatedAt = time.UnixMilli(updatedMs)
	return &ms, nil
}

func NewModelStats(workspace, provider, model string) ModelStats {
	return ModelStats{
		ID:        id.NewULID(),
		Workspace: workspace,
		Provider:  provider,
		Model:     model,
		UpdatedAt: time.Now(),
	}
}
