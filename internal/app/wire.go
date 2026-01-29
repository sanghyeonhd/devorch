package app

import (
	"context"
	"path/filepath"
	"runtime"
	"time"

	"devorch/internal/config"
	"devorch/internal/global"
	okaonsqlite "devorch/internal/okaon/sqlite"
	"devorch/internal/provider"
	"devorch/internal/provider/ollama"
	"devorch/internal/provider/openai"
	"devorch/internal/provider/openrouter"
	"devorch/internal/router"
	"devorch/internal/session"
	"devorch/internal/storage/sqlite"
)

func WireMinimal(cfg config.Config) (*Deps, error) {
	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	reg := provider.NewRegistry()
	if cfg.OpenAIKey != "" {
		reg.Register(openai.New(cfg.OpenAIKey))
	}
	if cfg.OpenRouterKey != "" {
		reg.Register(openrouter.New(cfg.OpenRouterKey))
	}

	okStore := okaonsqlite.NewStore(db.SQL)

	// Step2: ensure ollama local provider
	if !cfg.Local.OfflineMode {
		// even in offline, local ollama can exist; we still attempt below.
	}

	ollamaBin := cfg.Ollama.BundleOverride
	if ollamaBin == "" {
		ollamaBin = bundledOllamaPath(cfg.Local.RuntimeDir)
	}

	// AutoStart: if host not healthy, try start bundled
	if cfg.Ollama.AutoStart && ollamaBin != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if err := ollama.Health(ctx, cfg.Ollama.Host); err != nil {
			_ = global.EnsureDirs()
			stateDir := filepath.Join(global.DataDir(), "state")
			_ = ollama.StartBundled(ctx, ollamaBin, cfg.Ollama.Host, stateDir)
		}
	}

	reg.Register(ollama.New(cfg.Ollama.Host, ollamaBin))

	enricher := &router.OkAONEnricher{Store: okStore}
	rt := router.New(router.DefaultWeights(), router.NewEpsilonGreedy(), enricher)

	return &Deps{
		DB:               db,
		ProviderRegistry: reg,
		OkAONStore:       okStore,
		Router:           rt,
		SessionCaller:    session.NewCaller(),
	}, nil
}

func bundledOllamaPath(runtimeDir string) string {
	exe := "ollama"
	if runtime.GOOS == "windows" {
		exe = "ollama.exe"
	}
	osarch := runtime.GOOS + "-" + runtime.GOARCH
	return filepath.Join(runtimeDir, "llm", "ollama", osarch, exe)
}
