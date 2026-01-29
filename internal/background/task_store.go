package background

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrTaskNotFound = errors.New("task not found")
	ErrTaskExists   = errors.New("task already exists")
)

// ManagedTaskStore는 작업 저장소 인터페이스입니다
type ManagedTaskStore interface {
	Create(ctx context.Context, task *ManagedTask) error
	Get(ctx context.Context, id string) (*ManagedTask, error)
	Update(ctx context.Context, task *ManagedTask) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, status *TaskStatus, limit int) ([]*ManagedTask, error)
	ListByType(ctx context.Context, taskType string, limit int) ([]*ManagedTask, error)
}

// InMemoryManagedTaskStore는 메모리 기반 작업 저장소입니다
type InMemoryManagedTaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*ManagedTask
}

// NewInMemoryManagedTaskStore는 새 InMemoryManagedTaskStore를 생성합니다
func NewInMemoryManagedTaskStore() *InMemoryManagedTaskStore {
	return &InMemoryManagedTaskStore{
		tasks: make(map[string]*ManagedTask),
	}
}

// Create는 작업을 생성합니다
func (s *InMemoryManagedTaskStore) Create(ctx context.Context, task *ManagedTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; exists {
		return ErrTaskExists
	}

	s.tasks[task.ID] = task
	return nil
}

// Get은 작업을 조회합니다
func (s *InMemoryManagedTaskStore) Get(ctx context.Context, id string) (*ManagedTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// Update는 작업을 업데이트합니다
func (s *InMemoryManagedTaskStore) Update(ctx context.Context, task *ManagedTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[task.ID]; !exists {
		return ErrTaskNotFound
	}

	task.UpdatedAt = time.Now()
	s.tasks[task.ID] = task
	return nil
}

// Delete는 작업을 삭제합니다
func (s *InMemoryManagedTaskStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tasks[id]; !exists {
		return ErrTaskNotFound
	}

	delete(s.tasks, id)
	return nil
}

// List는 작업 목록을 반환합니다
func (s *InMemoryManagedTaskStore) List(ctx context.Context, status *TaskStatus, limit int) ([]*ManagedTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ManagedTask
	for _, task := range s.tasks {
		if status != nil && task.Status != *status {
			continue
		}
		result = append(result, task)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

// ListByType은 타입별 작업 목록을 반환합니다
func (s *InMemoryManagedTaskStore) ListByType(ctx context.Context, taskType string, limit int) ([]*ManagedTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ManagedTask
	for _, task := range s.tasks {
		if task.Type != taskType {
			continue
		}
		result = append(result, task)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}
