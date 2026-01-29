// Package command provides custom command management (Ctrl+K style)
package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Type represents the command type
type Type string

const (
	TypeShell  Type = "shell"  // Shell command
	TypePrompt Type = "prompt" // Prompt template
	TypeMacro  Type = "macro"  // Multi-step macro
	TypeScript Type = "script" // Script file
)

// Command represents a custom command
type Command struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        Type              `json:"type"`
	Content     string            `json:"content"`            // Command content/template
	Args        []ArgDef          `json:"args,omitempty"`     // Argument definitions
	Shortcut    string            `json:"shortcut,omitempty"` // Keyboard shortcut
	Tags        []string          `json:"tags,omitempty"`
	Hidden      bool              `json:"hidden,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	UsageCount  int               `json:"usage_count"`
	LastUsed    *time.Time        `json:"last_used,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ArgDef defines a command argument
type ArgDef struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Required    bool     `json:"required"`
	Default     string   `json:"default,omitempty"`
	Type        string   `json:"type,omitempty"`    // string, int, bool, file, dir
	Options     []string `json:"options,omitempty"` // For enum types
}

// ExecutionResult represents the result of command execution
type ExecutionResult struct {
	CommandID string        `json:"command_id"`
	Output    string        `json:"output"`
	Error     string        `json:"error,omitempty"`
	ExitCode  int           `json:"exit_code"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// Manager manages custom commands
type Manager struct {
	commands   map[string]*Command
	commandsMu sync.RWMutex
	configPath string
	executor   func(ctx context.Context, cmd *Command, args map[string]string) (*ExecutionResult, error)
}

// NewManager creates a new command manager
func NewManager(configPath string) (*Manager, error) {
	if configPath == "" {
		homeDir, _ := os.UserHomeDir()
		configPath = filepath.Join(homeDir, ".devorch", "commands.json")
	}

	m := &Manager{
		commands:   make(map[string]*Command),
		configPath: configPath,
	}

	// Load existing commands
	if err := m.Load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Add built-in commands
	m.addBuiltinCommands()

	return m, nil
}

func (m *Manager) addBuiltinCommands() {
	builtins := []*Command{
		{
			ID:          "git-status",
			Name:        "Git Status",
			Description: "Show git repository status",
			Type:        TypeShell,
			Content:     "git status",
			Tags:        []string{"git", "status"},
		},
		{
			ID:          "git-diff",
			Name:        "Git Diff",
			Description: "Show unstaged changes",
			Type:        TypeShell,
			Content:     "git diff",
			Tags:        []string{"git", "diff"},
		},
		{
			ID:          "git-log",
			Name:        "Git Log",
			Description: "Show recent commits",
			Type:        TypeShell,
			Content:     "git log --oneline -n {{count}}",
			Args: []ArgDef{
				{Name: "count", Description: "Number of commits", Default: "10"},
			},
			Tags: []string{"git", "log"},
		},
		{
			ID:          "find-files",
			Name:        "Find Files",
			Description: "Find files by pattern",
			Type:        TypeShell,
			Content:     "find . -name '{{pattern}}' -type f",
			Args: []ArgDef{
				{Name: "pattern", Description: "File pattern", Required: true},
			},
			Tags: []string{"find", "files"},
		},
		{
			ID:          "search-code",
			Name:        "Search Code",
			Description: "Search for text in code files",
			Type:        TypeShell,
			Content:     "grep -rn '{{query}}' --include='*.{{ext}}'",
			Args: []ArgDef{
				{Name: "query", Description: "Search query", Required: true},
				{Name: "ext", Description: "File extension", Default: "*"},
			},
			Tags: []string{"search", "grep"},
		},
		{
			ID:          "explain-code",
			Name:        "Explain Code",
			Description: "Ask AI to explain selected code",
			Type:        TypePrompt,
			Content:     "Please explain the following code:\n\n```\n{{code}}\n```",
			Args: []ArgDef{
				{Name: "code", Description: "Code to explain", Required: true},
			},
			Tags: []string{"ai", "explain"},
		},
		{
			ID:          "refactor",
			Name:        "Refactor Code",
			Description: "Ask AI to refactor code",
			Type:        TypePrompt,
			Content:     "Please refactor the following code to be more {{style}}:\n\n```\n{{code}}\n```",
			Args: []ArgDef{
				{Name: "code", Description: "Code to refactor", Required: true},
				{Name: "style", Description: "Refactoring style", Default: "clean and maintainable"},
			},
			Tags: []string{"ai", "refactor"},
		},
		{
			ID:          "add-tests",
			Name:        "Add Tests",
			Description: "Ask AI to generate tests",
			Type:        TypePrompt,
			Content:     "Please generate unit tests for the following code:\n\n```\n{{code}}\n```",
			Args: []ArgDef{
				{Name: "code", Description: "Code to test", Required: true},
			},
			Tags: []string{"ai", "test"},
		},
		{
			ID:          "fix-error",
			Name:        "Fix Error",
			Description: "Ask AI to fix an error",
			Type:        TypePrompt,
			Content:     "I'm getting the following error:\n\n```\n{{error}}\n```\n\nIn this code:\n\n```\n{{code}}\n```\n\nPlease help me fix it.",
			Args: []ArgDef{
				{Name: "error", Description: "Error message", Required: true},
				{Name: "code", Description: "Related code", Required: true},
			},
			Tags: []string{"ai", "debug"},
		},
		{
			ID:          "document",
			Name:        "Add Documentation",
			Description: "Ask AI to add documentation",
			Type:        TypePrompt,
			Content:     "Please add comprehensive documentation to the following code:\n\n```\n{{code}}\n```",
			Args: []ArgDef{
				{Name: "code", Description: "Code to document", Required: true},
			},
			Tags: []string{"ai", "docs"},
		},
	}

	m.commandsMu.Lock()
	defer m.commandsMu.Unlock()

	now := time.Now()
	for _, cmd := range builtins {
		if _, exists := m.commands[cmd.ID]; !exists {
			cmd.CreatedAt = now
			cmd.UpdatedAt = now
			m.commands[cmd.ID] = cmd
		}
	}
}

// Add adds a new command
func (m *Manager) Add(cmd *Command) error {
	if cmd.ID == "" {
		cmd.ID = generateCommandID(cmd.Name)
	}

	m.commandsMu.Lock()
	defer m.commandsMu.Unlock()

	if _, exists := m.commands[cmd.ID]; exists {
		return fmt.Errorf("command already exists: %s", cmd.ID)
	}

	now := time.Now()
	cmd.CreatedAt = now
	cmd.UpdatedAt = now

	m.commands[cmd.ID] = cmd

	return m.Save()
}

// Update updates an existing command
func (m *Manager) Update(cmd *Command) error {
	m.commandsMu.Lock()
	defer m.commandsMu.Unlock()

	if _, exists := m.commands[cmd.ID]; !exists {
		return fmt.Errorf("command not found: %s", cmd.ID)
	}

	cmd.UpdatedAt = time.Now()
	m.commands[cmd.ID] = cmd

	return m.Save()
}

// Delete deletes a command
func (m *Manager) Delete(id string) error {
	m.commandsMu.Lock()
	defer m.commandsMu.Unlock()

	if _, exists := m.commands[id]; !exists {
		return fmt.Errorf("command not found: %s", id)
	}

	delete(m.commands, id)

	return m.Save()
}

// Get retrieves a command
func (m *Manager) Get(id string) (*Command, bool) {
	m.commandsMu.RLock()
	defer m.commandsMu.RUnlock()

	cmd, ok := m.commands[id]
	return cmd, ok
}

// List returns all commands
func (m *Manager) List() []*Command {
	m.commandsMu.RLock()
	defer m.commandsMu.RUnlock()

	result := make([]*Command, 0, len(m.commands))
	for _, cmd := range m.commands {
		if !cmd.Hidden {
			result = append(result, cmd)
		}
	}

	return result
}

// Search searches for commands
func (m *Manager) Search(query string) []*Command {
	m.commandsMu.RLock()
	defer m.commandsMu.RUnlock()

	query = strings.ToLower(query)
	var result []*Command

	for _, cmd := range m.commands {
		if cmd.Hidden {
			continue
		}

		// Search in name, description, and tags
		if strings.Contains(strings.ToLower(cmd.Name), query) ||
			strings.Contains(strings.ToLower(cmd.Description), query) {
			result = append(result, cmd)
			continue
		}

		for _, tag := range cmd.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				result = append(result, cmd)
				break
			}
		}
	}

	return result
}

// GetByTag returns commands with a specific tag
func (m *Manager) GetByTag(tag string) []*Command {
	m.commandsMu.RLock()
	defer m.commandsMu.RUnlock()

	tag = strings.ToLower(tag)
	var result []*Command

	for _, cmd := range m.commands {
		if cmd.Hidden {
			continue
		}

		for _, t := range cmd.Tags {
			if strings.ToLower(t) == tag {
				result = append(result, cmd)
				break
			}
		}
	}

	return result
}

// GetByShortcut returns a command by shortcut
func (m *Manager) GetByShortcut(shortcut string) (*Command, bool) {
	m.commandsMu.RLock()
	defer m.commandsMu.RUnlock()

	for _, cmd := range m.commands {
		if cmd.Shortcut == shortcut {
			return cmd, true
		}
	}

	return nil, false
}

// Execute executes a command
func (m *Manager) Execute(ctx context.Context, id string, args map[string]string) (*ExecutionResult, error) {
	cmd, ok := m.Get(id)
	if !ok {
		return nil, fmt.Errorf("command not found: %s", id)
	}

	// Validate required arguments
	for _, arg := range cmd.Args {
		if arg.Required {
			if _, ok := args[arg.Name]; !ok {
				if arg.Default != "" {
					args[arg.Name] = arg.Default
				} else {
					return nil, fmt.Errorf("missing required argument: %s", arg.Name)
				}
			}
		} else if arg.Default != "" {
			if _, ok := args[arg.Name]; !ok {
				args[arg.Name] = arg.Default
			}
		}
	}

	// Update usage stats
	m.commandsMu.Lock()
	cmd.UsageCount++
	now := time.Now()
	cmd.LastUsed = &now
	m.commandsMu.Unlock()

	// Execute based on type
	if m.executor != nil {
		return m.executor(ctx, cmd, args)
	}

	return nil, fmt.Errorf("no executor configured")
}

// SetExecutor sets the command executor function
func (m *Manager) SetExecutor(executor func(ctx context.Context, cmd *Command, args map[string]string) (*ExecutionResult, error)) {
	m.executor = executor
}

// RenderContent renders command content with arguments
func RenderContent(content string, args map[string]string) string {
	result := content
	for k, v := range args {
		placeholder := "{{" + k + "}}"
		result = strings.ReplaceAll(result, placeholder, v)
	}
	return result
}

// Load loads commands from disk
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var commands map[string]*Command
	if err := json.Unmarshal(data, &commands); err != nil {
		return err
	}

	m.commandsMu.Lock()
	defer m.commandsMu.Unlock()

	for k, v := range commands {
		m.commands[k] = v
	}

	return nil
}

// Save saves commands to disk
func (m *Manager) Save() error {
	m.commandsMu.RLock()
	defer m.commandsMu.RUnlock()

	data, err := json.MarshalIndent(m.commands, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// Export exports commands to a file
func (m *Manager) Export(path string) error {
	m.commandsMu.RLock()
	defer m.commandsMu.RUnlock()

	data, err := json.MarshalIndent(m.commands, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Import imports commands from a file
func (m *Manager) Import(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var commands map[string]*Command
	if err := json.Unmarshal(data, &commands); err != nil {
		return err
	}

	m.commandsMu.Lock()
	defer m.commandsMu.Unlock()

	for k, v := range commands {
		m.commands[k] = v
	}

	return m.Save()
}

func generateCommandID(name string) string {
	// Convert name to kebab-case ID
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, "_", "-")
	return id
}
