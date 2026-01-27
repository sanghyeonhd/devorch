package local

import (
	"runtime"
)

type HWInfo struct {
	OS      string
	Arch    string
	Cores   int
	MemMB   int // Step2: 보수적으로 0(추후 OS별 구현로 확장)
	GPUHint string
}

func DetectHW() HWInfo {
	return HWInfo{
		OS:    runtime.GOOS,
		Arch:  runtime.GOARCH,
		Cores: runtime.NumCPU(),
		MemMB: 0,
	}
}
