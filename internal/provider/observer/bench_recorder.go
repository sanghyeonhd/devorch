package observer

import (
	"context"
	"time"

	"devorch/internal/learning"
)

// BenchRecorderObserver: ProviderCall -> learning.Recorder로 기록
type BenchRecorderObserver struct {
	Recorder *learning.Recorder
}

func NewBenchRecorderObserver(r *learning.Recorder) *BenchRecorderObserver {
	return &BenchRecorderObserver{Recorder: r}
}

func (o *BenchRecorderObserver) OnComplete(ctx context.Context, call ProviderCall) error {
	if o == nil || o.Recorder == nil {
		return nil
	}
	created := call.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}

	m := learning.CompletionMetrics{
		CreatedAt: created,

		OS:                 call.OS,
		Arch:               call.Arch,
		MachineFingerprint: call.MachineFingerprint,

		Provider: call.Provider,
		Model:    call.Model,
		Scenario: call.Scenario,

		InputTokens:  call.InputTokens,
		OutputTokens: call.OutputTokens,
		LatencyMs:    call.LatencyMs,

		Success: call.Success,
		Error:   call.Error,

		QualityScore: call.QualityScore,
		CostUSD:      call.CostUSD,

		Metadata: call.Metadata,
	}
	return o.Recorder.RecordCompletion(ctx, m)
}
