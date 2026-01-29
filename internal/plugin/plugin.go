// Package plugin provides a plugin system for DevOrch extensions.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Plugin represents an extension to DevOrch.
type Plugin interface {
	// Metadata returns plugin information.
	Metadata() Metadata

	// Initialize initializes the plugin with the host.
	Initialize(ctx context.Context, host Host) error

	// Shutdown gracefully shuts down the plugin.
	Shutdown(ctx context.Context) error
}

// Metadata describes a plugin.
type Metadata struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Version     string         `json:"version"`
	Description string         `json:"description"`
	Author      string         `json:"author"`
	License     string         `json:"license"`
	Homepage    string         `json:"homepage,omitempty"`
	Repository  string         `json:"repository,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Provides    []string       `json:"provides"`           // What the plugin provides: "tool", "provider", "hook", "agent"
	Requires    []string       `json:"requires,omitempty"` // Required dependencies
	Config      map[string]any `json:"config,omitempty"`
}

// Host provides access to DevOrch functionality for plugins.
type Host interface {
	// RegisterTool registers a new tool.
	RegisterTool(name string, handler ToolHandler) error

	// RegisterProvider registers a new LLM provider.
	RegisterProvider(name string, provider LLMProvider) error

	// RegisterHook registers a hook handler.
	RegisterHook(event string, handler HookHandler) error

	// RegisterAgent registers a new agent.
	RegisterAgent(name string, agent Agent) error

	// GetConfig returns the plugin configuration.
	GetConfig(key string) (any, bool)

	// SetConfig sets a configuration value.
	SetConfig(key string, value any)

	// Log logs a message.
	Log(level, message string, args ...any)

	// GetWorkspacePath returns the current workspace path.
	GetWorkspacePath() string

	// GetDataPath returns the plugin's data directory.
	GetDataPath() string
}

// ToolHandler is a function that handles tool calls.
type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

// LLMProvider is an interface for language model providers.
type LLMProvider interface {
	Chat(ctx context.Context, messages []Message, options ChatOptions) (*ChatResponse, error)
	Stream(ctx context.Context, messages []Message, options ChatOptions, callback StreamCallback) error
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatOptions for LLM requests.
type ChatOptions struct {
	Model       string   `json:"model"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	TopP        float64  `json:"top_p,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

// ChatResponse from an LLM.
type ChatResponse struct {
	Content      string `json:"content"`
	FinishReason string `json:"finish_reason"`
	Usage        Usage  `json:"usage"`
}

// Usage token statistics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamCallback for streaming responses.
type StreamCallback func(chunk StreamChunk) error

// StreamChunk represents a streaming chunk.
type StreamChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
}

// HookHandler handles hook events.
type HookHandler func(ctx context.Context, event HookEvent) error

// HookEvent represents a hook event.
type HookEvent struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// Agent represents an AI agent.
type Agent interface {
	Name() string
	Description() string
	Execute(ctx context.Context, task string) (string, error)
}

// PluginManifest is the manifest file for a plugin.
type PluginManifest struct {
	Metadata    Metadata `json:"metadata"`
	EntryPoint  string   `json:"entry_point"` // Main file or function
	Permissions []string `json:"permissions,omitempty"`
}

// LoadManifest loads a plugin manifest from a directory.
func LoadManifest(dir string) (*PluginManifest, error) {
	path := filepath.Join(dir, "plugin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &manifest, nil
}

// SaveManifest saves a plugin manifest to a directory.
func SaveManifest(dir string, manifest *PluginManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "plugin.json")
	return os.WriteFile(path, data, 0644)
}
