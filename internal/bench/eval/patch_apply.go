// Package eval provides patch application quality evaluation.
// Phase 33: Patch evaluator
package eval

import (
	"context"
	"os/exec"
	"strings"
)

// PatchApply evaluates patch application quality.
type PatchApply struct{}

// Name returns the evaluator name.
func (p PatchApply) Name() string { return "patch_apply" }

// Supports returns true for patch-related task types.
func (p PatchApply) Supports(taskType string) bool {
	return taskType == "patch" || taskType == "edit" || taskType == "code-edit"
}

// Evaluate performs patch quality evaluation.
func (p PatchApply) Evaluate(ctx context.Context, in Input) (Result, error) {
	signals := make(map[string]float64)

	// If no patch content, evaluate based on output
	if in.Patch == "" {
		return p.evaluateOutput(in.Output, signals)
	}

	// Check patch format
	if !isValidPatch(in.Patch) {
		signals["invalid_format"] = 1
		return Result{
			QualityScore: 0.3,
			Signals:      signals,
			Notes:        []string{"invalid_patch_format"},
		}, nil
	}

	signals["valid_format"] = 1

	// If we have repo root, try dry-run apply
	if in.RepoRoot != "" {
		applied := tryDryRunPatch(ctx, in.RepoRoot, in.Patch)
		if applied {
			signals["applies_cleanly"] = 1
			return Result{
				QualityScore: 0.9,
				Signals:      signals,
				Notes:        []string{"patch_applies_cleanly"},
			}, nil
		}
		signals["apply_failed"] = 1
		return Result{
			QualityScore: 0.5,
			Signals:      signals,
			Notes:        []string{"patch_apply_failed"},
		}, nil
	}

	// Can't verify, assume moderate quality
	return Result{
		QualityScore: 0.6,
		Signals:      signals,
		Notes:        []string{"patch_not_verified"},
	}, nil
}

func (p PatchApply) evaluateOutput(output string, signals map[string]float64) (Result, error) {
	// Check if output looks like a patch
	if strings.Contains(output, "@@") && strings.Contains(output, "---") {
		signals["looks_like_patch"] = 1
		return Result{
			QualityScore: 0.6,
			Signals:      signals,
		}, nil
	}

	// Check if it contains code blocks
	if strings.Contains(output, "```") {
		signals["has_code_block"] = 1
		return Result{
			QualityScore: 0.5,
			Signals:      signals,
		}, nil
	}

	signals["no_patch_content"] = 1
	return Result{
		QualityScore: 0.3,
		Signals:      signals,
	}, nil
}

func isValidPatch(patch string) bool {
	// Basic validation: unified diff format markers
	return (strings.Contains(patch, "@@") ||
		strings.Contains(patch, "---") ||
		strings.Contains(patch, "+++")) &&
		strings.Contains(patch, "\n")
}

func tryDryRunPatch(ctx context.Context, repoRoot, patch string) bool {
	cmd := exec.CommandContext(ctx, "git", "apply", "--check", "-")
	cmd.Dir = repoRoot
	cmd.Stdin = strings.NewReader(patch)

	err := cmd.Run()
	return err == nil
}
