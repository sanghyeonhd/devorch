// Package tools provides the agent tool implementation (sub-agent for parallel tasks)
package builtin

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"devorch/internal/tool"
)

// AgentParams represents parameters for the agent tool
type AgentParams struct {
	Prompt string `json:"prompt"`
}

// AgentTool implements launching sub-agents for parallel task execution
type AgentTool struct {
	// Tools available to the sub-agent (limited set)
	tools []tool.Tool
	mu    sync.Mutex
}

const AgentToolName = "agent"

// NewAgentTool creates a new agent tool with limited tool access
func NewAgentTool(availableTools []tool.Tool) *AgentTool {
	// Filter to only safe, read-only tools
	var safeTools []tool.Tool
	allowedTools := map[string]bool{
		"glob":        true,
		"grep":        true,
		"ls":          true,
		"view":        true,
		"sourcegraph": true,
	}

	for _, t := range availableTools {
		if allowedTools[t.Info().Name] {
			safeTools = append(safeTools, t)
		}
	}

	return &AgentTool{
		tools: safeTools,
	}
}

// Info returns tool information
func (t *AgentTool) Info() tool.ToolInfo {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The task description for the sub-agent to perform",
			},
		},
		"required": []string{"prompt"},
	}
	schemaJSON, _ := json.Marshal(schema)

	return tool.ToolInfo{
		Name: AgentToolName,
		Description: `Launch a new agent that has access to read-only tools: glob, grep, ls, view, sourcegraph.

WHEN TO USE THIS TOOL:
- When searching for a keyword or file and are not confident that you will find the right match on the first try
- For parallel searches across multiple directories or patterns
- When you need to explore the codebase to answer a question

WHEN NOT TO USE THIS TOOL:
- If you want to read a specific file path, use the view or glob tool instead
- If you are searching for a specific class definition like "class Foo", use the glob tool instead

EXAMPLES:
- "Find all files that import the logging module"
- "Search for the configuration file that contains database settings"
- "Which file implements the UserService interface?"

USAGE NOTES:
1. Launch multiple agents concurrently whenever possible to maximize performance
2. The agent will return a single message with the results
3. Agent responses should generally be trusted
4. The agent is stateless - each invocation is independent

The sub-agent has access to:
- glob: Find files by pattern
- grep: Search file contents
- ls: List directory contents
- view: View file contents
- sourcegraph: Search public code repositories`,
		Parameters: schemaJSON,
	}
}

// Execute runs the agent tool
func (t *AgentTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p AgentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error parsing parameters: %s", err),
			ExitCode: 1,
		}, nil
	}

	if p.Prompt == "" {
		return &tool.Result{
			Output:   "prompt is required",
			ExitCode: 1,
		}, nil
	}

	// Execute the sub-agent task
	// In a real implementation, this would use the LLM to reason and call tools
	// For now, we implement a simplified version that executes based on the prompt

	result, err := t.executeSubAgent(ctx, p.Prompt)
	if err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Sub-agent error: %s", err),
			ExitCode: 1,
		}, nil
	}

	return &tool.Result{
		Output: result,
	}, nil
}

func (t *AgentTool) executeSubAgent(ctx *tool.Context, prompt string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var results []string
	promptLower := strings.ToLower(prompt)

	// Simple keyword-based task routing
	// In a full implementation, this would use an LLM to plan and execute

	// File search tasks
	if strings.Contains(promptLower, "find file") || strings.Contains(promptLower, "search file") ||
		strings.Contains(promptLower, "which file") || strings.Contains(promptLower, "locate") {
		// Use glob to find files
		for _, tool := range t.tools {
			if tool.Info().Name == "glob" {
				// Extract potential patterns from prompt
				pattern := extractPattern(prompt)
				params, _ := json.Marshal(map[string]string{"pattern": pattern})
				result, err := tool.Execute(ctx, params)
				if err == nil {
					results = append(results, fmt.Sprintf("File search results:\n%s", result.Output))
				}
				break
			}
		}
	}

	// Content search tasks
	if strings.Contains(promptLower, "search for") || strings.Contains(promptLower, "find") ||
		strings.Contains(promptLower, "grep") || strings.Contains(promptLower, "contain") ||
		strings.Contains(promptLower, "import") || strings.Contains(promptLower, "implement") {
		// Use grep to search content
		for _, tool := range t.tools {
			if tool.Info().Name == "grep" {
				pattern := extractSearchTerm(prompt)
				params, _ := json.Marshal(map[string]string{"pattern": pattern})
				result, err := tool.Execute(ctx, params)
				if err == nil {
					results = append(results, fmt.Sprintf("Content search results:\n%s", result.Output))
				}
				break
			}
		}
	}

	// Directory listing tasks
	if strings.Contains(promptLower, "list") || strings.Contains(promptLower, "directory") ||
		strings.Contains(promptLower, "structure") || strings.Contains(promptLower, "what files") {
		// Use ls to list directory
		for _, tool := range t.tools {
			if tool.Info().Name == "ls" {
				path := extractPath(prompt)
				params, _ := json.Marshal(map[string]string{"path": path})
				result, err := tool.Execute(ctx, params)
				if err == nil {
					results = append(results, fmt.Sprintf("Directory listing:\n%s", result.Output))
				}
				break
			}
		}
	}

	if len(results) == 0 {
		// Default: try grep with the main keywords
		for _, tool := range t.tools {
			if tool.Info().Name == "grep" {
				keywords := extractKeywords(prompt)
				if keywords != "" {
					params, _ := json.Marshal(map[string]string{"pattern": keywords})
					result, err := tool.Execute(ctx, params)
					if err == nil {
						results = append(results, fmt.Sprintf("Search results for '%s':\n%s", keywords, result.Output))
					}
				}
				break
			}
		}
	}

	if len(results) == 0 {
		return fmt.Sprintf("Sub-agent completed task: %s\n\nNo specific results found. You may need to provide more specific search criteria.", prompt), nil
	}

	return fmt.Sprintf("Sub-agent completed task: %s\n\n%s", prompt, strings.Join(results, "\n\n")), nil
}

func extractPattern(prompt string) string {
	// Simple pattern extraction

	promptLower := strings.ToLower(prompt)
	for _, ext := range []string{"go", "python", "javascript", "typescript", "java", "rust", "c", "cpp"} {
		if strings.Contains(promptLower, ext) {
			switch ext {
			case "go":
				return "**/*.go"
			case "python":
				return "**/*.py"
			case "javascript":
				return "**/*.js"
			case "typescript":
				return "**/*.ts"
			case "java":
				return "**/*.java"
			case "rust":
				return "**/*.rs"
			case "c":
				return "**/*.c"
			case "cpp":
				return "**/*.cpp"
			}
		}
	}

	// Default pattern
	return "**/*"
}

func extractSearchTerm(prompt string) string {
	// Extract quoted strings first
	if idx := strings.Index(prompt, `"`); idx != -1 {
		endIdx := strings.Index(prompt[idx+1:], `"`)
		if endIdx != -1 {
			return prompt[idx+1 : idx+1+endIdx]
		}
	}

	// Extract words after common keywords
	keywords := []string{"search for", "find", "import", "contain", "implement"}
	promptLower := strings.ToLower(prompt)

	for _, kw := range keywords {
		if idx := strings.Index(promptLower, kw); idx != -1 {
			rest := strings.TrimSpace(prompt[idx+len(kw):])
			words := strings.Fields(rest)
			if len(words) > 0 {
				// Return first meaningful word
				for _, w := range words {
					if len(w) > 2 && !isStopWord(w) {
						return w
					}
				}
			}
		}
	}

	// Fallback: extract longest word
	return extractKeywords(prompt)
}

func extractPath(prompt string) string {
	// Look for path-like strings
	words := strings.Fields(prompt)
	for _, w := range words {
		if strings.Contains(w, "/") || strings.HasPrefix(w, ".") {
			return w
		}
	}
	return "."
}

func extractKeywords(prompt string) string {
	words := strings.Fields(prompt)
	var keywords []string

	for _, w := range words {
		w = strings.Trim(w, ".,?!\"'")
		if len(w) > 3 && !isStopWord(w) {
			keywords = append(keywords, w)
		}
	}

	if len(keywords) > 0 {
		// Return the longest keyword (likely most specific)
		longest := keywords[0]
		for _, k := range keywords[1:] {
			if len(k) > len(longest) {
				longest = k
			}
		}
		return longest
	}

	return ""
}

func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"that": true, "this": true, "these": true, "those": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true,
		"file": true, "files": true, "find": true, "search": true,
		"all": true, "any": true, "which": true, "what": true, "where": true,
	}
	return stopWords[strings.ToLower(word)]
}
