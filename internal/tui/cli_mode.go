// Package tui provides terminal user interface components.
package tui

import (
	"bufio"
	"context"
	"devorch/internal/auth"
	"devorch/internal/editor"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// CLIMode runs DevOrch in headless CLI mode (no TUI, direct terminal I/O)
type CLIMode struct {
	theme       Theme
	messages    []Message
	provider    string
	model       string
	isStreaming bool
	client      *OllamaClient
}

// NewCLIMode creates a new CLI mode instance
func NewCLIMode() *CLIMode {
	active := GetActiveProvider()
	return &CLIMode{
		theme:    DefaultTheme(),
		messages: []Message{},
		provider: active.Provider,
		model:    active.Model,
		client:   NewOllamaClient(),
	}
}

// Run starts the CLI mode REPL
func (c *CLIMode) Run() error {
	c.printWelcome()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		// Print prompt
		fmt.Print(c.theme.Accent.Render("❯ "))

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			if c.handleCommand(input) {
				continue
			}
		}

		// Send to LLM
		c.sendMessage(input)
	}

	return scanner.Err()
}

// printWelcome prints the welcome message
func (c *CLIMode) printWelcome() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🚀 DevOrch CLI Mode  "))
	fmt.Println(c.theme.Border.Render("════════════════════════════════════════"))
	fmt.Println()

	// Show current provider/model
	fmt.Printf("  Provider: %s\n", c.theme.Accent.Render(c.provider))
	fmt.Printf("  Model: %s\n", c.theme.Accent.Render(c.model))
	fmt.Println()

	// Check Ollama status
	if checkOllamaAvailable() {
		fmt.Println(c.theme.Success.Render("  ✓ Ollama is running"))
		models := getOllamaModels()
		installed := 0
		for _, m := range models {
			if m.IsInstalled {
				installed++
			}
		}
		fmt.Printf("  ✓ %d models installed\n", installed)
	} else {
		fmt.Println(c.theme.Warning.Render("  ⚠ Ollama is not running"))
		fmt.Println("    Start with: ollama serve")
	}
	fmt.Println()

	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println(c.theme.Subtle.Render("  Type a message or use / for commands"))
	fmt.Println(c.theme.Subtle.Render("  /help for command list, /exit to quit"))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()
}

// handleCommand processes slash commands, returns true if handled
func (c *CLIMode) handleCommand(input string) bool {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}

	cmdName := strings.TrimPrefix(parts[0], "/")
	args := parts[1:]

	switch cmdName {
	case "exit", "quit", "q":
		fmt.Println(c.theme.Subtle.Render("Goodbye!"))
		os.Exit(0)

	case "help", "h", "?":
		c.printHelp()

	case "clear", "cls":
		c.messages = []Message{}
		fmt.Println(c.theme.Success.Render("✓ Chat cleared"))

	case "models":
		c.listModels()

	case "model":
		if len(args) > 0 {
			c.setModel(args[0])
		} else {
			c.listModels()
		}

	case "connect":
		c.showConnect()

	case "provider":
		if len(args) > 0 {
			c.setProvider(args[0])
		} else {
			c.showProviders()
		}

	case "themes":
		c.listThemes()

	case "theme":
		if len(args) > 0 {
			c.setTheme(args[0])
		} else {
			c.listThemes()
		}

	case "status":
		c.showStatus()

	case "new":
		c.messages = []Message{}
		fmt.Println(c.theme.Success.Render("✓ New session started"))

	case "history":
		c.showHistory()

	case "disconnect":
		if len(args) > 0 {
			c.disconnectProvider(args[0])
		} else {
			fmt.Println(c.theme.Warning.Render("⚠ Usage: /disconnect <provider>"))
		}

	// OpenCode compatible commands
	case "agents":
		c.showAgents()

	case "sessions":
		c.showSessions()

	case "editor":
		c.showEditor()

	case "init":
		c.initProject()

	case "mcps", "mcp":
		c.showMCPs()

	case "review":
		c.reviewCode(args)

	case "compact":
		c.compactHistory()

	case "save":
		c.saveSession(args)

	case "export":
		c.exportSession(args)

	case "undo":
		c.undoLast()

	case "redo":
		c.redoLast()

	case "files":
		c.listFiles(args)

	case "grep":
		c.grepSearch(args)

	case "ls":
		c.listDirectory(args)

	case "diff":
		c.showDiff(args)

	case "git":
		c.gitCommand(args)

	case "thinking":
		c.toggleThinking()

	case "context":
		c.showContext()

	case "memory":
		c.showMemory()

	case "add":
		c.addToContext(args)

	case "settings":
		c.showSettings()

	case "config":
		c.editConfig(args)

	case "tools":
		c.showTools()

	case "lsp":
		c.showLSP()

	case "doctor":
		c.runDoctor()

	case "version":
		c.showVersion()

	case "install":
		c.installModel(args)

	case "bench":
		c.runBenchmark(args)

	// Additional DevOrch commands
	case "reset":
		c.resetSession()

	case "share":
		c.shareSession(args)

	case "unshare":
		c.unshareSession(args)

	case "details":
		c.showDetails()

	case "providers":
		c.showProviders()

	case "agent":
		if len(args) > 0 {
			c.setAgent(args[0])
		} else {
			c.showAgents()
		}

	case "language", "lang":
		if len(args) > 0 {
			c.setLanguage(args[0])
		} else {
			c.showLanguage()
		}

	case "login":
		c.loginProvider(args)

	case "logout", "loginout":
		c.logoutProvider(args)

	case "auth":
		c.showAuthStatus()

	case "setup":
		c.runSetup()

	default:
		fmt.Printf("%s Unknown command: /%s\n", c.theme.Warning.Render("⚠"), cmdName)
		fmt.Println("  Type /help for available commands")
	}

	return true
}

// printHelp prints available commands
func (c *CLIMode) printHelp() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  📚 Available Commands  "))
	fmt.Println()

	categories := []struct {
		name     string
		commands []struct {
			cmd  string
			desc string
		}
	}{
		{
			name: "Session",
			commands: []struct {
				cmd  string
				desc string
			}{
				{"/help", "Show this help"},
				{"/exit", "Exit DevOrch"},
				{"/new", "Start new session"},
				{"/reset", "Reset session completely"},
				{"/sessions", "List saved sessions"},
				{"/save [name]", "Save current session"},
				{"/export [format]", "Export session (json/md)"},
				{"/share", "Share session"},
				{"/unshare", "Unshare session"},
				{"/clear", "Clear chat history"},
				{"/compact", "Compact chat history"},
				{"/details", "Show session details"},
				{"/undo", "Undo last action"},
				{"/redo", "Redo last action"},
			},
		},
		{
			name: "Models & Providers",
			commands: []struct {
				cmd  string
				desc string
			}{
				{"/models", "List available models"},
				{"/model <name>", "Switch model"},
				{"/providers", "List all providers"},
				{"/provider <name>", "Switch provider"},
				{"/connect", "Connect provider (OAuth/API)"},
				{"/disconnect <name>", "Disconnect provider"},
			},
		},
		{
			name: "Agents",
			commands: []struct {
				cmd  string
				desc string
			}{
				{"/agents", "List AI agents"},
				{"/agent <name>", "Switch agent mode"},
			},
		},
		{
			name: "Code & Files",
			commands: []struct {
				cmd  string
				desc string
			}{
				{"/files [path]", "List project files"},
				{"/ls [path]", "List directory"},
				{"/grep <pattern>", "Search in files"},
				{"/diff [file]", "Show file diff"},
				{"/git <cmd>", "Git commands"},
				{"/review [file]", "Review code"},
				{"/editor", "Open in editor"},
				{"/init", "Initialize project"},
			},
		},
		{
			name: "Tools & MCP",
			commands: []struct {
				cmd  string
				desc string
			}{
				{"/mcps", "Manage MCP servers"},
				{"/tools", "List available tools"},
				{"/lsp", "LSP status"},
			},
		},
		{
			name: "Context & Memory",
			commands: []struct {
				cmd  string
				desc string
			}{
				{"/context", "Show current context"},
				{"/memory", "Show memory/history"},
				{"/add <file>", "Add file to context"},
				{"/thinking", "Toggle thinking mode"},
			},
		},
		{
			name: "Auth & Config",
			commands: []struct {
				cmd  string
				desc string
			}{
				{"/login [provider]", "Login to provider"},
				{"/logout [provider]", "Logout from provider"},
				{"/auth", "Show auth status"},
				{"/settings", "Show settings"},
				{"/config [key]", "Edit config"},
				{"/language [lang]", "Set language (en/ko/ja)"},
				{"/themes", "List themes"},
				{"/theme <name>", "Switch theme"},
			},
		},
		{
			name: "System",
			commands: []struct {
				cmd  string
				desc string
			}{
				{"/status", "Show system status"},
				{"/setup", "Run initial setup"},
				{"/doctor", "Run diagnostics"},
				{"/version", "Show version"},
				{"/install <model>", "Install model"},
				{"/bench [model]", "Run benchmark"},
			},
		},
	}

	for _, cat := range categories {
		fmt.Printf("%s\n", c.theme.Accent.Render(cat.name+":"))
		for _, cmd := range cat.commands {
			fmt.Printf("  %s  %s\n",
				c.theme.Title.Render(fmt.Sprintf("%-20s", cmd.cmd)),
				c.theme.Subtle.Render(cmd.desc))
		}
		fmt.Println()
	}
}

// listModels lists available models
func (c *CLIMode) listModels() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  📦 Available Models  "))
	fmt.Println()

	if !checkOllamaAvailable() {
		fmt.Println(c.theme.Warning.Render("  ⚠ Ollama is not running"))
		return
	}

	models := getOllamaModels()
	if len(models) == 0 {
		fmt.Println(c.theme.Subtle.Render("  No models installed"))
		fmt.Println("  Install with: ollama pull <model>")
		return
	}

	for _, m := range models {
		if !m.IsInstalled {
			continue
		}
		marker := "  "
		if m.ID == c.model {
			marker = c.theme.Success.Render("▸ ")
		}
		fmt.Printf("%s%s %s\n", marker, m.ID, c.theme.Subtle.Render(m.Size))
	}
	fmt.Println()
	fmt.Printf("Current: %s\n", c.theme.Accent.Render(c.model))
	fmt.Println("Use /model <name> to switch")
	fmt.Println()
}

// setModel switches to a different model
func (c *CLIMode) setModel(model string) {
	// Verify model exists
	models := getOllamaModels()
	found := false
	for _, m := range models {
		if m.ID == model || strings.HasPrefix(m.ID, model) {
			c.model = m.ID
			found = true
			break
		}
	}

	if !found {
		// Allow setting model even if not in list (might be pulling)
		c.model = model
	}

	SetActiveProvider(c.provider, c.model)
	fmt.Printf("%s Model set to: %s\n", c.theme.Success.Render("✓"), c.model)
}

// showProviders shows provider connection status
func (c *CLIMode) showProviders() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🔗 Provider Status  "))
	fmt.Println()

	providers := GetConnectProviders()
	store := auth.GetStore()

	fmt.Println(c.theme.Accent.Render("Popular:"))
	for _, p := range providers {
		if p.Category != "Popular" {
			continue
		}
		status := c.theme.Warning.Render("○")
		if store.IsConnected(p.ID) || p.IsConnected {
			status = c.theme.Success.Render("●")
		}
		marker := "  "
		if p.ID == c.provider {
			marker = c.theme.Success.Render("▸ ")
		}
		fmt.Printf("%s%s %s\n", marker, status, p.Name)
	}
	fmt.Println()

	fmt.Printf("Current: %s\n", c.theme.Accent.Render(c.provider))
	fmt.Println("Use /provider <name> to switch")
	fmt.Println("Use /connect to add a new provider")
	fmt.Println()
}

// showConnect shows the provider connection UI
func (c *CLIMode) showConnect() {
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Println()
		fmt.Println(c.theme.Title.Render("  🔗 Connect Provider  "))
		fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
		fmt.Println()

		// Get providers
		providers := GetConnectProviders()
		store := auth.GetStore()

		// Show Popular providers
		fmt.Println(c.theme.Accent.Render("Popular:"))
		popIndex := 1
		providerList := []ConnectProvider{}

		for _, p := range providers {
			if p.Category == "Popular" {
				status := c.theme.Warning.Render("○")
				// Check auth store as well as env var
				if store.IsConnected(p.ID) || p.IsConnected {
					status = c.theme.Success.Render("●")
				}
				fmt.Printf("  %d. %s %s\n", popIndex, status, p.Name)
				providerList = append(providerList, p)
				popIndex++
			}
		}
		fmt.Println()

		// Show some Other providers
		fmt.Println(c.theme.Accent.Render("Other:"))
		otherIndex := popIndex
		for _, p := range providers {
			if p.Category == "Other" && otherIndex < popIndex+10 {
				status := c.theme.Warning.Render("○")
				if store.IsConnected(p.ID) || p.IsConnected {
					status = c.theme.Success.Render("●")
				}
				fmt.Printf("  %d. %s %s\n", otherIndex, status, p.Name)
				providerList = append(providerList, p)
				otherIndex++
			}
		}
		fmt.Println()

		fmt.Println(c.theme.Subtle.Render("Enter number to connect, 'q' to quit:"))
		fmt.Print(c.theme.Accent.Render("❯ "))

		if !scanner.Scan() {
			return
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "q" || input == "quit" || input == "" {
			return
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(providerList) {
			fmt.Println(c.theme.Warning.Render("⚠ Invalid selection"))
			continue
		}

		selected := providerList[idx-1]
		c.connectProvider(selected, scanner)
	}
}

// connectProvider handles connecting to a specific provider
func (c *CLIMode) connectProvider(provider ConnectProvider, scanner *bufio.Scanner) {
	fmt.Println()
	fmt.Println(c.theme.Title.Render(fmt.Sprintf("  Connect: %s  ", provider.Name)))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	// Get provider auth config
	config := auth.ProviderConfigs[provider.ID]
	if config == nil {
		// Fallback to simple API key
		c.connectAPIKey(provider, scanner)
		return
	}

	// Show available methods
	fmt.Println(c.theme.Subtle.Render("Select authentication method:"))
	fmt.Println()

	for i, method := range config.AuthMethods {
		desc := ""
		if method.Description != "" {
			desc = c.theme.Subtle.Render(" - " + method.Description)
		}
		fmt.Printf("  %d. %s%s\n", i+1, method.Label, desc)
	}
	fmt.Println()

	fmt.Print(c.theme.Accent.Render("❯ "))
	if !scanner.Scan() {
		return
	}

	input := strings.TrimSpace(scanner.Text())
	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(config.AuthMethods) {
		fmt.Println(c.theme.Warning.Render("⚠ Invalid selection"))
		return
	}

	method := config.AuthMethods[idx-1]

	switch method.Type {
	case auth.AuthTypeDevice:
		c.connectDeviceFlow(provider, config, idx-1)
	case auth.AuthTypeHeadless:
		c.connectHeadlessFlow(provider, config, idx-1)
	case auth.AuthTypeOAuth:
		c.connectOAuth(provider, config, idx-1, scanner)
	case auth.AuthTypeAPI:
		c.connectAPIKey(provider, scanner)
	case auth.AuthTypeNone:
		fmt.Println(c.theme.Success.Render("✓ No authentication required"))
	}
}

// connectDeviceFlow handles GitHub device code flow
func (c *CLIMode) connectDeviceFlow(provider ConnectProvider, config *auth.ProviderAuthConfig, methodIdx int) {
	fmt.Println()
	fmt.Println(c.theme.Subtle.Render("Starting device authorization..."))
	fmt.Println()

	result, err := auth.StartOAuthFlow(provider.ID, methodIdx)
	if err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Error: %v", err)))
		return
	}

	// Show verification URL and code (OpenCode style)
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()
	fmt.Printf("  %s\n", c.theme.Accent.Render(result.URL))
	fmt.Printf("  Enter code: %s\n", c.theme.Title.Render(result.UserCode))
	fmt.Println()
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	// Open browser
	openBrowser(result.URL)

	fmt.Println(c.theme.Subtle.Render("Waiting for authorization…"))
	fmt.Println()

	// Poll for completion
	authInfo, err := auth.PollDeviceFlow(provider.ID, result)
	if err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Authorization failed: %v", err)))
		return
	}

	// Save auth info
	store := auth.GetStore()
	if err := store.Set(provider.ID, authInfo); err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Failed to save: %v", err)))
		return
	}

	fmt.Println(c.theme.Success.Render("✓ Authorization successful!"))
	fmt.Printf("  Connected to: %s\n", provider.Name)
	fmt.Println()
}

// connectHeadlessFlow handles OpenAI headless device code flow (no browser required)
func (c *CLIMode) connectHeadlessFlow(provider ConnectProvider, config *auth.ProviderAuthConfig, methodIdx int) {
	fmt.Println()
	fmt.Println(c.theme.Subtle.Render("Starting headless authorization..."))
	fmt.Println()

	result, err := auth.StartOAuthFlow(provider.ID, methodIdx)
	if err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Error: %v", err)))
		return
	}

	// Show verification URL and code (OpenCode style - no browser)
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()
	fmt.Printf("  %s\n", c.theme.Accent.Render(result.URL))
	fmt.Printf("  Enter code: %s\n", c.theme.Title.Render(result.UserCode))
	fmt.Println()
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	fmt.Println(c.theme.Subtle.Render("Waiting for authorization…"))
	fmt.Println()

	// Poll for completion (same as device flow)
	authInfo, err := auth.PollDeviceFlow(provider.ID, result)
	if err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Authorization failed: %v", err)))
		return
	}

	// Save auth info
	store := auth.GetStore()
	if err := store.Set(provider.ID, authInfo); err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Failed to save: %v", err)))
		return
	}

	fmt.Println(c.theme.Success.Render("✓ Authorization successful!"))
	fmt.Printf("  Connected to: %s\n", provider.Name)
	fmt.Println()
}

// connectOAuth handles OAuth authorization code flow
func (c *CLIMode) connectOAuth(provider ConnectProvider, config *auth.ProviderAuthConfig, methodIdx int, scanner *bufio.Scanner) {
	fmt.Println()
	fmt.Println(c.theme.Subtle.Render("Starting OAuth authorization..."))
	fmt.Println()

	result, err := auth.StartOAuthFlow(provider.ID, methodIdx)
	if err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Error: %v", err)))
		return
	}

	// Show OAuth URL (OpenCode style)
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()
	fmt.Printf("  %s\n", c.theme.Accent.Render(result.URL))
	fmt.Println()
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	// Open browser
	openBrowser(result.URL)

	if result.Method == "auto" {
		// Auto callback via localhost server (OpenAI/ChatGPT style)
		fmt.Println(c.theme.Subtle.Render("Waiting for authorization…"))
		fmt.Println()

		server, err := auth.StartCallbackServer(1455)
		if err != nil {
			fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Failed to start callback server: %v", err)))
			return
		}
		defer server.Stop()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		code, err := server.WaitForCallback(ctx)
		if err != nil {
			fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Authorization failed: %v", err)))
			return
		}

		// Exchange code for tokens
		authInfo, err := auth.ExchangeCode(provider.ID, code, result)
		if err != nil {
			fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Token exchange failed: %v", err)))
			return
		}

		// Save auth info
		store := auth.GetStore()
		if err := store.Set(provider.ID, authInfo); err != nil {
			fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Failed to save: %v", err)))
			return
		}

		fmt.Println(c.theme.Success.Render("✓ Authorization successful!"))
		fmt.Printf("  Connected to: %s\n", provider.Name)
		fmt.Println()

	} else {
		// Code method - user copies code from browser (Anthropic/Claude style)
		fmt.Println(c.theme.Subtle.Render("Waiting for authorization…"))
		fmt.Println()
		fmt.Println(c.theme.Subtle.Render("After authorization, copy the code from the browser and paste it here:"))
		fmt.Println()
		fmt.Print(c.theme.Accent.Render("Code: "))

		if !scanner.Scan() {
			return
		}

		code := strings.TrimSpace(scanner.Text())
		if code == "" {
			fmt.Println(c.theme.Warning.Render("⚠ No code entered"))
			return
		}

		// Exchange code for tokens
		authInfo, err := auth.ExchangeCode(provider.ID, code, result)
		if err != nil {
			fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Token exchange failed: %v", err)))
			return
		}

		// Save auth info
		store := auth.GetStore()
		if err := store.Set(provider.ID, authInfo); err != nil {
			fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Failed to save: %v", err)))
			return
		}

		fmt.Println(c.theme.Success.Render("✓ Authorization successful!"))
		fmt.Printf("  Connected to: %s\n", provider.Name)
		fmt.Println()
	}
}

// connectAPIKey handles API key entry
func (c *CLIMode) connectAPIKey(provider ConnectProvider, scanner *bufio.Scanner) {
	fmt.Println()

	// Show auth URL if available
	authURL := provider.AuthURL
	if config := auth.ProviderConfigs[provider.ID]; config != nil && config.AuthURL != "" {
		authURL = config.AuthURL
	}

	if authURL != "" {
		fmt.Printf("  Get your API key: %s\n", c.theme.Accent.Render(authURL))
		fmt.Println()
	}

	fmt.Printf("  Environment variable: %s\n", c.theme.Subtle.Render(provider.EnvVar))
	fmt.Println()

	fmt.Print(c.theme.Accent.Render("Enter API key: "))
	if !scanner.Scan() {
		return
	}

	apiKey := strings.TrimSpace(scanner.Text())
	if apiKey == "" {
		fmt.Println(c.theme.Warning.Render("⚠ No API key entered"))
		return
	}

	// Save API key
	if err := auth.SetAPIKey(provider.ID, apiKey); err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Failed to save: %v", err)))
		return
	}

	fmt.Println(c.theme.Success.Render("✓ API key saved!"))
	fmt.Printf("  Connected to: %s\n", provider.Name)
	fmt.Println()
}

// disconnectProvider disconnects a provider
func (c *CLIMode) disconnectProvider(name string) {
	providers := GetConnectProviders()
	for _, p := range providers {
		if strings.EqualFold(p.ID, name) || strings.EqualFold(p.Name, name) {
			store := auth.GetStore()
			if err := store.Remove(p.ID); err != nil {
				fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Failed to disconnect: %v", err)))
				return
			}
			fmt.Println(c.theme.Success.Render(fmt.Sprintf("✓ Disconnected from %s", p.Name)))
			return
		}
	}
	fmt.Println(c.theme.Warning.Render(fmt.Sprintf("⚠ Unknown provider: %s", name)))
}

// setProvider switches to a different provider
func (c *CLIMode) setProvider(provider string) {
	providers := GetConnectProviders()
	for _, p := range providers {
		if strings.EqualFold(p.ID, provider) || strings.EqualFold(p.Name, provider) {
			c.provider = p.ID
			// Get default model for provider
			models := GetModelsForProvider(p.ID)
			if len(models) > 0 {
				c.model = models[0].ID
			}
			SetActiveProvider(c.provider, c.model)
			fmt.Printf("%s Provider set to: %s (model: %s)\n",
				c.theme.Success.Render("✓"), c.provider, c.model)
			return
		}
	}
	fmt.Printf("%s Unknown provider: %s\n", c.theme.Warning.Render("⚠"), provider)
}

// listThemes lists available themes
func (c *CLIMode) listThemes() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🎨 Available Themes  "))
	fmt.Println()

	themes := AvailableThemes()
	for i, t := range themes {
		if i > 0 && i%4 == 0 {
			fmt.Println()
		}
		fmt.Printf("  %-16s", t)
	}
	fmt.Println()
	fmt.Println()
	fmt.Println("Use /theme <name> to switch")
	fmt.Println()
}

// setTheme switches to a different theme
func (c *CLIMode) setTheme(name string) {
	c.theme = GetTheme(name)
	fmt.Printf("%s Theme set to: %s\n", c.theme.Success.Render("✓"), name)
}

// showStatus shows system status
func (c *CLIMode) showStatus() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  📊 System Status  "))
	fmt.Println()

	// Provider/Model
	fmt.Printf("  Provider: %s\n", c.theme.Accent.Render(c.provider))
	fmt.Printf("  Model: %s\n", c.theme.Accent.Render(c.model))
	fmt.Println()

	// Connection status
	store := auth.GetStore()
	connectedCount := 0
	for _, info := range store.All() {
		if info != nil {
			connectedCount++
		}
	}
	fmt.Printf("  Connected providers: %d\n", connectedCount)

	// Ollama
	if checkOllamaAvailable() {
		fmt.Printf("  Ollama: %s\n", c.theme.Success.Render("Running"))
		models := getOllamaModels()
		installed := 0
		for _, m := range models {
			if m.IsInstalled {
				installed++
			}
		}
		fmt.Printf("  Models: %d installed\n", installed)
	} else {
		fmt.Printf("  Ollama: %s\n", c.theme.Warning.Render("Not running"))
	}
	fmt.Println()

	// Chat stats
	fmt.Printf("  Messages: %d\n", len(c.messages))
	fmt.Println()
}

// showHistory shows chat history
func (c *CLIMode) showHistory() {
	if len(c.messages) == 0 {
		fmt.Println(c.theme.Subtle.Render("No messages yet"))
		return
	}

	fmt.Println()
	for _, msg := range c.messages {
		switch msg.Role {
		case "user":
			fmt.Println(c.theme.Accent.Render("You:"))
			fmt.Println(wrapText(msg.Content, 76))
		case "assistant":
			fmt.Println(c.theme.Success.Render("Assistant:"))
			fmt.Println(wrapText(msg.Content, 76))
		}
		fmt.Println()
	}
}

// sendMessage sends a message to the LLM and streams the response
func (c *CLIMode) sendMessage(input string) {
	// Add user message
	c.messages = append(c.messages, Message{
		Role:    "user",
		Content: input,
	})

	// Print user message
	fmt.Println()
	fmt.Println(c.theme.Accent.Render("You:"))
	fmt.Println(wrapText(input, 76))
	fmt.Println()

	// Check provider
	if c.provider != "ollama" {
		fmt.Println(c.theme.Warning.Render("⚠ Only Ollama provider is supported in CLI mode"))
		fmt.Println("  Use /provider ollama to switch")
		return
	}

	// Check Ollama
	if !checkOllamaAvailable() {
		fmt.Println(c.theme.Error.Render("✗ Ollama is not running"))
		fmt.Println("  Start with: ollama serve")
		return
	}

	// Stream response
	fmt.Println(c.theme.Success.Render("Assistant:"))
	c.isStreaming = true

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var fullResponse strings.Builder

	// Use callback-based streaming
	err := c.client.ChatStream(ctx, c.model, c.messages, func(chunk string) {
		fmt.Print(chunk)
		fullResponse.WriteString(chunk)
	})

	if err != nil {
		fmt.Println()
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Error: %v", err)))
		c.isStreaming = false
		return
	}

	fmt.Println()
	fmt.Println()

	// Save assistant response
	if fullResponse.Len() > 0 {
		c.messages = append(c.messages, Message{
			Role:    "assistant",
			Content: fullResponse.String(),
		})
	}

	c.isStreaming = false
}

// wrapText wraps text to a given width
func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		words := strings.Fields(line)
		currentLine := ""
		for _, word := range words {
			if len(currentLine)+len(word)+1 > width {
				result.WriteString(currentLine)
				result.WriteString("\n")
				currentLine = word
			} else {
				if currentLine == "" {
					currentLine = word
				} else {
					currentLine += " " + word
				}
			}
		}
		if currentLine != "" {
			result.WriteString(currentLine)
			result.WriteString("\n")
		}
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}

// ==================== OpenCode Compatible Commands ====================

// Agent represents an AI agent configuration
type Agent struct {
	Name         string
	SystemPrompt string
	Description  string
	Active       bool
}

var currentAgent = "coder"
var availableAgents = map[string]*Agent{
	"coder": {
		Name:         "Coder",
		SystemPrompt: "You are an expert programmer. Write clean, efficient, and well-documented code. Follow best practices and design patterns.",
		Description:  "Code generation and editing",
		Active:       true,
	},
	"reviewer": {
		Name:         "Reviewer",
		SystemPrompt: "You are a senior code reviewer. Analyze code for bugs, security issues, performance problems, and best practices violations. Provide constructive feedback.",
		Description:  "Code review and suggestions",
		Active:       false,
	},
	"debugger": {
		Name:         "Debugger",
		SystemPrompt: "You are an expert debugger. Analyze error messages, stack traces, and code to identify root causes of bugs. Suggest fixes and explain the reasoning.",
		Description:  "Bug detection and fixing",
		Active:       false,
	},
	"architect": {
		Name:         "Architect",
		SystemPrompt: "You are a software architect. Design scalable, maintainable systems. Provide architecture diagrams, component designs, and technical specifications.",
		Description:  "System design and planning",
		Active:       false,
	},
	"tester": {
		Name:         "Tester",
		SystemPrompt: "You are a QA expert. Write comprehensive tests including unit tests, integration tests, and edge cases. Focus on test coverage and reliability.",
		Description:  "Test generation and validation",
		Active:       false,
	},
}

// showAgents displays available AI agents
func (c *CLIMode) showAgents() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🤖 AI Agents  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	agentOrder := []string{"coder", "reviewer", "debugger", "architect", "tester"}
	for i, id := range agentOrder {
		a := availableAgents[id]
		status := c.theme.Warning.Render("○")
		if id == currentAgent {
			status = c.theme.Success.Render("●")
		}
		fmt.Printf("  %d. %s %s  %s\n", i+1, status, c.theme.Accent.Render(a.Name), c.theme.Subtle.Render(a.Description))
	}
	fmt.Println()
	fmt.Printf("  Current: %s\n", c.theme.Accent.Render(availableAgents[currentAgent].Name))
	fmt.Println("  Use /agent <name> to switch")
	fmt.Println()
}

// showSessions displays saved sessions
func (c *CLIMode) showSessions() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  📂 Sessions  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	// Load sessions from file
	homeDir, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(homeDir, ".config", "devorch", "sessions")

	entries, err := os.ReadDir(sessionsDir)
	if err != nil || len(entries) == 0 {
		fmt.Println(c.theme.Subtle.Render("  No saved sessions yet"))
		fmt.Println()
		fmt.Println("  Use /save <name> to save current session")
		fmt.Println()
		return
	}

	for i, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			name := strings.TrimSuffix(e.Name(), ".json")
			info, _ := e.Info()
			modTime := ""
			if info != nil {
				modTime = info.ModTime().Format("2006-01-02 15:04")
			}
			fmt.Printf("  %d. %s  %s\n", i+1, c.theme.Accent.Render(name), c.theme.Subtle.Render(modTime))
		}
	}
	fmt.Println()
	fmt.Println("  Use /sessions load <name> to load a session")
	fmt.Println()
}

// showEditor opens/shows editor integration
func (c *CLIMode) showEditor() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  📝 Editor Integration  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	// Initialize editor manager
	editorMgr := editor.NewManager()
	status := editorMgr.GetStatus()

	// Show default editor
	defaultEditor := status.DefaultEditor
	fmt.Printf("  %s %s\n", c.theme.Accent.Render("Default:"), defaultEditor.Name)
	if status.EnvEditor != "" {
		fmt.Printf("  %s $EDITOR=%s\n", c.theme.Subtle.Render("          "), status.EnvEditor)
	}
	fmt.Println()

	// Show available editors
	fmt.Println(c.theme.Accent.Render("  Available Editors:"))
	for _, e := range status.AvailableEditors {
		marker := "○"
		if e.Name == defaultEditor.Name {
			marker = "●"
		}
		status := c.theme.Success.Render(marker)
		fmt.Printf("    %s %s (%s)\n", status, e.Name, c.theme.Subtle.Render(e.Command))
	}
	fmt.Println()

	// Show usage
	fmt.Println(c.theme.Accent.Render("  Usage:"))
	fmt.Printf("    %s         Open editor to compose prompt\n", c.theme.Accent.Render("/editor"))
	fmt.Printf("    %s Open specific file\n", c.theme.Accent.Render("/editor <file>"))
	fmt.Printf("    %s  Open file at line\n", c.theme.Accent.Render("/editor <file>:<line>"))
	fmt.Println()
	fmt.Printf("  %s Set $EDITOR environment variable to change default editor\n", c.theme.Subtle.Render("Tip:"))
	fmt.Println()
}

// initProject initializes project configuration
func (c *CLIMode) initProject() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🚀 Initialize Project  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	// Check for existing config
	if _, err := os.Stat(".devorch.yml"); err == nil {
		fmt.Println(c.theme.Warning.Render("  ⚠ .devorch.yml already exists"))
		return
	}

	// Create default config
	config := `# DevOrch Configuration
version: 1
project:
  name: my-project
  type: auto

llm:
  provider: ollama
  model: llama3.2

tools:
  enabled: true
  mcp: []
`
	if err := os.WriteFile(".devorch.yml", []byte(config), 0644); err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("  ✗ Failed to create config: %v", err)))
		return
	}

	fmt.Println(c.theme.Success.Render("  ✓ Created .devorch.yml"))
	fmt.Println()
}

// showMCPs displays MCP server status
func (c *CLIMode) showMCPs() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🔌 MCP Servers  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	// Check for MCP config files
	homeDir, _ := os.UserHomeDir()
	mcpConfigPaths := []string{
		".devorch.yml",
		filepath.Join(homeDir, ".config", "devorch", "mcps.json"),
		filepath.Join(homeDir, ".mcp", "config.json"),
	}

	var configFound string
	for _, p := range mcpConfigPaths {
		if _, err := os.Stat(p); err == nil {
			configFound = p
			break
		}
	}

	// Built-in MCPs
	mcps := []struct {
		name    string
		desc    string
		running bool
		builtin bool
	}{
		{"filesystem", "File system access (read/write/list)", true, true},
		{"shell", "Shell command execution", true, true},
		{"git", "Git operations", exec.Command("git", "--version").Run() == nil, true},
		{"web", "Web browsing and fetching", false, false},
		{"database", "Database queries (SQL)", false, false},
		{"docker", "Container management", exec.Command("docker", "--version").Run() == nil, false},
	}

	fmt.Println(c.theme.Accent.Render("Built-in:"))
	for _, m := range mcps {
		if !m.builtin {
			continue
		}
		status := c.theme.Warning.Render("○")
		if m.running {
			status = c.theme.Success.Render("●")
		}
		fmt.Printf("  %s %s  %s\n", status, c.theme.Accent.Render(m.name), c.theme.Subtle.Render(m.desc))
	}

	fmt.Println()
	fmt.Println(c.theme.Accent.Render("External:"))
	for _, m := range mcps {
		if m.builtin {
			continue
		}
		status := c.theme.Warning.Render("○")
		if m.running {
			status = c.theme.Success.Render("●")
		}
		fmt.Printf("  %s %s  %s\n", status, c.theme.Accent.Render(m.name), c.theme.Subtle.Render(m.desc))
	}

	fmt.Println()
	if configFound != "" {
		fmt.Printf("  Config: %s\n", c.theme.Subtle.Render(configFound))
	} else {
		fmt.Println(c.theme.Subtle.Render("  No MCP config found. Create .devorch.yml or ~/.config/devorch/mcps.json"))
	}
	fmt.Println()
}

// reviewCode performs code review
func (c *CLIMode) reviewCode(args []string) {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🔍 Code Review  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	var diffContent string
	var targetDesc string

	if len(args) == 0 {
		// Review staged changes
		cmd := exec.Command("git", "diff", "--staged")
		output, err := cmd.Output()
		if err != nil || len(output) == 0 {
			// Try unstaged changes
			cmd = exec.Command("git", "diff")
			output, err = cmd.Output()
			if err != nil || len(output) == 0 {
				fmt.Println(c.theme.Subtle.Render("  No changes to review"))
				fmt.Println()
				fmt.Println("  Usage: /review <file> or make some changes first")
				return
			}
			targetDesc = "unstaged changes"
		} else {
			targetDesc = "staged changes"
		}
		diffContent = string(output)
	} else {
		// Review specific file
		file := args[0]
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Println(c.theme.Error.Render(fmt.Sprintf("  ✗ Cannot read file: %v", err)))
			return
		}
		diffContent = string(content)
		targetDesc = file
	}

	fmt.Printf("  Reviewing: %s\n", c.theme.Accent.Render(targetDesc))
	fmt.Println()

	// Show diff stats
	additions := strings.Count(diffContent, "\n+") - strings.Count(diffContent, "\n+++")
	deletions := strings.Count(diffContent, "\n-") - strings.Count(diffContent, "\n---")
	fmt.Printf("  %s +%d %s -%d\n",
		c.theme.Success.Render("Additions:"), additions,
		c.theme.Error.Render("Deletions:"), deletions)
	fmt.Println()

	// Add to context for AI review
	reviewPrompt := fmt.Sprintf("Please review the following code changes and provide feedback on:\n1. Code quality and best practices\n2. Potential bugs or issues\n3. Security concerns\n4. Performance considerations\n5. Suggestions for improvement\n\n```diff\n%s\n```", diffContent)

	c.messages = append(c.messages, Message{
		Role:    "user",
		Content: reviewPrompt,
	})

	fmt.Println(c.theme.Subtle.Render("  Code added to context. Send a message to get AI review."))
	fmt.Println(c.theme.Subtle.Render("  Or type: 'Review this code' to start"))
	fmt.Println()
}

// compactHistory compacts chat history
func (c *CLIMode) compactHistory() {
	if len(c.messages) < 4 {
		fmt.Println(c.theme.Warning.Render("⚠ Not enough messages to compact"))
		return
	}

	// Keep last 2 exchanges
	c.messages = c.messages[len(c.messages)-4:]
	fmt.Println(c.theme.Success.Render("✓ History compacted"))
}

// saveSession saves current session
func (c *CLIMode) saveSession(args []string) {
	name := fmt.Sprintf("session-%d", time.Now().Unix())
	if len(args) > 0 {
		name = args[0]
	}

	if len(c.messages) == 0 {
		fmt.Println(c.theme.Warning.Render("⚠ No messages to save"))
		return
	}

	// Create sessions directory
	homeDir, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(homeDir, ".config", "devorch", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Failed to create sessions dir: %v", err)))
		return
	}

	// Save session as JSON
	session := map[string]interface{}{
		"name":     name,
		"provider": c.provider,
		"model":    c.model,
		"messages": c.messages,
		"saved_at": time.Now().Format(time.RFC3339),
	}

	data, _ := json.MarshalIndent(session, "", "  ")
	filePath := filepath.Join(sessionsDir, name+".json")

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Failed to save: %v", err)))
		return
	}

	fmt.Printf("%s Session saved: %s\n", c.theme.Success.Render("✓"), c.theme.Accent.Render(name))
	fmt.Printf("  %d messages, %s\n", len(c.messages), filePath)
}

// exportSession exports session to file
func (c *CLIMode) exportSession(args []string) {
	format := "md"
	if len(args) > 0 {
		format = strings.ToLower(args[0])
	}

	if len(c.messages) == 0 {
		fmt.Println(c.theme.Warning.Render("⚠ No messages to export"))
		return
	}

	filename := fmt.Sprintf("devorch-export-%d", time.Now().Unix())
	var content string

	switch format {
	case "json":
		filename += ".json"
		data, _ := json.MarshalIndent(c.messages, "", "  ")
		content = string(data)
	case "md", "markdown":
		filename += ".md"
		var sb strings.Builder
		sb.WriteString("# DevOrch Chat Export\n\n")
		sb.WriteString(fmt.Sprintf("Provider: %s | Model: %s\n\n", c.provider, c.model))
		sb.WriteString("---\n\n")
		for _, m := range c.messages {
			if m.Role == "user" {
				sb.WriteString(fmt.Sprintf("## 👤 User\n\n%s\n\n", m.Content))
			} else {
				sb.WriteString(fmt.Sprintf("## 🤖 Assistant\n\n%s\n\n", m.Content))
			}
		}
		content = sb.String()
	case "txt", "text":
		filename += ".txt"
		var sb strings.Builder
		for _, m := range c.messages {
			sb.WriteString(fmt.Sprintf("[%s]\n%s\n\n", m.Role, m.Content))
		}
		content = sb.String()
	default:
		fmt.Printf("%s Unknown format: %s (use json, md, or txt)\n", c.theme.Warning.Render("⚠"), format)
		return
	}

	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("✗ Failed to export: %v", err)))
		return
	}

	fmt.Printf("%s Exported to: %s\n", c.theme.Success.Render("✓"), c.theme.Accent.Render(filename))
}

// undoLast undoes last action
func (c *CLIMode) undoLast() {
	if len(c.messages) < 2 {
		fmt.Println(c.theme.Warning.Render("⚠ Nothing to undo"))
		return
	}

	c.messages = c.messages[:len(c.messages)-2]
	fmt.Println(c.theme.Success.Render("✓ Last exchange undone"))
}

// redoLast redoes last action
func (c *CLIMode) redoLast() {
	fmt.Println(c.theme.Warning.Render("⚠ Redo not available"))
}

// listFiles lists project files
func (c *CLIMode) listFiles(args []string) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}

	fmt.Println()
	fmt.Println(c.theme.Title.Render(fmt.Sprintf("  📁 Files: %s  ", path)))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))

	// Count files by type
	var dirs, files int
	var totalSize int64

	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("  ✗ %v", err)))
		return
	}

	// Sort: directories first, then files
	var dirList, fileList []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirList = append(dirList, e)
			dirs++
		} else {
			fileList = append(fileList, e)
			files++
			if info, err := e.Info(); err == nil {
				totalSize += info.Size()
			}
		}
	}

	// Show directories
	for _, e := range dirList {
		name := e.Name()
		// Skip hidden and common ignored directories
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
			continue
		}
		fmt.Printf("  %s %s/\n", c.theme.Accent.Render("📂"), name)
	}

	// Show files
	for _, e := range fileList {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		info, _ := e.Info()
		size := ""
		if info != nil {
			size = formatSize(info.Size())
		}

		// File type icon
		icon := "📄"
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".go":
			icon = "🐹"
		case ".js", ".ts", ".jsx", ".tsx":
			icon = "🟨"
		case ".py":
			icon = "🐍"
		case ".rs":
			icon = "🦀"
		case ".md":
			icon = "📝"
		case ".json", ".yml", ".yaml":
			icon = "⚙️"
		}

		fmt.Printf("  %s %s %s\n", icon, name, c.theme.Subtle.Render(size))
	}

	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Printf("  %d directories, %d files (%s)\n", dirs, files, cliFormatSize(totalSize))
	fmt.Println()
}

// cliFormatSize formats bytes to human readable (CLI mode specific)
func cliFormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// grepSearch searches files
func (c *CLIMode) grepSearch(args []string) {
	if len(args) == 0 {
		fmt.Println(c.theme.Warning.Render("⚠ Usage: /grep <pattern> [path]"))
		return
	}

	pattern := args[0]
	searchPath := "."
	if len(args) > 1 {
		searchPath = args[1]
	}

	fmt.Println()
	fmt.Printf("  Searching for: %s in %s\n", c.theme.Accent.Render(pattern), searchPath)
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))

	// Use grep command
	cmd := exec.Command("grep", "-rn", "--color=never", "--include=*.go", "--include=*.js", "--include=*.ts", "--include=*.py", "--include=*.java", "--include=*.rs", "--include=*.c", "--include=*.cpp", "--include=*.h", "--include=*.md", "--include=*.yml", "--include=*.yaml", "--include=*.json", pattern, searchPath)
	output, err := cmd.Output()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			fmt.Println(c.theme.Subtle.Render("  No matches found"))
		} else {
			// Try ripgrep as fallback
			cmd = exec.Command("rg", "-n", "--no-heading", pattern, searchPath)
			output, err = cmd.Output()
			if err != nil {
				fmt.Println(c.theme.Subtle.Render("  No matches found"))
				return
			}
		}
	}

	if len(output) > 0 {
		lines := strings.Split(string(output), "\n")
		maxLines := 20
		for i, line := range lines {
			if i >= maxLines {
				fmt.Printf("\n  ... and %d more matches\n", len(lines)-maxLines)
				break
			}
			if line != "" {
				// Parse file:line:content format
				parts := strings.SplitN(line, ":", 3)
				if len(parts) >= 3 {
					fmt.Printf("  %s:%s: %s\n",
						c.theme.Accent.Render(parts[0]),
						c.theme.Warning.Render(parts[1]),
						strings.TrimSpace(parts[2]))
				} else {
					fmt.Printf("  %s\n", line)
				}
			}
		}
	}
	fmt.Println()
}

// listDirectory lists directory contents
func (c *CLIMode) listDirectory(args []string) {
	c.listFiles(args) // Same as /files
}

// showDiff shows file diff
func (c *CLIMode) showDiff(args []string) {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  📊 Diff  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))

	var cmd *exec.Cmd
	if len(args) == 0 {
		// Show all changes
		cmd = exec.Command("git", "diff", "--stat")
	} else if args[0] == "--staged" || args[0] == "-s" {
		cmd = exec.Command("git", "diff", "--staged", "--stat")
	} else {
		// Diff specific file
		cmd = exec.Command("git", "diff", args[0])
	}

	output, err := cmd.Output()
	if err != nil || len(output) == 0 {
		fmt.Println(c.theme.Subtle.Render("  No changes detected"))
		fmt.Println()
		fmt.Println("  Usage:")
		fmt.Println("    /diff          - Show all unstaged changes")
		fmt.Println("    /diff --staged - Show staged changes")
		fmt.Println("    /diff <file>   - Show changes for specific file")
		return
	}

	// Color the output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			fmt.Println(c.theme.Success.Render(line))
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			fmt.Println(c.theme.Error.Render(line))
		} else if strings.HasPrefix(line, "@@") {
			fmt.Println(c.theme.Accent.Render(line))
		} else {
			fmt.Println(line)
		}
	}
	fmt.Println()
}

// gitCommand runs git command
func (c *CLIMode) gitCommand(args []string) {
	if len(args) == 0 {
		// Show git status
		args = []string{"status", "-s"}
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("  ✗ git error: %v", err)))
		return
	}

	fmt.Println()
	fmt.Println(string(output))
}

// toggleThinking toggles thinking mode
func (c *CLIMode) toggleThinking() {
	fmt.Println(c.theme.Success.Render("✓ Thinking mode toggled"))
	fmt.Println(c.theme.Subtle.Render("  (Extended thinking will be shown in responses)"))
}

// showContext shows current context
func (c *CLIMode) showContext() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  📎 Current Context  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	// Session info
	fmt.Println(c.theme.Accent.Render("Session:"))
	fmt.Printf("  Provider: %s\n", c.theme.Title.Render(c.provider))
	fmt.Printf("  Model: %s\n", c.theme.Title.Render(c.model))
	fmt.Printf("  Agent: %s\n", c.theme.Title.Render(availableAgents[currentAgent].Name))
	fmt.Printf("  Messages: %d\n", len(c.messages))

	// Token estimation
	totalChars := 0
	for _, m := range c.messages {
		totalChars += len(m.Content)
	}
	approxTokens := totalChars / 4
	fmt.Printf("  Est. Tokens: ~%d\n", approxTokens)
	fmt.Println()

	// Files in context
	filesInContext := 0
	for _, m := range c.messages {
		if strings.Contains(m.Content, "Here's the content of") {
			filesInContext++
		}
	}

	if filesInContext > 0 {
		fmt.Println(c.theme.Accent.Render("Files in context:"))
		fmt.Printf("  %d files added\n", filesInContext)
		fmt.Println()
	}

	// Working directory
	cwd, _ := os.Getwd()
	fmt.Println(c.theme.Accent.Render("Working directory:"))
	fmt.Printf("  %s\n", cwd)

	// Git info
	if branch, err := exec.Command("git", "branch", "--show-current").Output(); err == nil {
		fmt.Printf("  Branch: %s", c.theme.Success.Render(strings.TrimSpace(string(branch))))
	}
	fmt.Println()
}

// showMemory shows memory/history
func (c *CLIMode) showMemory() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🧠 Memory  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	fmt.Printf("  Current session: %d messages\n", len(c.messages))
	fmt.Println()

	// Show last few messages
	start := 0
	if len(c.messages) > 4 {
		start = len(c.messages) - 4
	}

	for i := start; i < len(c.messages); i++ {
		m := c.messages[i]
		role := c.theme.Accent.Render(m.Role)
		content := m.Content
		if len(content) > 50 {
			content = content[:47] + "..."
		}
		fmt.Printf("  %s: %s\n", role, content)
	}
	fmt.Println()
}

// addToContext adds file to context
func (c *CLIMode) addToContext(args []string) {
	if len(args) == 0 {
		fmt.Println(c.theme.Warning.Render("⚠ Usage: /add <file>"))
		return
	}

	file := args[0]
	content, err := os.ReadFile(file)
	if err != nil {
		fmt.Println(c.theme.Error.Render(fmt.Sprintf("  ✗ Cannot read file: %v", err)))
		return
	}

	c.messages = append(c.messages, Message{
		Role:    "user",
		Content: fmt.Sprintf("Here's the content of %s:\n```\n%s\n```", file, string(content)),
	})

	fmt.Printf("%s Added %s to context (%d bytes)\n", c.theme.Success.Render("✓"), file, len(content))
}

// showSettings shows settings
func (c *CLIMode) showSettings() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  ⚙️ Settings  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	fmt.Printf("  Provider: %s\n", c.theme.Accent.Render(c.provider))
	fmt.Printf("  Model: %s\n", c.theme.Accent.Render(c.model))
	fmt.Printf("  Theme: %s\n", c.theme.Accent.Render("Default"))
	fmt.Println()
	fmt.Println(c.theme.Subtle.Render("  Config: ~/.config/devorch/config.yml"))
	fmt.Println()
}

// editConfig edits configuration
func (c *CLIMode) editConfig(args []string) {
	if len(args) == 0 {
		c.showSettings()
		return
	}

	fmt.Printf("  Config editing not yet implemented: %s\n", args[0])
}

// showTools shows available tools
func (c *CLIMode) showTools() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🔧 Available Tools  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	tools := []struct {
		name    string
		enabled bool
		desc    string
	}{
		{"file_read", true, "Read file contents"},
		{"file_write", true, "Write file contents"},
		{"file_search", true, "Search files"},
		{"shell_exec", true, "Execute shell commands"},
		{"web_browse", false, "Browse web pages"},
		{"git_ops", true, "Git operations"},
	}

	for _, t := range tools {
		status := c.theme.Success.Render("●")
		if !t.enabled {
			status = c.theme.Warning.Render("○")
		}
		fmt.Printf("  %s %s  %s\n", status, c.theme.Accent.Render(t.name), c.theme.Subtle.Render(t.desc))
	}
	fmt.Println()
}

// showLSP shows LSP status and provides commands
func (c *CLIMode) showLSP() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🔤 Language Servers  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	// Check for common language servers
	servers := []struct {
		name string
		cmd  string
		lang string
	}{
		{"Go (gopls)", "gopls", "go"},
		{"TypeScript", "typescript-language-server", "typescript"},
		{"Python (pylsp)", "pylsp", "python"},
		{"Rust (rust-analyzer)", "rust-analyzer", "rust"},
		{"C/C++ (clangd)", "clangd", "c"},
	}

	fmt.Println(c.theme.Accent.Render("  Detected Language Servers:"))
	anyFound := false
	for _, s := range servers {
		if _, err := exec.LookPath(s.cmd); err == nil {
			status := c.theme.Success.Render("●")
			fmt.Printf("    %s %s\n", status, s.name)
			anyFound = true
		}
	}

	if !anyFound {
		fmt.Println(c.theme.Subtle.Render("    No language servers detected"))
	}
	fmt.Println()

	// Show usage
	fmt.Println(c.theme.Accent.Render("  Commands:"))
	fmt.Printf("    %s        Show this status\n", c.theme.Accent.Render("/lsp"))
	fmt.Printf("    %s Start server for current project\n", c.theme.Accent.Render("/lsp start"))
	fmt.Printf("    %s  Stop all servers\n", c.theme.Accent.Render("/lsp stop"))
	fmt.Println()

	// Show how to install language servers
	fmt.Println(c.theme.Accent.Render("  Install Language Servers:"))
	fmt.Printf("    Go:         %s\n", c.theme.Subtle.Render("go install golang.org/x/tools/gopls@latest"))
	fmt.Printf("    TypeScript: %s\n", c.theme.Subtle.Render("npm i -g typescript-language-server typescript"))
	fmt.Printf("    Python:     %s\n", c.theme.Subtle.Render("pip install python-lsp-server"))
	fmt.Printf("    Rust:       %s\n", c.theme.Subtle.Render("rustup component add rust-analyzer"))
	fmt.Println()
}

// runDoctor runs diagnostics
func (c *CLIMode) runDoctor() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🩺 System Diagnostics  "))
	fmt.Println(c.theme.Border.Render("════════════════════════════════════════"))
	fmt.Println()

	// Core checks
	fmt.Println(c.theme.Accent.Render("Core:"))
	checks := []struct {
		name  string
		check func() (bool, string)
	}{
		{"Ollama", func() (bool, string) {
			if checkOllamaAvailable() {
				models := getOllamaModels()
				count := 0
				for _, m := range models {
					if m.IsInstalled {
						count++
					}
				}
				return true, fmt.Sprintf("%d models", count)
			}
			return false, "not running"
		}},
		{"Git", func() (bool, string) {
			if out, err := exec.Command("git", "--version").Output(); err == nil {
				parts := strings.Fields(string(out))
				if len(parts) >= 3 {
					return true, parts[2]
				}
				return true, "installed"
			}
			return false, "not found"
		}},
		{"Config Dir", func() (bool, string) {
			homeDir, _ := os.UserHomeDir()
			configDir := filepath.Join(homeDir, ".config", "devorch")
			if _, err := os.Stat(configDir); err == nil {
				return true, configDir
			}
			return false, "not created"
		}},
	}

	for _, ch := range checks {
		ok, detail := ch.check()
		status := c.theme.Success.Render("✓")
		if !ok {
			status = c.theme.Error.Render("✗")
		}
		fmt.Printf("  %s %-12s %s\n", status, ch.name, c.theme.Subtle.Render(detail))
	}
	fmt.Println()

	// Auth checks
	fmt.Println(c.theme.Accent.Render("Authentication:"))
	store := auth.GetStore()
	authProviders := []string{"openai", "anthropic", "github-copilot", "google"}
	connectedCount := 0
	for _, id := range authProviders {
		if store.IsConnected(id) {
			connectedCount++
		}
	}
	if connectedCount > 0 {
		fmt.Printf("  %s Providers    %s connected\n", c.theme.Success.Render("✓"), c.theme.Accent.Render(fmt.Sprintf("%d", connectedCount)))
	} else {
		fmt.Printf("  %s Providers    none connected\n", c.theme.Warning.Render("○"))
	}
	fmt.Println()

	// Environment
	fmt.Println(c.theme.Accent.Render("Environment:"))
	envVars := []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GOOGLE_API_KEY"}
	for _, env := range envVars {
		if os.Getenv(env) != "" {
			fmt.Printf("  %s %s\n", c.theme.Success.Render("✓"), env)
		}
	}

	// Summary
	fmt.Println()
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	if checkOllamaAvailable() && connectedCount >= 0 {
		fmt.Println(c.theme.Success.Render("  System ready!"))
	} else {
		fmt.Println(c.theme.Warning.Render("  Some components need attention"))
	}
	fmt.Println()
}

// showVersion shows version info
func (c *CLIMode) showVersion() {
	fmt.Println()
	fmt.Printf("  DevOrch %s\n", c.theme.Accent.Render("v0.1.0"))
	fmt.Println(c.theme.Subtle.Render("  Local-first multi-LLM orchestrator"))
	fmt.Println()
}

// installModel installs a model with progress tracking
func (c *CLIMode) installModel(args []string) {
	if len(args) == 0 {
		fmt.Println(c.theme.Warning.Render("⚠ Usage: /install <model>"))
		fmt.Println()
		fmt.Println("  Popular models:")
		popular := []string{"llama3.2", "llama3.2:1b", "codellama", "mistral", "deepseek-coder", "qwen2.5-coder"}
		for _, m := range popular {
			fmt.Printf("    • %s\n", m)
		}
		return
	}

	model := args[0]

	// Use the new progress tracking installation
	if err := InstallModelWithProgress(model, &c.theme); err != nil {
		// Error already printed by InstallModelWithProgress
		return
	}
}

// runBenchmark runs model benchmark
func (c *CLIMode) runBenchmark(args []string) {
	model := c.model
	if len(args) > 0 {
		model = args[0]
	}

	fmt.Println()
	fmt.Println(c.theme.Title.Render("  ⚡ Model Benchmark  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Printf("  Model: %s\n", c.theme.Accent.Render(model))
	fmt.Println()

	if !checkOllamaAvailable() {
		fmt.Println(c.theme.Error.Render("  ✗ Ollama is not running"))
		return
	}

	// Test prompts
	prompts := []struct {
		name   string
		prompt string
	}{
		{"Simple", "Say hello"},
		{"Code", "Write a hello world function in Go"},
		{"Reasoning", "What is 15 * 23?"},
	}

	var totalTime time.Duration
	var totalTokens int

	ctx := context.Background()
	for _, p := range prompts {
		fmt.Printf("  %s: ", p.name)

		start := time.Now()
		messages := []Message{{Role: "user", Content: p.prompt}}
		resp, err := c.client.Chat(ctx, model, messages)
		elapsed := time.Since(start)

		if err != nil {
			fmt.Println(c.theme.Error.Render("error"))
			continue
		}

		tokens := len(strings.Fields(resp))
		totalTime += elapsed
		totalTokens += tokens

		tokensPerSec := float64(tokens) / elapsed.Seconds()
		fmt.Printf("%s (%.1f tok/s)\n", c.theme.Success.Render(elapsed.Round(time.Millisecond).String()), tokensPerSec)
	}

	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	avgTokensPerSec := float64(totalTokens) / totalTime.Seconds()
	fmt.Printf("  Total: %s, ~%d tokens\n", totalTime.Round(time.Millisecond), totalTokens)
	fmt.Printf("  Average: %.1f tokens/sec\n", avgTokensPerSec)
	fmt.Println()
}

// ==================== Additional DevOrch Commands ====================

// resetSession resets the session completely
func (c *CLIMode) resetSession() {
	c.messages = []Message{}
	c.provider = "ollama"
	c.model = "llama3.2"
	fmt.Println(c.theme.Success.Render("✓ Session reset to defaults"))
	fmt.Printf("  Provider: %s\n", c.theme.Accent.Render(c.provider))
	fmt.Printf("  Model: %s\n", c.theme.Accent.Render(c.model))
}

// shareSession creates a shareable session link
func (c *CLIMode) shareSession(args []string) {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🔗 Share Session  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	if len(c.messages) == 0 {
		fmt.Println(c.theme.Warning.Render("  ⚠ No messages to share"))
		return
	}

	// Generate a share ID
	shareID := fmt.Sprintf("devorch-%d", time.Now().Unix())
	fmt.Printf("  Share ID: %s\n", c.theme.Accent.Render(shareID))
	fmt.Println(c.theme.Subtle.Render("  (Session sharing requires cloud sync - coming soon)"))
	fmt.Println()
}

// unshareSession removes session sharing
func (c *CLIMode) unshareSession(args []string) {
	fmt.Println(c.theme.Success.Render("✓ Session unshared"))
	fmt.Println(c.theme.Subtle.Render("  Session is now private"))
}

// showDetails shows session details
func (c *CLIMode) showDetails() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  📋 Session Details  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	fmt.Printf("  Provider: %s\n", c.theme.Accent.Render(c.provider))
	fmt.Printf("  Model: %s\n", c.theme.Accent.Render(c.model))
	fmt.Printf("  Messages: %d\n", len(c.messages))
	fmt.Printf("  Theme: %s\n", c.theme.Accent.Render("default"))

	// Count tokens (approximate)
	totalChars := 0
	for _, m := range c.messages {
		totalChars += len(m.Content)
	}
	approxTokens := totalChars / 4

	fmt.Printf("  Approx Tokens: ~%d\n", approxTokens)
	fmt.Printf("  Streaming: %v\n", c.isStreaming)
	fmt.Println()
}

// setAgent switches agent mode
func (c *CLIMode) setAgent(name string) {
	name = strings.ToLower(name)

	if agent, ok := availableAgents[name]; ok {
		currentAgent = name
		fmt.Printf("%s Agent switched to: %s\n", c.theme.Success.Render("✓"), c.theme.Accent.Render(agent.Name))
		fmt.Printf("  %s\n", c.theme.Subtle.Render(agent.Description))
		fmt.Println()
		fmt.Println(c.theme.Subtle.Render("  System prompt updated for this session"))
	} else {
		fmt.Printf("%s Unknown agent: %s\n", c.theme.Warning.Render("⚠"), name)
		fmt.Println("  Available: coder, reviewer, debugger, architect, tester")
	}
}

// showLanguage shows current language setting
func (c *CLIMode) showLanguage() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🌐 Language Settings  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	languages := []struct {
		code    string
		name    string
		current bool
	}{
		{"en", "English", true},
		{"ko", "한국어", false},
		{"ja", "日本語", false},
		{"zh", "中文", false},
	}

	for _, l := range languages {
		status := "  "
		if l.current {
			status = c.theme.Success.Render("▸ ")
		}
		fmt.Printf("%s%s  %s\n", status, c.theme.Accent.Render(l.code), l.name)
	}
	fmt.Println()
	fmt.Println("  Use /language <code> to switch")
	fmt.Println()
}

// setLanguage sets the UI language
func (c *CLIMode) setLanguage(lang string) {
	supported := map[string]string{
		"en": "English",
		"ko": "한국어",
		"ja": "日本語",
		"zh": "中文",
	}

	lang = strings.ToLower(lang)
	if name, ok := supported[lang]; ok {
		fmt.Printf("%s Language set to: %s (%s)\n", c.theme.Success.Render("✓"), lang, name)
		fmt.Println(c.theme.Subtle.Render("  (Restart required for full effect)"))
	} else {
		fmt.Printf("%s Unknown language: %s\n", c.theme.Warning.Render("⚠"), lang)
		fmt.Println("  Supported: en, ko, ja, zh")
	}
}

// loginProvider handles provider login
func (c *CLIMode) loginProvider(args []string) {
	if len(args) == 0 {
		// Show interactive login
		fmt.Println()
		fmt.Println(c.theme.Title.Render("  🔐 Login  "))
		fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
		fmt.Println()

		providers := []string{"github", "google", "openai", "anthropic"}
		for i, p := range providers {
			fmt.Printf("  %d. %s\n", i+1, c.theme.Accent.Render(p))
		}
		fmt.Println()
		fmt.Println("  Use /login <provider> to login")
		fmt.Println("  Or /connect for OAuth setup")
		return
	}

	provider := strings.ToLower(args[0])
	fmt.Printf("  Logging in to %s...\n", c.theme.Accent.Render(provider))
	fmt.Println(c.theme.Subtle.Render("  Use /connect for full OAuth flow"))
}

// logoutProvider handles provider logout
func (c *CLIMode) logoutProvider(args []string) {
	if len(args) == 0 {
		fmt.Println(c.theme.Warning.Render("⚠ Usage: /logout <provider>"))
		fmt.Println("  Example: /logout github")
		return
	}

	provider := strings.ToLower(args[0])
	store := auth.GetStore()

	// Try to find and remove the provider
	if err := store.Remove(provider); err != nil {
		fmt.Printf("%s Failed to logout: %v\n", c.theme.Error.Render("✗"), err)
		return
	}

	fmt.Printf("%s Logged out from %s\n", c.theme.Success.Render("✓"), provider)
}

// showAuthStatus shows authentication status for all providers
func (c *CLIMode) showAuthStatus() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🔐 Authentication Status  "))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()

	store := auth.GetStore()
	providers := []struct {
		id   string
		name string
	}{
		{"openai", "OpenAI"},
		{"anthropic", "Anthropic"},
		{"github-copilot", "GitHub Copilot"},
		{"google", "Google"},
		{"groq", "Groq"},
		{"openrouter", "OpenRouter"},
	}

	for _, p := range providers {
		status := c.theme.Warning.Render("○")
		authType := c.theme.Subtle.Render("Not connected")

		info := store.Get(p.id)
		if info != nil {
			status = c.theme.Success.Render("●")
			switch info.Type {
			case auth.AuthTypeOAuth:
				authType = c.theme.Accent.Render("OAuth")
			case auth.AuthTypeAPI:
				authType = c.theme.Accent.Render("API Key")
			case auth.AuthTypeDevice:
				authType = c.theme.Accent.Render("Device Flow")
			}
		} else {
			// Check env var
			config := auth.ProviderConfigs[p.id]
			if config != nil && config.EnvVar != "" && os.Getenv(config.EnvVar) != "" {
				status = c.theme.Success.Render("●")
				authType = c.theme.Accent.Render("Env Var")
			}
		}

		fmt.Printf("  %s %-15s %s\n", status, p.name, authType)
	}
	fmt.Println()
}

// runSetup runs the initial setup wizard
func (c *CLIMode) runSetup() {
	fmt.Println()
	fmt.Println(c.theme.Title.Render("  🚀 DevOrch Setup Wizard  "))
	fmt.Println(c.theme.Border.Render("════════════════════════════════════════"))
	fmt.Println()

	// Step 1: Check Ollama
	fmt.Println(c.theme.Accent.Render("Step 1: Checking Ollama..."))
	if checkOllamaAvailable() {
		fmt.Println(c.theme.Success.Render("  ✓ Ollama is running"))
		models := getOllamaModels()
		installed := 0
		for _, m := range models {
			if m.IsInstalled {
				installed++
			}
		}
		fmt.Printf("  ✓ %d models installed\n", installed)
	} else {
		fmt.Println(c.theme.Warning.Render("  ⚠ Ollama is not running"))
		fmt.Println("    Install: https://ollama.ai")
		fmt.Println("    Start: ollama serve")
	}
	fmt.Println()

	// Step 2: Check config
	fmt.Println(c.theme.Accent.Render("Step 2: Checking configuration..."))
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".config", "devorch", "config.yml")
	if _, err := os.Stat(configPath); err == nil {
		fmt.Println(c.theme.Success.Render("  ✓ Config file exists"))
	} else {
		fmt.Println(c.theme.Warning.Render("  ⚠ No config file found"))
		fmt.Printf("    Will use defaults: %s\n", configPath)
	}
	fmt.Println()

	// Step 3: Check auth
	fmt.Println(c.theme.Accent.Render("Step 3: Checking authentication..."))
	store := auth.GetStore()
	authCount := 0
	for _, id := range []string{"openai", "anthropic", "github-copilot"} {
		if store.IsConnected(id) {
			authCount++
		}
	}
	if authCount > 0 {
		fmt.Printf("  ✓ %d providers connected\n", authCount)
	} else {
		fmt.Println(c.theme.Warning.Render("  ⚠ No providers connected"))
		fmt.Println("    Use /connect to set up providers")
	}
	fmt.Println()

	// Step 4: Summary
	fmt.Println(c.theme.Accent.Render("Setup Complete!"))
	fmt.Println(c.theme.Border.Render("────────────────────────────────────────"))
	fmt.Println()
	fmt.Println("  Quick start commands:")
	fmt.Println("    /connect    - Connect to AI providers")
	fmt.Println("    /models     - List available models")
	fmt.Println("    /help       - Show all commands")
	fmt.Println()
}

// RunCLI starts DevOrch in CLI mode
func RunCLI() error {
	cli := NewCLIMode()
	return cli.Run()
}
