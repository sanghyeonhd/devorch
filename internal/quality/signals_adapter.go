package quality

import (
	"context"

	"devorch/internal/signal"
)

type ToolRunReader interface {
	RecentRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.ToolRun, error)
}
type TestRunReader interface {
	RecentRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.TestRun, error)
}

type SignalAdapter struct {
	Tools ToolRunReader
	Tests TestRunReader
}

func (a *SignalAdapter) RecentToolRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.ToolRun, error) {
	if a == nil || a.Tools == nil {
		return nil, nil
	}
	return a.Tools.RecentRuns(ctx, workspace, sessionID, taskID, limit)
}

func (a *SignalAdapter) RecentTestRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.TestRun, error) {
	if a == nil || a.Tests == nil {
		return nil, nil
	}
	return a.Tests.RecentRuns(ctx, workspace, sessionID, taskID, limit)
}
