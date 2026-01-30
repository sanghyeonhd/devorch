package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ServerConfig represents MCP server configuration
type ServerConfig struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Enabled bool              `json:"enabled"`
}

// Config holds all MCP server configurations
type Config struct {
	Servers map[string]ServerConfig `json:"mcpServers"`
}

// Manager manages multiple MCP server connections
type Manager struct {
	config  *Config
	clients map[string]*Client
	mu      sync.RWMutex
}

// NewManager creates a new MCP manager
func NewManager() *Manager {
	return &Manager{
		config:  &Config{Servers: make(map[string]ServerConfig)},
		clients: make(map[string]*Client),
	}
}

// LoadConfig loads MCP configuration from file
func (m *Manager) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	m.mu.Lock()
	m.config = &config
	m.mu.Unlock()

	return nil
}

// LoadDefaultConfig loads config from default locations
func (m *Manager) LoadDefaultConfig() error {
	locations := []string{
		"mcp.json",
		".mcp.json",
		filepath.Join(os.Getenv("HOME"), ".config", "devorch", "mcp.json"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return m.LoadConfig(loc)
		}
	}

	return nil
}

// AddServer adds a server configuration
func (m *Manager) AddServer(name string, config ServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config.Servers == nil {
		m.config.Servers = make(map[string]ServerConfig)
	}
	config.Name = name
	m.config.Servers[name] = config
}

// RemoveServer removes a server configuration
func (m *Manager) RemoveServer(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[name]; ok {
		client.Close()
		delete(m.clients, name)
	}

	delete(m.config.Servers, name)
}

// GetClient returns or creates a client for a server
func (m *Manager) GetClient(ctx context.Context, name string) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[name]; ok {
		return client, nil
	}

	serverConfig, ok := m.config.Servers[name]
	if !ok {
		return nil, fmt.Errorf("server not found: %s", name)
	}

	if !serverConfig.Enabled {
		return nil, fmt.Errorf("server disabled: %s", name)
	}

	var transport Transport
	var err error

	if serverConfig.Command != "" {
		transport, err = NewStdioTransport(serverConfig.Command, serverConfig.Args...)
		if err != nil {
			return nil, fmt.Errorf("failed to create transport: %w", err)
		}
	} else if serverConfig.URL != "" {
		transport = NewHTTPTransport(serverConfig.URL)
	} else {
		return nil, fmt.Errorf("no command or URL specified for server: %s", name)
	}

	client := NewClient(transport)

	if err := client.Initialize(ctx, ClientInfo{
		Name:    "devorch",
		Version: "1.0.0",
	}); err != nil {
		transport.Close()
		return nil, fmt.Errorf("failed to initialize client: %w", err)
	}

	m.clients[name] = client
	return client, nil
}

// ListServers returns all configured servers with their configs
func (m *Manager) ListServers() map[string]ServerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]ServerConfig)
	for name, cfg := range m.config.Servers {
		result[name] = cfg
	}
	return result
}

// ListServerNames returns all configured server names
func (m *Manager) ListServerNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.config.Servers))
	for name := range m.config.Servers {
		names = append(names, name)
	}
	return names
}

// ListEnabledServers returns enabled server names
func (m *Manager) ListEnabledServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var names []string
	for name, config := range m.config.Servers {
		if config.Enabled {
			names = append(names, name)
		}
	}
	return names
}

// ListAllTools returns tools from all enabled servers
func (m *Manager) ListAllTools(ctx context.Context) (map[string][]Tool, error) {
	m.mu.RLock()
	servers := m.config.Servers
	m.mu.RUnlock()

	result := make(map[string][]Tool)

	for name, config := range servers {
		if !config.Enabled {
			continue
		}

		client, err := m.GetClient(ctx, name)
		if err != nil {
			continue
		}

		tools, err := client.ListTools(ctx)
		if err != nil {
			continue
		}

		result[name] = tools
	}

	return result, nil
}

// CallTool calls a tool on a specific server
func (m *Manager) CallTool(ctx context.Context, server, tool string, args json.RawMessage) (*ToolCallResult, error) {
	client, err := m.GetClient(ctx, server)
	if err != nil {
		return nil, err
	}

	return client.CallTool(ctx, tool, args)
}

// Close closes all clients
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		client.Close()
	}
	m.clients = make(map[string]*Client)

	return nil
}

// SaveConfig saves current configuration to file
func (m *Manager) SaveConfig(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
