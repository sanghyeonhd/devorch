package hook

import (
	"context"
	"encoding/json"
)

// Hook defines the interface for lifecycle hooks
type Hook interface {
	// Name returns the hook identifier
	Name() string

	// Events returns which events this hook handles
	Events() []EventType

	// Execute runs the hook for the given event
	Execute(ctx context.Context, event EventType, data any) (any, error)
}

// HookFunc is a function adapter for simple hooks
type HookFunc func(ctx context.Context, event EventType, data any) (any, error)

// PreToolUseData contains data for pre tool use event
type PreToolUseData struct {
	SessionID string          `json:"session_id"`
	ToolName  string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
}

// PostToolUseData contains data for post tool use event
type PostToolUseData struct {
	SessionID string          `json:"session_id"`
	ToolName  string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
	Output    any             `json:"output"`
	Error     error           `json:"error,omitempty"`
	Duration  int64           `json:"duration_ms"`
}

// PromptSubmitData contains data for prompt submit event
type PromptSubmitData struct {
	SessionID string `json:"session_id"`
	Prompt    string `json:"prompt"`
	Agent     string `json:"agent,omitempty"`
}

// MessageCreateData contains data for message create event
type MessageCreateData struct {
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id"`
	Role      string `json:"role"` // "user", "assistant", "system"
	Content   string `json:"content"`
}

// SessionStartData contains data for session start event
type SessionStartData struct {
	SessionID   string `json:"session_id"`
	WorkspaceID string `json:"workspace_id"`
	Agent       string `json:"agent,omitempty"`
}

// SessionStopData contains data for session stop event
type SessionStopData struct {
	SessionID string `json:"session_id"`
	Reason    string `json:"reason"` // "complete", "error", "timeout", "user"
}

// ModelSelectData contains data for model selection event
type ModelSelectData struct {
	SessionID     string `json:"session_id"`
	RequestModel  string `json:"request_model"`
	ResolvedModel string `json:"resolved_model"`
	Provider      string `json:"provider"`
	Reason        string `json:"reason,omitempty"`
}

// ResponseStreamData contains data for response streaming event
type ResponseStreamData struct {
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id"`
	Delta     string `json:"delta"`
	Done      bool   `json:"done"`
}
