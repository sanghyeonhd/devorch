package pool

import (
	"sync"
	"time"
)

// 모델/프로바이더별 "실측 신뢰도" (EMA)
type Reliability struct {
	mu sync.Mutex

	// 0..1
	SuccessEMA float64
	LatencyEMA float64 // ms (정규화 전 raw)
	LastTs     time.Time

	// EMA 계수(0..1)
	Alpha float64
}

func NewReliability(alpha float64) *Reliability {
	if alpha <= 0 || alpha >= 1 {
		alpha = 0.2
	}
	return &Reliability{Alpha: alpha}
}

// success: 1/0, latencyMs: 측정치
func (r *Reliability) Observe(success float64, latencyMs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.LastTs.IsZero() {
		r.SuccessEMA = success
		r.LatencyEMA = latencyMs
		r.LastTs = time.Now()
		return
	}

	a := r.Alpha
	r.SuccessEMA = a*success + (1-a)*r.SuccessEMA
	r.LatencyEMA = a*latencyMs + (1-a)*r.LatencyEMA
	r.LastTs = time.Now()
}

func (r *Reliability) Snapshot() (successEMA, latencyEMA float64, last time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.SuccessEMA, r.LatencyEMA, r.LastTs
}
