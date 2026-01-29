package builtin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"devorch/internal/tool"
)

const (
	grepMaxResults    = 500
	grepMaxPerFile    = 50
	grepContextLines  = 2
	grepMaxLineLength = 200
)

// GrepParams defines parameters for grep search
type GrepParams struct {
	Pattern  string `json:"pattern"`
	Path     string `json:"path,omitempty"`          // directory to search
	Include  string `json:"include,omitempty"`       // file pattern to include (e.g., "*.go")
	IsRegex  bool   `json:"isRegex,omitempty"`       // treat pattern as regex
	CaseSens bool   `json:"caseSensitive,omitempty"` // case sensitive search
}

// GrepMatch represents a single match
type GrepMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
	Context string `json:"context,omitempty"`
}

// GrepTool searches for text patterns in files
type GrepTool struct {
	// RootDir is the base directory for searches
	RootDir string

	// ExcludePatterns are patterns to exclude
	ExcludePatterns []string
}

// Info returns tool metadata
func (t *GrepTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "grep",
		Description: `Search for text patterns in files.
Returns matching lines with file paths and line numbers.
Supports both literal text and regular expression patterns.
Examples:
  - Search for "TODO": {"pattern": "TODO"}
  - Search in Go files: {"pattern": "func main", "include": "*.go"}
  - Regex search: {"pattern": "func\\s+\\w+\\(", "isRegex": true}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"pattern": {
					"type": "string",
					"description": "Text or regex pattern to search for"
				},
				"path": {
					"type": "string",
					"description": "Directory to search in (default: project root)"
				},
				"include": {
					"type": "string",
					"description": "File pattern to include (e.g., '*.go', '*.ts')"
				},
				"isRegex": {
					"type": "boolean",
					"description": "Treat pattern as regular expression"
				},
				"caseSensitive": {
					"type": "boolean",
					"description": "Case sensitive search (default: false)"
				}
			},
			"required": ["pattern"]
		}`),
	}
}

// Execute runs the grep search
func (t *GrepTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p GrepParams
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

	// Compile pattern
	var re *regexp.Regexp
	var err error

	pattern := p.Pattern
	if !p.IsRegex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if !p.CaseSens {
		pattern = "(?i)" + pattern
	}

	re, err = regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	// Find matching files
	var matches []GrepMatch
	totalMatches := 0
	truncated := false

	err = filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories
		if info.IsDir() {
			// Skip excluded directories
			name := info.Name()
			for _, ex := range t.getExcludeDirs() {
				if name == ex {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check include pattern
		if p.Include != "" {
			matched, _ := filepath.Match(p.Include, filepath.Base(path))
			if !matched {
				return nil
			}
		}

		// Skip binary files
		if isBinaryFile(path) {
			return nil
		}

		// Search in file
		fileMatches, err := t.searchFile(path, basePath, re)
		if err != nil {
			return nil
		}

		totalMatches += len(fileMatches)

		// Limit matches per file
		if len(fileMatches) > grepMaxPerFile {
			fileMatches = fileMatches[:grepMaxPerFile]
		}

		matches = append(matches, fileMatches...)

		// Check total limit
		if len(matches) >= grepMaxResults {
			truncated = true
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, fmt.Errorf("search error: %w", err)
	}

	// Sort by file then line
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].File != matches[j].File {
			return matches[i].File < matches[j].File
		}
		return matches[i].Line < matches[j].Line
	})

	// Build output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d matches for '%s'\n\n", totalMatches, p.Pattern))

	currentFile := ""
	for _, m := range matches {
		if m.File != currentFile {
			if currentFile != "" {
				output.WriteString("\n")
			}
			output.WriteString(fmt.Sprintf("## %s\n", m.File))
			currentFile = m.File
		}
		output.WriteString(fmt.Sprintf("%d: %s\n", m.Line, m.Content))
	}

	if truncated {
		output.WriteString(fmt.Sprintf("\n... (showing first %d results of %d)", len(matches), totalMatches))
	}

	return &tool.Result{
		Output:    output.String(),
		Title:     fmt.Sprintf("grep: %s", p.Pattern),
		Truncated: truncated,
		Metadata: map[string]any{
			"pattern":       p.Pattern,
			"total_matches": totalMatches,
			"shown_matches": len(matches),
			"truncated":     truncated,
			"matches":       matches,
		},
	}, nil
}

// searchFile searches for pattern in a single file
func (t *GrepTool) searchFile(filePath, basePath string, re *regexp.Regexp) ([]GrepMatch, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	relPath, _ := filepath.Rel(basePath, filePath)
	if relPath == "" {
		relPath = filePath
	}

	var matches []GrepMatch
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if re.MatchString(line) {
			// Truncate long lines
			content := line
			if len(content) > grepMaxLineLength {
				content = content[:grepMaxLineLength] + "..."
			}

			matches = append(matches, GrepMatch{
				File:    relPath,
				Line:    lineNum,
				Content: strings.TrimSpace(content),
			})

			if len(matches) >= grepMaxPerFile {
				break
			}
		}
	}

	return matches, scanner.Err()
}

// getExcludeDirs returns directories to exclude
func (t *GrepTool) getExcludeDirs() []string {
	defaults := []string{
		"node_modules",
		".git",
		"vendor",
		"__pycache__",
		".venv",
		"dist",
		"build",
		".next",
		".cache",
	}

	if len(t.ExcludePatterns) > 0 {
		return append(defaults, t.ExcludePatterns...)
	}
	return defaults
}

// Name returns tool name for legacy interface
func (t *GrepTool) Name() string {
	return "grep"
}
