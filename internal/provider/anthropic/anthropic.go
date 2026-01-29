package anthropic

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

// Provider implements the Anthropic Claude provider
type Provider struct {
	c *openai_compat.Client
}

// New creates a new Anthropic provider
func New() *Provider {
	return &Provider{
		c: openai_compat.New(
			"https://api.anthropic.com/v1",
			"", // API key from ANTHROPIC_API_KEY env
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
