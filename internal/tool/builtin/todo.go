package builtin

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"devorch/internal/tool"
)

// TodoItem represents a single todo item
type TodoItem struct {
	ID        int       `json:"id"`
	Text      string    `json:"text"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
	DoneAt    time.Time `json:"done_at,omitempty"`
}

// TodoParams defines parameters for todo operations
type TodoParams struct {
	Action string `json:"action"`           // add, done, undone, remove, list, clear
	ID     int    `json:"id,omitempty"`     // item ID for done/undone/remove
	Text   string `json:"text,omitempty"`   // text for add
	Filter string `json:"filter,omitempty"` // "all", "pending", "done"
}

// TodoTool manages a simple todo list
type TodoTool struct {
	mu     sync.RWMutex
	items  []TodoItem
	nextID int
}

// NewTodoTool creates a new todo tool
func NewTodoTool() *TodoTool {
	return &TodoTool{
		items:  []TodoItem{},
		nextID: 1,
	}
}

// Info returns tool metadata
func (t *TodoTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "todo",
		Description: `Manage a simple todo list.
Use for tracking simple items to do during a session.

Actions:
- add: Add a new todo item
- done: Mark item as done
- undone: Mark item as not done
- remove: Remove an item
- list: List all items
- clear: Clear all items

Examples:
  - {"action": "add", "text": "Fix the bug in parser"}
  - {"action": "done", "id": 1}
  - {"action": "list"}
  - {"action": "list", "filter": "pending"}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"action": {
					"type": "string",
					"enum": ["add", "done", "undone", "remove", "list", "clear"],
					"description": "Action to perform"
				},
				"id": {
					"type": "integer",
					"description": "Item ID (for done/undone/remove)"
				},
				"text": {
					"type": "string",
					"description": "Todo text (for add)"
				},
				"filter": {
					"type": "string",
					"enum": ["all", "pending", "done"],
					"description": "Filter for list action"
				}
			},
			"required": ["action"]
		}`),
	}
}

// Execute performs todo operations
func (t *TodoTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p TodoParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	switch p.Action {
	case "add":
		return t.addItem(p)
	case "done":
		return t.markDone(p, true)
	case "undone":
		return t.markDone(p, false)
	case "remove":
		return t.removeItem(p)
	case "list":
		return t.listItems(p)
	case "clear":
		return t.clearItems()
	default:
		return nil, fmt.Errorf("unknown action: %s", p.Action)
	}
}

func (t *TodoTool) addItem(p TodoParams) (*tool.Result, error) {
	if p.Text == "" {
		return nil, fmt.Errorf("text is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	item := TodoItem{
		ID:        t.nextID,
		Text:      p.Text,
		Done:      false,
		CreatedAt: time.Now(),
	}
	t.nextID++
	t.items = append(t.items, item)

	return &tool.Result{
		Output: fmt.Sprintf("Added: [%d] %s", item.ID, item.Text),
		Title:  "todo added",
		Metadata: map[string]any{
			"id":   item.ID,
			"text": item.Text,
		},
	}, nil
}

func (t *TodoTool) markDone(p TodoParams, done bool) (*tool.Result, error) {
	if p.ID <= 0 {
		return nil, fmt.Errorf("item ID is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for i := range t.items {
		if t.items[i].ID == p.ID {
			t.items[i].Done = done
			if done {
				t.items[i].DoneAt = time.Now()
			} else {
				t.items[i].DoneAt = time.Time{}
			}

			status := "pending"
			if done {
				status = "done"
			}
			return &tool.Result{
				Output: fmt.Sprintf("Marked as %s: [%d] %s", status, t.items[i].ID, t.items[i].Text),
				Title:  fmt.Sprintf("todo %s", status),
			}, nil
		}
	}

	return nil, fmt.Errorf("item not found: %d", p.ID)
}

func (t *TodoTool) removeItem(p TodoParams) (*tool.Result, error) {
	if p.ID <= 0 {
		return nil, fmt.Errorf("item ID is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for i, item := range t.items {
		if item.ID == p.ID {
			t.items = append(t.items[:i], t.items[i+1:]...)
			return &tool.Result{
				Output: fmt.Sprintf("Removed: [%d] %s", item.ID, item.Text),
				Title:  "todo removed",
			}, nil
		}
	}

	return nil, fmt.Errorf("item not found: %d", p.ID)
}

func (t *TodoTool) listItems(p TodoParams) (*tool.Result, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	filter := p.Filter
	if filter == "" {
		filter = "all"
	}

	var filtered []TodoItem
	for _, item := range t.items {
		switch filter {
		case "pending":
			if !item.Done {
				filtered = append(filtered, item)
			}
		case "done":
			if item.Done {
				filtered = append(filtered, item)
			}
		default:
			filtered = append(filtered, item)
		}
	}

	if len(filtered) == 0 {
		return &tool.Result{
			Output: "No items.",
			Title:  "todo list",
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Todo List (%d items):\n\n", len(filtered)))

	// Pending items first
	pendingCount := 0
	doneCount := 0

	for _, item := range filtered {
		if !item.Done {
			pendingCount++
			sb.WriteString(fmt.Sprintf("[ ] %d. %s\n", item.ID, item.Text))
		}
	}

	if pendingCount > 0 && filter == "all" {
		sb.WriteString("\n--- Done ---\n")
	}

	for _, item := range filtered {
		if item.Done {
			doneCount++
			sb.WriteString(fmt.Sprintf("[✓] %d. %s\n", item.ID, item.Text))
		}
	}

	sb.WriteString(fmt.Sprintf("\nSummary: %d pending, %d done\n", pendingCount, doneCount))

	return &tool.Result{
		Output: sb.String(),
		Title:  "todo list",
		Metadata: map[string]any{
			"total":   len(filtered),
			"pending": pendingCount,
			"done":    doneCount,
			"items":   filtered,
		},
	}, nil
}

func (t *TodoTool) clearItems() (*tool.Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	count := len(t.items)
	t.items = []TodoItem{}
	t.nextID = 1

	return &tool.Result{
		Output: fmt.Sprintf("Cleared %d items.", count),
		Title:  "todo cleared",
		Metadata: map[string]any{
			"cleared": count,
		},
	}, nil
}

// Ensure TodoTool implements Tool interface
var _ tool.Tool = (*TodoTool)(nil)
