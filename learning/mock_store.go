package learning

import (
	"context"
	"fmt"
	"sync"
	"time"

	"devorch/internal/okaon"
)

// MockStore is an in-memory implementation of OkAONStore for testing/demo
type MockStore struct {
	mu    sync.RWMutex
	stats map[string]okaon.ArmStat
}

func NewMockStore() *MockStore {
	return &MockStore{
		stats: make(map[string]okaon.ArmStat),
	}
}

// GetArmStats retrieves arm statistics
func (m *MockStore) GetArmStats(ctx context.Context, fingerprint, category, provider, model string) (okaon.ArmStat, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%s:%s:%s:%s", fingerprint, category, provider, model)
	stat, ok := m.stats[key]
	return stat, ok, nil
}

// UpsertArmStats updates or inserts arm statistics
func (m *MockStore) UpsertArmStats(ctx context.Context, u okaon.ArmStatUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%s:%s:%s", u.Fingerprint, u.Category, u.Provider, u.Model)

	existing, ok := m.stats[key]
	if !ok {
		// New arm - initialize with default values
		existing = okaon.ArmStat{
			Fingerprint:  u.Fingerprint,
			Category:     u.Category,
			Provider:     u.Provider,
			Model:        u.Model,
			CountRuns:    0,
			MeanReward01: 0.5, // Start with neutral assumption
			UpdatedTsUTC: time.Now().Unix(),
		}
	}

	// Update statistics (simple averaging for demo)
	if u.DeltaCount > 0 {
		totalRuns := existing.CountRuns + u.DeltaCount
		existing.MeanReward01 = (existing.MeanReward01*float64(existing.CountRuns) + u.DeltaSumReward) / float64(totalRuns)
		existing.CountRuns = totalRuns
		existing.UpdatedTsUTC = time.Now().Unix()
	}

	m.stats[key] = existing
	return nil
}

// InsertWork inserts a work record (mock implementation)
func (m *MockStore) InsertWork(ctx context.Context, w okaon.Work) error {
	return nil
}

// InsertRun inserts a run record (mock implementation)
func (m *MockStore) InsertRun(ctx context.Context, r okaon.Run) error {
	return nil
}

// InsertQuality inserts a quality record (mock implementation)
func (m *MockStore) InsertQuality(ctx context.Context, q okaon.Quality) error {
	return nil
}

// InsertReward inserts a reward record (mock implementation)
func (m *MockStore) InsertReward(ctx context.Context, r okaon.Reward) error {
	return nil
}

// GetModelAggregate returns model aggregates (mock implementation)
func (m *MockStore) GetModelAggregate(ctx context.Context, since int64) ([]okaon.ModelAggregate, error) {
	return nil, nil
}

// GetCategoryAggregate returns category aggregates (mock implementation)
func (m *MockStore) GetCategoryAggregate(ctx context.Context, since int64) ([]okaon.CategoryAggregate, error) {
	return nil, nil
}
