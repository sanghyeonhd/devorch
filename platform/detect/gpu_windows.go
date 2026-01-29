//go:build windows

package platform

import (
	"os/exec"
	"strconv"
	"strings"
)

func detectGPU() *GPUInfo {
	// NVIDIA GPU 확인
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.total,memory.free", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err == nil {
		var devices []GPUDev
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")

		for _, line := range lines {
			parts := strings.Split(line, ", ")
			if len(parts) >= 3 {
				memTotal, _ := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
				memFree, _ := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 64)

				devices = append(devices, GPUDev{
					Name:        strings.TrimSpace(parts[0]),
					MemoryTotal: memTotal * 1024 * 1024,
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
	}

	// WMIC로 GPU 정보 조회 (레거시)
	cmd2 := exec.Command("wmic", "path", "win32_VideoController", "get", "name,AdapterRAM")
	output2, err := cmd2.Output()
	if err == nil {
		lines := strings.Split(string(output2), "\n")
		var devices []GPUDev

		for _, line := range lines[1:] { // 헤더 스킵
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// 간단히 이름만 추출
			devices = append(devices, GPUDev{
				Name: line,
			})
		}

		if len(devices) > 0 {
			return &GPUInfo{
				Available: true,
				Devices:   devices,
			}
		}
	}

	// PowerShell로 GPU 정보 조회
	cmd3 := exec.Command("powershell", "-Command", "Get-WmiObject Win32_VideoController | Select-Object Name, AdapterRAM | Format-List")
	output3, err := cmd3.Output()
	if err == nil {
		content := string(output3)
		if strings.Contains(content, "Name") {
			// 이름 추출
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				if strings.HasPrefix(strings.TrimSpace(line), "Name") {
					parts := strings.SplitN(line, ":", 2)
					if len(parts) == 2 {
						return &GPUInfo{
							Available: true,
							Devices: []GPUDev{
								{Name: strings.TrimSpace(parts[1])},
							},
						}
					}
				}
			}
		}
	}

	return &GPUInfo{Available: false}
}

// GetSystemMemory는 Windows 시스템 메모리를 반환합니다
func GetSystemMemory() (total, free uint64) {
	// PowerShell로 메모리 정보 조회
	cmd := exec.Command("powershell", "-Command",
		"[math]::Round((Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory); [math]::Round((Get-CimInstance Win32_OperatingSystem).FreePhysicalMemory * 1024)")
	output, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) >= 2 {
		total, _ = strconv.ParseUint(strings.TrimSpace(lines[0]), 10, 64)
		free, _ = strconv.ParseUint(strings.TrimSpace(lines[1]), 10, 64)
	}

	return total, free
}
