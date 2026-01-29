package checks

import (
	"context"
	"time"

	"devorch/internal/diagnostics"
)

type Config struct{}

func (c Config) ID() string   { return "config" }
func (c Config) Name() string { return "Configuration" }

func (c Config) Run(ctx context.Context, in diagnostics.Input) diagnostics.CheckResult {
	start := time.Now()
	defer func() { _ = ctx }()

	// Step 1~8에서 config.Validate(...) 같은 게 존재한다면 여기서 호출하도록 확장하세요.
	// 지금은 "경로 존재 여부" 수준의 최소 체크만 수행합니다.
	passed := in.ConfigDir != ""

	severity := diagnostics.SeverityInfo
	summary := "Configuration looks okay"
	suggestions := []string(nil)

	if !passed {
		severity = diagnostics.SeverityError
		summary = "Config directory is empty"
		suggestions = []string{
			"Set XDG_CONFIG_HOME or ensure config directory resolution works",
			"Run `devorch debug paths` to verify resolved paths",
		}
	}

	finish := time.Now()
	return diagnostics.CheckResult{
		ID:          c.ID(),
		Name:        c.Name(),
		Severity:    severity,
		Passed:      passed,
		Summary:     summary,
		Details:     map[string]any{"configDir": in.ConfigDir},
		Suggestions: suggestions,
		StartedAt:   start,
		FinishedAt:  finish,
		DurationMs:  finish.Sub(start).Milliseconds(),
	}
}
