package auth

import "context"

type TokenStore interface {
	Get(ctx context.Context, workspaceID string, provider Provider) (Token, bool, error)
	Put(ctx context.Context, workspaceID string, provider Provider, tok Token) error
	Delete(ctx context.Context, workspaceID string, provider Provider) error
}
