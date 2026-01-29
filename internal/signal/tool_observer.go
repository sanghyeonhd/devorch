package signal

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"devorch/internal/id"
)

type ToolRunWriter interface {
	InsertToolRun(ctx context.Context, r ToolRun) (string, error)
}

type ToolObserver struct {
	Store ToolRunWriter
}

type ToolStart struct {
	Workspace string
	SessionID string
	TaskID    string
	ToolName  string
	Input     string // raw json or stringified (MVP)
	Time      time.Time
}

type ToolEnd struct {
	Start    ToolStart
	Success  bool
	Error    error
	Output   []byte // raw output bytes
	Time     time.Time
	ExitCode int
	Meta     map[string]any
}

func NewToolObserver(store ToolRunWriter) *ToolObserver {
	return &ToolObserver{Store: store}
}

func (o *ToolObserver) OnToolEnd(ctx context.Context, end ToolEnd) {
	if o == nil || o.Store == nil {
		return
	}

	inSum := summarize(end.Start.Input, 800)
	outSum := summarize(string(end.Output), 1200)

	metaJSON := ""
	if end.Meta != nil {
		if b, err := json.Marshal(end.Meta); err == nil {
			metaJSON = string(b)
		}
	}

	tr := ToolRun{
		ID:            id.NewULID(),
		Workspace:     end.Start.Workspace,
		SessionID:     end.Start.SessionID,
		TaskID:        end.Start.TaskID,
		ToolName:      end.Start.ToolName,
		StartedAt:     end.Start.Time,
		EndedAt:       end.Time,
		DurationMs:    end.Time.Sub(end.Start.Time).Milliseconds(),
		Success:       end.Success,
		Error:         errString(end.Error),
		InputSummary:  inSum,
		OutputSummary: outSum,
		OutputBytes:   int64(len(end.Output)),
		Tags:          "",
		ExitCode:      end.ExitCode,
		MetaJSON:      metaJSON,
	}

	_, _ = o.Store.InsertToolRun(ctx, tr)
}

func summarize(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func errString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}
