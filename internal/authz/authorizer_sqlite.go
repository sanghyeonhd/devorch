package authz

import (
	"context"
	"time"

	"devorch/internal/storage/sqlite"
)

type SessionLookup interface {
	FindByBearer(ctx context.Context, bearer string) (*sqlite.AuthSession, error)
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}

type SQLiteAuthorizer struct {
	Sessions SessionLookup
	Clock    func() time.Time
}

func NewSQLiteAuthorizer(s SessionLookup) *SQLiteAuthorizer {
	return &SQLiteAuthorizer{
		Sessions: s,
		Clock:    time.Now,
	}
}

func (a *SQLiteAuthorizer) Authenticate(ctx context.Context, bearer string) (*Principal, error) {
	if bearer == "" || a.Sessions == nil {
		return nil, ErrUnauthorized
	}

	now := a.Clock()
	// opportunistic cleanup (cheap)
	_, _ = a.Sessions.DeleteExpired(ctx, now)

	sess, err := a.Sessions.FindByBearer(ctx, bearer)
	if err != nil || sess == nil {
		return nil, ErrUnauthorized
	}
	if sess.ExpiresAt.Before(now) {
		return nil, ErrExpired
	}

	return &Principal{
		UserID: sess.UserID,
		Email:  sess.Email,
	}, nil
}

func (a *SQLiteAuthorizer) CanAccessWorkspace(ctx context.Context, p *Principal, workspace string) (bool, error) {
	// MVP 정책:
	// - bearer 토큰이 특정 workspace로 발급된 세션이면 접근 허용
	// - 실제 role/tenancy 기반은 이후 단계에서 확장
	// 여기서는 Authenticate 단계에서 workspace를 알 수 없으므로,
	// Stream/API에서 workspace별로 "workspace 바인딩 토큰"을 쓰는 구조를 권장.
	// 다만 현재 미들웨어 구조에서는 bearer만 주어지므로,
	// CanAccessWorkspace는 "true"로 두고, 라우트에서 workspace 바인딩을 강제하려면
	// 별도의 헤더(X-Workspace-Token) 분리 권장.
	//
	// 9단계에서는 간단히 "세션이 존재하면 허용"으로 둡니다.
	if p == nil {
		return false, ErrUnauthorized
	}
	if workspace == "" {
		return false, nil
	}
	return true, nil
}
