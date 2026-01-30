package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"devorch/internal/app"
	"devorch/internal/auth"
	"devorch/internal/autosetup"
	"devorch/internal/cli"
	"devorch/internal/config"
	"devorch/internal/global"
	"devorch/internal/log"
	"devorch/internal/mcp"
	"devorch/internal/provider"
	"devorch/internal/router"
	"devorch/internal/tui"
)

func usage() {
	fmt.Println(`devorch - local-first multi-LLM orchestrator

Usage:
  devorch                   # Start CLI mode (direct terminal, chat with LLM)
  devorch tui               # Start TUI mode (bubbletea with alt screen)
  
  devorch doctor            # Check system health
  devorch providers         # List available providers
  devorch models --provider <name>
  devorch ollama-pull --model <model>
  devorch chat --provider <name> --model <model> --prompt "..."
  
  devorch connect           # Interactive provider setup (OpenCode style)
  devorch connect-list      # List all providers with connection status
  devorch themes            # List available themes
  devorch theme <name>      # Preview a theme
  
  devorch login --provider github|google|openai|anthropic
  devorch login-status --provider github|google|openai|anthropic
  devorch logout --provider github|google|openai|anthropic
  
  devorch setup             # Auto-detect hardware and setup optimal model
  devorch install           # Install essential models for your hardware
  devorch bench --provider ollama --model llama3.2 --iterations 5
  devorch stats --provider ollama --model llama3.2
  devorch mcp               # Manage MCP servers
  
  devorch test-tui          # Test all TUI components without interactive mode
  devorch test-commands     # List all slash commands

CLI Mode Commands (inside devorch):
  /help                     Show available commands
  /exit                     Exit DevOrch
  /models                   List available models
  /model <name>             Switch model
  /connect                  Show provider status
  /themes                   List themes
  /clear                    Clear chat history

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

	// No args: start CLI mode (headless, direct terminal I/O)
	if len(os.Args) < 2 {
		startCLI()
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
	case "tui":
		// Start TUI mode (bubbletea with alt screen)
		startTUI()
		return

	case "test-commands":
		// Debug: list all slash commands
		runTestCommands()
		return

	case "test-tui":
		// Test all TUI components without interactive mode
		runTestTUI()
		return

	case "connect-list":
		// List all providers with connection status (OpenCode style)
		runConnectList()
		return

	case "themes":
		// List all available themes
		runListThemes()
		return

	case "theme":
		// Preview a specific theme
		themeName := ""
		if len(os.Args) > 2 {
			themeName = os.Args[2]
		}
		runPreviewTheme(themeName)
		return

	case "setup":
		runAutoSetup()
		return

	case "install":
		runInstallModels()
		return

	case "mcp":
		runMCPSetup()
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

// startCLI starts the CLI mode (headless, direct terminal I/O)
func startCLI() {
	if err := tui.RunCLI(); err != nil {
		theme := tui.DefaultTheme()
		tui.PrintError(err, theme)
		os.Exit(1)
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

	fmt.Println(theme.Success.Render("✓ Hardware detection complete!"))
	fmt.Printf("  System tier: %s\n", result.Tier)
	fmt.Printf("  Primary model: %s\n", result.RecommendedModel)
	fmt.Println()

	// Show all recommended models for this tier
	essentialModels := autosetup.GetEssentialModels(result.Tier)
	fmt.Println(theme.Accent.Render("📦 Essential Models for your system:"))
	for _, model := range essentialModels {
		if autosetup.IsModelInstalled(model) {
			fmt.Printf("  ✓ %s (installed)\n", model)
		} else {
			fmt.Printf("  ○ %s\n", model)
		}
	}
	fmt.Println()

	// Show MCP server recommendations
	fmt.Println(theme.Accent.Render("🔌 Essential MCP Servers:"))
	fmt.Println("  • filesystem   - File system operations")
	fmt.Println("  • github       - GitHub API (requires GITHUB_TOKEN)")
	fmt.Println("  • context7     - Library documentation")
	fmt.Println("  • fetch        - HTTP fetch operations")
	fmt.Println("  • memory       - Persistent memory storage")
	fmt.Println("  • sequential-thinking - Enhanced reasoning")
	fmt.Println()

	if result.OllamaInstalled {
		fmt.Println(theme.Success.Render("  ✓ Ollama ready"))
	} else {
		fmt.Println(theme.Warning.Render("  ⚠ Ollama not installed"))
		fmt.Println("    Install: curl -fsSL https://ollama.ai/install.sh | sh")
	}

	if result.ModelInstalled {
		fmt.Println(theme.Success.Render("  ✓ Primary model installed"))
	}
	fmt.Println()

	// Instructions for full setup
	fmt.Println(theme.Title.Render("📋 Quick Start Commands:"))
	fmt.Println()
	fmt.Println("  # Install essential models (recommended):")
	fmt.Println("  DEVORCH_AUTO_INSTALL=1 devorch setup")
	fmt.Println()
	fmt.Println("  # Or install manually:")
	for _, model := range essentialModels[:min(3, len(essentialModels))] {
		fmt.Printf("  ollama pull %s\n", model)
	}
	fmt.Println()
	fmt.Println("  # Configure MCP servers:")
	fmt.Println("  devorch mcp setup")
	fmt.Println()
	fmt.Println("Run 'devorch' to start the interactive session.")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// runInstallModels installs essential Ollama models
func runInstallModels() {
	theme := tui.DefaultTheme()

	fmt.Println()
	fmt.Println(theme.Title.Render("  📦 DevOrch Model Installer  "))
	fmt.Println()

	// Get hardware tier
	profile, err := global.GetHWProfile()
	tier := "mid"
	if err != nil {
		fmt.Printf("Warning: couldn't detect hardware: %v\n", err)
	} else {
		tier = profile.Tier()
	}

	fmt.Printf("Hardware tier: %s\n\n", tier)

	// Install essential models
	fmt.Println("Installing essential models for your system...")
	fmt.Println("This may take a while depending on your internet speed.")
	fmt.Println()

	if err := autosetup.InstallEssentialModels(true); err != nil {
		tui.PrintError(err, theme)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(theme.Success.Render("✓ Model installation complete!"))
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

// runMCPSetup runs MCP server auto-configuration
func runMCPSetup() {
	theme := tui.DefaultTheme()

	fmt.Println()
	fmt.Println(theme.Title.Render("  🔌 DevOrch MCP Setup  "))
	fmt.Println()

	// Check subcommand
	subCmd := "setup"
	if len(os.Args) > 2 {
		subCmd = os.Args[2]
	}

	manager := mcp.NewManager()
	configurator := mcp.NewAutoConfigurator(manager, mcp.DefaultAutoConfigOptions())

	switch subCmd {
	case "setup", "configure":
		env := configurator.DetectEnvironment()

		fmt.Println("🔍 Detecting environment...")
		fmt.Printf("  OS: %s (%s)\n", env.OS, env.Arch)
		fmt.Printf("  Node.js: %v", env.HasNode)
		if env.HasNode {
			fmt.Printf(" (%s)", env.NodeVersion)
		}
		fmt.Println()
		fmt.Printf("  NPX: %v\n", env.HasNPX)
		fmt.Println()

		// Show available API keys
		fmt.Println("🔑 API Keys detected:")
		apiKeys := []string{"GITHUB_TOKEN", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"}
		for _, key := range apiKeys {
			if env.AvailableAPIKeys[key] {
				fmt.Printf("  ✓ %s\n", key)
			} else {
				fmt.Printf("  ○ %s (not set)\n", key)
			}
		}
		fmt.Println()

		// Show essential servers
		essentialServers := mcp.GetEssentialMCPServers()
		fmt.Println(theme.Accent.Render("📦 Essential MCP Servers:"))
		for _, server := range essentialServers {
			status := "○"
			note := ""
			if server.RequiresAPI && server.APIKeyEnv != "" {
				if !env.AvailableAPIKeys[server.APIKeyEnv] {
					status = "⚠"
					note = fmt.Sprintf(" (requires %s)", server.APIKeyEnv)
				} else {
					status = "✓"
				}
			} else if server.URL != "" {
				status = "✓" // URL-based servers always work
			} else if env.HasNPX {
				status = "✓" // NPX-based servers work if NPX is available
			}
			fmt.Printf("  %s %-20s - %s%s\n", status, server.Name, server.Description, note)
		}
		fmt.Println()

		// Auto-configure
		fmt.Println("🔧 Configuring MCP servers...")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := configurator.QuickSetup(ctx)
		if err != nil {
			fmt.Printf("  ✗ Configuration failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println(theme.Success.Render("✓ MCP Configuration Complete!"))
		fmt.Printf("  Config saved to: %s\n", result.ConfigPath)
		fmt.Printf("  Configured: %d servers\n", len(result.Configured))
		if len(result.Skipped) > 0 {
			fmt.Printf("  Skipped: %d servers\n", len(result.Skipped))
		}
		fmt.Println()

		if len(result.Configured) > 0 {
			fmt.Println("Configured servers:")
			for _, name := range result.Configured {
				fmt.Printf("  ✓ %s\n", name)
			}
		}

		if len(result.Skipped) > 0 {
			fmt.Println()
			fmt.Println("Skipped servers (missing requirements):")
			for _, s := range result.Skipped {
				fmt.Printf("  ○ %s - %s\n", s.Name, s.Reason)
			}
		}

	case "list":
		fmt.Println("Available MCP servers:")
		servers := mcp.GetRecommendedServers()
		categories := mcp.GetServersByCategory()

		for cat, catServers := range categories {
			fmt.Printf("\n%s:\n", theme.Accent.Render(cat))
			for _, s := range catServers {
				apiNote := ""
				if s.RequiresAPI {
					apiNote = fmt.Sprintf(" [%s]", s.APIKeyEnv)
				}
				fmt.Printf("  • %-20s - %s%s\n", s.Name, s.Description, apiNote)
			}
		}
		fmt.Printf("\nTotal: %d servers available\n", len(servers))

	case "test":
		fmt.Println("Testing MCP server connections...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		results := configurator.TestAllServers(ctx)
		success := 0
		for name, err := range results {
			if err == nil {
				fmt.Printf("  ✓ %s - OK\n", name)
				success++
			} else {
				fmt.Printf("  ✗ %s - %v\n", name, err)
			}
		}
		fmt.Printf("\n%d/%d servers working\n", success, len(results))

	default:
		fmt.Println("Usage: devorch mcp <command>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  setup   - Auto-configure essential MCP servers")
		fmt.Println("  list    - List all available MCP servers")
		fmt.Println("  test    - Test MCP server connections")
	}
}

// runTestCommands tests the slash command system
func runTestCommands() {
	fmt.Println("=== DevOrch Slash Commands Test ===")
	fmt.Println()

	commands := tui.GetSlashCommands()
	fmt.Printf("Total commands: %d\n\n", len(commands))

	// Group by category
	categories := make(map[string][]tui.SlashCommand)
	for _, cmd := range commands {
		categories[cmd.Category] = append(categories[cmd.Category], cmd)
	}

	// Print by category
	catOrder := []string{"Session", "Edit", "Project", "Model", "Context", "Settings", "Auth", "Tools", "System", "Help", "Custom"}
	for _, cat := range catOrder {
		cmds, ok := categories[cat]
		if !ok {
			continue
		}
		fmt.Printf("📁 %s (%d):\n", cat, len(cmds))
		for _, cmd := range cmds {
			fmt.Printf("   /%s - %s\n", cmd.Name, cmd.Description)
		}
		fmt.Println()
	}

	// Test filter
	fmt.Println("=== Filter Test ===")
	testFilters := []string{"/th", "/con", "/mod", "/new"}
	for _, filter := range testFilters {
		filtered := tui.FilterCommands(filter)
		fmt.Printf("Filter '%s': ", filter)
		names := make([]string, len(filtered))
		for i, cmd := range filtered {
			names[i] = "/" + cmd.Name
		}
		fmt.Println(strings.Join(names, ", "))
	}

	// Test themes
	fmt.Println()
	fmt.Println("=== Available Themes ===")
	themes := tui.AvailableThemes()
	fmt.Printf("Total: %d themes\n", len(themes))
	for i, th := range themes {
		if i > 0 && i%5 == 0 {
			fmt.Println()
		}
		fmt.Printf("  • %s", th)
	}
	fmt.Println()
}

// runTestTUI tests all TUI components without interactive mode
func runTestTUI() {
	theme := tui.DefaultTheme()

	fmt.Println()
	fmt.Println(theme.Title.Render("  🧪 DevOrch TUI Component Test  "))
	fmt.Println(theme.Border.Render("════════════════════════════════════════"))
	fmt.Println()

	// 1. Test Slash Commands
	fmt.Println(theme.Accent.Render("1️⃣  Slash Commands"))
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	commands := tui.GetSlashCommands()
	fmt.Printf("   Total: %d commands\n", len(commands))

	categories := make(map[string]int)
	for _, cmd := range commands {
		categories[cmd.Category]++
	}
	for cat, count := range categories {
		fmt.Printf("   • %s: %d\n", cat, count)
	}
	fmt.Println()

	// 2. Test Themes
	fmt.Println(theme.Accent.Render("2️⃣  Themes"))
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	themes := tui.AvailableThemes()
	fmt.Printf("   Total: %d themes\n", len(themes))
	fmt.Printf("   Preview: %s, %s, %s...\n", themes[0], themes[1], themes[2])
	fmt.Println()

	// 3. Test Providers (Connect)
	fmt.Println(theme.Accent.Render("3️⃣  Providers (/connect)"))
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	providers := tui.GetConnectProviders()
	popularCount := 0
	otherCount := 0
	connectedCount := 0
	for _, p := range providers {
		if p.Category == "Popular" {
			popularCount++
		} else {
			otherCount++
		}
		if p.IsConnected {
			connectedCount++
		}
	}
	fmt.Printf("   Total: %d providers\n", len(providers))
	fmt.Printf("   Popular: %d | Other: %d\n", popularCount, otherCount)
	fmt.Printf("   Connected: %d\n", connectedCount)
	fmt.Println()

	// 4. Test Models (Local)
	fmt.Println(theme.Accent.Render("4️⃣  Local Models (Ollama)"))
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	localProviders := tui.GetAvailableProviders()
	ollamaAvailable := false
	ollamaModels := 0
	for _, p := range localProviders {
		if p.Name == "ollama" && p.IsAvailable {
			ollamaAvailable = true
			ollamaModels = len(p.Models)
		}
	}
	if ollamaAvailable {
		fmt.Printf("   Ollama: %s\n", theme.Success.Render("✓ Running"))
		fmt.Printf("   Installed models: %d\n", ollamaModels)
	} else {
		fmt.Printf("   Ollama: %s\n", theme.Warning.Render("✗ Not running"))
	}
	fmt.Println()

	// 5. Test Essential Models
	fmt.Println(theme.Accent.Render("5️⃣  Essential Models Catalog"))
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	essentials := tui.EssentialOllamaModels
	fmt.Printf("   Total: %d models in catalog\n", len(essentials))
	for i, m := range essentials[:min(5, len(essentials))] {
		fmt.Printf("   %d. %s (%s) - %s\n", i+1, m.Name, m.Size, m.Description)
	}
	if len(essentials) > 5 {
		fmt.Printf("   ... and %d more\n", len(essentials)-5)
	}
	fmt.Println()

	// 6. Test System Info
	fmt.Println(theme.Accent.Render("6️⃣  System Info"))
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	profile, err := global.GetHWProfile()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   OS: %s/%s\n", profile.OS, profile.Arch)
		fmt.Printf("   RAM: %d GB\n", profile.MemTotalMB/1024)
		fmt.Printf("   GPU: %v (%s)\n", profile.HasAccel, profile.AccelKind)
		fmt.Printf("   Tier: %s\n", profile.Tier())
		fmt.Printf("   Recommended: %s\n", profile.RecommendedModelSize())
	}
	fmt.Println()

	// 7. Test Filter
	fmt.Println(theme.Accent.Render("7️⃣  Command Filter Test"))
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	testFilters := []string{"/con", "/mod", "/the", "/new", "/hel"}
	for _, filter := range testFilters {
		filtered := tui.FilterCommands(filter)
		names := make([]string, 0, len(filtered))
		for _, cmd := range filtered {
			names = append(names, "/"+cmd.Name)
		}
		if len(names) > 3 {
			names = append(names[:3], "...")
		}
		fmt.Printf("   %s → %s\n", filter, strings.Join(names, ", "))
	}
	fmt.Println()

	// Summary
	fmt.Println(theme.Border.Render("════════════════════════════════════════"))
	fmt.Println(theme.Success.Render("✓ All TUI components tested successfully!"))
	fmt.Println()
	fmt.Println("Run 'devorch' to start the interactive TUI.")
}

// runConnectList lists all providers with connection status
func runConnectList() {
	theme := tui.DefaultTheme()

	fmt.Println()
	fmt.Println(theme.Title.Render("  🔗 Provider Connection Status  "))
	fmt.Println()

	providers := tui.GetConnectProviders()
	store := auth.GetStore()

	// Group by category
	fmt.Println(theme.Accent.Render("Popular:"))
	for _, p := range providers {
		if p.Category != "Popular" {
			continue
		}
		status := theme.Warning.Render("○")
		statusText := "Not connected"
		if store.IsConnected(p.ID) || p.IsConnected {
			status = theme.Success.Render("●")
			statusText = "Connected"
		}
		authType := ""
		switch p.AuthType {
		case "api_key":
			authType = fmt.Sprintf("[%s]", p.EnvVar)
		case "oauth":
			authType = "[OAuth]"
		case "none":
			authType = "[Local]"
		}
		fmt.Printf("  %s %-20s %-15s %s\n", status, p.Name, statusText, authType)
	}
	fmt.Println()

	fmt.Println(theme.Accent.Render("Other:"))
	for _, p := range providers {
		if p.Category != "Other" {
			continue
		}
		status := theme.Warning.Render("○")
		statusText := "Not connected"
		if store.IsConnected(p.ID) || p.IsConnected {
			status = theme.Success.Render("●")
			statusText = "Connected"
		}
		fmt.Printf("  %s %-20s %-15s [%s]\n", status, p.Name, statusText, p.EnvVar)
	}
	fmt.Println()

	// Summary
	connectedCount := 0
	for _, p := range providers {
		if store.IsConnected(p.ID) || p.IsConnected {
			connectedCount++
		}
	}
	fmt.Printf("Total: %d/%d providers connected\n", connectedCount, len(providers))
	fmt.Println()
	fmt.Println("Use 'devorch connect' for interactive setup or 'devorch' (CLI mode) for OAuth.")
}

// runListThemes lists all available themes
func runListThemes() {
	theme := tui.DefaultTheme()

	fmt.Println()
	fmt.Println(theme.Title.Render("  🎨 Available Themes  "))
	fmt.Println()

	themes := tui.AvailableThemes()

	// Group themes by style
	darkThemes := []string{}
	lightThemes := []string{}
	colorThemes := []string{}

	for _, t := range themes {
		lower := strings.ToLower(t)
		if strings.Contains(lower, "light") {
			lightThemes = append(lightThemes, t)
		} else if strings.Contains(lower, "dark") || strings.Contains(lower, "dracula") ||
			strings.Contains(lower, "monokai") || strings.Contains(lower, "nord") ||
			strings.Contains(lower, "gruvbox") || strings.Contains(lower, "tokyo") {
			darkThemes = append(darkThemes, t)
		} else {
			colorThemes = append(colorThemes, t)
		}
	}

	fmt.Println(theme.Accent.Render("Dark Themes:"))
	for i, t := range darkThemes {
		if i > 0 && i%4 == 0 {
			fmt.Println()
		}
		fmt.Printf("  %-18s", t)
	}
	fmt.Println()
	fmt.Println()

	fmt.Println(theme.Accent.Render("Light Themes:"))
	for i, t := range lightThemes {
		if i > 0 && i%4 == 0 {
			fmt.Println()
		}
		fmt.Printf("  %-18s", t)
	}
	fmt.Println()
	fmt.Println()

	fmt.Println(theme.Accent.Render("Color Themes:"))
	for i, t := range colorThemes {
		if i > 0 && i%4 == 0 {
			fmt.Println()
		}
		fmt.Printf("  %-18s", t)
	}
	fmt.Println()
	fmt.Println()

	fmt.Printf("Total: %d themes\n", len(themes))
	fmt.Println()
	fmt.Println("Use 'devorch theme <name>' to preview a theme.")
}

// runPreviewTheme previews a specific theme
func runPreviewTheme(name string) {
	if name == "" {
		fmt.Println("Usage: devorch theme <name>")
		fmt.Println()
		fmt.Println("Available themes:")
		themes := tui.AvailableThemes()
		for i, t := range themes {
			if i > 0 && i%5 == 0 {
				fmt.Println()
			}
			fmt.Printf("  %s", t)
		}
		fmt.Println()
		return
	}

	theme := tui.GetTheme(name)

	fmt.Println()
	fmt.Println(theme.Title.Render(fmt.Sprintf("  🎨 Theme Preview: %s  ", name)))
	fmt.Println(theme.Border.Render("════════════════════════════════════════"))
	fmt.Println()

	// Show different styles
	fmt.Println(theme.Title.Render("Title Style"))
	fmt.Println(theme.Accent.Render("Accent Style"))
	fmt.Println(theme.Success.Render("Success Style"))
	fmt.Println(theme.Warning.Render("Warning Style"))
	fmt.Println(theme.Error.Render("Error Style"))
	fmt.Println(theme.Subtle.Render("Subtle Style"))
	fmt.Println(theme.Border.Render("Border Style"))
	fmt.Println()

	// Sample message display
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	fmt.Println(theme.Accent.Render("User:"))
	fmt.Println("  Hello, how are you?")
	fmt.Println()
	fmt.Println(theme.Success.Render("Assistant:"))
	fmt.Println("  I'm doing well, thank you for asking!")
	fmt.Println(theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	fmt.Println("To use this theme in TUI, run: /themes and select", name)
}
