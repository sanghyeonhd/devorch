package auth

import (
	"context"
	"time"

	"devorch/internal/tenancy"
)

type ManagerWithWorkspace struct {
	*Manager
	tenancy *tenancy.Store
}

func NewManagerWithWorkspace(m *Manager, ts *tenancy.Store) *ManagerWithWorkspace {
	return &ManagerWithWorkspace{Manager: m, tenancy: ts}
}

func (mw *ManagerWithWorkspace) EnsureFreshForWorkspace(ctx context.Context, p Provider, skew time.Duration) (Token, bool, error) {
	ws := tenancy.DefaultWorkspace()
	if mw.tenancy != nil {
		ws = mw.tenancy.Current(ctx)
	}
	// MVP: 단일 워크스페이스라 workspace 차별화 안 함
	// 추후 workspace-scoped token store 연동 시 확장
	_ = ws
	return mw.Manager.EnsureFresh(ctx, p, skew)
}
