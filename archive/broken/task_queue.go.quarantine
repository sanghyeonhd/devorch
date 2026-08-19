package orchestrator

import (
	"fmt"
	"sync"
	"time"
)

// TaskQueue manages autonomous task scheduling and execution
type TaskQueue struct {
	tasks          []*Task
	completedTasks []*Task
	currentTask    *Task
	mutex          sync.RWMutex
	processingChan chan *Task
	resultChan     chan *TaskResult
	isProcessing   bool
}

// Task represents a single autonomous task
type Task struct {
	ID                string                 `json:"id"`
	Type              TaskType               `json:"type"`
	Description       string                 `json:"description"`
	Priority          Priority               `json:"priority"`
	Tier              Tier                   `json:"tier"` // Which AI tier should handle this
	Status            TaskStatus             `json:"status"`
	CreatedAt         time.Time              `json:"created_at"`
	StartedAt         *time.Time             `json:"started_at,omitempty"`
	CompletedAt       *time.Time             `json:"completed_at,omitempty"`
	Dependencies      []string               `json:"dependencies"` // Task IDs this depends on
	Context           map[string]interface{} `json:"context"`
	Result            *TaskResult            `json:"result,omitempty"`
	EstimatedDuration time.Duration          `json:"estimated_duration"`
	ActualDuration    time.Duration          `json:"actual_duration"`
}

// TaskType defines the type of task
type TaskType int

const (
	TaskTypeAnalyze TaskType = iota
	TaskTypeDesign
	TaskTypeImplement
	TaskTypeTest
	TaskTypeReview
	TaskTypeMonitor
	TaskTypeDecision
	TaskTypeOptimize
)

// TaskStatus defines the current status of a task
type TaskStatus int

const (
	TaskStatusPending TaskStatus = iota
	TaskStatusReady
	TaskStatusRunning
	TaskStatusCompleted
	TaskStatusFailed
	TaskStatusSkipped
)

// TaskResult contains the result of a completed task
type TaskResult struct {
	Success   bool                   `json:"success"`
	Output    string                 `json:"output"`
	ErrorMsg  string                 `json:"error_msg,omitempty"`
	Artifacts map[string]interface{} `json:"artifacts,omitempty"` // Generated files, data, etc.
	Metrics   *TaskMetrics           `json:"metrics,omitempty"`
}

// TaskMetrics contains performance metrics for a task
type TaskMetrics struct {
	TokensUsed   int     `json:"tokens_used"`
	Cost         float64 `json:"cost"`
	ModelUsed    string  `json:"model_used"`
	QualityScore float64 `json:"quality_score"` // 0.0 - 1.0
	SuccessScore float64 `json:"success_score"` // 0.0 - 1.0
}

// NewTaskQueue creates a new task queue
func NewTaskQueue() *TaskQueue {
	tq := &TaskQueue{
		tasks:          make([]*Task, 0),
		completedTasks: make([]*Task, 0),
		processingChan: make(chan *Task, 100),
		resultChan:     make(chan *TaskResult, 100),
		isProcessing:   false,
	}

	// Start background processor
	go tq.processLoop()

	return tq
}

// LoadFromPlan parses an AI-generated plan and creates tasks
func (tq *TaskQueue) LoadFromPlan(planJSON string) error {
	tq.mutex.Lock()
	defer tq.mutex.Unlock()

	// Parse AI response into structured tasks
	var plan struct {
		Goal     string `json:"goal"`
		Strategy string `json:"strategy"`
		Tasks    []struct {
			ID             string                 `json:"id"`
			Type           string                 `json:"type"`
			Description    string                 `json:"description"`
			Priority       string                 `json:"priority"`
			Tier           int                    `json:"tier"`
			Dependencies   []string               `json:"dependencies"`
			Context        map[string]interface{} `json:"context"`
			EstimatedHours float64                `json:"estimated_hours"`
		} `json:"tasks"`
	}

	// For now, create sample tasks since we don't have real AI response
	tq.createSampleTasks()

	return nil
}

// createSampleTasks creates sample tasks for demonstration
func (tq *TaskQueue) createSampleTasks() {
	sampleTasks := []*Task{
		{
			ID:                "task_001",
			Type:              TaskTypeAnalyze,
			Description:       "Analyze project requirements and define architecture",
			Priority:          PriorityHigh,
			Tier:              TierOrchestrator,
			Status:            TaskStatusReady,
			CreatedAt:         time.Now(),
			Dependencies:      []string{},
			Context:           map[string]interface{}{"phase": "planning"},
			EstimatedDuration: 30 * time.Minute,
		},
		{
			ID:                "task_002",
			Type:              TaskTypeDesign,
			Description:       "Design database schema and API structure",
			Priority:          PriorityHigh,
			Tier:              TierSenior,
			Status:            TaskStatusPending,
			CreatedAt:         time.Now(),
			Dependencies:      []string{"task_001"},
			Context:           map[string]interface{}{"component": "backend"},
			EstimatedDuration: 45 * time.Minute,
		},
		{
			ID:                "task_003",
			Type:              TaskTypeImplement,
			Description:       "Implement REST API endpoints",
			Priority:          PriorityMedium,
			Tier:              TierRegular,
			Status:            TaskStatusPending,
			CreatedAt:         time.Now(),
			Dependencies:      []string{"task_002"},
			Context:           map[string]interface{}{"language": "Go"},
			EstimatedDuration: 90 * time.Minute,
		},
		{
			ID:                "task_004",
			Type:              TaskTypeTest,
			Description:       "Write unit tests for API endpoints",
			Priority:          PriorityMedium,
			Tier:              TierRegular,
			Status:            TaskStatusPending,
			CreatedAt:         time.Now(),
			Dependencies:      []string{"task_003"},
			Context:           map[string]interface{}{"test_framework": "go test"},
			EstimatedDuration: 60 * time.Minute,
		},
		{
			ID:                "task_005",
			Type:              TaskTypeMonitor,
			Description:       "Monitor system performance and log issues",
			Priority:          PriorityLow,
			Tier:              TierBackground,
			Status:            TaskStatusReady,
			CreatedAt:         time.Now(),
			Dependencies:      []string{},
			Context:           map[string]interface{}{"continuous": true},
			EstimatedDuration: 24 * time.Hour, // Continuous
		},
	}

	tq.tasks = append(tq.tasks, sampleTasks...)
}

// GetNext returns the next ready task based on priority and dependencies
func (tq *TaskQueue) GetNext() *Task {
	tq.mutex.Lock()
	defer tq.mutex.Unlock()

	// Find highest priority ready task
	var nextTask *Task
	highestPriority := PriorityLow

	for _, task := range tq.tasks {
		if task.Status == TaskStatusReady && task.Priority >= highestPriority {
			// Check if dependencies are satisfied
			if tq.areDependenciesSatisfied(task) {
				nextTask = task
				highestPriority = task.Priority
			}
		}
	}

	if nextTask != nil {
		nextTask.Status = TaskStatusRunning
		nextTask.StartedAt = &[]time.Time{time.Now()}[0]
		tq.currentTask = nextTask
	}

	return nextTask
}

// CompleteTask marks a task as completed with results
func (tq *TaskQueue) CompleteTask(taskID string, result *TaskResult) error {
	tq.mutex.Lock()
	defer tq.mutex.Unlock()

	for i, task := range tq.tasks {
		if task.ID == taskID {
			task.Status = TaskStatusCompleted
			task.Result = result
			completedAt := time.Now()
			task.CompletedAt = &completedAt

			if task.StartedAt != nil {
				task.ActualDuration = completedAt.Sub(*task.StartedAt)
			}

			// Move to completed tasks
			tq.completedTasks = append(tq.completedTasks, task)
			tq.tasks = append(tq.tasks[:i], tq.tasks[i+1:]...)

			// Update dependent tasks to ready if their dependencies are now satisfied
			tq.updateDependentTasks(taskID)

			return nil
		}
	}

	return fmt.Errorf("task not found: %s", taskID)
}

// GetQueueStatus returns current queue status
func (tq *TaskQueue) GetQueueStatus() *QueueStatus {
	tq.mutex.RLock()
	defer tq.mutex.RUnlock()

	status := &QueueStatus{
		TotalTasks:     len(tq.tasks) + len(tq.completedTasks),
		PendingTasks:   0,
		ReadyTasks:     0,
		RunningTasks:   0,
		CompletedTasks: len(tq.completedTasks),
		FailedTasks:    0,
		CurrentTask:    tq.currentTask,
	}

	for _, task := range tq.tasks {
		switch task.Status {
		case TaskStatusPending:
			status.PendingTasks++
		case TaskStatusReady:
			status.ReadyTasks++
		case TaskStatusRunning:
			status.RunningTasks++
		case TaskStatusFailed:
			status.FailedTasks++
		}
	}

	return status
}

// areDependenciesSatisfied checks if all task dependencies are completed
func (tq *TaskQueue) areDependenciesSatisfied(task *Task) bool {
	if len(task.Dependencies) == 0 {
		return true
	}

	// Check each dependency
	for _, depID := range task.Dependencies {
		found := false
		for _, completedTask := range tq.completedTasks {
			if completedTask.ID == depID && completedTask.Status == TaskStatusCompleted {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// updateDependentTasks updates tasks that depend on the completed task
func (tq *TaskQueue) updateDependentTasks(completedTaskID string) {
	for _, task := range tq.tasks {
		if task.Status == TaskStatusPending {
			for _, depID := range task.Dependencies {
				if depID == completedTaskID && tq.areDependenciesSatisfied(task) {
					task.Status = TaskStatusReady
					break
				}
			}
		}
	}
}

// processLoop is the background task processor
func (tq *TaskQueue) processLoop() {
	for {
		select {
		case task := <-tq.processingChan:
			// Process task (this would integrate with AI models)
			tq.processTask(task)
		case result := <-tq.resultChan:
			// Handle task result
			tq.handleTaskResult(result)
		case <-time.After(5 * time.Second):
			// Periodic maintenance
			tq.performMaintenance()
		}
	}
}

// processTask processes a single task (placeholder for AI integration)
func (tq *TaskQueue) processTask(task *Task) {
	// TODO: Integrate with AI models based on task.Tier
	// This would call the appropriate tier model to execute the task
}

// handleTaskResult handles the result of a processed task
func (tq *TaskQueue) handleTaskResult(result *TaskResult) {
	// TODO: Process task result, update metrics, trigger next tasks
}

// performMaintenance performs periodic queue maintenance
func (tq *TaskQueue) performMaintenance() {
	// TODO: Clean up old tasks, update priorities, etc.
}

// QueueStatus represents the current status of the task queue
type QueueStatus struct {
	TotalTasks     int   `json:"total_tasks"`
	PendingTasks   int   `json:"pending_tasks"`
	ReadyTasks     int   `json:"ready_tasks"`
	RunningTasks   int   `json:"running_tasks"`
	CompletedTasks int   `json:"completed_tasks"`
	FailedTasks    int   `json:"failed_tasks"`
	CurrentTask    *Task `json:"current_task"`
}
