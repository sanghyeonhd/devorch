package okaon

import (
	"context"
	"database/sql"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) InsertRun(ctx context.Context, r Run) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO okaon_runs(
  run_id, created_at,
  policy_key,
  workspace_id, user_id, project_id,
  agent_type, category, task_type,
  provider, model, endpoint,
  latency_ms, ok, quality,
  prompt_tokens, completion_tokens, total_tokens, cost_micro_usd,
  trace_id, error_code, error_message
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.RunID, r.CreatedAt,
		r.PolicyKey,
		r.WorkspaceID, r.UserID, r.ProjectID,
		r.AgentType, r.Category, r.TaskType,
		r.Provider, r.Model, r.Endpoint,
		r.LatencyMS, r.OK, r.Quality,
		r.PromptTokens, r.CompletionTokens, r.TotalTokens, r.CostMicroUSD,
		nullable(r.TraceID), nullable(r.ErrorCode), nullable(r.ErrorMessage),
	)
	return err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Aggregate recent quality/latency for (policy_key, provider, model).
type Aggregate struct {
	Count     int
	AvgQual   float64
	AvgLatMS  float64
	SuccessRt float64
}

func (s *Store) AggregateRecent(ctx context.Context, policyKey, providerName, model string, limit int) (Aggregate, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT ok, quality, latency_ms
FROM okaon_runs
WHERE policy_key = ? AND provider = ? AND model = ?
ORDER BY created_at DESC
LIMIT ?`, policyKey, providerName, model, limit)
	if err != nil {
		return Aggregate{}, err
	}
	defer rows.Close()

	var (
		n     int
		sumQ  float64
		sumL  float64
		sumOK float64
	)
	for rows.Next() {
		var ok int
		var q float64
		var lat int
		if err := rows.Scan(&ok, &q, &lat); err != nil {
			return Aggregate{}, err
		}
		n++
		sumQ += q
		sumL += float64(lat)
		sumOK += float64(ok)
	}
	if n == 0 {
		return Aggregate{Count: 0, AvgQual: 0.0, AvgLatMS: 0.0, SuccessRt: 0.0}, nil
	}
	return Aggregate{
		Count:     n,
		AvgQual:   sumQ / float64(n),
		AvgLatMS:  sumL / float64(n),
		SuccessRt: sumOK / float64(n),
	}, nil
}
