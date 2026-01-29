package rollback

import (
	"context"
	"encoding/json"

	"devorch/internal/router"
	"devorch/internal/router/scope"
)

type Executor struct {
	Snapshots *Store
	Policies  *router.PolicyStore
}

func NewExecutor(ss *Store, ps *router.PolicyStore) *Executor {
	return &Executor{Snapshots: ss, Policies: ps}
}

// 스냅샷에 있는 정책 JSON을 복원
func (e *Executor) Rollback(ctx context.Context, snapshotID string) (scope.Key, string, string, string, error) {
	snap, err := e.Snapshots.LoadSnapshot(ctx, snapshotID)
	if err != nil {
		return scope.Key{}, "", "", "", err
	}

	var policies []router.Policy
	if err := json.Unmarshal([]byte(snap.PoliciesJSON), &policies); err != nil {
		return scope.Key{}, "", "", "", err
	}

	key := scope.Key{Type: scope.Type(snap.ScopeType), ID: snap.ScopeID}

	// 기존 scope/os/arch/scenario 정책을 삭제 후 복원
	if err := e.Policies.DeleteScopeScenario(ctx, key, snap.OS, snap.Arch, snap.Scenario); err != nil {
		return scope.Key{}, "", "", "", err
	}

	for _, p := range policies {
		p.ScopeType = snap.ScopeType
		p.ScopeID = snap.ScopeID
		p.OS, p.Arch, p.Scenario = snap.OS, snap.Arch, snap.Scenario

		if err := e.Policies.Upsert(ctx, p); err != nil {
			return scope.Key{}, "", "", "", err
		}
	}

	return key, snap.OS, snap.Arch, snap.Scenario, nil
}
