package hook

// EventType defines hook event types
type EventType string

const (
	// Tool lifecycle events
	EventPreToolUse  EventType = "pre_tool_use"  // Before tool execution
	EventPostToolUse EventType = "post_tool_use" // After tool execution

	// Prompt/message events
	EventPromptSubmit  EventType = "prompt_submit"  // User submits prompt
	EventMessageCreate EventType = "message_create" // Message created

	// Session events
	EventSessionStart EventType = "session_start" // Session starts
	EventSessionStop  EventType = "session_stop"  // Session ends

	// Model events
	EventModelSelect   EventType = "model_select"    // Model selected/resolved
	EventOnModelSwitch EventType = "on_model_switch" // Model switched during retry

	// Response events
	EventResponseStream EventType = "response_stream" // Streaming response chunk

	// Error events
	EventOnRetryExhausted EventType = "on_retry_exhausted" // All retries failed
	EventOnError          EventType = "on_error"           // General error
)

// AllEvents returns all available event types
func AllEvents() []EventType {
	return []EventType{
		EventPreToolUse,
		EventPostToolUse,
		EventPromptSubmit,
		EventMessageCreate,
		EventSessionStart,
		EventSessionStop,
		EventModelSelect,
		EventOnModelSwitch,
		EventResponseStream,
		EventOnRetryExhausted,
		EventOnError,
	}
}

// ToolEvents returns tool-related events
func ToolEvents() []EventType {
	return []EventType{
		EventPreToolUse,
		EventPostToolUse,
	}
}

// SessionEvents returns session-related events
func SessionEvents() []EventType {
	return []EventType{
		EventSessionStart,
		EventSessionStop,
	}
}

// MessageEvents returns message-related events
func MessageEvents() []EventType {
	return []EventType{
		EventPromptSubmit,
		EventMessageCreate,
		EventResponseStream,
	}
}
