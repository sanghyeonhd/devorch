package experiment

import (
	"context"
	"database/sql"

	"devorch/internal/id"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (st *Store) CreateExperiment(ctx context.Context, e Experiment) error {
	if e.ID == "" {
		e.ID = id.NewULID()
	}
	_, err := st.db.ExecContext(ctx, `
INSERT INTO experiments (id, name, kind, status, strategy, traffic, seed)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, e.ID, e.Name, string(e.Kind), string(e.Status), string(e.Strategy), e.Traffic, e.Seed)
	return err
}

func (st *Store) UpsertVariant(ctx context.Context, v Variant) error {
	if v.ID == "" {
		v.ID = id.NewULID()
	}
	isWinner := 0
	if v.IsWinner {
		isWinner = 1
	}
	_, err := st.db.ExecContext(ctx, `
INSERT INTO experiment_variants (id, experiment_id, key, weight, payload_json, is_winner)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  weight=excluded.weight,
  payload_json=excluded.payload_json,
  is_winner=excluded.is_winner
`, v.ID, v.ExperimentID, v.Key, v.Weight, v.PayloadJSON, isWinner)
	return err
}

func (st *Store) GetRunningExperiments(ctx context.Context) ([]Experiment, error) {
	rows, err := st.db.QueryContext(ctx, `
SELECT id, name, kind, status, strategy, traffic, seed
FROM experiments
WHERE status = 'running'
ORDER BY created_at ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Experiment
	for rows.Next() {
		var e Experiment
		var kind, status, strategy string
		if err := rows.Scan(&e.ID, &e.Name, &kind, &status, &strategy, &e.Traffic, &e.Seed); err != nil {
			return nil, err
		}
		e.Kind = Kind(kind)
		e.Status = Status(status)
		e.Strategy = Strategy(strategy)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (st *Store) GetVariants(ctx context.Context, experimentID string) ([]Variant, error) {
	rows, err := st.db.QueryContext(ctx, `
SELECT id, experiment_id, key, weight, payload_json, is_winner
FROM experiment_variants
WHERE experiment_id = ?
ORDER BY key ASC
`, experimentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Variant
	for rows.Next() {
		var v Variant
		var isWinner int
		if err := rows.Scan(&v.ID, &v.ExperimentID, &v.Key, &v.Weight, &v.PayloadJSON, &isWinner); err != nil {
			return nil, err
		}
		v.IsWinner = isWinner == 1
		out = append(out, v)
	}
	return out, rows.Err()
}

func (st *Store) GetAssignment(ctx context.Context, experimentID, subject string) (Assignment, bool, error) {
	row := st.db.QueryRowContext(ctx, `
SELECT id, experiment_id, subject, variant_key
FROM experiment_assignments
WHERE experiment_id = ? AND subject = ?
`, experimentID, subject)

	var a Assignment
	err := row.Scan(&a.ID, &a.ExperimentID, &a.Subject, &a.VariantKey)
	if err == sql.ErrNoRows {
		return Assignment{}, false, nil
	}
	if err != nil {
		return Assignment{}, false, err
	}
	return a, true, nil
}

func (st *Store) InsertAssignment(ctx context.Context, experimentID, subject, variantKey string) (Assignment, error) {
	a := Assignment{
		ID:           id.NewULID(),
		ExperimentID: experimentID,
		Subject:      subject,
		VariantKey:   variantKey,
	}
	_, err := st.db.ExecContext(ctx, `
INSERT INTO experiment_assignments (id, experiment_id, subject, variant_key)
VALUES (?, ?, ?, ?)
ON CONFLICT(experiment_id, subject) DO NOTHING
`, a.ID, a.ExperimentID, a.Subject, a.VariantKey)
	return a, err
}

func (st *Store) InsertOutcome(ctx context.Context, o Outcome) error {
	if o.ID == "" {
		o.ID = id.NewULID()
	}
	success := 0
	if o.Success {
		success = 1
	}
	_, err := st.db.ExecContext(ctx, `
INSERT INTO experiment_outcomes
(id, experiment_id, variant_key, bench_run_id, session_id, message_id,
 success, latency_ms, cost_microusd, quality_score, tokens_in, tokens_out)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		o.ID, o.ExperimentID, o.VariantKey, nullIfEmpty(o.BenchRunID), nullIfEmpty(o.SessionID), nullIfEmpty(o.MessageID),
		success, o.LatencyMS, o.CostMicroUSD, o.QualityScore, o.TokensIn, o.TokensOut,
	)
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (st *Store) MarkWinner(ctx context.Context, experimentID, winnerKey, reason string) error {
	// reset winner flags
	if _, err := st.db.ExecContext(ctx, `
UPDATE experiment_variants SET is_winner = 0
WHERE experiment_id = ?
`, experimentID); err != nil {
		return err
	}

	// set winner
	if _, err := st.db.ExecContext(ctx, `
UPDATE experiment_variants SET is_winner = 1
WHERE experiment_id = ? AND key = ?
`, experimentID, winnerKey); err != nil {
		return err
	}

	// decision record
	_, err := st.db.ExecContext(ctx, `
INSERT INTO experiment_decisions (id, experiment_id, decided_variant_key, reason)
VALUES (?, ?, ?, ?)
`, id.NewULID(), experimentID, winnerKey, reason)
	return err
}

func (st *Store) StopExperiment(ctx context.Context, experimentID string, completed bool) error {
	status := "stopped"
	if completed {
		status = "completed"
	}
	_, err := st.db.ExecContext(ctx, `
UPDATE experiments SET status = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
`, status, experimentID)
	return err
}
