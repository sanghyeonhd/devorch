package openai

import (
	"context"
	"os"

	"devorch/internal/auth"
	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

type OpenAI struct {
	c *openai_compat.Client
}

func New(apiKey string) *OpenAI {
	// If no API key provided, try auth store, then environment variable
	if apiKey == "" {
		store := auth.GetStore()
		if authInfo := store.Get("openai"); authInfo != nil {
			// OAuth token takes precedence over API key
			if authInfo.AccessToken != "" {
				apiKey = authInfo.AccessToken
			} else if authInfo.APIKey != "" {
				apiKey = authInfo.APIKey
			}
		} else {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
	}

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
