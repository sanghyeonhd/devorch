package hook

type EventType string

const (
	EventOnModelSwitch    EventType = "on_model_switch"
	EventOnRetryExhausted EventType = "on_retry_exhausted"
)
