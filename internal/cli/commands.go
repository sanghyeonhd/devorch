package cli

// SlashCommand represents a CLI slash command
type SlashCommand struct {
	Command     string
	Description string
	Category    string
	Usage       string
	Examples    []string
}

// GetSlashCommands returns all available slash commands
func GetSlashCommands() []SlashCommand {
	return []SlashCommand{
		// Chat Commands
		{Command: "clear", Description: "Clear the current conversation", Category: "Chat", Usage: "/clear", Examples: []string{"/clear"}},
		{Command: "save", Description: "Save current conversation", Category: "Chat", Usage: "/save [name]", Examples: []string{"/save", "/save important-chat"}},
		{Command: "load", Description: "Load a saved conversation", Category: "Chat", Usage: "/load <name>", Examples: []string{"/load important-chat"}},
		{Command: "export", Description: "Export conversation to file", Category: "Chat", Usage: "/export [format]", Examples: []string{"/export", "/export markdown", "/export json"}},

		// Model Commands
		{Command: "model", Description: "Manage AI models", Category: "Model", Usage: "/model <action>", Examples: []string{"/model list", "/model switch gpt-4", "/model test"}},
		{Command: "provider", Description: "Manage AI providers", Category: "Model", Usage: "/provider <action>", Examples: []string{"/provider list", "/provider connect openai", "/provider status"}},

		// Context Commands
		{Command: "context", Description: "Manage conversation context", Category: "Context", Usage: "/context <action>", Examples: []string{"/context show", "/context add file.txt", "/context clear"}},
		{Command: "memory", Description: "Manage conversation memory", Category: "Context", Usage: "/memory <action>", Examples: []string{"/memory show", "/memory save", "/memory clear"}},

		// Code Commands
		{Command: "code", Description: "Code generation and analysis", Category: "Code", Usage: "/code <action>", Examples: []string{"/code generate", "/code review", "/code format"}},
		{Command: "file", Description: "File operations", Category: "Code", Usage: "/file <action>", Examples: []string{"/file read", "/file write", "/file edit"}},

		// Agent & Orchestration Commands
		{Command: "agent", Description: "Agent management and orchestration", Category: "Agent", Usage: "/agent <action>", Examples: []string{"/agent list", "/agent create", "/agent task"}},
		{Command: "orchestrate", Description: "Advanced task orchestration", Category: "Agent", Usage: "/orchestrate <action>", Examples: []string{"/orchestrate plan", "/orchestrate execute", "/orchestrate status"}},
		{Command: "task", Description: "Background task management", Category: "Agent", Usage: "/task <action>", Examples: []string{"/task list", "/task status", "/task cancel"}},
		{Command: "delegate", Description: "Task delegation system", Category: "Agent", Usage: "/delegate <action>", Examples: []string{"/delegate task", "/delegate status", "/delegate cancel"}},

		// Learning & Analytics Commands
		{Command: "learn", Description: "Multi-LLM learning system", Category: "Learning", Usage: "/learn <action>", Examples: []string{"/learn status", "/learn optimize", "/learn stats"}},
		{Command: "analytics", Description: "Performance analytics and attribution", Category: "Analytics", Usage: "/analytics <action>", Examples: []string{"/analytics performance", "/analytics drift", "/analytics costs"}},
		{Command: "quality", Description: "Quality evaluation system", Category: "Analytics", Usage: "/quality <action>", Examples: []string{"/quality check", "/quality metrics", "/quality report"}},
		{Command: "benchmark", Description: "Model benchmarking and evaluation", Category: "Analytics", Usage: "/benchmark <action>", Examples: []string{"/benchmark run", "/benchmark compare", "/benchmark report"}},

		// Hardware & Setup Commands
		{Command: "hardware", Description: "Hardware detection and optimization", Category: "Hardware", Usage: "/hardware <action>", Examples: []string{"/hardware detect", "/hardware optimize", "/hardware status"}},
		{Command: "autosetup", Description: "Automated system setup", Category: "Hardware", Usage: "/autosetup <action>", Examples: []string{"/autosetup run", "/autosetup status", "/autosetup models"}},
		{Command: "install", Description: "Model installation and management", Category: "Hardware", Usage: "/install <action>", Examples: []string{"/install model llama3.2", "/install list", "/install status"}},

		// Development & Tools Commands
		{Command: "tool", Description: "Tool management and execution", Category: "Tools", Usage: "/tool <action>", Examples: []string{"/tool list", "/tool exec", "/tool reload"}},
		{Command: "lsp", Description: "Language Server Protocol integration", Category: "Tools", Usage: "/lsp <action>", Examples: []string{"/lsp status", "/lsp diagnostics", "/lsp symbols"}},
		{Command: "editor", Description: "Editor integration and synchronization", Category: "Tools", Usage: "/editor <action>", Examples: []string{"/editor connect", "/editor sync", "/editor status"}},
		{Command: "terminal", Description: "Terminal and PTY management", Category: "Tools", Usage: "/terminal <action>", Examples: []string{"/terminal new", "/terminal list", "/terminal attach"}},
		{Command: "shell", Description: "Shell session management", Category: "Tools", Usage: "/shell <action>", Examples: []string{"/shell new", "/shell list", "/shell exec"}},

		// Management & Monitoring Commands
		{Command: "workspace", Description: "Workspace and project management", Category: "Management", Usage: "/workspace <action>", Examples: []string{"/workspace create", "/workspace list", "/workspace switch"}},
		{Command: "history", Description: "File history and version tracking", Category: "Management", Usage: "/history <action>", Examples: []string{"/history list", "/history diff", "/history rollback"}},
		{Command: "session", Description: "Session management", Category: "Management", Usage: "/session <action>", Examples: []string{"/session list", "/session fork", "/session merge"}},
		{Command: "plugin", Description: "Plugin and extension management", Category: "Management", Usage: "/plugin <action>", Examples: []string{"/plugin list", "/plugin install", "/plugin enable"}},
		{Command: "experiment", Description: "A/B testing and experiments", Category: "Management", Usage: "/experiment <action>", Examples: []string{"/experiment create", "/experiment run", "/experiment results"}},

		// Networking & Integration Commands
		{Command: "websocket", Description: "WebSocket server management", Category: "Network", Usage: "/websocket <action>", Examples: []string{"/websocket status", "/websocket clients", "/websocket broadcast"}},
		{Command: "mcp", Description: "Model Context Protocol management", Category: "Network", Usage: "/mcp <action>", Examples: []string{"/mcp list", "/mcp connect", "/mcp status"}},
		{Command: "api", Description: "API server and endpoints", Category: "Network", Usage: "/api <action>", Examples: []string{"/api status", "/api endpoints", "/api logs"}},
		{Command: "events", Description: "Event bus and messaging", Category: "Network", Usage: "/events <action>", Examples: []string{"/events list", "/events subscribe", "/events publish"}},

		// Cost & Billing Commands
		{Command: "billing", Description: "Cost tracking and billing", Category: "Billing", Usage: "/billing <action>", Examples: []string{"/billing status", "/billing costs", "/billing report"}},
		{Command: "budget", Description: "Budget management and alerts", Category: "Billing", Usage: "/budget <action>", Examples: []string{"/budget set", "/budget status", "/budget alerts"}},
		{Command: "usage", Description: "Resource usage tracking", Category: "Billing", Usage: "/usage <action>", Examples: []string{"/usage tokens", "/usage models", "/usage costs"}},

		// Diagnostics & System Commands
		{Command: "doctor", Description: "System health diagnostics", Category: "Diagnostics", Usage: "/doctor <action>", Examples: []string{"/doctor check", "/doctor fix", "/doctor report"}},
		{Command: "logs", Description: "System logs and monitoring", Category: "Diagnostics", Usage: "/logs <action>", Examples: []string{"/logs view", "/logs filter", "/logs export"}},
		{Command: "metrics", Description: "Performance metrics and monitoring", Category: "Diagnostics", Usage: "/metrics <action>", Examples: []string{"/metrics show", "/metrics export", "/metrics alerts"}},
		{Command: "debug", Description: "Debug mode and troubleshooting", Category: "Diagnostics", Usage: "/debug <action>", Examples: []string{"/debug enable", "/debug trace", "/debug dump"}},

		// International & Accessibility Commands
		{Command: "i18n", Description: "Internationalization and localization", Category: "I18n", Usage: "/i18n <action>", Examples: []string{"/i18n set ko", "/i18n list", "/i18n reload"}},
		{Command: "theme", Description: "Theme and appearance management", Category: "I18n", Usage: "/theme <action>", Examples: []string{"/theme set dark", "/theme list", "/theme custom"}},

		// System Commands
		{Command: "system", Description: "System configuration", Category: "System", Usage: "/system <action>", Examples: []string{"/system status", "/system config", "/system logs"}},
		{Command: "auth", Description: "Authentication management", Category: "System", Usage: "/auth <action>", Examples: []string{"/auth status", "/auth login", "/auth logout"}},
		{Command: "config", Description: "Configuration management", Category: "System", Usage: "/config <action>", Examples: []string{"/config get", "/config set", "/config reset"}},
		{Command: "permission", Description: "Permission and access control", Category: "System", Usage: "/permission <action>", Examples: []string{"/permission list", "/permission grant", "/permission revoke"}},

		// Help Commands
		{Command: "help", Description: "Show help information", Category: "Help", Usage: "/help [command]", Examples: []string{"/help", "/help model"}},
		{Command: "version", Description: "Show version information", Category: "Help", Usage: "/version", Examples: []string{"/version"}},
		{Command: "exit", Description: "Exit the application", Category: "Help", Usage: "/exit", Examples: []string{"/exit"}},
		{Command: "quit", Description: "Quit the application", Category: "Help", Usage: "/quit", Examples: []string{"/quit"}},
	}
}

// FilterCommands filters commands by search term
func FilterCommands(filter string) []SlashCommand {
	if filter == "" {
		return GetSlashCommands()
	}

	var filtered []SlashCommand
	for _, cmd := range GetSlashCommands() {
		if contains(cmd.Command, filter) || contains(cmd.Description, filter) || contains(cmd.Category, filter) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Provider represents a connection provider
type Provider struct {
	ID          string
	Name        string
	Type        string
	Status      string
	Description string
	Category    string
	AuthType    string
	EnvVar      string
	IsConnected bool
	Config      map[string]interface{}
}

// GetConnectProviders returns available connection providers
func GetConnectProviders() []Provider {
	return []Provider{
		{
			ID:          "openai",
			Name:        "openai",
			Type:        "llm",
			Status:      "available",
			Description: "OpenAI GPT models",
			Category:    "Popular",
			AuthType:    "api_key",
			EnvVar:      "OPENAI_API_KEY",
			IsConnected: false,
			Config:      map[string]interface{}{"api_key": "required"},
		},
		{
			ID:          "anthropic",
			Name:        "anthropic",
			Type:        "llm",
			Status:      "available",
			Description: "Anthropic Claude models",
			Category:    "Popular",
			AuthType:    "api_key",
			EnvVar:      "ANTHROPIC_API_KEY",
			IsConnected: false,
			Config:      map[string]interface{}{"api_key": "required"},
		},
		{
			ID:          "google",
			Name:        "google",
			Type:        "llm",
			Status:      "available",
			Description: "Google Gemini models",
			Category:    "Popular",
			AuthType:    "api_key",
			EnvVar:      "GOOGLE_API_KEY",
			IsConnected: false,
			Config:      map[string]interface{}{"api_key": "required"},
		},
		{
			ID:          "ollama",
			Name:        "ollama",
			Type:        "llm",
			Status:      "available",
			Description: "Local Ollama models",
			Category:    "Local",
			AuthType:    "none",
			EnvVar:      "",
			IsConnected: false,
			Config:      map[string]interface{}{"url": "http://localhost:11434"},
		},
		{
			ID:          "github",
			Name:        "github",
			Type:        "code",
			Status:      "available",
			Description: "GitHub Copilot integration",
			Category:    "Code",
			AuthType:    "oauth",
			EnvVar:      "GITHUB_TOKEN",
			IsConnected: false,
			Config:      map[string]interface{}{"token": "required"},
		},
	}
}
