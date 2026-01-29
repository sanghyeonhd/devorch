package builtin

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"devorch/internal/tool"
)

// PlanStep represents a single step in a plan
type PlanStep struct {
	ID          int       `json:"id"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // pending, active, done, skipped
	Notes       string    `json:"notes,omitempty"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// Plan represents a multi-step plan
type Plan struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Steps       []PlanStep `json:"steps"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Status      string     `json:"status"` // draft, active, completed, abandoned
}

// PlanParams defines parameters for plan operations
type PlanParams struct {
	Action      string   `json:"action"`                // create, add_step, update_step, next, complete, list, get, abandon
	PlanID      string   `json:"plan_id,omitempty"`     // plan ID
	Title       string   `json:"title,omitempty"`       // for create
	Description string   `json:"description,omitempty"` // for create or add_step
	Steps       []string `json:"steps,omitempty"`       // initial steps for create
	StepID      int      `json:"step_id,omitempty"`     // step ID for update_step
	StepStatus  string   `json:"step_status,omitempty"` // for update_step
	Notes       string   `json:"notes,omitempty"`       // for update_step
}

// PlanTool manages execution plans
type PlanTool struct {
	mu     sync.RWMutex
	plans  map[string]*Plan
	nextID int
}

// NewPlanTool creates a new plan tool
func NewPlanTool() *PlanTool {
	return &PlanTool{
		plans:  make(map[string]*Plan),
		nextID: 1,
	}
}

// Info returns tool metadata
func (t *PlanTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "plan",
		Description: `Create and manage execution plans for complex tasks.
Use this to organize multi-step operations with clear checkpoints.

Actions:
- create: Create a new plan with steps
- add_step: Add a step to existing plan
- update_step: Update step status/notes
- next: Move to next step
- complete: Mark plan as completed
- list: List all plans
- get: Get plan details
- abandon: Abandon a plan

Step statuses: pending, active, done, skipped

Examples:
  - {"action": "create", "title": "Refactor auth module", "steps": ["Backup current code", "Write tests", "Implement changes", "Run tests", "Deploy"]}
  - {"action": "update_step", "plan_id": "plan-1", "step_id": 1, "step_status": "done"}
  - {"action": "next", "plan_id": "plan-1"}
  - {"action": "get", "plan_id": "plan-1"}`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"action": {
					"type": "string",
					"enum": ["create", "add_step", "update_step", "next", "complete", "list", "get", "abandon"],
					"description": "Action to perform"
				},
				"plan_id": {
					"type": "string",
					"description": "Plan ID"
				},
				"title": {
					"type": "string",
					"description": "Plan title (for create)"
				},
				"description": {
					"type": "string",
					"description": "Plan or step description"
				},
				"steps": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Initial steps (for create)"
				},
				"step_id": {
					"type": "integer",
					"description": "Step ID (for update_step)"
				},
				"step_status": {
					"type": "string",
					"enum": ["pending", "active", "done", "skipped"],
					"description": "Step status"
				},
				"notes": {
					"type": "string",
					"description": "Notes for step"
				}
			},
			"required": ["action"]
		}`),
	}
}

// Execute performs plan operations
func (t *PlanTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p PlanParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	switch p.Action {
	case "create":
		return t.createPlan(p)
	case "add_step":
		return t.addStep(p)
	case "update_step":
		return t.updateStep(p)
	case "next":
		return t.nextStep(p)
	case "complete":
		return t.completePlan(p)
	case "list":
		return t.listPlans()
	case "get":
		return t.getPlan(p)
	case "abandon":
		return t.abandonPlan(p)
	default:
		return nil, fmt.Errorf("unknown action: %s", p.Action)
	}
}

func (t *PlanTool) createPlan(p PlanParams) (*tool.Result, error) {
	if p.Title == "" {
		return nil, fmt.Errorf("title is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	id := fmt.Sprintf("plan-%d", t.nextID)
	t.nextID++

	steps := make([]PlanStep, len(p.Steps))
	for i, desc := range p.Steps {
		steps[i] = PlanStep{
			ID:          i + 1,
			Description: desc,
			Status:      "pending",
		}
	}

	// Mark first step as active
	if len(steps) > 0 {
		steps[0].Status = "active"
		steps[0].StartedAt = time.Now()
	}

	plan := &Plan{
		ID:          id,
		Title:       p.Title,
		Description: p.Description,
		Steps:       steps,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Status:      "active",
	}

	t.plans[id] = plan

	return &tool.Result{
		Output: t.formatPlan(plan),
		Title:  fmt.Sprintf("plan created: %s", id),
		Metadata: map[string]any{
			"plan_id": id,
			"plan":    plan,
		},
	}, nil
}

func (t *PlanTool) addStep(p PlanParams) (*tool.Result, error) {
	if p.PlanID == "" {
		return nil, fmt.Errorf("plan_id is required")
	}
	if p.Description == "" {
		return nil, fmt.Errorf("description is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	plan, ok := t.plans[p.PlanID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", p.PlanID)
	}

	stepID := len(plan.Steps) + 1
	step := PlanStep{
		ID:          stepID,
		Description: p.Description,
		Status:      "pending",
	}

	plan.Steps = append(plan.Steps, step)
	plan.UpdatedAt = time.Now()

	return &tool.Result{
		Output: fmt.Sprintf("Added step %d: %s", stepID, p.Description),
		Title:  "step added",
		Metadata: map[string]any{
			"step_id": stepID,
		},
	}, nil
}

func (t *PlanTool) updateStep(p PlanParams) (*tool.Result, error) {
	if p.PlanID == "" {
		return nil, fmt.Errorf("plan_id is required")
	}
	if p.StepID <= 0 {
		return nil, fmt.Errorf("step_id is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	plan, ok := t.plans[p.PlanID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", p.PlanID)
	}

	stepIdx := p.StepID - 1
	if stepIdx < 0 || stepIdx >= len(plan.Steps) {
		return nil, fmt.Errorf("step not found: %d", p.StepID)
	}

	if p.StepStatus != "" {
		plan.Steps[stepIdx].Status = p.StepStatus
		if p.StepStatus == "active" {
			plan.Steps[stepIdx].StartedAt = time.Now()
		} else if p.StepStatus == "done" {
			plan.Steps[stepIdx].CompletedAt = time.Now()
		}
	}
	if p.Notes != "" {
		plan.Steps[stepIdx].Notes = p.Notes
	}

	plan.UpdatedAt = time.Now()

	return &tool.Result{
		Output: fmt.Sprintf("Updated step %d: %s", p.StepID, plan.Steps[stepIdx].Status),
		Title:  "step updated",
	}, nil
}

func (t *PlanTool) nextStep(p PlanParams) (*tool.Result, error) {
	if p.PlanID == "" {
		return nil, fmt.Errorf("plan_id is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	plan, ok := t.plans[p.PlanID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", p.PlanID)
	}

	// Find current active step and mark as done
	nextIdx := -1
	for i := range plan.Steps {
		if plan.Steps[i].Status == "active" {
			plan.Steps[i].Status = "done"
			plan.Steps[i].CompletedAt = time.Now()
			nextIdx = i + 1
			break
		}
	}

	// If no active step, find first pending
	if nextIdx == -1 {
		for i := range plan.Steps {
			if plan.Steps[i].Status == "pending" {
				nextIdx = i
				break
			}
		}
	}

	// Activate next step
	if nextIdx >= 0 && nextIdx < len(plan.Steps) {
		plan.Steps[nextIdx].Status = "active"
		plan.Steps[nextIdx].StartedAt = time.Now()
		plan.UpdatedAt = time.Now()

		return &tool.Result{
			Output: fmt.Sprintf("Now on step %d: %s", plan.Steps[nextIdx].ID, plan.Steps[nextIdx].Description),
			Title:  "next step",
			Metadata: map[string]any{
				"step_id":     plan.Steps[nextIdx].ID,
				"description": plan.Steps[nextIdx].Description,
			},
		}, nil
	}

	// All steps done
	plan.Status = "completed"
	plan.UpdatedAt = time.Now()

	return &tool.Result{
		Output: "Plan completed! All steps done.",
		Title:  "plan completed",
	}, nil
}

func (t *PlanTool) completePlan(p PlanParams) (*tool.Result, error) {
	if p.PlanID == "" {
		return nil, fmt.Errorf("plan_id is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	plan, ok := t.plans[p.PlanID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", p.PlanID)
	}

	// Mark all remaining steps as done
	for i := range plan.Steps {
		if plan.Steps[i].Status == "pending" || plan.Steps[i].Status == "active" {
			plan.Steps[i].Status = "done"
			plan.Steps[i].CompletedAt = time.Now()
		}
	}

	plan.Status = "completed"
	plan.UpdatedAt = time.Now()

	return &tool.Result{
		Output: fmt.Sprintf("Plan '%s' marked as completed.", plan.Title),
		Title:  "plan completed",
	}, nil
}

func (t *PlanTool) listPlans() (*tool.Result, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.plans) == 0 {
		return &tool.Result{
			Output: "No plans.",
			Title:  "plan list",
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Plans (%d):\n\n", len(t.plans)))

	for _, plan := range t.plans {
		doneCount := 0
		for _, step := range plan.Steps {
			if step.Status == "done" {
				doneCount++
			}
		}

		statusIcon := "📋"
		switch plan.Status {
		case "active":
			statusIcon = "▶️"
		case "completed":
			statusIcon = "✅"
		case "abandoned":
			statusIcon = "❌"
		}

		sb.WriteString(fmt.Sprintf("%s [%s] %s\n", statusIcon, plan.ID, plan.Title))
		sb.WriteString(fmt.Sprintf("   Progress: %d/%d steps\n", doneCount, len(plan.Steps)))
		sb.WriteString("\n")
	}

	return &tool.Result{
		Output: sb.String(),
		Title:  "plan list",
	}, nil
}

func (t *PlanTool) getPlan(p PlanParams) (*tool.Result, error) {
	if p.PlanID == "" {
		return nil, fmt.Errorf("plan_id is required")
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	plan, ok := t.plans[p.PlanID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", p.PlanID)
	}

	return &tool.Result{
		Output: t.formatPlan(plan),
		Title:  fmt.Sprintf("plan: %s", plan.ID),
		Metadata: map[string]any{
			"plan": plan,
		},
	}, nil
}

func (t *PlanTool) abandonPlan(p PlanParams) (*tool.Result, error) {
	if p.PlanID == "" {
		return nil, fmt.Errorf("plan_id is required")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	plan, ok := t.plans[p.PlanID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", p.PlanID)
	}

	plan.Status = "abandoned"
	plan.UpdatedAt = time.Now()

	return &tool.Result{
		Output: fmt.Sprintf("Plan '%s' abandoned.", plan.Title),
		Title:  "plan abandoned",
	}, nil
}

func (t *PlanTool) formatPlan(plan *Plan) string {
	var sb strings.Builder

	statusIcon := "📋"
	switch plan.Status {
	case "active":
		statusIcon = "▶️"
	case "completed":
		statusIcon = "✅"
	case "abandoned":
		statusIcon = "❌"
	}

	sb.WriteString(fmt.Sprintf("%s Plan: %s\n", statusIcon, plan.Title))
	sb.WriteString(fmt.Sprintf("ID: %s\n", plan.ID))
	sb.WriteString(fmt.Sprintf("Status: %s\n", plan.Status))
	if plan.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", plan.Description))
	}
	sb.WriteString("\nSteps:\n")

	for _, step := range plan.Steps {
		icon := "○"
		switch step.Status {
		case "active":
			icon = "▶"
		case "done":
			icon = "✓"
		case "skipped":
			icon = "⊘"
		}

		sb.WriteString(fmt.Sprintf("  %s %d. %s\n", icon, step.ID, step.Description))
		if step.Notes != "" {
			sb.WriteString(fmt.Sprintf("      Notes: %s\n", step.Notes))
		}
	}

	doneCount := 0
	for _, step := range plan.Steps {
		if step.Status == "done" {
			doneCount++
		}
	}
	sb.WriteString(fmt.Sprintf("\nProgress: %d/%d steps complete\n", doneCount, len(plan.Steps)))

	return sb.String()
}

// Ensure PlanTool implements Tool interface
var _ tool.Tool = (*PlanTool)(nil)
