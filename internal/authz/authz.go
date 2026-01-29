package authz

import "context"

type Principal struct {
	UserID string
	Email  string
}

type Authorizer interface {
	Authenticate(ctx context.Context, bearer string) (*Principal, error)
	CanAccessWorkspace(ctx context.Context, p *Principal, workspace string) (bool, error)
}
