package anthropic

import (
	"context"
	"os"

	"devorch/internal/auth"
	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

// Provider implements the Anthropic Claude provider
type Provider struct {
	c *openai_compat.Client
}

// New creates a new Anthropic provider
func New() *Provider {
	// Get API key from auth store first, fallback to environment variable
	var apiKey string
	store := auth.GetStore()
	if authInfo := store.Get("anthropic"); authInfo != nil {
		// OAuth token takes precedence over API key
		if authInfo.AccessToken != "" {
			apiKey = authInfo.AccessToken
		} else if authInfo.APIKey != "" {
			apiKey = authInfo.APIKey
		}
	} else {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	return &Provider{
		c: openai_compat.New(
			"https://api.anthropic.com/v1",
			apiKey,
			map[string]string{
				"anthropic-version": "2023-06-01",
			},
		),
	}
}

func (p *Provider) Name() string     { return "anthropic" }
func (p *Provider) Kind() string     { return "remote" }
func (p *Provider) Endpoint() string { return "https://api.anthropic.com/v1" }

func (p *Provider) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *Provider) ListModels(ctx context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{ID: "claude-3-opus-20240229"},
		{ID: "claude-3-sonnet-20240229"},
		{ID: "claude-3-haiku-20240307"},
		{ID: "claude-3-5-sonnet-20240620"},
		{ID: "claude-3-5-sonnet-20241022"},
		{ID: "claude-3-5-haiku-20241022"},
		{ID: "claude-sonnet-4-20250514"},
		{ID: "claude-opus-4-20250514"},
	}, nil
}

func (p *Provider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}
