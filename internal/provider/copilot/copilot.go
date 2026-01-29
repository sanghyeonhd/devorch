package copilot

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

// Provider implements the GitHub Copilot provider
type Provider struct {
	c *openai_compat.Client
}

// New creates a new GitHub Copilot provider
func New() *Provider {
	return &Provider{
		c: openai_compat.New(
			"https://api.githubcopilot.com",
			"", // Token from GITHUB_TOKEN or COPILOT_TOKEN env
			nil,
		),
	}
}

func (p *Provider) Name() string     { return "copilot" }
func (p *Provider) Kind() string     { return "remote" }
func (p *Provider) Endpoint() string { return "https://api.githubcopilot.com" }

func (p *Provider) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *Provider) ListModels(ctx context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{ID: "gpt-4o"},
		{ID: "gpt-4o-mini"},
		{ID: "gpt-4-turbo"},
		{ID: "claude-3.5-sonnet"},
		{ID: "o1-preview"},
	}, nil
}

func (p *Provider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}
