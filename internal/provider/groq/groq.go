// Package groq provides Groq AI provider implementation
package groq

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

const (
	defaultBaseURL = "https://api.groq.com/openai/v1"
)

// Groq implements the Groq AI provider
// Groq offers extremely fast inference using custom LPU chips
type Groq struct {
	c *openai_compat.Client
}

// New creates a new Groq provider
func New(apiKey string) *Groq {
	return &Groq{
		c: openai_compat.New(defaultBaseURL, apiKey, nil),
	}
}

func (p *Groq) Name() string     { return "groq" }
func (p *Groq) Kind() string     { return "remote" }
func (p *Groq) Endpoint() string { return defaultBaseURL }

func (p *Groq) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *Groq) ListModels(ctx context.Context) ([]provider.Model, error) {
	return p.c.ListModels(ctx)
}

func (p *Groq) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}

// AvailableModels returns commonly available Groq models
func AvailableModels() []string {
	return []string{
		"llama-3.3-70b-versatile",
		"llama-3.1-8b-instant",
		"llama-3.1-70b-versatile",
		"llama3-8b-8192",
		"llama3-70b-8192",
		"mixtral-8x7b-32768",
		"gemma2-9b-it",
		"gemma-7b-it",
	}
}
