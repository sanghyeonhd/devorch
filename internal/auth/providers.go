package auth

import (
	"context"
	"errors"
	"net/url"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type ProviderConfig struct {
	ClientID      string
	ClientSecret  string
	AuthURL       string // OIDC custom
	TokenURL      string // OIDC custom
	IssuerURL     string // OIDC discovery base URL
	DeviceAuthURL string // Device flow endpoint
	Scopes        []string
	Audience      string // for some providers
}

type Manager struct {
	Store     TokenStore
	State     *StateStore
	BaseURL   string // daemon public base, e.g. http://127.0.0.1:7777
	Workspace func() string
	Configs   map[Provider]ProviderConfig
}

func (m *Manager) oauthConfig(p Provider) (*oauth2.Config, error) {
	c, ok := m.Configs[p]
	if !ok {
		return nil, errors.New("provider not configured")
	}
	redirect := m.BaseURL + "/oauth/callback/" + url.PathEscape(string(p))

	switch p {
	case ProviderGitHub:
		return &oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			Endpoint:     github.Endpoint,
			RedirectURL:  redirect,
			Scopes:       c.Scopes,
		}, nil
	case ProviderGoogle:
		return &oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			Endpoint:     google.Endpoint,
			RedirectURL:  redirect,
			Scopes:       c.Scopes,
		}, nil
	case ProviderOIDC:
		if c.AuthURL == "" || c.TokenURL == "" {
			return nil, errors.New("custom oidc missing endpoints")
		}
		return &oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  c.AuthURL,
				TokenURL: c.TokenURL,
			},
			RedirectURL: redirect,
			Scopes:      c.Scopes,
		}, nil
	default:
		return nil, errors.New("provider not implemented")
	}
}

func (m *Manager) Begin(ctx context.Context, p Provider) (authURL string, state string, err error) {
	pkce, err := NewPKCE()
	if err != nil {
		return "", "", err
	}
	state, err = randURLSafe(24)
	if err != nil {
		return "", "", err
	}
	conf, err := m.oauthConfig(p)
	if err != nil {
		return "", "", err
	}

	ws := m.Workspace()
	m.State.Put(state, Pending{
		WorkspaceID: ws,
		Provider:    p,
		PKCE:        pkce,
		CreatedAt:   time.Now(),
	})

	u := conf.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", pkce.Challenge),
		oauth2.SetAuthURLParam("code_challenge_method", pkce.Method),
	)
	return u, state, nil
}

func (m *Manager) Complete(ctx context.Context, p Provider, code string, state string) error {
	pending, ok := m.State.Get(state)
	if !ok {
		return errors.New("invalid/expired oauth state")
	}
	if pending.Provider != p {
		return errors.New("provider mismatch")
	}
	conf, err := m.oauthConfig(p)
	if err != nil {
		return err
	}

	tok, err := conf.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", pending.PKCE.Verifier),
	)
	if err != nil {
		return err
	}

	at := Token{
		AccessToken:  tok.AccessToken,
		TokenType:    tok.TokenType,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
	}

	// scope/id_token 등은 필요하면 tok.Extra에서 추출
	if v, ok := tok.Extra("scope").(string); ok {
		at.Scope = v
	}
	if v, ok := tok.Extra("id_token").(string); ok {
		at.IDToken = v
	}

	return m.Store.Put(ctx, pending.WorkspaceID, p, at)
}

func (m *Manager) Status(ctx context.Context, p Provider) (Token, bool, error) {
	ws := m.Workspace()
	return m.Store.Get(ctx, ws, p)
}

func (m *Manager) Logout(ctx context.Context, p Provider) error {
	ws := m.Workspace()
	return m.Store.Delete(ctx, ws, p)
}
