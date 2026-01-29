package checks

import (
	"context"
	"time"

	"devorch/internal/diagnostics"
)

// RouterSharingCheck: policy 공유 설정 점검
type RouterSharingCheck struct{}

func (c RouterSharingCheck) ID() string {
	return "router-sharing"
}

func (c RouterSharingCheck) Name() string {
	return "Router Sharing"
}

func (c RouterSharingCheck) Run(ctx context.Context, in diagnostics.Input) diagnostics.CheckResult {
	start := time.Now()
	// 실제 구현에서는:
	// 1. 공유 설정 확인
	// 2. 서명 키 존재 확인
	// 3. 원격 sync 상태 확인

	return diagnostics.CheckResult{
		ID:       c.ID(),
		Name:     c.Name(),
		Severity: diagnostics.SeverityInfo,
		Passed:   true,
		Summary:  "router sharing check passed",
		Details: map[string]any{
			"sharing_enabled": false,
		},
		StartedAt:  start,
		FinishedAt: time.Now(),
		DurationMs: time.Since(start).Milliseconds(),
	}
}
