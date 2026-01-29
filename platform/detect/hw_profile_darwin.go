//go:build darwin

// Package platform provides hardware profile detection for macOS.
// Phase 33: macOS-specific HW detection
package platform

import (
	"bytes"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// DetectHWProfile detects hardware profile on macOS.
func DetectHWProfile() (HWProfile, error) {
	p := HWProfile{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCount: runtime.NumCPU(),
	}

	// Memory: sysctl hw.memsize (bytes)
	memBytes, _ := sysctlInt64("hw.memsize")
	if memBytes > 0 {
		p.MemTotalMB = memBytes / (1024 * 1024)
	}

	// Disk: df -k /
	dfOut, _ := exec.Command("df", "-k", "/").CombinedOutput()
	parseDFOutput(&p, dfOut)

	// Acceleration: Apple Silicon (arm64) has Metal support
	if runtime.GOARCH == "arm64" {
		p.HasAccel = true
		p.AccelKind = "metal"
	}

	return p, nil
}

func sysctlInt64(key string) (int64, error) {
	out, err := exec.Command("sysctl", "-n", key).CombinedOutput()
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(out))
	return strconv.ParseInt(s, 10, 64)
}

func parseDFOutput(p *HWProfile, out []byte) {
	// Format: Filesystem 1024-blocks Used Available Capacity Mounted on
	lines := bytes.Split(out, []byte("\n"))
	if len(lines) < 2 {
		return
	}
	fields := strings.Fields(string(lines[1]))
	if len(fields) < 5 {
		return
	}
	totalKB, _ := strconv.ParseInt(fields[1], 10, 64)
	freeKB, _ := strconv.ParseInt(fields[3], 10, 64)
	p.DiskTotalGB = totalKB / 1024 / 1024
	p.DiskFreeGB = freeKB / 1024 / 1024
}
