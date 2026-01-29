// Package router provides reliability tracking for providers.
// Phase 32: Provider reliability tracking
package router

import (
	"sync"
	"time"
)

// ReliabilityTracker tracks success/failure rates for providers.
type ReliabilityTracker struct {
	mu    sync.RWMutex
	stats map[string]*reliabilityStat // key = provider/model
}

type reliabilityStat struct {
	ok   int64
	fail int64
	last time.Time
}

// NewReliabilityTracker creates a new reliability tracker.
func NewReliabilityTracker() *ReliabilityTracker {
	return &ReliabilityTracker{
		stats: make(map[string]*reliabilityStat),
	}
}

func reliabilityKey(provider, model string) string {
	return provider + "/" + model
}

// Observe records an observation (success or failure).
func (t *ReliabilityTracker) Observe(provider, model string, success bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	k := reliabilityKey(provider, model)
	s, ok := t.stats[k]
	if !ok {
		s = &reliabilityStat{}
		t.stats[k] = s
	}
	if success {
		s.ok++
	} else {
		s.fail++
	}
	s.last = time.Now()
}

// Reliability returns the success rate for a provider/model (0..1).
// Returns 0.7 for unknown providers.
func (t *ReliabilityTracker) Reliability(provider, model string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.stats[reliabilityKey(provider, model)]
	if !ok {
		return 0.7 // Unknown default
	}
	total := float64(s.ok + s.fail)
	if total == 0 {
		return 0.7
	}
	return float64(s.ok) / total
}

// ReliabilityWithDecay returns reliability with time-based decay.
// Recent failures have more impact than old failures.
func (t *ReliabilityTracker) ReliabilityWithDecay(provider, model string, decayDuration time.Duration) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.stats[reliabilityKey(provider, model)]
	if !ok {
		return 0.7 // Unknown default
	}

	total := float64(s.ok + s.fail)
	if total == 0 {
		return 0.7
	}

	baseReliability := float64(s.ok) / total

	// Apply decay if data is old
	if decayDuration > 0 && time.Since(s.last) > decayDuration {
		// Decay towards 0.7 (unknown)
		decayFactor := float64(time.Since(s.last)) / float64(decayDuration*10)
		if decayFactor > 1 {
			decayFactor = 1
		}
		baseReliability = baseReliability*(1-decayFactor) + 0.7*decayFactor
	}

	return baseReliability
}

// Reset clears all tracked statistics.
func (t *ReliabilityTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats = make(map[string]*reliabilityStat)
}

// Stats returns a copy of all statistics (for diagnostics).
func (t *ReliabilityTracker) Stats() map[string]ReliabilityStat {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]ReliabilityStat, len(t.stats))
	for k, v := range t.stats {
		result[k] = ReliabilityStat{
			OK:   v.ok,
			Fail: v.fail,
			Last: v.last,
		}
	}
	return result
}

// ReliabilityStat is an exported version of reliability statistics.
type ReliabilityStat struct {
	OK   int64
	Fail int64
	Last time.Time
}
