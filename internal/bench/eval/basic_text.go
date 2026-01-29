// Package eval provides basic text quality evaluation.
// Phase 33: Basic text evaluator (fallback for all task types)
package eval

import (
	"context"
	"strings"
)

// BasicText evaluates basic text quality metrics.
type BasicText struct{}

// Name returns the evaluator name.
func (b BasicText) Name() string { return "basic_text" }

// Supports returns true for all task types (fallback evaluator).
func (b BasicText) Supports(taskType string) bool {
	return true
}

// Evaluate performs basic text quality evaluation.
func (b BasicText) Evaluate(ctx context.Context, in Input) (Result, error) {
	_ = ctx

	output := strings.TrimSpace(in.Output)
	signals := make(map[string]float64)

	// Empty output
	if output == "" {
		return Result{
			QualityScore: 0.0,
			Signals:      map[string]float64{"empty": 1},
			Notes:        []string{"empty_output"},
		}, nil
	}

	score := 0.5

	// Length checks
	length := len(output)
	if length < 20 {
		score -= 0.3
		signals["very_short"] = 1
	} else if length < 50 {
		score -= 0.15
		signals["short"] = 1
	} else if length > 500 {
		score += 0.1
		signals["substantial"] = 1
	}

	// Check for refusal/apology patterns
	lower := strings.ToLower(output)
	refusalPatterns := []string{
		"i can't", "i cannot", "i'm unable", "i am unable",
		"sorry, but", "i apologize", "unfortunately, i",
	}
	for _, pattern := range refusalPatterns {
		if strings.Contains(lower, pattern) {
			score -= 0.2
			signals["refusal"] = 1
			break
		}
	}

	// Check for code indicators (positive for code tasks)
	if strings.Contains(output, "```") ||
		strings.Contains(output, "package ") ||
		strings.Contains(output, "func ") ||
		strings.Contains(output, "def ") ||
		strings.Contains(output, "function ") {
		score += 0.1
		signals["has_code"] = 1
	}

	// Check for structured output
	if strings.Contains(output, "\n\n") ||
		strings.Count(output, "\n") > 5 {
		score += 0.05
		signals["structured"] = 1
	}

	// Clamp score
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return Result{
		QualityScore: score,
		Signals:      signals,
	}, nil
}
