package router

import (
	"context"
	"database/sql"
	"devorch/internal/router/scope"
	"time"
)

// Policy: router_policy 테이블 행
type Policy struct {
	ID int64

	ScopeType string
	ScopeID   string

	OS       string
	Arch     string
	Scenario string

	Provider string
	Model    string

	Weight       float64
	SuccessRate  float64
	AvgLatencyMs float64
	AvgCost      float64
	SampleCount  int

	UpdatedAt time.Time
}

type PolicyStore struct {
	db *sql.DB
}

func NewPolicyStore(db *sql.DB) *PolicyStore {
	return &PolicyStore{db: db}
}

func (s *PolicyStore) Upsert(ctx context.Context, p Policy) error {
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = time.Now()
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO router_policy(
  scope_type, scope_id,
  os, arch, scenario,
  provider, model,
  weight, success_rate, avg_latency_ms, avg_cost, sample_count,
  updated_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(os, arch, scenario, provider, model) DO UPDATE SET
  scope_type = excluded.scope_type,
  scope_id = excluded.scope_id,
  weight = excluded.weight,
  success_rate = excluded.success_rate,
  avg_latency_ms = excluded.avg_latency_ms,
  avg_cost = excluded.avg_cost,
  sample_count = excluded.sample_count,
  updated_at = excluded.updated_at
`,
		p.ScopeType, p.ScopeID,
		p.OS, p.Arch, p.Scenario,
		p.Provider, p.Model,
		p.Weight, p.SuccessRate, p.AvgLatencyMs, p.AvgCost, p.SampleCount,
		p.UpdatedAt,
	)
	return err
}

func (s *PolicyStore) Query(ctx context.Context, scopeType, scopeID, os, arch, scenario string) ([]Policy, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, scope_type, scope_id, os, arch, scenario, provider, model,
       weight, success_rate, avg_latency_ms, avg_cost, sample_count, updated_at
  FROM router_policy
 WHERE scope_type = ? AND scope_id = ?
   AND os = ? AND arch = ? AND scenario = ?
 ORDER BY weight DESC
`, scopeType, scopeID, os, arch, scenario)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Policy
	for rows.Next() {
		var p Policy
		var ts int64
		if err := rows.Scan(
			&p.ID, &p.ScopeType, &p.ScopeID, &p.OS, &p.Arch, &p.Scenario,
			&p.Provider, &p.Model,
			&p.Weight, &p.SuccessRate, &p.AvgLatencyMs, &p.AvgCost, &p.SampleCount, &ts,
		); err != nil {
			return nil, err
		}
		p.UpdatedAt = time.UnixMilli(ts)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *PolicyStore) All(ctx context.Context) ([]Policy, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, scope_type, scope_id, os, arch, scenario, provider, model,
       weight, success_rate, avg_latency_ms, avg_cost, sample_count, updated_at
  FROM router_policy
 ORDER BY scope_type, scope_id, os, arch, scenario, weight DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Policy
	for rows.Next() {
		var p Policy
		var ts int64
		if err := rows.Scan(
			&p.ID, &p.ScopeType, &p.ScopeID, &p.OS, &p.Arch, &p.Scenario,
			&p.Provider, &p.Model,
			&p.Weight, &p.SuccessRate, &p.AvgLatencyMs, &p.AvgCost, &p.SampleCount, &ts,
		); err != nil {
			return nil, err
		}
		p.UpdatedAt = time.UnixMilli(ts)
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeleteScopeScenario: 특정 scope/os/arch/scenario의 정책을 모두 삭제
func (s *PolicyStore) DeleteScopeScenario(ctx context.Context, key scope.Key, os, arch, scenario string) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM router_policy
WHERE scope_type = ? AND scope_id = ?
  AND os = ? AND arch = ? AND scenario = ?
`, string(key.Type), key.ID, os, arch, scenario)
	return err
}
