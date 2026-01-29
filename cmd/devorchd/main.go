package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"devorch/internal/app"
	"devorch/internal/config"
	"devorch/internal/log"
)

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
		provider := r.URL.Path[len("/oauth/authorize/"):]
		if provider == "" {
			http.Error(w, "missing provider", http.StatusBadRequest)
			return
		}

		// For now, return a placeholder response
		// OAuth flow requires proper setup with client IDs
		resp := map[string]string{
			"auth_url": fmt.Sprintf("https://oauth.example.com/authorize?provider=%s&redirect_uri=http://%s/oauth/callback/%s", provider, addr, provider),
			"state":    "placeholder-state",
			"message":  "OAuth is configured but requires client IDs. Set GITHUB_CLIENT_ID, GOOGLE_CLIENT_ID, etc.",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	// OAuth callback endpoint
	mux.HandleFunc("/oauth/callback/", func(w http.ResponseWriter, r *http.Request) {
		provider := r.URL.Path[len("/oauth/callback/"):]
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")

		if code == "" || state == "" {
			http.Error(w, "missing code or state", http.StatusBadRequest)
			return
		}

		// Placeholder: actual token exchange would happen here
		_, _ = w.Write([]byte(fmt.Sprintf("OAuth callback received for %s. Token exchange pending implementation.", provider)))
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

	log.Infof("devorchd listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
