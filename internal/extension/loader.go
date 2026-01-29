// Package extension provides functionality to load custom tools and agents
// from the user's ~/.devorch directory.
package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"devorch/internal/tool"
)

// Extension represents a loaded extension (custom tool or agent)
type Extension struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"` // "tool" or "agent"
	Path        string            `json:"path"`
	Command     string            `json:"command"` // execution command (e.g., "python", "node", "bash")
	Parameters  []ParameterDef    `json:"parameters"`
	Metadata    map[string]string `json:"metadata"`
}

// ParameterDef defines a parameter for an extension
type ParameterDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "number", "boolean", "array", "object"
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
}

// Loader manages loading and executing extensions
type Loader struct {
	baseDir    string
	extensions map[string]*Extension
	mu         sync.RWMutex
}

// NewLoader creates a new extension loader
func NewLoader() (*Loader, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	baseDir := filepath.Join(homeDir, ".devorch")

	loader := &Loader{
		baseDir:    baseDir,
		extensions: make(map[string]*Extension),
	}

	return loader, nil
}

// BaseDir returns the base directory for extensions
func (l *Loader) BaseDir() string {
	return l.baseDir
}

// Initialize creates the necessary directories if they don't exist
func (l *Loader) Initialize() error {
	dirs := []string{
		l.baseDir,
		filepath.Join(l.baseDir, "tools"),
		filepath.Join(l.baseDir, "agents"),
		filepath.Join(l.baseDir, "config"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create example extension manifest if it doesn't exist
	examplePath := filepath.Join(l.baseDir, "tools", "example.json")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		example := &Extension{
			Name:        "example",
			Description: "Example custom tool - edit this or create your own",
			Type:        "tool",
			Path:        filepath.Join(l.baseDir, "tools", "example.sh"),
			Command:     "bash",
			Parameters: []ParameterDef{
				{
					Name:        "input",
					Type:        "string",
					Description: "Input to process",
					Required:    true,
				},
			},
			Metadata: map[string]string{
				"author":  "DevOrch",
				"version": "1.0.0",
			},
		}

		data, err := json.MarshalIndent(example, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal example: %w", err)
		}

		if err := os.WriteFile(examplePath, data, 0644); err != nil {
			return fmt.Errorf("failed to write example: %w", err)
		}

		// Create example script
		scriptPath := filepath.Join(l.baseDir, "tools", "example.sh")
		script := `#!/bin/bash
# Example custom tool script
# This script receives arguments as JSON via stdin

# Read input JSON
INPUT=$(cat)

# Parse the "input" parameter using jq or simple grep
VALUE=$(echo "$INPUT" | grep -o '"input":"[^"]*"' | cut -d'"' -f4)

# Process and output result
echo "Processed: $VALUE"
`
		if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
			return fmt.Errorf("failed to write example script: %w", err)
		}
	}

	return nil
}

// LoadAll loads all extensions from the tools and agents directories
func (l *Loader) LoadAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Clear existing extensions
	l.extensions = make(map[string]*Extension)

	// Load tools
	toolsDir := filepath.Join(l.baseDir, "tools")
	if err := l.loadFromDir(toolsDir, "tool"); err != nil {
		return fmt.Errorf("failed to load tools: %w", err)
	}

	// Load agents
	agentsDir := filepath.Join(l.baseDir, "agents")
	if err := l.loadFromDir(agentsDir, "agent"); err != nil {
		return fmt.Errorf("failed to load agents: %w", err)
	}

	return nil
}

func (l *Loader) loadFromDir(dir string, extType string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only load JSON manifest files
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		ext, err := l.loadExtension(path)
		if err != nil {
			// Log error but continue loading other extensions
			fmt.Fprintf(os.Stderr, "Warning: failed to load extension %s: %v\n", path, err)
			continue
		}

		ext.Type = extType
		l.extensions[ext.Name] = ext
	}

	return nil
}

func (l *Loader) loadExtension(path string) (*Extension, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var ext Extension
	if err := json.Unmarshal(data, &ext); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate required fields
	if ext.Name == "" {
		return nil, fmt.Errorf("extension name is required")
	}
	if ext.Path == "" {
		return nil, fmt.Errorf("extension path is required")
	}

	// Check if the extension script exists
	if _, err := os.Stat(ext.Path); err != nil {
		return nil, fmt.Errorf("extension script not found: %s", ext.Path)
	}

	return &ext, nil
}

// GetExtension returns an extension by name
func (l *Loader) GetExtension(name string) (*Extension, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	ext, ok := l.extensions[name]
	return ext, ok
}

// ListExtensions returns all loaded extensions
func (l *Loader) ListExtensions() []*Extension {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*Extension, 0, len(l.extensions))
	for _, ext := range l.extensions {
		result = append(result, ext)
	}
	return result
}

// ListTools returns all loaded tool extensions
func (l *Loader) ListTools() []*Extension {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*Extension, 0)
	for _, ext := range l.extensions {
		if ext.Type == "tool" {
			result = append(result, ext)
		}
	}
	return result
}

// ListAgents returns all loaded agent extensions
func (l *Loader) ListAgents() []*Extension {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*Extension, 0)
	for _, ext := range l.extensions {
		if ext.Type == "agent" {
			result = append(result, ext)
		}
	}
	return result
}

// Execute runs an extension with the given arguments
func (l *Loader) Execute(ctx context.Context, name string, args map[string]any) (string, error) {
	ext, ok := l.GetExtension(name)
	if !ok {
		return "", fmt.Errorf("extension not found: %s", name)
	}

	// Validate required parameters
	for _, param := range ext.Parameters {
		if param.Required {
			if _, ok := args[param.Name]; !ok {
				return "", fmt.Errorf("missing required parameter: %s", param.Name)
			}
		}
	}

	// Convert args to JSON
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("failed to marshal arguments: %w", err)
	}

	// Determine the command to run
	var cmd *exec.Cmd
	if ext.Command != "" {
		cmd = exec.CommandContext(ctx, ext.Command, ext.Path)
	} else {
		cmd = exec.CommandContext(ctx, ext.Path)
	}

	// Pass arguments via stdin
	cmd.Stdin = strings.NewReader(string(argsJSON))

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("extension execution failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// ExtensionTool wraps an extension as a tool.Tool
type ExtensionTool struct {
	loader *Loader
	ext    *Extension
}

// NewExtensionTool creates a tool wrapper for an extension
func NewExtensionTool(loader *Loader, ext *Extension) *ExtensionTool {
	return &ExtensionTool{
		loader: loader,
		ext:    ext,
	}
}

// Info returns the tool information
func (t *ExtensionTool) Info() tool.ToolInfo {
	// Build JSON schema for parameters
	properties := make(map[string]any)
	required := make([]string, 0)

	for _, p := range t.ext.Parameters {
		prop := map[string]any{
			"type":        p.Type,
			"description": p.Description,
		}
		if p.Default != nil {
			prop["default"] = p.Default
		}
		properties[p.Name] = prop

		if p.Required {
			required = append(required, p.Name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	schemaJSON, _ := json.Marshal(schema)

	return tool.ToolInfo{
		Name:        "ext_" + t.ext.Name, // Prefix with ext_ to avoid conflicts
		Description: t.ext.Description + " (custom extension)",
		Parameters:  schemaJSON,
	}
}

// Execute runs the extension
func (t *ExtensionTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	// Parse params
	var args map[string]any
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	output, err := t.loader.Execute(ctx.Ctx, t.ext.Name, args)
	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Extension execution failed: %v", err),
			ExitCode: 1,
		}, nil
	}

	return &tool.Result{
		Output:   output,
		ExitCode: 0,
	}, nil
}

// GetExtensionTools returns all extension tools as tool.Tool interfaces
func (l *Loader) GetExtensionTools() []tool.Tool {
	tools := l.ListTools()
	result := make([]tool.Tool, len(tools))

	for i, ext := range tools {
		result[i] = NewExtensionTool(l, ext)
	}

	return result
}
