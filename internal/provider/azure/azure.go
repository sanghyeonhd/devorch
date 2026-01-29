// Package azure provides Azure OpenAI provider implementation
package azure

import (
	"context"
	"fmt"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

// Azure implements the Azure OpenAI provider
type Azure struct {
	c          *openai_compat.Client
	endpoint   string
	deployment string
	apiVersion string
}

// Config holds Azure OpenAI configuration
type Config struct {
	Endpoint   string // e.g., https://your-resource.openai.azure.com
	APIKey     string
	Deployment string // deployment name
	APIVersion string // e.g., 2024-02-01
}

// New creates a new Azure OpenAI provider
func New(cfg Config) *Azure {
	if cfg.APIVersion == "" {
		cfg.APIVersion = "2024-02-01"
	}

	// Azure OpenAI uses different URL structure and header
	baseURL := fmt.Sprintf("%s/openai/deployments/%s", cfg.Endpoint, cfg.Deployment)

	headers := map[string]string{
		"api-key": cfg.APIKey,
	}

	// Create client with empty API key since we use api-key header
	c := openai_compat.New(baseURL, "", headers)

	return &Azure{
		c:          c,
		endpoint:   cfg.Endpoint,
		deployment: cfg.Deployment,
		apiVersion: cfg.APIVersion,
	}
}

func (p *Azure) Name() string     { return "azure" }
func (p *Azure) Kind() string     { return "remote" }
func (p *Azure) Endpoint() string { return p.endpoint }

func (p *Azure) Health(ctx context.Context) error {
	// Azure doesn't have a models endpoint in the same way
	// Try a simple completion to check health
	req := provider.ChatRequest{
		Model: p.deployment,
		Messages: []provider.ChatMessage{
			{Role: "user", Content: "ping"},
		},
		MaxTokens: 5,
	}
	_, err := p.Chat(ctx, req)
	return err
}

func (p *Azure) ListModels(ctx context.Context) ([]provider.Model, error) {
	// Azure OpenAI doesn't list models the same way
	// Return the deployment as the available model
	return []provider.Model{
		{ID: p.deployment},
	}, nil
}

func (p *Azure) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	// Azure uses deployment name instead of model
	req.Model = p.deployment
	return p.c.ChatWithQueryParam(ctx, req, "api-version", p.apiVersion)
}
