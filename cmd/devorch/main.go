package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"devorch/internal/app"
	"devorch/internal/cli"
	"devorch/internal/config"
	"devorch/internal/log"
	"devorch/internal/provider"
	"devorch/internal/router"
)

func usage() {
	fmt.Println(`devorch - local-first multi-LLM orchestrator (Phase 1-40 Complete)

Usage:
  devorch doctor
  devorch providers
  devorch models --provider openai|openrouter|ollama
  devorch ollama-pull --model <model>
  devorch chat --provider openai|openrouter|ollama --model <model> --prompt "hi"
  
  devorch login --provider github|google|openai|anthropic
  devorch login-status --provider github|google|openai|anthropic
  devorch logout --provider github|google|openai|anthropic
  
  devorch bench --provider ollama --model llama3.2 --iterations 5
  devorch stats --provider ollama --model llama3.2

Env:
  DEVORCH_DB_PATH=./devorch.db
  DEVORCH_OFFLINE=1
  DEVORCH_AUTO_INSTALL=1
  DEVORCH_DAEMON_ADDR=127.0.0.1:8787
  DEVORCH_OLLAMA_HOST=http://127.0.0.1:11434
  DEVORCH_OLLAMA_AUTOSTART=1
  DEVORCH_OLLAMA_BUNDLE=<path to ollama binary override>

  OPENAI_API_KEY=...
  OPENROUTER_API_KEY=...
  ANTHROPIC_API_KEY=...
  GOOGLE_API_KEY=...`)
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

	// Commands that don't need full wire setup
	cmd := os.Args[1]
	daemonAddr := cfg.DaemonAddr
	if daemonAddr == "" {
		daemonAddr = "http://127.0.0.1:8787"
	}

	switch cmd {
	case "login":
		fs := flag.NewFlagSet("login", flag.ExitOnError)
		provName := fs.String("provider", "", "github|google|openai|anthropic")
		_ = fs.Parse(os.Args[2:])
		if *provName == "" {
			fmt.Println("missing --provider")
			os.Exit(2)
		}
		if err := cli.Login(daemonAddr, *provName); err != nil {
			log.Errorf("login failed: %v", err)
			os.Exit(1)
		}
		return

	case "login-status":
		fs := flag.NewFlagSet("login-status", flag.ExitOnError)
		provName := fs.String("provider", "", "github|google|openai|anthropic")
		_ = fs.Parse(os.Args[2:])
		if *provName == "" {
			fmt.Println("missing --provider")
			os.Exit(2)
		}
		if err := cli.LoginStatus(daemonAddr, *provName); err != nil {
			log.Errorf("login-status failed: %v", err)
			os.Exit(1)
		}
		return

	case "logout":
		fs := flag.NewFlagSet("logout", flag.ExitOnError)
		provName := fs.String("provider", "", "github|google|openai|anthropic")
		_ = fs.Parse(os.Args[2:])
		if *provName == "" {
			fmt.Println("missing --provider")
			os.Exit(2)
		}
		if err := cli.Logout(daemonAddr, *provName); err != nil {
			log.Errorf("logout failed: %v", err)
			os.Exit(1)
		}
		return
	}

	// Commands that need wire setup
	deps, err := app.WireMinimal(cfg)
	if err != nil {
		log.Errorf("wire failed: %v", err)
		os.Exit(1)
	}
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

	case "bench":
		fs := flag.NewFlagSet("bench", flag.ExitOnError)
		provName := fs.String("provider", "ollama", "provider name")
		model := fs.String("model", "", "model id")
		iterations := fs.Int("iterations", 3, "number of iterations")
		_ = fs.Parse(os.Args[2:])
		if *model == "" {
			fmt.Println("missing --model")
			os.Exit(2)
		}

		p, ok := deps.ProviderRegistry.GetProvider(*provName)
		if !ok {
			fmt.Printf("provider not found: %s\n", *provName)
			os.Exit(2)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		fmt.Printf("Running benchmark: provider=%s model=%s iterations=%d\n", *provName, *model, *iterations)

		var totalLatency int64
		var successes int
		for i := 0; i < *iterations; i++ {
			req := provider.ChatRequest{
				Model: *model,
				Messages: []provider.ChatMessage{
					{Role: "user", Content: "Say 'Hello' in one word."},
				},
				Temperature: 0.0,
				MaxTokens:   10,
			}
			start := time.Now()
			_, err := p.Chat(ctx, req)
			latency := time.Since(start).Milliseconds()
			totalLatency += latency
			if err != nil {
				fmt.Printf("  [%d/%d] FAIL: %v (latency: %dms)\n", i+1, *iterations, err, latency)
			} else {
				successes++
				fmt.Printf("  [%d/%d] OK (latency: %dms)\n", i+1, *iterations, latency)
			}
		}
		avgLatency := totalLatency / int64(*iterations)
		successRate := float64(successes) / float64(*iterations) * 100
		fmt.Printf("\nSummary: avg_latency=%dms success_rate=%.1f%%\n", avgLatency, successRate)

	case "stats":
		fs := flag.NewFlagSet("stats", flag.ExitOnError)
		provName := fs.String("provider", "", "provider name (optional)")
		model := fs.String("model", "", "model id (optional)")
		limit := fs.Int("limit", 20, "max records to show")
		_ = fs.Parse(os.Args[2:])

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Get stats from OkAON store
		runs, err := deps.OkAONStore.QueryRuns(ctx, "", "", *limit)
		if err != nil {
			log.Errorf("stats failed: %v", err)
			os.Exit(1)
		}

		if len(runs) == 0 {
			fmt.Println("No runs recorded yet. Use 'devorch chat' to record runs.")
			return
		}

		fmt.Println("Recent OkAON Runs:")
		fmt.Println("------------------")
		for _, r := range runs {
			if *provName != "" && r.Provider != *provName {
				continue
			}
			if *model != "" && r.Model != *model {
				continue
			}
			status := "OK"
			if !r.Success {
				status = "FAIL"
			}
			fmt.Printf("[%s] %s/%s latency=%dms quality=%.2f cost=$%.6f\n",
				status, r.Provider, r.Model, r.LatencyMs, r.Quality, float64(r.CostMicroUSD)/1e6)
		}

	default:
		usage()
		os.Exit(2)
	}
}
