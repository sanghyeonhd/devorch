// Package mcp provides auto-configuration for MCP servers
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// RecommendedServer represents a pre-configured MCP server recommendation
type RecommendedServer struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	URL         string            `json:"url,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Category    string            `json:"category"`              // "search", "code", "docs", "system", "ai"
	RequiresAPI bool              `json:"requires_api"`          // Needs API key
	APIKeyEnv   string            `json:"api_key_env,omitempty"` // Environment variable for API key
	NpmPackage  string            `json:"npm_package,omitempty"` // NPM package to install
	Priority    int               `json:"priority"`              // Higher = more important
}

// GetRecommendedServers returns a list of recommended MCP servers
func GetRecommendedServers() []RecommendedServer {
	return []RecommendedServer{
		// Search & Web
		{
			Name:        "websearch",
			Description: "Web search using Exa AI - find real-time information",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-websearch"},
			Category:    "search",
			RequiresAPI: true,
			APIKeyEnv:   "EXA_API_KEY",
			NpmPackage:  "@anthropic/mcp-server-websearch",
			Priority:    100,
		},
		{
			Name:        "brave-search",
			Description: "Brave Search API for web search",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-brave-search"},
			Category:    "search",
			RequiresAPI: true,
			APIKeyEnv:   "BRAVE_API_KEY",
			NpmPackage:  "@anthropic/mcp-server-brave-search",
			Priority:    90,
		},

		// Code & GitHub
		{
			Name:        "github",
			Description: "GitHub API - repositories, issues, PRs, code search",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-github"},
			Category:    "code",
			RequiresAPI: true,
			APIKeyEnv:   "GITHUB_TOKEN",
			NpmPackage:  "@anthropic/mcp-server-github",
			Priority:    95,
		},
		{
			Name:        "grep-app",
			Description: "Search GitHub code using grep.app",
			URL:         "https://mcp.grep.app",
			Category:    "code",
			RequiresAPI: false,
			Priority:    80,
		},

		// Documentation & Knowledge
		{
			Name:        "context7",
			Description: "Library documentation lookup via Context7",
			URL:         "https://mcp.context7.com",
			Category:    "docs",
			RequiresAPI: false,
			Priority:    85,
		},
		{
			Name:        "sequential-thinking",
			Description: "Enhanced reasoning with step-by-step thinking",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-sequential-thinking"},
			Category:    "ai",
			RequiresAPI: false,
			NpmPackage:  "@anthropic/mcp-server-sequential-thinking",
			Priority:    75,
		},

		// File System
		{
			Name:        "filesystem",
			Description: "File system operations with safety controls",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-filesystem", "--allow-dir", "."},
			Category:    "system",
			RequiresAPI: false,
			NpmPackage:  "@anthropic/mcp-server-filesystem",
			Priority:    90,
		},

		// Memory & Persistence
		{
			Name:        "memory",
			Description: "Persistent memory storage across sessions",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-memory"},
			Category:    "system",
			RequiresAPI: false,
			NpmPackage:  "@anthropic/mcp-server-memory",
			Priority:    70,
		},

		// Fetch & HTTP
		{
			Name:        "fetch",
			Description: "HTTP fetch operations for web content",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-fetch"},
			Category:    "system",
			RequiresAPI: false,
			NpmPackage:  "@anthropic/mcp-server-fetch",
			Priority:    65,
		},

		// Database
		{
			Name:        "sqlite",
			Description: "SQLite database operations",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-sqlite"},
			Category:    "system",
			RequiresAPI: false,
			NpmPackage:  "@anthropic/mcp-server-sqlite",
			Priority:    60,
		},

		// Shell/Terminal
		{
			Name:        "shell",
			Description: "Safe shell command execution",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-shell"},
			Category:    "system",
			RequiresAPI: false,
			NpmPackage:  "@anthropic/mcp-server-shell",
			Priority:    50,
		},
	}
}

// AutoConfigOptions configures auto-setup behavior
type AutoConfigOptions struct {
	InstallMissing  bool     // Install missing packages
	EnableAll       bool     // Enable all available servers
	Categories      []string // Only enable specific categories
	SkipAPIRequired bool     // Skip servers requiring API keys
	Verbose         bool     // Print detailed output
}

// DefaultAutoConfigOptions returns sensible defaults
func DefaultAutoConfigOptions() AutoConfigOptions {
	return AutoConfigOptions{
		InstallMissing:  true,
		EnableAll:       false,
		Categories:      []string{"code", "docs", "system"},
		SkipAPIRequired: false,
		Verbose:         false,
	}
}

// AutoConfigResult contains the results of auto-configuration
type AutoConfigResult struct {
	Configured []string        `json:"configured"`
	Skipped    []SkippedServer `json:"skipped"`
	Failed     []FailedServer  `json:"failed"`
	ConfigPath string          `json:"config_path"`
	Config     *Config         `json:"config"`
}

// SkippedServer represents a server that was skipped
type SkippedServer struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// FailedServer represents a server that failed to configure
type FailedServer struct {
	Name  string `json:"name"`
	Error string `json:"error"`
}

// AutoConfigurator handles automatic MCP server configuration
type AutoConfigurator struct {
	manager *Manager
	opts    AutoConfigOptions
}

// NewAutoConfigurator creates a new auto configurator
func NewAutoConfigurator(manager *Manager, opts AutoConfigOptions) *AutoConfigurator {
	return &AutoConfigurator{
		manager: manager,
		opts:    opts,
	}
}

// DetectEnvironment detects the current environment capabilities
func (a *AutoConfigurator) DetectEnvironment() *EnvironmentInfo {
	info := &EnvironmentInfo{
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		HasNode:    a.commandExists("node"),
		HasNPX:     a.commandExists("npx"),
		HasNPM:     a.commandExists("npm"),
		HasDocker:  a.commandExists("docker"),
		HomeDir:    os.Getenv("HOME"),
		WorkingDir: ".",
	}

	if wd, err := os.Getwd(); err == nil {
		info.WorkingDir = wd
	}

	// Detect Node version
	if info.HasNode {
		if out, err := exec.Command("node", "--version").Output(); err == nil {
			info.NodeVersion = strings.TrimSpace(string(out))
		}
	}

	// Check for available API keys
	info.AvailableAPIKeys = make(map[string]bool)
	apiKeyEnvs := []string{"EXA_API_KEY", "BRAVE_API_KEY", "GITHUB_TOKEN", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"}
	for _, env := range apiKeyEnvs {
		info.AvailableAPIKeys[env] = os.Getenv(env) != ""
	}

	return info
}

// EnvironmentInfo contains detected environment information
type EnvironmentInfo struct {
	OS               string          `json:"os"`
	Arch             string          `json:"arch"`
	HasNode          bool            `json:"has_node"`
	HasNPX           bool            `json:"has_npx"`
	HasNPM           bool            `json:"has_npm"`
	HasDocker        bool            `json:"has_docker"`
	NodeVersion      string          `json:"node_version,omitempty"`
	HomeDir          string          `json:"home_dir"`
	WorkingDir       string          `json:"working_dir"`
	AvailableAPIKeys map[string]bool `json:"available_api_keys"`
}

// Configure performs automatic MCP configuration
func (a *AutoConfigurator) Configure(ctx context.Context) (*AutoConfigResult, error) {
	env := a.DetectEnvironment()

	result := &AutoConfigResult{
		Configured: []string{},
		Skipped:    []SkippedServer{},
		Failed:     []FailedServer{},
	}

	// Check for Node.js requirement
	if !env.HasNPX {
		if a.opts.InstallMissing {
			// Try to guide user to install Node.js
			return nil, fmt.Errorf("npx not found. Please install Node.js from https://nodejs.org/")
		}
	}

	servers := GetRecommendedServers()

	// Filter by categories if specified
	if len(a.opts.Categories) > 0 && !a.opts.EnableAll {
		filtered := make([]RecommendedServer, 0)
		categorySet := make(map[string]bool)
		for _, c := range a.opts.Categories {
			categorySet[c] = true
		}
		for _, s := range servers {
			if categorySet[s.Category] {
				filtered = append(filtered, s)
			}
		}
		servers = filtered
	}

	// Create config
	config := &Config{
		Servers: make(map[string]ServerConfig),
	}

	for _, server := range servers {
		// Check API key requirement
		if server.RequiresAPI && a.opts.SkipAPIRequired {
			result.Skipped = append(result.Skipped, SkippedServer{
				Name:   server.Name,
				Reason: "requires API key",
			})
			continue
		}

		// Check if API key is available
		if server.RequiresAPI && server.APIKeyEnv != "" {
			if !env.AvailableAPIKeys[server.APIKeyEnv] {
				result.Skipped = append(result.Skipped, SkippedServer{
					Name:   server.Name,
					Reason: fmt.Sprintf("missing %s environment variable", server.APIKeyEnv),
				})
				continue
			}
		}

		// Create server config
		serverConfig := ServerConfig{
			Name:    server.Name,
			Command: server.Command,
			Args:    server.Args,
			URL:     server.URL,
			Env:     server.Env,
			Enabled: true,
		}

		// Install npm package if needed
		if a.opts.InstallMissing && server.NpmPackage != "" && env.HasNPM {
			if err := a.installNpmPackage(ctx, server.NpmPackage); err != nil {
				if a.opts.Verbose {
					fmt.Printf("Warning: failed to install %s: %v\n", server.NpmPackage, err)
				}
				// Don't fail - npx will download on demand
			}
		}

		// Test if server works (quick check)
		if server.Command != "" && !server.RequiresAPI {
			if _, err := exec.LookPath(server.Command); err != nil && server.Command != "npx" {
				result.Failed = append(result.Failed, FailedServer{
					Name:  server.Name,
					Error: fmt.Sprintf("command not found: %s", server.Command),
				})
				continue
			}
		}

		config.Servers[server.Name] = serverConfig
		result.Configured = append(result.Configured, server.Name)
	}

	// Save configuration
	configPath, err := a.saveConfig(config)
	if err != nil {
		return result, fmt.Errorf("failed to save config: %w", err)
	}

	result.ConfigPath = configPath
	result.Config = config

	// Update manager
	a.manager.mu.Lock()
	a.manager.config = config
	a.manager.mu.Unlock()

	return result, nil
}

// commandExists checks if a command exists
func (a *AutoConfigurator) commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// installNpmPackage installs an npm package globally
func (a *AutoConfigurator) installNpmPackage(ctx context.Context, pkg string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "npm", "install", "-g", pkg)
	cmd.Env = append(os.Environ(), "NPM_CONFIG_YES=true")

	return cmd.Run()
}

// saveConfig saves the configuration to a file
func (a *AutoConfigurator) saveConfig(config *Config) (string, error) {
	// Determine config directory
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "devorch")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	configPath := filepath.Join(configDir, "mcp.json")

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", err
	}

	return configPath, nil
}

// GetQuickSetupServers returns minimal essential servers for quick setup
func GetQuickSetupServers() []string {
	return []string{
		"filesystem",          // File operations
		"github",              // GitHub integration (if token available)
		"context7",            // Documentation lookup
		"fetch",               // HTTP requests
		"memory",              // Persistent memory
		"sequential-thinking", // Enhanced reasoning
	}
}

// GetEssentialMCPServers returns essential MCP servers with their descriptions
func GetEssentialMCPServers() []RecommendedServer {
	all := GetRecommendedServers()
	essential := GetQuickSetupServers()
	essentialSet := make(map[string]bool)
	for _, name := range essential {
		essentialSet[name] = true
	}

	var result []RecommendedServer
	for _, s := range all {
		if essentialSet[s.Name] {
			result = append(result, s)
		}
	}
	return result
}

// QuickSetup performs a minimal quick setup
func (a *AutoConfigurator) QuickSetup(ctx context.Context) (*AutoConfigResult, error) {
	quickServers := GetQuickSetupServers()

	// Create a custom category filter for quick setup
	a.opts.EnableAll = false
	a.opts.SkipAPIRequired = false

	all := GetRecommendedServers()
	quickSet := make(map[string]bool)
	for _, s := range quickServers {
		quickSet[s] = true
	}

	result := &AutoConfigResult{
		Configured: []string{},
		Skipped:    []SkippedServer{},
		Failed:     []FailedServer{},
	}

	config := &Config{
		Servers: make(map[string]ServerConfig),
	}

	env := a.DetectEnvironment()

	for _, server := range all {
		if !quickSet[server.Name] {
			continue
		}

		// Skip if API required but not available
		if server.RequiresAPI && server.APIKeyEnv != "" {
			if !env.AvailableAPIKeys[server.APIKeyEnv] {
				result.Skipped = append(result.Skipped, SkippedServer{
					Name:   server.Name,
					Reason: fmt.Sprintf("missing %s", server.APIKeyEnv),
				})
				continue
			}
		}

		serverConfig := ServerConfig{
			Name:    server.Name,
			Command: server.Command,
			Args:    server.Args,
			URL:     server.URL,
			Env:     server.Env,
			Enabled: true,
		}

		config.Servers[server.Name] = serverConfig
		result.Configured = append(result.Configured, server.Name)
	}

	configPath, err := a.saveConfig(config)
	if err != nil {
		return result, err
	}

	result.ConfigPath = configPath
	result.Config = config

	a.manager.mu.Lock()
	a.manager.config = config
	a.manager.mu.Unlock()

	return result, nil
}

// TestServer tests if a server can be connected
func (a *AutoConfigurator) TestServer(ctx context.Context, name string) error {
	client, err := a.manager.GetClient(ctx, name)
	if err != nil {
		return err
	}

	// Try to list tools as a connectivity test
	_, err = client.ListTools(ctx)
	return err
}

// TestAllServers tests all configured servers
func (a *AutoConfigurator) TestAllServers(ctx context.Context) map[string]error {
	results := make(map[string]error)

	a.manager.mu.RLock()
	servers := a.manager.config.Servers
	a.manager.mu.RUnlock()

	for name := range servers {
		testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		results[name] = a.TestServer(testCtx, name)
		cancel()
	}

	return results
}

// GenerateConfigTemplate generates a config template with all options
func GenerateConfigTemplate() string {
	servers := GetRecommendedServers()

	config := Config{
		Servers: make(map[string]ServerConfig),
	}

	for _, s := range servers {
		config.Servers[s.Name] = ServerConfig{
			Name:    s.Name,
			Command: s.Command,
			Args:    s.Args,
			URL:     s.URL,
			Env:     s.Env,
			Enabled: false, // Disabled by default in template
		}
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	return string(data)
}

// GetServersByCategory returns servers grouped by category
func GetServersByCategory() map[string][]RecommendedServer {
	result := make(map[string][]RecommendedServer)

	for _, s := range GetRecommendedServers() {
		result[s.Category] = append(result[s.Category], s)
	}

	return result
}
