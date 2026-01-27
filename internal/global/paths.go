package global

import (
	"os"
	"path/filepath"
	"runtime"
)

func DefaultDBPath() string {
	if v := os.Getenv("DEVORCH_DB_PATH"); v != "" {
		return v
	}
	return filepath.Join(".", "devorch.db")
}

func HomeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return "."
}

// Step2: 최소 OS별 디폴트 data dir (추후 XDG/Windows KnownFolder로 고도화)
func DataDir() string {
	if v := os.Getenv("DEVORCH_DATA_DIR"); v != "" {
		return v
	}
	home := HomeDir()
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "devorch")
		}
		return filepath.Join(home, "AppData", "Roaming", "devorch")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "devorch")
	default:
		// linux and others
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "devorch")
		}
		return filepath.Join(home, ".local", "share", "devorch")
	}
}

func RuntimeDir() string {
	if v := os.Getenv("DEVORCH_RUNTIME_DIR"); v != "" {
		return v
	}
	return filepath.Join(DataDir(), "runtime")
}

func EnsureDirs() error {
	dirs := []string{
		DataDir(),
		RuntimeDir(),
		filepath.Join(DataDir(), "state"),
		filepath.Join(DataDir(), "log"),
		filepath.Join(DataDir(), "cache"),
		filepath.Join(DataDir(), "models"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
