package hook

import (
	"context"
	"sync"
)

// Registry manages hook registration and dispatch
type Registry struct {
	mu    sync.RWMutex
	hooks map[EventType][]Hook
}

// NewRegistry creates a new hook registry
func NewRegistry() *Registry {
	return &Registry{
		hooks: make(map[EventType][]Hook),
	}
}

// Register adds a hook to the registry
func (r *Registry) Register(h Hook) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, event := range h.Events() {
		r.hooks[event] = append(r.hooks[event], h)
	}
}

// RegisterFunc registers a function as a hook for specific events
func (r *Registry) RegisterFunc(name string, events []EventType, fn HookFunc) {
	r.Register(&funcHook{
		name:   name,
		events: events,
		fn:     fn,
	})
}

// Unregister removes a hook from the registry
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for event, hooks := range r.hooks {
		var filtered []Hook
		for _, h := range hooks {
			if h.Name() != name {
				filtered = append(filtered, h)
			}
		}
		r.hooks[event] = filtered
	}
}

// Dispatch sends an event to all registered hooks
func (r *Registry) Dispatch(ctx context.Context, event EventType, data any) (any, error) {
	r.mu.RLock()
	hooks := r.hooks[event]
	r.mu.RUnlock()

	// No hooks registered for this event
	if len(hooks) == 0 {
		return data, nil
	}

	// Execute hooks in order, passing modified data through
	result := data
	var err error

	for _, h := range hooks {
		result, err = h.Execute(ctx, event, result)
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

// DispatchAsync sends an event to all hooks asynchronously (fire and forget)
func (r *Registry) DispatchAsync(event EventType, data any) {
	r.mu.RLock()
	hooks := r.hooks[event]
	r.mu.RUnlock()

	if len(hooks) == 0 {
		return
	}

	go func() {
		ctx := context.Background()
		for _, h := range hooks {
			h.Execute(ctx, event, data)
		}
	}()
}

// HasHooks returns true if any hooks are registered for the event
func (r *Registry) HasHooks(event EventType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hooks[event]) > 0
}

// Count returns the number of hooks registered for an event
func (r *Registry) Count(event EventType) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.hooks[event])
}

// ListHooks returns all hook names for an event
func (r *Registry) ListHooks(event EventType) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var names []string
	for _, h := range r.hooks[event] {
		names = append(names, h.Name())
	}
	return names
}

// funcHook adapts a function to the Hook interface
type funcHook struct {
	name   string
	events []EventType
	fn     HookFunc
}

func (h *funcHook) Name() string {
	return h.name
}

func (h *funcHook) Events() []EventType {
	return h.events
}

func (h *funcHook) Execute(ctx context.Context, event EventType, data any) (any, error) {
	return h.fn(ctx, event, data)
}
