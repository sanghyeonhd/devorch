package builtin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"devorch/internal/tool"
)

const (
	globMaxResults = 1000
)

// GlobParams defines parameters for glob search
type GlobParams struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"` // base path
}

// GlobTool searches for files using glob patterns
type GlobTool struct {
	// RootDir is the base directory for searches
	RootDir string

	// ExcludePatterns are patterns to exclude (e.g., node_modules)
	ExcludePatterns []string
}

// Info returns tool metadata
func (t *GlobTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "glob",
		Description: `Search for files matching a glob pattern.
Returns a list of file paths matching the pattern.
Supports standard glob syntax: *, **, ?
Examples:
  - "*.go" - all Go files in current directory
  - "**/*.go" - all Go files recursively
  - "src/**/*.ts" - all TypeScript files under src
  - "*.{js,ts}" - all JS and TS files`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"pattern": {
					"type": "string",
					"description": "Glob pattern to match files"
				},
				"path": {
					"type": "string",
					"description": "Base path to search from (default: project root)"
				}
			},
			"required": ["pattern"]
		}`),
	}
}

// Execute runs the glob search
func (t *GlobTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p GlobParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	// Determine base path
	basePath := p.Path
	if basePath == "" {
		basePath = ctx.WorkDir
	}
	if basePath == "" {
		basePath = t.RootDir
	}
	if basePath == "" {
		wd, _ := os.Getwd()
		basePath = wd
	}
	if !filepath.IsAbs(basePath) {
		basePath = filepath.Join(ctx.WorkDir, basePath)
	}

	// Handle ** pattern (recursive)
	var matches []string
	var err error

	if strings.Contains(p.Pattern, "**") {
		matches, err = t.globRecursive(basePath, p.Pattern)
	} else {
		// Simple glob
		pattern := filepath.Join(basePath, p.Pattern)
		matches, err = filepath.Glob(pattern)
	}

	if err != nil {
		return nil, fmt.Errorf("glob error: %w", err)
	}

	// Filter excluded patterns
	filtered := t.filterExcluded(matches, basePath)

	// Sort and limit results
	sort.Strings(filtered)
	truncated := false
	if len(filtered) > globMaxResults {
		filtered = filtered[:globMaxResults]
		truncated = true
	}

	// Build output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d files matching '%s'\n\n", len(filtered), p.Pattern))

	for _, match := range filtered {
		relPath, _ := filepath.Rel(basePath, match)
		if relPath == "" {
			relPath = match
		}
		output.WriteString(relPath)
		output.WriteString("\n")
	}

	if truncated {
		output.WriteString(fmt.Sprintf("\n... (showing first %d results)", globMaxResults))
	}

	return &tool.Result{
		Output:    output.String(),
		Title:     fmt.Sprintf("glob: %s", p.Pattern),
		Truncated: truncated,
		Metadata: map[string]any{
			"pattern":   p.Pattern,
			"base_path": basePath,
			"count":     len(filtered),
			"truncated": truncated,
			"files":     filtered,
		},
	}, nil
}

// globRecursive handles ** patterns
func (t *GlobTool) globRecursive(basePath, pattern string) ([]string, error) {
	var matches []string

	// Split pattern by **
	parts := strings.Split(pattern, "**")

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip directories for result
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return nil
		}

		// Match pattern
		if t.matchPattern(relPath, parts, pattern) {
			matches = append(matches, path)
		}

		return nil
	})

	return matches, err
}

// matchPattern checks if path matches the ** pattern
func (t *GlobTool) matchPattern(relPath string, parts []string, fullPattern string) bool {
	// Simple case: pattern ends with /**/*.ext
	if strings.HasPrefix(fullPattern, "**/") {
		suffix := strings.TrimPrefix(fullPattern, "**/")
		matched, _ := filepath.Match(suffix, filepath.Base(relPath))
		return matched
	}

	// Pattern like src/**/*.ts
	if len(parts) == 2 {
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")

		// Check prefix
		if prefix != "" && !strings.HasPrefix(relPath, prefix) {
			return false
		}

		// Check suffix
		if suffix != "" {
			matched, _ := filepath.Match(suffix, filepath.Base(relPath))
			return matched
		}

		return true
	}

	// Fallback: simple match
	matched, _ := filepath.Match(fullPattern, relPath)
	return matched
}

// filterExcluded removes files matching exclude patterns
func (t *GlobTool) filterExcluded(matches []string, basePath string) []string {
	if len(t.ExcludePatterns) == 0 {
		// Default excludes
		t.ExcludePatterns = []string{
			"**/node_modules/**",
			"**/.git/**",
			"**/vendor/**",
			"**/__pycache__/**",
			"**/dist/**",
			"**/build/**",
			"**/.venv/**",
		}
	}

	var filtered []string
	for _, match := range matches {
		relPath, _ := filepath.Rel(basePath, match)
		excluded := false

		for _, pattern := range t.ExcludePatterns {
			if t.matchExclude(relPath, pattern) {
				excluded = true
				break
			}
		}

		if !excluded {
			filtered = append(filtered, match)
		}
	}

	return filtered
}

// matchExclude checks if path should be excluded
func (t *GlobTool) matchExclude(relPath, pattern string) bool {
	// Handle ** pattern
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")

			if prefix != "" && !strings.Contains(relPath, prefix) {
				return false
			}
			if suffix != "" && !strings.Contains(relPath, suffix) {
				return false
			}
			return true
		}
	}

	matched, _ := filepath.Match(pattern, relPath)
	return matched
}

// Name returns tool name for legacy interface
func (t *GlobTool) Name() string {
	return "glob"
}
