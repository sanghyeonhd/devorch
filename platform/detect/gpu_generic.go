//go:build !darwin && !linux && !windows

package platform

// Fallback GPU detection for unsupported platforms.
func detectGPU() *GPUInfo {
	return &GPUInfo{Available: false}
}
