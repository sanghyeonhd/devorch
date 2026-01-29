package drift

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/id"
)

type Event struct {
	ID string

	ScopeType string
	ScopeID   string
	OS        string
	Arch      string
	Scenario  string

	BaselineSamples int
	RecentSamples   int

	BaselineSuccess float64
	RecentSuccess   float64

	BaselineLatency float64
	RecentLatency   float64

	BaselineCost float64
	RecentCost   float64

	DriftScore float64
	Threshold  float64

	Action     string // "none" | "rollback"
	SnapshotID *string
	Note       *string

	CreatedAt time.Time
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (st *Store) Insert(ctx context.Context, e Event) error {
	if e.ID == "" {
		e.ID = id.NewULID()
	}
	_, err := st.db.ExecContext(ctx, `
INSERT INTO router_drift_events
(id, scope_type, scope_id, os, arch, scenario,
 baseline_window, recent_window,
 baseline_success, recent_success,
 baseline_latency_ms, recent_latency_ms,
 baseline_cost, recent_cost,
 drift_score, threshold,
 action, snapshot_id, note)
VALUES
(?, ?, ?, ?, ?, ?,
 ?, ?,
 ?, ?,
 ?, ?,
 ?, ?,
 ?, ?,
 ?, ?, ?)
`,
		e.ID, e.ScopeType, e.ScopeID, e.OS, e.Arch, e.Scenario,
		e.BaselineSamples, e.RecentSamples,
		e.BaselineSuccess, e.RecentSuccess,
		e.BaselineLatency, e.RecentLatency,
		e.BaselineCost, e.RecentCost,
		e.DriftScore, e.Threshold,
		e.Action, e.SnapshotID, e.Note,
	)
	return err
}
