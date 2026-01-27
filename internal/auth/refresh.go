package auth

import (
	"context"
	"errors"
	"time"

	"golang.org/x/oauth2"
)

var ErrNoRefreshToken = errors.New("no refresh token")

func (m *Manager) EnsureFresh(ctx context.Context, p Provider, skew time.Duration) (Token, bool, error) {
	tok, ok, err := m.Status(ctx, p)
	if err != nil || !ok {
		return Token{}, false, err
	}
	if !tok.Expired(skew) {
		return tok, true, nil
	}
	if tok.RefreshToken == "" {
		return Token{}, false, ErrNoRefreshToken
	}

	conf, err := m.oauthConfig(p)
	if err != nil {
		return Token{}, false, err
	}

	src := conf.TokenSource(ctx, &oauth2.Token{
		AccessToken:  tok.AccessToken,
		TokenType:    tok.TokenType,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
	})

	nt, err := src.Token()
	if err != nil {
		return Token{}, false, err
	}

	newTok := Token{
		AccessToken:  nt.AccessToken,
		TokenType:    nt.TokenType,
		RefreshToken: nt.RefreshToken,
		Expiry:       nt.Expiry,
		Scope:        tok.Scope,
		IDToken:      tok.IDToken,
	}
	// refresh 응답이 refresh_token을 비워주는 경우 대비
	if newTok.RefreshToken == "" {
		newTok.RefreshToken = tok.RefreshToken
	}
	if err := m.Store.Put(ctx, m.Workspace(), p, newTok); err != nil {
		return Token{}, false, err
	}
	return newTok, true, nil
}
