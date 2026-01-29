package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"devorch/internal/app"
	"devorch/internal/config"
	"devorch/internal/global"
	"devorch/internal/log"
)

//go:embed static
var staticFS embed.FS

func main() {
	log.Init()

	cfg, err := config.Load()
	if err != nil {
		log.Errorf("config load failed: %v", err)
		os.Exit(1)
	}

	deps, err := app.WireMinimal(cfg)
	if err != nil {
		log.Errorf("wire failed: %v", err)
		os.Exit(1)
	}

	addr := cfg.DaemonAddr
	if addr == "" {
		addr = "127.0.0.1:8787"
	}

	mux := http.NewServeMux()

	// System info endpoint
	mux.HandleFunc("/api/system/info", func(w http.ResponseWriter, r *http.Request) {
		profile, err := global.GetHWProfile()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		info := map[string]any{
			"os":               runtime.GOOS,
			"arch":             runtime.GOARCH,
			"cpu_count":        profile.CPUCount,
			"mem_total_mb":     profile.MemTotalMB,
			"disk_total_gb":    profile.DiskTotalGB,
			"disk_free_gb":     profile.DiskFreeGB,
			"has_accel":        profile.HasAccel,
			"accel_kind":       profile.AccelKind,
			"tier":             profile.Tier(),
			"can_run_local":    profile.CanRunLocalLLM(),
			"recommended_size": profile.RecommendedModelSize(),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	})

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	// Providers endpoint
	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, r *http.Request) {
		providers := deps.ProviderRegistry.List()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(providers)
	})

	// OAuth authorize endpoint (starts OAuth flow)
	mux.HandleFunc("/oauth/authorize/", func(w http.ResponseWriter, r *http.Request) {
		// Extract provider from path
		providerName := r.URL.Path[len("/oauth/authorize/"):]
		if providerName == "" {
			http.Error(w, "missing provider", http.StatusBadRequest)
			return
		}

		// Get provider-specific OAuth URLs
		var authURL string
		var envVar string

		switch providerName {
		case "github":
			clientID := os.Getenv("GITHUB_CLIENT_ID")
			if clientID == "" {
				http.Error(w, "GITHUB_CLIENT_ID not set", http.StatusBadRequest)
				return
			}
			authURL = fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=http://%s/oauth/callback/github&scope=repo,user", clientID, addr)
			envVar = "GITHUB_CLIENT_ID"

		case "google":
			clientID := os.Getenv("GOOGLE_CLIENT_ID")
			if clientID == "" {
				http.Error(w, "GOOGLE_CLIENT_ID not set", http.StatusBadRequest)
				return
			}
			authURL = fmt.Sprintf("https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=http://%s/oauth/callback/google&response_type=code&scope=openid%%20email%%20profile", clientID, addr)
			envVar = "GOOGLE_CLIENT_ID"

		default:
			// For providers without OAuth, redirect to API key page
			var apiKeyURL string
			switch providerName {
			case "anthropic":
				apiKeyURL = "https://console.anthropic.com/settings/keys"
			case "openai":
				apiKeyURL = "https://platform.openai.com/api-keys"
			case "openrouter":
				apiKeyURL = "https://openrouter.ai/keys"
			case "groq":
				apiKeyURL = "https://console.groq.com/keys"
			default:
				http.Error(w, "unknown provider: "+providerName, http.StatusBadRequest)
				return
			}

			resp := map[string]string{
				"type":    "api_key",
				"url":     apiKeyURL,
				"message": fmt.Sprintf("Get your API key from %s and set the environment variable", apiKeyURL),
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		state := fmt.Sprintf("devorch_%d", time.Now().UnixNano())
		resp := map[string]string{
			"type":     "oauth",
			"auth_url": authURL,
			"state":    state,
			"env_var":  envVar,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// OAuth callback endpoint
	mux.HandleFunc("/oauth/callback/", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Path[len("/oauth/callback/"):]
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		if code == "" {
			http.Error(w, "missing authorization code", http.StatusBadRequest)
			return
		}

		// Verify state has our prefix
		if state != "" && len(state) >= 8 && state[:8] != "devorch_" {
			http.Error(w, "invalid state parameter", http.StatusBadRequest)
			return
		}

		// Token exchange based on provider
		var tokenURL string
		var clientID, clientSecret string
		var formData map[string]string

		switch provider {
		case "github":
			clientID = os.Getenv("GITHUB_CLIENT_ID")
			clientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
			if clientID == "" || clientSecret == "" {
				http.Error(w, "GitHub OAuth not configured (missing GITHUB_CLIENT_ID or GITHUB_CLIENT_SECRET)", http.StatusBadRequest)
				return
			}
			tokenURL = "https://github.com/login/oauth/access_token"
			formData = map[string]string{
				"client_id":     clientID,
				"client_secret": clientSecret,
				"code":          code,
			}

		case "google":
			clientID = os.Getenv("GOOGLE_CLIENT_ID")
			clientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
			if clientID == "" || clientSecret == "" {
				http.Error(w, "Google OAuth not configured (missing GOOGLE_CLIENT_ID or GOOGLE_CLIENT_SECRET)", http.StatusBadRequest)
				return
			}
			tokenURL = "https://oauth2.googleapis.com/token"
			formData = map[string]string{
				"client_id":     clientID,
				"client_secret": clientSecret,
				"code":          code,
				"grant_type":    "authorization_code",
				"redirect_uri":  fmt.Sprintf("http://%s/oauth/callback/google", addr),
			}

		default:
			http.Error(w, "OAuth callback not supported for provider: "+provider, http.StatusBadRequest)
			return
		}

		// Make token exchange request
		form := make([]string, 0, len(formData))
		for k, v := range formData {
			form = append(form, fmt.Sprintf("%s=%s", k, v))
		}
		reqBody := strings.NewReader(strings.Join(form, "&"))

		req, err := http.NewRequest("POST", tokenURL, reqBody)
		if err != nil {
			http.Error(w, "failed to create token request: "+err.Error(), http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "token exchange failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		var tokenResp map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			http.Error(w, "failed to parse token response: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Check for error in response
		if errMsg, ok := tokenResp["error"].(string); ok {
			errDesc, _ := tokenResp["error_description"].(string)
			http.Error(w, fmt.Sprintf("OAuth error: %s - %s", errMsg, errDesc), http.StatusBadRequest)
			return
		}

		// Success response with HTML that shows the token
		accessToken, _ := tokenResp["access_token"].(string)
		tokenType, _ := tokenResp["token_type"].(string)
		if tokenType == "" {
			tokenType = "Bearer"
		}

		// Return success page with instructions
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>OAuth Success - DevOrch</title></head>
<body style="font-family: system-ui; padding: 2rem; max-width: 600px; margin: 0 auto;">
<h1>✅ Authentication Successful!</h1>
<p>Provider: <strong>%s</strong></p>
<p>Your access token has been received. To use it with DevOrch:</p>
<pre style="background: #f0f0f0; padding: 1rem; border-radius: 4px; overflow-x: auto;">export %s_TOKEN=%s</pre>
<p>You can close this window now.</p>
</body>
</html>`, provider, strings.ToUpper(provider), accessToken)

		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	})

	// OAuth status endpoint
	mux.HandleFunc("/oauth/status/", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Path[len("/oauth/status/"):]
		resp := map[string]any{
			"provider":      provider,
			"authenticated": false,
			"message":       "OAuth status check. No tokens stored yet.",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// OAuth logout endpoint
	mux.HandleFunc("/oauth/logout/", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Path[len("/oauth/logout/"):]
		resp := map[string]string{
			"provider": provider,
			"status":   "logged_out",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// OkAON stats endpoint
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		runs, err := deps.OkAONStore.QueryRuns(ctx, "", "", 50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(runs)
	})

	// Static files (Web UI) - must be last
	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Errorf("failed to load static files: %v", err)
		os.Exit(1)
	}
	mux.Handle("/", http.FileServer(http.FS(staticContent)))

	log.Infof("devorchd listening on %s", addr)
	log.Infof("Web UI available at http://%s/", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
