package bedrock

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

// Provider implements the AWS Bedrock provider
type Provider struct {
	c *openai_compat.Client
}

// New creates a new AWS Bedrock provider
func New() *Provider {
	// Bedrock uses AWS credentials from environment
	// AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION
	return &Provider{
		c: openai_compat.New(
			"https://bedrock-runtime.us-east-1.amazonaws.com",
			"", // AWS credentials from env
			nil,
		),
	}
}

func (p *Provider) Name() string     { return "bedrock" }
func (p *Provider) Kind() string     { return "remote" }
func (p *Provider) Endpoint() string { return "https://bedrock-runtime.amazonaws.com" }

func (p *Provider) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *Provider) ListModels(ctx context.Context) ([]provider.Model, error) {
	return []provider.Model{
		{ID: "anthropic.claude-3-5-sonnet-20240620-v1:0"},
		{ID: "anthropic.claude-3-opus-20240229-v1:0"},
		{ID: "anthropic.claude-3-sonnet-20240229-v1:0"},
		{ID: "anthropic.claude-3-haiku-20240307-v1:0"},
		{ID: "amazon.titan-text-express-v1"},
		{ID: "amazon.titan-text-lite-v1"},
		{ID: "meta.llama3-70b-instruct-v1:0"},
		{ID: "meta.llama3-8b-instruct-v1:0"},
		{ID: "mistral.mistral-7b-instruct-v0:2"},
		{ID: "mistral.mixtral-8x7b-instruct-v0:1"},
	}, nil
}

func (p *Provider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}
