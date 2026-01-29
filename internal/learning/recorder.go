package learning

import (
	"context"
	"time"

	"devorch/internal/id"
)

// BenchmarksStore: 벤치마크 저장 인터페이스 (sqlite 의존성 제거)
type BenchmarksStore interface {
	Insert(ctx context.Context, row BenchmarkRow) error
}

// BenchmarkRow: 벤치마크 데이터 행
type BenchmarkRow struct {
	ID                 string
	CreatedAt          time.Time
	OS                 string
	Arch               string
	MachineFingerprint string
	Provider           string
	Model              string
	Scenario           string
	InputTokens        int64
	OutputTokens       int64
	LatencyMs          int64
	Success            bool
	Error              string
	QualityScore       *float64
	CostUSD            *float64
	Metadata           map[string]any
}

type Recorder struct {
	Store BenchmarksStore
}

func NewRecorder(store BenchmarksStore) *Recorder {
	return &Recorder{Store: store}
}

// RecordCompletion: 작업 완료 메트릭을 benchmarks 테이블로 적재
func (r *Recorder) RecordCompletion(ctx context.Context, m CompletionMetrics) error {
	if r == nil || r.Store == nil {
		return nil // store가 없으면 학습 비활성
	}

	row := BenchmarkRow{
		ID:                 id.NewULID(),
		CreatedAt:          m.CreatedAt,
		OS:                 m.OS,
		Arch:               m.Arch,
		MachineFingerprint: m.MachineFingerprint,
		Provider:           m.Provider,
		Model:              m.Model,
		Scenario:           m.Scenario,
		InputTokens:        m.InputTokens,
		OutputTokens:       m.OutputTokens,
		LatencyMs:          m.LatencyMs,
		Success:            m.Success,
		Error:              m.Error,
		QualityScore:       m.QualityScore,
		CostUSD:            m.CostUSD,
		Metadata:           m.Metadata,
	}
	return r.Store.Insert(ctx, row)
}
