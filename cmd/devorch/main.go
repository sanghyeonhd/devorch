package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"devorch/internal/app"
	"devorch/internal/config"
	"devorch/internal/log"
	"devorch/internal/provider"
	"devorch/internal/router"
)

func usage() {
	fmt.Println(`devorch - local-first multi-LLM orchestrator (Step2)

Usage:
  devorch doctor
  devorch providers
  devorch models --provider openai|openrouter|ollama
  devorch ollama-pull --model <model>
  devorch chat --provider openai|openrouter|ollama --model <model> --prompt "hi"

Env:
  DEVORCH_DB_PATH=./devorch.db
  DEVORCH_OFFLINE=1
  DEVORCH_AUTO_INSTALL=1
  DEVORCH_OLLAMA_HOST=http://127.0.0.1:11434
  DEVORCH_OLLAMA_AUTOSTART=1
  DEVORCH_OLLAMA_BUNDLE=<path to ollama binary override>

  OPENAI_API_KEY=...
  OPENROUTER_API_KEY=...
`)
}

func main() {
	log.Init()

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Errorf("config load failed: %v", err)
		os.Exit(1)
	}

	deps, err := app.WireMinimal(cfg)
	if err != nil {
		log.Errorf("wire failed: %v", err)
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "doctor":
		// doctor is implemented in diagnostics (step1)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		// simple call via provider health (wire already registered)
		for _, pinfo := range deps.ProviderRegistry.List() {
			p, ok := deps.ProviderRegistry.GetProvider(pinfo.Name)
			if !ok {
				continue
			}
			if err := p.Health(ctx); err != nil {
				log.Errorf("provider %s health failed: %v", pinfo.Name, err)
				os.Exit(1)
			}
		}
		fmt.Println("OK")

	case "providers":
		for _, p := range deps.ProviderRegistry.List() {
			fmt.Printf("- %s (%s)\n", p.Name, p.Kind)
		}

	case "models":
		fs := flag.NewFlagSet("models", flag.ExitOnError)
		provName := fs.String("provider", "", "openai|openrouter|ollama")
		_ = fs.Parse(os.Args[2:])
		if *provName == "" {
			fmt.Println("missing --provider")
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		p, ok := deps.ProviderRegistry.GetProvider(*provName)
		if !ok {
			fmt.Printf("provider not found: %s\n", *provName)
			os.Exit(2)
		}
		models, err := p.ListModels(ctx)
		if err != nil {
			log.Errorf("list models failed: %v", err)
			os.Exit(1)
		}
		for _, m := range models {
			fmt.Printf("- %s\n", m.ID)
		}

	case "ollama-pull":
		fs := flag.NewFlagSet("ollama-pull", flag.ExitOnError)
		model := fs.String("model", "", "ollama model name (e.g. llama3.1:8b)")
		_ = fs.Parse(os.Args[2:])
		if *model == "" {
			fmt.Println("missing --model")
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 0) // pull can be long
		defer cancel()

		p, ok := deps.ProviderRegistry.GetProvider("ollama")
		if !ok {
			fmt.Println("ollama provider not registered")
			os.Exit(1)
		}
		// pull is optional interface; use type assertion
		type puller interface {
			Pull(ctx context.Context, model string) error
		}
		if pl, ok := p.(puller); ok {
			if err := pl.Pull(ctx, *model); err != nil {
				log.Errorf("ollama pull failed: %v", err)
				os.Exit(1)
			}
			fmt.Println("OK")
		} else {
			fmt.Println("ollama pull not supported")
			os.Exit(1)
		}

	case "chat":
		fs := flag.NewFlagSet("chat", flag.ExitOnError)
		provName := fs.String("provider", "", "openai|openrouter|ollama")
		model := fs.String("model", "", "model id")
		prompt := fs.String("prompt", "", "prompt text")
		_ = fs.Parse(os.Args[2:])
		if *provName == "" || *model == "" || *prompt == "" {
			fmt.Println("missing --provider/--model/--prompt")
			os.Exit(2)
		}

		p, ok := deps.ProviderRegistry.GetProvider(*provName)
		if !ok {
			fmt.Printf("provider not found: %s\n", *provName)
			os.Exit(2)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		req := provider.ChatRequest{
			Model: *model,
			Messages: []provider.ChatMessage{
				{Role: "user", Content: *prompt},
			},
			Temperature: 0.2,
		}

		wl := router.Workload{
			WorkspaceID: "local",
			UserID:      "local",
			ProjectID:   "adhoc",
			AgentType:   "cli",
			Category:    "quick",
			TaskType:    "chat",
		}

		// Step2: candidates를 "여러 provider 조합"으로 넣을 수 있게 구조 유지
		// 현재는 CLI에서 지정한 provider/model을 기본 후보로 넣되,
		// 오토라우팅 모드(다음 단계)에서 candidates가 확장될 예정
		cands := []router.Candidate{
			{Provider: p.Name(), Model: *model, Endpoint: p.Endpoint()},
		}

		resp, err := deps.SessionCaller.CallAndRecord(ctx, deps.ProviderRegistry, deps.Router, deps.OkAONStore, wl, cands, req)
		if err != nil {
			log.Errorf("chat failed: %v", err)
			os.Exit(1)
		}
		fmt.Println(resp.Text())

	default:
		usage()
		os.Exit(2)
	}
}
