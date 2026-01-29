// Package plugin provides plugin management.
package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Manager manages plugins.
type Manager struct {
	mu            sync.RWMutex
	plugins       map[string]*loadedPlugin
	pluginsDir    string
	dataDir       string
	workspacePath string
	config        map[string]any

	// Registries
	tools     map[string]ToolHandler
	providers map[string]LLMProvider
	hooks     map[string][]HookHandler
	agents    map[string]Agent
}

type loadedPlugin struct {
	manifest *PluginManifest
	plugin   Plugin
	enabled  bool
}

// NewManager creates a new plugin manager.
func NewManager(pluginsDir, dataDir, workspacePath string) *Manager {
	return &Manager{
		plugins:       make(map[string]*loadedPlugin),
		pluginsDir:    pluginsDir,
		dataDir:       dataDir,
		workspacePath: workspacePath,
		config:        make(map[string]any),
		tools:         make(map[string]ToolHandler),
		providers:     make(map[string]LLMProvider),
		hooks:         make(map[string][]HookHandler),
		agents:        make(map[string]Agent),
	}
}

// LoadAll loads all plugins from the plugins directory.
func (m *Manager) LoadAll(ctx context.Context) error {
	if err := os.MkdirAll(m.pluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	entries, err := os.ReadDir(m.pluginsDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(m.pluginsDir, entry.Name())
		if err := m.Load(ctx, pluginDir); err != nil {
			// Log error but continue loading other plugins
			m.log("warn", "Failed to load plugin %s: %v", entry.Name(), err)
		}
	}

	return nil
}

// Load loads a single plugin from a directory.
func (m *Manager) Load(ctx context.Context, dir string) error {
	manifest, err := LoadManifest(dir)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[manifest.Metadata.ID]; exists {
		return fmt.Errorf("plugin already loaded: %s", manifest.Metadata.ID)
	}

	// Check dependencies
	for _, dep := range manifest.Metadata.Requires {
		if _, ok := m.plugins[dep]; !ok {
			return fmt.Errorf("missing dependency: %s", dep)
		}
	}

	// Create plugin data directory
	dataPath := filepath.Join(m.dataDir, manifest.Metadata.ID)
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return err
	}

	// For built-in plugins, we don't use dynamic loading
	// Instead, plugins are registered programmatically
	m.plugins[manifest.Metadata.ID] = &loadedPlugin{
		manifest: manifest,
		plugin:   nil, // Will be set when plugin is registered
		enabled:  false,
	}

	return nil
}

// Register registers a plugin implementation.
func (m *Manager) Register(ctx context.Context, plugin Plugin) error {
	meta := plugin.Metadata()

	m.mu.Lock()
	lp, exists := m.plugins[meta.ID]
	if !exists {
		// Create a new entry for programmatic registration
		dataPath := filepath.Join(m.dataDir, meta.ID)
		if err := os.MkdirAll(dataPath, 0755); err != nil {
			m.mu.Unlock()
			return err
		}

		lp = &loadedPlugin{
			manifest: &PluginManifest{Metadata: meta},
			enabled:  false,
		}
		m.plugins[meta.ID] = lp
	}
	lp.plugin = plugin
	m.mu.Unlock()

	// Initialize the plugin
	host := m.createHost(meta.ID)
	if err := plugin.Initialize(ctx, host); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", meta.ID, err)
	}

	m.mu.Lock()
	lp.enabled = true
	m.mu.Unlock()

	m.log("info", "Plugin loaded: %s v%s", meta.Name, meta.Version)
	return nil
}

// Unload unloads a plugin.
func (m *Manager) Unload(ctx context.Context, id string) error {
	m.mu.Lock()
	lp, ok := m.plugins[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("plugin not found: %s", id)
	}
	m.mu.Unlock()

	if lp.plugin != nil {
		if err := lp.plugin.Shutdown(ctx); err != nil {
			m.log("warn", "Plugin %s shutdown error: %v", id, err)
		}
	}

	m.mu.Lock()
	delete(m.plugins, id)
	m.mu.Unlock()

	m.log("info", "Plugin unloaded: %s", id)
	return nil
}

// Enable enables a plugin.
func (m *Manager) Enable(ctx context.Context, id string) error {
	m.mu.Lock()
	lp, ok := m.plugins[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("plugin not found: %s", id)
	}

	if lp.enabled {
		m.mu.Unlock()
		return nil
	}

	if lp.plugin == nil {
		m.mu.Unlock()
		return fmt.Errorf("plugin not registered: %s", id)
	}

	lp.enabled = true
	m.mu.Unlock()

	return nil
}

// Disable disables a plugin.
func (m *Manager) Disable(ctx context.Context, id string) error {
	m.mu.Lock()
	lp, ok := m.plugins[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("plugin not found: %s", id)
	}

	lp.enabled = false
	m.mu.Unlock()

	return nil
}

// List returns all loaded plugins.
func (m *Manager) List() []Metadata {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Metadata, 0, len(m.plugins))
	for _, lp := range m.plugins {
		result = append(result, lp.manifest.Metadata)
	}
	return result
}

// GetPlugin returns a plugin by ID.
func (m *Manager) GetPlugin(id string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lp, ok := m.plugins[id]
	if !ok || lp.plugin == nil {
		return nil, false
	}
	return lp.plugin, true
}

// IsEnabled checks if a plugin is enabled.
func (m *Manager) IsEnabled(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lp, ok := m.plugins[id]
	return ok && lp.enabled
}

// createHost creates a host interface for a plugin.
func (m *Manager) createHost(pluginID string) Host {
	return &pluginHost{
		manager:  m,
		pluginID: pluginID,
		dataPath: filepath.Join(m.dataDir, pluginID),
	}
}

func (m *Manager) log(level, format string, args ...any) {
	// Simple logging - can be enhanced later
	fmt.Printf("[plugin][%s] %s\n", level, fmt.Sprintf(format, args...))
}

// Tool access

// GetTool returns a registered tool handler.
func (m *Manager) GetTool(name string) (ToolHandler, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, ok := m.tools[name]
	return h, ok
}

// ListTools returns all registered tools.
func (m *Manager) ListTools() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, 0, len(m.tools))
	for name := range m.tools {
		result = append(result, name)
	}
	return result
}

// Provider access

// GetProvider returns a registered provider.
func (m *Manager) GetProvider(name string) (LLMProvider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.providers[name]
	return p, ok
}

// ListProviders returns all registered providers.
func (m *Manager) ListProviders() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, 0, len(m.providers))
	for name := range m.providers {
		result = append(result, name)
	}
	return result
}

// Agent access

// GetAgent returns a registered agent.
func (m *Manager) GetAgent(name string) (Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.agents[name]
	return a, ok
}

// ListAgents returns all registered agents.
func (m *Manager) ListAgents() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, 0, len(m.agents))
	for name := range m.agents {
		result = append(result, name)
	}
	return result
}

// Hook dispatch

// DispatchHook dispatches a hook event to all handlers.
func (m *Manager) DispatchHook(ctx context.Context, event HookEvent) error {
	m.mu.RLock()
	handlers := m.hooks[event.Type]
	m.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			m.log("warn", "Hook handler error for %s: %v", event.Type, err)
		}
	}
	return nil
}

// pluginHost implements the Host interface for a specific plugin.
type pluginHost struct {
	manager  *Manager
	pluginID string
	dataPath string
}

func (h *pluginHost) RegisterTool(name string, handler ToolHandler) error {
	h.manager.mu.Lock()
	defer h.manager.mu.Unlock()

	if _, exists := h.manager.tools[name]; exists {
		return fmt.Errorf("tool already registered: %s", name)
	}

	h.manager.tools[name] = handler
	return nil
}

func (h *pluginHost) RegisterProvider(name string, provider LLMProvider) error {
	h.manager.mu.Lock()
	defer h.manager.mu.Unlock()

	if _, exists := h.manager.providers[name]; exists {
		return fmt.Errorf("provider already registered: %s", name)
	}

	h.manager.providers[name] = provider
	return nil
}

func (h *pluginHost) RegisterHook(event string, handler HookHandler) error {
	h.manager.mu.Lock()
	defer h.manager.mu.Unlock()

	h.manager.hooks[event] = append(h.manager.hooks[event], handler)
	return nil
}

func (h *pluginHost) RegisterAgent(name string, agent Agent) error {
	h.manager.mu.Lock()
	defer h.manager.mu.Unlock()

	if _, exists := h.manager.agents[name]; exists {
		return fmt.Errorf("agent already registered: %s", name)
	}

	h.manager.agents[name] = agent
	return nil
}

func (h *pluginHost) GetConfig(key string) (any, bool) {
	h.manager.mu.RLock()
	defer h.manager.mu.RUnlock()

	fullKey := h.pluginID + "." + key
	val, ok := h.manager.config[fullKey]
	return val, ok
}

func (h *pluginHost) SetConfig(key string, value any) {
	h.manager.mu.Lock()
	defer h.manager.mu.Unlock()

	fullKey := h.pluginID + "." + key
	h.manager.config[fullKey] = value
}

func (h *pluginHost) Log(level, message string, args ...any) {
	h.manager.log(level, "[%s] %s", h.pluginID, fmt.Sprintf(message, args...))
}

func (h *pluginHost) GetWorkspacePath() string {
	return h.manager.workspacePath
}

func (h *pluginHost) GetDataPath() string {
	return h.dataPath
}
