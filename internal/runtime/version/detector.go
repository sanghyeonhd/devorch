package version

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func Detect() RuntimeInfo {
	info := RuntimeInfo{
		CPUCores: runtime.NumCPU(),
	}

	// 플랫폼별 CPU 모델 감지
	info.CPUModel = detectCPU()
	info.GPUModel, info.GPUDriver, info.GPUBackend = detectGPU()

	info.OllamaVersion = detectCmd("ollama", "version")
	info.LlamaCppVersion = detectCmd("llama-cli", "--version")

	info.RAMGB = detectRAM()

	return info
}

func detectCmd(bin string, arg string) string {
	out, err := exec.Command(bin, arg).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectCPU() string {
	switch runtime.GOOS {
	case "darwin":
		// macOS: sysctl로 CPU 브랜드 조회
		out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	case "linux":
		// Linux: /proc/cpuinfo에서 모델명 추출
		data, err := os.ReadFile("/proc/cpuinfo")
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "model name") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						return strings.TrimSpace(parts[1])
					}
				}
			}
		}
	case "windows":
		// Windows: wmic로 CPU 정보 조회
		out, err := exec.Command("wmic", "cpu", "get", "name").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && line != "Name" {
					return line
				}
			}
		}
	}
	return runtime.GOARCH
}

func detectGPU() (model, driver, backend string) {
	switch runtime.GOOS {
	case "darwin":
		// macOS: system_profiler로 GPU 정보 조회
		out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "Chipset Model:") {
					model = strings.TrimSpace(strings.TrimPrefix(line, "Chipset Model:"))
				}
				if strings.HasPrefix(line, "Metal Support:") || strings.HasPrefix(line, "Metal Family:") {
					backend = "metal"
				}
			}
		}
		if model != "" {
			if backend == "" {
				backend = "metal" // Apple Silicon은 기본적으로 Metal 지원
			}
			return model, "apple", backend
		}

	case "linux":
		// Linux: nvidia-smi 시도
		out, err := exec.Command("nvidia-smi", "--query-gpu=name,driver_version", "--format=csv,noheader").Output()
		if err == nil && len(out) > 0 {
			parts := strings.Split(strings.TrimSpace(string(out)), ",")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), "cuda"
			}
		}
		// AMD GPU 확인
		if _, err := os.Stat("/sys/class/drm/card0/device/vendor"); err == nil {
			vendor, _ := os.ReadFile("/sys/class/drm/card0/device/vendor")
			if bytes.Contains(vendor, []byte("0x1002")) { // AMD vendor ID
				return "AMD GPU", "amdgpu", "rocm"
			}
		}

	case "windows":
		// Windows: wmic로 GPU 정보 조회
		out, err := exec.Command("wmic", "path", "win32_videocontroller", "get", "name").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && line != "Name" {
					if strings.Contains(strings.ToLower(line), "nvidia") {
						return line, "nvidia", "cuda"
					}
					if strings.Contains(strings.ToLower(line), "amd") || strings.Contains(strings.ToLower(line), "radeon") {
						return line, "amd", "directml"
					}
					return line, "unknown", "cpu"
				}
			}
		}
	}

	return "", "", "cpu"
}

func detectRAM() float64 {
	switch runtime.GOOS {
	case "darwin":
		// macOS: sysctl로 물리 메모리 조회
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			if bytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
				return float64(bytes) / (1024 * 1024 * 1024) // bytes to GB
			}
		}

	case "linux":
		// Linux: /proc/meminfo에서 MemTotal 읽기
		data, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
							return float64(kb) / (1024 * 1024) // kB to GB
						}
					}
				}
			}
		}

	case "windows":
		// Windows: wmic로 총 메모리 조회
		out, err := exec.Command("wmic", "computersystem", "get", "totalphysicalmemory").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && line != "TotalPhysicalMemory" {
					if bytes, err := strconv.ParseInt(line, 10, 64); err == nil {
						return float64(bytes) / (1024 * 1024 * 1024)
					}
				}
			}
		}
	}

	// fallback: Go runtime 메모리 (시스템 전체 메모리 아님)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Sys) / (1024 * 1024 * 1024)
}
