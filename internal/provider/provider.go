package provider

import "context"

type Model struct {
	ID string
}

type ChatMessage struct {
	Role    string // system|user|assistant|tool
	Content string
}

type ChatRequest struct {
	Model       string
	Messages    []ChatMessage
	Temperature float64
	MaxTokens   int
}

type ChatResponse struct {
	RawText          string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CostMicroUSD     int64
}

func (r ChatResponse) Text() string { return r.RawText }

type Provider interface {
	Name() string
	Kind() string        // remote|local
	Endpoint() string    // base url or local runtime id
	Health(ctx context.Context) error
	ListModels(ctx context.Context) ([]Model, error)
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
