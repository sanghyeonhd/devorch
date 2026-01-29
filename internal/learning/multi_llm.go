// Package learning implements advanced multi-LLM self-learning and optimization
package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// ModelPerformance tracks individual model performance metrics
type ModelPerformance struct {
	Provider       string    `json:"provider"`
	Model          string    `json:"model"`
	TotalCalls     int64     `json:"total_calls"`
	SuccessfulCalls int64    `json:"successful_calls"`
	FailedCalls    int64     `json:"failed_calls"`
	TotalLatencyMs int64     `json:"total_latency_ms"`
	TotalTokens    int64     `json:"total_tokens"`
	TotalCostMicro int64     `json:"total_cost_micro"`
	QualitySum     float64   `json:"quality_sum"`
	LastUsed       time.Time `json:"last_used"`
	
	// Thompson Sampling state (Beta distribution)
	Alpha float64 `json:"alpha"` // Success count + 1
	Beta  float64 `json:"beta"`  // Failure count + 1
	
	// Task-specific performance
	TaskScores map[string]*TaskScore `json:"task_scores"` // task_type -> score
}

// TaskScore tracks per-task-type performance
type TaskScore struct {
	TaskType     string  `json:"task_type"`
	Calls        int64   `json:"calls"`
	AvgQuality   float64 `json:"avg_quality"`
	Alpha        float64 `json:"alpha"`
	Beta         float64 `json:"beta"`
	LastReward   float64 `json:"last_reward"`
}

// EnsembleStrategy defines how multiple models work together
type EnsembleStrategy string

const (
	StrategyBestSingle     EnsembleStrategy = "best_single"      // Use single best model
	StrategyParallel       EnsembleStrategy = "parallel"         // Run in parallel, use best response
	StrategyChain          EnsembleStrategy = "chain"            // Chain models for refinement
	StrategyVoting         EnsembleStrategy = "voting"           // Multiple models vote
	StrategySpecialized    EnsembleStrategy = "specialized"      // Route to task-specific expert
	StrategyAdaptive       EnsembleStrategy = "adaptive"         // Dynamically select strategy
)

// EnsembleConfig configures multi-model ensemble behavior
type EnsembleConfig struct {
	Strategy           EnsembleStrategy `json:"strategy"`
	MaxParallelModels  int              `json:"max_parallel_models"`
	MinConsensus       float64          `json:"min_consensus"`        // For voting
	QualityThreshold   float64          `json:"quality_threshold"`    // Min quality to accept
	LatencyBudgetMs    int64            `json:"latency_budget_ms"`    // Max latency allowed
	ExplorationRate    float64          `json:"exploration_rate"`     // Thompson exploration
	EnableAutoOptimize bool             `json:"enable_auto_optimize"` // Self-optimization
}

// DefaultEnsembleConfig returns sensible defaults
func DefaultEnsembleConfig() EnsembleConfig {
	return EnsembleConfig{
		Strategy:           StrategyAdaptive,
		MaxParallelModels:  3,
		MinConsensus:       0.7,
		QualityThreshold:   0.6,
		LatencyBudgetMs:    30000,
		ExplorationRate:    0.15,
		EnableAutoOptimize: true,
	}
}

// scoredModel is used internally for model ranking
type scoredModel struct {
	key           string
	thompsonScore float64
	exploitScore  float64
	taskScore     float64
}

// MultiLLMOrchestrator manages multi-model orchestration with self-learning
type MultiLLMOrchestrator struct {
	mu         sync.RWMutex
	models     map[string]*ModelPerformance // key: provider:model
	config     EnsembleConfig
	store      PerformanceStore
	rng        *rand.Rand
	userProfile *UserProfile
}

// UserProfile tracks user-specific preferences learned over time
type UserProfile struct {
	UserID           string                    `json:"user_id"`
	PreferredModels  []string                  `json:"preferred_models"`
	TaskPreferences  map[string]string         `json:"task_preferences"`  // task_type -> preferred model
	QualitySensitivity float64                 `json:"quality_sensitivity"` // 0-1, how much quality matters
	LatencySensitivity float64                 `json:"latency_sensitivity"` // 0-1, how much speed matters
	CostSensitivity    float64                 `json:"cost_sensitivity"`    // 0-1, how much cost matters
	InteractionCount   int64                   `json:"interaction_count"`
	LastOptimized      time.Time               `json:"last_optimized"`
}

// PerformanceStore persists learning state
type PerformanceStore interface {
	SaveModelPerformance(ctx context.Context, perf *ModelPerformance) error
	LoadModelPerformance(ctx context.Context, provider, model string) (*ModelPerformance, error)
	ListAllModels(ctx context.Context) ([]*ModelPerformance, error)
	SaveUserProfile(ctx context.Context, profile *UserProfile) error
	LoadUserProfile(ctx context.Context, userID string) (*UserProfile, error)
	SaveEnsembleConfig(ctx context.Context, config EnsembleConfig) error
	LoadEnsembleConfig(ctx context.Context) (EnsembleConfig, error)
}

// NewMultiLLMOrchestrator creates a new orchestrator
func NewMultiLLMOrchestrator(store PerformanceStore, config EnsembleConfig) *MultiLLMOrchestrator {
	return &MultiLLMOrchestrator{
		models:      make(map[string]*ModelPerformance),
		config:      config,
		store:       store,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		userProfile: &UserProfile{
			QualitySensitivity: 0.5,
			LatencySensitivity: 0.3,
			CostSensitivity:    0.2,
		},
	}
}

// ModelKey creates a unique key for provider:model
func ModelKey(provider, model string) string {
	return fmt.Sprintf("%s:%s", provider, model)
}

// SelectionRequest is the input for model selection
type SelectionRequest struct {
	TaskType      string   // e.g., "code_generation", "chat", "analysis"
	Complexity    float64  // 0-1, estimated task complexity
	RequiredCaps  []string // Required capabilities
	Candidates    []string // Available model keys (provider:model)
	LatencyBudget int64    // Max latency in ms (0 = no limit)
	QualityMin    float64  // Minimum acceptable quality
}

// SelectionResult is the output of model selection
type SelectionResult struct {
	SelectedModels []string         // Ordered by preference
	Strategy       EnsembleStrategy // Strategy to use
	Confidence     float64          // How confident we are in this selection
	Explanation    string           // Why these models were selected
}

// SelectModels uses Thompson Sampling to select optimal model(s)
func (o *MultiLLMOrchestrator) SelectModels(ctx context.Context, req SelectionRequest) (*SelectionResult, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if len(req.Candidates) == 0 {
		return nil, fmt.Errorf("no candidate models provided")
	}

	// Calculate Thompson samples for each candidate
	scored := make([]scoredModel, 0, len(req.Candidates))
	
	for _, key := range req.Candidates {
		perf, exists := o.models[key]
		if !exists {
			// New model - use optimistic prior
			scored = append(scored, scoredModel{
				key:           key,
				thompsonScore: o.sampleBeta(1.5, 1.0), // Optimistic prior
				exploitScore:  0.5,
				taskScore:     0.5,
			})
			continue
		}

		// Thompson Sampling: sample from Beta distribution
		thompsonScore := o.sampleBeta(perf.Alpha, perf.Beta)
		
		// Exploit score based on actual performance
		exploitScore := o.calculateExploitScore(perf, req)
		
		// Task-specific score if available
		taskScore := 0.5
		if ts, ok := perf.TaskScores[req.TaskType]; ok {
			taskScore = o.sampleBeta(ts.Alpha, ts.Beta)
		}

		scored = append(scored, scoredModel{
			key:           key,
			thompsonScore: thompsonScore,
			exploitScore:  exploitScore,
			taskScore:     taskScore,
		})
	}

	// Combine scores with user preferences
	for i := range scored {
		combined := o.combineScores(scored[i].thompsonScore, scored[i].exploitScore, scored[i].taskScore)
		scored[i].thompsonScore = combined
	}

	// Sort by combined score
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].thompsonScore > scored[j].thompsonScore
	})

	// Determine strategy based on task and config
	strategy := o.selectStrategy(req, scored)
	
	// Select models based on strategy
	selectedCount := 1
	if strategy == StrategyParallel || strategy == StrategyVoting {
		selectedCount = min(o.config.MaxParallelModels, len(scored))
	} else if strategy == StrategyChain {
		selectedCount = min(2, len(scored)) // Chain typically uses 2 models
	}

	selected := make([]string, 0, selectedCount)
	for i := 0; i < selectedCount && i < len(scored); i++ {
		selected = append(selected, scored[i].key)
	}

	// Calculate confidence
	confidence := 0.5
	if len(scored) > 0 {
		topScore := scored[0].thompsonScore
		if len(scored) > 1 {
			gap := topScore - scored[1].thompsonScore
			confidence = math.Min(1.0, 0.5+gap)
		} else {
			confidence = topScore
		}
	}

	explanation := o.generateExplanation(selected, strategy, scored)

	return &SelectionResult{
		SelectedModels: selected,
		Strategy:       strategy,
		Confidence:     confidence,
		Explanation:    explanation,
	}, nil
}

// RecordOutcome records the result of a model interaction for learning
func (o *MultiLLMOrchestrator) RecordOutcome(ctx context.Context, outcome *ModelOutcome) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	key := ModelKey(outcome.Provider, outcome.Model)
	
	perf, exists := o.models[key]
	if !exists {
		perf = &ModelPerformance{
			Provider:   outcome.Provider,
			Model:      outcome.Model,
			Alpha:      1.0, // Prior
			Beta:       1.0, // Prior
			TaskScores: make(map[string]*TaskScore),
		}
		o.models[key] = perf
	}

	// Update basic stats
	perf.TotalCalls++
	perf.TotalLatencyMs += outcome.LatencyMs
	perf.TotalTokens += outcome.Tokens
	perf.TotalCostMicro += outcome.CostMicro
	perf.LastUsed = time.Now()

	if outcome.Success {
		perf.SuccessfulCalls++
	} else {
		perf.FailedCalls++
	}

	perf.QualitySum += outcome.Quality

	// Update Thompson Sampling state
	reward := o.calculateReward(outcome)
	perf.Alpha += reward
	perf.Beta += (1.0 - reward)

	// Update task-specific scores
	if outcome.TaskType != "" {
		ts, exists := perf.TaskScores[outcome.TaskType]
		if !exists {
			ts = &TaskScore{
				TaskType: outcome.TaskType,
				Alpha:    1.0,
				Beta:     1.0,
			}
			perf.TaskScores[outcome.TaskType] = ts
		}
		ts.Calls++
		ts.Alpha += reward
		ts.Beta += (1.0 - reward)
		ts.LastReward = reward
		ts.AvgQuality = ((ts.AvgQuality * float64(ts.Calls-1)) + outcome.Quality) / float64(ts.Calls)
	}

	// Persist if store available
	if o.store != nil {
		if err := o.store.SaveModelPerformance(ctx, perf); err != nil {
			// Log error but don't fail
			fmt.Printf("Warning: failed to persist model performance: %v\n", err)
		}
	}

	// Trigger auto-optimization if enabled
	if o.config.EnableAutoOptimize && perf.TotalCalls%100 == 0 {
		go o.autoOptimize(context.Background())
	}

	return nil
}

// ModelOutcome represents the result of a model interaction
type ModelOutcome struct {
	Provider   string
	Model      string
	TaskType   string
	Success    bool
	Quality    float64 // 0-1
	LatencyMs  int64
	Tokens     int64
	CostMicro  int64
	UserRating *float64 // Optional explicit user feedback
}

// calculateReward computes the learning reward from an outcome
func (o *MultiLLMOrchestrator) calculateReward(outcome *ModelOutcome) float64 {
	if !outcome.Success {
		return 0.0
	}

	// Base reward from quality
	reward := outcome.Quality

	// Apply user profile adjustments if available
	if o.userProfile != nil {
		// Latency adjustment (faster = better if user cares about latency)
		if o.userProfile.LatencySensitivity > 0 && outcome.LatencyMs > 0 {
			// Normalize latency: 1s = 0, 30s = -1
			latencyPenalty := float64(outcome.LatencyMs) / 30000.0 * o.userProfile.LatencySensitivity * 0.3
			reward = math.Max(0, reward-latencyPenalty)
		}

		// Cost adjustment
		if o.userProfile.CostSensitivity > 0 && outcome.CostMicro > 0 {
			// Normalize cost: $0.001 = 0, $0.1 = -1
			costPenalty := float64(outcome.CostMicro) / 100000.0 * o.userProfile.CostSensitivity * 0.3
			reward = math.Max(0, reward-costPenalty)
		}
	}

	// Explicit user rating overrides calculated reward
	if outcome.UserRating != nil {
		reward = *outcome.UserRating * 0.7 + reward * 0.3
	}

	return clamp01(reward)
}

// calculateExploitScore calculates a deterministic exploitation score
func (o *MultiLLMOrchestrator) calculateExploitScore(perf *ModelPerformance, req SelectionRequest) float64 {
	if perf.TotalCalls == 0 {
		return 0.5 // Neutral for new models
	}

	// Success rate
	successRate := float64(perf.SuccessfulCalls) / float64(perf.TotalCalls)
	
	// Average quality
	avgQuality := perf.QualitySum / float64(perf.TotalCalls)
	
	// Latency score (lower is better)
	avgLatency := float64(perf.TotalLatencyMs) / float64(perf.TotalCalls)
	latencyScore := 1.0
	if req.LatencyBudget > 0 {
		latencyScore = math.Max(0, 1.0-avgLatency/float64(req.LatencyBudget))
	} else {
		latencyScore = math.Max(0, 1.0-avgLatency/30000.0)
	}

	// Combine with user preferences
	qs := 0.4
	ls := 0.3
	ss := 0.3
	if o.userProfile != nil {
		total := o.userProfile.QualitySensitivity + o.userProfile.LatencySensitivity + o.userProfile.CostSensitivity
		if total > 0 {
			qs = o.userProfile.QualitySensitivity / total
			ls = o.userProfile.LatencySensitivity / total
			ss = 1.0 - qs - ls
		}
	}

	return qs*avgQuality + ls*latencyScore + ss*successRate
}

// combineScores combines exploration and exploitation scores
func (o *MultiLLMOrchestrator) combineScores(thompson, exploit, task float64) float64 {
	// Use exploration rate to balance
	exploration := o.config.ExplorationRate
	
	// Thompson provides exploration through sampling
	// Exploit provides exploitation through historical performance
	// Task provides task-specific expertise
	
	combined := thompson*exploration + exploit*(1-exploration)*0.6 + task*(1-exploration)*0.4
	return clamp01(combined)
}

// selectStrategy determines the best ensemble strategy
func (o *MultiLLMOrchestrator) selectStrategy(req SelectionRequest, scored []scoredModel) EnsembleStrategy {
	if o.config.Strategy != StrategyAdaptive {
		return o.config.Strategy
	}

	// Adaptive strategy selection based on task characteristics
	
	// High complexity tasks benefit from parallel/voting
	if req.Complexity > 0.7 && len(scored) >= 2 {
		return StrategyParallel
	}

	// Code tasks often benefit from chain (draft + refine)
	if req.TaskType == "code_generation" || req.TaskType == "code_review" {
		if len(scored) >= 2 {
			return StrategyChain
		}
	}

	// If we have clear task-specific experts
	if req.TaskType != "" && o.hasTaskExpert(req.TaskType) {
		return StrategySpecialized
	}

	// If top model is significantly better, use single
	if len(scored) >= 2 {
		gap := scored[0].thompsonScore - scored[1].thompsonScore
		if gap > 0.2 {
			return StrategyBestSingle
		}
	}

	// Default to parallel for uncertain cases
	if len(scored) >= 2 {
		return StrategyParallel
	}

	return StrategyBestSingle
}

// hasTaskExpert checks if there's a clear expert model for a task
func (o *MultiLLMOrchestrator) hasTaskExpert(taskType string) bool {
	var bestScore float64 = -1
	var secondScore float64 = -1
	
	for _, perf := range o.models {
		if ts, ok := perf.TaskScores[taskType]; ok {
			score := ts.AvgQuality * float64(ts.Calls) // Weighted by experience
			if score > bestScore {
				secondScore = bestScore
				bestScore = score
			} else if score > secondScore {
				secondScore = score
			}
		}
	}

	// Expert if significantly better than second best
	return bestScore > 0 && (secondScore < 0 || bestScore > secondScore*1.5)
}

// generateExplanation creates a human-readable explanation
func (o *MultiLLMOrchestrator) generateExplanation(selected []string, strategy EnsembleStrategy, scored []scoredModel) string {
	if len(selected) == 0 {
		return "No models selected"
	}

	strategyDesc := map[EnsembleStrategy]string{
		StrategyBestSingle:  "Using single best model based on historical performance",
		StrategyParallel:    "Running models in parallel for higher quality",
		StrategyChain:       "Chaining models: first drafts, second refines",
		StrategyVoting:      "Multiple models voting for consensus",
		StrategySpecialized: "Using task-specialized expert model",
		StrategyAdaptive:    "Adaptively selected strategy",
	}

	explanation := strategyDesc[strategy]
	if len(selected) > 0 {
		explanation += fmt.Sprintf(". Primary: %s (score: %.2f)", selected[0], scored[0].thompsonScore)
	}

	return explanation
}

// sampleBeta samples from a Beta distribution using Box-Muller approximation
func (o *MultiLLMOrchestrator) sampleBeta(alpha, beta float64) float64 {
	// Use gamma sampling for accurate Beta distribution
	// Beta(a,b) = Gamma(a) / (Gamma(a) + Gamma(b))
	x := o.sampleGamma(alpha)
	y := o.sampleGamma(beta)
	if x+y == 0 {
		return 0.5
	}
	return x / (x + y)
}

// sampleGamma samples from Gamma distribution using Marsaglia and Tsang's method
func (o *MultiLLMOrchestrator) sampleGamma(shape float64) float64 {
	if shape < 1 {
		return o.sampleGamma(1+shape) * math.Pow(o.rng.Float64(), 1/shape)
	}

	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9*d)

	for {
		var x, v float64
		for {
			x = o.rng.NormFloat64()
			v = 1 + c*x
			if v > 0 {
				break
			}
		}
		v = v * v * v
		u := o.rng.Float64()
		if u < 1-0.0331*(x*x)*(x*x) {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v
		}
	}
}

// autoOptimize performs automatic configuration optimization
func (o *MultiLLMOrchestrator) autoOptimize(ctx context.Context) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Calculate overall performance metrics
	var totalQuality, totalLatency float64
	var successCount, totalCount int64

	for _, perf := range o.models {
		if perf.TotalCalls > 0 {
			totalQuality += perf.QualitySum
			totalLatency += float64(perf.TotalLatencyMs)
			successCount += perf.SuccessfulCalls
			totalCount += perf.TotalCalls
		}
	}

	if totalCount < 100 {
		return // Not enough data for optimization
	}

	avgQuality := totalQuality / float64(totalCount)
	avgLatency := totalLatency / float64(totalCount)
	successRate := float64(successCount) / float64(totalCount)

	// Adjust exploration rate based on stability
	if avgQuality > 0.8 && successRate > 0.9 {
		// Good performance - reduce exploration
		o.config.ExplorationRate = math.Max(0.05, o.config.ExplorationRate*0.95)
	} else if avgQuality < 0.5 || successRate < 0.7 {
		// Poor performance - increase exploration
		o.config.ExplorationRate = math.Min(0.3, o.config.ExplorationRate*1.1)
	}

	// Adjust latency budget based on actual performance
	if avgLatency > 0 && float64(o.config.LatencyBudgetMs) < avgLatency*1.5 {
		o.config.LatencyBudgetMs = int64(avgLatency * 1.5)
	}

	// Update user profile
	if o.userProfile != nil {
		o.userProfile.InteractionCount = totalCount
		o.userProfile.LastOptimized = time.Now()
	}

	// Persist optimized config
	if o.store != nil {
		o.store.SaveEnsembleConfig(ctx, o.config)
		if o.userProfile != nil {
			o.store.SaveUserProfile(ctx, o.userProfile)
		}
	}
}

// GetStats returns current learning statistics
func (o *MultiLLMOrchestrator) GetStats() *LearningStats {
	o.mu.RLock()
	defer o.mu.RUnlock()

	stats := &LearningStats{
		TotalModels:      len(o.models),
		ModelPerformances: make([]ModelSummary, 0, len(o.models)),
		Config:           o.config,
		UserProfile:      o.userProfile,
	}

	for key, perf := range o.models {
		avgQuality := 0.0
		avgLatency := 0.0
		if perf.TotalCalls > 0 {
			avgQuality = perf.QualitySum / float64(perf.TotalCalls)
			avgLatency = float64(perf.TotalLatencyMs) / float64(perf.TotalCalls)
		}

		stats.ModelPerformances = append(stats.ModelPerformances, ModelSummary{
			Key:            key,
			Provider:       perf.Provider,
			Model:          perf.Model,
			TotalCalls:     perf.TotalCalls,
			SuccessRate:    float64(perf.SuccessfulCalls) / math.Max(1, float64(perf.TotalCalls)),
			AvgQuality:     avgQuality,
			AvgLatencyMs:   avgLatency,
			ThompsonAlpha:  perf.Alpha,
			ThompsonBeta:   perf.Beta,
			LastUsed:       perf.LastUsed,
		})
	}

	// Sort by Thompson score (alpha/(alpha+beta))
	sort.Slice(stats.ModelPerformances, func(i, j int) bool {
		scoreI := stats.ModelPerformances[i].ThompsonAlpha / 
			(stats.ModelPerformances[i].ThompsonAlpha + stats.ModelPerformances[i].ThompsonBeta)
		scoreJ := stats.ModelPerformances[j].ThompsonAlpha / 
			(stats.ModelPerformances[j].ThompsonAlpha + stats.ModelPerformances[j].ThompsonBeta)
		return scoreI > scoreJ
	})

	return stats
}

// LearningStats represents overall learning system statistics
type LearningStats struct {
	TotalModels       int             `json:"total_models"`
	ModelPerformances []ModelSummary  `json:"model_performances"`
	Config            EnsembleConfig  `json:"config"`
	UserProfile       *UserProfile    `json:"user_profile"`
}

// ModelSummary is a summary of a model's performance
type ModelSummary struct {
	Key           string    `json:"key"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	TotalCalls    int64     `json:"total_calls"`
	SuccessRate   float64   `json:"success_rate"`
	AvgQuality    float64   `json:"avg_quality"`
	AvgLatencyMs  float64   `json:"avg_latency_ms"`
	ThompsonAlpha float64   `json:"thompson_alpha"`
	ThompsonBeta  float64   `json:"thompson_beta"`
	LastUsed      time.Time `json:"last_used"`
}

// Export exports learning data to JSON for inspection
func (o *MultiLLMOrchestrator) Export() ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	data := struct {
		Models      map[string]*ModelPerformance `json:"models"`
		Config      EnsembleConfig               `json:"config"`
		UserProfile *UserProfile                 `json:"user_profile"`
		ExportedAt  time.Time                    `json:"exported_at"`
	}{
		Models:      o.models,
		Config:      o.config,
		UserProfile: o.userProfile,
		ExportedAt:  time.Now(),
	}

	return json.MarshalIndent(data, "", "  ")
}

// Import imports previously exported learning data
func (o *MultiLLMOrchestrator) Import(data []byte) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	var imported struct {
		Models      map[string]*ModelPerformance `json:"models"`
		Config      EnsembleConfig               `json:"config"`
		UserProfile *UserProfile                 `json:"user_profile"`
	}

	if err := json.Unmarshal(data, &imported); err != nil {
		return fmt.Errorf("failed to unmarshal learning data: %w", err)
	}

	// Merge models (keep better-performing version if conflict)
	for key, perf := range imported.Models {
		existing, exists := o.models[key]
		if !exists || perf.TotalCalls > existing.TotalCalls {
			o.models[key] = perf
		}
	}

	// Update config if import has data
	if imported.Config.Strategy != "" {
		o.config = imported.Config
	}

	// Update user profile
	if imported.UserProfile != nil {
		o.userProfile = imported.UserProfile
	}

	return nil
}

// UpdateUserPreference updates a user preference based on feedback
func (o *MultiLLMOrchestrator) UpdateUserPreference(ctx context.Context, prefType string, value float64) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.userProfile == nil {
		o.userProfile = &UserProfile{}
	}

	value = clamp01(value)

	switch prefType {
	case "quality":
		o.userProfile.QualitySensitivity = value
	case "latency":
		o.userProfile.LatencySensitivity = value
	case "cost":
		o.userProfile.CostSensitivity = value
	}

	if o.store != nil {
		o.store.SaveUserProfile(ctx, o.userProfile)
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
