// Package tools provides the sourcegraph tool implementation
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"devorch/internal/tool"
)

// SourcegraphParams represents parameters for the sourcegraph tool
type SourcegraphParams struct {
	Query         string `json:"query"`
	Count         int    `json:"count"`
	ContextWindow int    `json:"context_window"`
	Timeout       int    `json:"timeout"` // milliseconds
}

// SourcegraphTool implements code search via Sourcegraph
type SourcegraphTool struct {
	httpClient *http.Client
}

const (
	SourcegraphToolName = "sourcegraph"
	sourcegraphURL      = "https://sourcegraph.com/.api/search/stream"
	defaultCount        = 10
	defaultContextLines = 3
	defaultTimeout      = 30000 // 30 seconds
	maxResults          = 50
)

// NewSourcegraphTool creates a new sourcegraph tool
func NewSourcegraphTool() *SourcegraphTool {
	return &SourcegraphTool{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Info returns tool information
func (t *SourcegraphTool) Info() tool.ToolInfo {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query. Supports Sourcegraph query syntax (e.g., 'repo:', 'file:', 'lang:', 'type:')",
			},
			"count": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results to return (default: 10, max: 50)",
			},
			"context_window": map[string]any{
				"type":        "integer",
				"description": "Number of context lines around matches (default: 3)",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Timeout in milliseconds (default: 30000)",
			},
		},
		"required": []string{"query"},
	}
	schemaJSON, _ := json.Marshal(schema)

	return tool.ToolInfo{
		Name: SourcegraphToolName,
		Description: `Search code across public repositories using Sourcegraph.

WHEN TO USE THIS TOOL:
- Use when searching for code patterns across open source repositories
- Great for finding implementation examples
- Helpful for discovering how APIs are used in real projects

SOURCEGRAPH QUERY SYNTAX:
- repo:github.com/owner/repo - Search in specific repository
- file:*.go - Search in files matching pattern
- lang:go - Search in specific language
- type:symbol - Search for symbol definitions
- type:file - Search for file names
- case:yes - Case sensitive search

EXAMPLES:
- "repo:kubernetes/kubernetes func main" - Search main functions in Kubernetes
- "lang:python import pandas" - Find Python files importing pandas
- "type:symbol Logger" - Find Logger symbol definitions

TIPS:
- Use specific file extensions to narrow results
- Add repo: filters for more targeted searches
- Use type:symbol to find function/method definitions
- Use type:file to find relevant files`,
		Parameters: schemaJSON,
	}
}

// Execute runs the sourcegraph tool
func (t *SourcegraphTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p SourcegraphParams
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

	// Set defaults
	count := p.Count
	if count <= 0 {
		count = defaultCount
	}
	if count > maxResults {
		count = maxResults
	}

	contextLines := p.ContextWindow
	if contextLines <= 0 {
		contextLines = defaultContextLines
	}

	timeout := p.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	// Create search URL
	searchURL := fmt.Sprintf("%s?q=%s&v=V3&t=literal&display=%d",
		sourcegraphURL,
		url.QueryEscape(p.Query),
		count,
	)

	// Create request with timeout
	reqCtx, cancel := context.WithTimeout(ctx.Ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", searchURL, nil)
	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error creating request: %s", err),
			ExitCode: 1,
		}, nil
	}

	req.Header.Set("Accept", "text/event-stream")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error making request: %s", err),
			ExitCode: 1,
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &tool.Result{
			Output:   fmt.Sprintf("Search failed with status %d: %s", resp.StatusCode, string(body)),
			ExitCode: 1,
		}, nil
	}

	// Parse streaming response
	results, err := parseSourcegraphResponse(resp.Body, count, contextLines)
	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error parsing response: %s", err),
			ExitCode: 1,
		}, nil
	}

	if len(results) == 0 {
		return &tool.Result{
			Output: fmt.Sprintf("No results found for query: %s", p.Query),
		}, nil
	}

	return &tool.Result{
		Output: formatSourcegraphResults(results, p.Query),
	}, nil
}

// SourcegraphResult represents a single search result from Sourcegraph
type SourcegraphResult struct {
	Repository string
	FilePath   string
	LineNumber int
	Content    string
	Language   string
}

func parseSourcegraphResponse(body io.Reader, maxResults, contextLines int) ([]SourcegraphResult, error) {
	var results []SourcegraphResult

	// Read all response body
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	// Parse SSE events
	events := strings.Split(string(data), "\n\n")
	for _, event := range events {
		if !strings.HasPrefix(event, "event: matches") {
			continue
		}

		// Extract data
		lines := strings.Split(event, "\n")
		for _, line := range lines {
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			jsonData := strings.TrimPrefix(line, "data: ")
			var matches []map[string]any
			if err := json.Unmarshal([]byte(jsonData), &matches); err != nil {
				continue
			}

			for _, match := range matches {
				result := parseMatch(match)
				if result != nil {
					results = append(results, *result)
					if len(results) >= maxResults {
						return results, nil
					}
				}
			}
		}
	}

	return results, nil
}

func parseMatch(match map[string]any) *SourcegraphResult {
	result := &SourcegraphResult{}

	if repo, ok := match["repository"].(string); ok {
		result.Repository = repo
	}

	if path, ok := match["path"].(string); ok {
		result.FilePath = path
	}

	if lineMatches, ok := match["lineMatches"].([]any); ok && len(lineMatches) > 0 {
		if firstMatch, ok := lineMatches[0].(map[string]any); ok {
			if lineNum, ok := firstMatch["lineNumber"].(float64); ok {
				result.LineNumber = int(lineNum)
			}
			if preview, ok := firstMatch["preview"].(string); ok {
				result.Content = preview
			}
		}
	}

	if chunkMatches, ok := match["chunkMatches"].([]any); ok && len(chunkMatches) > 0 {
		if firstChunk, ok := chunkMatches[0].(map[string]any); ok {
			if content, ok := firstChunk["content"].(string); ok {
				result.Content = content
			}
		}
	}

	if result.Repository == "" || result.FilePath == "" {
		return nil
	}

	return result
}

func formatSourcegraphResults(results []SourcegraphResult, query string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Sourcegraph Search Results for: %s\n\n", query))
	sb.WriteString(fmt.Sprintf("Found %d results:\n\n", len(results)))

	for i, result := range results {
		sb.WriteString(fmt.Sprintf("## %d. %s\n", i+1, result.Repository))
		sb.WriteString(fmt.Sprintf("**File:** %s", result.FilePath))
		if result.LineNumber > 0 {
			sb.WriteString(fmt.Sprintf(" (line %d)", result.LineNumber))
		}
		sb.WriteString("\n\n")

		if result.Content != "" {
			sb.WriteString("```\n")
			sb.WriteString(strings.TrimSpace(result.Content))
			sb.WriteString("\n```\n\n")
		}

		sb.WriteString(fmt.Sprintf("[View on Sourcegraph](https://sourcegraph.com/%s/-/blob/%s)\n\n",
			result.Repository, result.FilePath))
		sb.WriteString("---\n\n")
	}

	return sb.String()
}
