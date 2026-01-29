package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"
)

type AuthSession struct {
	ID              string
	UserID          string
	Workspace       string
	AccessTokenHash string
	ExpiresAt       time.Time
	Email           string
	Provider        string
	CreatedAt       time.Time
}

type AuthSessionStore struct {
	DB *sql.DB
}

func HashBearerToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *AuthSessionStore) Insert(ctx context.Context, a AuthSession) (string, error) {
	_, err := s.DB.ExecContext(ctx, `
	  INSERT INTO auth_sessions (
	    id, user_id, workspace, access_token_hash, expires_at,
	    email, provider, created_at
	  ) VALUES (?,?,?,?,?,?,?,?)
	`, a.ID, a.UserID, a.Workspace, a.AccessTokenHash, a.ExpiresAt.UnixMilli(),
		a.Email, a.Provider, a.CreatedAt.UnixMilli(),
	)
	return a.ID, err
}

func (s *AuthSessionStore) FindByBearer(ctx context.Context, bearer string) (*AuthSession, error) {
	h := HashBearerToken(bearer)
	row := s.DB.QueryRowContext(ctx, `
	  SELECT id, user_id, workspace, access_token_hash, expires_at, email, provider, created_at
	  FROM auth_sessions
	  WHERE access_token_hash=?
	  LIMIT 1
	`, h)

	var a AuthSession
	var expMs, createdMs int64
	if err := row.Scan(&a.ID, &a.UserID, &a.Workspace, &a.AccessTokenHash, &expMs, &a.Email, &a.Provider, &createdMs); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	a.ExpiresAt = time.UnixMilli(expMs)
	a.CreatedAt = time.UnixMilli(createdMs)
	return &a, nil
}

func (s *AuthSessionStore) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM auth_sessions WHERE expires_at < ?`, now.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
