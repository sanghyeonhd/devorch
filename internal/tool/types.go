package tool

import (
	"encoding/json"
)

// ParameterInfo describes a tool parameter
type ParameterInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}

// ToolInfo defines a tool that can be used by AI agents
type ToolInfo struct {
	// Name is the unique identifier for the tool
	Name string `json:"name"`

	// Description explains what the tool does (for AI to understand)
	Description string `json:"description"`

	// Category groups similar tools together
	Category string `json:"category,omitempty"`

	// Version of the tool
	Version string `json:"version,omitempty"`

	// Parameters can be either JSON schema or structured parameter list
	Parameters     json.RawMessage `json:"parameters,omitempty"`
	ParameterInfos []ParameterInfo `json:"parameter_infos,omitempty"`
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
