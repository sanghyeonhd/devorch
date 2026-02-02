// Package core provides the unified core system for DevOrch
// This serves as the foundation for all UI interfaces (CLI/TUI/WebUI)
package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"devorch/internal/agent"
	"devorch/internal/compaction"
	"devorch/internal/config"
	"devorch/internal/execution"
	"devorch/internal/okaon"
	"devorch/internal/proxy"
	"devorch/internal/session"
	"devorch/internal/tool"
	"devorch/internal/tool/builtin"
	"devorch/internal/websocket"
	"devorch/learning"
)

// DevOrchCore is the unified core system that all interfaces share
type DevOrchCore struct {
	// Core managers
	SessionManager *session.Manager
	ToolRegistry   *tool.Registry
	Config         *config.Config

	// Agent orchestration
	AgentRegistry *agent.Registry
	Orchestrator  *agent.Orchestrator
	// Learning system
	OkAONStore okaon.Store
	Learner    *learning.Learner

	// Phase 4 Services
	WebSocketServer  *websocket.Server
	ExecutionEngine  *execution.Engine
	CompactionEngine *compaction.Engine
	ProxyManager     *proxy.Manager

	// State management
	mu            sync.RWMutex
	activeSession *session.Session

	// Context
	ctx    context.Context
	cancel context.CancelFunc
}

// CoreConfig contains configuration for DevOrchCore
type CoreConfig struct {
	SessionStorePath string
	ToolPluginPaths  []string
	EnableOkaon      bool
	EventBusSize     int
}

// NewDevOrchCore creates a new unified core instance
func NewDevOrchCore(coreConfig CoreConfig) (*DevOrchCore, error) {
	ctx, cancel := context.WithCancel(context.Background())

	core := &DevOrchCore{
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize managers in dependency order
	if err := core.initializeManagers(coreConfig); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize core: %w", err)
	}

	return core, nil
}

// initializeManagers initializes all core managers
func (c *DevOrchCore) initializeManagers(coreConfig CoreConfig) error {
	var err error

	// 1. Load Config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	c.Config = &cfg

	// 2. Session Manager
	c.SessionManager, err = session.NewManager(coreConfig.SessionStorePath)
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	// 3. Tool Registry
	c.ToolRegistry, err = tool.NewRegistry()
	if err != nil {
		return fmt.Errorf("failed to initialize tool registry: %w", err)
	}

	// Set WorkDir for tool execution
	workDir, _ := os.Getwd()
	c.ToolRegistry.WorkDir = workDir

	// 4. Agent System
	c.AgentRegistry = agent.NewRegistry()
	c.Orchestrator = agent.NewOrchestrator(c.AgentRegistry)

	// Load default agents
	if err := c.loadDefaultAgents(); err != nil {
		return fmt.Errorf("failed to load default agents: %w", err)
	}

	// 4.5. Learning System
	if err := c.initializeLearningSystem(coreConfig); err != nil {
		return fmt.Errorf("failed to initialize learning system: %w", err)
	}

	// 5. Load builtin tools
	if err := c.loadBuiltinTools(coreConfig); err != nil {
		return fmt.Errorf("failed to load builtin tools: %w", err)
	}

	// 6. Initialize Phase 4 Services
	if err := c.initializePhase4Services(coreConfig); err != nil {
		return fmt.Errorf("failed to initialize Phase 4 services: %w", err)
	}

	return nil
}

// GetActiveSession returns the currently active session
func (c *DevOrchCore) GetActiveSession() *session.Session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeSession
}

// SetActiveSession sets the active session
func (c *DevOrchCore) SetActiveSession(sess *session.Session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeSession = sess
}

// GetSessionManager returns the session manager
func (c *DevOrchCore) GetSessionManager() *session.Manager {
	return c.SessionManager
}

// GetToolRegistry returns the unified tool registry
func (c *DevOrchCore) GetToolRegistry() *tool.Registry {
	return c.ToolRegistry
}

// GetConfig returns the config
func (c *DevOrchCore) GetConfig() *config.Config {
	return c.Config
}

// Shutdown gracefully shuts down the core
func (c *DevOrchCore) Shutdown() error {
	c.cancel()

	// Shutdown managers in reverse order
	var errors []error

	// Shutdown Phase 4 services
	if c.ProxyManager != nil {
		if err := c.ProxyManager.Shutdown(); err != nil {
			errors = append(errors, err)
		}
	}

	if c.CompactionEngine != nil {
		if err := c.CompactionEngine.Shutdown(); err != nil {
			errors = append(errors, err)
		}
	}

	if c.ExecutionEngine != nil {
		if err := c.ExecutionEngine.Shutdown(); err != nil {
			errors = append(errors, err)
		}
	}

	if c.WebSocketServer != nil {
		if err := c.WebSocketServer.Stop(); err != nil {
			errors = append(errors, err)
		}
	}

	if err := c.ToolRegistry.Shutdown(); err != nil {
		errors = append(errors, err)
	}

	if err := c.SessionManager.Shutdown(); err != nil {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	return nil
}

// Context returns the core context
func (c *DevOrchCore) Context() context.Context {
	return c.ctx
}

// loadBuiltinTools loads all builtin tools into the registry
func (c *DevOrchCore) loadBuiltinTools(coreConfig CoreConfig) error {
	// Get current working directory as default root
	rootDir, err := os.Getwd()
	if err != nil {
		rootDir = "."
	}

	// If session store path is set, use its parent as root
	if coreConfig.SessionStorePath != "" {
		rootDir = filepath.Dir(coreConfig.SessionStorePath)
	}

	// Load all default builtin tools
	builtinTools := builtin.DefaultTools(rootDir)

	// Register all tools
	for _, tool := range builtinTools {
		c.ToolRegistry.Register(tool)
	}

	return nil
}

// initializeLearningSystem initializes OkAON learning system
func (c *DevOrchCore) initializeLearningSystem(coreConfig CoreConfig) error {
	// For now, use a simple in-memory store for demo
	// In production, this would use the SQLite store
	c.OkAONStore = learning.NewMockStore()

	// Initialize learner with Thompson Sampling bandit
	thompsonBandit := learning.NewThompsonSamplingBandit(c.OkAONStore)
	c.Learner = learning.NewLearner(c.OkAONStore, thompsonBandit)

	// Connect learner to orchestrator for agent selection
	if c.Orchestrator != nil {
		c.Orchestrator.SetLearner(c.Learner)
	}

	return nil
}

// loadDefaultAgents loads default agent configurations
func (c *DevOrchCore) loadDefaultAgents() error {
	// File Operations Agent
	fileAgent := &agent.Agent{
		Name:         "file-ops",
		Model:        "anthropic/claude-3-haiku",
		Temperature:  0.3,
		SystemPrompt: "You are a file operations specialist. You excel at reading, writing, and organizing files. Use ls, read, write, and edit tools efficiently.",
		AllowedTools: []string{"ls", "read", "write", "edit", "grep", "glob"},
		MaxTokens:    1000,
	}

	// Research Agent
	researchAgent := &agent.Agent{
		Name:         "research",
		Model:        "openrouter/anthropic/claude-3-opus",
		Temperature:  0.5,
		SystemPrompt: "You are a research specialist. You excel at finding information, analyzing code, and discovering patterns. Use grep, glob, view, and sourcegraph tools.",
		AllowedTools: []string{"grep", "glob", "view", "sourcegraph", "read", "ls"},
		MaxTokens:    2000,
	}

	// Development Agent
	devAgent := &agent.Agent{
		Name:         "developer",
		Model:        "openrouter/openai/gpt-4",
		Temperature:  0.4,
		SystemPrompt: "You are a software developer. You write code, fix bugs, and implement features. Use all available tools as needed.",
		AllowedTools: []string{"*"},    // Allow all tools
		DeniedTools:  []string{"bash"}, // Except dangerous ones
		MaxTokens:    3000,
	}

	// Register agents
	c.AgentRegistry.Register(fileAgent)
	c.AgentRegistry.Register(researchAgent)
	c.AgentRegistry.Register(devAgent)

	return nil
}

// initializePhase4Services initializes Phase 4 services
func (c *DevOrchCore) initializePhase4Services(coreConfig CoreConfig) error {
	// WebSocket Server
	c.WebSocketServer = websocket.NewServer(":8787")
	if err := c.WebSocketServer.Start(); err != nil {
		return fmt.Errorf("failed to start WebSocket server: %w", err)
	}

	// Execution Engine
	c.ExecutionEngine = execution.NewEngine()

	// Compaction Engine
	c.CompactionEngine = compaction.NewEngine()

	// Proxy Manager
	c.ProxyManager = proxy.NewManager()

	return nil
}

// GetWebSocketServer returns the WebSocket server
func (c *DevOrchCore) GetWebSocketServer() *websocket.Server {
	return c.WebSocketServer
}

// GetExecutionEngine returns the execution engine
func (c *DevOrchCore) GetExecutionEngine() *execution.Engine {
	return c.ExecutionEngine
}

// GetCompactionEngine returns the compaction engine
func (c *DevOrchCore) GetCompactionEngine() *compaction.Engine {
	return c.CompactionEngine
}

// GetProxyManager returns the proxy manager
func (c *DevOrchCore) GetProxyManager() *proxy.Manager {
	return c.ProxyManager
}
