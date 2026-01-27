package ollama

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// StartBundled starts bundled ollama binary in background if needed.
func StartBundled(ctx context.Context, binaryPath string, host string, stateDir string) error {
	if binaryPath == "" {
		return fmt.Errorf("empty binaryPath")
	}
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("ollama binary missing: %w", err)
	}
	_ = os.MkdirAll(stateDir, 0o755)

	// pid file best-effort (step2 minimal)
	pidFile := filepath.Join(stateDir, "ollama.pid")

	// env: OLLAMA_HOST
	cmd := exec.CommandContext(ctx, binaryPath, "serve")
	cmd.Env = append(os.Environ(),
		"OLLAMA_HOST="+stripScheme(host),
	)

	// detach-ish: start without waiting
	if runtime.GOOS == "windows" {
		// windows: normal start is okay for step2
	} else {
		// unix: keep default
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	// write pid
	_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0o644)

	// wait until healthy (short)
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		if err := Health(waitCtx, host); err == nil {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("ollama serve timeout")
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func stripScheme(host string) string {
	// Ollama expects "127.0.0.1:11434" style in env
	if len(host) >= 7 && host[:7] == "http://" {
		return host[7:]
	}
	if len(host) >= 8 && host[:8] == "https://" {
		return host[8:]
	}
	return host
}
