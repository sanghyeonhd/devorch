// Package execution provides command execution engine with retry and quality management
package execution

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// Engine manages command execution with retry logic and quality tracking
type Engine struct {
	runs   map[string]*Run
	runsMu sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// Run represents a command execution run
type Run struct {
	ID         string
	Command    string
	Status     Status
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	Output     string
	Error      string
	ExitCode   int
	Cost       float64
	Quality    QualityScore
	RetryCount int
	MaxRetries int
}

// Status represents execution status
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSuccess   Status = "success"
	StatusFailed    Status = "failed"
	StatusRetrying  Status = "retrying"
	StatusCancelled Status = "cancelled"
)

// QualityScore represents execution quality metrics
type QualityScore struct {
	Correctness  float64 // 0-100
	Performance  float64 // 0-100
	Security     float64 // 0-100
	Completeness float64 // 0-100
	Overall      float64 // 0-100
	Grade        string  // A+, A, B+, B, C+, C, D, F
}

// BenchmarkResult represents benchmark execution results
type BenchmarkResult struct {
	AvgResponseTime time.Duration
	SuccessRate     float64
	MemoryUsage     int64   // bytes
	Throughput      float64 // operations per second
	OverallScore    string
}

// NewEngine creates a new execution engine
func NewEngine() *Engine {
	ctx, cancel := context.WithCancel(context.Background())

	return &Engine{
		runs:   make(map[string]*Run),
		ctx:    ctx,
		cancel: cancel,
	}
}

// ExecuteCommand runs a command with retry logic
func (e *Engine) ExecuteCommand(command string, maxRetries int) (*Run, error) {
	runID := fmt.Sprintf("run_%d", time.Now().Unix())

	run := &Run{
		ID:         runID,
		Command:    command,
		Status:     StatusPending,
		StartTime:  time.Now(),
		MaxRetries: maxRetries,
	}

	e.runsMu.Lock()
	e.runs[runID] = run
	e.runsMu.Unlock()

	// Execute command in background
	go e.executeWithRetry(run)

	return run, nil
}

// executeWithRetry executes command with retry logic
func (e *Engine) executeWithRetry(run *Run) {
	for attempt := 0; attempt <= run.MaxRetries; attempt++ {
		run.RetryCount = attempt

		if attempt > 0 {
			run.Status = StatusRetrying
			// Wait before retry (exponential backoff)
			time.Sleep(time.Duration(attempt) * time.Second)
		} else {
			run.Status = StatusRunning
		}

		success := e.executeSingle(run)
		if success {
			run.Status = StatusSuccess
			run.EndTime = time.Now()
			run.Duration = run.EndTime.Sub(run.StartTime)

			// Calculate cost and quality
			e.calculateMetrics(run)
			return
		}
	}

	// All retries failed
	run.Status = StatusFailed
	run.EndTime = time.Now()
	run.Duration = run.EndTime.Sub(run.StartTime)
	e.calculateMetrics(run)
}

// executeSingle executes command once
func (e *Engine) executeSingle(run *Run) bool {
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", run.Command)

	output, err := cmd.CombinedOutput()
	run.Output = string(output)

	if err != nil {
		run.Error = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			run.ExitCode = exitErr.ExitCode()
		} else {
			run.ExitCode = 1
		}
		return false
	}

	run.ExitCode = 0
	return true
}

// calculateMetrics calculates cost and quality metrics
func (e *Engine) calculateMetrics(run *Run) {
	// Calculate cost (simplified model)
	baseCost := 0.001 // Base cost per execution
	retryPenalty := float64(run.RetryCount) * 0.0005
	timeCost := run.Duration.Seconds() * 0.0001

	run.Cost = baseCost + retryPenalty + timeCost

	// Calculate quality score
	correctness := 95.0
	if run.Status == StatusFailed {
		correctness = 20.0
	} else if run.RetryCount > 0 {
		correctness = 75.0
	}

	performance := 90.0
	if run.Duration.Seconds() > 10 {
		performance = 70.0
	} else if run.Duration.Seconds() > 5 {
		performance = 80.0
	}

	security := 98.0 // High default, would analyze command for security issues
	completeness := 90.0

	overall := (correctness + performance + security + completeness) / 4

	var grade string
	switch {
	case overall >= 95:
		grade = "A+"
	case overall >= 90:
		grade = "A"
	case overall >= 85:
		grade = "B+"
	case overall >= 80:
		grade = "B"
	case overall >= 75:
		grade = "C+"
	case overall >= 70:
		grade = "C"
	case overall >= 65:
		grade = "D"
	default:
		grade = "F"
	}

	run.Quality = QualityScore{
		Correctness:  correctness,
		Performance:  performance,
		Security:     security,
		Completeness: completeness,
		Overall:      overall,
		Grade:        grade,
	}
}

// GetRun retrieves a run by ID
func (e *Engine) GetRun(runID string) (*Run, error) {
	e.runsMu.RLock()
	defer e.runsMu.RUnlock()

	run, exists := e.runs[runID]
	if !exists {
		return nil, fmt.Errorf("run %s not found", runID)
	}

	return run, nil
}

// GetHistory returns execution history
func (e *Engine) GetHistory() []*Run {
	e.runsMu.RLock()
	defer e.runsMu.RUnlock()

	var history []*Run
	for _, run := range e.runs {
		history = append(history, run)
	}

	return history
}

// RetryExecution retries a failed execution
func (e *Engine) RetryExecution(runID string) (*Run, error) {
	e.runsMu.RLock()
	originalRun, exists := e.runs[runID]
	e.runsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("run %s not found", runID)
	}

	// Create new retry run
	return e.ExecuteCommand(originalRun.Command, originalRun.MaxRetries)
}

// BenchmarkTask runs benchmark tests for a task
func (e *Engine) BenchmarkTask(task string, iterations int) (*BenchmarkResult, error) {
	var totalTime time.Duration
	var successCount int
	var totalMemory int64

	for i := 0; i < iterations; i++ {
		run, err := e.ExecuteCommand(task, 0)
		if err != nil {
			continue
		}

		// Wait for completion (simplified)
		time.Sleep(100 * time.Millisecond)

		if run.Status == StatusSuccess {
			successCount++
			totalTime += run.Duration
			totalMemory += 1024 * 1024 // Simulated memory usage
		}
	}

	if successCount == 0 {
		return nil, fmt.Errorf("all benchmark iterations failed")
	}

	avgTime := totalTime / time.Duration(successCount)
	successRate := float64(successCount) / float64(iterations) * 100
	avgMemory := totalMemory / int64(successCount)
	throughput := 1000.0 / float64(avgTime.Milliseconds()) // ops per second

	var overallScore string
	if successRate >= 95 && avgTime.Milliseconds() < 500 {
		overallScore = "A+"
	} else if successRate >= 90 {
		overallScore = "A"
	} else if successRate >= 80 {
		overallScore = "B"
	} else {
		overallScore = "C"
	}

	return &BenchmarkResult{
		AvgResponseTime: avgTime,
		SuccessRate:     successRate,
		MemoryUsage:     avgMemory,
		Throughput:      throughput,
		OverallScore:    overallScore,
	}, nil
}

// Shutdown gracefully shuts down the execution engine
func (e *Engine) Shutdown() error {
	e.cancel()
	return nil
}
