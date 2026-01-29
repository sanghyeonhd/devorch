package builtin

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"devorch/internal/tool"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusBlocked    TaskStatus = "blocked"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// Task represents a single task
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	Priority    int        `json:"priority"` // 1=high, 2=medium, 3=low
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Parent      string     `json:"parent,omitempty"` // parent task ID for subtasks
	Tags        []string   `json:"tags,omitempty"`
}

// TaskParams defines parameters for task operations
type TaskParams struct {
	Action      string     `json:"action"`                // create, update, delete, list, get
	ID          string     `json:"id,omitempty"`          // task ID for update/delete/get
	Title       string     `json:"title,omitempty"`       // for create/update
	Description string     `json:"description,omitempty"` // for create/update
	Status      TaskStatus `json:"status,omitempty"`      // for update
	Priority    int        `json:"priority,omitempty"`    // for create/update (1-3)
	Parent      string     `json:"parent,omitempty"`      // parent task ID
	Tags        []string   `json:"tags,omitempty"`        // tags
	Filter      string     `json:"filter,omitempty"`      // status filter for list
}

// TaskTool manages tasks for AI agents
type TaskTool struct {
	mu     sync.RWMutex
	tasks  map[string]*Task
	nextID int
}

// NewTaskTool creates a new task tool
func NewTaskTool() *TaskTool {
	return &TaskTool{
		tasks:  make(map[string]*Task),
		nextID: 1,
	}
}

// Info returns tool metadata
func (t *TaskTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "task",
		Description: `Manage tasks and subtasks for complex operations.
Use this to break down complex work into trackable tasks.

Actions:
- create: Create a new task
- update: Update task status/details
- delete: Remove a task
- list: List all tasks (optionally filtered by status)
- get: Get details of a specific task

Examples:
  - {"action": "create", "title": "Implement feature X", "priority": 1}
  - {"action": "update", "id": "task-1", "status": "completed"}
  - {"action": "list", "filter": "pending"}
  - {"action": "create", "title": "Add tests", "parent": "task-1"}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"action": {
					"type": "string",
					"enum": ["create", "update", "delete", "list", "get"],
					"description": "Action to perform"
				},
				"id": {
					"type": "string",
					"description": "Task ID (for update/delete/get)"
				},
				"title": {
					"type": "string",
					"description": "Task title (for create/update)"
				},
				"description": {
					"type": "string",
					"description": "Task description"
				},
				"status": {
					"type": "string",
					"enum": ["pending", "in_progress", "completed", "blocked", "cancelled"],
					"description": "Task status"
				},
				"priority": {
					"type": "integer",
					"minimum": 1,
					"maximum": 3,
					"description": "Priority (1=high, 2=medium, 3=low)"
				},
				"parent": {
					"type": "string",
					"description": "Parent task ID for subtasks"
				},
				"tags": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Tags for categorization"
				},
				"filter": {
					"type": "string",
					"description": "Status filter for list action"
				}
			},
			"required": ["action"]
		}`),
	}
}

// Execute performs task operations
func (t *TaskTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p TaskParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	switch p.Action {
	case "create":
		return t.createTask(p)
	case "update":
		return t.updateTask(p)
	case "delete":
		return t.deleteTask(p)
	case "list":
		return t.listTasks(p)
	case "get":
		return t.getTask(p)
	default:
		return nil, fmt.Errorf("unknown action: %s", p.Action)
	}
}

func (t *TaskTool) createTask(p TaskParams) (*tool.Result, error) {
	if p.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	id := fmt.Sprintf("task-%d", t.nextID)
	t.nextID++

	priority := p.Priority
	if priority < 1 || priority > 3 {
		priority = 2 // default medium
	}

	task := &Task{
		ID:          id,
		Title:       p.Title,
		Description: p.Description,
		Status:      TaskStatusPending,
		Priority:    priority,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Parent:      p.Parent,
		Tags:        p.Tags,
	}

	t.tasks[id] = task

	return &tool.Result{
		Output: fmt.Sprintf("Created task: %s\nID: %s\nPriority: %d", task.Title, task.ID, task.Priority),
		Title:  fmt.Sprintf("task created: %s", id),
		Metadata: map[string]any{
			"task_id": id,
			"task":    task,
		},
	}, nil
}

func (t *TaskTool) updateTask(p TaskParams) (*tool.Result, error) {
	if p.ID == "" {
		return nil, fmt.Errorf("task ID is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	task, ok := t.tasks[p.ID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", p.ID)
	}

	if p.Title != "" {
		task.Title = p.Title
	}
	if p.Description != "" {
		task.Description = p.Description
	}
	if p.Status != "" {
		task.Status = p.Status
	}
	if p.Priority >= 1 && p.Priority <= 3 {
		task.Priority = p.Priority
	}
	if p.Tags != nil {
		task.Tags = p.Tags
	}
	task.UpdatedAt = time.Now()

	return &tool.Result{
		Output: fmt.Sprintf("Updated task: %s\nStatus: %s", task.Title, task.Status),
		Title:  fmt.Sprintf("task updated: %s", p.ID),
		Metadata: map[string]any{
			"task_id": p.ID,
			"task":    task,
		},
	}, nil
}

func (t *TaskTool) deleteTask(p TaskParams) (*tool.Result, error) {
	if p.ID == "" {
		return nil, fmt.Errorf("task ID is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	task, ok := t.tasks[p.ID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", p.ID)
	}

	delete(t.tasks, p.ID)

	return &tool.Result{
		Output: fmt.Sprintf("Deleted task: %s", task.Title),
		Title:  fmt.Sprintf("task deleted: %s", p.ID),
	}, nil
}

func (t *TaskTool) listTasks(p TaskParams) (*tool.Result, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var filtered []*Task
	for _, task := range t.tasks {
		if p.Filter != "" && string(task.Status) != p.Filter {
			continue
		}
		filtered = append(filtered, task)
	}

	if len(filtered) == 0 {
		return &tool.Result{
			Output: "No tasks found.",
			Title:  "task list",
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tasks (%d):\n\n", len(filtered)))

	// Group by status
	statusOrder := []TaskStatus{TaskStatusInProgress, TaskStatusPending, TaskStatusBlocked, TaskStatusCompleted, TaskStatusCancelled}

	for _, status := range statusOrder {
		var statusTasks []*Task
		for _, task := range filtered {
			if task.Status == status {
				statusTasks = append(statusTasks, task)
			}
		}

		if len(statusTasks) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("## %s\n", strings.ToUpper(string(status))))
		for _, task := range statusTasks {
			priorityMark := ""
			switch task.Priority {
			case 1:
				priorityMark = "🔴"
			case 2:
				priorityMark = "🟡"
			case 3:
				priorityMark = "🟢"
			}

			sb.WriteString(fmt.Sprintf("- [%s] %s %s\n", task.ID, priorityMark, task.Title))
			if task.Description != "" {
				sb.WriteString(fmt.Sprintf("  %s\n", task.Description))
			}
		}
		sb.WriteString("\n")
	}

	return &tool.Result{
		Output: sb.String(),
		Title:  "task list",
		Metadata: map[string]any{
			"count": len(filtered),
			"tasks": filtered,
		},
	}, nil
}

func (t *TaskTool) getTask(p TaskParams) (*tool.Result, error) {
	if p.ID == "" {
		return nil, fmt.Errorf("task ID is required")
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	task, ok := t.tasks[p.ID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", p.ID)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Task: %s\n", task.Title))
	sb.WriteString(fmt.Sprintf("ID: %s\n", task.ID))
	sb.WriteString(fmt.Sprintf("Status: %s\n", task.Status))
	sb.WriteString(fmt.Sprintf("Priority: %d\n", task.Priority))
	if task.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", task.Description))
	}
	if task.Parent != "" {
		sb.WriteString(fmt.Sprintf("Parent: %s\n", task.Parent))
	}
	if len(task.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(task.Tags, ", ")))
	}
	sb.WriteString(fmt.Sprintf("Created: %s\n", task.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Updated: %s\n", task.UpdatedAt.Format(time.RFC3339)))

	return &tool.Result{
		Output: sb.String(),
		Title:  fmt.Sprintf("task: %s", task.ID),
		Metadata: map[string]any{
			"task": task,
		},
	}, nil
}

// Ensure TaskTool implements Tool interface
var _ tool.Tool = (*TaskTool)(nil)
