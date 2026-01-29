package version

import (
	"os/exec"
	"runtime"
	"strings"
)

func Detect() RuntimeInfo {
	info := RuntimeInfo{
		CPUCores: runtime.NumCPU(),
	}

	// (단순 구현 - 실제로는 플랫폼별로 확장)
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
	return runtime.GOARCH
}

func detectGPU() (model, driver, backend string) {
	// 최소 구현 (실제 제품에서는 nvidia-smi, system_profiler 등 사용)
	return "", "", "cpu"
}

func detectRAM() float64 {
	// 단순 stub
	return 0
}
