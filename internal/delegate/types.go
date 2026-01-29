package delegate

import (
	"context"
	"time"
)

// DelegateRequest는 위임 요청 구조체입니다
type DelegateRequest struct {
	ID          string         `json:"id"`
	AgentID     string         `json:"agent_id"`
	TaskType    string         `json:"task_type"`
	Payload     map[string]any `json:"payload"`
	Priority    int            `json:"priority"`
	Timeout     time.Duration  `json:"timeout,omitempty"`
	RetryCount  int            `json:"retry_count,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	ScheduledAt time.Time      `json:"scheduled_at,omitempty"`
}

// DelegateResponse는 위임 응답 구조체입니다
type DelegateResponse struct {
	RequestID   string         `json:"request_id"`
	AgentID     string         `json:"agent_id"`
	Status      DelegateStatus `json:"status"`
	Result      map[string]any `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt time.Time      `json:"completed_at"`
	Duration    time.Duration  `json:"duration"`
}

// DelegateStatus는 위임 상태입니다
type DelegateStatus string

const (
	StatusPending   DelegateStatus = "pending"
	StatusRunning   DelegateStatus = "running"
	StatusCompleted DelegateStatus = "completed"
	StatusFailed    DelegateStatus = "failed"
	StatusCancelled DelegateStatus = "cancelled"
	StatusTimeout   DelegateStatus = "timeout"
)

// DelegateHandler는 위임 처리 핸들러 인터페이스입니다
type DelegateHandler interface {
	Handle(ctx context.Context, req DelegateRequest) (DelegateResponse, error)
	CanHandle(taskType string) bool
}

// DelegateCallback은 위임 완료 콜백입니다
type DelegateCallback func(resp DelegateResponse)

// DelegateOption은 위임 옵션입니다
type DelegateOption func(*DelegateRequest)

// WithPriority는 우선순위를 설정합니다
func WithPriority(priority int) DelegateOption {
	return func(req *DelegateRequest) {
		req.Priority = priority
	}
}

// WithTimeout는 타임아웃을 설정합니다
func WithTimeout(timeout time.Duration) DelegateOption {
	return func(req *DelegateRequest) {
		req.Timeout = timeout
	}
}

// WithRetry는 재시도 횟수를 설정합니다
func WithRetry(count int) DelegateOption {
	return func(req *DelegateRequest) {
		req.RetryCount = count
	}
}

// WithSchedule는 예약 실행 시간을 설정합니다
func WithSchedule(at time.Time) DelegateOption {
	return func(req *DelegateRequest) {
		req.ScheduledAt = at
	}
}

// WithPayload는 페이로드를 설정합니다
func WithPayload(payload map[string]any) DelegateOption {
	return func(req *DelegateRequest) {
		req.Payload = payload
	}
}
