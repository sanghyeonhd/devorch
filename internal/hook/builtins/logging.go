package builtins

import (
	"context"
	"log/slog"
)

// LoggingHook logs all hook events for debugging/auditing
type LoggingHook struct {
	Logger *slog.Logger
	Level  slog.Level
}

// Name returns the hook name
func (h *LoggingHook) Name() string {
	return "logging"
}

// AllEvents returns all event types for logging
func AllEvents() []HookEventType {
	return []HookEventType{
		"pre_tool_use",
		"post_tool_use",
		"prompt_submit",
		"message_create",
		"session_start",
		"session_stop",
		"model_select",
		"on_model_switch",
		"response_stream",
		"on_retry_exhausted",
		"on_error",
	}
}

// Events returns all events (logs everything)
func (h *LoggingHook) Events() []HookEventType {
	return AllEvents()
}

// Execute logs the event
func (h *LoggingHook) Execute(ctx context.Context, event HookEventType, data any) (any, error) {
	if h.Logger == nil {
		return data, nil
	}

	attrs := []slog.Attr{
		slog.String("event", string(event)),
	}

	// Add event-specific attributes based on type assertion
	if m, ok := data.(map[string]any); ok {
		for k, v := range m {
			switch val := v.(type) {
			case string:
				attrs = append(attrs, slog.String(k, val))
			case int:
				attrs = append(attrs, slog.Int(k, val))
			case int64:
				attrs = append(attrs, slog.Int64(k, val))
			case bool:
				attrs = append(attrs, slog.Bool(k, val))
			}
		}
	}

	h.Logger.LogAttrs(ctx, h.Level, "hook event", attrs...)

	return data, nil
}

// NewLoggingHook creates a new logging hook
func NewLoggingHook(logger *slog.Logger) *LoggingHook {
	return &LoggingHook{
		Logger: logger,
		Level:  slog.LevelDebug,
	}
}
