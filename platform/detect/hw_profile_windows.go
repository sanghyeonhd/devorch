//go:build windows

// Package platform provides hardware profile detection for Windows.
// Phase 33: Windows-specific HW detection
package platform

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// DetectHWProfile detects hardware profile on Windows.
func DetectHWProfile() (HWProfile, error) {
	p := HWProfile{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCount: runtime.NumCPU(),
	}

	// Memory: wmic computersystem get TotalPhysicalMemory
	if out, err := exec.Command("wmic", "computersystem", "get", "TotalPhysicalMemory").CombinedOutput(); err == nil {
		p.MemTotalMB = parseWMICBytesToMB(string(out))
	}

	// Disk: PowerShell Get-PSDrive C
	ps := `$d=Get-PSDrive -Name C; Write-Output ($d.Used+$d.Free); Write-Output $d.Free`
	if out, err := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps).CombinedOutput(); err == nil {
		parsePSDrive(&p, string(out))
	}

	// Acceleration: nvidia-smi (CUDA) or DirectML availability
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		p.HasAccel = true
		p.AccelKind = "cuda"
	} else {
		// DirectML is generally available on Windows 10+ with compatible GPU
		// Check for dxdiag or assume DirectML availability on modern Windows
		p.HasAccel = false
		p.AccelKind = ""
	}

	return p, nil
}

func parseWMICBytesToMB(out string) int64 {
	lines := strings.Split(out, "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.Contains(strings.ToLower(ln), "totalphysicalmemory") {
			continue
		}
		b, err := strconv.ParseInt(ln, 10, 64)
		if err != nil || b <= 0 {
			continue
		}
		return b / (1024 * 1024)
	}
	return 0
}

func parsePSDrive(p *HWProfile, out string) {
	// Output: totalBytes \n freeBytes
	lines := strings.Fields(out)
	if len(lines) < 2 {
		return
	}
	total, _ := strconv.ParseInt(lines[0], 10, 64)
	free, _ := strconv.ParseInt(lines[1], 10, 64)
	if total > 0 {
		p.DiskTotalGB = total / (1024 * 1024 * 1024)
	}
	if free > 0 {
		p.DiskFreeGB = free / (1024 * 1024 * 1024)
	}
}
