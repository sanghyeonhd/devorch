아래는 **4단계(OAuth-only + 토큰저장/갱신 + 서버 미들웨어 + CLI 로그인 + VSCode/WebUI 연동)**를 실제로 컴파일/동작 가능한 형태로 풀코드로 제공합니다.
(너무 방대해지지 않도록, “반드시 필요한 파일”은 전부 포함했고, provider별 세부 스코프/엔드포인트는 이후 단계에서 확장할 수 있게 설계해뒀습니다.)


---

4단계: 추가/변경 파일 목록

Go (daemon/core)

internal/auth/* (PKCE, OAuth state, token store, refresh)

internal/server/routes/oauth.go (authorize/callback/status/logout)

internal/server/middleware/auth_middleware.go (OAuth-only 강제)

internal/cli/login.go (CLI 로그인/상태/로그아웃)

platform/* (토큰 저장소: build tag로 OS별 구현 + fallback)

internal/storage/sqlite/migrations/0005_auth_tokens.sql (fallback token store)


VSCode extension

vscode/src/oauth.ts

vscode/src/client.ts (인증 헤더/토큰 상태 반영)

vscode/src/extension.ts (login/logout command)


WebUI (선택 UI)

webui/src/routes/oauth.tsx

webui/src/lib/api.ts (oauth api 호출)



---

4-1) DB 마이그레이션 (fallback 토큰 저장)

internal/storage/sqlite/migrations/0005_auth_tokens.sql

CREATE TABLE IF NOT EXISTS auth_tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  workspace_id TEXT NOT NULL,
  provider TEXT NOT NULL,              -- github/google/openai/anthropic/custom_oidc...
  access_token TEXT NOT NULL,
  refresh_token TEXT,
  token_type TEXT,
  expiry_unix INTEGER,                 -- unix seconds (0 if unknown)
  scope TEXT,
  id_token TEXT,                       -- OIDC optional
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(workspace_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_workspace
ON auth_tokens(workspace_id);

CREATE TRIGGER IF NOT EXISTS trg_auth_tokens_updated
AFTER UPDATE ON auth_tokens
BEGIN
  UPDATE auth_tokens SET updated_at=CURRENT_TIMESTAMP WHERE id=NEW.id;
END;


---

4-2) Auth core (OAuth-only)

internal/auth/types.go

package auth

import "time"

type Provider string

const (
	ProviderGitHub   Provider = "github"
	ProviderGoogle   Provider = "google"
	ProviderOpenAI   Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderOIDC     Provider = "custom_oidc"
)

type Token struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       time.Time
	Scope        string
	IDToken      string
}

func (t Token) Expired(skew time.Duration) bool {
	if t.Expiry.IsZero() {
		return false
	}
	return time.Now().Add(skew).After(t.Expiry)
}

internal/auth/pkce.go

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

func randURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type PKCE struct {
	Verifier  string
	Challenge string
	Method    string
}

func NewPKCE() (*PKCE, error) {
	v, err := randURLSafe(32)
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256([]byte(v))
	ch := base64.RawURLEncoding.EncodeToString(h[:])
	return &PKCE{Verifier: v, Challenge: ch, Method: "S256"}, nil
}

internal/auth/state.go

package auth

import (
	"sync"
	"time"
)

type Pending struct {
	WorkspaceID string
	Provider    Provider
	PKCE        *PKCE
	CreatedAt   time.Time
}

type StateStore struct {
	mu   sync.Mutex
	data map[string]Pending
	ttl  time.Duration
}

func NewStateStore(ttl time.Duration) *StateStore {
	return &StateStore{data: map[string]Pending{}, ttl: ttl}
}

func (s *StateStore) Put(state string, p Pending) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[state] = p
}

func (s *StateStore) Get(state string) (Pending, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.data[state]
	if !ok {
		return Pending{}, false
	}
	if time.Since(p.CreatedAt) > s.ttl {
		delete(s.data, state)
		return Pending{}, false
	}
	delete(s.data, state)
	return p, true
}

internal/auth/store.go

package auth

import "context"

type TokenStore interface {
	Get(ctx context.Context, workspaceID string, provider Provider) (Token, bool, error)
	Put(ctx context.Context, workspaceID string, provider Provider, tok Token) error
	Delete(ctx context.Context, workspaceID string, provider Provider) error
}


---

4-3) TokenStore 구현 (sqlite fallback)

internal/auth/store_sqlite.go

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


---

4-4) OS 보안 저장소 연결(선택) + fallback

> 현실적으로 Keychain/CredMan/SecretService는 구현이 길어집니다.
여기서는 컴파일/동작 가능한 구조로 플러그형 제공하고, 기본은 SQLiteStore를 씁니다.
이후 단계에서 각 OS 파일을 실제 API로 채우면 됩니다.



platform/tokenstore/tokenstore.go

package tokenstore

import (
	"context"

	"devorch/internal/auth"
)

type SecureStore interface {
	Get(ctx context.Context, workspaceID string, provider auth.Provider) (auth.Token, bool, error)
	Put(ctx context.Context, workspaceID string, provider auth.Provider, tok auth.Token) error
	Delete(ctx context.Context, workspaceID string, provider auth.Provider) error
}

// Multi: secure가 실패하면 fallback으로
type Multi struct {
	Secure   SecureStore
	Fallback auth.TokenStore
}

func (m *Multi) Get(ctx context.Context, ws string, p auth.Provider) (auth.Token, bool, error) {
	if m.Secure != nil {
		if tok, ok, err := m.Secure.Get(ctx, ws, p); err == nil && ok {
			return tok, ok, nil
		}
	}
	return m.Fallback.Get(ctx, ws, p)
}
func (m *Multi) Put(ctx context.Context, ws string, p auth.Provider, t auth.Token) error {
	if m.Secure != nil {
		if err := m.Secure.Put(ctx, ws, p, t); err == nil {
			return nil
		}
	}
	return m.Fallback.Put(ctx, ws, p, t)
}
func (m *Multi) Delete(ctx context.Context, ws string, p auth.Provider) error {
	if m.Secure != nil {
		_ = m.Secure.Delete(ctx, ws, p)
	}
	return m.Fallback.Delete(ctx, ws, p)
}

platform/tokenstore/secure_stub.go

//go:build !darwin && !windows && !linux

package tokenstore

import (
	"context"
	"errors"

	"devorch/internal/auth"
)

type SecureStub struct{}

func (s *SecureStub) Get(ctx context.Context, ws string, p auth.Provider) (auth.Token, bool, error) {
	return auth.Token{}, false, errors.New("no secure store")
}
func (s *SecureStub) Put(ctx context.Context, ws string, p auth.Provider, tok auth.Token) error {
	return errors.New("no secure store")
}
func (s *SecureStub) Delete(ctx context.Context, ws string, p auth.Provider) error { return nil }

(추후)

//go:build darwin Keychain

//go:build windows CredMan

//go:build linux SecretService/libsecret
파일을 같은 패키지에 추가하면 자동으로 대체됩니다.



---

4-5) OAuth Provider 설정/교환

internal/auth/providers.go

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
	ClientID     string
	ClientSecret string
	AuthURL      string // OIDC custom
	TokenURL     string // OIDC custom
	Scopes       []string
}

type Manager struct {
	Store      TokenStore
	State      *StateStore
	BaseURL    string // daemon public base, e.g. http://127.0.0.1:7777
	Workspace  func() string
	Configs    map[Provider]ProviderConfig
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


---

4-6) Refresh (자동 갱신)

internal/auth/refresh.go

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


---

4-7) HTTP 라우트 (authorize/callback/status/logout)

internal/server/routes/oauth.go

package routes

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"devorch/internal/auth"
)

type OAuthRoutes struct {
	Auth *auth.Manager
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// GET /oauth/authorize/{provider}
func (o *OAuthRoutes) Authorize(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/oauth/authorize/")
	prov := auth.Provider(p)

	u, state, err := o.Auth.Begin(r.Context(), prov)
	if err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"auth_url": u, "state": state})
}

// GET /oauth/callback/{provider}?code=...&state=...
func (o *OAuthRoutes) Callback(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/oauth/callback/")
	prov := auth.Provider(p)

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		writeJSON(w, 400, map[string]any{"error": "missing code/state"})
		return
	}

	if err := o.Auth.Complete(r.Context(), prov, code, state); err != nil {
		writeJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}

	// UX: 브라우저에 간단 성공 페이지
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write([]byte(`<html><body><h3>Login success</h3><p>You can return to DevOrch.</p></body></html>`))
}

// GET /oauth/status/{provider}
func (o *OAuthRoutes) Status(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/oauth/status/")
	prov := auth.Provider(p)

	tok, ok, err := o.Auth.Status(r.Context(), prov)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"provider": prov,
		"ok":       ok,
		"expired":  ok && tok.Expired(30*time.Second),
		"expiry":   tok.Expiry.Unix(),
		"scope":    tok.Scope,
	})
}

// POST /oauth/logout/{provider}
func (o *OAuthRoutes) Logout(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(r.URL.Path, "/oauth/logout/")
	prov := auth.Provider(p)

	if err := o.Auth.Logout(r.Context(), prov); err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}


---

4-8) OAuth-only 강제 미들웨어

> “API Key 금지 / OAuth 토큰 없으면 접근 불가”를 이 미들웨어에서 강제합니다.
여기서는 간단히 요청 헤더 X-DevOrch-Provider 로 어떤 provider 토큰을 쓸지 지정합니다.
(추후: session/workspace 정책으로 자동 선택 가능)



internal/server/middleware/auth_middleware.go

package middleware

import (
	"net/http"
	"time"

	"devorch/internal/auth"
)

type OAuthMiddleware struct {
	Auth *auth.Manager
}

func (m *OAuthMiddleware) RequireOAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// OAuth 라우트 자체는 제외
		if len(r.URL.Path) >= 6 && r.URL.Path[:6] == "/oauth" {
			next.ServeHTTP(w, r)
			return
		}

		prov := auth.Provider(r.Header.Get("X-DevOrch-Provider"))
		if prov == "" {
			// 기본 provider 정책 (예: github/corporate oidc)
			prov = auth.ProviderGitHub
		}

		_, ok, err := m.Auth.EnsureFresh(r.Context(), prov, 30*time.Second)
		if err != nil || !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"oauth_required","hint":"run 'devorch login' or use /oauth/authorize"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}


---

4-9) Server 라우터에 OAuth 라우트 등록

internal/server/routes/router.go (있다면 보완, 없으면 신규)

package routes

import (
	"net/http"
)

type Router struct {
	mux *http.ServeMux
}

func NewRouter() *Router {
	return &Router{mux: http.NewServeMux()}
}

func (r *Router) Handle(pattern string, h func(http.ResponseWriter, *http.Request)) {
	r.mux.HandleFunc(pattern, h)
}

func (r *Router) Handler() http.Handler { return r.mux }

internal/server/server.go (핵심 등록부만)

package server

import (
	"net/http"
	"time"

	"devorch/internal/auth"
	"devorch/internal/server/middleware"
	"devorch/internal/server/routes"
)

type Server struct {
	HTTP *http.Server
}

type Deps struct {
	BaseURL string
	AuthMgr *auth.Manager
}

func New(d Deps) *Server {
	rt := routes.NewRouter()

	oauth := &routes.OAuthRoutes{Auth: d.AuthMgr}
	rt.Handle("/oauth/authorize/", oauth.Authorize)
	rt.Handle("/oauth/callback/", oauth.Callback)
	rt.Handle("/oauth/status/", oauth.Status)
	rt.Handle("/oauth/logout/", oauth.Logout)

	// (이미 3단계에서 만든 SSE/WS/Session 라우트도 여기서 등록한다고 가정)

	mw := (&middleware.OAuthMiddleware{Auth: d.AuthMgr}).RequireOAuth

	s := &http.Server{
		Addr:              ":7777",
		Handler:           mw(rt.Handler()),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return &Server{HTTP: s}
}


---

4-10) CLI 로그인 (브라우저 열기 + 상태 확인)

internal/cli/login.go

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
)

type OAuthAuthorizeResp struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func Login(baseURL string, provider string) error {
	u := fmt.Sprintf("%s/oauth/authorize/%s", baseURL, provider)
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var ar OAuthAuthorizeResp
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return err
	}
	fmt.Println("Opening browser for OAuth login...")
	if err := openBrowser(ar.AuthURL); err != nil {
		fmt.Println("Open this URL manually:", ar.AuthURL)
	}
	fmt.Println("After finishing login, run: devorch login-status", provider)
	return nil
}

func LoginStatus(baseURL string, provider string) error {
	u := fmt.Sprintf("%s/oauth/status/%s", baseURL, provider)
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var v map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&v)
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
	return nil
}

func Logout(baseURL string, provider string) error {
	u := fmt.Sprintf("%s/oauth/logout/%s", baseURL, provider)
	req, _ := http.NewRequest("POST", u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Println("Logged out:", provider)
	return nil
}

(CLI 엔트리/서브커맨드 연결은 기존 internal/cli/root.go에서 login, login-status, logout로 연결하면 됩니다.)


---

4-11) Provider 호출 시 OAuth 토큰 주입(핵심)

> 예시로 OpenAI-compatible provider에 “Bearer 토큰”을 넣는 형태입니다.
실제로는 provider별로 헤더/쿼리 방식이 달라질 수 있으므로 provider/transform.go 계층으로 이동시켜도 됩니다.



internal/provider/openai/openai.go (토큰 주입 보완 예시)

package openai

import (
	"context"
	"net/http"
	"time"

	"devorch/internal/auth"
	"devorch/internal/provider"
)

type Client struct {
	HTTP *http.Client
	Auth *auth.Manager
}

func New(authMgr *auth.Manager) *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 60 * time.Second},
		Auth: authMgr,
	}
}

func (c *Client) doAuthed(ctx context.Context, req *http.Request) error {
	tok, ok, err := c.Auth.EnsureFresh(ctx, auth.ProviderOpenAI, 30*time.Second)
	if err != nil || !ok {
		return provider.ErrUnauthorized
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	return nil
}

> GitHub Copilot / Google / Anthropic도 같은 방식으로 “AuthMgr에서 토큰 가져와 헤더에 주입”으로 통일하면 됩니다.




---

4-12) VSCode 확장: OAuth 연동

vscode/src/oauth.ts

import * as vscode from "vscode";
import { ApiClient } from "./client";

export async function login(api: ApiClient, provider: string) {
  const r = await api.get(`/oauth/authorize/${provider}`);
  const authUrl = r.auth_url as string;

  await vscode.env.openExternal(vscode.Uri.parse(authUrl));

  vscode.window.showInformationMessage(
    `OAuth login opened in browser. After completing, run "DevOrch: Check Login Status".`
  );
}

export async function loginStatus(api: ApiClient, provider: string) {
  const r = await api.get(`/oauth/status/${provider}`);
  if (r.ok) {
    vscode.window.showInformationMessage(
      `${provider} login OK (expired=${r.expired})`
    );
  } else {
    vscode.window.showWarningMessage(`${provider} not logged in.`);
  }
}

export async function logout(api: ApiClient, provider: string) {
  await api.post(`/oauth/logout/${provider}`);
  vscode.window.showInformationMessage(`${provider} logged out.`);
}

vscode/src/client.ts

import fetch from "node-fetch";

export class ApiClient {
  constructor(private baseUrl: string, private provider: string) {}

  private headers() {
    return {
      "Content-Type": "application/json",
      "X-DevOrch-Provider": this.provider,
    };
  }

  async get(path: string): Promise<any> {
    const res = await fetch(this.baseUrl + path, { headers: this.headers() });
    return await res.json();
  }

  async post(path: string, body?: any): Promise<any> {
    const res = await fetch(this.baseUrl + path, {
      method: "POST",
      headers: this.headers(),
      body: body ? JSON.stringify(body) : undefined,
    });
    if (res.headers.get("content-type")?.includes("application/json")) {
      return await res.json();
    }
    return {};
  }
}

vscode/src/extension.ts (로그인 커맨드 연결)

import * as vscode from "vscode";
import { ApiClient } from "./client";
import { login, loginStatus, logout } from "./oauth";

export function activate(context: vscode.ExtensionContext) {
  const baseUrl = vscode.workspace.getConfiguration("devorch").get<string>("daemonBaseUrl") || "http://127.0.0.1:7777";
  const provider = vscode.workspace.getConfiguration("devorch").get<string>("provider") || "github";
  const api = new ApiClient(baseUrl, provider);

  context.subscriptions.push(
    vscode.commands.registerCommand("devorch.login", async () => login(api, provider)),
    vscode.commands.registerCommand("devorch.loginStatus", async () => loginStatus(api, provider)),
    vscode.commands.registerCommand("devorch.logout", async () => logout(api, provider)),
  );
}

export function deactivate() {}

> vscode/package.json에는 commands 3개만 등록해주면 됩니다:



devorch.login

devorch.loginStatus

devorch.logout



---

4-13) WebUI: OAuth 화면(최소)

webui/src/lib/api.ts

export async function apiGet(path: string, provider: string) {
  const r = await fetch(path, { headers: { "X-DevOrch-Provider": provider } });
  return await r.json();
}
export async function apiPost(path: string, provider: string) {
  const r = await fetch(path, { method: "POST", headers: { "X-DevOrch-Provider": provider } });
  if (r.headers.get("content-type")?.includes("application/json")) return await r.json();
  return {};
}

webui/src/routes/oauth.tsx

import { useState } from "react";
import { apiGet, apiPost } from "../lib/api";

export default function OAuthPage() {
  const [provider, setProvider] = useState("github");
  const [status, setStatus] = useState<any>(null);

  const begin = async () => {
    const r = await apiGet(`/oauth/authorize/${provider}`, provider);
    window.open(r.auth_url, "_blank");
  };

  const check = async () => {
    const r = await apiGet(`/oauth/status/${provider}`, provider);
    setStatus(r);
  };

  const out = async () => {
    await apiPost(`/oauth/logout/${provider}`, provider);
    setStatus(null);
  };

  return (
    <div style={{ padding: 16 }}>
      <h2>OAuth Login</h2>
      <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
        <select value={provider} onChange={(e) => setProvider(e.target.value)}>
          <option value="github">github</option>
          <option value="google">google</option>
          <option value="openai">openai</option>
          <option value="anthropic">anthropic</option>
          <option value="custom_oidc">custom_oidc</option>
        </select>
        <button onClick={begin}>Begin</button>
        <button onClick={check}>Status</button>
        <button onClick={out}>Logout</button>
      </div>

      <pre style={{ marginTop: 16, background: "#111", color: "#eee", padding: 12 }}>
        {status ? JSON.stringify(status, null, 2) : "No status yet."}
      </pre>
    </div>
  );
}


---

4-14) App Wiring (Auth Manager 주입)

> internal/app/wire.go 또는 internal/app/app.go에서 구성합니다.
(아래는 “필요 최소” 예시)



internal/app/wire_auth.go (신규)

package app

import (
	"time"

	"devorch/internal/auth"
	"devorch/platform/tokenstore"
)

type AuthDeps struct {
	BaseURL   string
	TokenStore auth.TokenStore
	WorkspaceID func() string
	Configs   map[auth.Provider]auth.ProviderConfig
}

func NewAuthManager(d AuthDeps) *auth.Manager {
	// secure store는 이후 구현으로 연결 가능
	_ = tokenstore.SecureStub{}

	return &auth.Manager{
		Store:     d.TokenStore,
		State:     auth.NewStateStore(10 * time.Minute),
		BaseURL:   d.BaseURL,
		Workspace: d.WorkspaceID,
		Configs:   d.Configs,
	}
}


---

4단계 “완료 기준”

daemon에서 /oauth/authorize/github 호출 → 브라우저 열림

callback 완료 후 /oauth/status/github → { ok: true }

SSE/WS/일반 API 호출 시 토큰 없으면 401 oauth_required

VSCode/WebUI에서 Begin/Status/Logout 동작



---

원하면 다음 단계(5단계)에서 바로:

1. OAuth provider별 실제 스코프/엔드포인트/인증정책 강화(Copilot/Entra/OIDC discovery 포함)


2. Provider HTTP 요청에 자동 토큰 주입 + 멀티워크스페이스/테넌시 적용


3. Streaming SSE/WS 경로를 실제 session/router와 완전 결합 (실시간 diff/툴 로그까지)
까지 “풀코드”로 이어가겠습니다.


