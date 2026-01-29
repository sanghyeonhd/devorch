// Package mistral provides Mistral AI provider implementation
package mistral

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

const (
	defaultBaseURL = "https://api.mistral.ai/v1"
)

// Mistral implements the Mistral AI provider
// Mistral AI is a European AI company offering various models
type Mistral struct {
	c *openai_compat.Client
}

// New creates a new Mistral AI provider
func New(apiKey string) *Mistral {
	return &Mistral{
		c: openai_compat.New(defaultBaseURL, apiKey, nil),
	}
}

func (p *Mistral) Name() string     { return "mistral" }
func (p *Mistral) Kind() string     { return "remote" }
func (p *Mistral) Endpoint() string { return defaultBaseURL }

func (p *Mistral) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *Mistral) ListModels(ctx context.Context) ([]provider.Model, error) {
	return p.c.ListModels(ctx)
}

func (p *Mistral) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}

// AvailableModels returns commonly available Mistral AI models
func AvailableModels() []string {
	return []string{
		// Mistral Large
		"mistral-large-latest",
		"mistral-large-2411",
		// Mistral Small
		"mistral-small-latest",
		"mistral-small-2409",
		// Codestral
		"codestral-latest",
		"codestral-2405",
		// Open models
		"open-mistral-7b",
		"open-mixtral-8x7b",
		"open-mixtral-8x22b",
		// Ministral
		"ministral-3b-latest",
		"ministral-8b-latest",
		// Pixtral (vision)
		"pixtral-12b-2409",
		"pixtral-large-latest",
	}
}
