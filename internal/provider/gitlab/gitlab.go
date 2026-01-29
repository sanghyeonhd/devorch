package gitlab

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

// Provider implements the GitLab AI provider
type Provider struct {
	c *openai_compat.Client
}

// New creates a new GitLab provider
func New() *Provider {
	return &Provider{
		c: openai_compat.New(
			"https://gitlab.com/api/v4/ai/completions",
			"", // Token from GITLAB_TOKEN or CI_JOB_TOKEN env
			nil,
		),
	}
}

func (p *Provider) Name() string     { return "gitlab" }
func (p *Provider) Kind() string     { return "remote" }
func (p *Provider) Endpoint() string { return "https://gitlab.com/api/v4/ai/completions" }

func (p *Provider) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *Provider) ListModels(ctx context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{ID: "code-suggestions"},
		{ID: "code-completions"},
		{ID: "chat"},
		{ID: "duo-chat"},
		{ID: "gitlab-ai-gateway"},
	}, nil
}

func (p *Provider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}
