package pool

import (
	"sync"
	"time"
)

type Key struct {
	Provider string
	Model    string
}

type Pool struct {
	mu sync.RWMutex

	// (provider, model) -> reliability
	rel map[Key]*Reliability

	halfLife time.Duration
}

func NewPool(halfLife time.Duration) *Pool {
	if halfLife <= 0 {
		halfLife = 24 * time.Hour
	}
	return &Pool{
		rel:      make(map[Key]*Reliability),
		halfLife: halfLife,
	}
}

func (p *Pool) Observe(provider, model string, success bool, latencyMs int64) {
	if provider == "" || model == "" {
		return
	}
	k := Key{Provider: provider, Model: model}

	p.mu.Lock()
	r := p.rel[k]
	if r == nil {
		r = NewReliability(0.2)
		p.rel[k] = r
	}
	p.mu.Unlock()

	s := 0.0
	if success {
		s = 1.0
	}
	r.Observe(s, float64(latencyMs))
}

type ReliabilitySnapshot struct {
	SuccessEMA float64
	LatencyEMA float64
	Decay      float64
	Last       time.Time
}

func (p *Pool) Snapshot(provider, model string) ReliabilitySnapshot {
	k := Key{Provider: provider, Model: model}

	p.mu.RLock()
	r := p.rel[k]
	p.mu.RUnlock()
	if r == nil {
		return ReliabilitySnapshot{}
	}

	succ, lat, last := r.Snapshot()
	return ReliabilitySnapshot{
		SuccessEMA: succ,
		LatencyEMA: lat,
		Decay:      DecayFactor(last, p.halfLife),
		Last:       last,
	}
}
