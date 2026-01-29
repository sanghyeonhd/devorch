package bench

import (
	"context"
	"errors"
	"time"

	"devorch/internal/id"
	"devorch/internal/storage/sqlite"
)

type ProviderProbe interface {
	// "아주 짧은 작업"으로 RTT/응답성을 측정하는 최소 단위
	// Step 1~8 provider abstraction과 연결하세요.
	Probe(ctx context.Context, provider string, model string) (latency time.Duration, inTok, outTok int64, costUSD *float64, err error)
}

type MachineInfo interface {
	OS() string
	Arch() string
	Fingerprint() string
}

type RunnerImpl struct {
	probe   ProviderProbe
	store   *sqlite.BenchmarksStore
	machine MachineInfo

	// 벤치할 대상 (local+remote)
	Targets []Target
}

type Target struct {
	Provider string
	Model    string
	Scenario string // "quick"
}

func NewRunner(probe ProviderProbe, store *sqlite.BenchmarksStore, machine MachineInfo) *RunnerImpl {
	return &RunnerImpl{
		probe:   probe,
		store:   store,
		machine: machine,
		Targets: []Target{},
	}
}

func (r *RunnerImpl) RunQuick(ctx context.Context) (QuickSummary, error) {
	if r.probe == nil {
		return QuickSummary{}, errors.New("bench: probe is nil")
	}
	if r.machine == nil {
		return QuickSummary{}, errors.New("bench: machine info is nil")
	}
	if len(r.Targets) == 0 {
		// 기본값: 최소 2개(로컬/원격) 정도만.
		r.Targets = []Target{
			{Provider: "ollama", Model: "llama3.1:8b", Scenario: "quick"},
			{Provider: "openrouter", Model: "openrouter/auto", Scenario: "quick"},
		}
	}

	now := time.Now()
	items := make([]Item, 0, len(r.Targets))

	for _, t := range r.Targets {
		lat, inTok, outTok, cost, err := r.probe.Probe(ctx, t.Provider, t.Model)
		it := Item{
			Provider:  t.Provider,
			Model:     t.Model,
			Scenario:  t.Scenario,
			LatencyMs: lat.Milliseconds(),
			Success:   err == nil,
			InputTok:  inTok,
			OutputTok: outTok,
		}
		if err != nil {
			it.Error = err.Error()
		}
		if cost != nil {
			it.CostUSD = cost
		}
		items = append(items, it)

		// DB에 저장 (store가 없으면 skip)
		if r.store != nil {
			_ = r.store.Insert(ctx, sqlite.BenchmarkRow{
				ID:                 id.NewULID(),
				CreatedAt:          now,
				OS:                 r.machine.OS(),
				Arch:               r.machine.Arch(),
				MachineFingerprint: r.machine.Fingerprint(),
				Provider:           t.Provider,
				Model:              t.Model,
				Scenario:           t.Scenario,
				InputTokens:        inTok,
				OutputTokens:       outTok,
				LatencyMs:          lat.Milliseconds(),
				Success:            err == nil,
				Error:              errStr(err),
				CostUSD:            cost,
				Metadata: map[string]any{
					"type": "quick_probe",
				},
			})
		}
	}

	return QuickSummary{CreatedAt: now, Items: items}, nil
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
