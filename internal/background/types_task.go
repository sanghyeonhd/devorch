package background

import (
	"context"
	"time"
)

// TaskStatus는 백그라운드 작업 상태입니다
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// ManagedTask는 관리되는 백그라운드 작업 구조체입니다
type ManagedTask struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Status      TaskStatus     `json:"status"`
	Payload     map[string]any `json:"payload,omitempty"`
	Result      map[string]any `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	Progress    int            `json:"progress"`
	CreatedAt   time.Time      `json:"created_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// TaskHandler는 작업 처리 핸들러 인터페이스입니다
type TaskHandler interface {
	Handle(ctx context.Context, task *ManagedTask, progress func(int)) error
	Type() string
}

// TaskOption은 작업 옵션입니다
type TaskOption func(*ManagedTask)

// WithTaskPayload는 페이로드를 설정합니다
func WithTaskPayload(payload map[string]any) TaskOption {
	return func(t *ManagedTask) {
		t.Payload = payload
	}
}

// TaskEvent는 작업 이벤트입니다
type TaskEvent struct {
	TaskID    string         `json:"task_id"`
	Type      string         `json:"type"`
	Status    TaskStatus     `json:"status"`
	Progress  int            `json:"progress"`
	Error     string         `json:"error,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

// TaskEventType은 이벤트 타입입니다
type TaskEventType string

const (
	EventTaskCreated   TaskEventType = "task.created"
	EventTaskStarted   TaskEventType = "task.started"
	EventTaskProgress  TaskEventType = "task.progress"
	EventTaskCompleted TaskEventType = "task.completed"
	EventTaskFailed    TaskEventType = "task.failed"
	EventTaskCancelled TaskEventType = "task.cancelled"
)
