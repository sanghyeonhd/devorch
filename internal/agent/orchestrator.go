package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// Task represents a unit of work for an agent
type Task struct {
	ID          string          `json:"id"`
	AgentName   string          `json:"agent_name"`
	ParentID    string          `json:"parent_id,omitempty"`
	Description string          `json:"description"`
	Input       json.RawMessage `json:"input,omitempty"`
	Status      TaskStatus      `json:"status"`
	Priority    int             `json:"priority"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Result      any             `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// TaskResult contains the output of a completed task
type TaskResult struct {
	TaskID    string        `json:"task_id"`
	AgentName string        `json:"agent_name"`
	Success   bool          `json:"success"`
	Output    any           `json:"output,omitempty"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration_ms"`
	Subtasks  []*TaskResult `json:"subtasks,omitempty"`
}

// ExecutionPlan represents a planned sequence of tasks
type ExecutionPlan struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Tasks       []*Task   `json:"tasks"`
	CreatedAt   time.Time `json:"created_at"`
}

// Orchestrator coordinates multiple agents to complete complex tasks
type Orchestrator struct {
	Registry *Registry

	// ExecuteFunc is called to execute a task with an agent
	// This should be provided by the runtime
	ExecuteFunc func(ctx context.Context, agent *Agent, task *Task) (*TaskResult, error)

	// MaxConcurrent limits concurrent task execution
	MaxConcurrent int

	// TaskTimeout is the default timeout for tasks
	TaskTimeout time.Duration
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(registry *Registry) *Orchestrator {
	return &Orchestrator{
		Registry:      registry,
		MaxConcurrent: 3,
		TaskTimeout:   5 * time.Minute,
	}
}

// Execute runs a task with the appropriate agent
func (o *Orchestrator) Execute(ctx context.Context, task *Task) (*TaskResult, error) {
	if o.ExecuteFunc == nil {
		return nil, fmt.Errorf("ExecuteFunc not configured")
	}

	agent, ok := o.Registry.Get(task.AgentName)
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", task.AgentName)
	}

	// Apply timeout
	timeout := o.TaskTimeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Mark task as running
	now := time.Now()
	task.Status = TaskRunning
	task.StartedAt = &now

	// Execute
	result, err := o.ExecuteFunc(ctx, agent, task)

	// Update task status
	completed := time.Now()
	task.CompletedAt = &completed

	if err != nil {
		task.Status = TaskFailed
		task.Error = err.Error()
		if result == nil {
			result = &TaskResult{
				TaskID:    task.ID,
				AgentName: task.AgentName,
				Success:   false,
				Error:     err.Error(),
				Duration:  completed.Sub(now),
			}
		}
		return result, err
	}

	task.Status = TaskCompleted
	task.Result = result.Output

	return result, nil
}

// ExecutePlan executes all tasks in a plan
func (o *Orchestrator) ExecutePlan(ctx context.Context, plan *ExecutionPlan) ([]*TaskResult, error) {
	results := make([]*TaskResult, 0, len(plan.Tasks))

	for _, task := range plan.Tasks {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		result, err := o.Execute(ctx, task)
		results = append(results, result)

		if err != nil {
			// Decide whether to continue or abort on error
			if task.Priority <= 1 {
				// High priority task failed, abort
				return results, fmt.Errorf("critical task %s failed: %w", task.ID, err)
			}
		}
	}

	return results, nil
}

// DelegateTask creates and executes a subtask for another agent
func (o *Orchestrator) DelegateTask(ctx context.Context, parentTask *Task, agentName, description string, input json.RawMessage) (*TaskResult, error) {
	subtask := &Task{
		ID:          fmt.Sprintf("%s-sub-%d", parentTask.ID, time.Now().UnixNano()),
		AgentName:   agentName,
		ParentID:    parentTask.ID,
		Description: description,
		Input:       input,
		Status:      TaskPending,
		Priority:    parentTask.Priority + 1,
		CreatedAt:   time.Now(),
	}

	return o.Execute(ctx, subtask)
}

// SelectAgent chooses the best agent for a task based on description
func (o *Orchestrator) SelectAgent(description string) *Agent {
	// Simple keyword-based selection
	// In production, this could use embedding similarity or LLM

	keywords := map[string]string{
		"write":     "coder",
		"implement": "coder",
		"create":    "coder",
		"fix":       "coder",
		"debug":     "oracle",
		"analyze":   "oracle",
		"review":    "reviewer",
		"search":    "explorer",
		"find":      "explorer",
		"document":  "documenter",
		"readme":    "documenter",
	}

	for keyword, agentName := range keywords {
		if containsWord(description, keyword) {
			if agent, ok := o.Registry.Get(agentName); ok {
				return agent
			}
		}
	}

	// Default to orchestrator
	if agent, ok := o.Registry.Get("orchestrator"); ok {
		return agent
	}

	return nil
}

// containsWord checks if text contains a word (case insensitive)
func containsWord(text, word string) bool {
	// Simple substring check - could be improved with word boundaries
	lower := []byte(text)
	wordLower := []byte(word)

	for i := range lower {
		if lower[i] >= 'A' && lower[i] <= 'Z' {
			lower[i] += 32
		}
	}
	for i := range wordLower {
		if wordLower[i] >= 'A' && wordLower[i] <= 'Z' {
			wordLower[i] += 32
		}
	}

	for i := 0; i <= len(lower)-len(wordLower); i++ {
		match := true
		for j := 0; j < len(wordLower); j++ {
			if lower[i+j] != wordLower[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
