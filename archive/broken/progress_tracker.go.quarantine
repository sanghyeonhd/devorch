package orchestrator

import (
	"fmt"
	"sync"
	"time"
)

// ProgressTracker tracks the overall progress of autonomous operations
type ProgressTracker struct {
	currentGoal      *AutonomousGoal
	startTime        time.Time
	lastUpdateTime   time.Time
	totalTasks       int
	completedTasks   int
	failedTasks      int
	estimatedEndTime time.Time
	actualProgress   float64 // 0.0 - 1.0
	milestones       []*Milestone
	blockers         []*Blocker
	metrics          *OverallMetrics
	mu               sync.RWMutex
}

// Milestone represents a significant achievement in the autonomous process
type Milestone struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	TargetDate   time.Time  `json:"target_date"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Progress     float64    `json:"progress"` // 0.0 - 1.0
	IsCompleted  bool       `json:"is_completed"`
	Dependencies []string   `json:"dependencies"`
}

// Blocker represents an issue preventing progress
type Blocker struct {
	ID          string                 `json:"id"`
	Type        BlockerType            `json:"type"`
	Severity    BlockerSeverity        `json:"severity"`
	Description string                 `json:"description"`
	DetectedAt  time.Time              `json:"detected_at"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	IsResolved  bool                   `json:"is_resolved"`
	Context     map[string]interface{} `json:"context"`
	AutoRetries int                    `json:"auto_retries"`
}

// BlockerType defines types of blockers
type BlockerType int

const (
	BlockerTypeAPIError BlockerType = iota
	BlockerTypeResourceLimit
	BlockerTypeDependencyMissing
	BlockerTypeQualityIssue
	BlockerTypeBudgetExceeded
	BlockerTypeHardwareLimitation
)

// BlockerSeverity defines blocker severity levels
type BlockerSeverity int

const (
	BlockerSeverityLow BlockerSeverity = iota
	BlockerSeverityMedium
	BlockerSeverityHigh
	BlockerSeverityCritical
)

// OverallMetrics contains comprehensive progress metrics
type OverallMetrics struct {
	TotalCost          float64                    `json:"total_cost"`
	CostPerHour        float64                    `json:"cost_per_hour"`
	TokensUsed         int64                      `json:"tokens_used"`
	APICallsCount      int64                      `json:"api_calls_count"`
	AverageQuality     float64                    `json:"average_quality"`
	SuccessRate        float64                    `json:"success_rate"`
	TierUtilization    map[int]float64            `json:"tier_utilization"` // Tier -> utilization %
	ProviderUsage      map[string]ProviderMetrics `json:"provider_usage"`
	PerformanceHistory []PerformanceSnapshot      `json:"performance_history"`
}

// ProviderMetrics tracks usage per AI provider
type ProviderMetrics struct {
	TokensUsed     int64   `json:"tokens_used"`
	Cost           float64 `json:"cost"`
	CallsCount     int64   `json:"calls_count"`
	SuccessRate    float64 `json:"success_rate"`
	AverageLatency float64 `json:"average_latency_ms"`
	ErrorCount     int64   `json:"error_count"`
}

// PerformanceSnapshot captures performance at a point in time
type PerformanceSnapshot struct {
	Timestamp    time.Time `json:"timestamp"`
	Progress     float64   `json:"progress"`
	TasksPerHour float64   `json:"tasks_per_hour"`
	CostPerHour  float64   `json:"cost_per_hour"`
	QualityScore float64   `json:"quality_score"`
	ActiveTiers  []int     `json:"active_tiers"`
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		startTime:      time.Now(),
		lastUpdateTime: time.Now(),
		actualProgress: 0.0,
		milestones:     make([]*Milestone, 0),
		blockers:       make([]*Blocker, 0),
		metrics: &OverallMetrics{
			TierUtilization:    make(map[int]float64),
			ProviderUsage:      make(map[string]ProviderMetrics),
			PerformanceHistory: make([]PerformanceSnapshot, 0),
		},
	}
}

// SetGoal sets the current autonomous goal being tracked
func (pt *ProgressTracker) SetGoal(goal *AutonomousGoal) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.currentGoal = goal
	pt.startTime = time.Now()
	pt.lastUpdateTime = time.Now()

	// Create initial milestones based on goal
	pt.createInitialMilestones(goal)
}

// UpdateProgress updates the current progress
func (pt *ProgressTracker) UpdateProgress() error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.lastUpdateTime = time.Now()

	// Calculate progress based on completed tasks and milestones
	pt.calculateProgress()

	// Update metrics
	pt.updateMetrics()

	// Check for blockers
	pt.detectBlockers()

	// Take performance snapshot
	pt.takePerformanceSnapshot()

	return nil
}

// GetCurrentStatus returns the current progress status
func (pt *ProgressTracker) GetCurrentStatus() (*ProgressStatus, error) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	runtime := time.Since(pt.startTime)
	estimatedTotal := pt.estimateRemainingTime()

	var currentTaskName string
	var completedTasksList []string
	var pendingTasksList []string
	var blockersList []string

	// In real implementation, these would come from TaskQueue
	currentTaskName = "Implementing REST API endpoints"
	completedTasksList = []string{"Requirements Analysis", "Database Design"}
	pendingTasksList = []string{"Unit Testing", "Documentation", "Deployment"}

	for _, blocker := range pt.blockers {
		if !blocker.IsResolved {
			blockersList = append(blockersList, blocker.Description)
		}
	}

	return &ProgressStatus{
		GoalID:         pt.currentGoal.ID,
		Progress:       pt.actualProgress,
		EstimatedTime:  duration(estimatedTotal),
		ActiveTier:     pt.getCurrentActiveTier(),
		CurrentTask:    currentTaskName,
		CompletedTasks: completedTasksList,
		PendingTasks:   pendingTasksList,
		Blockers:       blockersList,
		CostSoFar:      pt.metrics.TotalCost,
		LastActivity:   pt.lastUpdateTime,
	}, nil
}

// AddMilestone adds a new milestone
func (pt *ProgressTracker) AddMilestone(milestone *Milestone) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.milestones = append(pt.milestones, milestone)
}

// CompleteMilestone marks a milestone as completed
func (pt *ProgressTracker) CompleteMilestone(milestoneID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	for _, milestone := range pt.milestones {
		if milestone.ID == milestoneID {
			milestone.IsCompleted = true
			milestone.Progress = 1.0
			now := time.Now()
			milestone.CompletedAt = &now
			break
		}
	}
}

// AddBlocker adds a new blocker
func (pt *ProgressTracker) AddBlocker(blocker *Blocker) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.blockers = append(pt.blockers, blocker)
}

// ResolveBlocker marks a blocker as resolved
func (pt *ProgressTracker) ResolveBlocker(blockerID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	for _, blocker := range pt.blockers {
		if blocker.ID == blockerID {
			blocker.IsResolved = true
			now := time.Now()
			blocker.ResolvedAt = &now
			break
		}
	}
}

// GetMilestones returns all milestones
func (pt *ProgressTracker) GetMilestones() []*Milestone {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.milestones
}

// GetActiveBlockers returns unresolved blockers
func (pt *ProgressTracker) GetActiveBlockers() []*Blocker {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	var activeBlockers []*Blocker
	for _, blocker := range pt.blockers {
		if !blocker.IsResolved {
			activeBlockers = append(activeBlockers, blocker)
		}
	}

	return activeBlockers
}

// GetMetrics returns current overall metrics
func (pt *ProgressTracker) GetMetrics() *OverallMetrics {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.metrics
}

// createInitialMilestones creates initial milestones based on the goal
func (pt *ProgressTracker) createInitialMilestones(goal *AutonomousGoal) {
	// Create standard milestones for AI OS development process
	baseMilestones := []*Milestone{
		{
			ID:           "milestone_analysis",
			Name:         "Requirements Analysis",
			Description:  "Complete analysis of project requirements and constraints",
			TargetDate:   time.Now().Add(30 * time.Minute),
			Progress:     0.0,
			Dependencies: []string{},
		},
		{
			ID:           "milestone_design",
			Name:         "System Design",
			Description:  "Complete architecture and component design",
			TargetDate:   time.Now().Add(90 * time.Minute),
			Progress:     0.0,
			Dependencies: []string{"milestone_analysis"},
		},
		{
			ID:           "milestone_implementation",
			Name:         "Core Implementation",
			Description:  "Implement core functionality and features",
			TargetDate:   time.Now().Add(4 * time.Hour),
			Progress:     0.0,
			Dependencies: []string{"milestone_design"},
		},
		{
			ID:           "milestone_testing",
			Name:         "Testing & Validation",
			Description:  "Complete testing and quality validation",
			TargetDate:   time.Now().Add(6 * time.Hour),
			Progress:     0.0,
			Dependencies: []string{"milestone_implementation"},
		},
		{
			ID:           "milestone_deployment",
			Name:         "Deployment Ready",
			Description:  "System ready for deployment",
			TargetDate:   time.Now().Add(8 * time.Hour),
			Progress:     0.0,
			Dependencies: []string{"milestone_testing"},
		},
	}

	pt.milestones = append(pt.milestones, baseMilestones...)
}

// calculateProgress calculates overall progress based on milestones and tasks
func (pt *ProgressTracker) calculateProgress() {
	if len(pt.milestones) == 0 {
		return
	}

	totalWeight := float64(len(pt.milestones))
	completedWeight := 0.0

	for _, milestone := range pt.milestones {
		completedWeight += milestone.Progress
	}

	pt.actualProgress = completedWeight / totalWeight

	// Ensure progress doesn't exceed 1.0
	if pt.actualProgress > 1.0 {
		pt.actualProgress = 1.0
	}
}

// updateMetrics updates the overall metrics
func (pt *ProgressTracker) updateMetrics() {
	// In real implementation, this would aggregate metrics from:
	// - TierManager (costs, usage)
	// - TaskQueue (completion rates, quality scores)
	// - Individual AI model calls

	// Update tier utilization (sample data)
	pt.metrics.TierUtilization[1] = 0.25 // Orchestrator 25% utilization
	pt.metrics.TierUtilization[2] = 0.60 // Senior 60% utilization
	pt.metrics.TierUtilization[3] = 0.80 // Regular 80% utilization
	pt.metrics.TierUtilization[4] = 0.95 // Background 95% utilization

	// Update provider usage (sample data)
	pt.metrics.ProviderUsage["openai"] = ProviderMetrics{
		TokensUsed:     50000,
		Cost:           5.50,
		CallsCount:     125,
		SuccessRate:    0.96,
		AverageLatency: 1200,
	}

	pt.metrics.ProviderUsage["openrouter"] = ProviderMetrics{
		TokensUsed:     30000,
		Cost:           2.40,
		CallsCount:     80,
		SuccessRate:    0.94,
		AverageLatency: 1500,
	}

	// Update totals
	pt.metrics.TotalCost = 7.90
	runtime := time.Since(pt.startTime)
	pt.metrics.CostPerHour = pt.metrics.TotalCost / runtime.Hours()
	pt.metrics.TokensUsed = 80000
	pt.metrics.AverageQuality = 0.87
	pt.metrics.SuccessRate = 0.95
}

// detectBlockers automatically detects potential blockers
func (pt *ProgressTracker) detectBlockers() {
	// Check for budget issues
	if pt.metrics.CostPerHour > 15.0 { // $15/hour budget limit
		pt.AddBlocker(&Blocker{
			ID:          fmt.Sprintf("blocker_%d", time.Now().Unix()),
			Type:        BlockerTypeBudgetExceeded,
			Severity:    BlockerSeverityHigh,
			Description: fmt.Sprintf("Cost per hour ($%.2f) exceeds budget limit", pt.metrics.CostPerHour),
			DetectedAt:  time.Now(),
			Context:     map[string]interface{}{"current_cost": pt.metrics.CostPerHour},
		})
	}

	// Check for quality issues
	if pt.metrics.AverageQuality < 0.7 {
		pt.AddBlocker(&Blocker{
			ID:          fmt.Sprintf("blocker_%d", time.Now().Unix()),
			Type:        BlockerTypeQualityIssue,
			Severity:    BlockerSeverityMedium,
			Description: fmt.Sprintf("Quality score (%.2f) below acceptable threshold", pt.metrics.AverageQuality),
			DetectedAt:  time.Now(),
			Context:     map[string]interface{}{"quality_score": pt.metrics.AverageQuality},
		})
	}

	// Check for provider errors
	for provider, metrics := range pt.metrics.ProviderUsage {
		if metrics.SuccessRate < 0.9 {
			pt.AddBlocker(&Blocker{
				ID:          fmt.Sprintf("blocker_%s_%d", provider, time.Now().Unix()),
				Type:        BlockerTypeAPIError,
				Severity:    BlockerSeverityMedium,
				Description: fmt.Sprintf("Provider %s has low success rate (%.2f)", provider, metrics.SuccessRate),
				DetectedAt:  time.Now(),
				Context:     map[string]interface{}{"provider": provider, "success_rate": metrics.SuccessRate},
			})
		}
	}
}

// takePerformanceSnapshot captures current performance metrics
func (pt *ProgressTracker) takePerformanceSnapshot() {
	runtime := time.Since(pt.startTime)

	snapshot := PerformanceSnapshot{
		Timestamp:    time.Now(),
		Progress:     pt.actualProgress,
		TasksPerHour: float64(pt.completedTasks) / runtime.Hours(),
		CostPerHour:  pt.metrics.CostPerHour,
		QualityScore: pt.metrics.AverageQuality,
		ActiveTiers:  []int{1, 2, 3, 4}, // All tiers active
	}

	pt.metrics.PerformanceHistory = append(pt.metrics.PerformanceHistory, snapshot)

	// Keep only last 100 snapshots
	if len(pt.metrics.PerformanceHistory) > 100 {
		pt.metrics.PerformanceHistory = pt.metrics.PerformanceHistory[1:]
	}
}

// estimateRemainingTime estimates how much time is left
func (pt *ProgressTracker) estimateRemainingTime() time.Duration {
	if pt.actualProgress <= 0 {
		return 8 * time.Hour // Default estimate
	}

	runtime := time.Since(pt.startTime)
	estimatedTotal := time.Duration(float64(runtime) / pt.actualProgress)
	return estimatedTotal - runtime
}

// getCurrentActiveTier returns the tier currently handling the most important task
func (pt *ProgressTracker) getCurrentActiveTier() int {
	// In real implementation, this would come from TaskQueue
	// For now, return tier based on progress stage
	if pt.actualProgress < 0.1 {
		return 1 // Orchestrator doing initial planning
	} else if pt.actualProgress < 0.3 {
		return 2 // Senior developers doing design
	} else if pt.actualProgress < 0.8 {
		return 3 // Regular developers implementing
	} else {
		return 4 // Background workers finishing up
	}
}
