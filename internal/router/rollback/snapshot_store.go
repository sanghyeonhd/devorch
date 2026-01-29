package rollback

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"devorch/internal/id"
	"devorch/internal/router"
	"devorch/internal/router/scope"
)

type Snapshot struct {
	ID string

	ScopeType string
	ScopeID   string
	OS        string
	Arch      string
	Scenario  string

	PoliciesJSON string
	Reason       string

	CreatedAt time.Time
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (st *Store) SaveSnapshot(ctx context.Context, ps *router.PolicyStore, key scope.Key, os, arch, scenario, reason string) (string, error) {
	pols, err := ps.Query(ctx, string(key.Type), key.ID, os, arch, scenario)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(pols)
	if err != nil {
		return "", err
	}

	sid := id.NewULID()
	_, err = st.db.ExecContext(ctx, `
INSERT INTO router_policy_snapshots
(id, scope_type, scope_id, os, arch, scenario, policies_json, reason)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, sid, string(key.Type), key.ID, os, arch, scenario, string(b), reason)
	if err != nil {
		return "", err
	}
	return sid, nil
}

func (st *Store) LoadSnapshot(ctx context.Context, snapshotID string) (Snapshot, error) {
	row := st.db.QueryRowContext(ctx, `
SELECT id, scope_type, scope_id, os, arch, scenario, policies_json, reason, created_at
FROM router_policy_snapshots
WHERE id = ?
`, snapshotID)

	var s Snapshot
	if err := row.Scan(
		&s.ID, &s.ScopeType, &s.ScopeID, &s.OS, &s.Arch, &s.Scenario,
		&s.PoliciesJSON, &s.Reason, &s.CreatedAt,
	); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}
