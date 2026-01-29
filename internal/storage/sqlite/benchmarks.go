package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type BenchmarkRow struct {
	ID                 string
	CreatedAt          time.Time
	OS                 string
	Arch               string
	MachineFingerprint string

	Provider string
	Model    string

	Scenario     string
	InputTokens  int64
	OutputTokens int64

	LatencyMs int64
	Success   bool
	Error     string

	QualityScore *float64
	CostUSD      *float64
	Metadata     map[string]any
}

type BenchmarksStore struct {
	db *sql.DB
}

func NewBenchmarksStore(db *sql.DB) *BenchmarksStore {
	return &BenchmarksStore{db: db}
}

func (s *BenchmarksStore) Insert(ctx context.Context, r BenchmarkRow) error {
	if s.db == nil {
		return errors.New("sqlite: db is nil")
	}

	meta := ""
	if r.Metadata != nil {
		b, err := json.Marshal(r.Metadata)
		if err != nil {
			return err
		}
		meta = string(b)
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO benchmarks(
  id, created_at, os, arch, machine_fingerprint,
  provider, model,
  scenario, input_tokens, output_tokens,
  latency_ms, success, error,
  quality_score, cost_usd, metadata_json
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, r.ID, r.CreatedAt.UnixMilli(), r.OS, r.Arch, r.MachineFingerprint,
		r.Provider, r.Model,
		r.Scenario, r.InputTokens, r.OutputTokens,
		r.LatencyMs, boolToInt(r.Success), r.Error,
		r.QualityScore, r.CostUSD, meta)
	return err
}

// RecentByModelAll returns recent benchmarks without scenario filter
func (s *BenchmarksStore) RecentByModelAll(ctx context.Context, provider, model string, limit int) ([]BenchmarkRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, created_at, os, arch, machine_fingerprint,
       provider, model,
       scenario, input_tokens, output_tokens,
       latency_ms, success, error,
       quality_score, cost_usd, metadata_json
FROM benchmarks
WHERE provider = ? AND model = ?
ORDER BY created_at DESC
LIMIT ?
`, provider, model, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBenchmarks(rows)
}

// RecentByModel returns recent benchmarks filtered by scenario
func (s *BenchmarksStore) RecentByModel(ctx context.Context, provider, model, scenario string, limit int) ([]BenchmarkRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, created_at, os, arch, machine_fingerprint,
       provider, model,
       scenario, input_tokens, output_tokens,
       latency_ms, success, error,
       quality_score, cost_usd, metadata_json
FROM benchmarks
WHERE provider = ? AND model = ? AND scenario = ?
ORDER BY created_at DESC
LIMIT ?
`, provider, model, scenario, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBenchmarks(rows)
}

// RecentByModelAndMachine returns recent benchmarks for same machine
func (s *BenchmarksStore) RecentByModelAndMachine(ctx context.Context, provider, model, scenario, machineFP string, limit int) ([]BenchmarkRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, created_at, os, arch, machine_fingerprint,
       provider, model,
       scenario, input_tokens, output_tokens,
       latency_ms, success, error,
       quality_score, cost_usd, metadata_json
FROM benchmarks
WHERE provider = ? AND model = ? AND scenario = ? AND machine_fingerprint = ?
ORDER BY created_at DESC
LIMIT ?
`, provider, model, scenario, machineFP, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBenchmarks(rows)
}

func scanBenchmarks(rows *sql.Rows) ([]BenchmarkRow, error) {
	out := make([]BenchmarkRow, 0, 32)
	for rows.Next() {
		var (
			id, os, arch, fp, prov, mdl, scenario, errStr, metaStr string
			createdAtMs                                            int64
			inTok, outTok, latMs                                   int64
			successInt                                             int
			quality, cost                                          sql.NullFloat64
		)

		if scanErr := rows.Scan(&id, &createdAtMs, &os, &arch, &fp,
			&prov, &mdl,
			&scenario, &inTok, &outTok,
			&latMs, &successInt, &errStr,
			&quality, &cost, &metaStr,
		); scanErr != nil {
			return nil, scanErr
		}

		var meta map[string]any
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &meta)
		}

		var qPtr *float64
		if quality.Valid {
			v := quality.Float64
			qPtr = &v
		}
		var cPtr *float64
		if cost.Valid {
			v := cost.Float64
			cPtr = &v
		}

		out = append(out, BenchmarkRow{
			ID:                 id,
			CreatedAt:          time.UnixMilli(createdAtMs),
			OS:                 os,
			Arch:               arch,
			MachineFingerprint: fp,
			Provider:           prov,
			Model:              mdl,
			Scenario:           scenario,
			InputTokens:        inTok,
			OutputTokens:       outTok,
			LatencyMs:          latMs,
			Success:            successInt == 1,
			Error:              errStr,
			QualityScore:       qPtr,
			CostUSD:            cPtr,
			Metadata:           meta,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
