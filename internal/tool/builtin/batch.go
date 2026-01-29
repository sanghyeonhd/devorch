package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"devorch/internal/tool"
)

// BatchOperation represents a single operation in a batch
type BatchOperation struct {
	ID     string          `json:"id"`
	Tool   string          `json:"tool"`
	Params json.RawMessage `json:"params"`
}

// BatchResult represents the result of a single operation
type BatchResult struct {
	ID        string         `json:"id"`
	Tool      string         `json:"tool"`
	Success   bool           `json:"success"`
	Output    string         `json:"output,omitempty"`
	Error     string         `json:"error,omitempty"`
	Duration  string         `json:"duration"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// BatchParams defines parameters for the batch tool
type BatchParams struct {
	Operations  []BatchOperation `json:"operations"`              // List of operations
	Parallel    bool             `json:"parallel,omitempty"`      // Run in parallel
	MaxParallel int              `json:"max_parallel,omitempty"`  // Max parallel ops (default: 4)
	StopOnError bool             `json:"stop_on_error,omitempty"` // Stop on first error
	Timeout     int              `json:"timeout,omitempty"`       // Timeout per op in seconds
}

// BatchTool executes multiple tool operations
type BatchTool struct {
	registry tool.Registry
}

// NewBatchTool creates a new batch tool
func NewBatchTool(registry tool.Registry) *BatchTool {
	return &BatchTool{
		registry: registry,
	}
}

// Info returns tool metadata
func (t *BatchTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "batch",
		Description: `Execute multiple tool operations in batch.

Supports sequential or parallel execution with error handling.

Examples:
  - Sequential: {"operations": [{"id": "1", "tool": "read", "params": {"path": "a.go"}}, {"id": "2", "tool": "read", "params": {"path": "b.go"}}]}
  - Parallel: {"operations": [...], "parallel": true, "max_parallel": 4}
  - Stop on error: {"operations": [...], "stop_on_error": true}
  - With timeout: {"operations": [...], "timeout": 30}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"required": ["operations"],
			"properties": {
				"operations": {
					"type": "array",
					"description": "List of operations to execute",
					"items": {
						"type": "object",
						"required": ["tool", "params"],
						"properties": {
							"id": {"type": "string", "description": "Optional ID for the operation"},
							"tool": {"type": "string", "description": "Tool name to execute"},
							"params": {"type": "object", "description": "Parameters for the tool"}
						}
					}
				},
				"parallel": {
					"type": "boolean",
					"description": "Execute operations in parallel"
				},
				"max_parallel": {
					"type": "integer",
					"description": "Maximum parallel operations (default: 4)"
				},
				"stop_on_error": {
					"type": "boolean",
					"description": "Stop execution on first error"
				},
				"timeout": {
					"type": "integer",
					"description": "Timeout per operation in seconds (default: 60)"
				}
			}
		}`),
	}
}

// Execute runs the batch operations
func (t *BatchTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p BatchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return &tool.Result{
			Output:   fmt.Sprintf("Error parsing parameters: %s", err),
			ExitCode: 1,
		}, nil
	}

	if len(p.Operations) == 0 {
		return &tool.Result{
			Output:   "No operations provided",
			ExitCode: 1,
		}, nil
	}

	// Set defaults
	if p.MaxParallel <= 0 {
		p.MaxParallel = 4
	}
	if p.Timeout <= 0 {
		p.Timeout = 60
	}

	// Assign IDs if not provided
	for i := range p.Operations {
		if p.Operations[i].ID == "" {
			p.Operations[i].ID = fmt.Sprintf("op_%d", i+1)
		}
	}

	var results []BatchResult

	if p.Parallel {
		results = t.executeParallel(ctx, p)
	} else {
		results = t.executeSequential(ctx, p)
	}

	// Build summary
	return t.buildSummary(results, p)
}

func (t *BatchTool) executeSequential(ctx *tool.Context, p BatchParams) []BatchResult {
	results := make([]BatchResult, 0, len(p.Operations))

	for _, op := range p.Operations {
		result := t.executeOperation(ctx, op, time.Duration(p.Timeout)*time.Second)
		results = append(results, result)

		if p.StopOnError && !result.Success {
			break
		}
	}

	return results
}

func (t *BatchTool) executeParallel(ctx *tool.Context, p BatchParams) []BatchResult {
	results := make([]BatchResult, len(p.Operations))
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Semaphore for max parallel
	sem := make(chan struct{}, p.MaxParallel)
	stopCh := make(chan struct{})
	var stopped bool

	for i, op := range p.Operations {
		// Check if we should stop
		select {
		case <-stopCh:
			mu.Lock()
			results[i] = BatchResult{
				ID:        op.ID,
				Tool:      op.Tool,
				Success:   false,
				Error:     "Skipped due to previous error",
				Timestamp: time.Now(),
			}
			mu.Unlock()
			continue
		default:
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, op BatchOperation) {
			defer wg.Done()
			defer func() { <-sem }()

			result := t.executeOperation(ctx, op, time.Duration(p.Timeout)*time.Second)

			mu.Lock()
			results[idx] = result
			if p.StopOnError && !result.Success && !stopped {
				stopped = true
				close(stopCh)
			}
			mu.Unlock()
		}(i, op)
	}

	wg.Wait()
	return results
}

func (t *BatchTool) executeOperation(ctx *tool.Context, op BatchOperation, timeout time.Duration) BatchResult {
	start := time.Now()

	result := BatchResult{
		ID:        op.ID,
		Tool:      op.Tool,
		Timestamp: start,
	}

	// Get tool
	toolInstance, ok := t.registry.Get(op.Tool)
	if !ok || toolInstance == nil {
		result.Success = false
		result.Error = fmt.Sprintf("Tool not found: %s", op.Tool)
		result.Duration = time.Since(start).String()
		return result
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Execute with timeout
	done := make(chan struct{})
	var toolResult *tool.Result
	var toolErr error

	go func() {
		toolResult, toolErr = toolInstance.Execute(ctx, op.Params)
		close(done)
	}()

	select {
	case <-execCtx.Done():
		result.Success = false
		result.Error = "Operation timed out"
	case <-done:
		if toolErr != nil {
			result.Success = false
			result.Error = toolErr.Error()
		} else if toolResult != nil {
			result.Success = toolResult.ExitCode == 0
			result.Output = toolResult.Output
			result.Metadata = toolResult.Metadata
			if toolResult.ExitCode != 0 {
				result.Error = fmt.Sprintf("Exit code: %d", toolResult.ExitCode)
			}
		}
	}

	result.Duration = time.Since(start).String()
	return result
}

func (t *BatchTool) buildSummary(results []BatchResult, p BatchParams) (*tool.Result, error) {
	var sb strings.Builder

	// Count successes and failures
	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	// Header
	sb.WriteString("# Batch Execution Results\n\n")
	sb.WriteString(fmt.Sprintf("**Total Operations:** %d\n", len(p.Operations)))
	sb.WriteString(fmt.Sprintf("**Successful:** %d\n", successCount))
	sb.WriteString(fmt.Sprintf("**Failed:** %d\n", failCount))
	sb.WriteString(fmt.Sprintf("**Mode:** %s\n\n", map[bool]string{true: "Parallel", false: "Sequential"}[p.Parallel]))

	// Results table
	sb.WriteString("## Results\n\n")
	sb.WriteString("| ID | Tool | Status | Duration | Details |\n")
	sb.WriteString("|---|---|---|---|---|\n")

	for _, r := range results {
		status := "✅"
		details := truncateString(r.Output, 50)
		if !r.Success {
			status = "❌"
			details = r.Error
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			r.ID, r.Tool, status, r.Duration, truncateStringBatch(details, 50)))
	}

	// Detailed output for failures
	if failCount > 0 {
		sb.WriteString("\n## Failures\n\n")
		for _, r := range results {
			if !r.Success {
				sb.WriteString(fmt.Sprintf("### %s (%s)\n", r.ID, r.Tool))
				sb.WriteString(fmt.Sprintf("**Error:** %s\n\n", r.Error))
			}
		}
	}

	exitCode := 0
	if failCount > 0 {
		exitCode = 1
	}

	return &tool.Result{
		Output:   sb.String(),
		ExitCode: exitCode,
		Metadata: map[string]any{
			"total":    len(p.Operations),
			"success":  successCount,
			"failed":   failCount,
			"parallel": p.Parallel,
			"results":  results,
		},
	}, nil
}

func truncateStringBatch(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

var _ tool.Tool = (*BatchTool)(nil)
