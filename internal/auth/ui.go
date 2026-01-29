package auth

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// ProviderInfo contains display information for a provider.
type ProviderInfo struct {
	ID           Provider `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	AuthType     string   `json:"auth_type"` // "oauth", "api_key", "device_flow", "none"
	Icon         string   `json:"icon"`
	RequiresAuth bool     `json:"requires_auth"`
}

// AllProviders returns information about all supported providers.
var AllProviders = []ProviderInfo{
	{
		ID:           ProviderAnthropic,
		Name:         "Anthropic",
		Description:  "Claude models (Sonnet, Opus, Haiku)",
		AuthType:     "api_key",
		Icon:         "🤖",
		RequiresAuth: true,
	},
	{
		ID:           ProviderOpenAI,
		Name:         "OpenAI",
		Description:  "GPT models (GPT-4o, GPT-4)",
		AuthType:     "api_key",
		Icon:         "🧠",
		RequiresAuth: true,
	},
	{
		ID:           ProviderGoogle,
		Name:         "Google",
		Description:  "Gemini models (Pro, Flash)",
		AuthType:     "oauth",
		Icon:         "🔮",
		RequiresAuth: true,
	},
	{
		ID:           ProviderGitHub,
		Name:         "GitHub Copilot",
		Description:  "GitHub Copilot integration",
		AuthType:     "oauth",
		Icon:         "🐙",
		RequiresAuth: true,
	},
	{
		ID:           "azure",
		Name:         "Azure OpenAI",
		Description:  "Azure-hosted OpenAI models",
		AuthType:     "api_key",
		Icon:         "☁️",
		RequiresAuth: true,
	},
	{
		ID:           "ollama",
		Name:         "Ollama",
		Description:  "Local models (no auth required)",
		AuthType:     "none",
		Icon:         "🦙",
		RequiresAuth: false,
	},
	{
		ID:           "openrouter",
		Name:         "OpenRouter",
		Description:  "Multi-provider gateway",
		AuthType:     "api_key",
		Icon:         "🔄",
		RequiresAuth: true,
	},
	{
		ID:           "groq",
		Name:         "Groq",
		Description:  "Ultra-fast inference",
		AuthType:     "api_key",
		Icon:         "⚡",
		RequiresAuth: true,
	},
}

// AuthStatus represents the authentication status for a provider.
type AuthStatus struct {
	Provider      Provider  `json:"provider"`
	Authenticated bool      `json:"authenticated"`
	Username      string    `json:"username,omitempty"`
	Email         string    `json:"email,omitempty"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
	NeedsRefresh  bool      `json:"needs_refresh"`
}

// GetAuthStatus returns the authentication status for a provider.
func (m *Manager) GetAuthStatus(ctx context.Context, p Provider) AuthStatus {
	status := AuthStatus{Provider: p}

	tok, found, err := m.Status(ctx, p)
	if err != nil || !found {
		return status
	}

	status.Authenticated = true
	status.ExpiresAt = tok.Expiry
	status.NeedsRefresh = tok.Expired(5 * time.Minute)

	return status
}

// GetAllAuthStatus returns authentication status for all providers.
func (m *Manager) GetAllAuthStatus(ctx context.Context) []AuthStatus {
	var statuses []AuthStatus
	for _, p := range AllProviders {
		if p.RequiresAuth {
			statuses = append(statuses, m.GetAuthStatus(ctx, p.ID))
		} else {
			statuses = append(statuses, AuthStatus{
				Provider:      p.ID,
				Authenticated: true, // No auth required
			})
		}
	}
	return statuses
}

// DeviceFlowUI handles the device flow authentication with user prompts.
type DeviceFlowUI struct {
	Manager *Manager
}

// StartDeviceFlowAuth starts device flow authentication with UI prompts.
func (d *DeviceFlowUI) StartDeviceFlowAuth(ctx context.Context, p Provider) error {
	fmt.Printf("🔐 Starting authentication for %s...\n\n", p)

	// Begin device flow
	resp, err := d.Manager.BeginDeviceFlow(ctx, p)
	if err != nil {
		return fmt.Errorf("failed to start device flow: %w", err)
	}

	// Show user code
	fmt.Printf("Please visit: %s\n", resp.VerificationURI)
	fmt.Printf("And enter code: %s\n\n", resp.UserCode)

	// Try to open browser
	if resp.VerificationURIComplete != "" {
		fmt.Println("Opening browser...")
		openBrowser(resp.VerificationURIComplete)
	} else {
		openBrowser(resp.VerificationURI)
	}

	fmt.Println("Waiting for authorization...")

	// Poll for completion
	tok, err := d.Manager.PollDeviceFlow(ctx, p, resp.DeviceCode, resp.Interval)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Save token
	ws := d.Manager.Workspace()
	if err := d.Manager.Store.Put(ctx, ws, p, tok); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	fmt.Println("✓ Authentication successful!")
	return nil
}

// openBrowser opens the specified URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

// APIKeyAuth handles API key authentication.
type APIKeyAuth struct {
	Store     TokenStore
	Workspace func() string
}

// SetAPIKey sets an API key for a provider.
func (a *APIKeyAuth) SetAPIKey(ctx context.Context, p Provider, apiKey string) error {
	ws := a.Workspace()
	tok := Token{
		AccessToken: apiKey,
		TokenType:   "Bearer",
	}
	return a.Store.Put(ctx, ws, p, tok)
}

// GetAPIKey retrieves the API key for a provider.
func (a *APIKeyAuth) GetAPIKey(ctx context.Context, p Provider) (string, bool, error) {
	ws := a.Workspace()
	tok, found, err := a.Store.Get(ctx, ws, p)
	if err != nil || !found {
		return "", false, err
	}
	return tok.AccessToken, true, nil
}

// FormatAuthStatusTable formats auth status as a table string.
func FormatAuthStatusTable(statuses []AuthStatus) string {
	result := "Provider         Status          Expires\n"
	result += "───────────────  ──────────────  ────────────────\n"

	for _, s := range statuses {
		providerInfo := GetProviderInfo(s.Provider)
		status := "❌ Not authenticated"
		expires := "-"

		if s.Authenticated {
			if s.NeedsRefresh {
				status = "⚠️  Needs refresh"
			} else {
				status = "✅ Authenticated"
			}
			if !s.ExpiresAt.IsZero() {
				expires = s.ExpiresAt.Format("2006-01-02 15:04")
			}
		}

		result += fmt.Sprintf("%-16s %-15s %s\n",
			providerInfo.Name, status, expires)
	}

	return result
}

// GetProviderInfo returns info for a provider by ID.
func GetProviderInfo(p Provider) ProviderInfo {
	for _, info := range AllProviders {
		if info.ID == p {
			return info
		}
	}
	return ProviderInfo{ID: p, Name: string(p)}
}
