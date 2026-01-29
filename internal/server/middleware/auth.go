package middleware

import (
	"context"
	"net/http"
	"strings"

	"devorch/internal/authz"
)

type key int

const principalKey key = 1

func WithAuthz(a authz.Authorizer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			http.Error(w, "missing authorization", http.StatusUnauthorized)
			return
		}
		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "invalid authorization", http.StatusUnauthorized)
			return
		}
		p, err := a.Authenticate(r.Context(), parts[1])
		if err != nil || p == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), principalKey, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func PrincipalFromRequest(r *http.Request) (*authz.Principal, bool) {
	p, ok := r.Context().Value(principalKey).(*authz.Principal)
	return p, ok
}
