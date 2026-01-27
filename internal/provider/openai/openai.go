package openai

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

type OpenAI struct {
	c *openai_compat.Client
}

func New(apiKey string) *OpenAI {
	return &OpenAI{
		c: openai_compat.New("https://api.openai.com/v1", apiKey, nil),
	}
}

func (p *OpenAI) Name() string     { return "openai" }
func (p *OpenAI) Kind() string     { return "remote" }
func (p *OpenAI) Endpoint() string { return "https://api.openai.com/v1" }

func (p *OpenAI) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *OpenAI) ListModels(ctx context.Context) ([]provider.Model, error) {
	return p.c.ListModels(ctx)
}

func (p *OpenAI) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}
