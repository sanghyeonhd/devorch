// Package bench provides quality evaluation and benchmark utilities.
// Phase 31: Quality Gate implementation
package bench

import (
	"math"
	"strings"
)

// QualityInput represents input for quality evaluation.
type QualityInput struct {
	Prompt    string
	Output    string
	LatencyMs int64
}

// QualityResult represents the result of quality evaluation.
type QualityResult struct {
	Score float64  // 0.0 ~ 1.0
	Notes []string // Evaluation notes
}

// EvaluateQuality performs lightweight local quality heuristics.
// This can be replaced with LLM-as-judge or task-type specific evaluators in later phases.
func EvaluateQuality(in QualityInput) QualityResult {
	var notes []string

	output := strings.TrimSpace(in.Output)
	if output == "" {
		return QualityResult{Score: 0.0, Notes: []string{"empty_output"}}
	}

	length := len(output)
	score := 0.5

	// Penalize very short outputs
	if length < 50 {
		score -= 0.2
		notes = append(notes, "too_short")
	}

	// Bonus for code/structure hints
	if strings.Contains(output, "```") || strings.Contains(output, "package ") {
		score += 0.1
	}

	// Penalize error/apology patterns
	lower := strings.ToLower(output)
	if strings.Contains(lower, "sorry") || strings.Contains(lower, "cannot") ||
		strings.Contains(lower, "i can't") || strings.Contains(lower, "unable to") {
		score -= 0.1
		notes = append(notes, "apology_or_refusal")
	}

	// Latency penalty (linear decrease after 5 seconds)
	if in.LatencyMs > 5000 {
		penalty := math.Min(0.3, float64(in.LatencyMs-5000)/20000.0)
		score -= penalty
		notes = append(notes, "slow_response")
	}

	// Clamp to valid range
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return QualityResult{Score: score, Notes: notes}
}
