package provider

import (
	"context"
	"net/http"
	"strings"
	"time"

	"devorch/internal/auth"
)

type Auther interface {
	EnsureFresh(ctx context.Context, p auth.Provider, skew time.Duration) (auth.Token, bool, error)
}

type AuthInjector struct {
	Auth Auther
}

func (a *AuthInjector) Inject(ctx context.Context, req *http.Request, providerName string) error {
	if a.Auth == nil {
		return ErrUnauthorized
	}
	p := auth.Provider(providerName)
	tok, ok, err := a.Auth.EnsureFresh(ctx, p, 30*time.Second)
	if err != nil || !ok {
		return ErrUnauthorized
	}
	tt := tok.TokenType
	if tt == "" {
		tt = "Bearer"
	}
	req.Header.Set("Authorization", tt+" "+tok.AccessToken)
	if tok.Scope != "" {
		req.Header.Set("X-DevOrch-Token-Scope", strings.TrimSpace(tok.Scope))
	}
	return nil
}
