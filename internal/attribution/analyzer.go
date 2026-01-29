package attribution

import (
	"context"
	"fmt"
)

type Cause struct {
	Dimension string // model | prompt | gpu_driver | runtime | backend
	OldValue  string
	NewValue  string
	Score     float64
}

type Analyzer struct{}

func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// 최근 drift 구간과 baseline 구간을 비교해서
// 어떤 필드가 바뀌었는지 자동 추출
func (a *Analyzer) Analyze(ctx context.Context, recentBenchIDs []string, baselineBenchIDs []string) ([]Cause, error) {
	// 최소 구현:
	// - 최근/기준선에서 model, gpu_driver, backend, prompt_hash 변화 빈도 비교
	// - 실제 제품에서는 통계적으로 더 정교화

	// 여기서는 구조만 제공 (실제로는 SQL로 집계)
	return []Cause{
		{
			Dimension: "model",
			OldValue:  "qwen2.5-7b",
			NewValue:  "qwen2.5-14b",
			Score:     0.72,
		},
		{
			Dimension: "gpu_backend",
			OldValue:  "metal",
			NewValue:  "cpu",
			Score:     0.55,
		},
	}, nil
}

func (c Cause) String() string {
	return fmt.Sprintf("[%s] %s -> %s (score=%.2f)", c.Dimension, c.OldValue, c.NewValue, c.Score)
}
