//go:build linux

package platform

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func detectGPU() *GPUInfo {
	// NVIDIA GPU 확인 (nvidia-smi)
	if nvidiaInfo := detectNVIDIA(); nvidiaInfo != nil {
		return nvidiaInfo
	}

	// AMD GPU 확인
	if amdInfo := detectAMD(); amdInfo != nil {
		return amdInfo
	}

	// lspci로 그래픽 카드 확인
	cmd := exec.Command("lspci")
	output, err := cmd.Output()
	if err != nil {
		return &GPUInfo{Available: false}
	}

	content := strings.ToLower(string(output))
	if strings.Contains(content, "vga") || strings.Contains(content, "3d") {
		return &GPUInfo{
			Available: true,
			Devices: []GPUDev{
				{Name: "Generic GPU detected via lspci"},
			},
		}
	}

	return &GPUInfo{Available: false}
}

func detectNVIDIA() *GPUInfo {
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.total,memory.free", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var devices []GPUDev
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		parts := strings.Split(line, ", ")
		if len(parts) >= 3 {
			memTotal, _ := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
			memFree, _ := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 64)

			devices = append(devices, GPUDev{
				Name:        strings.TrimSpace(parts[0]),
				MemoryTotal: memTotal * 1024 * 1024, // MiB to bytes
				MemoryFree:  memFree * 1024 * 1024,
				Vendor:      "NVIDIA",
			})
		}
	}

	if len(devices) > 0 {
		return &GPUInfo{
			Available: true,
			Devices:   devices,
		}
	}
	return nil
}

func detectAMD() *GPUInfo {
	// ROCm 확인
	rocmPath := "/opt/rocm/bin/rocm-smi"
	if _, err := os.Stat(rocmPath); err != nil {
		return nil
	}

	cmd := exec.Command(rocmPath, "--showproductname")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	content := strings.TrimSpace(string(output))
	if content == "" {
		return nil
	}

	return &GPUInfo{
		Available: true,
		Devices: []GPUDev{
			{
				Name:   content,
				Vendor: "AMD",
			},
		},
	}
}

// GetSystemMemory는 Linux 시스템 메모리를 반환합니다
func GetSystemMemory() (total, free uint64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, _ := strconv.ParseUint(fields[1], 10, 64)
		value *= 1024 // kB to bytes

		switch fields[0] {
		case "MemTotal:":
			total = value
		case "MemAvailable:":
			free = value
		}
	}

	return total, free
}
