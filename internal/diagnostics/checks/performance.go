package checks

import (
	"context"
	"time"

	"devorch/internal/diagnostics"
)

type Performance struct{}

func (c Performance) ID() string   { return "performance" }
func (c Performance) Name() string { return "Performance / Quick Bench" }

func (c Performance) Run(ctx context.Context, in diagnostics.Input) diagnostics.CheckResult {
	start := time.Now()

	details := map[string]any{}
	passed := true
	severity := diagnostics.SeverityInfo
	summary := "Quick bench succeeded"
	suggestions := []string(nil)

	if in.Bench == nil {
		passed = false
		severity = diagnostics.SeverityWarn
		summary = "Bench runner not configured"
		suggestions = []string{
			"Wire BenchRunner into diagnostics input",
			"Enable benchmark history storage to improve routing over time",
		}
	} else {
		sum, err := in.Bench.RunQuick(ctx)
		if err != nil {
			passed = false
			severity = diagnostics.SeverityWarn
			summary = "Quick bench failed"
			details["error"] = err.Error()
			suggestions = []string{
				"Verify local runtime (ollama/llama.cpp) is reachable",
				"Verify remote providers (openrouter/openai/anthropic) credentials",
				"Try again with `devorch bench --quick --verbose`",
			}
		} else {
			details["createdAt"] = sum.CreatedAt
			details["items"] = sum.Items
		}
	}

	finish := time.Now()
	return diagnostics.CheckResult{
		ID:          c.ID(),
		Name:        c.Name(),
		Severity:    severity,
		Passed:      passed,
		Summary:     summary,
		Details:     details,
		Suggestions: suggestions,
		StartedAt:   start,
		FinishedAt:  finish,
		DurationMs:  finish.Sub(start).Milliseconds(),
	}
}
