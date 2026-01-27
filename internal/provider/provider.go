package provider

import (
	"context"
	"time"
)

type Role string

const (
	RoleSystem Role = "system"
	RoleUser   Role = "user"
	RoleAssist Role = "assistant"
	RoleTool   Role = "tool"
)

type Model struct {
	ID string
}

// ChatMessage legacy support
type ChatMessage struct {
	Role    string // system|user|assistant|tool
	Content string
}

// Message is Phase 6 style
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

type ChatRequest struct {
	Provider    string
	Model       string
	Messages    []ChatMessage
	Temperature float64
	MaxTokens   int
	Stream      bool

	// routing/meta
	Workspace string
	SessionID string
	TaskID    string
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ChatResponse struct {
	RawText          string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CostMicroUSD     int64
	// Phase 6 additions
	Text_     string // Text() now returns this if set
	Model_    string
	Provider_ string
	Usage_    Usage
	ToolCalls []ToolCall
	RawJSON   []byte
}

func (r ChatResponse) Text() string {
	if r.Text_ != "" {
		return r.Text_
	}
	return r.RawText
}

type StreamChunk struct {
	TextDelta string
	Done      bool
}

type BenchResult struct {
	Workspace     string
	SessionID     string
	TaskID        string
	Provider      string
	Model         string
	StartedAt     time.Time
	EndedAt       time.Time
	LatencyMs     int64
	PromptTok     int
	CompletionTok int
	TotalTok      int
	Error         string
}

// ChatClient interface for Phase 6 style providers
type ChatClient interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest, yield func(StreamChunk) error) (ChatResponse, error)
}

type Provider interface {
	Name() string
	Kind() string     // remote|local
	Endpoint() string // base url or local runtime id
	Health(ctx context.Context) error
	ListModels(ctx context.Context) ([]Model, error)
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
