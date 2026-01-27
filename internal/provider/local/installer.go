package local

import (
	"fmt"
	"os"
	"path/filepath"
)

type Installer struct {
	RuntimeDir string
}

func NewInstaller(runtimeDir string) *Installer {
	return &Installer{RuntimeDir: runtimeDir}
}

// Step2: "런타임 번들"이 이미 runtime/llm/ollama/... 형태로 존재한다고 가정
// => 설치는 곧 "해당 바이너리를 실행 가능하게 보장(권한/경로)"하는 작업
func (i *Installer) EnsureBundledExecutable(src string) (string, error) {
	if src == "" {
		return "", fmt.Errorf("empty bundled path")
	}
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("bundled runtime missing: %w", err)
	}
	// In place 사용. (추후: RuntimeDir로 복사/버전별 관리 가능)
	// mac/linux: chmod +x
	_ = os.Chmod(src, 0o755)
	return filepath.Clean(src), nil
}
