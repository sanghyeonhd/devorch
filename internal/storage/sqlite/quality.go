package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/id"
)

type QualityStore struct {
	DB *sql.DB
}

type QualityRun struct {
	ID        string
	Workspace string
	SessionID string
	TaskID    string
	Provider  string
	Model     string
	CreatedAt time.Time
	Success   bool
	Score     float64
	LatencyMs int64
	TotalTok  int
	Error     string
	Notes     string
}

func (s *QualityStore) Insert(ctx context.Context, r QualityRun) (string, error) {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	if r.ID == "" {
		r.ID = id.NewULID()
	}

	success := 0
	if r.Success {
		success = 1
	}

	_, err := s.DB.ExecContext(ctx, `
	  INSERT INTO quality_runs (
	    id, workspace, session_id, task_id, provider, model,
	    created_at, success, score, latency_ms, total_tokens, error, notes
	  ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
	`, r.ID, r.Workspace, r.SessionID, r.TaskID, r.Provider, r.Model,
		r.CreatedAt.UnixMilli(), success, r.Score, r.LatencyMs, r.TotalTok, r.Error, r.Notes)
	return r.ID, err
}

type QualityAgg struct {
	Provider string
	Model    string
	Count    int
	AvgScore float64
	FailRate float64
}

func (s *QualityStore) RecentAgg(ctx context.Context, workspace string, limit int) ([]QualityAgg, error) {
	rows, err := s.DB.QueryContext(ctx, `
	  SELECT provider, model,
	         COUNT(*) as cnt,
	         AVG(score) as avg_score,
	         AVG(CASE WHEN success=1 THEN 0 ELSE 1 END) as fail_rate
	  FROM quality_runs
	  WHERE workspace=?
	  GROUP BY provider, model
	  ORDER BY cnt DESC
	  LIMIT ?
	`, workspace, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []QualityAgg{}
	for rows.Next() {
		var a QualityAgg
		if err := rows.Scan(&a.Provider, &a.Model, &a.Count, &a.AvgScore, &a.FailRate); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}
