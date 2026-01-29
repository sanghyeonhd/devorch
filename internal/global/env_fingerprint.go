// Package global provides environment fingerprinting utilities.
// Phase 32/33: Extended env fingerprint with HW profile support
package global

import (
	"os"

	platform "devorch/platform/detect"
)

// EnvFingerprintFromEnvOrDefault returns the environment fingerprint.
// Uses DEVORCH_ENV_FINGERPRINT if set, otherwise detects from hardware.
func EnvFingerprintFromEnvOrDefault() string {
	if v := os.Getenv("DEVORCH_ENV_FINGERPRINT"); v != "" {
		return v
	}

	// Try to detect HW profile and use its hash
	p, err := platform.DetectHWProfile()
	if err == nil {
		return p.Hash()
	}

	// Fallback to basic fingerprint
	fp := GetFingerprint()
	return fp.InstallID
}

// EnvFingerprintWithTier returns the fingerprint along with the machine tier.
func EnvFingerprintWithTier() (fingerprint string, tier string) {
	if v := os.Getenv("DEVORCH_ENV_FINGERPRINT"); v != "" {
		return v, "unknown"
	}

	p, err := platform.DetectHWProfile()
	if err == nil {
		return p.Hash(), p.Tier()
	}

	fp := GetFingerprint()
	return fp.InstallID, "unknown"
}

// GetHWProfile returns the detected hardware profile.
func GetHWProfile() (platform.HWProfile, error) {
	return platform.DetectHWProfile()
}
