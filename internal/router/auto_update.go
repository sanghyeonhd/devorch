package router

import (
	"context"
	"log"
	"time"
)

// AutoUpdater: 주기적으로 bench 데이터를 집계 → policy 갱신
type AutoUpdater struct {
	Interval time.Duration
	StopCh   chan struct{}

	// 외부에서 주입
	FetchFeatures func(ctx context.Context) ([]Feature, error)
	UpdatePolicy  func(ctx context.Context, aggs []Aggregate) error
}

type Feature struct {
	ScopeType string
	ScopeID   string

	OS       string
	Arch     string
	Scenario string

	Provider string
	Model    string

	LatencyMs    int64
	Success      bool
	InputTokens  int64
	OutputTokens int64
	Cost         float64
}

type Aggregate struct {
	ScopeType string
	ScopeID   string

	OS       string
	Arch     string
	Scenario string

	Provider string
	Model    string

	SampleCount  int
	SuccessCount int
	AvgLatency   float64
	AvgCost      float64
}

func NewAutoUpdater(interval time.Duration) *AutoUpdater {
	return &AutoUpdater{
		Interval: interval,
		StopCh:   make(chan struct{}),
	}
}

func (u *AutoUpdater) Start(ctx context.Context) {
	ticker := time.NewTicker(u.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-u.StopCh:
			return
		case <-ticker.C:
			u.runOnce(ctx)
		}
	}
}

func (u *AutoUpdater) Stop() {
	close(u.StopCh)
}

func (u *AutoUpdater) runOnce(ctx context.Context) {
	if u.FetchFeatures == nil || u.UpdatePolicy == nil {
		return
	}

	features, err := u.FetchFeatures(ctx)
	if err != nil {
		log.Printf("AutoUpdater: fetch features failed: %v", err)
		return
	}
	if len(features) == 0 {
		return
	}

	aggs := aggregateFeatures(features)
	if err := u.UpdatePolicy(ctx, aggs); err != nil {
		log.Printf("AutoUpdater: update policy failed: %v", err)
	}
}

func aggregateFeatures(features []Feature) []Aggregate {
	type key struct {
		st, sid, os, arch, scenario, provider, model string
	}
	m := make(map[key]*Aggregate)

	for _, f := range features {
		k := key{f.ScopeType, f.ScopeID, f.OS, f.Arch, f.Scenario, f.Provider, f.Model}
		agg, ok := m[k]
		if !ok {
			agg = &Aggregate{
				ScopeType: f.ScopeType,
				ScopeID:   f.ScopeID,
				OS:        f.OS,
				Arch:      f.Arch,
				Scenario:  f.Scenario,
				Provider:  f.Provider,
				Model:     f.Model,
			}
			m[k] = agg
		}
		agg.SampleCount++
		if f.Success {
			agg.SuccessCount++
		}
		agg.AvgLatency += float64(f.LatencyMs)
		agg.AvgCost += f.Cost
	}

	out := make([]Aggregate, 0, len(m))
	for _, agg := range m {
		if agg.SampleCount > 0 {
			agg.AvgLatency /= float64(agg.SampleCount)
			agg.AvgCost /= float64(agg.SampleCount)
		}
		out = append(out, *agg)
	}
	return out
}
