package tool

import "encoding/json"

// ToolInfo defines a tool that can be used by AI agents
type ToolInfo struct {
	// Name is the unique identifier for the tool
	Name string `json:"name"`

	// Description explains what the tool does (for AI to understand)
	Description string `json:"description"`

	// Parameters is JSON schema for the tool's input parameters
	Parameters json.RawMessage `json:"parameters,omitempty"`
}

// Tool is the interface that all tools must implement
type Tool interface {
	// Info returns the tool's metadata
	Info() ToolInfo

	// Execute runs the tool with given parameters
	Execute(ctx *Context, params json.RawMessage) (*Result, error)
}

// Result represents the output of a tool execution
type Result struct {
	// Output is the main output text
	Output string `json:"output"`

	// Title is optional short title for display
	Title string `json:"title,omitempty"`

	// Truncated indicates if output was truncated
	Truncated bool `json:"truncated,omitempty"`

	// ExitCode for command execution (0 = success)
	ExitCode int `json:"exit_code,omitempty"`

	// Metadata for additional structured data
	Metadata map[string]any `json:"metadata,omitempty"`

	// Attachments for binary content (images, PDFs, etc.)
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Meta implements ResultMeta interface
func (r *Result) Meta() map[string]any {
	if r.Metadata == nil {
		return map[string]any{}
	}
	return r.Metadata
}

// Attachment represents binary content attached to a result
type Attachment struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id"`
	Type      string `json:"type"` // "file", "image", etc.
	MIME      string `json:"mime"`
	URL       string `json:"url"` // data: URL or file path
	FileName  string `json:"filename,omitempty"`
}
