package xai

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

// Provider implements the xAI (Grok) provider
type Provider struct {
	c *openai_compat.Client
}

// New creates a new xAI provider
func New() *Provider {
	return &Provider{
		c: openai_compat.New(
			"https://api.x.ai/v1",
			"", // API key from XAI_API_KEY env
			nil,
		),
	}
}

func (p *Provider) Name() string     { return "xai" }
func (p *Provider) Kind() string     { return "remote" }
func (p *Provider) Endpoint() string { return "https://api.x.ai/v1" }

func (p *Provider) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *Provider) ListModels(ctx context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{ID: "grok-3"},
		{ID: "grok-3-turbo"},
		{ID: "grok-2"},
		{ID: "grok-2-turbo"},
		{ID: "grok-1"},
	}, nil
}

func (p *Provider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}
