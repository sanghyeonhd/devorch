// Package eval provides quality evaluation interfaces.
// Phase 33: Pluggable task-type evaluators
package eval

import "context"

// Input represents input for quality evaluation.
type Input struct {
	TaskType string

	Prompt   string
	Output   string // Model output text (final)
	Patch    string // Patch content if applicable
	TestCmd  string // Test command if applicable
	RepoRoot string // Repository root for test/patch application
}

// Result represents the result of quality evaluation.
type Result struct {
	QualityScore float64            // 0..1
	Signals      map[string]float64 // Detailed signals
	Notes        []string           // Evaluation notes
}

// Evaluator evaluates quality for specific task types.
type Evaluator interface {
	Name() string
	Supports(taskType string) bool
	Evaluate(ctx context.Context, in Input) (Result, error)
}
