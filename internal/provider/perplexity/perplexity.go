package perplexity

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

// Provider implements the Perplexity AI provider
type Provider struct {
	c *openai_compat.Client
}

// New creates a new Perplexity provider
func New() *Provider {
	return &Provider{
		c: openai_compat.New(
			"https://api.perplexity.ai",
			"", // API key from PERPLEXITY_API_KEY env
			nil,
		),
	}
}

func (p *Provider) Name() string     { return "perplexity" }
func (p *Provider) Kind() string     { return "remote" }
func (p *Provider) Endpoint() string { return "https://api.perplexity.ai" }

func (p *Provider) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *Provider) ListModels(ctx context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{ID: "llama-3.1-sonar-small-128k-online"},
		{ID: "llama-3.1-sonar-large-128k-online"},
		{ID: "llama-3.1-sonar-huge-128k-online"},
		{ID: "sonar"},
		{ID: "sonar-pro"},
	}, nil
}

func (p *Provider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}
