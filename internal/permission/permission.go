// Package permission provides tool permission management
package permission

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

// Level represents the permission level for a tool
type Level string

const (
	// LevelAllow always allows the tool
	LevelAllow Level = "allow"
	// LevelDeny always denies the tool
	LevelDeny Level = "deny"
	// LevelAsk asks the user every time
	LevelAsk Level = "ask"
	// LevelAskOnce asks once per session
	LevelAskOnce Level = "ask_once"
)

// Category represents tool categories
type Category string

const (
	CategoryRead      Category = "read"      // Tools that read files/data
	CategoryWrite     Category = "write"     // Tools that modify files
	CategoryExecute   Category = "execute"   // Tools that execute commands
	CategoryNetwork   Category = "network"   // Tools that access network
	CategoryDangerous Category = "dangerous" // Potentially dangerous operations
)

// Request represents a permission request
type Request struct {
	Tool        string                 `json:"tool"`
	Category    Category               `json:"category"`
	Description string                 `json:"description"`
	Args        map[string]interface{} `json:"args"`
	Timestamp   time.Time              `json:"timestamp"`
}

// Decision represents a permission decision
type Decision struct {
	Allowed   bool      `json:"allowed"`
	Remember  bool      `json:"remember"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason,omitempty"`
}

// Rule represents a permission rule
type Rule struct {
	Tool      string    `json:"tool"`
	Level     Level     `json:"level"`
	Paths     []string  `json:"paths,omitempty"`    // Allowed/denied paths
	Commands  []string  `json:"commands,omitempty"` // Allowed/denied commands
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Manager manages tool permissions
type Manager struct {
	rules            map[string]*Rule
	rulesMu          sync.RWMutex
	sessionDecisions map[string]bool // Session-specific decisions
	sessionMu        sync.RWMutex
	configPath       string
	askFunc          func(context.Context, Request) (Decision, error)
}

// Config represents the permission configuration
type Config struct {
	DefaultLevel Level              `json:"default_level"`
	Rules        map[string]*Rule   `json:"rules"`
	Categories   map[Category]Level `json:"categories"`
}

// NewManager creates a new permission manager
func NewManager(configPath string, askFunc func(context.Context, Request) (Decision, error)) (*Manager, error) {
	if configPath == "" {
		homeDir, _ := os.UserHomeDir()
		configPath = filepath.Join(homeDir, ".devorch", "permissions.json")
	}

	m := &Manager{
		rules:            make(map[string]*Rule),
		sessionDecisions: make(map[string]bool),
		configPath:       configPath,
		askFunc:          askFunc,
	}

	// Load existing configuration
	if err := m.Load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Set default rules
	m.setDefaultRules()

	return m, nil
}

func (m *Manager) setDefaultRules() {
	m.rulesMu.Lock()
	defer m.rulesMu.Unlock()

	// Default rules for common tools
	defaults := map[string]Level{
		// Read-only tools - generally safe
		"read":        LevelAllow,
		"view":        LevelAllow,
		"glob":        LevelAllow,
		"grep":        LevelAllow,
		"ls":          LevelAllow,
		"diagnostics": LevelAllow,
		"sourcegraph": LevelAllow,

		// Write tools - ask for permission
		"write":     LevelAskOnce,
		"edit":      LevelAskOnce,
		"patch":     LevelAskOnce,
		"multiedit": LevelAskOnce,

		// Execute tools - always ask
		"bash":  LevelAsk,
		"shell": LevelAsk,

		// Network tools - ask once
		"fetch":     LevelAskOnce,
		"webfetch":  LevelAskOnce,
		"websearch": LevelAskOnce,

		// Agent tools - ask once
		"agent": LevelAskOnce,
	}

	for tool, level := range defaults {
		if _, exists := m.rules[tool]; !exists {
			m.rules[tool] = &Rule{
				Tool:      tool,
				Level:     level,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
		}
	}
}

// Check checks if a tool is allowed
func (m *Manager) Check(ctx context.Context, req Request) (bool, error) {
	m.rulesMu.RLock()
	rule, exists := m.rules[req.Tool]
	m.rulesMu.RUnlock()

	if !exists {
		// Default to asking
		rule = &Rule{
			Tool:  req.Tool,
			Level: LevelAsk,
		}
	}

	switch rule.Level {
	case LevelAllow:
		// Check path restrictions if any
		if len(rule.Paths) > 0 {
			if !m.checkPathAllowed(req, rule.Paths) {
				return m.askPermission(ctx, req)
			}
		}
		return true, nil

	case LevelDeny:
		return false, nil

	case LevelAskOnce:
		// Check session decisions
		m.sessionMu.RLock()
		if decision, ok := m.sessionDecisions[req.Tool]; ok {
			m.sessionMu.RUnlock()
			return decision, nil
		}
		m.sessionMu.RUnlock()

		allowed, err := m.askPermission(ctx, req)
		if err != nil {
			return false, err
		}

		// Remember for session
		m.sessionMu.Lock()
		m.sessionDecisions[req.Tool] = allowed
		m.sessionMu.Unlock()

		return allowed, nil

	case LevelAsk:
		return m.askPermission(ctx, req)

	default:
		return m.askPermission(ctx, req)
	}
}

func (m *Manager) checkPathAllowed(req Request, allowedPaths []string) bool {
	// Extract path from args
	var path string
	if p, ok := req.Args["path"].(string); ok {
		path = p
	} else if p, ok := req.Args["file"].(string); ok {
		path = p
	} else if p, ok := req.Args["file_path"].(string); ok {
		path = p
	}

	if path == "" {
		return true // No path to check
	}

	absPath, _ := filepath.Abs(path)

	for _, allowed := range allowedPaths {
		absAllowed, _ := filepath.Abs(allowed)

		// Check if path is under allowed path
		if strings.HasPrefix(absPath, absAllowed) {
			return true
		}

		// Check glob patterns
		if matched, _ := filepath.Match(allowed, path); matched {
			return true
		}
	}

	return false
}

func (m *Manager) askPermission(ctx context.Context, req Request) (bool, error) {
	if m.askFunc == nil {
		// Default to deny if no ask function
		return false, nil
	}

	req.Timestamp = time.Now()
	decision, err := m.askFunc(ctx, req)
	if err != nil {
		return false, err
	}

	// Remember decision if requested
	if decision.Remember {
		m.SetRule(req.Tool, LevelAllow)
	}

	return decision.Allowed, nil
}

// SetRule sets a permission rule
func (m *Manager) SetRule(tool string, level Level) {
	m.rulesMu.Lock()
	defer m.rulesMu.Unlock()

	if rule, exists := m.rules[tool]; exists {
		rule.Level = level
		rule.UpdatedAt = time.Now()
	} else {
		m.rules[tool] = &Rule{
			Tool:      tool,
			Level:     level,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	// Save changes
	m.Save()
}

// SetRuleWithPaths sets a permission rule with path restrictions
func (m *Manager) SetRuleWithPaths(tool string, level Level, paths []string) {
	m.rulesMu.Lock()
	defer m.rulesMu.Unlock()

	if rule, exists := m.rules[tool]; exists {
		rule.Level = level
		rule.Paths = paths
		rule.UpdatedAt = time.Now()
	} else {
		m.rules[tool] = &Rule{
			Tool:      tool,
			Level:     level,
			Paths:     paths,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	m.Save()
}

// GetRule gets a permission rule
func (m *Manager) GetRule(tool string) (*Rule, bool) {
	m.rulesMu.RLock()
	defer m.rulesMu.RUnlock()

	rule, exists := m.rules[tool]
	return rule, exists
}

// GetAllRules returns all permission rules
func (m *Manager) GetAllRules() map[string]*Rule {
	m.rulesMu.RLock()
	defer m.rulesMu.RUnlock()

	rules := make(map[string]*Rule)
	for k, v := range m.rules {
		rules[k] = v
	}
	return rules
}

// ResetSession resets session-specific decisions
func (m *Manager) ResetSession() {
	m.sessionMu.Lock()
	defer m.sessionMu.Unlock()

	m.sessionDecisions = make(map[string]bool)
}

// Load loads permission configuration
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	m.rulesMu.Lock()
	defer m.rulesMu.Unlock()

	for k, v := range config.Rules {
		m.rules[k] = v
	}

	return nil
}

// Save saves permission configuration
func (m *Manager) Save() error {
	m.rulesMu.RLock()
	defer m.rulesMu.RUnlock()

	config := Config{
		DefaultLevel: LevelAsk,
		Rules:        m.rules,
	}

	data, err := json.MarshalIndent(config, "", "  ")
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

// ToolCategory returns the category for a tool
func ToolCategory(tool string) Category {
	categories := map[string]Category{
		// Read tools
		"read":        CategoryRead,
		"view":        CategoryRead,
		"glob":        CategoryRead,
		"grep":        CategoryRead,
		"ls":          CategoryRead,
		"diagnostics": CategoryRead,
		"tree":        CategoryRead,

		// Write tools
		"write":     CategoryWrite,
		"edit":      CategoryWrite,
		"patch":     CategoryWrite,
		"multiedit": CategoryWrite,
		"mkdir":     CategoryWrite,
		"rm":        CategoryWrite,

		// Execute tools
		"bash":  CategoryExecute,
		"shell": CategoryExecute,
		"agent": CategoryExecute,

		// Network tools
		"fetch":       CategoryNetwork,
		"webfetch":    CategoryNetwork,
		"websearch":   CategoryNetwork,
		"sourcegraph": CategoryNetwork,
	}

	if cat, ok := categories[tool]; ok {
		return cat
	}
	return CategoryDangerous
}

// FormatRequest formats a permission request for display
func FormatRequest(req Request) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Tool: %s\n", req.Tool))
	sb.WriteString(fmt.Sprintf("Category: %s\n", req.Category))

	if req.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", req.Description))
	}

	if len(req.Args) > 0 {
		sb.WriteString("Arguments:\n")
		for k, v := range req.Args {
			sb.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}
	}

	return sb.String()
}
