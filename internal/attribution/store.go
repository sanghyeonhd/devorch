package attribution

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

func (st *Store) Insert(ctx context.Context, ev AttributionEvent) error {
	if ev.ID == "" {
		ev.ID = id.NewULID()
	}
	_, err := st.db.ExecContext(ctx, `
INSERT INTO attribution_events
(id, bench_run_id, env_signature_id,
 model, provider,
 prompt_hash, prompt_version,
 toolchain_hash, runtime_flags)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		ev.ID, ev.BenchRunID, ev.EnvSignatureID,
		ev.Model, ev.Provider,
		ev.PromptHash, ev.PromptVersion,
		ev.ToolchainHash, ev.RuntimeFlags,
	)
	return err
}
