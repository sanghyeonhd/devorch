package local

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type HWInfo struct {
	OS      string
	Arch    string
	Cores   int
	MemMB   int
	GPUHint string
}

func DetectHW() HWInfo {
	return HWInfo{
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		Cores:   runtime.NumCPU(),
		MemMB:   detectSystemMemoryMB(),
		GPUHint: detectGPUHint(),
	}
}

// detectSystemMemoryMB returns system memory in MB
func detectSystemMemoryMB() int {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			if bytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
				return int(bytes / (1024 * 1024))
			}
		}

	case "linux":
		data, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
							return int(kb / 1024)
						}
					}
				}
			}
		}

	case "windows":
		out, err := exec.Command("wmic", "computersystem", "get", "totalphysicalmemory").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && line != "TotalPhysicalMemory" {
					if bytes, err := strconv.ParseInt(line, 10, 64); err == nil {
						return int(bytes / (1024 * 1024))
					}
				}
			}
		}
	}
	return 0
}

// detectGPUHint returns a hint about available GPU
func detectGPUHint() string {
	switch runtime.GOOS {
	case "darwin":
		// Apple Silicon has integrated GPU with Metal support
		if runtime.GOARCH == "arm64" {
			return "apple-metal"
		}
		out, _ := exec.Command("system_profiler", "SPDisplaysDataType").Output()
		if strings.Contains(string(out), "Metal") {
			return "metal"
		}

	case "linux":
		// Check for NVIDIA GPU
		if _, err := exec.LookPath("nvidia-smi"); err == nil {
			return "nvidia-cuda"
		}
		// Check for AMD GPU
		if _, err := os.Stat("/sys/class/drm/card0/device/vendor"); err == nil {
			vendor, _ := os.ReadFile("/sys/class/drm/card0/device/vendor")
			if strings.Contains(string(vendor), "0x1002") {
				return "amd-rocm"
			}
		}

	case "windows":
		out, _ := exec.Command("wmic", "path", "win32_videocontroller", "get", "name").Output()
		outStr := strings.ToLower(string(out))
		if strings.Contains(outStr, "nvidia") {
			return "nvidia-cuda"
		}
		if strings.Contains(outStr, "amd") || strings.Contains(outStr, "radeon") {
			return "amd-directml"
		}
	}
	return "cpu"
}
