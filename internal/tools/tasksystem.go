// Package tools provides task delegation and subagent capabilities
package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// TaskSystem provides subagent task delegation capabilities
type TaskSystem struct {
	agentPath   string
	workingDir  string
	environment map[string]string
	timeout     time.Duration
}

// NewTaskSystem creates a new task system
func NewTaskSystem() *TaskSystem {
	return &TaskSystem{
		agentPath:   "devorch", // Default to self-delegation
		workingDir:  ".",
		environment: make(map[string]string),
		timeout:     5 * time.Minute,
	}
}

// TaskOptions configures task execution behavior
type TaskOptions struct {
	Description  string
	Instructions string
	Context      map[string]interface{}
	Timeout      time.Duration
	WorkingDir   string
	Environment  map[string]string
	AgentPath    string
	MaxRetries   int
	RequireHuman bool
	SaveProgress bool
	Background   bool
}

// DefaultTaskOptions returns sensible defaults
func DefaultTaskOptions() TaskOptions {
	return TaskOptions{
		Description:  "",
		Instructions: "",
		Context:      make(map[string]interface{}),
		Timeout:      5 * time.Minute,
		WorkingDir:   ".",
		Environment:  make(map[string]string),
		AgentPath:    "devorch",
		MaxRetries:   3,
		RequireHuman: false,
		SaveProgress: true,
		Background:   false,
	}
}

// TaskResult contains the result of a task execution
type TaskResult struct {
	TaskID      string
	Status      string // "completed", "failed", "timeout", "cancelled"
	Description string
	Output      string
	Error       string
	ExitCode    int
	Duration    time.Duration
	Context     map[string]interface{}
	Metadata    map[string]interface{}
}

// TaskContext provides execution context for tasks
type TaskContext struct {
	TaskID       string
	Description  string
	Instructions string
	WorkingDir   string
	Environment  map[string]string
	UserContext  map[string]interface{}
	StartTime    time.Time
}

// RunSubagentTask delegates a task to a subagent
func (ts *TaskSystem) RunSubagentTask(description, instructions string, options TaskOptions) (*TaskResult, error) {
	fmt.Printf("🤖 Starting subagent task: %s\n", description)

	taskID := fmt.Sprintf("task_%d", time.Now().Unix())
	startTime := time.Now()

	// Create task context
	taskCtx := &TaskContext{
		TaskID:       taskID,
		Description:  description,
		Instructions: instructions,
		WorkingDir:   options.WorkingDir,
		Environment:  options.Environment,
		UserContext:  options.Context,
		StartTime:    startTime,
	}

	// Save task context if requested
	if options.SaveProgress {
		if err := ts.saveTaskContext(taskCtx); err != nil {
			log.Printf("Warning: Failed to save task context: %v", err)
		}
	}

	// Execute with retries (simplified - no timeout for now)
	var result *TaskResult
	var err error

	for attempt := 1; attempt <= options.MaxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("🔄 Retry attempt %d/%d\n", attempt, options.MaxRetries)
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		cmd := ts.prepareSubagentCommand(taskCtx, options)
		result, err = ts.executeTask(cmd, taskCtx)

		if err == nil && result.Status == "completed" {
			break
		}

		if attempt < options.MaxRetries {
			fmt.Printf("⚠️ Attempt %d failed: %v\n", attempt, err)
		}
	}

	// Update result
	if result != nil {
		result.Duration = time.Since(startTime)

		// Save final result if requested
		if options.SaveProgress {
			if err := ts.saveTaskResult(result); err != nil {
				log.Printf("Warning: Failed to save task result: %v", err)
			}
		}
	}

	if result != nil && result.Status == "completed" {
		fmt.Printf("✅ Task completed in %v\n", result.Duration)
	} else {
		fmt.Printf("❌ Task failed after %d attempts\n", options.MaxRetries)
	}

	return result, err
}

// prepareSubagentCommand prepares the command for subagent execution
func (ts *TaskSystem) prepareSubagentCommand(taskCtx *TaskContext, options TaskOptions) *exec.Cmd {
	agentPath := options.AgentPath
	if agentPath == "" {
		agentPath = ts.agentPath
	}

	// Build arguments
	args := []string{
		"--task-mode",
		"--task-id", taskCtx.TaskID,
		"--description", taskCtx.Description,
	}

	if taskCtx.Instructions != "" {
		args = append(args, "--instructions", taskCtx.Instructions)
	}

	// Add context as JSON if available
	if len(taskCtx.UserContext) > 0 {
		contextJSON, err := json.Marshal(taskCtx.UserContext)
		if err == nil {
			args = append(args, "--context", string(contextJSON))
		}
	}

	cmd := exec.Command(agentPath, args...)

	// Set working directory
	workingDir := options.WorkingDir
	if workingDir == "" {
		workingDir = ts.workingDir
	}
	cmd.Dir = workingDir

	// Set environment
	env := os.Environ()
	for key, value := range options.Environment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	for key, value := range ts.environment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = env

	return cmd
}

// executeTask executes a single task attempt
func (ts *TaskSystem) executeTask(cmd *exec.Cmd, taskCtx *TaskContext) (*TaskResult, error) {
	output, err := cmd.CombinedOutput()

	result := &TaskResult{
		TaskID:      taskCtx.TaskID,
		Description: taskCtx.Description,
		Output:      string(output),
		Context:     taskCtx.UserContext,
		Metadata:    make(map[string]interface{}),
	}

	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()

		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		}

		return result, err
	}

	result.Status = "completed"
	result.ExitCode = 0

	return result, nil
}

// saveTaskContext saves task context to disk
func (ts *TaskSystem) saveTaskContext(taskCtx *TaskContext) error {
	contextDir := ".devorch/tasks"
	if err := os.MkdirAll(contextDir, 0755); err != nil {
		return err
	}

	contextFile := fmt.Sprintf("%s/%s_context.json", contextDir, taskCtx.TaskID)

	data, err := json.MarshalIndent(taskCtx, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(contextFile, data, 0644)
}

// saveTaskResult saves task result to disk
func (ts *TaskSystem) saveTaskResult(result *TaskResult) error {
	resultDir := ".devorch/tasks"
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		return err
	}

	resultFile := fmt.Sprintf("%s/%s_result.json", resultDir, result.TaskID)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(resultFile, data, 0644)
}

// QuickTask executes a simple task with minimal configuration
func (ts *TaskSystem) QuickTask(description, instructions string) (*TaskResult, error) {
	options := DefaultTaskOptions()
	options.Description = description
	options.Instructions = instructions
	options.MaxRetries = 1
	options.Timeout = 2 * time.Minute

	return ts.RunSubagentTask(description, instructions, options)
}

// BackgroundTask executes a task in the background
func (ts *TaskSystem) BackgroundTask(description, instructions string, options TaskOptions) error {
	options.Background = true

	go func() {
		result, err := ts.RunSubagentTask(description, instructions, options)
		if err != nil {
			log.Printf("Background task failed: %v", err)
		} else {
			log.Printf("Background task completed: %s", result.TaskID)
		}
	}()

	fmt.Printf("🚀 Started background task: %s\n", description)
	return nil
}

// ListTasks lists recent tasks
func (ts *TaskSystem) ListTasks() ([]TaskResult, error) {
	resultDir := ".devorch/tasks"

	entries, err := os.ReadDir(resultDir)
	if err != nil {
		return nil, err
	}

	var results []TaskResult

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_result.json") {
			filePath := fmt.Sprintf("%s/%s", resultDir, entry.Name())

			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			var result TaskResult
			if err := json.Unmarshal(data, &result); err != nil {
				continue
			}

			results = append(results, result)
		}
	}

	return results, nil
}

// GetTaskStatus gets the status of a specific task
func (ts *TaskSystem) GetTaskStatus(taskID string) (*TaskResult, error) {
	resultFile := fmt.Sprintf(".devorch/tasks/%s_result.json", taskID)

	data, err := os.ReadFile(resultFile)
	if err != nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	var result TaskResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// CancelTask attempts to cancel a running task
func (ts *TaskSystem) CancelTask(taskID string) error {
	// This is a simplified implementation
	// In a real system, you'd track running processes
	fmt.Printf("⚠️ Task cancellation not yet implemented: %s\n", taskID)
	return fmt.Errorf("task cancellation not implemented")
}

// CleanupTasks removes old task data
func (ts *TaskSystem) CleanupTasks(olderThan time.Duration) error {
	resultDir := ".devorch/tasks"

	entries, err := os.ReadDir(resultDir)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-olderThan)
	var removed int

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			filePath := fmt.Sprintf("%s/%s", resultDir, entry.Name())
			if err := os.Remove(filePath); err == nil {
				removed++
			}
		}
	}

	fmt.Printf("🧹 Cleaned up %d old task files\n", removed)
	return nil
}
