package quality

import (
	"context"

	"devorch/internal/signal"
)

type SignalReader interface {
	RecentToolRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.ToolRun, error)
	RecentTestRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.TestRun, error)
}
