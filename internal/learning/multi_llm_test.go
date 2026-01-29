package learning

import (
	"context"
	"testing"
	"time"
)

func TestMultiLLMOrchestrator_SelectModels(t *testing.T) {
	store := NewInMemoryStore()
	config := DefaultEnsembleConfig()
	orch := NewMultiLLMOrchestrator(store, config)

	req := SelectionRequest{
		TaskType:   "code_generation",
		Complexity: 0.5,
		Candidates: []string{
			"anthropic:claude-sonnet-4-20250514",
			"openai:gpt-4o",
			"ollama:llama3.2:7b",
		},
	}

	result, err := orch.SelectModels(context.Background(), req)
	if err != nil {
		t.Fatalf("SelectModels failed: %v", err)
	}

	if len(result.SelectedModels) == 0 {
		t.Error("Expected at least one model to be selected")
	}

	t.Logf("Selected models: %v", result.SelectedModels)
	t.Logf("Strategy: %s", result.Strategy)
	t.Logf("Confidence: %.2f", result.Confidence)
}

func TestMultiLLMOrchestrator_RecordOutcome(t *testing.T) {
	store := NewInMemoryStore()
	config := DefaultEnsembleConfig()
	orch := NewMultiLLMOrchestrator(store, config)

	outcomes := []ModelOutcome{
		{Provider: "anthropic", Model: "claude", TaskType: "chat", Success: true, Quality: 0.9, LatencyMs: 2000},
		{Provider: "anthropic", Model: "claude", TaskType: "chat", Success: true, Quality: 0.85, LatencyMs: 1800},
		{Provider: "openai", Model: "gpt4", TaskType: "chat", Success: true, Quality: 0.8, LatencyMs: 2500},
	}

	for _, outcome := range outcomes {
		if err := orch.RecordOutcome(context.Background(), &outcome); err != nil {
			t.Fatalf("RecordOutcome failed: %v", err)
		}
	}

	stats := orch.GetStats()
	if stats.TotalModels != 2 {
		t.Errorf("Expected 2 models, got %d", stats.TotalModels)
	}
}

func TestEnsembleConfig_Default(t *testing.T) {
	config := DefaultEnsembleConfig()

	if config.Strategy != StrategyAdaptive {
		t.Errorf("Expected adaptive strategy, got %s", config.Strategy)
	}

	if config.MaxParallelModels != 3 {
		t.Errorf("Expected 3 max parallel models, got %d", config.MaxParallelModels)
	}

	if config.ExplorationRate != 0.15 {
		t.Errorf("Expected 0.15 exploration rate, got %.2f", config.ExplorationRate)
	}
}

func TestUserProfile_Update(t *testing.T) {
	store := NewInMemoryStore()
	config := DefaultEnsembleConfig()
	orch := NewMultiLLMOrchestrator(store, config)

	orch.UpdateUserPreference(context.Background(), "quality", 0.8)
	orch.UpdateUserPreference(context.Background(), "latency", 0.1)

	stats := orch.GetStats()
	if stats.UserProfile == nil {
		t.Fatal("UserProfile is nil")
	}

	if stats.UserProfile.QualitySensitivity != 0.8 {
		t.Errorf("Expected quality sensitivity 0.8, got %.2f", stats.UserProfile.QualitySensitivity)
	}
}

func TestExportImport(t *testing.T) {
	store := NewInMemoryStore()
	config := DefaultEnsembleConfig()
	orch := NewMultiLLMOrchestrator(store, config)

	orch.RecordOutcome(context.Background(), &ModelOutcome{
		Provider: "anthropic", Model: "claude", TaskType: "chat", Success: true, Quality: 0.9,
	})

	data, err := orch.Export()
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	orch2 := NewMultiLLMOrchestrator(NewInMemoryStore(), DefaultEnsembleConfig())
	if err := orch2.Import(data); err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	stats := orch2.GetStats()
	if stats.TotalModels != 1 {
		t.Errorf("Expected 1 model after import, got %d", stats.TotalModels)
	}
}

func TestInMemoryStore_Basic(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	perf := &ModelPerformance{
		Provider:        "test",
		Model:           "model",
		TotalCalls:      10,
		SuccessfulCalls: 9,
		Alpha:           10,
		Beta:            2,
		LastUsed:        time.Now(),
		TaskScores:      make(map[string]*TaskScore),
	}

	if err := store.SaveModelPerformance(ctx, perf); err != nil {
		t.Fatalf("SaveModelPerformance failed: %v", err)
	}

	loaded, err := store.LoadModelPerformance(ctx, "test", "model")
	if err != nil {
		t.Fatalf("LoadModelPerformance failed: %v", err)
	}

	if loaded == nil {
		t.Fatal("Loaded performance is nil")
	}

	if loaded.TotalCalls != 10 {
		t.Errorf("Expected 10 calls, got %d", loaded.TotalCalls)
	}

	all, err := store.ListAllModels(ctx)
	if err != nil {
		t.Fatalf("ListAllModels failed: %v", err)
	}

	if len(all) != 1 {
		t.Errorf("Expected 1 model, got %d", len(all))
	}
}

func TestInMemoryStore_UserProfile(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	profile := &UserProfile{
		UserID:             "user1",
		QualitySensitivity: 0.7,
	}

	if err := store.SaveUserProfile(ctx, profile); err != nil {
		t.Fatalf("SaveUserProfile failed: %v", err)
	}

	loadedProfile, err := store.LoadUserProfile(ctx, "user1")
	if err != nil {
		t.Fatalf("LoadUserProfile failed: %v", err)
	}

	if loadedProfile == nil {
		t.Fatal("Loaded profile is nil")
	}

	if loadedProfile.QualitySensitivity != 0.7 {
		t.Errorf("Expected 0.7, got %.2f", loadedProfile.QualitySensitivity)
	}
}

func TestInMemoryStore_EnsembleConfig(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	cfg := DefaultEnsembleConfig()
	cfg.MaxParallelModels = 5

	if err := store.SaveEnsembleConfig(ctx, cfg); err != nil {
		t.Fatalf("SaveEnsembleConfig failed: %v", err)
	}

	loadedCfg, err := store.LoadEnsembleConfig(ctx)
	if err != nil {
		t.Fatalf("LoadEnsembleConfig failed: %v", err)
	}

	if loadedCfg.MaxParallelModels != 5 {
		t.Errorf("Expected 5, got %d", loadedCfg.MaxParallelModels)
	}
}
