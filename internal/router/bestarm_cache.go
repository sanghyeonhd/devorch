// Package router provides best-arm caching for cold-start acceleration.
// Phase 32: envfp × category based best-arm cache
package router

import (
	"context"
	"sync"
	"time"
)

// BestArm represents the best performing arm for a given context.
type BestArm struct {
	Provider string
	Model    string

	AvgReward  float64
	AvgQuality float64
	AvgLatency float64
	N          int64

	UpdatedAt time.Time
}

// BestArmCache caches best arm information keyed by envfp|category.
type BestArmCache struct {
	mu   sync.RWMutex
	ttl  time.Duration
	data map[string]BestArm // key = envfp|category
}

// NewBestArmCache creates a new best-arm cache with the given TTL.
func NewBestArmCache(ttl time.Duration) *BestArmCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &BestArmCache{
		ttl:  ttl,
		data: make(map[string]BestArm),
	}
}

func bestArmCacheKey(envfp, category string) string {
	return envfp + "|" + category
}

// Get retrieves a cached best arm if it exists and is not expired.
func (c *BestArmCache) Get(envfp, category string) (BestArm, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	k := bestArmCacheKey(envfp, category)
	v, ok := c.data[k]
	if !ok {
		return BestArm{}, false
	}
	if time.Since(v.UpdatedAt) > c.ttl {
		return BestArm{}, false
	}
	return v, true
}

// Put stores a best arm in the cache.
func (c *BestArmCache) Put(envfp, category string, v BestArm) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v.UpdatedAt = time.Now()
	c.data[bestArmCacheKey(envfp, category)] = v
}

// Invalidate removes a cached entry.
func (c *BestArmCache) Invalidate(envfp, category string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, bestArmCacheKey(envfp, category))
}

// Clear removes all cached entries.
func (c *BestArmCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]BestArm)
}

// BestArmLoader loads best arm data from storage.
type BestArmLoader interface {
	BestArmByEnvCategory(ctx context.Context, envfp, category string, limit int) ([]BestArmRow, error)
}

// BestArmRow represents a row from the best-arm query.
type BestArmRow struct {
	Provider     string
	Model        string
	AvgReward    float64
	AvgQuality   float64
	AvgLatencyMs float64
	N            int64
}

// Warm loads the best arm from storage if not cached.
func (c *BestArmCache) Warm(ctx context.Context, loader BestArmLoader, envfp, category string) (BestArm, bool) {
	if v, ok := c.Get(envfp, category); ok {
		return v, true
	}

	rows, err := loader.BestArmByEnvCategory(ctx, envfp, category, 1)
	if err != nil || len(rows) == 0 {
		return BestArm{}, false
	}

	r := rows[0]
	best := BestArm{
		Provider:   r.Provider,
		Model:      r.Model,
		AvgReward:  r.AvgReward,
		AvgQuality: r.AvgQuality,
		AvgLatency: r.AvgLatencyMs,
		N:          r.N,
		UpdatedAt:  time.Now(),
	}
	c.Put(envfp, category, best)
	return best, true
}
