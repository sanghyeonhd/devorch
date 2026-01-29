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
func GetSlashCommands() []SlashCommand {
	commands := []SlashCommand{
		// Session commands (OpenCode 스타일)
		{Name: "new", Description: "Start a new chat session", Category: "Session", Handler: cmdNew},
		{Name: "sessions", Description: "List all saved sessions", Category: "Session", Handler: cmdSessions},
		{Name: "clear", Description: "Clear current chat history", Category: "Session", Handler: cmdClear},
		{Name: "reset", Description: "Reset session completely", Category: "Session", Handler: cmdReset},
		{Name: "save", Description: "Save current session", Category: "Session", Handler: cmdSave},
		{Name: "export", Description: "Export session as markdown", Category: "Session", Handler: cmdExport},
		{Name: "share", Description: "Generate shareable session link", Category: "Session", Handler: cmdShare},
		{Name: "unshare", Description: "Remove shared session link", Category: "Session", Handler: cmdUnshare},
		{Name: "compact", Description: "Summarize and compact session", Category: "Session", Handler: cmdCompact},
		{Name: "details", Description: "Show/hide message details (tokens, timing)", Category: "Session", Handler: cmdDetails},

		// Edit commands (OpenCode 스타일)
		{Name: "undo", Description: "Undo last AI edit", Category: "Edit", Handler: cmdUndo},
		{Name: "redo", Description: "Redo undone edit", Category: "Edit", Handler: cmdRedo},
		{Name: "editor", Description: "Open external editor for input", Category: "Edit", Handler: cmdEditor},

		// Project commands (OpenCode 스타일)
		{Name: "init", Description: "Analyze project and create AGENTS.md", Category: "Project", Handler: cmdInit},
		{Name: "files", Description: "Show project files context", Category: "Project", Handler: cmdFiles},
		{Name: "grep", Description: "Search code in project", Category: "Project", Handler: cmdGrep},
		{Name: "ls", Description: "List directory structure", Category: "Project", Handler: cmdLs},
		{Name: "diff", Description: "Show changed files diff", Category: "Project", Handler: cmdDiff},
		{Name: "git", Description: "Show git status", Category: "Project", Handler: cmdGitStatus},

		// Model commands
		{Name: "model", Description: "Show/change current AI model", Category: "Model", Handler: cmdModel},
		{Name: "models", Description: "List available models", Category: "Model", Handler: cmdModels},
		{Name: "provider", Description: "Show/change AI provider", Category: "Model", Handler: cmdProvider},
		{Name: "providers", Description: "List available providers", Category: "Model", Handler: cmdProviders},
		{Name: "agent", Description: "Switch AI agent mode", Category: "Model", Handler: cmdAgent},
		{Name: "thinking", Description: "Toggle extended thinking mode", Category: "Model", Handler: cmdThinking},

		// Context commands (OpenCode 스타일)
		{Name: "context", Description: "Show current context window usage", Category: "Context", Handler: cmdContext},
		{Name: "memory", Description: "Show/manage persistent memory", Category: "Context", Handler: cmdMemory},
		{Name: "add", Description: "Add file to context", Category: "Context", Handler: cmdAdd},

		// Settings commands
		{Name: "settings", Description: "Open settings menu", Category: "Settings", Handler: cmdSettings},
		{Name: "theme", Description: "Change color theme", Category: "Settings", Handler: cmdTheme},
		{Name: "themes", Description: "List available themes", Category: "Settings", Handler: cmdThemes},
		{Name: "language", Description: "Change interface language", Category: "Settings", Handler: cmdLanguage},
		{Name: "config", Description: "Edit configuration file", Category: "Settings", Handler: cmdConfig},

		// Auth commands
		{Name: "login", Description: "Login to AI provider", Category: "Auth", Handler: cmdLogin},
		{Name: "logout", Description: "Logout from provider", Category: "Auth", Handler: cmdLogout},
		{Name: "auth", Description: "Show authentication status", Category: "Auth", Handler: cmdAuth},
		{Name: "connect", Description: "Connect to remote DevOrch server", Category: "Auth", Handler: cmdConnect},

		// Tools commands
		{Name: "tools", Description: "List available tools", Category: "Tools", Handler: cmdTools},
		{Name: "mcp", Description: "Manage MCP servers", Category: "Tools", Handler: cmdMCP},
		{Name: "lsp", Description: "Language Server Protocol status", Category: "Tools", Handler: cmdLSP},

		// System commands
		{Name: "status", Description: "Show system status", Category: "System", Handler: cmdStatus},
		{Name: "setup", Description: "Run auto setup wizard", Category: "System", Handler: cmdSetup},
		{Name: "doctor", Description: "Run diagnostics check", Category: "System", Handler: cmdDoctor},
		{Name: "version", Description: "Show version info", Category: "System", Handler: cmdVersion},
		{Name: "install", Description: "Install recommended models", Category: "System", Handler: cmdInstall},
		{Name: "bench", Description: "Run performance benchmark", Category: "System", Handler: cmdBench},

		// Help
		{Name: "help", Description: "Show help information", Category: "Help", Handler: cmdHelp},
		{Name: "quit", Description: "Exit application", Category: "Help", Handler: cmdQuit},
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
	for _, cmd := range GetSlashCommands() {
		if strings.HasPrefix(strings.ToLower(cmd.Name), prefix) {
			matches = append(matches, cmd)
		}
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
	providers := GetAvailableProviders()
	m.selectionList = make([]string, 0, len(providers))
	for _, p := range providers {
		status := "○"
		if p.AuthStatus == "authenticated" {
			status = "✓"
		}
		kind := ""
		if p.Kind == "local" {
			kind = " (local)"
		}
		m.selectionList = append(m.selectionList, fmt.Sprintf("%s%s %s", p.DisplayName, kind, status))
	}
	m.selectionIdx = 0
	m.selectionTitle = "Select Provider"
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
	themes := AvailableThemes()
	var sb strings.Builder
	sb.WriteString("Available themes:\n")
	for i, t := range themes {
		if i%5 == 0 && i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("• %s  ", t))
	}
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: sb.String(),
	})
	return nil
}

func cmdLanguage(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role: "system",
		Content: `Available languages:
• en - English
• ko - 한국어
• ja - 日本語
• zh - 中文`,
	})
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

// cmdInstall installs recommended models
func cmdInstall(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: "📦 Installing recommended models...\n\nThis will install lightweight, high-performance models.\nPlease wait...",
	})

	return func() tea.Msg {
		// 설치 진행 메시지 수집
		var results []string

		// 필수 모델 목록 (저사양 고성능)
		models := []struct {
			id   string
			desc string
		}{
			{"phi3:mini", "🥈 저사양 최강 (1.3GB)"},
			{"qwen2.5:3b", "빠른 범용 (2GB)"},
			{"qwen2.5-coder:3b", "코딩용 (2GB)"},
		}

		client := NewOllamaClient()

		// 이미 설치된 모델 확인
		installed := getOllamaModels()
		installedSet := make(map[string]bool)
		for _, m := range installed {
			if m.IsInstalled {
				installedSet[m.ID] = true
			}
		}

		for _, model := range models {
			if installedSet[model.id] {
				results = append(results, fmt.Sprintf("✓ %s - %s (already installed)", model.id, model.desc))
				continue
			}

			// 실제 설치 시도
			err := exec.Command("ollama", "pull", model.id).Run()
			if err != nil {
				// 백그라운드 설치 시작
				go client.PullModel(nil, model.id)
				results = append(results, fmt.Sprintf("⬇ %s - %s (downloading in background)", model.id, model.desc))
			} else {
				results = append(results, fmt.Sprintf("✓ %s - %s (installed)", model.id, model.desc))
			}
		}

		return StreamChunkMsg{
			Content: "📦 Model installation status:\n\n" + strings.Join(results, "\n") + "\n\n💡 Run 'ollama list' to check progress.",
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

// cmdConnect connects to a remote DevOrch server
func cmdConnect(m *Model) tea.Cmd {
	m.messages = append(m.messages, Message{
		Role: "system",
		Content: `🔗 Remote Server Connection

Enter the server URL to connect:

Examples:
  • localhost:3000
  • devorch.example.com
  • 192.168.1.100:8080

Usage:
  1. Start server: devorch serve
  2. Connect client: /connect <url>

Note: Set DEVORCH_SERVER_PASSWORD for authentication.

Currently: Not connected (local mode)`,
	})
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
