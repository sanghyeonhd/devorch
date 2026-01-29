package background

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// TaskObserver는 작업 상태 관찰자입니다
type TaskObserver interface {
	OnTaskEvent(event TaskEvent)
}

// TaskManager는 백그라운드 작업 관리자입니다
type TaskManager struct {
	store     ManagedTaskStore
	handlers  map[string]TaskHandler
	observers []TaskObserver
	running   map[string]context.CancelFunc
	mu        sync.RWMutex
	logger    *slog.Logger
}

// NewTaskManager는 새 TaskManager를 생성합니다
func NewTaskManager(store ManagedTaskStore, logger *slog.Logger) *TaskManager {
	return &TaskManager{
		store:     store,
		handlers:  make(map[string]TaskHandler),
		observers: make([]TaskObserver, 0),
		running:   make(map[string]context.CancelFunc),
		logger:    logger,
	}
}

// RegisterHandler는 핸들러를 등록합니다
func (m *TaskManager) RegisterHandler(handler TaskHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[handler.Type()] = handler
	m.logger.Debug("task handler registered", "type", handler.Type())
}

// AddObserver는 관찰자를 추가합니다
func (m *TaskManager) AddObserver(observer TaskObserver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.observers = append(m.observers, observer)
}

// SubmitTask는 작업을 제출합니다
func (m *TaskManager) SubmitTask(ctx context.Context, taskType string, opts ...TaskOption) (string, error) {
	task := &ManagedTask{
		ID:        ulid.Make().String(),
		Type:      taskType,
		Status:    TaskStatusPending,
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(task)
	}

	if err := m.store.Create(ctx, task); err != nil {
		return "", err
	}

	m.emit(TaskEvent{
		TaskID:    task.ID,
		Type:      string(EventTaskCreated),
		Status:    task.Status,
		Timestamp: time.Now(),
	})

	// 비동기 실행
	go m.executeTask(task)

	return task.ID, nil
}

// CancelTask는 작업을 취소합니다
func (m *TaskManager) CancelTask(ctx context.Context, taskID string) error {
	m.mu.Lock()
	cancel, ok := m.running[taskID]
	m.mu.Unlock()

	if !ok {
		return ErrTaskNotFound
	}

	cancel()

	task, err := m.store.Get(ctx, taskID)
	if err != nil {
		return err
	}

	task.Status = TaskStatusCancelled
	now := time.Now()
	task.CompletedAt = &now
	if err := m.store.Update(ctx, task); err != nil {
		return err
	}

	m.emit(TaskEvent{
		TaskID:    taskID,
		Type:      string(EventTaskCancelled),
		Status:    TaskStatusCancelled,
		Timestamp: time.Now(),
	})

	return nil
}

// GetTask는 작업을 조회합니다
func (m *TaskManager) GetTask(ctx context.Context, taskID string) (*ManagedTask, error) {
	return m.store.Get(ctx, taskID)
}

// ListTasks는 작업 목록을 반환합니다
func (m *TaskManager) ListTasks(ctx context.Context, status *TaskStatus, limit int) ([]*ManagedTask, error) {
	return m.store.List(ctx, status, limit)
}

func (m *TaskManager) executeTask(task *ManagedTask) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.mu.Lock()
	m.running[task.ID] = cancel
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.running, task.ID)
		m.mu.Unlock()
	}()

	m.mu.RLock()
	handler, ok := m.handlers[task.Type]
	m.mu.RUnlock()

	if !ok {
		m.failTask(task, "no handler for task type: "+task.Type)
		return
	}

	// 시작
	task.Status = TaskStatusRunning
	now := time.Now()
	task.StartedAt = &now
	if err := m.store.Update(ctx, task); err != nil {
		m.logger.Error("failed to update task", "error", err)
		return
	}

	m.emit(TaskEvent{
		TaskID:    task.ID,
		Type:      string(EventTaskStarted),
		Status:    TaskStatusRunning,
		Timestamp: time.Now(),
	})

	// 진행률 콜백
	progress := func(p int) {
		task.Progress = p
		if err := m.store.Update(ctx, task); err != nil {
			m.logger.Error("failed to update progress", "error", err)
		}
		m.emit(TaskEvent{
			TaskID:    task.ID,
			Type:      string(EventTaskProgress),
			Status:    TaskStatusRunning,
			Progress:  p,
			Timestamp: time.Now(),
		})
	}

	// 실행
	if err := handler.Handle(ctx, task, progress); err != nil {
		m.failTask(task, err.Error())
		return
	}

	// 완료
	task.Status = TaskStatusCompleted
	task.Progress = 100
	completedAt := time.Now()
	task.CompletedAt = &completedAt
	if err := m.store.Update(ctx, task); err != nil {
		m.logger.Error("failed to update completed task", "error", err)
	}

	m.emit(TaskEvent{
		TaskID:    task.ID,
		Type:      string(EventTaskCompleted),
		Status:    TaskStatusCompleted,
		Progress:  100,
		Timestamp: time.Now(),
	})
}

func (m *TaskManager) failTask(task *ManagedTask, errMsg string) {
	task.Status = TaskStatusFailed
	task.Error = errMsg
	now := time.Now()
	task.CompletedAt = &now

	if err := m.store.Update(context.Background(), task); err != nil {
		m.logger.Error("failed to update failed task", "error", err)
	}

	m.emit(TaskEvent{
		TaskID:    task.ID,
		Type:      string(EventTaskFailed),
		Status:    TaskStatusFailed,
		Error:     errMsg,
		Timestamp: time.Now(),
	})
}

func (m *TaskManager) emit(event TaskEvent) {
	m.mu.RLock()
	observers := m.observers
	m.mu.RUnlock()

	for _, obs := range observers {
		go obs.OnTaskEvent(event)
	}
}
