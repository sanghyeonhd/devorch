아래는 **5단계(Provider OAuth 실전화 + OIDC discovery + 멀티 워크스페이스/테넌시 적용 + Provider 호출 시 자동 토큰 주입 + 서버/SDK/VSCode/WebUI 연동 강화 + Doctor 체크)**를 “컴파일/동작 가능한 풀코드”로 제공합니다.

> 범위가 매우 커서, 5단계는 “동작 최소완성(MVP) + 확장 포인트 포함” 형태로 설계했습니다.
(Copilot은 공식/비공식 흐름이 다양해서 OIDC/Device Flow 포함 구조만 “완성”하고, 실제 Copilot Chat endpoint는 이후 6단계에서 provider 세부로 확장하는 게 안전합니다.)




---

5단계: 추가/변경 파일 목록

Go

internal/tenancy/* (workspace/org/role 최소)

internal/server/middleware/workspace.go (workspace 선택/주입)

internal/config/oauth.go + internal/config/load_oauth.go (oauth.jsonc 로드)

internal/auth/providers_github.go

internal/auth/providers_google.go

internal/auth/providers_openai.go (OpenAI/OIDC 스타일)

internal/auth/providers_anthropic.go (Anthropic OAuth는 실제로 제한될 수 있어 OIDC/Custom로 처리)

internal/auth/oidc_discovery.go (OIDC discovery + jwks url 등)

internal/auth/device_flow.go (Device Authorization Flow)

internal/provider/registry_auth.go (provider 호출 전에 토큰 자동 주입)

internal/server/routes/provider_proxy.go (provider “프록시” 예시)

internal/diagnostics/checks/oauth.go (doctor oauth 체크)

internal/pkg/sdk/client_auth.go (SDK에서 provider/workspace 헤더 일관화)


VSCode/WebUI

vscode/src/settings.ts (workspace/provider 선택)

webui/src/lib/api.ts (workspace/provider 헤더 추가)



---

5-1) Tenancy(Workspace/Org) 최소 구현

internal/tenancy/workspace.go

package tenancy

import "errors"

type Workspace struct {
	ID   string
	Name string
	Org  string
	Role string
}

var ErrNoWorkspace = errors.New("no workspace")

// MVP: 단일 워크스페이스(로컬) 기본값
func DefaultWorkspace() Workspace {
	return Workspace{
		ID:   "local",
		Name: "Local Workspace",
		Org:  "local",
		Role: "owner",
	}
}

internal/tenancy/store.go

package tenancy

import (
	"context"
	"sync"
)

type Store struct {
	mu  sync.Mutex
	cur Workspace
}

func NewStore() *Store {
	return &Store{cur: DefaultWorkspace()}
}

func (s *Store) Current(ctx context.Context) Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

func (s *Store) SetCurrent(ctx context.Context, ws Workspace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cur = ws
}


---

5-2) Workspace 미들웨어(헤더로 선택)

요청 헤더:

X-DevOrch-Workspace: <id>

X-DevOrch-Provider: <provider>



internal/server/middleware/workspace.go

package middleware

import (
	"context"
	"net/http"

	"devorch/internal/tenancy"
)

type ctxKey string

const (
	ctxWorkspaceKey ctxKey = "workspace"
)

func WithWorkspace(next http.Handler, ts *tenancy.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws := ts.Current(r.Context())
		if v := r.Header.Get("X-DevOrch-Workspace"); v != "" {
			// MVP: 알려진 목록 검증은 후속 단계(6~)
			ws.ID = v
		}
		ctx := context.WithValue(r.Context(), ctxWorkspaceKey, ws)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func WorkspaceFrom(ctx context.Context) tenancy.Workspace {
	if v := ctx.Value(ctxWorkspaceKey); v != nil {
		if ws, ok := v.(tenancy.Workspace); ok {
			return ws
		}
	}
	return tenancy.DefaultWorkspace()
}


---

5-3) OAuth 설정(oauth.jsonc) 로드 (프로젝트/.devorch + 글로벌)

> 4단계에서 “Configs map”을 코드에서 넣었는데, 5단계에서는 oauth.jsonc로부터 읽어옵니다.



internal/config/oauth.go

package config

type OAuthProviderConfig struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	AuthURL      string   `json:"auth_url,omitempty"`  // custom oidc
	TokenURL     string   `json:"token_url,omitempty"` // custom oidc
	IssuerURL    string   `json:"issuer_url,omitempty"`// oidc discovery
	Scopes       []string `json:"scopes,omitempty"`
	// Device Flow
	DeviceAuthURL string `json:"device_auth_url,omitempty"`
	// Extra (optional)
	Audience string `json:"audience,omitempty"`
}

type OAuthConfig struct {
	BaseURL   string                         `json:"base_url"` // callback base (daemon public)
	Providers map[string]OAuthProviderConfig  `json:"providers"`
}

internal/config/load_oauth.go

package config

import (
	"errors"
	"os"
	"path/filepath"
)

func LoadOAuthConfig(paths ...string) (OAuthConfig, error) {
	// 우선순위: 첫번째로 존재하는 파일
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var c OAuthConfig
		if err := UnmarshalJSONC(b, &c); err != nil {
			return OAuthConfig{}, err
		}
		if c.Providers == nil {
			c.Providers = map[string]OAuthProviderConfig{}
		}
		return c, nil
	}
	return OAuthConfig{}, errors.New("oauth config not found")
}

// 예: .devorch/oauth.jsonc, ~/.config/devorch/oauth.jsonc 형태로 호출
func DefaultOAuthPaths(projectRoot string, userConfigDir string) []string {
	return []string{
		filepath.Join(projectRoot, ".devorch", "oauth.jsonc"),
		filepath.Join(userConfigDir, "devorch", "oauth.jsonc"),
	}
}

> UnmarshalJSONC는 기존 3단계 internal/config/jsonc.go에 이미 있다고 가정합니다. (없으면 말해주면 추가해드립니다)




---

5-4) OIDC Discovery (issuer_url 기반 자동 endpoint 구성)

internal/auth/oidc_discovery.go

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type OIDCDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	DeviceEndpoint        string `json:"device_authorization_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

func DiscoverOIDC(ctx context.Context, issuer string) (OIDCDiscovery, error) {
	if issuer == "" {
		return OIDCDiscovery{}, errors.New("issuer empty")
	}
	issuer = strings.TrimRight(issuer, "/")
	u := issuer + "/.well-known/openid-configuration"

	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	cl := &http.Client{Timeout: 15 * time.Second}
	res, err := cl.Do(req)
	if err != nil {
		return OIDCDiscovery{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return OIDCDiscovery{}, errors.New("oidc discovery http " + res.Status)
	}
	var d OIDCDiscovery
	if err := json.NewDecoder(res.Body).Decode(&d); err != nil {
		return OIDCDiscovery{}, err
	}
	return d, nil
}


---

5-5) Device Authorization Flow (CLI/Headless 환경 대응)

internal/auth/device_flow.go

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type deviceTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Error        string `json:"error,omitempty"`
}

func (m *Manager) BeginDeviceFlow(ctx context.Context, p Provider) (DeviceCodeResponse, error) {
	conf, ok := m.Configs[p]
	if !ok || conf.TokenURL == "" {
		// device는 providerconfig에 DeviceAuthURL 또는 OIDC discovery 필요
	}
	pc, ok := m.Configs[p]
	if !ok {
		return DeviceCodeResponse{}, errors.New("provider not configured")
	}

	deviceURL := ""
	if pc.DeviceAuthURL != "" {
		deviceURL = pc.DeviceAuthURL
	} else if pc.AuthURL == "" && pc.TokenURL == "" && pc.IssuerURL != "" {
		d, err := DiscoverOIDC(ctx, pc.IssuerURL)
		if err != nil {
			return DeviceCodeResponse{}, err
		}
		deviceURL = d.DeviceEndpoint
		if deviceURL == "" {
			return DeviceCodeResponse{}, errors.New("issuer has no device endpoint")
		}
	} else {
		return DeviceCodeResponse{}, errors.New("device flow endpoints not configured")
	}

	form := url.Values{}
	form.Set("client_id", pc.ClientID)
	if len(pc.Scopes) > 0 {
		form.Set("scope", strings.Join(pc.Scopes, " "))
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", deviceURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	cl := &http.Client{Timeout: 20 * time.Second}
	res, err := cl.Do(req)
	if err != nil {
		return DeviceCodeResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return DeviceCodeResponse{}, errors.New("device auth http " + res.Status)
	}

	var out DeviceCodeResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return DeviceCodeResponse{}, err
	}
	if out.Interval == 0 {
		out.Interval = 5
	}
	return out, nil
}

func (m *Manager) PollDeviceFlow(ctx context.Context, p Provider, deviceCode string, intervalSec int) (Token, error) {
	pc, ok := m.Configs[p]
	if !ok {
		return Token{}, errors.New("provider not configured")
	}

	tokenURL := pc.TokenURL
	if tokenURL == "" && pc.IssuerURL != "" {
		d, err := DiscoverOIDC(ctx, pc.IssuerURL)
		if err != nil {
			return Token{}, err
		}
		tokenURL = d.TokenEndpoint
	}
	if tokenURL == "" {
		return Token{}, errors.New("token endpoint not configured")
	}

	interval := time.Duration(intervalSec) * time.Second
	if interval < 2*time.Second {
		interval = 2 * time.Second
	}

	for {
		select {
		case <-ctx.Done():
			return Token{}, ctx.Err()
		default:
		}

		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("client_id", pc.ClientID)
		form.Set("device_code", deviceCode)
		// 일부 IdP는 secret 요구
		if pc.ClientSecret != "" {
			form.Set("client_secret", pc.ClientSecret)
		}

		req, _ := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		cl := &http.Client{Timeout: 20 * time.Second}
		res, err := cl.Do(req)
		if err != nil {
			return Token{}, err
		}

		var tr deviceTokenResponse
		_ = json.NewDecoder(res.Body).Decode(&tr)
		res.Body.Close()

		// RFC: authorization_pending / slow_down / expired_token 등 처리
		if tr.Error == "authorization_pending" {
			time.Sleep(interval)
			continue
		}
		if tr.Error == "slow_down" {
			interval += 2 * time.Second
			time.Sleep(interval)
			continue
		}
		if tr.Error != "" {
			return Token{}, errors.New("device flow error: " + tr.Error)
		}
		if tr.AccessToken == "" {
			return Token{}, errors.New("device flow: no access_token")
		}

		exp := time.Time{}
		if tr.ExpiresIn > 0 {
			exp = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
		}
		return Token{
			AccessToken:  tr.AccessToken,
			RefreshToken: tr.RefreshToken,
			TokenType:    tr.TokenType,
			Expiry:       exp,
			Scope:        tr.Scope,
			IDToken:      tr.IDToken,
		}, nil
	}
}


---

5-6) Provider별 OAuth config 빌드(issuer_url/oidc 포함)

기존 4단계 Manager.oauthConfig()를 확장합니다.

internal/auth/providers_build.go (기존 providers.go의 oauthConfig를 교체/확장)

package auth

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

type ProviderConfig struct {
	ClientID     string
	ClientSecret string
	Scopes       []string

	// OIDC/custom
	IssuerURL string
	AuthURL   string
	TokenURL  string

	// Device flow
	DeviceAuthURL string

	Audience string
}

func (m *Manager) oauthConfig(ctx context.Context, p Provider) (*oauth2.Config, error) {
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

	case ProviderOpenAI, ProviderAnthropic, ProviderOIDC:
		// OpenAI/Anthropic은 “공식 OAuth”가 항상 제공되는 게 아니라서
		// enterprise(OIDC) / OpenRouter / custom_oidc 방식으로 흡수하는 설계를 권장.
		authURL := c.AuthURL
		tokenURL := c.TokenURL

		if (authURL == "" || tokenURL == "") && c.IssuerURL != "" {
			d, err := DiscoverOIDC(ctx, c.IssuerURL)
			if err != nil {
				return nil, err
			}
			if authURL == "" {
				authURL = d.AuthorizationEndpoint
			}
			if tokenURL == "" {
				tokenURL = d.TokenEndpoint
			}
		}

		if authURL == "" || tokenURL == "" {
			return nil, errors.New("oidc/custom endpoints missing")
		}

		// audience는 일부 IdP가 요구(추후 단계에서 AuthCodeURL param로 추가 가능)
		return &oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authURL,
				TokenURL: tokenURL,
			},
			RedirectURL: redirect,
			Scopes:      normalizeScopes(c.Scopes),
		}, nil

	default:
		return nil, errors.New("provider not implemented")
	}
}

func normalizeScopes(scopes []string) []string {
	// 빈 값 제거
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

그리고 4단계 providers.go에서 m.oauthConfig(p) 호출 부분을 m.oauthConfig(ctx,p)로 바꿉니다.

internal/auth/providers.go 변경(핵심만)

// Begin()
conf, err := m.oauthConfig(ctx, p)
// Complete()
conf, err := m.oauthConfig(ctx, p)


---

5-7) oauth.jsonc -> auth.Manager.Configs 로딩/주입

internal/app/wire_oauth_config.go

package app

import (
	"devorch/internal/auth"
	"devorch/internal/config"
)

func OAuthConfigsFromFile(c config.OAuthConfig) map[auth.Provider]auth.ProviderConfig {
	out := map[auth.Provider]auth.ProviderConfig{}
	for k, v := range c.Providers {
		p := auth.Provider(k)
		out[p] = auth.ProviderConfig{
			ClientID:       v.ClientID,
			ClientSecret:   v.ClientSecret,
			AuthURL:        v.AuthURL,
			TokenURL:       v.TokenURL,
			IssuerURL:      v.IssuerURL,
			Scopes:         v.Scopes,
			DeviceAuthURL:  v.DeviceAuthURL,
			Audience:       v.Audience,
		}
	}
	return out
}


---

5-8) Provider Registry에서 “자동 OAuth 토큰 주입” 표준화

> 핵심: provider 구현체들이 각자 auth를 신경쓰지 않게 하고, “요청 만들기 전에” 공통 계층에서 토큰을 붙입니다.



internal/provider/registry_auth.go

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
	// 필요 시 scope/tenant 등 추가 헤더 주입 가능
	if tok.Scope != "" {
		req.Header.Set("X-DevOrch-Token-Scope", strings.TrimSpace(tok.Scope))
	}
	return nil
}


---

5-9) Provider Proxy API (서버가 외부 Provider로 호출을 “대신” 수행)

> DevOrch의 라우터/스케줄러가 provider를 선택한 후, 서버가 provider 호출을 수행하는 형태의 샘플입니다.
(이 구조가 있어야 “OAuth-only”를 사용성 있게 강제하면서도 SDK/VSCode/WebUI는 단순해집니다.)



internal/server/routes/provider_proxy.go

package routes

import (
	"io"
	"net/http"
	"strings"
	"time"

	"devorch/internal/server/middleware"
	"devorch/internal/tenancy"
	"devorch/internal/provider"
)

type ProviderProxy struct {
	Injector *provider.AuthInjector
	// 실제로는 provider registry + router + scheduler를 여기서 호출
}

func (p *ProviderProxy) ChatProxy(w http.ResponseWriter, r *http.Request) {
	// POST /provider/chat/{providerName}
	prov := strings.TrimPrefix(r.URL.Path, "/provider/chat/")
	if prov == "" {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"missing provider"}`))
		return
	}

	// workspace는 정책/쿼터에 필요
	ws := middleware.WorkspaceFrom(r.Context())
	_ = ws // 확장 포인트

	// MVP: 단순 프록시 (외부 URL은 provider별로 다르므로 예시만)
	target := externalChatURL(prov)
	if target == "" {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"unknown provider target"}`))
		return
	}

	body, _ := io.ReadAll(r.Body)
	req, _ := http.NewRequestWithContext(r.Context(), "POST", target, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")

	if err := p.Injector.Inject(r.Context(), req, prov); err != nil {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"oauth_required"}`))
		return
	}

	cl := &http.Client{Timeout: 60 * time.Second}
	res, err := cl.Do(req)
	if err != nil {
		w.WriteHeader(502)
		_, _ = w.Write([]byte(`{"error":"upstream_failed"}`))
		return
	}
	defer res.Body.Close()

	w.Header().Set("Content-Type", res.Header.Get("Content-Type"))
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, res.Body)
}

func externalChatURL(providerName string) string {
	// 6단계에서 provider별 endpoint/streaming까지 완성
	switch providerName {
	case "openai":
		return "https://api.openai.com/v1/chat/completions"
	case "google":
		// Gemini는 REST가 다름: 6단계에서 정식 구현
		return ""
	case "github":
		return "" // copilot chat endpoint는 별도
	default:
		return ""
	}
}

// 참고: 추후 workspace 정책 적용 시 이런 형태로 provider 제한 가능
func allowed(ws tenancy.Workspace, providerName string) bool {
	_ = ws
	_ = providerName
	return true
}

그리고 서버 라우터 등록:

internal/server/server.go (추가 등록 부분만)

// ... oauth routes 아래에 추가
proxy := &routes.ProviderProxy{Injector: &provider.AuthInjector{Auth: d.AuthMgr}}
rt.Handle("/provider/chat/", proxy.ChatProxy)


---

5-10) OAuth Middleware 개선: workspace 기반 토큰 조회

4단계에서는 m.Workspace()가 고정이었는데, 이제는 요청 컨텍스트의 workspace를 사용해야 합니다.

internal/auth/manager_workspace.go

package auth

import (
	"context"

	"devorch/internal/server/middleware"
)

func WorkspaceFromRequest(ctx context.Context) string {
	ws := middleware.WorkspaceFrom(ctx)
	return ws.ID
}

그리고 auth.Manager 생성 시 Workspace 함수 교체:

internal/app/wire_auth.go (수정)

func NewAuthManager(d AuthDeps) *auth.Manager {
	return &auth.Manager{
		Store:     d.TokenStore,
		State:     auth.NewStateStore(10 * time.Minute),
		BaseURL:   d.BaseURL,
		Workspace: func() string { return "local" }, // <- 서버 요청 ctx에서 읽도록 6단계에서 완전 전환
		Configs:   d.Configs,
	}
}

> 중요: 서버 요청마다 workspace가 달라질 수 있으므로, 6단계에서 EnsureFresh(ctx, ...)가 ctx에서 workspace를 읽도록 Manager API를 개선하는 게 베스트입니다.
5단계에서는 “동작”을 위해 기본 workspace를 유지했고, middleware에서 workspace 헤더를 넣으면 프록시/SDK 레벨에선 정책 반영이 가능합니다.




---

5-11) Doctor: OAuth 설정/로그인 상태 체크

internal/diagnostics/checks/oauth.go

package checks

import (
	"context"
	"fmt"

	"devorch/internal/auth"
)

type OAuthCheck struct {
	Auth *auth.Manager
}

func (c *OAuthCheck) Name() string { return "oauth" }

func (c *OAuthCheck) Run(ctx context.Context) (string, error) {
	// 대표 provider 몇 개만 체크
	providers := []auth.Provider{
		auth.ProviderGitHub,
		auth.ProviderGoogle,
		auth.ProviderOIDC,
	}

	out := ""
	for _, p := range providers {
		_, ok, err := c.Auth.Status(ctx, p)
		if err != nil {
			out += fmt.Sprintf("- %s: error=%v\n", p, err)
			continue
		}
		out += fmt.Sprintf("- %s: logged_in=%v\n", p, ok)
	}
	return out, nil
}

(doctor 메인에서 체크 등록은 기존 internal/diagnostics/doctor.go에 add)


---

5-12) SDK: workspace/provider 헤더 통일

pkg/sdk/client_auth.go

package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

type Client struct {
	BaseURL   string
	Provider  string
	Workspace string
	HTTP      *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		Provider: "github",
		Workspace: "local",
		HTTP: http.DefaultClient,
	}
}

func (c *Client) headers(req *http.Request) {
	if c.Provider != "" {
		req.Header.Set("X-DevOrch-Provider", c.Provider)
	}
	if c.Workspace != "" {
		req.Header.Set("X-DevOrch-Workspace", c.Workspace)
	}
}

func (c *Client) PostJSON(ctx context.Context, path string, body any) (*http.Response, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	c.headers(req)
	return c.HTTP.Do(req)
}


---

5-13) VSCode/WebUI 헤더 확장 (workspace 선택)

vscode/src/settings.ts (신규)

import * as vscode from "vscode";

export function getProvider(): string {
  return vscode.workspace.getConfiguration("devorch").get<string>("provider") || "github";
}

export function getWorkspaceId(): string {
  return vscode.workspace.getConfiguration("devorch").get<string>("workspaceId") || "local";
}

export function getDaemonBaseUrl(): string {
  return vscode.workspace.getConfiguration("devorch").get<string>("daemonBaseUrl") || "http://127.0.0.1:7777";
}

vscode/src/client.ts (수정: workspace header)

import fetch from "node-fetch";

export class ApiClient {
  constructor(private baseUrl: string, private provider: string, private workspaceId: string) {}

  private headers() {
    return {
      "Content-Type": "application/json",
      "X-DevOrch-Provider": this.provider,
      "X-DevOrch-Workspace": this.workspaceId,
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
    if (res.headers.get("content-type")?.includes("application/json")) return await res.json();
    return {};
  }
}

vscode/src/extension.ts (수정: settings 사용)

import * as vscode from "vscode";
import { ApiClient } from "./client";
import { login, loginStatus, logout } from "./oauth";
import { getDaemonBaseUrl, getProvider, getWorkspaceId } from "./settings";

export function activate(context: vscode.ExtensionContext) {
  const baseUrl = getDaemonBaseUrl();
  const provider = getProvider();
  const workspaceId = getWorkspaceId();
  const api = new ApiClient(baseUrl, provider, workspaceId);

  context.subscriptions.push(
    vscode.commands.registerCommand("devorch.login", async () => login(api, provider)),
    vscode.commands.registerCommand("devorch.loginStatus", async () => loginStatus(api, provider)),
    vscode.commands.registerCommand("devorch.logout", async () => logout(api, provider)),
  );
}

export function deactivate() {}

webui/src/lib/api.ts (수정)

export async function apiGet(path: string, provider: string, workspaceId: string) {
  const r = await fetch(path, {
    headers: { "X-DevOrch-Provider": provider, "X-DevOrch-Workspace": workspaceId }
  });
  return await r.json();
}

export async function apiPost(path: string, provider: string, workspaceId: string) {
  const r = await fetch(path, {
    method: "POST",
    headers: { "X-DevOrch-Provider": provider, "X-DevOrch-Workspace": workspaceId }
  });
  if (r.headers.get("content-type")?.includes("application/json")) return await r.json();
  return {};
}


---

5-14) oauth.jsonc 예시(실사용)

/.devorch/oauth.jsonc

{
  "base_url": "http://127.0.0.1:7777",
  "providers": {
    "github": {
      "client_id": "GITHUB_OAUTH_CLIENT_ID",
      "client_secret": "GITHUB_OAUTH_CLIENT_SECRET",
      "scopes": ["read:user", "user:email"]
    },
    "google": {
      "client_id": "GOOGLE_CLIENT_ID",
      "client_secret": "GOOGLE_CLIENT_SECRET",
      "scopes": ["openid", "email", "profile"]
    },
    "custom_oidc": {
      "issuer_url": "https://login.microsoftonline.com/<TENANT_ID>/v2.0",
      "client_id": "ENTRA_APP_ID",
      "client_secret": "ENTRA_APP_SECRET",
      "scopes": ["openid", "profile", "email", "offline_access"]
    }
  }
}


---

5단계 완료 기준(동작 체크)

1. devorchd 실행


2. GET /oauth/authorize/github → 브라우저 로그인 → callback 성공


3. GET /oauth/status/github → ok=true


4. POST /provider/chat/openai 같은 프록시 호출 시

토큰 없으면 401

토큰 있으면 Authorization 헤더가 upstream에 붙어서 요청됨



5. VSCode/WebUI에서 provider/workspace 헤더가 항상 붙음


6. doctor에서 oauth 체크가 표시됨




---

다음(6단계)에서 바로 완성할 것

5단계가 “OAuth 시스템/멀티 워크스페이스/자동주입/프록시 골격”이라면, 6단계는 실제 멀티LLM 사용성의 핵심입니다:

Provider별 “정식” Chat/Embedding/Tool/Streaming 구현

OpenAI: SSE streaming, tool-calls

Anthropic: messages API, streaming

Google Gemini: REST/stream

OpenRouter: OpenAI 호환

Local(Ollama): stream + 모델풀/벤치/오프라인 설치 연동


라우터/스케줄러와 “벤치마크 이력(오카온 저장)” 결합

작업 완료 기준으로 quality/latency/cost score를 누적 학습(로컬)


원하시면, 지금 바로 6단계 풀코드로 이어서:

/session 파이프라인에서 “작업 단위 평가/벤치 기록”

router.score에 “실측 벤치 기반 가중치”

storage에 벤치/평가 테이블(SQL 포함) 까지 한 번에 붙여드릴게요.