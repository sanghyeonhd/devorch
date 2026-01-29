package delegate

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	ErrNoHandler    = errors.New("no handler found for task type")
	ErrTimeout      = errors.New("delegate request timeout")
	ErrCancelled    = errors.New("delegate request cancelled")
	ErrMaxRetries   = errors.New("max retries exceeded")
	ErrInvalidAgent = errors.New("invalid agent id")
)

// Delegator는 작업 위임을 관리합니다
type Delegator struct {
	router    *DelegateRouter
	pending   map[string]*delegateJob
	callbacks map[string]DelegateCallback
	mu        sync.RWMutex
	logger    *slog.Logger
}

type delegateJob struct {
	request   DelegateRequest
	response  chan DelegateResponse
	cancel    context.CancelFunc
	startedAt time.Time
}

// NewDelegator는 새 Delegator를 생성합니다
func NewDelegator(router *DelegateRouter, logger *slog.Logger) *Delegator {
	return &Delegator{
		router:    router,
		pending:   make(map[string]*delegateJob),
		callbacks: make(map[string]DelegateCallback),
		logger:    logger,
	}
}

// Delegate는 작업을 위임합니다 (동기)
func (d *Delegator) Delegate(ctx context.Context, agentID string, taskType string, opts ...DelegateOption) (DelegateResponse, error) {
	req := d.buildRequest(agentID, taskType, opts...)

	handler, ok := d.router.Route(req)
	if !ok {
		return DelegateResponse{
			RequestID:   req.ID,
			AgentID:     agentID,
			Status:      StatusFailed,
			Error:       ErrNoHandler.Error(),
			CompletedAt: time.Now(),
		}, ErrNoHandler
	}

	return d.execute(ctx, handler, req)
}

// DelegateAsync는 작업을 비동기로 위임합니다
func (d *Delegator) DelegateAsync(ctx context.Context, agentID string, taskType string, callback DelegateCallback, opts ...DelegateOption) (string, error) {
	req := d.buildRequest(agentID, taskType, opts...)

	handler, ok := d.router.Route(req)
	if !ok {
		return "", ErrNoHandler
	}

	// 콜백 등록
	d.mu.Lock()
	d.callbacks[req.ID] = callback
	d.mu.Unlock()

	// 비동기 실행
	go func() {
		resp, _ := d.execute(ctx, handler, req)

		// 콜백 호출
		d.mu.RLock()
		cb, ok := d.callbacks[req.ID]
		d.mu.RUnlock()

		if ok {
			cb(resp)
		}

		// 정리
		d.mu.Lock()
		delete(d.callbacks, req.ID)
		d.mu.Unlock()
	}()

	return req.ID, nil
}

// Cancel는 진행 중인 위임을 취소합니다
func (d *Delegator) Cancel(requestID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	job, ok := d.pending[requestID]
	if !ok {
		return errors.New("request not found")
	}

	job.cancel()
	delete(d.pending, requestID)
	return nil
}

// GetStatus는 위임 상태를 조회합니다
func (d *Delegator) GetStatus(requestID string) (DelegateStatus, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if _, ok := d.pending[requestID]; ok {
		return StatusRunning, true
	}
	return "", false
}

func (d *Delegator) buildRequest(agentID string, taskType string, opts ...DelegateOption) DelegateRequest {
	req := DelegateRequest{
		ID:        ulid.Make().String(),
		AgentID:   agentID,
		TaskType:  taskType,
		Priority:  0,
		Timeout:   30 * time.Second,
		CreatedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(&req)
	}

	return req
}

func (d *Delegator) execute(ctx context.Context, handler DelegateHandler, req DelegateRequest) (DelegateResponse, error) {
	startedAt := time.Now()

	// 타임아웃 컨텍스트
	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	// 작업 등록
	job := &delegateJob{
		request:   req,
		response:  make(chan DelegateResponse, 1),
		cancel:    cancel,
		startedAt: startedAt,
	}

	d.mu.Lock()
	d.pending[req.ID] = job
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.pending, req.ID)
		d.mu.Unlock()
	}()

	d.logger.Debug("executing delegate",
		"request_id", req.ID,
		"agent_id", req.AgentID,
		"task_type", req.TaskType,
	)

	// 핸들러 실행
	resp, err := handler.Handle(execCtx, req)
	completedAt := time.Now()

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return DelegateResponse{
				RequestID:   req.ID,
				AgentID:     req.AgentID,
				Status:      StatusTimeout,
				Error:       ErrTimeout.Error(),
				StartedAt:   startedAt,
				CompletedAt: completedAt,
				Duration:    completedAt.Sub(startedAt),
			}, ErrTimeout
		}

		if errors.Is(err, context.Canceled) {
			return DelegateResponse{
				RequestID:   req.ID,
				AgentID:     req.AgentID,
				Status:      StatusCancelled,
				Error:       ErrCancelled.Error(),
				StartedAt:   startedAt,
				CompletedAt: completedAt,
				Duration:    completedAt.Sub(startedAt),
			}, ErrCancelled
		}

		return DelegateResponse{
			RequestID:   req.ID,
			AgentID:     req.AgentID,
			Status:      StatusFailed,
			Error:       err.Error(),
			StartedAt:   startedAt,
			CompletedAt: completedAt,
			Duration:    completedAt.Sub(startedAt),
		}, err
	}

	resp.RequestID = req.ID
	resp.AgentID = req.AgentID
	resp.StartedAt = startedAt
	resp.CompletedAt = completedAt
	resp.Duration = completedAt.Sub(startedAt)

	if resp.Status == "" {
		resp.Status = StatusCompleted
	}

	return resp, nil
}
