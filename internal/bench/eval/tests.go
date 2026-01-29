// Package eval provides test execution quality evaluation.
// Phase 33: Test evaluator
package eval

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

// Tests evaluates quality based on test execution.
type Tests struct {
	Timeout time.Duration
}

// NewTests creates a new test evaluator with default timeout.
func NewTests() Tests {
	return Tests{Timeout: 60 * time.Second}
}

// Name returns the evaluator name.
func (t Tests) Name() string { return "tests" }

// Supports returns true for test-related task types.
func (t Tests) Supports(taskType string) bool {
	return taskType == "tests" || taskType == "test" || taskType == "fix" || taskType == "debug"
}

// Evaluate performs test-based quality evaluation.
func (t Tests) Evaluate(ctx context.Context, in Input) (Result, error) {
	signals := make(map[string]float64)

	// If no test command or repo root, fall back to output analysis
	if in.TestCmd == "" || in.RepoRoot == "" {
		return t.evaluateOutput(in.Output, signals)
	}

	// Run tests with timeout
	timeout := t.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	passed, total, err := runTests(testCtx, in.RepoRoot, in.TestCmd)
	if err != nil {
		signals["test_error"] = 1
		return Result{
			QualityScore: 0.3,
			Signals:      signals,
			Notes:        []string{"test_execution_error: " + err.Error()},
		}, nil
	}

	if total == 0 {
		signals["no_tests"] = 1
		return Result{
			QualityScore: 0.5,
			Signals:      signals,
			Notes:        []string{"no_tests_found"},
		}, nil
	}

	passRate := float64(passed) / float64(total)
	signals["pass_rate"] = passRate
	signals["passed"] = float64(passed)
	signals["total"] = float64(total)

	// Score based on pass rate
	score := 0.3 + (passRate * 0.7)

	var notes []string
	if passRate == 1.0 {
		notes = append(notes, "all_tests_passed")
	} else if passRate >= 0.9 {
		notes = append(notes, "most_tests_passed")
	} else if passRate >= 0.5 {
		notes = append(notes, "some_tests_failed")
	} else {
		notes = append(notes, "many_tests_failed")
	}

	return Result{
		QualityScore: score,
		Signals:      signals,
		Notes:        notes,
	}, nil
}

func (t Tests) evaluateOutput(output string, signals map[string]float64) (Result, error) {
	lower := strings.ToLower(output)

	// Check for test-related keywords
	if strings.Contains(lower, "pass") || strings.Contains(lower, "ok") {
		signals["mentions_pass"] = 1
	}
	if strings.Contains(lower, "fail") || strings.Contains(lower, "error") {
		signals["mentions_fail"] = 1
	}

	// Check for code
	if strings.Contains(output, "```") || strings.Contains(output, "func Test") {
		signals["has_code"] = 1
		return Result{
			QualityScore: 0.6,
			Signals:      signals,
		}, nil
	}

	return Result{
		QualityScore: 0.5,
		Signals:      signals,
	}, nil
}

func runTests(ctx context.Context, repoRoot, testCmd string) (passed, total int, err error) {
	parts := strings.Fields(testCmd)
	if len(parts) == 0 {
		return 0, 0, nil
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = repoRoot

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// Parse test output
	output := stdout.String() + stderr.String()
	passed, total = parseTestOutput(output)

	// If command failed but we got some test info, return that
	if err != nil && total > 0 {
		return passed, total, nil
	}

	return passed, total, err
}

func parseTestOutput(output string) (passed, total int) {
	// Generic parsing for common test frameworks
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		lower := strings.ToLower(line)

		// Go test output
		if strings.HasPrefix(line, "---") {
			total++
			if strings.Contains(line, "PASS") {
				passed++
			}
		}

		// pytest style
		if strings.Contains(lower, " passed") {
			// Try to extract numbers
			// e.g., "5 passed, 2 failed"
		}

		// Generic PASS/FAIL
		if strings.Contains(line, "PASS") || strings.Contains(line, "✓") {
			passed++
			total++
		} else if strings.Contains(line, "FAIL") || strings.Contains(line, "✗") {
			total++
		}
	}

	return passed, total
}
