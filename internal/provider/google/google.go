package google

import (
	"context"
	"os"

	"devorch/internal/auth"
	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

// Provider implements the Google AI (Gemini) provider
type Provider struct {
	c *openai_compat.Client
}

// New creates a new Google AI provider
// Reads API key from auth store first, then fallback to GOOGLE_API_KEY environment variable
func New() *Provider {
	var apiKey string

	// Try auth store first
	store := auth.GetStore()
	if authInfo := store.Get("google"); authInfo != nil && authInfo.APIKey != "" {
		apiKey = authInfo.APIKey
	} else {
		// Fallback to environment variable
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}

	return &Provider{
		c: openai_compat.New(
			"https://generativelanguage.googleapis.com/v1beta",
			apiKey,
			map[string]string{
				"x-goog-api-key": apiKey,
			},
		),
	}
}

func (p *Provider) Name() string     { return "google" }
func (p *Provider) Kind() string     { return "remote" }
func (p *Provider) Endpoint() string { return "https://generativelanguage.googleapis.com/v1beta" }

func (p *Provider) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *Provider) ListModels(ctx context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{ID: "gemini-2.0-flash"},
		{ID: "gemini-1.5-pro"},
		{ID: "gemini-1.5-flash"},
		{ID: "gemini-1.0-pro"},
	}, nil
}

func (p *Provider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}
