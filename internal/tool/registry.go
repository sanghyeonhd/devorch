package tool

import (
	"context"
	"encoding/json"
	"time"

	"devorch/internal/signal"
)

// LegacyTool is the old tool interface for backward compatibility
type LegacyTool interface {
	Name() string
	Execute(ctx context.Context, input json.RawMessage) (any, error)
}

// Registry manages tool registration and execution
type Registry struct {
	tools       map[string]Tool
	legacyTools map[string]LegacyTool

	// WorkDir is the default working directory
	WorkDir string

	Observer *signal.ToolObserver
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools:       map[string]Tool{},
		legacyTools: map[string]LegacyTool{},
	}
}

// Register registers a new-style tool
func (r *Registry) Register(t Tool) {
	info := t.Info()
	r.tools[info.Name] = t
}

// RegisterLegacy registers a legacy tool
func (r *Registry) RegisterLegacy(t LegacyTool) {
	r.legacyTools[t.Name()] = t
}

// RegisterAll registers multiple tools at once
func (r *Registry) RegisterAll(tools []Tool) {
	for _, t := range tools {
		r.Register(t)
	}
}

// Get returns a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tool infos
func (r *Registry) List() []ToolInfo {
	var infos []ToolInfo
	for _, t := range r.tools {
		infos = append(infos, t.Info())
	}
	return infos
}

// Names returns all registered tool names
func (r *Registry) Names() []string {
	var names []string
	for name := range r.tools {
		names = append(names, name)
	}
	for name := range r.legacyTools {
		names = append(names, name)
	}
	return names
}

// Execute runs a tool by name (supports both new and legacy tools)
func (r *Registry) Execute(ctx context.Context, workspace, sessionID, taskID, name string, input json.RawMessage) (any, error) {
	start := time.Now()
	ts := signal.ToolStart{
		Workspace: workspace,
		SessionID: sessionID,
		TaskID:    taskID,
		ToolName:  name,
		Input:     string(input),
		Time:      start,
	}

	var out any
	var err error

	// Try new-style tool first
	if t, ok := r.tools[name]; ok {
		toolCtx := &Context{
			Ctx:         ctx,
			SessionID:   sessionID,
			WorkspaceID: workspace,
			WorkDir:     r.WorkDir,
			Abort:       ctx.Done(),
		}
		result, execErr := t.Execute(toolCtx, input)
		if result != nil {
			out = result
		}
		err = execErr
	} else if lt, ok := r.legacyTools[name]; ok {
		// Fall back to legacy tool
		out, err = lt.Execute(ctx, input)
	} else {
		return nil, ErrToolNotFound
	}

	// Marshal output to bytes for summary
	var outBytes []byte
	if out != nil {
		if b, mErr := json.Marshal(out); mErr == nil {
			outBytes = b
		}
	}

	meta := map[string]any{}
	exitCode := 0

	// Extract metadata from result
	if rm, ok := out.(ResultMeta); ok {
		for k, v := range rm.Meta() {
			meta[k] = v
		}
	}

	// Handle Result type
	if result, ok := out.(*Result); ok && result != nil {
		exitCode = result.ExitCode
		meta = result.Metadata
	}

	// Standard key handling (from JSON decode path)
	if v, ok := meta["exit_code"].(float64); ok {
		exitCode = int(v)
	}
	if v, ok := meta["exit_code"].(int); ok {
		exitCode = v
	}

	if r.Observer != nil {
		r.Observer.OnToolEnd(ctx, signal.ToolEnd{
			Start:    ts,
			Success:  err == nil,
			Error:    err,
			Output:   outBytes,
			Time:     time.Now(),
			ExitCode: exitCode,
			Meta:     meta,
		})
	}

	return out, err
}
