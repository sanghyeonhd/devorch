package platform

import (
	"os"
	"runtime"
)

func detectMemory() MemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Go runtime 메모리 통계 (실제 시스템 메모리와 다름)
	// 플랫폼별 실제 메모리 감지는 별도 구현 필요
	return MemInfo{
		TotalBytes: m.Sys,
		UsedBytes:  m.Alloc,
		FreeBytes:  m.Sys - m.Alloc,
	}
}

func detectEnv() EnvInfo {
	return EnvInfo{
		Shell:    os.Getenv("SHELL"),
		Terminal: os.Getenv("TERM"),
		Home:     os.Getenv("HOME"),
		PWD:      os.Getenv("PWD"),
	}
}
