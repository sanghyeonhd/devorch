package auth

import (
	"context"
	"database/sql"
	"time"
)

type SQLiteStore struct {
	DB *sql.DB
}

func NewSQLiteStore(db *sql.DB) *SQLiteStore { return &SQLiteStore{DB: db} }

func (s *SQLiteStore) Get(ctx context.Context, workspaceID string, provider Provider) (Token, bool, error) {
	row := s.DB.QueryRowContext(ctx, `
SELECT access_token, refresh_token, token_type, expiry_unix, scope, id_token
FROM auth_tokens WHERE workspace_id=? AND provider=?`,
		workspaceID, string(provider),
	)
	var at, rt, tt, sc, idt sql.NullString
	var exp sql.NullInt64
	if err := row.Scan(&at, &rt, &tt, &exp, &sc, &idt); err != nil {
		if err == sql.ErrNoRows {
			return Token{}, false, nil
		}
		return Token{}, false, err
	}
	var expiry time.Time
	if exp.Valid && exp.Int64 > 0 {
		expiry = time.Unix(exp.Int64, 0)
	}
	return Token{
		AccessToken:  at.String,
		RefreshToken: rt.String,
		TokenType:    tt.String,
		Expiry:       expiry,
		Scope:        sc.String,
		IDToken:      idt.String,
	}, true, nil
}

func (s *SQLiteStore) Put(ctx context.Context, workspaceID string, provider Provider, tok Token) error {
	var exp int64
	if !tok.Expiry.IsZero() {
		exp = tok.Expiry.Unix()
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO auth_tokens(workspace_id, provider, access_token, refresh_token, token_type, expiry_unix, scope, id_token)
VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(workspace_id, provider) DO UPDATE SET
  access_token=excluded.access_token,
  refresh_token=excluded.refresh_token,
  token_type=excluded.token_type,
  expiry_unix=excluded.expiry_unix,
  scope=excluded.scope,
  id_token=excluded.id_token
`,
		workspaceID, string(provider),
		tok.AccessToken, tok.RefreshToken, tok.TokenType, exp, tok.Scope, tok.IDToken,
	)
	return err
}

func (s *SQLiteStore) Delete(ctx context.Context, workspaceID string, provider Provider) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM auth_tokens WHERE workspace_id=? AND provider=?`,
		workspaceID, string(provider),
	)
	return err
}
