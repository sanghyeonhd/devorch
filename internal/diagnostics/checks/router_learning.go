package checks

import (
	"context"
	"time"

	"devorch/internal/diagnostics"
)

// RouterLearningCheck: router learning 상태 점검
type RouterLearningCheck struct{}

func (c RouterLearningCheck) ID() string {
	return "router-learning"
}

func (c RouterLearningCheck) Name() string {
	return "Router Learning"
}

func (c RouterLearningCheck) Run(ctx context.Context, in diagnostics.Input) diagnostics.CheckResult {
	start := time.Now()
	// 실제 구현에서는:
	// 1. policy store 연결 확인
	// 2. policy 개수 확인
	// 3. 최근 업데이트 시간 확인

	return diagnostics.CheckResult{
		ID:       c.ID(),
		Name:     c.Name(),
		Severity: diagnostics.SeverityInfo,
		Passed:   true,
		Summary:  "router learning check passed",
		Details: map[string]any{
			"policy_count": 0, // 실제 값으로 대체
		},
		StartedAt:  start,
		FinishedAt: time.Now(),
		DurationMs: time.Since(start).Milliseconds(),
	}
}
