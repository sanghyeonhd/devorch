package builtins

import "context"

type RetryStore interface {
	InsertRewardEvent(ctx context.Context, kind string, runID string, data any) error
}

type RetryExhaustedInput struct {
	RunID   string
	Message string
}

func OnRetryExhausted(ctx context.Context, store RetryStore, in RetryExhaustedInput) error {
	if store == nil {
		return nil
	}
	return store.InsertRewardEvent(ctx, "retry_exhausted", in.RunID, in)
}
