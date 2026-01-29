//go:build darwin

package platform

import (
	"os/exec"
	"strconv"
	"strings"
)

func detectGPU() *GPUInfo {
	// macOS에서 GPU 정보 감지
	// system_profiler를 사용하여 GPU 정보 조회
	cmd := exec.Command("system_profiler", "SPDisplaysDataType", "-json")
	output, err := cmd.Output()
	if err != nil {
		return &GPUInfo{Available: false}
	}

	// JSON 파싱은 복잡하므로 간단한 텍스트 파싱
	// 실제로는 JSON 파싱 권장
	content := string(output)
	if strings.Contains(content, "sppci_model") {
		return &GPUInfo{
			Available: true,
			Devices: []GPUDev{
				{
					Name:   extractGPUName(content),
					Vendor: "Apple",
				},
			},
		}
	}

	// 대안: ioreg 사용
	cmd2 := exec.Command("ioreg", "-l")
	output2, err := cmd2.Output()
	if err != nil {
		return &GPUInfo{Available: false}
	}

	if strings.Contains(string(output2), "AGXAccelerator") || strings.Contains(string(output2), "AppleIntelFramebuffer") {
		return &GPUInfo{
			Available: true,
			Devices: []GPUDev{
				{
					Name:   "Integrated GPU",
					Vendor: "Apple",
				},
			},
		}
	}

	return &GPUInfo{Available: false}
}

func extractGPUName(content string) string {
	// 간단한 파싱: "Chipset Model" 또는 GPU 이름 추출
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, "sppci_model") || strings.Contains(line, "Chipset Model") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(strings.Trim(parts[1], `"`))
			}
		}
	}

	// Apple Silicon 확인
	cmd := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
	output, err := cmd.Output()
	if err == nil {
		brand := strings.TrimSpace(string(output))
		if strings.Contains(brand, "Apple") {
			return brand + " GPU"
		}
	}

	return "Unknown GPU"
}

// GetSystemMemory는 macOS 시스템 메모리를 반환합니다
func GetSystemMemory() (total, free uint64) {
	// sysctl로 총 메모리 조회
	cmd := exec.Command("sysctl", "-n", "hw.memsize")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	total, _ = strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)

	// vm_stat로 free 메모리 계산
	cmd2 := exec.Command("vm_stat")
	output2, err := cmd2.Output()
	if err != nil {
		return total, 0
	}

	lines := strings.Split(string(output2), "\n")
	var freePages, inactivePages uint64
	pageSize := uint64(4096) // 기본 페이지 크기

	for _, line := range lines {
		if strings.HasPrefix(line, "Pages free:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				freePages, _ = strconv.ParseUint(strings.Trim(parts[2], "."), 10, 64)
			}
		}
		if strings.HasPrefix(line, "Pages inactive:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				inactivePages, _ = strconv.ParseUint(strings.Trim(parts[2], "."), 10, 64)
			}
		}
	}

	free = (freePages + inactivePages) * pageSize
	return total, free
}
