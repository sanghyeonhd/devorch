// Package platform provides hardware profile detection.
// Phase 33: Common types for HW Profile
package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// HWProfile represents hardware profile information.
type HWProfile struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	CPUCount int    `json:"cpu_count"`

	MemTotalMB  int64  `json:"mem_total_mb"`
	DiskTotalGB int64  `json:"disk_total_gb"`
	DiskFreeGB  int64  `json:"disk_free_gb"`
	HasAccel    bool   `json:"has_accel"`  // GPU/Metal/CUDA/DirectML
	AccelKind   string `json:"accel_kind"` // "metal", "cuda", "directml", "rocm", ""
}

// Hash returns a stable hash of the HW profile.
func (p HWProfile) Hash() string {
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Tier returns a rough machine tier classification.
func (p HWProfile) Tier() string {
	if p.HasAccel && p.MemTotalMB >= 32768 {
		return "high"
	}
	if p.MemTotalMB >= 16384 {
		return "mid"
	}
	return "low"
}

// CanRunLocalLLM checks if the machine can run local LLMs.
func (p HWProfile) CanRunLocalLLM() bool {
	// Minimum requirements: 8GB RAM or GPU acceleration
	return p.MemTotalMB >= 8192 || p.HasAccel
}

// RecommendedModelSize returns the recommended model size for local inference.
func (p HWProfile) RecommendedModelSize() string {
	if p.HasAccel && p.MemTotalMB >= 32768 {
		return "13B+"
	}
	if p.MemTotalMB >= 16384 {
		return "7B-13B"
	}
	if p.MemTotalMB >= 8192 {
		return "3B-7B"
	}
	return "<=3B"
}
