// Package together provides Together AI provider implementation
package together

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

const (
	defaultBaseURL = "https://api.together.xyz/v1"
)

// Together implements the Together AI provider
// Together AI offers various open-source models
type Together struct {
	c *openai_compat.Client
}

// New creates a new Together AI provider
func New(apiKey string) *Together {
	return &Together{
		c: openai_compat.New(defaultBaseURL, apiKey, nil),
	}
}

func (p *Together) Name() string     { return "together" }
func (p *Together) Kind() string     { return "remote" }
func (p *Together) Endpoint() string { return defaultBaseURL }

func (p *Together) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *Together) ListModels(ctx context.Context) ([]provider.Model, error) {
	return p.c.ListModels(ctx)
}

func (p *Together) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}

// AvailableModels returns commonly available Together AI models
func AvailableModels() []string {
	return []string{
		// Llama 3 models
		"meta-llama/Llama-3.3-70B-Instruct-Turbo",
		"meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo",
		"meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo",
		"meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo",
		// Mixtral
		"mistralai/Mixtral-8x7B-Instruct-v0.1",
		"mistralai/Mixtral-8x22B-Instruct-v0.1",
		// Qwen
		"Qwen/Qwen2.5-72B-Instruct-Turbo",
		"Qwen/Qwen2.5-7B-Instruct-Turbo",
		// Code models
		"deepseek-ai/DeepSeek-Coder-V2-Instruct",
		"Qwen/Qwen2.5-Coder-32B-Instruct",
	}
}
