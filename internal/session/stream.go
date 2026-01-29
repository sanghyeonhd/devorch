package session

import (
	"time"

	"devorch/internal/bus"
)

func BuildDeltaEvent(workspace, sessionID, taskID, delta string) bus.Event {
	return bus.Event{
		Topic:     bus.TopicSessionStream,
		Workspace: workspace,
		SessionID: sessionID,
		TaskID:    taskID,
		Type:      "delta",
		Payload:   StreamDelta{Text: delta},
		Time:      time.Now(),
	}
}

func BuildDoneEvent(workspace, sessionID, taskID, full string) bus.Event {
	return bus.Event{
		Topic:     bus.TopicSessionStream,
		Workspace: workspace,
		SessionID: sessionID,
		TaskID:    taskID,
		Type:      "done",
		Payload:   StreamDone{Text: full},
		Time:      time.Now(),
	}
}

func BuildErrorEvent(workspace, sessionID, taskID, msg string) bus.Event {
	return bus.Event{
		Topic:     bus.TopicSessionStream,
		Workspace: workspace,
		SessionID: sessionID,
		TaskID:    taskID,
		Type:      "error",
		Payload:   StreamError{Message: msg},
		Time:      time.Now(),
	}
}
