package checks

import (
	"context"
	"time"

	"devorch/internal/auth"
)

type OAuthCheck struct {
	Manager *auth.Manager
}

func (c *OAuthCheck) Name() string { return "oauth" }

func (c *OAuthCheck) Check(ctx context.Context) (bool, string) {
	if c.Manager == nil {
		return true, "oauth disabled"
	}
	for p := range c.Manager.Configs {
		tok, ok, _ := c.Manager.EnsureFresh(ctx, p, 30*time.Second)
		if !ok {
			return false, string(p) + ": no token"
		}
		if tok.Expiry.Before(time.Now()) {
			return false, string(p) + ": token expired"
		}
	}
	return true, "oauth ok"
}
