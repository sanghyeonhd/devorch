package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"devorch/internal/app"
	"devorch/internal/autosetup"
	"devorch/internal/cli"
	"devorch/internal/config"
	"devorch/internal/global"
	"devorch/internal/log"
	"devorch/internal/provider"
	"devorch/internal/router"
	"devorch/internal/tui"
)

func usage() {
	fmt.Println(`devorch - local-first multi-LLM orchestrator

Usage:
  devorch                   # Start interactive TUI mode
  devorch doctor            # Check system health
  devorch providers         # List available providers
  devorch models --provider <name>
  devorch ollama-pull --model <model>
  devorch chat --provider <name> --model <model> --prompt "..."
  
  devorch connect           # Interactive provider setup
  devorch login --provider github|google|openai|anthropic
  devorch login-status --provider github|google|openai|anthropic
  devorch logout --provider github|google|openai|anthropic
  
  devorch bench --provider ollama --model llama3.2 --iterations 5
  devorch stats --provider ollama --model llama3.2
  devorch setup             # Auto-detect hardware and setup optimal model

Env:
  DEVORCH_DB_PATH=./devorch.db
  DEVORCH_OFFLINE=1
  DEVORCH_AUTO_INSTALL=1
  DEVORCH_DAEMON_ADDR=127.0.0.1:8787
  DEVORCH_OLLAMA_HOST=http://127.0.0.1:11434
  DEVORCH_OLLAMA_AUTOSTART=1

  OPENAI_API_KEY=...
  OPENROUTER_API_KEY=...
  ANTHROPIC_API_KEY=...
  GOOGLE_API_KEY=...`)
}

func main() {
	log.Init()

	// No args: start TUI mode
	if len(os.Args) < 2 {
		startTUI()
		return
	}

	// Handle --help and -h
	if os.Args[1] == "--help" || os.Args[1] == "-h" {
		usage()
		return
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
	case "setup":
		runAutoSetup()
		return

	case "connect":
		// Interactive provider connection (like OpenCode's /connect)
		runConnect()
		return

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

// startTUI starts the interactive TUI mode
func startTUI() {
	// Show welcome banner
	theme := tui.DefaultTheme()

	profile, err := global.GetHWProfile()
	if err != nil {
		tui.PrintError(err, theme)
		os.Exit(1)
	}

	// Display system info banner
	fmt.Println()
	fmt.Println(theme.Title.Render("  🚀 DevOrch - AI Coding Agent  "))
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	fmt.Printf("  %s %s/%s | %dGB RAM | %s\n",
		theme.Subtle.Render("System:"),
		profile.OS, profile.Arch,
		profile.MemTotalMB/1024,
		func() string {
			if profile.HasAccel {
				return profile.AccelKind + " GPU"
			}
			return "No GPU"
		}())
	fmt.Printf("  %s %s (auto-recommended)\n",
		theme.Subtle.Render("Model Size:"),
		theme.Accent.Render(profile.RecommendedModelSize()))
	fmt.Printf("  %s %s\n",
		theme.Subtle.Render("Tier:"),
		theme.Success.Render(profile.Tier()))
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	// Start interactive TUI
	if err := tui.RunInteractive(""); err != nil {
		tui.PrintError(err, theme)
		os.Exit(1)
	}
}

// runAutoSetup runs the automatic hardware detection and model setup
func runAutoSetup() {
	theme := tui.DefaultTheme()

	fmt.Println()
	fmt.Println(theme.Title.Render("  🔧 DevOrch Auto Setup  "))
	fmt.Println()

	result, err := autosetup.Run()
	if err != nil {
		tui.PrintError(err, theme)
		os.Exit(1)
	}

	fmt.Println(theme.Success.Render("✓ Setup complete!"))
	fmt.Printf("  Recommended model: %s\n", result.RecommendedModel)
	fmt.Printf("  System tier: %s\n", result.Tier)
	if result.OllamaInstalled {
		fmt.Println(theme.Success.Render("  ✓ Ollama ready"))
	}
	if result.ModelInstalled {
		fmt.Println(theme.Success.Render("  ✓ Model installed"))
	}
	fmt.Println()
	fmt.Println("Run 'devorch' to start the interactive session.")
}

// runConnect handles interactive provider connection
func runConnect() {
	theme := tui.DefaultTheme()

	fmt.Println()
	fmt.Println(theme.Title.Render("  🔗 DevOrch Connect  "))
	fmt.Println()
	fmt.Println("Available providers:")
	fmt.Println("  1. ollama      (local) - Free, runs on your machine")
	fmt.Println("  2. openai      (cloud) - Requires API key")
	fmt.Println("  3. anthropic   (cloud) - Requires API key")
	fmt.Println("  4. openrouter  (cloud) - Requires API key")
	fmt.Println("  5. google      (cloud) - Requires API key")
	fmt.Println()

	fmt.Print("Select provider (1-5) or name: ")
	var input string
	fmt.Scanln(&input)

	var providerName string
	switch input {
	case "1", "ollama":
		providerName = "ollama"
		fmt.Println()
		fmt.Println(theme.Success.Render("✓ Ollama is a local provider - no authentication needed."))
		fmt.Println("  Make sure Ollama is running: ollama serve")
		fmt.Println("  Install a model: ollama pull llama3.2:7b")
		return
	case "2", "openai":
		providerName = "openai"
	case "3", "anthropic":
		providerName = "anthropic"
	case "4", "openrouter":
		providerName = "openrouter"
	case "5", "google":
		providerName = "google"
	default:
		providerName = input
	}

	fmt.Println()
	fmt.Printf("To configure %s, set the API key:\n", providerName)
	fmt.Println()

	switch providerName {
	case "openai":
		fmt.Println("  export OPENAI_API_KEY=sk-...")
		fmt.Println("  Get your key at: https://platform.openai.com/api-keys")
	case "anthropic":
		fmt.Println("  export ANTHROPIC_API_KEY=sk-ant-...")
		fmt.Println("  Get your key at: https://console.anthropic.com/")
	case "openrouter":
		fmt.Println("  export OPENROUTER_API_KEY=sk-or-...")
		fmt.Println("  Get your key at: https://openrouter.ai/keys")
	case "google":
		fmt.Println("  export GOOGLE_API_KEY=...")
		fmt.Println("  Get your key at: https://aistudio.google.com/app/apikey")
	default:
		fmt.Printf("  Unknown provider: %s\n", providerName)
	}
	fmt.Println()
	fmt.Println("After setting the key, restart devorch to use the provider.")
}
