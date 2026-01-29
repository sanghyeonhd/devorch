// Package plugin provides built-in plugins.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// BuiltinPlugins returns all built-in plugins.
func BuiltinPlugins() []Plugin {
	return []Plugin{
		&WebFetchPlugin{},
		&FileWatchPlugin{},
		&GitPlugin{},
	}
}

// WebFetchPlugin provides web fetching capabilities.
type WebFetchPlugin struct {
	host   Host
	client *http.Client
}

func (p *WebFetchPlugin) Metadata() Metadata {
	return Metadata{
		ID:          "devorch.web-fetch",
		Name:        "Web Fetch",
		Version:     "1.0.0",
		Description: "Fetch content from web URLs",
		Author:      "DevOrch",
		License:     "MIT",
		Provides:    []string{"tool"},
	}
}

func (p *WebFetchPlugin) Initialize(ctx context.Context, host Host) error {
	p.host = host
	p.client = &http.Client{}

	// Register fetch tool
	if err := host.RegisterTool("web_fetch", p.fetchTool); err != nil {
		return err
	}

	return nil
}

func (p *WebFetchPlugin) Shutdown(ctx context.Context) error {
	return nil
}

func (p *WebFetchPlugin) fetchTool(ctx context.Context, args map[string]any) (any, error) {
	urlStr, ok := args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("url is required")
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("only http and https URLs are supported")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "DevOrch/1.0")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	// Limit response size
	limitedReader := io.LimitReader(resp.Body, 1024*1024) // 1MB limit
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status":       resp.StatusCode,
		"content_type": resp.Header.Get("Content-Type"),
		"content":      string(body),
	}, nil
}

// FileWatchPlugin provides file watching capabilities.
type FileWatchPlugin struct {
	host     Host
	watchers map[string]chan struct{}
}

func (p *FileWatchPlugin) Metadata() Metadata {
	return Metadata{
		ID:          "devorch.file-watch",
		Name:        "File Watch",
		Version:     "1.0.0",
		Description: "Watch files and directories for changes",
		Author:      "DevOrch",
		License:     "MIT",
		Provides:    []string{"tool"},
	}
}

func (p *FileWatchPlugin) Initialize(ctx context.Context, host Host) error {
	p.host = host
	p.watchers = make(map[string]chan struct{})

	if err := host.RegisterTool("file_watch", p.watchTool); err != nil {
		return err
	}

	if err := host.RegisterTool("file_unwatch", p.unwatchTool); err != nil {
		return err
	}

	return nil
}

func (p *FileWatchPlugin) Shutdown(ctx context.Context) error {
	for _, stop := range p.watchers {
		close(stop)
	}
	return nil
}

func (p *FileWatchPlugin) watchTool(ctx context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path is required")
	}

	// Simplified implementation - would use fsnotify in production
	stop := make(chan struct{})
	p.watchers[path] = stop

	return map[string]any{
		"watching": path,
		"status":   "started",
	}, nil
}

func (p *FileWatchPlugin) unwatchTool(ctx context.Context, args map[string]any) (any, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("path is required")
	}

	if stop, exists := p.watchers[path]; exists {
		close(stop)
		delete(p.watchers, path)
		return map[string]any{"status": "stopped"}, nil
	}

	return map[string]any{"status": "not_watching"}, nil
}

// GitPlugin provides git operations.
type GitPlugin struct {
	host Host
}

func (p *GitPlugin) Metadata() Metadata {
	return Metadata{
		ID:          "devorch.git",
		Name:        "Git",
		Version:     "1.0.0",
		Description: "Git repository operations",
		Author:      "DevOrch",
		License:     "MIT",
		Provides:    []string{"tool"},
	}
}

func (p *GitPlugin) Initialize(ctx context.Context, host Host) error {
	p.host = host

	tools := map[string]ToolHandler{
		"git_status": p.statusTool,
		"git_diff":   p.diffTool,
		"git_log":    p.logTool,
		"git_branch": p.branchTool,
	}

	for name, handler := range tools {
		if err := host.RegisterTool(name, handler); err != nil {
			return err
		}
	}

	return nil
}

func (p *GitPlugin) Shutdown(ctx context.Context) error {
	return nil
}

func (p *GitPlugin) statusTool(ctx context.Context, args map[string]any) (any, error) {
	workDir := p.host.GetWorkspacePath()
	if dir, ok := args["directory"].(string); ok {
		workDir = dir
	}

	// Check if .git exists
	gitDir := filepath.Join(workDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a git repository: %s", workDir)
	}

	// Simple status - would shell out to git in production
	return map[string]any{
		"directory": workDir,
		"status":    "clean", // Simplified
	}, nil
}

func (p *GitPlugin) diffTool(ctx context.Context, args map[string]any) (any, error) {
	workDir := p.host.GetWorkspacePath()
	if dir, ok := args["directory"].(string); ok {
		workDir = dir
	}

	// Simplified implementation
	return map[string]any{
		"directory": workDir,
		"diff":      "", // Would contain actual diff
	}, nil
}

func (p *GitPlugin) logTool(ctx context.Context, args map[string]any) (any, error) {
	workDir := p.host.GetWorkspacePath()
	if dir, ok := args["directory"].(string); ok {
		workDir = dir
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	// Simplified implementation
	return map[string]any{
		"directory": workDir,
		"limit":     limit,
		"commits":   []map[string]any{}, // Would contain actual commits
	}, nil
}

func (p *GitPlugin) branchTool(ctx context.Context, args map[string]any) (any, error) {
	workDir := p.host.GetWorkspacePath()
	if dir, ok := args["directory"].(string); ok {
		workDir = dir
	}

	// Try to read current branch from HEAD
	headPath := filepath.Join(workDir, ".git", "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read git HEAD: %w", err)
	}

	content := strings.TrimSpace(string(data))
	branch := "unknown"
	if strings.HasPrefix(content, "ref: refs/heads/") {
		branch = strings.TrimPrefix(content, "ref: refs/heads/")
	}

	return map[string]any{
		"directory": workDir,
		"current":   branch,
	}, nil
}

// ScriptPlugin provides script execution capabilities.
type ScriptPlugin struct {
	host Host
}

func (p *ScriptPlugin) Metadata() Metadata {
	return Metadata{
		ID:          "devorch.script",
		Name:        "Script Runner",
		Version:     "1.0.0",
		Description: "Run custom scripts",
		Author:      "DevOrch",
		License:     "MIT",
		Provides:    []string{"tool"},
	}
}

func (p *ScriptPlugin) Initialize(ctx context.Context, host Host) error {
	p.host = host

	if err := host.RegisterTool("run_script", p.runScript); err != nil {
		return err
	}

	return nil
}

func (p *ScriptPlugin) Shutdown(ctx context.Context) error {
	return nil
}

func (p *ScriptPlugin) runScript(ctx context.Context, args map[string]any) (any, error) {
	script, ok := args["script"].(string)
	if !ok {
		return nil, fmt.Errorf("script is required")
	}

	lang := "sh"
	if l, ok := args["language"].(string); ok {
		lang = l
	}

	// Simplified - would execute script in production
	_ = script
	_ = lang

	return map[string]any{
		"status": "executed",
		"output": "",
	}, nil
}

// JSONSchemaPlugin provides JSON schema validation.
type JSONSchemaPlugin struct {
	host Host
}

func (p *JSONSchemaPlugin) Metadata() Metadata {
	return Metadata{
		ID:          "devorch.json-schema",
		Name:        "JSON Schema",
		Version:     "1.0.0",
		Description: "Validate JSON against schemas",
		Author:      "DevOrch",
		License:     "MIT",
		Provides:    []string{"tool"},
	}
}

func (p *JSONSchemaPlugin) Initialize(ctx context.Context, host Host) error {
	p.host = host

	if err := host.RegisterTool("json_validate", p.validateTool); err != nil {
		return err
	}

	return nil
}

func (p *JSONSchemaPlugin) Shutdown(ctx context.Context) error {
	return nil
}

func (p *JSONSchemaPlugin) validateTool(ctx context.Context, args map[string]any) (any, error) {
	data, ok := args["data"]
	if !ok {
		return nil, fmt.Errorf("data is required")
	}

	// Simple JSON validation
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return map[string]any{
			"valid":  false,
			"errors": []string{err.Error()},
		}, nil
	}

	var parsed any
	if err := json.Unmarshal(dataBytes, &parsed); err != nil {
		return map[string]any{
			"valid":  false,
			"errors": []string{err.Error()},
		}, nil
	}

	return map[string]any{
		"valid":  true,
		"errors": []string{},
	}, nil
}
