package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// EnvFingerprint는 환경 핑거프린트입니다
type EnvFingerprint struct {
	Hash     string   `json:"hash"`
	Platform string   `json:"platform"`
	Features []string `json:"features"`
}

// GenerateFingerprint는 환경 핑거프린트를 생성합니다
func GenerateFingerprint() EnvFingerprint {
	profile := Detect()

	features := collectFeatures(profile)
	platformStr := fmt.Sprintf("%s-%s-%d", profile.OS, profile.Arch, profile.NumCPU)

	// 해시 생성
	hashInput := platformStr + strings.Join(features, ",")
	hash := sha256.Sum256([]byte(hashInput))

	return EnvFingerprint{
		Hash:     hex.EncodeToString(hash[:8]), // 16자 해시
		Platform: platformStr,
		Features: features,
	}
}

func collectFeatures(profile Profile) []string {
	var features []string

	// OS 특성
	switch profile.OS {
	case "darwin":
		features = append(features, "macos")
		if profile.IsARM64() {
			features = append(features, "apple-silicon")
		}
	case "linux":
		features = append(features, "linux")
		if isWSL() {
			features = append(features, "wsl")
		}
		if isContainer() {
			features = append(features, "container")
		}
	case "windows":
		features = append(features, "windows")
	}

	// 아키텍처
	features = append(features, profile.Arch)

	// CPU 수 범위
	if profile.NumCPU >= 8 {
		features = append(features, "high-cpu")
	} else if profile.NumCPU >= 4 {
		features = append(features, "mid-cpu")
	} else {
		features = append(features, "low-cpu")
	}

	// 메모리 범위
	memGB := profile.MemoryGB()
	if memGB >= 32 {
		features = append(features, "high-mem")
	} else if memGB >= 16 {
		features = append(features, "mid-mem")
	} else {
		features = append(features, "low-mem")
	}

	// GPU
	if profile.HasGPU() {
		features = append(features, "gpu")
	}

	return features
}

func isWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	// /proc/version에 WSL 또는 Microsoft 문자열 확인
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	content := strings.ToLower(string(data))
	return strings.Contains(content, "wsl") || strings.Contains(content, "microsoft")
}

func isContainer() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	// /.dockerenv 파일 존재 확인
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	// /proc/1/cgroup에서 docker/kubepods 확인
	data, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}
	content := string(data)
	return strings.Contains(content, "docker") || strings.Contains(content, "kubepods")
}
