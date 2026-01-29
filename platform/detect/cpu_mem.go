package platform

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func detectMemory() MemInfo {
	totalBytes := detectTotalMemory()
	freeBytes := detectFreeMemory()
	usedBytes := totalBytes - freeBytes
	if usedBytes < 0 {
		usedBytes = 0
	}

	return MemInfo{
		TotalBytes: totalBytes,
		UsedBytes:  usedBytes,
		FreeBytes:  freeBytes,
	}
}

func detectTotalMemory() uint64 {
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			if bytes, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); err == nil {
				return bytes
			}
		}

	case "linux":
		data, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
							return kb * 1024 // kB to bytes
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
					if bytes, err := strconv.ParseUint(line, 10, 64); err == nil {
						return bytes
					}
				}
			}
		}
	}

	// fallback: Go runtime 메모리
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys
}

func detectFreeMemory() uint64 {
	switch runtime.GOOS {
	case "darwin":
		// macOS: vm_stat로 free pages 계산
		out, err := exec.Command("vm_stat").Output()
		if err == nil {
			var freePages, inactivePages uint64
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "Pages free:") {
					fields := strings.Fields(line)
					if len(fields) >= 3 {
						freePages, _ = strconv.ParseUint(strings.TrimSuffix(fields[2], "."), 10, 64)
					}
				}
				if strings.HasPrefix(line, "Pages inactive:") {
					fields := strings.Fields(line)
					if len(fields) >= 3 {
						inactivePages, _ = strconv.ParseUint(strings.TrimSuffix(fields[2], "."), 10, 64)
					}
				}
			}
			// Page size is typically 4096 bytes on macOS
			return (freePages + inactivePages) * 4096
		}

	case "linux":
		data, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			var available uint64
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "MemAvailable:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
							available = kb * 1024
						}
					}
				}
			}
			if available > 0 {
				return available
			}
		}

	case "windows":
		out, err := exec.Command("wmic", "os", "get", "freephysicalmemory").Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && line != "FreePhysicalMemory" {
					if kb, err := strconv.ParseUint(line, 10, 64); err == nil {
						return kb * 1024
					}
				}
			}
		}
	}

	// fallback
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Sys - m.Alloc
}

func detectEnv() EnvInfo {
	return EnvInfo{
		Shell:    os.Getenv("SHELL"),
		Terminal: os.Getenv("TERM"),
		Home:     os.Getenv("HOME"),
		PWD:      os.Getenv("PWD"),
	}
}
