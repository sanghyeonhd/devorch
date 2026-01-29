//go:build linux

// Package platform provides hardware profile detection for Linux.
// Phase 33: Linux-specific HW detection
package platform

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// DetectHWProfile detects hardware profile on Linux.
func DetectHWProfile() (HWProfile, error) {
	p := HWProfile{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCount: runtime.NumCPU(),
	}

	// Memory: /proc/meminfo MemTotal: kB
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		p.MemTotalMB = parseMeminfoMB(b)
	}

	// Disk: df -k /
	dfOut, _ := exec.Command("df", "-k", "/").CombinedOutput()
	parseDFOutputLinux(&p, dfOut)

	// Acceleration: check for nvidia-smi (CUDA)
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		p.HasAccel = true
		p.AccelKind = "cuda"
	} else if _, err := exec.LookPath("rocm-smi"); err == nil {
		// AMD ROCm
		p.HasAccel = true
		p.AccelKind = "rocm"
	}

	return p, nil
}

func parseMeminfoMB(b []byte) int64 {
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				return kb / 1024
			}
		}
	}
	return 0
}

func parseDFOutputLinux(p *HWProfile, out []byte) {
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
