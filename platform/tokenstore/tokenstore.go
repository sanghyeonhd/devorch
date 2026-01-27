package tokenstore

import (
	"context"

	"devorch/internal/auth"
)

type SecureStore interface {
	Get(ctx context.Context, workspaceID string, provider auth.Provider) (auth.Token, bool, error)
	Put(ctx context.Context, workspaceID string, provider auth.Provider, tok auth.Token) error
	Delete(ctx context.Context, workspaceID string, provider auth.Provider) error
}

// Multi: secure가 실패하면 fallback으로
type Multi struct {
	Secure   SecureStore
	Fallback auth.TokenStore
}

func (m *Multi) Get(ctx context.Context, ws string, p auth.Provider) (auth.Token, bool, error) {
	if m.Secure != nil {
		if tok, ok, err := m.Secure.Get(ctx, ws, p); err == nil && ok {
			return tok, ok, nil
		}
	}
	return m.Fallback.Get(ctx, ws, p)
}
func (m *Multi) Put(ctx context.Context, ws string, p auth.Provider, t auth.Token) error {
	if m.Secure != nil {
		if err := m.Secure.Put(ctx, ws, p, t); err == nil {
			return nil
		}
	}
	return m.Fallback.Put(ctx, ws, p, t)
}
func (m *Multi) Delete(ctx context.Context, ws string, p auth.Provider) error {
	if m.Secure != nil {
		_ = m.Secure.Delete(ctx, ws, p)
	}
	return m.Fallback.Delete(ctx, ws, p)
}
