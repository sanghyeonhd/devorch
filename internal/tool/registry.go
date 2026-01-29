package tool

import (
	"context"
	"encoding/json"
	"time"

	"devorch/internal/signal"
)

type Tool interface {
	Name() string
	Execute(ctx context.Context, input json.RawMessage) (any, error)
}

type Registry struct {
	tools map[string]Tool

	Observer *signal.ToolObserver
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) Execute(ctx context.Context, workspace, sessionID, taskID, name string, input json.RawMessage) (any, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, ErrToolNotFound
	}

	start := time.Now()
	ts := signal.ToolStart{
		Workspace: workspace,
		SessionID: sessionID,
		TaskID:    taskID,
		ToolName:  name,
		Input:     string(input),
		Time:      start,
	}

	out, err := t.Execute(ctx, input)

	// marshal output to bytes for summary
	var outBytes []byte
	if out != nil {
		if b, mErr := json.Marshal(out); mErr == nil {
			outBytes = b
		}
	}

	meta := map[string]any{}
	exitCode := 0

	// 구조화된 메타
	if rm, ok := out.(ResultMeta); ok {
		for k, v := range rm.Meta() {
			meta[k] = v
		}
	}

	// 표준 키(있으면)
	if v, ok := meta["exit_code"].(float64); ok { // json decode path
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
