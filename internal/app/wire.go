package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"devorch/internal/config"
	"devorch/internal/global"
	okaonsqlite "devorch/internal/okaon/sqlite"
	"devorch/internal/provider"
	"devorch/internal/provider/anthropic"
	"devorch/internal/provider/azure"
	"devorch/internal/provider/bedrock"
	"devorch/internal/provider/cohere"
	"devorch/internal/provider/copilot"
	"devorch/internal/provider/gitlab"
	"devorch/internal/provider/groq"
	"devorch/internal/provider/mistral"
	"devorch/internal/provider/ollama"
	"devorch/internal/provider/openai"
	"devorch/internal/provider/openrouter"
	"devorch/internal/provider/perplexity"
	"devorch/internal/provider/together"
	"devorch/internal/provider/xai"
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

	// OpenAI
	if cfg.OpenAIKey != "" {
		reg.Register(openai.New(cfg.OpenAIKey))
	}

	// OpenRouter
	if cfg.OpenRouterKey != "" {
		reg.Register(openrouter.New(cfg.OpenRouterKey))
	}

	// Groq (fast inference)
	if groqKey := os.Getenv("GROQ_API_KEY"); groqKey != "" {
		reg.Register(groq.New(groqKey))
	}

	// Together AI
	if togetherKey := os.Getenv("TOGETHER_API_KEY"); togetherKey != "" {
		reg.Register(together.New(togetherKey))
	}

	// Mistral AI
	if mistralKey := os.Getenv("MISTRAL_API_KEY"); mistralKey != "" {
		reg.Register(mistral.New(mistralKey))
	}

	// Azure OpenAI
	if azureEndpoint := os.Getenv("AZURE_OPENAI_ENDPOINT"); azureEndpoint != "" {
		azureKey := os.Getenv("AZURE_OPENAI_API_KEY")
		azureDeployment := os.Getenv("AZURE_OPENAI_DEPLOYMENT")
		if azureKey != "" && azureDeployment != "" {
			reg.Register(azure.New(azure.Config{
				Endpoint:   azureEndpoint,
				APIKey:     azureKey,
				Deployment: azureDeployment,
			}))
		}
	}

	// Anthropic Claude
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		reg.Register(anthropic.New())
	}

	// AWS Bedrock
	if os.Getenv("AWS_ACCESS_KEY_ID") != "" || os.Getenv("AWS_PROFILE") != "" {
		reg.Register(bedrock.New())
	}

	// Perplexity AI
	if os.Getenv("PERPLEXITY_API_KEY") != "" {
		reg.Register(perplexity.New())
	}

	// xAI (Grok)
	if os.Getenv("XAI_API_KEY") != "" {
		reg.Register(xai.New())
	}

	// GitHub Copilot
	if os.Getenv("GITHUB_TOKEN") != "" || os.Getenv("COPILOT_TOKEN") != "" {
		reg.Register(copilot.New())
	}

	// Cohere
	if os.Getenv("COHERE_API_KEY") != "" {
		reg.Register(cohere.New())
	}

	// GitLab AI
	if os.Getenv("GITLAB_TOKEN") != "" || os.Getenv("CI_JOB_TOKEN") != "" {
		reg.Register(gitlab.New())
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
