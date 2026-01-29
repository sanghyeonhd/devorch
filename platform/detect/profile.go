package platform

import (
	"runtime"
)

// Profile은 플랫폼 프로필입니다
type Profile struct {
	OS        string   `json:"os"`
	Arch      string   `json:"arch"`
	NumCPU    int      `json:"num_cpu"`
	GoVersion string   `json:"go_version"`
	Memory    MemInfo  `json:"memory"`
	GPU       *GPUInfo `json:"gpu,omitempty"`
	Env       EnvInfo  `json:"env"`
}

// MemInfo는 메모리 정보입니다
type MemInfo struct {
	TotalBytes uint64 `json:"total_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
}

// GPUInfo는 GPU 정보입니다
type GPUInfo struct {
	Available bool     `json:"available"`
	Devices   []GPUDev `json:"devices,omitempty"`
}

// GPUDev는 GPU 장치 정보입니다
type GPUDev struct {
	Name        string `json:"name"`
	MemoryTotal uint64 `json:"memory_total"`
	MemoryFree  uint64 `json:"memory_free"`
	Vendor      string `json:"vendor,omitempty"`
}

// EnvInfo는 환경 정보입니다
type EnvInfo struct {
	Shell    string `json:"shell,omitempty"`
	Terminal string `json:"terminal,omitempty"`
	Home     string `json:"home,omitempty"`
	PWD      string `json:"pwd,omitempty"`
}

// Detect는 현재 플랫폼 프로필을 감지합니다
func Detect() Profile {
	profile := Profile{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		NumCPU:    runtime.NumCPU(),
		GoVersion: runtime.Version(),
	}

	profile.Memory = detectMemory()
	profile.GPU = detectGPU()
	profile.Env = detectEnv()

	return profile
}

// IsDarwin은 macOS인지 확인합니다
func (p Profile) IsDarwin() bool {
	return p.OS == "darwin"
}

// IsLinux는 Linux인지 확인합니다
func (p Profile) IsLinux() bool {
	return p.OS == "linux"
}

// IsWindows는 Windows인지 확인합니다
func (p Profile) IsWindows() bool {
	return p.OS == "windows"
}

// IsARM64는 ARM64 아키텍처인지 확인합니다
func (p Profile) IsARM64() bool {
	return p.Arch == "arm64"
}

// IsAMD64는 x86_64 아키텍처인지 확인합니다
func (p Profile) IsAMD64() bool {
	return p.Arch == "amd64"
}

// HasGPU는 GPU가 있는지 확인합니다
func (p Profile) HasGPU() bool {
	return p.GPU != nil && p.GPU.Available
}

// GPUCount는 GPU 개수를 반환합니다
func (p Profile) GPUCount() int {
	if p.GPU == nil {
		return 0
	}
	return len(p.GPU.Devices)
}

// MemoryGB는 총 메모리를 GB로 반환합니다
func (p Profile) MemoryGB() float64 {
	return float64(p.Memory.TotalBytes) / (1024 * 1024 * 1024)
}
