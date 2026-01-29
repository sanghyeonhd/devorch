package builtin

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"devorch/internal/tool"
)

// CodeSearchParams defines parameters for code search
type CodeSearchParams struct {
	Query         string `json:"query"`                    // Search query (supports regex)
	Path          string `json:"path,omitempty"`           // Directory to search in
	FileType      string `json:"file_type,omitempty"`      // Filter by file type (go, py, js, etc.)
	Type          string `json:"type,omitempty"`           // Search type: text, symbol, definition, reference
	Limit         int    `json:"limit,omitempty"`          // Max results (default: 50)
	Context       int    `json:"context,omitempty"`        // Context lines (default: 2)
	CaseSensitive bool   `json:"case_sensitive,omitempty"` // Case sensitive search
}

// CodeSearchResult represents a search result
type CodeSearchResult struct {
	File       string   `json:"file"`
	Line       int      `json:"line"`
	Column     int      `json:"column,omitempty"`
	Match      string   `json:"match"`
	Context    []string `json:"context,omitempty"`
	Type       string   `json:"type,omitempty"` // function, type, variable, etc.
	Confidence float64  `json:"confidence,omitempty"`
}

// CodeSearchTool performs semantic code search
type CodeSearchTool struct{}

// NewCodeSearchTool creates a new code search tool
func NewCodeSearchTool() *CodeSearchTool {
	return &CodeSearchTool{}
}

// Info returns tool metadata
func (t *CodeSearchTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "codesearch",
		Description: `Search code semantically and by pattern.

Supports multiple search types:
- text: Full text search with regex support
- symbol: Search for function/type/variable names
- definition: Find where symbols are defined
- reference: Find where symbols are used

Examples:
  - {"query": "handleRequest", "type": "definition"}
  - {"query": "TODO|FIXME", "file_type": "go"}
  - {"query": "func.*Error", "type": "symbol", "path": "./internal"}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"required": ["query"],
			"properties": {
				"query": {
					"type": "string",
					"description": "Search query (supports regex)"
				},
				"path": {
					"type": "string",
					"description": "Directory to search in (default: current directory)"
				},
				"file_type": {
					"type": "string",
					"description": "Filter by file extension (go, py, js, ts, etc.)"
				},
				"type": {
					"type": "string",
					"enum": ["text", "symbol", "definition", "reference"],
					"description": "Search type (default: text)"
				},
				"limit": {
					"type": "integer",
					"description": "Maximum results (default: 50)"
				},
				"context": {
					"type": "integer",
					"description": "Context lines around match (default: 2)"
				},
				"case_sensitive": {
					"type": "boolean",
					"description": "Case sensitive search (default: false)"
				}
			}
		}`),
	}
}

// Execute performs the code search
func (t *CodeSearchTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p CodeSearchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error parsing parameters: %s", err),
			ExitCode: 1,
		}, nil
	}

	if p.Query == "" {
		return &tool.Result{
			Output:   "query is required",
			ExitCode: 1,
		}, nil
	}

	// Defaults
	if p.Path == "" {
		p.Path = "."
	}
	if p.Limit <= 0 {
		p.Limit = 50
	}
	if p.Context < 0 {
		p.Context = 2
	}
	if p.Type == "" {
		p.Type = "text"
	}

	// Resolve path
	if !filepath.IsAbs(p.Path) {
		p.Path = filepath.Join(ctx.WorkDir, p.Path)
	}

	var results []CodeSearchResult
	var err error

	switch p.Type {
	case "symbol", "definition":
		results, err = t.searchSymbols(p)
	case "reference":
		results, err = t.searchReferences(p)
	default:
		results, err = t.searchText(p)
	}

	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Search error: %s", err),
			ExitCode: 1,
		}, nil
	}

	return t.formatResults(results, p)
}

func (t *CodeSearchTool) searchText(p CodeSearchParams) ([]CodeSearchResult, error) {
	flags := ""
	if !p.CaseSensitive {
		flags = "(?i)"
	}

	re, err := regexp.Compile(flags + p.Query)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}

	var results []CodeSearchResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	err = filepath.Walk(p.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories and common ignore patterns
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Filter by file type
		if p.FileType != "" {
			ext := strings.TrimPrefix(filepath.Ext(path), ".")
			if ext != p.FileType {
				return nil
			}
		}

		// Only search text files
		if !isTextFile(path) {
			return nil
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(path string) {
			defer wg.Done()
			defer func() { <-sem }()

			content, err := os.ReadFile(path)
			if err != nil {
				return
			}

			lines := strings.Split(string(content), "\n")
			relPath, _ := filepath.Rel(p.Path, path)

			for i, line := range lines {
				if re.MatchString(line) {
					result := CodeSearchResult{
						File:    relPath,
						Line:    i + 1,
						Match:   strings.TrimSpace(line),
						Context: getContext(lines, i, p.Context),
					}

					mu.Lock()
					if len(results) < p.Limit {
						results = append(results, result)
					}
					mu.Unlock()
				}
			}
		}(path)

		return nil
	})

	wg.Wait()
	return results, err
}

func (t *CodeSearchTool) searchSymbols(p CodeSearchParams) ([]CodeSearchResult, error) {
	var results []CodeSearchResult

	err := filepath.Walk(p.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Only Go files for now
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(p.Path, path)
		query := strings.ToLower(p.Query)

		ast.Inspect(file, func(n ast.Node) bool {
			if len(results) >= p.Limit {
				return false
			}

			var name string
			var typ string
			var pos token.Pos

			switch x := n.(type) {
			case *ast.FuncDecl:
				name = x.Name.Name
				typ = "function"
				pos = x.Name.Pos()
			case *ast.TypeSpec:
				name = x.Name.Name
				typ = "type"
				pos = x.Name.Pos()
			case *ast.ValueSpec:
				for _, id := range x.Names {
					if matchesQuery(id.Name, query, p.CaseSensitive) {
						position := fset.Position(id.Pos())
						results = append(results, CodeSearchResult{
							File:   relPath,
							Line:   position.Line,
							Column: position.Column,
							Match:  id.Name,
							Type:   "variable",
						})
					}
				}
				return true
			}

			if name != "" && matchesQuery(name, query, p.CaseSensitive) {
				position := fset.Position(pos)
				results = append(results, CodeSearchResult{
					File:   relPath,
					Line:   position.Line,
					Column: position.Column,
					Match:  name,
					Type:   typ,
				})
			}

			return true
		})

		return nil
	})

	return results, err
}

func (t *CodeSearchTool) searchReferences(p CodeSearchParams) ([]CodeSearchResult, error) {
	// First find definitions to understand what we're looking for
	defs, err := t.searchSymbols(p)
	if err != nil {
		return nil, err
	}

	// Then search for all text occurrences
	refs, err := t.searchText(p)
	if err != nil {
		return nil, err
	}

	// Combine and deduplicate
	seen := make(map[string]bool)
	var results []CodeSearchResult

	// Mark definitions
	for _, d := range defs {
		key := fmt.Sprintf("%s:%d", d.File, d.Line)
		seen[key] = true
		d.Type = "definition"
		results = append(results, d)
	}

	// Add references
	for _, r := range refs {
		key := fmt.Sprintf("%s:%d", r.File, r.Line)
		if !seen[key] {
			r.Type = "reference"
			results = append(results, r)
			seen[key] = true
		}
	}

	// Sort by file and line
	sort.Slice(results, func(i, j int) bool {
		if results[i].File != results[j].File {
			return results[i].File < results[j].File
		}
		return results[i].Line < results[j].Line
	})

	if len(results) > p.Limit {
		results = results[:p.Limit]
	}

	return results, nil
}

func (t *CodeSearchTool) formatResults(results []CodeSearchResult, p CodeSearchParams) (*tool.Result, error) {
	if len(results) == 0 {
		return &tool.Result{
			Output: fmt.Sprintf("No results found for: %s", p.Query),
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Code Search Results\n\n"))
	sb.WriteString(fmt.Sprintf("**Query:** `%s`\n", p.Query))
	sb.WriteString(fmt.Sprintf("**Type:** %s\n", p.Type))
	sb.WriteString(fmt.Sprintf("**Results:** %d\n\n", len(results)))

	// Group by file
	byFile := make(map[string][]CodeSearchResult)
	for _, r := range results {
		byFile[r.File] = append(byFile[r.File], r)
	}

	// Sort files
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, file := range files {
		fileResults := byFile[file]
		sb.WriteString(fmt.Sprintf("## %s\n\n", file))

		for _, r := range fileResults {
			typeLabel := ""
			if r.Type != "" {
				typeLabel = fmt.Sprintf(" [%s]", r.Type)
			}
			sb.WriteString(fmt.Sprintf("**Line %d**%s\n", r.Line, typeLabel))
			sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", r.Match))
		}
	}

	return &tool.Result{
		Output: sb.String(),
		Metadata: map[string]any{
			"total_results": len(results),
			"files_count":   len(byFile),
			"results":       results,
		},
	}, nil
}

func matchesQuery(name, query string, caseSensitive bool) bool {
	if !caseSensitive {
		name = strings.ToLower(name)
		query = strings.ToLower(query)
	}

	// Try exact match first
	if strings.Contains(name, query) {
		return true
	}

	// Try regex
	re, err := regexp.Compile(query)
	if err != nil {
		return false
	}
	return re.MatchString(name)
}

func getContext(lines []string, index, contextLines int) []string {
	if contextLines == 0 {
		return nil
	}

	start := index - contextLines
	if start < 0 {
		start = 0
	}
	end := index + contextLines + 1
	if end > len(lines) {
		end = len(lines)
	}

	context := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		context = append(context, lines[i])
	}
	return context
}

func isTextFile(path string) bool {
	textExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".java": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".rs": true, ".rb": true, ".php": true, ".swift": true, ".kt": true,
		".sh": true, ".bash": true, ".zsh": true,
		".md": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true,
		".toml": true, ".xml": true, ".html": true, ".css": true, ".scss": true,
		".sql": true, ".graphql": true, ".proto": true,
		".vue": true, ".svelte": true,
		".dockerfile": true, ".makefile": true,
	}

	ext := strings.ToLower(filepath.Ext(path))
	if textExts[ext] {
		return true
	}

	// Check filename
	name := strings.ToLower(filepath.Base(path))
	nameMatches := []string{"makefile", "dockerfile", "gemfile", "rakefile", "procfile"}
	for _, n := range nameMatches {
		if name == n {
			return true
		}
	}

	return false
}

var _ tool.Tool = (*CodeSearchTool)(nil)
