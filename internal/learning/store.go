// Package learning implements persistence for multi-LLM learning state
package learning

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SQLiteStore implements PerformanceStore using SQLite
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite-based performance store
func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	store := &SQLiteStore{db: db}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}
	return store, nil
}

// initSchema creates the required tables
func (s *SQLiteStore) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS model_performance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			total_calls INTEGER DEFAULT 0,
			successful_calls INTEGER DEFAULT 0,
			failed_calls INTEGER DEFAULT 0,
			total_latency_ms INTEGER DEFAULT 0,
			total_tokens INTEGER DEFAULT 0,
			total_cost_micro INTEGER DEFAULT 0,
			quality_sum REAL DEFAULT 0,
			alpha REAL DEFAULT 1.0,
			beta REAL DEFAULT 1.0,
			task_scores_json TEXT DEFAULT '{}',
			last_used TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(provider, model)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_model_performance_provider_model 
		ON model_performance(provider, model)`,

		`CREATE TABLE IF NOT EXISTS user_profile (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL UNIQUE,
			preferred_models_json TEXT DEFAULT '[]',
			task_preferences_json TEXT DEFAULT '{}',
			quality_sensitivity REAL DEFAULT 0.5,
			latency_sensitivity REAL DEFAULT 0.3,
			cost_sensitivity REAL DEFAULT 0.2,
			interaction_count INTEGER DEFAULT 0,
			last_optimized TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS ensemble_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			strategy TEXT NOT NULL,
			max_parallel_models INTEGER DEFAULT 3,
			min_consensus REAL DEFAULT 0.7,
			quality_threshold REAL DEFAULT 0.6,
			latency_budget_ms INTEGER DEFAULT 30000,
			exploration_rate REAL DEFAULT 0.15,
			enable_auto_optimize INTEGER DEFAULT 1,
			updated_at TEXT NOT NULL
		)`,

		`CREATE TABLE IF NOT EXISTS learning_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			task_type TEXT,
			success INTEGER NOT NULL,
			quality REAL,
			latency_ms INTEGER,
			tokens INTEGER,
			cost_micro INTEGER,
			reward REAL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_history_provider_model 
		ON learning_history(provider, model)`,
		`CREATE INDEX IF NOT EXISTS idx_learning_history_created_at 
		ON learning_history(created_at)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("failed to execute: %s, error: %w", q, err)
		}
	}

	return nil
}

// SaveModelPerformance saves or updates model performance
func (s *SQLiteStore) SaveModelPerformance(ctx context.Context, perf *ModelPerformance) error {
	taskScoresJSON, err := json.Marshal(perf.TaskScores)
	if err != nil {
		return fmt.Errorf("failed to marshal task scores: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	lastUsed := ""
	if !perf.LastUsed.IsZero() {
		lastUsed = perf.LastUsed.UTC().Format(time.RFC3339)
	}

	query := `
		INSERT INTO model_performance (
			provider, model, total_calls, successful_calls, failed_calls,
			total_latency_ms, total_tokens, total_cost_micro, quality_sum,
			alpha, beta, task_scores_json, last_used, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, model) DO UPDATE SET
			total_calls = excluded.total_calls,
			successful_calls = excluded.successful_calls,
			failed_calls = excluded.failed_calls,
			total_latency_ms = excluded.total_latency_ms,
			total_tokens = excluded.total_tokens,
			total_cost_micro = excluded.total_cost_micro,
			quality_sum = excluded.quality_sum,
			alpha = excluded.alpha,
			beta = excluded.beta,
			task_scores_json = excluded.task_scores_json,
			last_used = excluded.last_used,
			updated_at = excluded.updated_at
	`

	_, err = s.db.ExecContext(ctx, query,
		perf.Provider, perf.Model, perf.TotalCalls, perf.SuccessfulCalls, perf.FailedCalls,
		perf.TotalLatencyMs, perf.TotalTokens, perf.TotalCostMicro, perf.QualitySum,
		perf.Alpha, perf.Beta, string(taskScoresJSON), lastUsed, now, now,
	)

	return err
}

// LoadModelPerformance loads model performance by provider and model
func (s *SQLiteStore) LoadModelPerformance(ctx context.Context, provider, model string) (*ModelPerformance, error) {
	query := `
		SELECT provider, model, total_calls, successful_calls, failed_calls,
			total_latency_ms, total_tokens, total_cost_micro, quality_sum,
			alpha, beta, task_scores_json, last_used
		FROM model_performance
		WHERE provider = ? AND model = ?
	`

	var perf ModelPerformance
	var taskScoresJSON string
	var lastUsed sql.NullString

	err := s.db.QueryRowContext(ctx, query, provider, model).Scan(
		&perf.Provider, &perf.Model, &perf.TotalCalls, &perf.SuccessfulCalls, &perf.FailedCalls,
		&perf.TotalLatencyMs, &perf.TotalTokens, &perf.TotalCostMicro, &perf.QualitySum,
		&perf.Alpha, &perf.Beta, &taskScoresJSON, &lastUsed,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load model performance: %w", err)
	}

	// Parse task scores
	perf.TaskScores = make(map[string]*TaskScore)
	if taskScoresJSON != "" {
		if err := json.Unmarshal([]byte(taskScoresJSON), &perf.TaskScores); err != nil {
			// Log error but don't fail
			fmt.Printf("Warning: failed to parse task scores: %v\n", err)
		}
	}

	// Parse last used
	if lastUsed.Valid && lastUsed.String != "" {
		if t, err := time.Parse(time.RFC3339, lastUsed.String); err == nil {
			perf.LastUsed = t
		}
	}

	return &perf, nil
}

// ListAllModels lists all model performances
func (s *SQLiteStore) ListAllModels(ctx context.Context) ([]*ModelPerformance, error) {
	query := `
		SELECT provider, model, total_calls, successful_calls, failed_calls,
			total_latency_ms, total_tokens, total_cost_micro, quality_sum,
			alpha, beta, task_scores_json, last_used
		FROM model_performance
		ORDER BY total_calls DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer rows.Close()

	var models []*ModelPerformance
	for rows.Next() {
		var perf ModelPerformance
		var taskScoresJSON string
		var lastUsed sql.NullString

		err := rows.Scan(
			&perf.Provider, &perf.Model, &perf.TotalCalls, &perf.SuccessfulCalls, &perf.FailedCalls,
			&perf.TotalLatencyMs, &perf.TotalTokens, &perf.TotalCostMicro, &perf.QualitySum,
			&perf.Alpha, &perf.Beta, &taskScoresJSON, &lastUsed,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		perf.TaskScores = make(map[string]*TaskScore)
		if taskScoresJSON != "" {
			json.Unmarshal([]byte(taskScoresJSON), &perf.TaskScores)
		}

		if lastUsed.Valid && lastUsed.String != "" {
			if t, err := time.Parse(time.RFC3339, lastUsed.String); err == nil {
				perf.LastUsed = t
			}
		}

		models = append(models, &perf)
	}

	return models, nil
}

// SaveUserProfile saves user profile
func (s *SQLiteStore) SaveUserProfile(ctx context.Context, profile *UserProfile) error {
	preferredModelsJSON, err := json.Marshal(profile.PreferredModels)
	if err != nil {
		return err
	}
	taskPreferencesJSON, err := json.Marshal(profile.TaskPreferences)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	lastOptimized := ""
	if !profile.LastOptimized.IsZero() {
		lastOptimized = profile.LastOptimized.UTC().Format(time.RFC3339)
	}

	query := `
		INSERT INTO user_profile (
			user_id, preferred_models_json, task_preferences_json,
			quality_sensitivity, latency_sensitivity, cost_sensitivity,
			interaction_count, last_optimized, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			preferred_models_json = excluded.preferred_models_json,
			task_preferences_json = excluded.task_preferences_json,
			quality_sensitivity = excluded.quality_sensitivity,
			latency_sensitivity = excluded.latency_sensitivity,
			cost_sensitivity = excluded.cost_sensitivity,
			interaction_count = excluded.interaction_count,
			last_optimized = excluded.last_optimized,
			updated_at = excluded.updated_at
	`

	_, err = s.db.ExecContext(ctx, query,
		profile.UserID, string(preferredModelsJSON), string(taskPreferencesJSON),
		profile.QualitySensitivity, profile.LatencySensitivity, profile.CostSensitivity,
		profile.InteractionCount, lastOptimized, now, now,
	)

	return err
}

// LoadUserProfile loads user profile
func (s *SQLiteStore) LoadUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	query := `
		SELECT user_id, preferred_models_json, task_preferences_json,
			quality_sensitivity, latency_sensitivity, cost_sensitivity,
			interaction_count, last_optimized
		FROM user_profile
		WHERE user_id = ?
	`

	var profile UserProfile
	var preferredModelsJSON, taskPreferencesJSON string
	var lastOptimized sql.NullString

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&profile.UserID, &preferredModelsJSON, &taskPreferencesJSON,
		&profile.QualitySensitivity, &profile.LatencySensitivity, &profile.CostSensitivity,
		&profile.InteractionCount, &lastOptimized,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load user profile: %w", err)
	}

	json.Unmarshal([]byte(preferredModelsJSON), &profile.PreferredModels)
	json.Unmarshal([]byte(taskPreferencesJSON), &profile.TaskPreferences)

	if lastOptimized.Valid && lastOptimized.String != "" {
		if t, err := time.Parse(time.RFC3339, lastOptimized.String); err == nil {
			profile.LastOptimized = t
		}
	}

	return &profile, nil
}

// SaveEnsembleConfig saves ensemble configuration
func (s *SQLiteStore) SaveEnsembleConfig(ctx context.Context, config EnsembleConfig) error {
	now := time.Now().UTC().Format(time.RFC3339)
	autoOptimize := 0
	if config.EnableAutoOptimize {
		autoOptimize = 1
	}

	query := `
		INSERT INTO ensemble_config (
			id, strategy, max_parallel_models, min_consensus,
			quality_threshold, latency_budget_ms, exploration_rate,
			enable_auto_optimize, updated_at
		) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			strategy = excluded.strategy,
			max_parallel_models = excluded.max_parallel_models,
			min_consensus = excluded.min_consensus,
			quality_threshold = excluded.quality_threshold,
			latency_budget_ms = excluded.latency_budget_ms,
			exploration_rate = excluded.exploration_rate,
			enable_auto_optimize = excluded.enable_auto_optimize,
			updated_at = excluded.updated_at
	`

	_, err := s.db.ExecContext(ctx, query,
		string(config.Strategy), config.MaxParallelModels, config.MinConsensus,
		config.QualityThreshold, config.LatencyBudgetMs, config.ExplorationRate,
		autoOptimize, now,
	)

	return err
}

// LoadEnsembleConfig loads ensemble configuration
func (s *SQLiteStore) LoadEnsembleConfig(ctx context.Context) (EnsembleConfig, error) {
	query := `
		SELECT strategy, max_parallel_models, min_consensus,
			quality_threshold, latency_budget_ms, exploration_rate,
			enable_auto_optimize
		FROM ensemble_config
		WHERE id = 1
	`

	var config EnsembleConfig
	var strategy string
	var autoOptimize int

	err := s.db.QueryRowContext(ctx, query).Scan(
		&strategy, &config.MaxParallelModels, &config.MinConsensus,
		&config.QualityThreshold, &config.LatencyBudgetMs, &config.ExplorationRate,
		&autoOptimize,
	)

	if err == sql.ErrNoRows {
		return DefaultEnsembleConfig(), nil
	}
	if err != nil {
		return DefaultEnsembleConfig(), fmt.Errorf("failed to load config: %w", err)
	}

	config.Strategy = EnsembleStrategy(strategy)
	config.EnableAutoOptimize = autoOptimize == 1

	return config, nil
}

// RecordLearningHistory records a learning event for analysis
func (s *SQLiteStore) RecordLearningHistory(ctx context.Context, provider, model, taskType string,
	success bool, quality float64, latencyMs, tokens, costMicro int64, reward float64) error {

	now := time.Now().UTC().Format(time.RFC3339)
	successInt := 0
	if success {
		successInt = 1
	}

	query := `
		INSERT INTO learning_history (
			provider, model, task_type, success, quality,
			latency_ms, tokens, cost_micro, reward, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		provider, model, taskType, successInt, quality,
		latencyMs, tokens, costMicro, reward, now,
	)

	return err
}

// GetLearningStats gets aggregated learning statistics
func (s *SQLiteStore) GetLearningStats(ctx context.Context, days int) (*LearningHistoryStats, error) {
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)

	query := `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as successes,
			AVG(quality) as avg_quality,
			AVG(latency_ms) as avg_latency,
			SUM(tokens) as total_tokens,
			SUM(cost_micro) as total_cost,
			AVG(reward) as avg_reward
		FROM learning_history
		WHERE created_at >= ?
	`

	var stats LearningHistoryStats
	var avgQuality, avgLatency, avgReward sql.NullFloat64
	var totalTokens, totalCost sql.NullInt64

	err := s.db.QueryRowContext(ctx, query, cutoff).Scan(
		&stats.TotalCalls, &stats.SuccessfulCalls,
		&avgQuality, &avgLatency,
		&totalTokens, &totalCost, &avgReward,
	)

	if err != nil {
		return nil, err
	}

	if avgQuality.Valid {
		stats.AvgQuality = avgQuality.Float64
	}
	if avgLatency.Valid {
		stats.AvgLatencyMs = avgLatency.Float64
	}
	if totalTokens.Valid {
		stats.TotalTokens = totalTokens.Int64
	}
	if totalCost.Valid {
		stats.TotalCostMicro = totalCost.Int64
	}
	if avgReward.Valid {
		stats.AvgReward = avgReward.Float64
	}

	return &stats, nil
}

// LearningHistoryStats contains aggregated history statistics
type LearningHistoryStats struct {
	TotalCalls      int64   `json:"total_calls"`
	SuccessfulCalls int64   `json:"successful_calls"`
	AvgQuality      float64 `json:"avg_quality"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	TotalTokens     int64   `json:"total_tokens"`
	TotalCostMicro  int64   `json:"total_cost_micro"`
	AvgReward       float64 `json:"avg_reward"`
}

// CleanupOldHistory removes learning history older than days
func (s *SQLiteStore) CleanupOldHistory(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days).UTC().Format(time.RFC3339)

	result, err := s.db.ExecContext(ctx,
		"DELETE FROM learning_history WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// InMemoryStore implements PerformanceStore for testing/development
type InMemoryStore struct {
	models   map[string]*ModelPerformance
	profiles map[string]*UserProfile
	config   EnsembleConfig
}

// NewInMemoryStore creates a new in-memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		models:   make(map[string]*ModelPerformance),
		profiles: make(map[string]*UserProfile),
		config:   DefaultEnsembleConfig(),
	}
}

func (s *InMemoryStore) SaveModelPerformance(ctx context.Context, perf *ModelPerformance) error {
	key := ModelKey(perf.Provider, perf.Model)
	s.models[key] = perf
	return nil
}

func (s *InMemoryStore) LoadModelPerformance(ctx context.Context, provider, model string) (*ModelPerformance, error) {
	key := ModelKey(provider, model)
	return s.models[key], nil
}

func (s *InMemoryStore) ListAllModels(ctx context.Context) ([]*ModelPerformance, error) {
	result := make([]*ModelPerformance, 0, len(s.models))
	for _, p := range s.models {
		result = append(result, p)
	}
	return result, nil
}

func (s *InMemoryStore) SaveUserProfile(ctx context.Context, profile *UserProfile) error {
	s.profiles[profile.UserID] = profile
	return nil
}

func (s *InMemoryStore) LoadUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	return s.profiles[userID], nil
}

func (s *InMemoryStore) SaveEnsembleConfig(ctx context.Context, config EnsembleConfig) error {
	s.config = config
	return nil
}

func (s *InMemoryStore) LoadEnsembleConfig(ctx context.Context) (EnsembleConfig, error) {
	return s.config, nil
}
