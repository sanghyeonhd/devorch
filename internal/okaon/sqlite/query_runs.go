package sqlite

import (
	"context"
)

// RunRow는 runs 조회 결과
type RunRow struct {
	RunID        string  `json:"run_id"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Success      bool    `json:"success"`
	LatencyMs    int64   `json:"latency_ms"`
	Quality      float64 `json:"quality"`
	CostMicroUSD int64   `json:"cost_micro_usd"`
	ErrorMsg     string  `json:"error_msg"`
	CreatedAt    string  `json:"created_at"`
}

// QueryRuns는 최근 runs를 조회합니다
func (s *Store) QueryRuns(ctx context.Context, fingerprint, category string, limit int) ([]RunRow, error) {
	if limit <= 0 {
		limit = 50
	}

	// okaon_runs 테이블에서 조회
	q := `
SELECT run_id, provider, model,
       ok as success,
       latency_ms, quality, cost_micro_usd, 
       COALESCE(error_message, ''),
       created_at
FROM okaon_runs
ORDER BY created_at DESC
LIMIT ?
`
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RunRow, 0, limit)
	for rows.Next() {
		var r RunRow
		var successInt int
		if err := rows.Scan(
			&r.RunID, &r.Provider, &r.Model,
			&successInt, &r.LatencyMs, &r.Quality, &r.CostMicroUSD,
			&r.ErrorMsg, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		r.Success = (successInt != 0)
		out = append(out, r)
	}
	return out, rows.Err()
}
