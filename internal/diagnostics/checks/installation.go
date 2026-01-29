package checks

import (
	"context"
	"os"
	"time"

	"devorch/internal/diagnostics"
)

type Installation struct{}

func (c Installation) ID() string   { return "installation" }
func (c Installation) Name() string { return "Installation / Paths" }

func (c Installation) Run(ctx context.Context, in diagnostics.Input) diagnostics.CheckResult {
	start := time.Now()
	defer func() { _ = ctx }()

	details := map[string]any{
		"os":      in.OS,
		"arch":    in.Arch,
		"version": in.Version,
		"paths": map[string]any{
			"config": in.ConfigDir,
			"data":   in.DataDir,
			"cache":  in.CacheDir,
			"state":  in.StateDir,
			"log":    in.LogDir,
		},
	}

	missing := make([]string, 0, 8)
	checkDir := func(label, p string) {
		if p == "" {
			missing = append(missing, label+":<empty>")
			return
		}
		st, err := os.Stat(p)
		if err != nil || !st.IsDir() {
			missing = append(missing, label+":"+p)
		}
	}

	checkDir("config", in.ConfigDir)
	checkDir("data", in.DataDir)
	checkDir("cache", in.CacheDir)
	checkDir("state", in.StateDir)
	checkDir("log", in.LogDir)

	passed := len(missing) == 0
	severity := diagnostics.SeverityInfo
	summary := "All required directories exist"
	suggestions := []string(nil)

	if !passed {
		severity = diagnostics.SeverityError
		summary = "Some required directories are missing or not accessible"
		suggestions = []string{
			"Run `devorch install` to create required directories",
			"Check filesystem permissions",
		}
		details["missing"] = missing
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
