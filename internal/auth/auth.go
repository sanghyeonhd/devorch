// Package auth provides OAuth and API key authentication for providers.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AuthType represents the type of authentication
type AuthType string

const (
	AuthTypeOAuth    AuthType = "oauth"
	AuthTypeAPI      AuthType = "api"
	AuthTypeNone     AuthType = "none"
	AuthTypeDevice   AuthType = "device"   // GitHub device flow
	AuthTypeHeadless AuthType = "headless" // OpenAI headless device flow
)

// AuthInfo represents stored authentication information
type AuthInfo struct {
	Type         AuthType `json:"type"`
	AccessToken  string   `json:"access,omitempty"`
	RefreshToken string   `json:"refresh,omitempty"`
	APIKey       string   `json:"key,omitempty"`
	ExpiresAt    int64    `json:"expires,omitempty"`
	AccountID    string   `json:"account_id,omitempty"`
	ProviderID   string   `json:"provider_id,omitempty"`
}

// AuthMethod represents an authentication method for a provider
type AuthMethod struct {
	Type        AuthType `json:"type"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
}

// OAuthResult represents the result of OAuth authorization
type OAuthResult struct {
	URL          string
	Instructions string
	Method       string // "auto" or "code"
	DeviceCode   string // For device flow
	UserCode     string // For device flow
	Interval     int    // Polling interval for device flow
	Verifier     string // PKCE verifier
	State        string // OAuth state
}

// Provider OAuth configurations
var ProviderConfigs = map[string]*ProviderAuthConfig{
	"anthropic": {
		ID:   "anthropic",
		Name: "Anthropic",
		AuthMethods: []AuthMethod{
			{Type: AuthTypeOAuth, Label: "Claude Pro/Max", Description: "Login with Claude subscription (OAuth)"},
			{Type: AuthTypeAPI, Label: "Create an API Key", Description: "Create key at console.anthropic.com"},
			{Type: AuthTypeAPI, Label: "Manually enter API Key", Description: "Enter existing API key"},
		},
		OAuth: &OAuthConfig{
			AuthorizeURL: "https://claude.ai/oauth/authorize",
			TokenURL:     "https://claude.ai/oauth/token",
			ClientID:     "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
			RedirectURI:  "https://console.anthropic.com/oauth/code/callback",
			Scopes:       []string{"org:create_api_key", "user:profile", "user:inference"},
			UsePKCE:      true,
			Method:       "code", // User copies code from browser
		},
		EnvVar:  "ANTHROPIC_API_KEY",
		AuthURL: "https://console.anthropic.com/settings/keys",
	},
	"openai": {
		ID:   "openai",
		Name: "OpenAI",
		AuthMethods: []AuthMethod{
			{Type: AuthTypeOAuth, Label: "ChatGPT Pro/Plus (Browser)", Description: "Login with browser (auto callback)"},
			{Type: AuthTypeHeadless, Label: "ChatGPT Pro/Plus (Headless)", Description: "Device code flow (no browser)"},
			{Type: AuthTypeAPI, Label: "Manually enter API Key", Description: "Get API key from platform.openai.com"},
		},
		OAuth: &OAuthConfig{
			AuthorizeURL: "https://auth.openai.com/oauth/authorize",
			TokenURL:     "https://auth.openai.com/oauth/token",
			ClientID:     "app_EMoamEEZ73f0CkXaXp7hrann",
			RedirectURI:  "http://localhost:1455/auth/callback",
			Scopes:       []string{"openid", "profile", "email", "offline_access"},
			UsePKCE:      true,
			Method:       "auto", // Auto callback via localhost
		},
		DeviceFlow: &DeviceFlowConfig{
			DeviceCodeURL:  "https://auth.openai.com/codex/device",
			AccessTokenURL: "https://auth.openai.com/oauth/token",
			ClientID:       "app_EMoamEEZ73f0CkXaXp7hrann",
			Scopes:         []string{"openid", "profile", "email", "offline_access"},
		},
		EnvVar:  "OPENAI_API_KEY",
		AuthURL: "https://platform.openai.com/api-keys",
	},
	"github-copilot": {
		ID:   "github-copilot",
		Name: "GitHub Copilot",
		AuthMethods: []AuthMethod{
			{Type: AuthTypeDevice, Label: "Login with GitHub Copilot", Description: "Device code flow (OAuth)"},
		},
		DeviceFlow: &DeviceFlowConfig{
			DeviceCodeURL:  "https://github.com/login/device/code",
			AccessTokenURL: "https://github.com/login/oauth/access_token",
			ClientID:       "Ov23li8tweQw6odWQebz",
			Scopes:         []string{"read:user"},
		},
		EnvVar: "GITHUB_COPILOT_TOKEN",
	},
	"google": {
		ID:   "google",
		Name: "Google",
		AuthMethods: []AuthMethod{
			{Type: AuthTypeAPI, Label: "Manually enter API Key", Description: "Get from aistudio.google.com"},
		},
		EnvVar:  "GOOGLE_API_KEY",
		AuthURL: "https://aistudio.google.com/apikey",
	},
	"groq": {
		ID:   "groq",
		Name: "Groq",
		AuthMethods: []AuthMethod{
			{Type: AuthTypeAPI, Label: "Manually enter API Key"},
		},
		EnvVar:  "GROQ_API_KEY",
		AuthURL: "https://console.groq.com/keys",
	},
	"openrouter": {
		ID:   "openrouter",
		Name: "OpenRouter",
		AuthMethods: []AuthMethod{
			{Type: AuthTypeAPI, Label: "Manually enter API Key"},
		},
		EnvVar:  "OPENROUTER_API_KEY",
		AuthURL: "https://openrouter.ai/keys",
	},
	"deepseek": {
		ID:   "deepseek",
		Name: "DeepSeek",
		AuthMethods: []AuthMethod{
			{Type: AuthTypeAPI, Label: "Manually enter API Key"},
		},
		EnvVar:  "DEEPSEEK_API_KEY",
		AuthURL: "https://platform.deepseek.com/api_keys",
	},
	"mistral": {
		ID:   "mistral",
		Name: "Mistral AI",
		AuthMethods: []AuthMethod{
			{Type: AuthTypeAPI, Label: "Manually enter API Key"},
		},
		EnvVar:  "MISTRAL_API_KEY",
		AuthURL: "https://console.mistral.ai/api-keys",
	},
	"together": {
		ID:   "together",
		Name: "Together AI",
		AuthMethods: []AuthMethod{
			{Type: AuthTypeAPI, Label: "Manually enter API Key"},
		},
		EnvVar:  "TOGETHER_API_KEY",
		AuthURL: "https://api.together.ai/settings/api-keys",
	},
	"xai": {
		ID:   "xai",
		Name: "xAI (Grok)",
		AuthMethods: []AuthMethod{
			{Type: AuthTypeAPI, Label: "Manually enter API Key"},
		},
		EnvVar:  "XAI_API_KEY",
		AuthURL: "https://console.x.ai",
	},
	"ollama": {
		ID:   "ollama",
		Name: "Ollama (Local)",
		AuthMethods: []AuthMethod{
			{Type: AuthTypeNone, Label: "Local (no auth required)"},
		},
	},
}

// ProviderAuthConfig holds provider authentication configuration
type ProviderAuthConfig struct {
	ID          string
	Name        string
	AuthMethods []AuthMethod
	OAuth       *OAuthConfig
	DeviceFlow  *DeviceFlowConfig
	EnvVar      string
	AuthURL     string
}

// OAuthConfig holds OAuth 2.0 configuration
type OAuthConfig struct {
	AuthorizeURL string
	TokenURL     string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
	UsePKCE      bool
	Method       string // "auto" or "code"
}

// DeviceFlowConfig holds device flow configuration
type DeviceFlowConfig struct {
	DeviceCodeURL  string
	AccessTokenURL string
	ClientID       string
	Scopes         []string
}

// AuthStore manages authentication storage
type AuthStore struct {
	mu       sync.RWMutex
	filePath string
	data     map[string]*AuthInfo
}

var globalStore *AuthStore
var storeOnce sync.Once

// GetStore returns the global auth store
func GetStore() *AuthStore {
	storeOnce.Do(func() {
		homeDir, _ := os.UserHomeDir()
		filePath := filepath.Join(homeDir, ".config", "devorch", "auth.json")
		globalStore = &AuthStore{
			filePath: filePath,
			data:     make(map[string]*AuthInfo),
		}
		globalStore.load()
	})
	return globalStore
}

func (s *AuthStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &s.data)
}

func (s *AuthStore) save() error {
	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0600)
}

// Get returns auth info for a provider
func (s *AuthStore) Get(providerID string) *AuthInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[providerID]
}

// Set stores auth info for a provider
func (s *AuthStore) Set(providerID string, info *AuthInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[providerID] = info
	return s.save()
}

// Remove deletes auth info for a provider
func (s *AuthStore) Remove(providerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, providerID)
	return s.save()
}

// IsConnected checks if a provider is authenticated
func (s *AuthStore) IsConnected(providerID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info := s.data[providerID]
	if info != nil {
		return true
	}

	// Also check environment variables
	config := ProviderConfigs[providerID]
	if config != nil && config.EnvVar != "" {
		return os.Getenv(config.EnvVar) != ""
	}

	return false
}

// All returns all stored auth info
func (s *AuthStore) All() map[string]*AuthInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*AuthInfo)
	for k, v := range s.data {
		result[k] = v
	}
	return result
}

// PKCE helpers
func generatePKCE() (verifier, challenge string, err error) {
	// Generate random verifier
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(bytes)

	// Generate challenge
	hash := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(hash[:])

	return verifier, challenge, nil
}

func generateState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// StartOAuthFlow starts OAuth authorization flow
func StartOAuthFlow(providerID string, methodIndex int) (*OAuthResult, error) {
	config := ProviderConfigs[providerID]
	if config == nil {
		return nil, fmt.Errorf("unknown provider: %s", providerID)
	}

	if methodIndex >= len(config.AuthMethods) {
		return nil, fmt.Errorf("invalid method index: %d", methodIndex)
	}

	method := config.AuthMethods[methodIndex]

	switch method.Type {
	case AuthTypeDevice:
		return startDeviceFlow(config)
	case AuthTypeHeadless:
		return startHeadlessDeviceFlow(config)
	case AuthTypeOAuth:
		return startOAuthAuthorization(config)
	case AuthTypeAPI:
		// Check if it's "Create an API Key" option
		if strings.Contains(method.Label, "Create") {
			return &OAuthResult{
				Method:       "create_api",
				URL:          config.AuthURL,
				Instructions: fmt.Sprintf("Create an API key at %s and enter it below.", config.AuthURL),
			}, nil
		}
		return &OAuthResult{
			Method:       "api",
			URL:          config.AuthURL,
			Instructions: fmt.Sprintf("Get your API key and enter it below.\nEnvironment variable: %s", config.EnvVar),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", method.Type)
	}
}

// startHeadlessDeviceFlow starts OpenAI headless device flow (no browser)
func startHeadlessDeviceFlow(config *ProviderAuthConfig) (*OAuthResult, error) {
	if config.DeviceFlow == nil {
		return nil, fmt.Errorf("device flow not configured for %s", config.ID)
	}

	df := config.DeviceFlow

	// Request device code from OpenAI
	data := url.Values{
		"client_id": {df.ClientID},
		"scope":     {strings.Join(df.Scopes, " ")},
	}

	req, err := http.NewRequest("POST", df.DeviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed: %s", string(body))
	}

	var result struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	verificationURL := result.VerificationURI
	if verificationURL == "" {
		verificationURL = "https://auth.openai.com/codex/device"
	}

	return &OAuthResult{
		URL:          verificationURL,
		Instructions: fmt.Sprintf("Enter code: %s", result.UserCode),
		Method:       "headless",
		DeviceCode:   result.DeviceCode,
		UserCode:     result.UserCode,
		Interval:     result.Interval,
	}, nil
}

func startOAuthAuthorization(config *ProviderAuthConfig) (*OAuthResult, error) {
	if config.OAuth == nil {
		return nil, fmt.Errorf("OAuth not configured for %s", config.ID)
	}

	oauth := config.OAuth

	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, err
	}

	state, err := generateState()
	if err != nil {
		return nil, err
	}

	params := url.Values{
		"response_type": {"code"},
		"client_id":     {oauth.ClientID},
		"redirect_uri":  {oauth.RedirectURI},
		"scope":         {strings.Join(oauth.Scopes, " ")},
		"state":         {state},
	}

	if oauth.UsePKCE {
		params.Set("code_challenge", challenge)
		params.Set("code_challenge_method", "S256")
	}

	// Provider-specific params
	switch config.ID {
	case "openai":
		// OpenAI/ChatGPT specific parameters
		params.Set("id_token_add_organizations", "true")
		params.Set("codex_cli_simplified_flow", "true")
		params.Set("originator", "opencode")
	case "anthropic":
		// Anthropic/Claude specific parameters - code=true for manual code entry
		params.Set("code", "true")
	}

	authURL := oauth.AuthorizeURL + "?" + params.Encode()

	instructions := "Complete authorization in your browser."
	if oauth.Method == "code" {
		instructions = "After authorization, copy the code from the browser and paste it here:"
	} else {
		instructions = "Complete authorization in your browser.\nWaiting for callback..."
	}

	return &OAuthResult{
		URL:          authURL,
		Instructions: instructions,
		Method:       oauth.Method,
		Verifier:     verifier,
		State:        state,
	}, nil
}

func startDeviceFlow(config *ProviderAuthConfig) (*OAuthResult, error) {
	if config.DeviceFlow == nil {
		return nil, fmt.Errorf("device flow not configured for %s", config.ID)
	}

	df := config.DeviceFlow

	// Request device code
	data := url.Values{
		"client_id": {df.ClientID},
		"scope":     {strings.Join(df.Scopes, " ")},
	}

	req, err := http.NewRequest("POST", df.DeviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device code request failed: %s", string(body))
	}

	var result struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &OAuthResult{
		URL:          result.VerificationURI,
		Instructions: fmt.Sprintf("Enter code: %s", result.UserCode),
		Method:       "auto",
		DeviceCode:   result.DeviceCode,
		UserCode:     result.UserCode,
		Interval:     result.Interval,
	}, nil
}

// PollDeviceFlow polls for device flow completion
func PollDeviceFlow(providerID string, result *OAuthResult) (*AuthInfo, error) {
	config := ProviderConfigs[providerID]
	if config == nil || config.DeviceFlow == nil {
		return nil, fmt.Errorf("invalid provider for device flow: %s", providerID)
	}

	df := config.DeviceFlow
	interval := result.Interval
	if interval < 5 {
		interval = 5
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for {
		time.Sleep(time.Duration(interval) * time.Second)

		data := url.Values{
			"client_id":   {df.ClientID},
			"device_code": {result.DeviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}

		req, err := http.NewRequest("POST", df.AccessTokenURL, strings.NewReader(data.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var tokenResp struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
			Interval    int    `json:"interval"`
		}

		if err := json.Unmarshal(body, &tokenResp); err != nil {
			continue
		}

		if tokenResp.AccessToken != "" {
			return &AuthInfo{
				Type:         AuthTypeOAuth,
				AccessToken:  tokenResp.AccessToken,
				RefreshToken: tokenResp.AccessToken,
				ProviderID:   providerID,
			}, nil
		}

		switch tokenResp.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5
			if tokenResp.Interval > 0 {
				interval = tokenResp.Interval
			}
			continue
		case "":
			continue
		default:
			return nil, fmt.Errorf("device flow failed: %s", tokenResp.Error)
		}
	}
}

// ExchangeCode exchanges authorization code for tokens
func ExchangeCode(providerID string, code string, result *OAuthResult) (*AuthInfo, error) {
	config := ProviderConfigs[providerID]
	if config == nil || config.OAuth == nil {
		return nil, fmt.Errorf("invalid provider for OAuth: %s", providerID)
	}

	oauth := config.OAuth

	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {oauth.RedirectURI},
		"client_id":    {oauth.ClientID},
	}

	if oauth.UsePKCE && result.Verifier != "" {
		data.Set("code_verifier", result.Verifier)
	}

	req, err := http.NewRequest("POST", oauth.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		IDToken      string `json:"id_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	expiresAt := int64(0)
	if tokenResp.ExpiresIn > 0 {
		expiresAt = time.Now().Unix() + int64(tokenResp.ExpiresIn)
	}

	return &AuthInfo{
		Type:         AuthTypeOAuth,
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    expiresAt,
		ProviderID:   providerID,
	}, nil
}

// SetAPIKey stores an API key for a provider
func SetAPIKey(providerID, apiKey string) error {
	store := GetStore()
	return store.Set(providerID, &AuthInfo{
		Type:       AuthTypeAPI,
		APIKey:     apiKey,
		ProviderID: providerID,
	})
}

// GetAPIKey retrieves the API key for a provider
func GetAPIKey(providerID string) string {
	// First check store
	store := GetStore()
	info := store.Get(providerID)
	if info != nil {
		if info.Type == AuthTypeAPI && info.APIKey != "" {
			return info.APIKey
		}
		if info.Type == AuthTypeOAuth && info.AccessToken != "" {
			return info.AccessToken
		}
	}

	// Then check environment
	config := ProviderConfigs[providerID]
	if config != nil && config.EnvVar != "" {
		return os.Getenv(config.EnvVar)
	}

	return ""
}

// OAuthCallbackServer handles OAuth callbacks
type OAuthCallbackServer struct {
	server *http.Server
	result chan string
	port   int
}

var callbackServer *OAuthCallbackServer
var callbackMu sync.Mutex

// StartCallbackServer starts the OAuth callback server
func StartCallbackServer(port int) (*OAuthCallbackServer, error) {
	callbackMu.Lock()
	defer callbackMu.Unlock()

	if callbackServer != nil {
		return callbackServer, nil
	}

	result := make(chan string, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errParam := r.URL.Query().Get("error")

		if errParam != "" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>Authorization Failed</title></head>
			<body style="font-family:system-ui;display:flex;justify-content:center;align-items:center;height:100vh;background:#131010;color:#f1ecec">
			<div style="text-align:center"><h1 style="color:#fc533a">Authorization Failed</h1><p>%s</p></div></body></html>`, errParam)
			result <- ""
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Authorization Successful</title></head>
		<body style="font-family:system-ui;display:flex;justify-content:center;align-items:center;height:100vh;background:#131010;color:#f1ecec">
		<div style="text-align:center"><h1>Authorization Successful</h1><p>You can close this window.</p></div>
		<script>setTimeout(()=>window.close(),2000)</script></body></html>`)

		result <- code
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	callbackServer = &OAuthCallbackServer{
		server: server,
		result: result,
		port:   port,
	}

	go server.ListenAndServe()

	return callbackServer, nil
}

// WaitForCallback waits for the OAuth callback
func (s *OAuthCallbackServer) WaitForCallback(ctx context.Context) (string, error) {
	select {
	case code := <-s.result:
		if code == "" {
			return "", fmt.Errorf("authorization failed")
		}
		return code, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Stop stops the callback server
func (s *OAuthCallbackServer) Stop() {
	callbackMu.Lock()
	defer callbackMu.Unlock()

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}
	callbackServer = nil
}
