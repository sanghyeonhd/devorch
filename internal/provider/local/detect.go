package local

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type RuntimeKind string

const (
	RuntimeOllama RuntimeKind = "ollama"
)

type DetectResult struct {
	Kind         RuntimeKind
	Found        bool
	BinaryPath   string
	Version      string
	Host         string // for daemon-based runtime
	BundleBinary string // bundled runtime candidate
}

func LookPath(name string) (string, bool) {
	p, err := exec.LookPath(name)
	return p, err == nil && p != ""
}

func FileExists(p string) bool {
	if p == "" {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func CandidateBundledBinary(runtimeDir string, exeName string) string {
	// runtimeDir/llm/ollama/<os-arch>/ollama(.exe)
	osarch := runtime.GOOS + "-" + runtime.GOARCH
	return filepath.Join(runtimeDir, "llm", "ollama", osarch, exeName)
}
