package delegate

import (
	"log/slog"
	"sync"
)

// HandlerRegistry는 핸들러 레지스트리입니다
type HandlerRegistry struct {
	mu       sync.RWMutex
	handlers map[string]DelegateHandler
	logger   *slog.Logger
}

// NewHandlerRegistry는 새 HandlerRegistry를 생성합니다
func NewHandlerRegistry(logger *slog.Logger) *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]DelegateHandler),
		logger:   logger,
	}
}

// Register는 핸들러를 등록합니다
func (hr *HandlerRegistry) Register(taskType string, handler DelegateHandler) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.handlers[taskType] = handler
	hr.logger.Debug("handler registered", "task_type", taskType)
}

// Unregister는 핸들러를 제거합니다
func (hr *HandlerRegistry) Unregister(taskType string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	delete(hr.handlers, taskType)
	hr.logger.Debug("handler unregistered", "task_type", taskType)
}

// Get은 핸들러를 조회합니다
func (hr *HandlerRegistry) Get(taskType string) (DelegateHandler, bool) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	handler, ok := hr.handlers[taskType]
	return handler, ok
}

// GetForTask는 태스크를 처리할 수 있는 핸들러를 찾습니다
func (hr *HandlerRegistry) GetForTask(taskType string) (DelegateHandler, bool) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	// 정확히 일치하는 핸들러 먼저
	if handler, ok := hr.handlers[taskType]; ok {
		return handler, true
	}

	// CanHandle로 처리 가능한 핸들러 찾기
	for _, handler := range hr.handlers {
		if handler.CanHandle(taskType) {
			return handler, true
		}
	}

	return nil, false
}

// List는 등록된 모든 태스크 타입을 반환합니다
func (hr *HandlerRegistry) List() []string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	types := make([]string, 0, len(hr.handlers))
	for t := range hr.handlers {
		types = append(types, t)
	}
	return types
}

// DelegateRouter는 위임 요청을 라우팅합니다
type DelegateRouter struct {
	registry *HandlerRegistry
	logger   *slog.Logger
}

// NewDelegateRouter는 새 DelegateRouter를 생성합니다
func NewDelegateRouter(registry *HandlerRegistry, logger *slog.Logger) *DelegateRouter {
	return &DelegateRouter{
		registry: registry,
		logger:   logger,
	}
}

// Route는 요청을 적절한 핸들러로 라우팅합니다
func (dr *DelegateRouter) Route(req DelegateRequest) (DelegateHandler, bool) {
	handler, ok := dr.registry.GetForTask(req.TaskType)
	if !ok {
		dr.logger.Warn("no handler found for task", "task_type", req.TaskType)
		return nil, false
	}
	return handler, true
}
