package config

import (
	"os"
	"path/filepath"

	"devorch/internal/global"
)

func Load() (Config, error) {
	_ = global.EnsureDirs()

	dataDir := global.DataDir()
	rtDir := global.RuntimeDir()

	modelsDir := os.Getenv("DEVORCH_MODELS_DIR")
	if modelsDir == "" {
		modelsDir = filepath.Join(dataDir, "models")
	}

	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = os.Getenv("DEVORCH_OLLAMA_HOST")
	}
	if ollamaHost == "" {
		ollamaHost = "http://127.0.0.1:11434"
	}

	offline := os.Getenv("DEVORCH_OFFLINE") == "1"
	autoInstall := os.Getenv("DEVORCH_AUTO_INSTALL") != "0" // default on

	return Config{
		DBPath:        global.DefaultDBPath(),
		DaemonAddr:    os.Getenv("DEVORCH_DAEMON_ADDR"),
		OpenAIKey:     os.Getenv("OPENAI_API_KEY"),
		OpenRouterKey: os.Getenv("OPENROUTER_API_KEY"),

		Local: LocalRuntimeConfig{
			OfflineMode:      offline,
			AllowAutoInstall: autoInstall,
			RuntimeDir:       rtDir,
			ModelsDir:        modelsDir,
		},
		Ollama: OllamaConfig{
			Host:           ollamaHost,
			AutoStart:      os.Getenv("DEVORCH_OLLAMA_AUTOSTART") != "0", // default on
			BundleOverride: os.Getenv("DEVORCH_OLLAMA_BUNDLE"),
		},
	}, nil
}
