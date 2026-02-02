// Package tui provides terminal user interface components.
package tui

import (
	"context"
	"devorch/internal/session"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SlashCommand represents a slash command.
type SlashCommand struct {
	Name        string
	Description string
	Category    string
	Handler     func(m *Model) tea.Cmd
}

// CommandCategory represents a category of commands.
type CommandCategory struct {
	Name     string
	Commands []SlashCommand
}

// commandItem implements list.Item for bubbletea list.
type commandItem struct {
	name        string
	description string
	category    string
}

func (i commandItem) Title() string       { return "/" + i.name }
func (i commandItem) Description() string { return i.description }
func (i commandItem) FilterValue() string { return i.name }

// GetSlashCommands returns all available slash commands.
// Synchronized with cli_mode.go - 16 core command groups + shortcuts
func GetSlashCommands() []SlashCommand {
	commands := []SlashCommand{
		// Quick Commands (shortcuts)
		{Name: "help", Description: "Show help", Category: "Quick", Handler: cmdHelp},
		{Name: "exit", Description: "Exit DevOrch", Category: "Quick", Handler: cmdQuit},
		{Name: "quit", Description: "Exit DevOrch (alias)", Category: "Quick", Handler: cmdQuit},
		{Name: "clear", Description: "Clear chat", Category: "Quick", Handler: cmdClear},
		{Name: "cls", Description: "Clear chat (alias)", Category: "Quick", Handler: cmdClear},
		{Name: "new", Description: "New session", Category: "Quick", Handler: cmdNew},
		{Name: "init", Description: "Initialize project", Category: "Quick", Handler: cmdInit},
		{Name: "history", Description: "Show chat history", Category: "Quick", Handler: cmdHistory},
		{Name: "mode", Description: "Switch work mode (ask/edit/agent/plan)", Category: "Quick", Handler: cmdModeGroup},
		{Name: "multimodel", Description: "Select multiple models for comparison", Category: "Quick", Handler: cmdMultiModel},
		{Name: "preset", Description: "Model preset management (save/load/list)", Category: "Quick", Handler: cmdPresetGroup},

		// Main Command Groups (16 core groups)
		{Name: "session", Description: "Session management (new, list, save, export...)", Category: "Main", Handler: cmdSessionGroup},
		{Name: "model", Description: "Model management (list, set, install, bench)", Category: "Main", Handler: cmdModelGroup},
		{Name: "provider", Description: "Provider management (list, connect, disconnect)", Category: "Main", Handler: cmdProviderGroup},
		{Name: "agent", Description: "Agent management (list, set)", Category: "Main", Handler: cmdAgentGroup},
		{Name: "code", Description: "Code/file operations (files, grep, diff, review)", Category: "Main", Handler: cmdCodeGroup},
		{Name: "context", Description: "Context management (add, memory, thinking)", Category: "Main", Handler: cmdContextGroup},
		{Name: "tools", Description: "Tool management (mcp, lsp)", Category: "Main", Handler: cmdToolsGroup},
		{Name: "config", Description: "Configuration (theme, lang, edit)", Category: "Main", Handler: cmdConfigGroup},
		{Name: "auth", Description: "Authentication (login, logout, status)", Category: "Main", Handler: cmdAuthGroup},
		{Name: "system", Description: "System (status, setup, doctor, version)", Category: "Main", Handler: cmdSystemGroup},

		// Legacy commands (with deprecation tips)
		{Name: "models", Description: "[Deprecated] Use /model list", Category: "Legacy", Handler: cmdModels},
		{Name: "themes", Description: "[Deprecated] Use /config themes", Category: "Legacy", Handler: cmdTheme},
		{Name: "status", Description: "[Deprecated] Use /system status", Category: "Legacy", Handler: cmdStatus},
		{Name: "agents", Description: "[Deprecated] Use /agent list", Category: "Legacy", Handler: cmdAgents},
		{Name: "sessions", Description: "[Deprecated] Use /session list", Category: "Legacy", Handler: cmdSessions},
		{Name: "files", Description: "[Deprecated] Use /code files", Category: "Legacy", Handler: cmdFiles},
		{Name: "grep", Description: "[Deprecated] Use /code grep", Category: "Legacy", Handler: cmdGrep},
		{Name: "diff", Description: "[Deprecated] Use /code diff", Category: "Legacy", Handler: cmdDiff},
		{Name: "doctor", Description: "[Deprecated] Use /system doctor", Category: "Legacy", Handler: cmdDoctor},
		{Name: "version", Description: "[Deprecated] Use /system version", Category: "Legacy", Handler: cmdVersion},
		{Name: "install", Description: "[Deprecated] Use /model install", Category: "Legacy", Handler: cmdInstall},
		{Name: "bench", Description: "[Deprecated] Use /model bench", Category: "Legacy", Handler: cmdBench},
		{Name: "login", Description: "[Deprecated] Use /auth login", Category: "Legacy", Handler: cmdLogin},
		{Name: "logout", Description: "[Deprecated] Use /auth logout", Category: "Legacy", Handler: cmdLogout},
		{Name: "setup", Description: "[Deprecated] Use /system setup", Category: "Legacy", Handler: cmdSetup},
		{Name: "language", Description: "[Deprecated] Use /config lang", Category: "Legacy", Handler: cmdLanguage},

		// Subcommands (for autocomplete)
		{Name: "save", Description: "Save current session", Category: "Session", Handler: cmdSave},
		{Name: "export", Description: "Export session", Category: "Session", Handler: cmdExport},
		{Name: "share", Description: "Share session", Category: "Session", Handler: cmdShare},
		{Name: "unshare", Description: "Unshare session", Category: "Session", Handler: cmdUnshare},
		{Name: "compact", Description: "Compact history", Category: "Session", Handler: cmdCompact},
		{Name: "reset", Description: "Reset session", Category: "Session", Handler: cmdReset},
		{Name: "undo", Description: "Undo last change", Category: "Session", Handler: cmdUndo},
		{Name: "redo", Description: "Redo last undo", Category: "Session", Handler: cmdRedo},
		{Name: "details", Description: "Show session details", Category: "Session", Handler: cmdDetails},
		{Name: "review", Description: "Review code", Category: "Code", Handler: cmdReview},
		{Name: "editor", Description: "Open in editor", Category: "Code", Handler: cmdEditor},
		{Name: "git", Description: "Git operations", Category: "Code", Handler: cmdGit},
		{Name: "add", Description: "Add to context", Category: "Context", Handler: cmdAdd},
		{Name: "memory", Description: "Show memory", Category: "Context", Handler: cmdMemory},
		{Name: "thinking", Description: "Toggle thinking mode", Category: "Context", Handler: cmdThinking},
		{Name: "connect", Description: "Connect provider", Category: "Provider", Handler: cmdConnect},
		{Name: "lsp", Description: "LSP status", Category: "Tools", Handler: cmdLSP},
		{Name: "mcps", Description: "MCP servers", Category: "Tools", Handler: cmdMCP},
		{Name: "settings", Description: "Show settings", Category: "Config", Handler: cmdSettings},
		{Name: "theme", Description: "Switch theme", Category: "Config", Handler: cmdThemeSwitch},
		{Name: "copy", Description: "Copy to clipboard", Category: "Edit", Handler: cmdCopy},
		{Name: "paste", Description: "Paste from clipboard", Category: "Edit", Handler: cmdPaste},
		{Name: "retry", Description: "Retry last message", Category: "Edit", Handler: cmdRetry},
		{Name: "permissions", Description: "Manage tool permissions", Category: "Tools", Handler: cmdPermissions},
		{Name: "perm", Description: "Manage tool permissions (alias)", Category: "Tools", Handler: cmdPermissions},
	}

	// Load custom commands (OpenCode 스타일)
	customCommands := LoadCustomCommands()
	commands = append(commands, customCommands...)

	return commands
}

// CustomCommandPrefix constants
const (
	UserCommandPrefix    = "user:"
	ProjectCommandPrefix = "project:"
)

// LoadCustomCommands loads custom commands from user and project directories
func LoadCustomCommands() []SlashCommand {
	var commands []SlashCommand

	// Load user commands from ~/.config/devorch/commands/
	homeDir, err := os.UserHomeDir()
	if err == nil {
		userCommandsDir := filepath.Join(homeDir, ".config", "devorch", "commands")
		userCmds := loadCommandsFromDir(userCommandsDir, UserCommandPrefix)
		commands = append(commands, userCmds...)
	}

	// Load project commands from .devorch/commands/
	cwd, err := os.Getwd()
	if err == nil {
		projectCommandsDir := filepath.Join(cwd, ".devorch", "commands")
		projectCmds := loadCommandsFromDir(projectCommandsDir, ProjectCommandPrefix)
		commands = append(commands, projectCmds...)
	}

	return commands
}

// loadCommandsFromDir loads commands from a directory
func loadCommandsFromDir(dir string, prefix string) []SlashCommand {
	var commands []SlashCommand

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return commands
	}

	// Walk through the directory and load all .md files
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process markdown files
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		// Read the file content
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Get the command ID from the file name without the .md extension
		commandID := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))

		// Get relative path from commands directory
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}

		// Create the command ID from the relative path
		// Replace directory separators with colons
		commandIDPath := strings.ReplaceAll(filepath.Dir(relPath), string(filepath.Separator), ":")
		if commandIDPath != "." {
			commandID = commandIDPath + ":" + commandID
		}

		// Create a command
		commandContent := string(content)
		cmd := SlashCommand{
			Name:        prefix + commandID,
			Description: fmt.Sprintf("Custom: %s", relPath),
			Category:    "Custom",
			Handler: func(m *Model) tea.Cmd {
				// Send the command content as a prompt to the AI
				m.messages = append(m.messages, Message{
					Role:    "user",
					Content: commandContent,
				})
				return m.sendToLLM(commandContent)
			},
		}

		commands = append(commands, cmd)
		return nil
	})

	return commands
}

// GetCommandsByCategory returns commands grouped by category.
func GetCommandsByCategory() []CommandCategory {
	commands := GetSlashCommands()
	categoryMap := make(map[string][]SlashCommand)
	categoryOrder := []string{"Session", "Edit", "Project", "Model", "Context", "Settings", "Auth", "Tools", "System", "Help", "Custom"}

	for _, cmd := range commands {
		categoryMap[cmd.Category] = append(categoryMap[cmd.Category], cmd)
	}

	var categories []CommandCategory
	for _, catName := range categoryOrder {
		if cmds, ok := categoryMap[catName]; ok {
			categories = append(categories, CommandCategory{
				Name:     catName,
				Commands: cmds,
			})
		}
	}

	return categories
}

// FindCommand finds a command by name.
func FindCommand(name string) *SlashCommand {
	name = strings.TrimPrefix(name, "/")
	for _, cmd := range GetSlashCommands() {
		if cmd.Name == name {
			return &cmd
		}
	}
	return nil
}

// FilterCommands filters commands by prefix.
func FilterCommands(prefix string) []SlashCommand {
	prefix = strings.TrimPrefix(prefix, "/")
	prefix = strings.ToLower(prefix)

	var matches []SlashCommand
	allCommands := GetSlashCommands()

	// If prefix is empty or just "/", return top commands (Quick + Main groups)
	if prefix == "" {
		for _, cmd := range allCommands {
			if cmd.Category == "Quick" || cmd.Category == "Main" {
				matches = append(matches, cmd)
			}
		}
		// Limit to first 20 for better UX
		if len(matches) > 20 {
			matches = matches[:20]
		}
		return matches
	}

	// Filter by prefix
	for _, cmd := range allCommands {
		if strings.HasPrefix(strings.ToLower(cmd.Name), prefix) {
			matches = append(matches, cmd)
		}
	}

	// Limit results
	if len(matches) > 15 {
		matches = matches[:15]
	}

	return matches
}

// CommandPaletteModel represents the command palette state.
type CommandPaletteModel struct {
	list     list.Model
	commands []SlashCommand
	filter   string
	selected int
	width    int
	height   int
	theme    Theme
}

// NewCommandPalette creates a new command palette.
func NewCommandPalette(theme Theme) CommandPaletteModel {
	commands := GetSlashCommands()
	items := make([]list.Item, len(commands))
	for i, cmd := range commands {
		items[i] = commandItem{
			name:        cmd.Name,
			description: cmd.Description,
			category:    cmd.Category,
		}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = theme.Selected
	delegate.Styles.SelectedDesc = theme.Subtle
	delegate.Styles.NormalTitle = theme.Normal
	delegate.Styles.NormalDesc = theme.Subtle

	l := list.New(items, delegate, 0, 0)
	l.Title = "Commands"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = theme.Title

	return CommandPaletteModel{
		list:     l,
		commands: commands,
		theme:    theme,
	}
}

// Init implements tea.Model.
func (m CommandPaletteModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m CommandPaletteModel) Update(msg tea.Msg) (CommandPaletteModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-6)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m CommandPaletteModel) View() string {
	return m.list.View()
}

// SelectedCommand returns the currently selected command.
func (m CommandPaletteModel) SelectedCommand() *SlashCommand {
	if item, ok := m.list.SelectedItem().(commandItem); ok {
		return FindCommand(item.name)
	}
	return nil
}

// Command handlers

func cmdNew(m *Model) tea.Cmd {
	m.messages = []Message{
		{Role: "system", Content: "Started a new session."},
	}
	m.sessionID = ""
	m.sessionName = ""
	return nil
}

func cmdSessions(m *Model) tea.Cmd {
	m.mode = ViewModeSessions
	return m.loadSessions()
}

func cmdClear(m *Model) tea.Cmd {
	m.messages = []Message{
		{Role: "system", Content: "Chat cleared."},
	}
	return nil
}

func cmdSave(m *Model) tea.Cmd {
	// Initialize session store
	if err := initSessionStore(); err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Failed to init session store: %v", err),
		})
		return nil
	}

	// Create or update session
	ctx := context.Background()

	if m.sessionID == "" {
		// Create new session
		cwd, _ := os.Getwd()
		name := fmt.Sprintf("Chat %s", time.Now().Format("2006-01-02 15:04"))
		sessID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
		sess, err := globalSessionStore.Create(ctx, sessID, name, cwd)
		if err != nil {
			m.messages = append(m.messages, Message{
				Role:    "system",
				Content: fmt.Sprintf("❌ Failed to create session: %v", err),
			})
			return nil
		}
		m.sessionID = sess.ID
		m.sessionName = sess.Name
	}

	// Get session and update messages
	sess, err := globalSessionStore.Get(ctx, m.sessionID)
	if err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Failed to get session: %v", err),
		})
		return nil
	}

	// Convert TUI messages to session messages
	sess.Messages = nil
	for _, msg := range m.messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Session saved") {
			continue // Skip save messages
		}
		sess.Messages = append(sess.Messages, session.Message{
			ID:        fmt.Sprintf("msg_%d", len(sess.Messages)),
			Role:      session.MessageRole(msg.Role),
			Content:   msg.Content,
			Tokens:    msg.Tokens,
			CreatedAt: time.Now(),
		})
	}

	// Save
	if err := globalSessionStore.Update(ctx, sess); err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Failed to save session: %v", err),
		})
		return nil
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("✓ Session saved: %s (%d messages)", m.sessionName, len(sess.Messages)),
	})
	return nil
}

func cmdExport(m *Model) tea.Cmd {
	// Export as markdown
	if err := initSessionStore(); err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Failed to init session store: %v", err),
		})
		return nil
	}

	// Generate markdown content
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# DevOrch Chat Export\n\n"))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("---\n\n")

	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			sb.WriteString(fmt.Sprintf("## 👤 User\n\n%s\n\n", msg.Content))
		case "assistant":
			sb.WriteString(fmt.Sprintf("## 🤖 Assistant\n\n%s\n\n", msg.Content))
		case "system":
			sb.WriteString(fmt.Sprintf("*%s*\n\n", msg.Content))
		}
	}

	// Save to file
	homeDir, _ := os.UserHomeDir()
	exportDir := filepath.Join(homeDir, ".config", "devorch", "exports")
	os.MkdirAll(exportDir, 0755)

	filename := fmt.Sprintf("chat_%s.md", time.Now().Format("20060102_150405"))
	exportPath := filepath.Join(exportDir, filename)

	if err := os.WriteFile(exportPath, []byte(sb.String()), 0644); err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Failed to export: %v", err),
		})
		return nil
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("✓ Exported to: %s", exportPath),
	})
	return nil
}

func cmdModel(m *Model) tea.Cmd {
	m.mode = ViewModeModelSelect
	// Get models for current provider
	active := GetActiveProvider()
	models := GetModelsForProvider(active.Provider)
	m.selectionList = make([]string, 0, len(models))
	for _, model := range models {
		status := ""
		if model.IsInstalled {
			status = " ✓"
		}
		m.selectionList = append(m.selectionList, model.ID+status)
	}
	m.selectionIdx = 0
	m.selectionTitle = fmt.Sprintf("Select Model (%s)", active.Provider)
	return nil
}

func cmdModels(m *Model) tea.Cmd {
	active := GetActiveProvider()
	models := GetModelsForProvider(active.Provider)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Available models for %s:\n", active.Provider))
	for _, model := range models {
		status := "○"
		if model.IsInstalled {
			status = "✓"
		}
		if model.IsPrimary {
			sb.WriteString(fmt.Sprintf("  %s %s (primary) - %s\n", status, model.ID, model.Description))
		} else {
			sb.WriteString(fmt.Sprintf("  %s %s - %s\n", status, model.ID, model.Description))
		}
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: sb.String(),
	})
	return nil
}

func cmdProvider(m *Model) tea.Cmd {
	m.mode = ViewModeProviderSelect
	m.providerFilter = ""
	m.showProviderSearch = false
	m.filterProviders() // Initialize the filtered list
	m.selectionIdx = 0
	m.selectionTitle = "Select Provider (/ to search)"
	return nil
}

func cmdProviders(m *Model) tea.Cmd {
	providers := GetAvailableProviders()

	var sb strings.Builder
	sb.WriteString("Available providers:\n\n")

	sb.WriteString("Local (no API key required):\n")
	for _, p := range providers {
		if p.Kind == "local" {
			status := "○ Not running"
			if p.IsAvailable {
				status = "✓ Running"
			}
			sb.WriteString(fmt.Sprintf("  • %s - %s\n", p.DisplayName, status))
		}
	}

	sb.WriteString("\nCloud (API key required):\n")
	for _, p := range providers {
		if p.Kind == "cloud" {
			status := "○ Not authenticated"
			if p.AuthStatus == "authenticated" {
				status = "✓ Authenticated"
			}
			sb.WriteString(fmt.Sprintf("  • %s - %s\n", p.DisplayName, status))
		}
	}

	sb.WriteString("\nCurrent: " + GetActiveProvider().Provider + "/" + GetActiveProvider().Model)

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: sb.String(),
	})
	return nil
}

func cmdSettings(m *Model) tea.Cmd {
	m.mode = ViewModeSettings
	return nil
}

func cmdTheme(m *Model) tea.Cmd {
	themes := AvailableThemes()
	m.selectionList = themes
	m.selectionTitle = "Select Theme"
	m.selectionIdx = 0
	m.mode = ViewModeThemeSelect
	return nil
}

func cmdThemes(m *Model) tea.Cmd {
	// Show theme selection UI instead of text list
	return cmdTheme(m)
}

func cmdLanguage(m *Model) tea.Cmd {
	m.selectionList = []string{
		"English (en)",
		"한국어 (ko)",
		"日本語 (ja)",
		"中文 (zh)",
		"Español (es)",
		"Français (fr)",
		"Deutsch (de)",
	}
	m.selectionIdx = 0
	m.selectionTitle = "Select Language"
	m.mode = ViewModeLanguageSelect
	return nil
}

// ========== 새로운 OpenCode 스타일 명령어 ==========

// cmdReset resets session completely
func cmdReset(m *Model) tea.Cmd {
	m.messages = []Message{
		{Role: "system", Content: "🔄 Session reset. All context cleared."},
	}
	m.sessionID = ""
	m.sessionName = ""
	return nil
}

// cmdShare generates a shareable session link
func cmdShare(m *Model) tea.Cmd {
	if len(m.messages) <= 1 {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "No conversation to share yet. Start chatting first!",
		})
		return nil
	}

	// Generate a simple hash-based ID
	shareID := fmt.Sprintf("devorch_%d", time.Now().UnixNano()%1000000)

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("🔗 Session share ID: %s\n\nTo share this session, export it with /export\nand share the markdown file.\n\n(Full link sharing requires devorch server)", shareID),
	})
	return nil
}

// cmdCompact summarizes and compacts the session
func cmdCompact(m *Model) tea.Cmd {
	if len(m.messages) < 5 {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "Session too short to compact. Continue chatting first.",
		})
		return nil
	}

	m.messages = append(m.messages, Message{
		Role:    "user",
		Content: "Please summarize our conversation so far in a concise way, keeping the key context and decisions we've made.",
	})

	return m.sendToLLM("Please summarize our conversation so far in a concise way, keeping the key context and decisions we've made.")
}

// cmdFiles shows project files in context
func cmdFiles(m *Model) tea.Cmd {
	cwd, _ := os.Getwd()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📁 Project files at: %s\n\n", cwd))

	// Walk the directory tree (limited depth)
	count := 0
	maxFiles := 50

	filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil || count >= maxFiles {
			return filepath.SkipDir
		}

		// Skip hidden and common ignore directories
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "__pycache__" {
				return filepath.SkipDir
			}
		}

		rel, _ := filepath.Rel(cwd, path)
		if rel == "." {
			return nil
		}

		depth := strings.Count(rel, string(os.PathSeparator))
		if depth > 3 {
			return filepath.SkipDir
		}

		indent := strings.Repeat("  ", depth)
		if info.IsDir() {
			sb.WriteString(fmt.Sprintf("%s📂 %s/\n", indent, info.Name()))
		} else {
			size := info.Size()
			sizeStr := fmt.Sprintf("%d B", size)
			if size > 1024*1024 {
				sizeStr = fmt.Sprintf("%.1f MB", float64(size)/1024/1024)
			} else if size > 1024 {
				sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
			}
			sb.WriteString(fmt.Sprintf("%s📄 %s (%s)\n", indent, info.Name(), sizeStr))
		}
		count++
		return nil
	})

	if count >= maxFiles {
		sb.WriteString(fmt.Sprintf("\n... truncated (showing %d files)\n", maxFiles))
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: sb.String(),
	})
	return nil
}

// cmdGrep searches code in project
func cmdGrep(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "🔍 Usage: Type your search pattern after the command.\n\nExample: /grep TODO\n\nOr simply ask me to search for something!",
	})
	return nil
}

// cmdLs lists directory structure
func cmdLs(m *Model) tea.Cmd {
	cwd, _ := os.Getwd()
	entries, err := os.ReadDir(cwd)
	if err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Failed to list directory: %v", err),
		})
		return nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📂 Contents of %s:\n\n", cwd))

	dirs := []string{}
	files := []string{}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue // Skip hidden files
		}
		if entry.IsDir() {
			dirs = append(dirs, entry.Name()+"/")
		} else {
			info, _ := entry.Info()
			size := ""
			if info != nil {
				if info.Size() > 1024*1024 {
					size = fmt.Sprintf(" (%.1fMB)", float64(info.Size())/1024/1024)
				} else if info.Size() > 1024 {
					size = fmt.Sprintf(" (%.1fKB)", float64(info.Size())/1024)
				}
			}
			files = append(files, entry.Name()+size)
		}
	}

	for _, d := range dirs {
		sb.WriteString(fmt.Sprintf("  📁 %s\n", d))
	}
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("  📄 %s\n", f))
	}

	sb.WriteString(fmt.Sprintf("\n%d directories, %d files", len(dirs), len(files)))

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: sb.String(),
	})
	return nil
}

// cmdDiff shows changed files
func cmdDiff(m *Model) tea.Cmd {
	// Check if we're in a git repo
	output, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "❌ Not a git repository or git not installed.",
		})
		return nil
	}

	if len(output) == 0 {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "✓ Working directory clean. No changes to show.",
		})
		return nil
	}

	var sb strings.Builder
	sb.WriteString("📝 Git changes:\n\n")

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		status := line[:2]
		file := strings.TrimSpace(line[2:])

		switch {
		case strings.Contains(status, "M"):
			sb.WriteString(fmt.Sprintf("  📝 Modified: %s\n", file))
		case strings.Contains(status, "A"):
			sb.WriteString(fmt.Sprintf("  ✚ Added: %s\n", file))
		case strings.Contains(status, "D"):
			sb.WriteString(fmt.Sprintf("  ✖ Deleted: %s\n", file))
		case strings.Contains(status, "?"):
			sb.WriteString(fmt.Sprintf("  ❓ Untracked: %s\n", file))
		default:
			sb.WriteString(fmt.Sprintf("  %s %s\n", status, file))
		}
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: sb.String(),
	})
	return nil
}

// cmdGitStatus shows git status
func cmdGitStatus(m *Model) tea.Cmd {
	// Get branch name
	branch, _ := exec.Command("git", "branch", "--show-current").Output()
	branchName := strings.TrimSpace(string(branch))
	if branchName == "" {
		branchName = "(detached)"
	}

	// Get status
	status, err := exec.Command("git", "status", "-sb").Output()
	if err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "❌ Not a git repository or git not installed.",
		})
		return nil
	}

	// Get recent commits
	commits, _ := exec.Command("git", "log", "--oneline", "-5").Output()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🌿 Branch: %s\n\n", branchName))
	sb.WriteString("Status:\n")
	sb.WriteString(string(status))

	if len(commits) > 0 {
		sb.WriteString("\nRecent commits:\n")
		sb.WriteString(string(commits))
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: sb.String(),
	})
	return nil
}

// cmdAgent switches agent mode
func cmdAgent(m *Model) tea.Cmd {
	m.mode = ViewModeAgentSelect
	m.selectionList = []string{
		"🤖 coder - Full coding agent (default)",
		"💬 chat - Simple chat mode",
		"🔍 researcher - Focus on research/analysis",
		"📝 writer - Focus on documentation",
		"🎯 task - Task-focused agent",
	}
	m.selectionIdx = 0
	m.selectionTitle = "Select Agent Mode"
	return nil
}

// cmdContext shows context window usage
func cmdContext(m *Model) tea.Cmd {
	// Count tokens (rough estimate: ~4 chars per token)
	totalChars := 0
	for _, msg := range m.messages {
		totalChars += len(msg.Content)
	}
	estimatedTokens := totalChars / 4

	// Typical context windows
	active := GetActiveProvider()
	maxTokens := 128000 // Default to Claude/GPT-4 level
	switch {
	case strings.Contains(active.Model, "gpt-4"):
		maxTokens = 128000
	case strings.Contains(active.Model, "claude"):
		maxTokens = 200000
	case strings.Contains(active.Model, "gemini"):
		maxTokens = 1000000
	case strings.Contains(active.Model, "llama"):
		maxTokens = 8192
	case strings.Contains(active.Model, "phi"):
		maxTokens = 4096
	}

	usedPercent := float64(estimatedTokens) / float64(maxTokens) * 100

	var sb strings.Builder
	sb.WriteString("📊 Context Window Usage\n\n")
	sb.WriteString(fmt.Sprintf("Model: %s/%s\n", active.Provider, active.Model))
	sb.WriteString(fmt.Sprintf("Estimated tokens: ~%d / %d (%.1f%%)\n", estimatedTokens, maxTokens, usedPercent))
	sb.WriteString(fmt.Sprintf("Messages: %d\n", len(m.messages)))

	// Visual bar
	barWidth := 30
	filledWidth := int(usedPercent / 100 * float64(barWidth))
	if filledWidth > barWidth {
		filledWidth = barWidth
	}
	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth)
	sb.WriteString(fmt.Sprintf("\n[%s] %.1f%%\n", bar, usedPercent))

	if usedPercent > 80 {
		sb.WriteString("\n⚠️ Context getting full! Consider using /compact")
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: sb.String(),
	})
	return nil
}

// cmdMemory shows persistent memory
func cmdMemory(m *Model) tea.Cmd {
	homeDir, _ := os.UserHomeDir()
	memoryFile := filepath.Join(homeDir, ".config", "devorch", "memory.md")

	content, err := os.ReadFile(memoryFile)
	if err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "📝 No persistent memory file found.\n\nCreate ~/.config/devorch/memory.md to store persistent context.",
		})
		return nil
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("📝 Persistent Memory:\n\n%s", string(content)),
	})
	return nil
}

// cmdAdd adds file to context
func cmdAdd(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "📎 Usage: /add <filepath>\n\nExample: /add main.go\n\nOr simply paste file contents or ask me to read a file!",
	})
	return nil
}

// cmdConfig opens configuration
func cmdConfig(m *Model) tea.Cmd {
	homeDir, _ := os.UserHomeDir()
	configFile := filepath.Join(homeDir, ".config", "devorch", "config.yaml")

	// Check if config exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Create default config
		defaultConfig := `# DevOrch Configuration
# See: https://github.com/devorch/devorch

# Default provider and model
provider: ollama
model: llama3.2:latest

# API keys (or use environment variables)
# anthropic_api_key: ""
# openai_api_key: ""
# google_api_key: ""

# Theme
theme: dracula

# Auto features
auto_compact: true
auto_save: true

# MCP servers
mcp:
  enabled: true
  servers: []
`
		os.MkdirAll(filepath.Dir(configFile), 0755)
		os.WriteFile(configFile, []byte(defaultConfig), 0644)
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("⚙️ Configuration file: %s\n\nEdit this file to customize DevOrch.\nChanges take effect on restart.", configFile),
	})
	return nil
}

// cmdLSP shows LSP status
func cmdLSP(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role: "system",
		Content: `🔧 Language Server Protocol Status

Available LSP features:
• Go: gopls (auto-detected)
• Python: pylsp/pyright (if installed)
• TypeScript: typescript-language-server
• Rust: rust-analyzer

LSP provides:
• Code completion
• Go to definition
• Find references
• Diagnostics

Note: LSP integration is automatic when available.`,
	})
	return nil
}

// cmdBench runs performance benchmark
func cmdBench(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "⏱️ Running quick benchmark...",
	})

	return func() tea.Msg {
		active := GetActiveProvider()
		start := time.Now()

		// Simple benchmark: send a short message and measure response time
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		testMsgs := []Message{{Role: "user", Content: "Say 'Hello' in exactly one word."}}
		_, err := ChatWithProvider(ctx, active.Provider, active.Model, testMsgs)

		elapsed := time.Since(start)

		if err != nil {
			return StreamChunkMsg{
				Content: fmt.Sprintf("❌ Benchmark failed: %v", err),
			}
		}

		var result string
		switch {
		case elapsed < 500*time.Millisecond:
			result = "🚀 Excellent"
		case elapsed < 1*time.Second:
			result = "✓ Good"
		case elapsed < 3*time.Second:
			result = "⚠️ Moderate"
		default:
			result = "🐢 Slow"
		}

		return StreamChunkMsg{
			Content: fmt.Sprintf("⏱️ Benchmark Results\n\nProvider: %s\nModel: %s\nResponse time: %v\nRating: %s",
				active.Provider, active.Model, elapsed.Round(time.Millisecond), result),
		}
	}
}

func cmdLogin(m *Model) tea.Cmd {
	m.mode = ViewModeLogin
	m.selectionList = []string{
		"1. Anthropic (Claude) - Get API key",
		"2. OpenAI (GPT) - Get API key",
		"3. Google (Gemini) - Get API key",
		"4. OpenRouter - Get API key",
		"5. Groq - Get API key",
		"6. GitHub - Get token",
	}
	m.selectionIdx = 0
	m.selectionTitle = "Select provider to login"
	return nil
}

func cmdLogout(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role: "system",
		Content: `To logout, remove or unset your API keys:
  unset ANTHROPIC_API_KEY
  unset OPENAI_API_KEY
  unset GOOGLE_API_KEY
  unset OPENROUTER_API_KEY
  unset GROQ_API_KEY

Or remove them from ~/.config/devorch/config.yaml`,
	})
	return nil
}

func cmdAuth(m *Model) tea.Cmd {
	var sb strings.Builder
	sb.WriteString("Authentication status:\n\n")

	providers := GetAvailableProviders()
	for _, p := range providers {
		if p.Kind == "local" {
			sb.WriteString(fmt.Sprintf("• %s: %s (no auth required)\n", p.DisplayName, "✓"))
		} else {
			status := "✗ Not authenticated"
			if p.AuthStatus == "authenticated" {
				status = "✓ Authenticated"
			}
			sb.WriteString(fmt.Sprintf("• %s: %s\n", p.DisplayName, status))
		}
	}

	sb.WriteString("\nUse /login to authenticate with a provider.")

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: sb.String(),
	})
	return nil
}

func cmdTools(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role: "system",
		Content: `Available tools (25):
• File: read, write, edit, multiedit, ls, glob
• Search: grep, codesearch, websearch
• Execute: bash, batch
• Web: webfetch
• Code: lsp, diagnostics, symbols
• Plan: todo, task
• Other: question, patch, memory`,
	})
	return nil
}

func cmdMCP(m *Model) tea.Cmd {
	m.mode = ViewModeMCP
	return nil
}

func cmdStatus(m *Model) tea.Cmd {
	// 실제 시스템 상태 확인
	ollamaStatus := "✗ Not running"
	if checkOllamaRunning() {
		ollamaStatus = "✓ Running"
	}

	// 설치된 모델 수
	models := getOllamaModels()
	installedCount := 0
	for _, model := range models {
		if model.IsInstalled {
			installedCount++
		}
	}

	// 현재 프로바이더/모델
	active := GetActiveProvider()

	// 인증 상태
	authStatus := ""
	providers := GetAvailableProviders()
	for _, p := range providers {
		if p.Kind == "cloud" && p.AuthStatus == "authenticated" {
			authStatus += fmt.Sprintf("  ✓ %s\n", p.DisplayName)
		}
	}
	if authStatus == "" {
		authStatus = "  No cloud providers authenticated\n"
	}

	content := fmt.Sprintf(`System Status:
• Ollama: %s
• Models installed: %d
• Current: %s/%s

Authenticated providers:
%s
Hardware:
• OS: %s/%s
• Memory: checking...`, ollamaStatus, installedCount, active.Provider, active.Model, authStatus, runtime.GOOS, runtime.GOARCH)

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: content,
	})
	return nil
}

// checkOllamaRunning checks if Ollama server is running
func checkOllamaRunning() bool {
	client := NewOllamaClient()
	models := getOllamaModels()
	return len(models) > 0 || client != nil
}

func cmdSetup(m *Model) tea.Cmd {
	m.mode = ViewModeSetup
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "🚀 Auto Setup Wizard\n\nAnalyzing your system...\n\nThis will:\n1. Check Ollama installation\n2. Detect system resources\n3. Recommend optimal models\n4. Install lightweight models\n\nPress Enter to continue, or Esc to cancel.",
	})
	return nil
}

func cmdDoctor(m *Model) tea.Cmd {
	// 실제 진단 수행
	var results []string

	// 1. Ollama 확인
	if checkOllamaRunning() {
		results = append(results, "✓ Ollama: Running")
	} else {
		results = append(results, "✗ Ollama: Not running - run 'ollama serve'")
	}

	// 2. 설치된 모델 확인
	models := getOllamaModels()
	installedCount := 0
	for _, model := range models {
		if model.IsInstalled {
			installedCount++
		}
	}
	if installedCount > 0 {
		results = append(results, fmt.Sprintf("✓ Models: %d installed", installedCount))
	} else {
		results = append(results, "✗ Models: No models installed - run /install")
	}

	// 3. API 키 확인
	apiKeys := []struct {
		name   string
		envVar string
	}{
		{"Anthropic", "ANTHROPIC_API_KEY"},
		{"OpenAI", "OPENAI_API_KEY"},
		{"Google", "GOOGLE_API_KEY"},
		{"OpenRouter", "OPENROUTER_API_KEY"},
		{"Groq", "GROQ_API_KEY"},
	}

	authCount := 0
	for _, key := range apiKeys {
		if os.Getenv(key.envVar) != "" {
			authCount++
		}
	}
	if authCount > 0 {
		results = append(results, fmt.Sprintf("✓ API Keys: %d configured", authCount))
	} else {
		results = append(results, "⚠ API Keys: None configured (optional for cloud providers)")
	}

	// 4. 설정 디렉토리 확인
	configDir := os.ExpandEnv("$HOME/.config/devorch")
	if _, err := os.Stat(configDir); err == nil {
		results = append(results, "✓ Config directory exists")
	} else {
		results = append(results, "⚠ Config directory: Not created yet")
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "Running diagnostics...\n\n" + strings.Join(results, "\n"),
	})
	return nil
}

func cmdVersion(m *Model) tea.Cmd {
	// 실제 버전 정보 (빌드 시 설정 가능)
	version := os.Getenv("DEVORCH_VERSION")
	if version == "" {
		version = "dev"
	}
	buildDate := os.Getenv("DEVORCH_BUILD_DATE")
	if buildDate == "" {
		buildDate = "unknown"
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("DevOrch %s\nBuild: %s\nGo: %s\nOS: %s/%s", version, buildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH),
	})
	return nil
}

func cmdHelp(m *Model) tea.Cmd {
	m.mode = ViewModeHelp
	return nil
}

func cmdPermissions(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "Permission management is only available in CLI mode. Use 'devorch' command and then '/perm' for tool permissions.",
	})
	return nil
}

func cmdQuit(m *Model) tea.Cmd {
	m.quitting = true
	return tea.Quit
}

// ========== 새로운 명령어 핸들러 ==========

// cmdUndo undoes the last AI edit
func cmdUndo(m *Model) tea.Cmd {
	// 히스토리에서 마지막 assistant 메시지 제거
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" {
			// 이전 상태 저장 (redo를 위해)
			m.messages = append(m.messages, Message{
				Role:    "system",
				Content: "⟲ Undone last response. Use /redo to restore.",
			})
			m.messages = append(m.messages[:i], m.messages[i+1:]...)
			return nil
		}
	}
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "Nothing to undo.",
	})
	return nil
}

// cmdRedo redoes the last undone edit
func cmdRedo(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "⟳ Redo functionality requires session persistence.\nUse /save to enable undo/redo history.",
	})
	return nil
}

// cmdInit analyzes the project and creates/updates AGENTS.md
func cmdInit(m *Model) tea.Cmd {
	// 현재 디렉토리 분석 요청을 LLM에 보내기
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("🔍 Analyzing project at: %s\n\nSending analysis request to AI...", cwd),
	})

	// LLM에 프로젝트 분석 요청 (실제 동작)
	prompt := fmt.Sprintf(`Please analyze this codebase and create an AGENTS.md file containing:
1. Build/lint/test commands
2. Code style guidelines
3. Project structure overview
4. Key dependencies

Current directory: %s

List the main files and directories, then provide the AGENTS.md content.`, cwd)

	// 사용자 메시지로 추가하여 LLM이 처리하도록
	m.messages = append(m.messages, Message{
		Role:    "user",
		Content: prompt,
	})

	return m.sendToLLM(prompt)
}

// InstallableModel represents a model available for installation
type InstallableModel struct {
	ID          string
	Name        string
	Size        string
	Description string
	Recommended bool
	Installed   bool
	Category    string // "Recommended", "Coding", "General", "Small", "Large"
}

// cmdInstall shows interactive model installation UI
func cmdInstall(m *Model) tea.Cmd {
	// 시스템 사양 감지
	specs := detectSystemSpecs()
	m.installSystemSpecs = specs

	// 이미 설치된 모델 확인
	installed := getOllamaModels()
	installedSet := make(map[string]bool)
	for _, model := range installed {
		if model.IsInstalled {
			installedSet[model.ID] = true
		}
	}

	// 모든 설치 가능한 모델 생성
	m.installableModels = buildInstallableModels(specs, installedSet)
	m.selectedModelIdxs = make(map[int]bool)

	// 추천 모델은 자동 선택
	for i, model := range m.installableModels {
		if model.Recommended && !model.Installed {
			m.selectedModelIdxs[i] = true
		}
	}

	// 선택 UI로 전환
	m.mode = ViewModeInstallSelect
	m.selectionIdx = 0
	m.selectionTitle = "Select Models to Install"

	return nil
}

// buildInstallableModels creates the list of all installable models
func buildInstallableModels(specs SystemSpecs, installedSet map[string]bool) []InstallableModel {
	var models []InstallableModel

	// 추천 모델
	recommended := recommendModelsDetailed(specs)
	for _, model := range recommended {
		models = append(models, InstallableModel{
			ID:          model.id,
			Name:        model.id,
			Size:        model.size,
			Description: model.desc,
			Recommended: true,
			Installed:   installedSet[model.id],
			Category:    "⭐ Recommended",
		})
	}

	// 코딩 특화 모델
	codingModels := []struct {
		id   string
		size string
		desc string
	}{
		{"qwen2.5-coder:3b", "2GB", "Qwen2.5 Coder 3B"},
		{"codellama:7b", "3.8GB", "CodeLlama 7B"},
		{"codellama:13b", "7.3GB", "CodeLlama 13B"},
		{"deepseek-coder:6.7b", "3.8GB", "DeepSeek Coder"},
	}
	for _, model := range codingModels {
		models = append(models, InstallableModel{
			ID:          model.id,
			Name:        model.id,
			Size:        model.size,
			Description: model.desc,
			Recommended: false,
			Installed:   installedSet[model.id],
			Category:    "💻 Coding",
		})
	}

	// 범용 모델
	generalModels := []struct {
		id   string
		size string
		desc string
	}{
		{"llama3.2:1b", "1.3GB", "Meta Llama 3.2 1B"},
		{"llama3.2:3b", "2GB", "Meta Llama 3.2 3B"},
		{"llama3.1:8b", "4.7GB", "Meta Llama 3.1 8B"},
		{"mistral:7b", "4.1GB", "Mistral 7B"},
		{"gemma2:2b", "1.6GB", "Google Gemma2 2B"},
		{"qwen2.5:3b", "2GB", "Qwen2.5 3B"},
		{"qwen2.5:7b", "4.7GB", "Qwen2.5 7B"},
	}
	for _, model := range generalModels {
		models = append(models, InstallableModel{
			ID:          model.id,
			Name:        model.id,
			Size:        model.size,
			Description: model.desc,
			Recommended: false,
			Installed:   installedSet[model.id],
			Category:    "🌐 General",
		})
	}

	// 소형 모델
	smallModels := []struct {
		id   string
		size string
		desc string
	}{
		{"phi3:mini", "1.3GB", "Microsoft Phi-3 Mini"},
		{"deepseek-r1:1.5b", "1GB", "DeepSeek R1 1.5B"},
		{"tinyllama:1.1b", "637MB", "TinyLlama 1.1B"},
	}
	for _, model := range smallModels {
		models = append(models, InstallableModel{
			ID:          model.id,
			Name:        model.id,
			Size:        model.size,
			Description: model.desc,
			Recommended: false,
			Installed:   installedSet[model.id],
			Category:    "🔰 Small",
		})
	}

	// 대형 모델 (고성능 시스템용)
	if specs.RAM >= 16 {
		largeModels := []struct {
			id   string
			size string
			desc string
		}{
			{"mixtral:8x7b", "26GB", "Mixtral 8x7B MoE"},
			{"qwen2.5:14b", "9GB", "Qwen2.5 14B"},
			{"llama3.1:70b", "40GB", "Meta Llama 3.1 70B"},
		}
		for _, model := range largeModels {
			models = append(models, InstallableModel{
				ID:          model.id,
				Name:        model.id,
				Size:        model.size,
				Description: model.desc,
				Recommended: false,
				Installed:   installedSet[model.id],
				Category:    "🚀 Large",
			})
		}
	}

	return models
}

// recommendModelsDetailed returns detailed recommended models
func recommendModelsDetailed(specs SystemSpecs) []struct {
	id   string
	size string
	desc string
} {
	if specs.RAM < 8 {
		return []struct {
			id   string
			size string
			desc string
		}{
			{"phi3:mini", "1.3GB", "Best for low-end (Microsoft)"},
			{"qwen2.5:3b", "2GB", "Fast general purpose"},
			{"llama3.2:1b", "1.3GB", "Tiny but capable (Meta)"},
		}
	} else if specs.RAM < 16 {
		return []struct {
			id   string
			size string
			desc string
		}{
			{"qwen2.5:3b", "2GB", "Fast general purpose"},
			{"qwen2.5-coder:3b", "2GB", "Specialized for coding"},
			{"mistral:7b", "4.1GB", "Balanced performance"},
		}
	} else if specs.RAM < 32 {
		return []struct {
			id   string
			size string
			desc string
		}{
			{"mistral:7b", "4.1GB", "Excellent balance"},
			{"llama3.2:3b", "2GB", "Latest Meta model"},
			{"codellama:7b", "3.8GB", "Best for coding"},
		}
	} else {
		return []struct {
			id   string
			size string
			desc string
		}{
			{"llama3.1:8b", "4.7GB", "Latest Llama"},
			{"mixtral:8x7b", "26GB", "Mixture of Experts"},
			{"qwen2.5:14b", "9GB", "Top performance"},
		}
	}
}

// cmdInstallOld is the old implementation (kept for backup)
func cmdInstallOld(m *Model) tea.Cmd {
	// 시스템 사양 감지
	specs := detectSystemSpecs()

	// 사양에 맞는 모델 추천
	recommended := recommendModels(specs)

	// 이미 설치된 모델 확인
	installed := getOllamaModels()
	installedSet := make(map[string]bool)
	for _, model := range installed {
		if model.IsInstalled {
			installedSet[model.ID] = true
		}
	}

	// 메시지 생성
	var msg strings.Builder
	msg.WriteString("📦 Model Installation Assistant\n\n")

	// 시스템 사양 표시
	msg.WriteString("🖥️  System Specs:\n")
	msg.WriteString(fmt.Sprintf("  • RAM: %.1f GB\n", specs.RAM))
	msg.WriteString(fmt.Sprintf("  • CPU: %d cores\n", specs.CPUCores))
	msg.WriteString(fmt.Sprintf("  • Recommendation: %s\n\n", specs.Tier))

	// 이미 설치된 모델
	if len(installedSet) > 0 {
		msg.WriteString("✅ Already Installed:\n")
		for id := range installedSet {
			msg.WriteString(fmt.Sprintf("  ✓ %s\n", id))
		}
		msg.WriteString("\n")
	}

	// 추천 모델 목록
	msg.WriteString("🎯 Recommended Models for Your System:\n\n")

	var toInstall []string
	for _, model := range recommended {
		status := "📥"
		if installedSet[model.id] {
			status = "✅"
		} else {
			toInstall = append(toInstall, model.id)
		}
		msg.WriteString(fmt.Sprintf("  %s %s\n", status, model.desc))
	}

	// 추가 모델 옵션
	msg.WriteString("\n📋 Other Popular Models:\n")
	otherModels := []struct {
		id   string
		size string
		desc string
	}{
		{"llama3.2:1b", "1.3GB", "Meta's smallest Llama"},
		{"gemma2:2b", "1.6GB", "Google's efficient model"},
		{"deepseek-r1:1.5b", "1GB", "Reasoning model"},
		{"mistral:7b", "4.1GB", "Balanced performance"},
		{"llama3.2:3b", "2GB", "Meta Llama 3.2"},
		{"codellama:7b", "3.8GB", "Specialized coding"},
	}

	for _, model := range otherModels {
		status := "📥"
		if installedSet[model.id] {
			status = "✅"
		}
		msg.WriteString(fmt.Sprintf("  %s %s (%s) - %s\n", status, model.id, model.size, model.desc))
	}

	msg.WriteString("\n💡 Commands:\n")
	msg.WriteString("  • Install recommended: Already installing below...\n")
	msg.WriteString("  • Install specific: ollama pull <model-name>\n")
	msg.WriteString("  • List all models: ollama list\n")
	msg.WriteString("  • Check progress: ollama ps\n")

	// 백그라운드 설치 시작
	if len(toInstall) > 0 {
		msg.WriteString(fmt.Sprintf("\n⬇️  Installing %d models in background...\n", len(toInstall)))
		for _, id := range toInstall {
			msg.WriteString(fmt.Sprintf("  • %s\n", id))
			go func(modelID string) {
				exec.Command("ollama", "pull", modelID).Run()
			}(id)
		}
		msg.WriteString("\n⏱️  This may take 5-10 minutes depending on your connection.")
	} else {
		msg.WriteString("\n✅ All recommended models are already installed!")
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: msg.String(),
	})

	return nil
}

// SystemSpecs holds system specifications
type SystemSpecs struct {
	RAM      float64 // GB
	CPUCores int
	Tier     string // "Low", "Medium", "High"
}

// detectSystemSpecs detects current system specifications
func detectSystemSpecs() SystemSpecs {
	specs := SystemSpecs{
		RAM:      8.0, // Default
		CPUCores: 4,   // Default
		Tier:     "Medium",
	}

	// Detect RAM
	if runtime.GOOS == "darwin" {
		// macOS
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err == nil {
			var bytes int64
			fmt.Sscanf(string(out), "%d", &bytes)
			specs.RAM = float64(bytes) / (1024 * 1024 * 1024)
		}
	} else if runtime.GOOS == "linux" {
		// Linux
		out, err := exec.Command("grep", "MemTotal", "/proc/meminfo").Output()
		if err == nil {
			var kb int64
			fmt.Sscanf(string(out), "MemTotal: %d kB", &kb)
			specs.RAM = float64(kb) / (1024 * 1024)
		}
	}

	// Detect CPU cores
	specs.CPUCores = runtime.NumCPU()

	// Determine tier
	if specs.RAM < 8 {
		specs.Tier = "Low (Recommend 1-3B models)"
	} else if specs.RAM < 16 {
		specs.Tier = "Medium (Can run up to 7B models)"
	} else if specs.RAM < 32 {
		specs.Tier = "High (Can run up to 13B models)"
	} else {
		specs.Tier = "Very High (Can run 30B+ models)"
	}

	return specs
}

// recommendModels recommends models based on system specs
func recommendModels(specs SystemSpecs) []struct {
	id   string
	desc string
} {
	if specs.RAM < 8 {
		// Low-end systems: 1-3B models
		return []struct {
			id   string
			desc string
		}{
			{"phi3:mini", "1.3GB - 🥈 Best for low-end (Microsoft)"},
			{"qwen2.5:3b", "2GB - Fast general purpose (Alibaba)"},
			{"llama3.2:1b", "1.3GB - Tiny but capable (Meta)"},
		}
	} else if specs.RAM < 16 {
		// Mid-range systems: 3-7B models
		return []struct {
			id   string
			desc string
		}{
			{"qwen2.5:3b", "2GB - Fast general purpose"},
			{"qwen2.5-coder:3b", "2GB - Specialized for coding"},
			{"mistral:7b", "4.1GB - Balanced performance"},
			{"phi3:mini", "1.3GB - Efficient fallback"},
		}
	} else if specs.RAM < 32 {
		// High-end systems: 7-13B models
		return []struct {
			id   string
			desc string
		}{
			{"mistral:7b", "4.1GB - Excellent balance"},
			{"llama3.2:3b", "2GB - Latest Meta model"},
			{"codellama:7b", "3.8GB - Best for coding"},
			{"qwen2.5:7b", "4.7GB - Advanced reasoning"},
		}
	} else {
		// Very high-end: 13B+ models
		return []struct {
			id   string
			desc string
		}{
			{"llama3.1:8b", "4.7GB - Latest Llama"},
			{"mixtral:8x7b", "26GB - Mixture of Experts"},
			{"codellama:13b", "7.3GB - Advanced coding"},
			{"qwen2.5:14b", "9GB - Top performance"},
		}
	}
}

// ========== 추가 OpenCode 스타일 명령어 핸들러 ==========

// cmdUnshare removes shared session link
func cmdUnshare(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "🗑️ Session share removed.\n\nNote: Full unshare functionality requires DevOrch server.",
	})
	return nil
}

// cmdDetails toggles showing message details (tokens, timing)
func cmdDetails(m *Model) tea.Cmd {
	m.showDetails = !m.showDetails
	status := "hidden"
	if m.showDetails {
		status = "visible"
	}
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("📊 Message details now %s.\n\nShowing: token count, response time, model info", status),
	})
	return nil
}

// cmdEditor opens external editor for composing input
func cmdEditor(m *Model) tea.Cmd {
	return func() tea.Msg {
		// Get editor from EDITOR or VISUAL env, or default to vim
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = os.Getenv("VISUAL")
		}
		if editor == "" {
			switch runtime.GOOS {
			case "darwin":
				editor = "nano"
			case "windows":
				editor = "notepad"
			default:
				editor = "vim"
			}
		}

		// Create temp file for editing
		tmpFile, err := os.CreateTemp("", "devorch-*.md")
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create temp file: %w", err)}
		}
		tmpPath := tmpFile.Name()
		tmpFile.Close()

		// Run editor
		cmd := exec.Command(editor, tmpPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		// Read content
		content, err := os.ReadFile(tmpPath)
		os.Remove(tmpPath)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read edited content: %w", err)}
		}

		text := strings.TrimSpace(string(content))
		if text == "" {
			return StreamChunkMsg{Content: "✏️ Editor closed without input."}
		}

		return StreamChunkMsg{Content: fmt.Sprintf("✏️ From editor:\n%s", text)}
	}
}

// cmdThinking toggles extended thinking mode for supporting models
func cmdThinking(m *Model) tea.Cmd {
	m.thinkingMode = !m.thinkingMode
	status := "disabled"
	if m.thinkingMode {
		status = "enabled"
	}
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("🧠 Extended thinking %s.\n\nWhen enabled, models that support it (Claude, o1) will show their reasoning process.\nThis may increase response time and token usage.", status),
	})
	return nil
}

// cmdConnect opens the OpenCode-style provider connect UI
func cmdConnect(m *Model) tea.Cmd {
	// Initialize connect providers
	m.connectProviders = GetConnectProviders()
	m.connectFilteredList = m.connectProviders
	m.connectSelectedIdx = 0
	m.connectFilter = ""
	m.connectInputMode = false
	m.mode = ViewModeConnect
	return nil
}

// RenderCommandPreview renders a preview of available commands.
func RenderCommandPreview(filter string, theme Theme, width int) string {
	commands := FilterCommands(filter)
	if len(commands) == 0 {
		return theme.Subtle.Render("No matching commands")
	}

	var sb strings.Builder
	maxShow := 8
	if len(commands) < maxShow {
		maxShow = len(commands)
	}

	// Group by category
	categories := make(map[string][]SlashCommand)
	for _, cmd := range commands[:maxShow] {
		categories[cmd.Category] = append(categories[cmd.Category], cmd)
	}

	for cat, cmds := range categories {
		sb.WriteString(theme.Accent.Render(cat) + "\n")
		for _, cmd := range cmds {
			line := fmt.Sprintf("  /%s - %s", cmd.Name, cmd.Description)
			if len(line) > width-4 {
				line = line[:width-7] + "..."
			}
			sb.WriteString(theme.Normal.Render(line) + "\n")
		}
	}

	if len(commands) > maxShow {
		sb.WriteString(theme.Subtle.Render(fmt.Sprintf("  ... and %d more", len(commands)-maxShow)))
	}

	return sb.String()
}

// QuickCommandList renders a compact command list for the footer area.
func QuickCommandList(theme Theme) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 1)

	return style.Render("Type / for commands • Ctrl+N: New • Ctrl+S: Sessions • Ctrl+H: Help")
}

// ===================== Additional Command Handlers =====================

// cmdCopy copies last response to clipboard
func cmdCopy(m *Model) tea.Cmd {
	if len(m.messages) == 0 {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "📋 No messages to copy",
		})
		return nil
	}

	// Find last assistant message
	var lastContent string
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "assistant" {
			lastContent = m.messages[i].Content
			break
		}
	}

	if lastContent == "" {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "📋 No response to copy",
		})
		return nil
	}

	// Use pbcopy on macOS, xclip on Linux
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("pbcopy")
	} else {
		cmd = exec.Command("xclip", "-selection", "clipboard")
	}
	cmd.Stdin = strings.NewReader(lastContent)
	if err := cmd.Run(); err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Failed to copy: %v", err),
		})
		return nil
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("📋 Copied %d characters to clipboard", len(lastContent)),
	})
	return nil
}

// cmdPaste pastes from clipboard as user message
func cmdPaste(m *Model) tea.Cmd {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("pbpaste")
	} else {
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	}

	output, err := cmd.Output()
	if err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Failed to paste: %v", err),
		})
		return nil
	}

	content := strings.TrimSpace(string(output))
	if content == "" {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "📋 Clipboard is empty",
		})
		return nil
	}

	// Add to input
	m.input.SetValue(content)
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("📋 Pasted %d characters. Press Enter to send.", len(content)),
	})
	return nil
}

// cmdRetry retries the last user message
func cmdRetry(m *Model) tea.Cmd {
	// Find last user message
	var lastUserMsg string
	var lastUserIdx int = -1
	for i := len(m.messages) - 1; i >= 0; i-- {
		if m.messages[i].Role == "user" {
			lastUserMsg = m.messages[i].Content
			lastUserIdx = i
			break
		}
	}

	if lastUserIdx == -1 {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "⚠️ No previous message to retry",
		})
		return nil
	}

	// Remove messages from lastUserIdx onwards
	m.messages = m.messages[:lastUserIdx]

	// Add the user message back and mark for retry
	m.messages = append(m.messages, Message{
		Role:    "user",
		Content: lastUserMsg,
	})

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "🔄 Retrying: " + lastUserMsg[:min(50, len(lastUserMsg))] + "...",
	})

	// User needs to press Enter to re-send
	m.input.SetValue(lastUserMsg)
	return nil
}

// cmdGit executes git commands
func cmdGit(m *Model) tea.Cmd {
	// Show git status by default
	cmd := exec.Command("git", "status", "--short")
	output, err := cmd.Output()
	if err != nil {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("❌ Git error: %v", err),
		})
		return nil
	}

	status := strings.TrimSpace(string(output))
	if status == "" {
		status = "Working tree clean"
	}

	// Get branch
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchOutput, _ := branchCmd.Output()
	branch := strings.TrimSpace(string(branchOutput))

	result := fmt.Sprintf("🌿 Branch: %s\n\n📄 Status:\n%s", branch, status)
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: result,
	})
	return nil
}

// cmdReview reviews code changes with AI
func cmdReview(m *Model) tea.Cmd {
	// Get staged diff first
	cmd := exec.Command("git", "diff", "--staged")
	output, err := cmd.Output()
	diffContent := string(output)

	// If no staged changes, get unstaged diff
	if err != nil || strings.TrimSpace(diffContent) == "" {
		cmd = exec.Command("git", "diff")
		output, err = cmd.Output()
		diffContent = string(output)
	}

	if err != nil || strings.TrimSpace(diffContent) == "" {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "⚠️ No changes to review. Stage or modify files first.",
		})
		return nil
	}

	// Truncate if too long
	if len(diffContent) > 8000 {
		diffContent = diffContent[:8000] + "\n... (truncated)"
	}

	// Add context for AI review - user needs to press Enter
	reviewPrompt := fmt.Sprintf("Please review these code changes:\n\n```diff\n%s\n```\n\nFocus on: code quality, bugs, performance, security", diffContent)
	m.input.SetValue(reviewPrompt)

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "📝 Code diff loaded. Press Enter to send for AI review.",
	})

	return nil
}

// cmdAgents shows available agents
func cmdAgents(m *Model) tea.Cmd {
	agents := []struct {
		name string
		desc string
	}{
		{"coder", "Expert programmer for writing code"},
		{"reviewer", "Code reviewer for quality checks"},
		{"debugger", "Debugging specialist"},
		{"architect", "System design expert"},
		{"tester", "QA and testing expert"},
	}

	var sb strings.Builder
	sb.WriteString("🤖 Available Agents\n\n")
	for i, a := range agents {
		marker := "  "
		if i == 0 { // Default to coder
			marker = "▸ "
		}
		sb.WriteString(fmt.Sprintf("%s%s - %s\n", marker, a.name, a.desc))
	}
	sb.WriteString("\nUse /agent <name> to switch")

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: sb.String(),
	})
	return nil
}

// cmdThemeSwitch switches to a specific theme
func cmdThemeSwitch(m *Model) tea.Cmd {
	// This is handled by cmdTheme with argument
	m.mode = ViewModeThemeSelect
	return nil
}

// ========== Command Group Handlers (16 core groups) ==========

// cmdSessionGroup handles /session command group
func cmdSessionGroup(m *Model) tea.Cmd {
	// Switch to subcommand selection mode
	m.selectedMainCommand = "session"
	m.subcommandIdx = 0
	m.subcommandList = []SubcommandItem{
		{Name: "new", Description: "Start a new session", Handler: cmdNew},
		{Name: "list", Description: "Show saved sessions", Handler: cmdSessions},
		{Name: "save", Description: "Save current session", Handler: cmdSave},
		{Name: "export", Description: "Export (json/md)", Handler: cmdExport},
		{Name: "share", Description: "Share session", Handler: cmdShare},
		{Name: "unshare", Description: "Stop sharing", Handler: cmdUnshare},
		{Name: "compact", Description: "Compact history", Handler: cmdCompact},
		{Name: "details", Description: "Show session info", Handler: cmdDetails},
		{Name: "reset", Description: "Reset completely", Handler: cmdReset},
		{Name: "undo", Description: "Undo last action", Handler: cmdUndo},
		{Name: "redo", Description: "Redo last action", Handler: cmdRedo},
		{Name: "history", Description: "Show chat history", Handler: cmdHistory},
		{Name: "clear", Description: "Clear chat", Handler: cmdClear},
	}
	m.mode = ViewModeSubCommands
	return nil
}

// cmdModelGroup handles /model command group
func cmdModelGroup(m *Model) tea.Cmd {
	// Switch to subcommand selection mode
	m.selectedMainCommand = "model"
	m.subcommandIdx = 0
	m.subcommandList = []SubcommandItem{
		{Name: "list", Description: "List available models", Handler: cmdModels},
		{Name: "set", Description: "Switch to model", Handler: cmdModel},
		{Name: "install", Description: "Install a model", Handler: cmdInstall},
		{Name: "bench", Description: "Run benchmark", Handler: cmdBench},
	}
	m.mode = ViewModeSubCommands
	return nil
}

// cmdProviderGroup handles /provider command group
func cmdProviderGroup(m *Model) tea.Cmd {
	m.selectedMainCommand = "provider"
	m.subcommandIdx = 0
	m.subcommandList = []SubcommandItem{
		{Name: "list", Description: "List available providers", Handler: cmdProviders},
		{Name: "connect", Description: "Connect new provider", Handler: cmdConnect},
	}
	m.mode = ViewModeSubCommands
	return nil
}

// cmdAgentGroup handles /agent command group
func cmdAgentGroup(m *Model) tea.Cmd {
	m.selectedMainCommand = "agent"
	m.subcommandIdx = 0
	m.subcommandList = []SubcommandItem{
		{Name: "list", Description: "List available agents", Handler: cmdAgents},
		{Name: "set", Description: "Switch to agent", Handler: cmdAgent},
	}
	m.mode = ViewModeSubCommands
	return nil
}

// cmdCodeGroup handles /code command group
func cmdCodeGroup(m *Model) tea.Cmd {
	m.selectedMainCommand = "code"
	m.subcommandIdx = 0
	m.subcommandList = []SubcommandItem{
		{Name: "files", Description: "List project files", Handler: cmdFiles},
		{Name: "grep", Description: "Search in files", Handler: cmdGrep},
		{Name: "diff", Description: "Show git diff", Handler: cmdDiff},
		{Name: "git", Description: "Git operations", Handler: cmdGit},
		{Name: "review", Description: "Review code changes", Handler: cmdReview},
		{Name: "editor", Description: "Open in editor", Handler: cmdEditor},
	}
	m.mode = ViewModeSubCommands
	return nil
}

// cmdContextGroup handles /context command group
func cmdContextGroup(m *Model) tea.Cmd {
	m.selectedMainCommand = "context"
	m.subcommandIdx = 0
	m.subcommandList = []SubcommandItem{
		{Name: "show", Description: "Show current context", Handler: cmdContext},
		{Name: "memory", Description: "Show conversation memory", Handler: cmdMemory},
		{Name: "add", Description: "Add file to context", Handler: cmdAdd},
		{Name: "thinking", Description: "Toggle thinking mode", Handler: cmdThinking},
	}
	m.mode = ViewModeSubCommands
	return nil
}

// cmdToolsGroup handles /tools command group
func cmdToolsGroup(m *Model) tea.Cmd {
	m.selectedMainCommand = "tools"
	m.subcommandIdx = 0
	m.subcommandList = []SubcommandItem{
		{Name: "list", Description: "List available tools", Handler: cmdTools},
		{Name: "mcp", Description: "MCP server management", Handler: cmdMCP},
		{Name: "lsp", Description: "LSP status", Handler: cmdLSP},
	}
	m.mode = ViewModeSubCommands
	return nil
}

// cmdConfigGroup handles /config command group
func cmdConfigGroup(m *Model) tea.Cmd {
	m.selectedMainCommand = "config"
	m.subcommandIdx = 0
	m.subcommandList = []SubcommandItem{
		{Name: "show", Description: "Show configuration", Handler: cmdSettings},
		{Name: "themes", Description: "List available themes", Handler: cmdTheme},
		{Name: "theme", Description: "Change theme", Handler: cmdThemeSwitch},
		{Name: "lang", Description: "Set language", Handler: cmdLanguage},
	}
	m.mode = ViewModeSubCommands
	return nil
}

// cmdAuthGroup handles /auth command group
func cmdAuthGroup(m *Model) tea.Cmd {
	m.selectedMainCommand = "auth"
	m.subcommandIdx = 0
	m.subcommandList = []SubcommandItem{
		{Name: "status", Description: "Show auth status", Handler: cmdAuth},
		{Name: "login", Description: "Login to provider", Handler: cmdLogin},
		{Name: "logout", Description: "Logout from provider", Handler: cmdLogout},
	}
	m.mode = ViewModeSubCommands
	return nil
}

// cmdSystemGroup handles /system command group
func cmdSystemGroup(m *Model) tea.Cmd {
	m.selectedMainCommand = "system"
	m.subcommandIdx = 0
	m.subcommandList = []SubcommandItem{
		{Name: "status", Description: "Show system status", Handler: cmdStatus},
		{Name: "setup", Description: "Run initial setup", Handler: cmdSetup},
		{Name: "doctor", Description: "Run diagnostics", Handler: cmdDoctor},
		{Name: "version", Description: "Show version info", Handler: cmdVersion},
	}
	m.mode = ViewModeSubCommands
	return nil
}

// cmdHistory shows chat history
func cmdHistory(m *Model) tea.Cmd {
	if len(m.messages) == 0 {
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "No messages in history",
		})
		return nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📜 Chat History (%d messages)\n\n", len(m.messages)))

	for i, msg := range m.messages {
		role := msg.Role
		if role == "user" {
			role = "You"
		} else if role == "assistant" {
			role = "AI"
		}

		content := msg.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}

		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, role, content))
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: sb.String(),
	})
	return nil
}

// ============================================================================
// Phase 2: Work Mode Commands (Ask/Edit/Agent/Plan)
// ============================================================================

// cmdModeGroup handles /mode command group
func cmdModeGroup(m *Model) tea.Cmd {
	m.selectedMainCommand = "mode"
	m.subcommandIdx = 0
	m.subcommandList = []SubcommandItem{
		{Name: "ask", Description: "💬 Quick Q&A mode", Handler: func(m *Model) tea.Cmd {
			return m.switchWorkMode(WorkModeAsk)
		}},
		{Name: "edit", Description: "✏️  Code editing mode", Handler: func(m *Model) tea.Cmd {
			return m.switchWorkMode(WorkModeEdit)
		}},
		{Name: "agent", Description: "🤖 Autonomous agent mode", Handler: func(m *Model) tea.Cmd {
			return m.switchWorkMode(WorkModeAgent)
		}},
		{Name: "plan", Description: "📋 Task planning mode", Handler: func(m *Model) tea.Cmd {
			return m.switchWorkMode(WorkModePlan)
		}},
	}
	m.mode = ViewModeSubCommands
	return nil
}

// cmdMultiModel opens the multi-model selection interface
func cmdMultiModel(m *Model) tea.Cmd {
	// Initialize model selections if not already done
	if len(m.selectedModels) == 0 {
		m.initializeModelSelections()
	}

	// Push current view to stack and switch to multi-model select
	m.PushView(m.mode)
	m.mode = ViewModeMultiModelSelect
	m.selectionIdx = 0

	return nil
}

// cmdPresetGroup handles preset-related subcommands
func cmdPresetGroup(m *Model) tea.Cmd {
	if m.presetManager == nil {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: "Preset manager not initialized",
		})
		return nil
	}

	// Parse subcommand
	parts := strings.Fields(m.input.Value())
	if len(parts) < 2 {
		// Show preset subcommands
		m.selectedMainCommand = "preset"
		m.subcommandIdx = 0
		m.subcommandList = []SubcommandItem{
			{Name: "list", Description: "List all available presets", Handler: cmdPresetList},
			{Name: "load", Description: "Load a preset", Handler: cmdPresetLoad},
			{Name: "save", Description: "Save current selection as preset", Handler: cmdPresetSave},
			{Name: "delete", Description: "Delete a preset", Handler: cmdPresetDelete},
		}
		m.mode = ViewModeSubCommands
		return nil
	}

	subCmd := parts[1]
	switch subCmd {
	case "list":
		return cmdPresetList(m)
	case "load":
		return cmdPresetLoad(m)
	case "save":
		return cmdPresetSave(m)
	case "delete":
		return cmdPresetDelete(m)
	default:
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: fmt.Sprintf("Unknown preset subcommand: %s", subCmd),
		})
		return nil
	}
}

// cmdPresetList lists all available presets
func cmdPresetList(m *Model) tea.Cmd {
	if m.presetManager == nil {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: "Preset manager not initialized",
		})
		return nil
	}

	presets := m.presetManager.ListPresets()

	var output strings.Builder
	output.WriteString("📋 Available Model Presets:\n\n")

	if len(presets) == 0 {
		output.WriteString("No presets saved yet.\n")
	} else {
		for _, preset := range presets {
			output.WriteString(fmt.Sprintf("• %s\n", preset.Name))
			if preset.Description != "" {
				output.WriteString(fmt.Sprintf("  %s\n", preset.Description))
			}
			output.WriteString(fmt.Sprintf("  Models: %d | Used: %d times\n", len(preset.Models), preset.UsageCount))
			output.WriteString("\n")
		}
	}

	m.messages = append(m.messages, Message{Role: "system", Content: output.String()})
	return nil
}

// cmdPresetLoad loads a preset
func cmdPresetLoad(m *Model) tea.Cmd {
	if m.presetManager == nil {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: "Preset manager not initialized",
		})
		return nil
	}

	parts := strings.Fields(m.input.Value())
	if len(parts) < 3 {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: "Usage: /preset load <name>",
		})
		return nil
	}

	presetName := parts[2]
	preset, err := m.presetManager.LoadPreset(presetName)
	if err != nil {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: fmt.Sprintf("Failed to load preset: %v", err),
		})
		return nil
	}

	// Apply preset to model
	m.ApplyPreset(preset)

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("✓ Loaded preset '%s' with %d models", preset.Name, len(preset.Models)),
	})
	return nil
}

// cmdPresetSave saves current selection as a preset
func cmdPresetSave(m *Model) tea.Cmd {
	if m.presetManager == nil {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: "Preset manager not initialized",
		})
		return nil
	}

	parts := strings.Fields(m.input.Value())
	if len(parts) < 3 {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: "Usage: /preset save <name> [description]",
		})
		return nil
	}

	presetName := parts[2]
	description := ""
	if len(parts) > 3 {
		description = strings.Join(parts[3:], " ")
	}

	// Get currently selected models
	var selectedModels []ModelSelection
	for _, ms := range m.selectedModels {
		if ms.Selected {
			selectedModels = append(selectedModels, ms)
		}
	}

	if len(selectedModels) == 0 {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: "No models selected. Please select models first using /multimodel",
		})
		return nil
	}

	if err := m.presetManager.SavePreset(presetName, description, selectedModels); err != nil {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: fmt.Sprintf("Failed to save preset: %v", err),
		})
		return nil
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("✓ Saved preset '%s' with %d models", presetName, len(selectedModels)),
	})
	return nil
}

// cmdPresetDelete deletes a preset
func cmdPresetDelete(m *Model) tea.Cmd {
	if m.presetManager == nil {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: "Preset manager not initialized",
		})
		return nil
	}

	parts := strings.Fields(m.input.Value())
	if len(parts) < 3 {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: "Usage: /preset delete <name>",
		})
		return nil
	}

	presetName := parts[2]
	if err := m.presetManager.DeletePreset(presetName); err != nil {
		m.messages = append(m.messages, Message{
			Role:    "error",
			Content: fmt.Sprintf("Failed to delete preset: %v", err),
		})
		return nil
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("✓ Deleted preset '%s'", presetName),
	})
	return nil
}
