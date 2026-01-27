package ollama

import (
	"context"
	"fmt"

	"devorch/internal/provider"
)

type Ollama struct {
	host       string
	binaryPath string // optional bundled binary
}

func New(host string, binaryPath string) *Ollama {
	return &Ollama{host: host, binaryPath: binaryPath}
}

func (p *Ollama) Name() string     { return "ollama" }
func (p *Ollama) Kind() string     { return "local" }
func (p *Ollama) Endpoint() string { return p.host }

func (p *Ollama) Health(ctx context.Context) error {
	return Health(ctx, p.host)
}

func (p *Ollama) ListModels(ctx context.Context) ([]provider.Model, error) {
	tags, err := Tags(ctx, p.host)
	if err != nil {
		return nil, err
	}
	out := make([]provider.Model, 0, len(tags))
	for _, t := range tags {
		out = append(out, provider.Model{ID: t})
	}
	return out, nil
}

func (p *Ollama) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	// Ollama API: /api/chat
	txt, pt, ct, tt, err := Chat(ctx, p.host, req)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	return provider.ChatResponse{
		RawText:          txt,
		PromptTokens:     pt,
		CompletionTokens: ct,
		TotalTokens:      tt,
		CostMicroUSD:     0,
	}, nil
}

func (p *Ollama) Pull(ctx context.Context, model string) error {
	if model == "" {
		return fmt.Errorf("empty model")
	}
	return Pull(ctx, p.host, model)
}
