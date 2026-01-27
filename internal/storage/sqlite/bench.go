package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/id"
	"devorch/internal/provider"
)

type BenchStore struct {
	DB *sql.DB
}

func (s *BenchStore) Insert(ctx context.Context, br provider.BenchResult) (string, error) {
	if br.StartedAt.IsZero() {
		br.StartedAt = time.Now()
	}
	if br.EndedAt.IsZero() {
		br.EndedAt = time.Now()
	}
	if br.LatencyMs == 0 {
		br.LatencyMs = br.EndedAt.Sub(br.StartedAt).Milliseconds()
	}
	rid := id.NewULID()

	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO bench_runs (
		  id, workspace, session_id, task_id, provider, model,
		  started_at, ended_at, latency_ms,
		  prompt_tokens, completion_tokens, total_tokens, error
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
	`, rid, br.Workspace, br.SessionID, br.TaskID, br.Provider, br.Model,
		br.StartedAt.UnixMilli(), br.EndedAt.UnixMilli(), br.LatencyMs,
		br.PromptTok, br.CompletionTok, br.TotalTok, br.Error,
	)
	return rid, err
}

type BenchAgg struct {
	Provider     string
	Model        string
	Count        int
	P50LatencyMs int64
	ErrorRate    float64
}

func (s *BenchStore) RecentAgg(ctx context.Context, workspace string, limit int) ([]BenchAgg, error) {
	// MVP: 아주 단순 집계(정교한 p50은 7단계에서)
	rows, err := s.DB.QueryContext(ctx, `
	  SELECT provider, model,
	         COUNT(*) as cnt,
	         AVG(latency_ms) as avg_latency,
	         AVG(CASE WHEN error IS NULL OR error='' THEN 0 ELSE 1 END) as err_rate
	  FROM bench_runs
	  WHERE workspace=?
	  GROUP BY provider, model
	  ORDER BY cnt DESC
	  LIMIT ?
	`, workspace, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []BenchAgg{}
	for rows.Next() {
		var a BenchAgg
		var avg float64
		if err := rows.Scan(&a.Provider, &a.Model, &a.Count, &avg, &a.ErrorRate); err != nil {
			return nil, err
		}
		a.P50LatencyMs = int64(avg)
		out = append(out, a)
	}
	return out, nil
}
