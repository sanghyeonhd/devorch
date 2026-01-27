package middleware

import (
	"context"
	"net/http"

	"devorch/internal/tenancy"
)

type ctxKey string

const (
	ctxWorkspaceKey ctxKey = "workspace"
)

func WithWorkspace(next http.Handler, ts *tenancy.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws := ts.Current(r.Context())
		if v := r.Header.Get("X-DevOrch-Workspace"); v != "" {
			// MVP: 알려진 목록 검증은 후속 단계(6~)
			ws.ID = v
		}
		ctx := context.WithValue(r.Context(), ctxWorkspaceKey, ws)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func WorkspaceFrom(ctx context.Context) tenancy.Workspace {
	if v := ctx.Value(ctxWorkspaceKey); v != nil {
		if ws, ok := v.(tenancy.Workspace); ok {
			return ws
		}
	}
	return tenancy.DefaultWorkspace()
}
