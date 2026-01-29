// Package lsp provides a manager for multiple language servers.
package lsp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// Manager manages multiple language server clients.
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*Client // language -> client
	configs map[string]ServerConfig
}

// ServerConfig defines configuration for a language server.
type ServerConfig struct {
	Language string   // e.g., "go", "typescript", "python"
	Command  string   // server executable
	Args     []string // command arguments
}

// DefaultConfigs returns default language server configurations.
var DefaultConfigs = map[string]ServerConfig{
	"go": {
		Language: "go",
		Command:  "gopls",
		Args:     []string{},
	},
	"typescript": {
		Language: "typescript",
		Command:  "typescript-language-server",
		Args:     []string{"--stdio"},
	},
	"javascript": {
		Language: "javascript",
		Command:  "typescript-language-server",
		Args:     []string{"--stdio"},
	},
	"python": {
		Language: "python",
		Command:  "pylsp",
		Args:     []string{},
	},
	"rust": {
		Language: "rust",
		Command:  "rust-analyzer",
		Args:     []string{},
	},
	"c": {
		Language: "c",
		Command:  "clangd",
		Args:     []string{},
	},
	"cpp": {
		Language: "cpp",
		Command:  "clangd",
		Args:     []string{},
	},
}

// NewManager creates a new language server manager.
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		configs: DefaultConfigs,
	}
}

// SetConfig sets the configuration for a language.
func (m *Manager) SetConfig(language string, config ServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[language] = config
}

// GetClient returns or creates a client for the given language.
func (m *Manager) GetClient(ctx context.Context, language, rootURI string) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[language]; ok {
		return client, nil
	}

	config, ok := m.configs[language]
	if !ok {
		return nil, fmt.Errorf("no configuration for language: %s", language)
	}

	client := NewClient(config.Command, config.Args...)

	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start language server: %w", err)
	}

	if err := client.Initialize(ctx, rootURI); err != nil {
		client.Close(ctx)
		return nil, fmt.Errorf("failed to initialize language server: %w", err)
	}

	m.clients[language] = client
	return client, nil
}

// CloseAll closes all active language server clients.
func (m *Manager) CloseAll(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		client.Close(ctx)
	}
	m.clients = make(map[string]*Client)
}

// GetLanguageForFile returns the language ID for a file path.
func GetLanguageForFile(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hxx":
		return "cpp"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".cs":
		return "csharp"
	case ".swift":
		return "swift"
	case ".kt", ".kts":
		return "kotlin"
	case ".lua":
		return "lua"
	case ".sh", ".bash":
		return "shellscript"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".scss":
		return "scss"
	case ".less":
		return "less"
	case ".md", ".markdown":
		return "markdown"
	case ".sql":
		return "sql"
	default:
		return "plaintext"
	}
}

// FileToURI converts a file path to a file URI.
func FileToURI(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	return "file://" + absPath
}

// URIToFile converts a file URI to a file path.
func URIToFile(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}
