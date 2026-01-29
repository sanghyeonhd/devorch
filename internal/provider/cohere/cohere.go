package cohere

import (
	"context"
	"os"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

// Provider implements the Cohere provider
type Provider struct {
	c *openai_compat.Client
}

// New creates a new Cohere provider
// Reads API key from COHERE_API_KEY environment variable
func New() *Provider {
	apiKey := os.Getenv("COHERE_API_KEY")
	return &Provider{
		c: openai_compat.New(
			"https://api.cohere.ai/v1",
			apiKey,
			nil,
		),
	}
}

func (p *Provider) Name() string     { return "cohere" }
func (p *Provider) Kind() string     { return "remote" }
func (p *Provider) Endpoint() string { return "https://api.cohere.ai/v1" }

func (p *Provider) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *Provider) ListModels(ctx context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{ID: "command-r-plus"},
		{ID: "command-r"},
		{ID: "command"},
		{ID: "command-light"},
		{ID: "command-nightly"},
		{ID: "command-r-plus-08-2024"},
		{ID: "command-r-08-2024"},
	}, nil
}

func (p *Provider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}
