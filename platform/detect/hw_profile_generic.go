//go:build !darwin && !linux && !windows

package platform

import "runtime"

// DetectHWProfile provides a minimal fallback implementation for unsupported platforms.
func DetectHWProfile() (HWProfile, error) {
	p := HWProfile{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCount: runtime.NumCPU(),
	}
	return p, nil
}
