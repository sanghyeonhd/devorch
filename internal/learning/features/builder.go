// Package features provides feature vector building for learning.
// Phase 33: Feature builder (envfp + repo + task_type + category)
package features

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// FeatureVector represents features used for learning/routing decisions.
type FeatureVector struct {
	EnvFingerprint  string
	RepoFingerprint string
	Category        string
	TaskType        string // "chat", "patch", "tests", "search", ...

	PromptBytes int
	FileTouches int

	// Hash key for caching/lookup
	Key string
}

// BuildInput contains input for building a feature vector.
type BuildInput struct {
	EnvFingerprint  string
	RepoFingerprint string
	Category        string
	TaskType        string

	Prompt      string
	FileTouches int
}

// Build constructs a feature vector from the given input.
func Build(in BuildInput) FeatureVector {
	pb := len([]byte(in.Prompt))

	fv := FeatureVector{
		EnvFingerprint:  strings.TrimSpace(in.EnvFingerprint),
		RepoFingerprint: strings.TrimSpace(in.RepoFingerprint),
		Category:        strings.TrimSpace(in.Category),
		TaskType:        strings.TrimSpace(in.TaskType),
		PromptBytes:     pb,
		FileTouches:     in.FileTouches,
	}
	fv.Key = computeFeatureKey(fv)
	return fv
}

// computeFeatureKey generates a stable key for the feature vector.
// Prompt length is bucketed to avoid excessive key variation.
func computeFeatureKey(f FeatureVector) string {
	// Bucket prompt size (every 256 bytes)
	promptBucket := f.PromptBytes / 256

	s := f.EnvFingerprint + "|" +
		f.RepoFingerprint + "|" +
		f.Category + "|" +
		f.TaskType + "|" +
		"pb:" + strconv.Itoa(promptBucket) + "|" +
		"ft:" + strconv.Itoa(f.FileTouches)

	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ShortKey returns a shortened version of the key.
func (f FeatureVector) ShortKey() string {
	if len(f.Key) > 16 {
		return f.Key[:16]
	}
	return f.Key
}

// IsComplete checks if the feature vector has required fields.
func (f FeatureVector) IsComplete() bool {
	return f.EnvFingerprint != "" && f.Category != ""
}

// Match checks if this feature vector matches another for learning purposes.
// Used for finding similar historical data.
func (f FeatureVector) Match(other FeatureVector, strictRepo bool) bool {
	if f.EnvFingerprint != other.EnvFingerprint {
		return false
	}
	if f.Category != other.Category {
		return false
	}
	if f.TaskType != other.TaskType {
		return false
	}
	if strictRepo && f.RepoFingerprint != other.RepoFingerprint {
		return false
	}
	return true
}
