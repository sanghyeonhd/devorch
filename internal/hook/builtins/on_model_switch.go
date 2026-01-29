package builtins

import "context"

type SwitchStore interface {
	InsertRewardEvent(ctx context.Context, kind string, runID string, data any) error
}

type ModelSwitchInput struct {
	RunID       string
	FromModel   string
	ToModel     string
	Reason      string
	ContextHash string
}

func OnModelSwitch(ctx context.Context, store SwitchStore, in ModelSwitchInput) error {
	if store == nil {
		return nil
	}
	return store.InsertRewardEvent(ctx, "model_switch", in.RunID, in)
}
