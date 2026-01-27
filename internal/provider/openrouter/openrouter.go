package openrouter

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

type OpenRouter struct {
	c *openai_compat.Client
}

func New(apiKey string) *OpenRouter {
	// OpenRouter is OpenAI-compatible, but requires extra headers in many cases
	headers := map[string]string{
		"HTTP-Referer": "http://localhost",
		"X-Title":      "devorch",
	}
	return &OpenRouter{
		c: openai_compat.New("https://openrouter.ai/api/v1", apiKey, headers),
	}
}

func (p *OpenRouter) Name() string     { return "openrouter" }
func (p *OpenRouter) Kind() string     { return "remote" }
func (p *OpenRouter) Endpoint() string { return "https://openrouter.ai/api/v1" }

func (p *OpenRouter) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *OpenRouter) ListModels(ctx context.Context) ([]provider.Model, error) {
	return p.c.ListModels(ctx)
}

func (p *OpenRouter) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}
