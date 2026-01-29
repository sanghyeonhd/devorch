package hook

import (
	"context"

	"devorch/internal/hook/builtins"
)

// RewardStore is the adapter interface to avoid coupling with okAON
type RewardStore interface {
	builtins.SwitchStore
	builtins.RetryStore
}

// Chain manages hook dispatch with built-in handlers
type Chain struct {
	Store    RewardStore
	Registry *Registry
}

// NewChain creates a new hook chain with optional registry
func NewChain(store RewardStore) *Chain {
	return &Chain{
		Store:    store,
		Registry: NewRegistry(),
	}
}

// DispatchInput defines input for legacy dispatch method
type DispatchInput struct {
	Type EventType

	// Common fields
	RunID string

	// Model switch
	FromModel   string
	ToModel     string
	Reason      string
	ContextHash string

	// Retry exhausted
	Message string
}

// Dispatch sends an event to built-in handlers and registered hooks
func (c *Chain) Dispatch(ctx context.Context, in DispatchInput) error {
	// Handle built-in events
	switch in.Type {
	case EventOnModelSwitch:
		if err := builtins.OnModelSwitch(ctx, c.Store, builtins.ModelSwitchInput{
			RunID:       in.RunID,
			FromModel:   in.FromModel,
			ToModel:     in.ToModel,
			Reason:      in.Reason,
			ContextHash: in.ContextHash,
		}); err != nil {
			return err
		}
	case EventOnRetryExhausted:
		if err := builtins.OnRetryExhausted(ctx, c.Store, builtins.RetryExhaustedInput{
			RunID:   in.RunID,
			Message: in.Message,
		}); err != nil {
			return err
		}
	}

	// Dispatch to registered hooks
	if c.Registry != nil {
		_, err := c.Registry.Dispatch(ctx, in.Type, in)
		return err
	}

	return nil
}

// DispatchEvent sends a typed event to registered hooks
func (c *Chain) DispatchEvent(ctx context.Context, event EventType, data any) (any, error) {
	if c.Registry == nil {
		return data, nil
	}
	return c.Registry.Dispatch(ctx, event, data)
}

// DispatchPreToolUse dispatches pre tool use event
func (c *Chain) DispatchPreToolUse(ctx context.Context, data PreToolUseData) (*PreToolUseData, error) {
	result, err := c.DispatchEvent(ctx, EventPreToolUse, &data)
	if err != nil {
		return nil, err
	}
	if r, ok := result.(*PreToolUseData); ok {
		return r, nil
	}
	return &data, nil
}

// DispatchPostToolUse dispatches post tool use event
func (c *Chain) DispatchPostToolUse(ctx context.Context, data PostToolUseData) (*PostToolUseData, error) {
	result, err := c.DispatchEvent(ctx, EventPostToolUse, &data)
	if err != nil {
		return nil, err
	}
	if r, ok := result.(*PostToolUseData); ok {
		return r, nil
	}
	return &data, nil
}

// DispatchPromptSubmit dispatches prompt submit event
func (c *Chain) DispatchPromptSubmit(ctx context.Context, data PromptSubmitData) (*PromptSubmitData, error) {
	result, err := c.DispatchEvent(ctx, EventPromptSubmit, &data)
	if err != nil {
		return nil, err
	}
	if r, ok := result.(*PromptSubmitData); ok {
		return r, nil
	}
	return &data, nil
}

// DispatchSessionStart dispatches session start event
func (c *Chain) DispatchSessionStart(ctx context.Context, data SessionStartData) error {
	_, err := c.DispatchEvent(ctx, EventSessionStart, data)
	return err
}

// DispatchSessionStop dispatches session stop event
func (c *Chain) DispatchSessionStop(ctx context.Context, data SessionStopData) error {
	_, err := c.DispatchEvent(ctx, EventSessionStop, data)
	return err
}

// RegisterHook registers a hook to the chain
func (c *Chain) RegisterHook(h Hook) {
	if c.Registry == nil {
		c.Registry = NewRegistry()
	}
	c.Registry.Register(h)
}
