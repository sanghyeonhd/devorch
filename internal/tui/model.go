// Package tui provides terminal user interface components using bubbletea.
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"devorch/internal/session"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// globalSessionStore is the shared session store
var globalSessionStore *session.Store

// initSessionStore initializes the global session store
func initSessionStore() error {
	if globalSessionStore != nil {
		return nil
	}

	// Default session directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	sessionDir := filepath.Join(homeDir, ".config", "devorch", "sessions")

	store, err := session.NewStore(sessionDir)
	if err != nil {
		return err
	}
	globalSessionStore = store
	return nil
}

// ViewMode represents the current view mode.
type ViewMode int

const (
	ViewModeChat ViewMode = iota
	ViewModeSessions
	ViewModeSettings
	ViewModeHelp
	ViewModeCommands       // Slash command palette
	ViewModeModelSelect    // Model selection
	ViewModeProviderSelect // Provider selection
	ViewModeThemeSelect    // Theme selection
	ViewModeLogin          // OAuth login
	ViewModeMCP            // MCP server management
	ViewModeSetup          // Auto setup wizard
	ViewModeAgentSelect    // Agent mode selection
)

// Model represents the main TUI application state.
type Model struct {
	// Core state
	mode     ViewMode
	ready    bool
	width    int
	height   int
	quitting bool
	err      error

	// Components
	input    textinput.Model
	viewport viewport.Model
	spinner  spinner.Model

	// Chat state
	messages    []Message
	isStreaming bool
	sessionID   string
	sessionName string

	// Sessions list
	sessions        []SessionInfo
	selectedIdx     int
	sessionsFocused bool

	// Command palette
	showCommandPreview bool
	commandPalette     CommandPaletteModel

	// Interactive command autocomplete (OpenCode style)
	showCommandAutocomplete bool
	commandFilter           string
	commandSelectedIdx      int
	filteredCommands        []SlashCommand

	// Selection lists (for models, themes, providers)
	selectionList  []string
	selectionIdx   int
	selectionTitle string

	// Display options (OpenCode style)
	showDetails  bool // Show token count, timing, model info
	thinkingMode bool // Extended thinking for supported models

	// Theme
	theme Theme
}

// Message represents a chat message.
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
	Tokens  int
}

// SessionInfo represents session metadata for listing.
type SessionInfo struct {
	ID        string
	Name      string
	UpdatedAt string
	Messages  int
}

// New creates a new TUI model.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Type your message or / for commands..."
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 80

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	theme := DefaultTheme()

	return Model{
		mode:           ViewModeChat,
		input:          ti,
		spinner:        sp,
		theme:          theme,
		commandPalette: NewCommandPalette(theme),
		messages: []Message{
			{Role: "system", Content: "Welcome to DevOrch! Type / for commands or start chatting."},
		},
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
	)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 3
		footerHeight := 3
		inputHeight := 3
		verticalMargins := headerHeight + footerHeight + inputHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMargins)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargins
		}

		m.input.Width = msg.Width - 4
		m.viewport.SetContent(m.renderMessages())

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case StreamChunkMsg:
		m.handleStreamChunk(msg)
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()

	case StreamDoneMsg:
		m.isStreaming = false

	case ErrorMsg:
		m.err = msg.Err
		m.isStreaming = false

	case SessionsLoadedMsg:
		m.sessions = msg.Sessions

	case SessionCreatedMsg:
		m.sessionID = msg.SessionID
		m.sessionName = msg.SessionName
		m.messages = []Message{
			{Role: "system", Content: fmt.Sprintf("📝 New session created: %s", msg.SessionName)},
		}

	case SessionMessagesLoadedMsg:
		m.messages = msg.Messages
	}

	// Update input
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleKeyPress handles keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle special modes first
	switch m.mode {
	case ViewModeCommands:
		return m.handleCommandPaletteKey(msg)
	case ViewModeThemeSelect, ViewModeModelSelect, ViewModeProviderSelect, ViewModeLogin, ViewModeAgentSelect:
		return m.handleSelectionKey(msg)
	}

	// Handle interactive command autocomplete
	if m.showCommandAutocomplete {
		return m.handleCommandAutocompleteKey(msg)
	}

	switch msg.String() {
	case "ctrl+c", "ctrl+q":
		m.quitting = true
		return m, tea.Quit

	case "ctrl+n":
		// New session
		return m, m.newSession()

	case "ctrl+s":
		// Toggle sessions view
		if m.mode == ViewModeSessions {
			m.mode = ViewModeChat
		} else {
			m.mode = ViewModeSessions
		}
		return m, nil

	case "ctrl+h":
		// Toggle help
		if m.mode == ViewModeHelp {
			m.mode = ViewModeChat
		} else {
			m.mode = ViewModeHelp
		}
		return m, nil

	case "ctrl+p":
		// Open command palette
		m.mode = ViewModeCommands
		return m, nil

	case "tab":
		// Autocomplete slash command (when preview is showing)
		if m.showCommandAutocomplete && len(m.filteredCommands) > 0 {
			// Select the highlighted command
			selected := m.filteredCommands[m.commandSelectedIdx]
			m.input.SetValue("/" + selected.Name)
			m.input.CursorEnd()
			m.showCommandAutocomplete = false
			m.commandSelectedIdx = 0
		}
		return m, nil

	case "enter":
		if m.showCommandAutocomplete && len(m.filteredCommands) > 0 {
			// Execute the selected command directly
			selected := m.filteredCommands[m.commandSelectedIdx]
			m.input.SetValue("")
			m.showCommandAutocomplete = false
			m.commandSelectedIdx = 0

			// Execute the command
			result := selected.Handler(&m)
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()
			return m, result
		}
		if m.mode == ViewModeChat && !m.isStreaming {
			return m.sendMessage()
		}
		if m.mode == ViewModeSessions {
			return m.selectSession()
		}

	case "up":
		if m.showCommandAutocomplete && len(m.filteredCommands) > 0 {
			if m.commandSelectedIdx > 0 {
				m.commandSelectedIdx--
			}
			return m, nil
		}
		if m.mode == ViewModeSessions && m.selectedIdx > 0 {
			m.selectedIdx--
			return m, nil
		}

	case "down":
		if m.showCommandAutocomplete && len(m.filteredCommands) > 0 {
			if m.commandSelectedIdx < len(m.filteredCommands)-1 {
				m.commandSelectedIdx++
			}
			return m, nil
		}
		if m.mode == ViewModeSessions && m.selectedIdx < len(m.sessions)-1 {
			m.selectedIdx++
			return m, nil
		}

	case "esc":
		if m.showCommandAutocomplete {
			m.showCommandAutocomplete = false
			m.commandSelectedIdx = 0
			return m, nil
		}
		if m.mode != ViewModeChat {
			m.mode = ViewModeChat
			m.showCommandPreview = false
			return m, nil
		}
	}

	// IMPORTANT: Forward all other key events to the input component
	// This allows text input to work properly
	if m.mode == ViewModeChat {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Check if input starts with / to show command autocomplete
	input := m.input.Value()
	if strings.HasPrefix(input, "/") && len(input) >= 1 {
		m.showCommandAutocomplete = true
		m.commandFilter = input
		m.filteredCommands = FilterCommands(input)
		// Reset selection if filter changed
		if m.commandSelectedIdx >= len(m.filteredCommands) {
			m.commandSelectedIdx = 0
		}
	} else {
		m.showCommandAutocomplete = false
		m.commandSelectedIdx = 0
	}
	m.showCommandPreview = m.showCommandAutocomplete // Keep compatibility

	return m, tea.Batch(cmds...)
}

// handleCommandAutocompleteKey handles keys when autocomplete is showing
func (m Model) handleCommandAutocompleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		if m.commandSelectedIdx > 0 {
			m.commandSelectedIdx--
		}
		return m, nil
	case "down":
		if m.commandSelectedIdx < len(m.filteredCommands)-1 {
			m.commandSelectedIdx++
		}
		return m, nil
	case "tab", "enter":
		if len(m.filteredCommands) > 0 {
			selected := m.filteredCommands[m.commandSelectedIdx]
			if msg.String() == "enter" {
				// Execute immediately
				m.input.SetValue("")
				m.showCommandAutocomplete = false
				m.commandSelectedIdx = 0
				result := selected.Handler(&m)
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
				return m, result
			} else {
				// Tab: fill in command name
				m.input.SetValue("/" + selected.Name)
				m.input.CursorEnd()
				m.showCommandAutocomplete = false
				m.commandSelectedIdx = 0
			}
		}
		return m, nil
	case "esc":
		m.showCommandAutocomplete = false
		m.commandSelectedIdx = 0
		return m, nil
	default:
		// Forward to input for typing
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)

		// Update filtered commands
		input := m.input.Value()
		if strings.HasPrefix(input, "/") {
			m.filteredCommands = FilterCommands(input)
			if m.commandSelectedIdx >= len(m.filteredCommands) {
				m.commandSelectedIdx = 0
			}
		} else {
			m.showCommandAutocomplete = false
		}
		return m, cmd
	}
}

// handleCommandPaletteKey handles keys in command palette mode.
func (m Model) handleCommandPaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = ViewModeChat
		return m, nil
	case "enter":
		if cmd := m.commandPalette.SelectedCommand(); cmd != nil {
			m.mode = ViewModeChat
			return m, cmd.Handler(&m)
		}
	case "up", "k":
		if m.commandPalette.list.Index() > 0 {
			m.commandPalette.list.CursorUp()
		}
	case "down", "j":
		m.commandPalette.list.CursorDown()
	}

	var cmd tea.Cmd
	m.commandPalette, cmd = m.commandPalette.Update(msg)
	return m, cmd
}

// handleSelectionKey handles keys in selection list mode.
func (m Model) handleSelectionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.mode = ViewModeChat
		return m, nil
	case "enter":
		if m.selectionIdx >= 0 && m.selectionIdx < len(m.selectionList) {
			selected := m.selectionList[m.selectionIdx]
			switch m.mode {
			case ViewModeThemeSelect:
				m.theme = GetTheme(selected)
				m.messages = append(m.messages, Message{
					Role:    "system",
					Content: fmt.Sprintf("Theme changed to: %s", selected),
				})
			case ViewModeModelSelect:
				// Extract model ID (remove status indicator if present)
				modelID := strings.TrimSuffix(selected, " ✓")
				modelID = strings.TrimSpace(modelID)
				active := GetActiveProvider()
				SetActiveProvider(active.Provider, modelID)
				m.messages = append(m.messages, Message{
					Role:    "system",
					Content: fmt.Sprintf("Model changed to: %s\nProvider: %s", modelID, active.Provider),
				})
			case ViewModeProviderSelect:
				// Extract provider name
				providers := GetAvailableProviders()
				if m.selectionIdx < len(providers) {
					p := providers[m.selectionIdx]
					// Get first model for this provider
					models := GetModelsForProvider(p.Name)
					defaultModel := ""
					for _, model := range models {
						if model.IsPrimary {
							defaultModel = model.ID
							break
						}
					}
					if defaultModel == "" && len(models) > 0 {
						defaultModel = models[0].ID
					}
					SetActiveProvider(p.Name, defaultModel)
					m.messages = append(m.messages, Message{
						Role:    "system",
						Content: fmt.Sprintf("Provider changed to: %s\nDefault model: %s", p.DisplayName, defaultModel),
					})
				}
			case ViewModeAgentSelect:
				// Extract agent mode
				agentModes := []string{"coder", "chat", "researcher", "writer", "task"}
				if m.selectionIdx < len(agentModes) {
					agentMode := agentModes[m.selectionIdx]
					m.messages = append(m.messages, Message{
						Role:    "system",
						Content: fmt.Sprintf("🤖 Agent mode: %s\n\nYour AI assistant will now focus on %s tasks.", agentMode, agentMode),
					})
				}
			case ViewModeLogin:
				// Open browser for the selected provider
				providerMap := map[int]string{
					0: "anthropic",
					1: "openai",
					2: "google",
					3: "openrouter",
					4: "groq",
					5: "github",
				}
				if provider, ok := providerMap[m.selectionIdx]; ok {
					if err := OpenBrowserForLogin(provider); err != nil {
						m.messages = append(m.messages, Message{
							Role:    "system",
							Content: fmt.Sprintf("Failed to open browser: %v", err),
						})
					} else {
						envVar := ""
						switch provider {
						case "anthropic":
							envVar = "ANTHROPIC_API_KEY"
						case "openai":
							envVar = "OPENAI_API_KEY"
						case "google":
							envVar = "GOOGLE_API_KEY"
						case "openrouter":
							envVar = "OPENROUTER_API_KEY"
						case "groq":
							envVar = "GROQ_API_KEY"
						case "github":
							envVar = "GITHUB_TOKEN"
						}
						m.messages = append(m.messages, Message{
							Role:    "system",
							Content: fmt.Sprintf("🌐 Opening browser for %s...\n\nAfter getting your API key, set it with:\n  export %s=your_key_here\n\nOr add to ~/.config/devorch/config.yaml", provider, envVar),
						})
					}
				}
			}
			m.mode = ViewModeChat
		}
		return m, nil
	case "up", "k":
		if m.selectionIdx > 0 {
			m.selectionIdx--
		}
	case "down", "j":
		if m.selectionIdx < len(m.selectionList)-1 {
			m.selectionIdx++
		}
	}
	return m, nil
}

// sendMessage sends the current input as a message.
func (m Model) sendMessage() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	if text == "" {
		return m, nil
	}

	// Handle commands
	if strings.HasPrefix(text, "/") {
		return m.handleCommand(text)
	}

	// Add user message
	m.messages = append(m.messages, Message{
		Role:    "user",
		Content: text,
	})

	m.input.SetValue("")
	m.isStreaming = true
	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	// Return command to send message to LLM
	return m, m.sendToLLM(text)
}

// handleCommand processes slash commands.
func (m Model) handleCommand(cmdStr string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return m, nil
	}

	m.input.SetValue("")
	m.showCommandPreview = false

	// Find and execute the command
	cmdName := strings.TrimPrefix(parts[0], "/")
	if cmd := FindCommand(cmdName); cmd != nil {
		result := cmd.Handler(&m)

		// Handle special mode transitions - these are now handled in command handlers
		// Just update viewport
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, result
	}

	// Unknown command
	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("Unknown command: %s. Type /help for available commands.", parts[0]),
	})

	m.viewport.SetContent(m.renderMessages())
	m.viewport.GotoBottom()

	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	if !m.ready {
		return "Initializing..."
	}

	switch m.mode {
	case ViewModeSessions:
		return m.viewSessions()
	case ViewModeHelp:
		return m.viewHelp()
	case ViewModeCommands:
		return m.viewCommandPalette()
	case ViewModeThemeSelect, ViewModeModelSelect, ViewModeProviderSelect, ViewModeLogin, ViewModeAgentSelect:
		return m.viewSelectionList()
	case ViewModeSettings:
		return m.viewSettings()
	case ViewModeSetup:
		return m.viewSetup()
	case ViewModeMCP:
		return m.viewMCP()
	default:
		return m.viewChat()
	}
}

// viewChat renders the chat view.
func (m Model) viewChat() string {
	header := m.renderHeader()
	content := m.viewport.View()

	// Show interactive command autocomplete (OpenCode style)
	if m.showCommandAutocomplete && len(m.filteredCommands) > 0 {
		autocomplete := m.renderCommandAutocomplete()
		autocompleteBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("39")). // Bright blue
			Padding(0, 1).
			Width(m.width - 4).
			Render(autocomplete)
		content = content + "\n" + autocompleteBox
	}

	input := m.renderInput()
	footer := m.renderFooter()

	return fmt.Sprintf("%s\n%s\n%s\n%s", header, content, input, footer)
}

// renderCommandAutocomplete renders the interactive command list
func (m Model) renderCommandAutocomplete() string {
	var sb strings.Builder

	maxShow := 10
	totalCommands := len(m.filteredCommands)

	// Calculate scroll window based on selected index
	startIdx := 0
	if m.commandSelectedIdx >= maxShow {
		startIdx = m.commandSelectedIdx - maxShow + 1
	}
	endIdx := startIdx + maxShow
	if endIdx > totalCommands {
		endIdx = totalCommands
	}

	// Header with scroll indicator
	if startIdx > 0 {
		sb.WriteString(m.theme.Accent.Render("Commands") + m.theme.Subtle.Render(" (↑↓ to select, Enter to run, Tab to complete)") + "\n")
		sb.WriteString(m.theme.Subtle.Render("  ↑ more above") + "\n")
	} else {
		sb.WriteString(m.theme.Accent.Render("Commands") + m.theme.Subtle.Render(" (↑↓ to select, Enter to run, Tab to complete)") + "\n")
	}

	for i := startIdx; i < endIdx; i++ {
		cmd := m.filteredCommands[i]
		cursor := "  "
		nameStyle := m.theme.Normal
		descStyle := m.theme.Subtle

		if i == m.commandSelectedIdx {
			cursor = "▶ "
			nameStyle = m.theme.Selected
			descStyle = m.theme.Selected
		}

		// Format: ▶ /command - description
		cmdLine := fmt.Sprintf("%s/%s", cursor, cmd.Name)
		descLine := fmt.Sprintf(" - %s", cmd.Description)

		// Truncate if too long
		maxLineLen := m.width - 10
		if maxLineLen > 0 && len(cmdLine)+len(descLine) > maxLineLen {
			if maxLineLen-len(cmdLine) > 6 {
				descLine = descLine[:maxLineLen-len(cmdLine)-3] + "..."
			}
		}

		sb.WriteString(nameStyle.Render(cmdLine) + descStyle.Render(descLine) + "\n")
	}

	// Show scroll indicator at bottom
	if endIdx < totalCommands {
		sb.WriteString(m.theme.Subtle.Render(fmt.Sprintf("  ↓ %d more below", totalCommands-endIdx)))
	}

	return sb.String()
}

// viewCommandPalette renders the command palette.
func (m Model) viewCommandPalette() string {
	var b strings.Builder

	title := m.theme.Title.Render("⌘ Command Palette")
	b.WriteString(title + "\n\n")

	b.WriteString(m.commandPalette.View())

	b.WriteString("\n\n" + m.theme.Subtle.Render("Enter: Select | Esc: Close | ↑/↓: Navigate | Type to filter"))

	return b.String()
}

// viewSelectionList renders a selection list (themes, models, providers).
func (m Model) viewSelectionList() string {
	var b strings.Builder

	title := m.theme.Title.Render("📋 " + m.selectionTitle)
	b.WriteString(title + "\n\n")

	if len(m.selectionList) == 0 {
		b.WriteString(m.theme.Subtle.Render("No items available."))
	} else {
		maxShow := 15
		startIdx := 0
		if m.selectionIdx >= maxShow {
			startIdx = m.selectionIdx - maxShow + 1
		}
		endIdx := startIdx + maxShow
		if endIdx > len(m.selectionList) {
			endIdx = len(m.selectionList)
		}

		for i := startIdx; i < endIdx; i++ {
			cursor := "  "
			style := m.theme.Normal
			if i == m.selectionIdx {
				cursor = "▶ "
				style = m.theme.Selected
			}
			b.WriteString(style.Render(cursor+m.selectionList[i]) + "\n")
		}

		if len(m.selectionList) > maxShow {
			b.WriteString(m.theme.Subtle.Render(fmt.Sprintf("\n  ... %d more items", len(m.selectionList)-maxShow)))
		}
	}

	b.WriteString("\n\n" + m.theme.Subtle.Render("Enter: Select | Esc: Cancel | ↑/↓: Navigate"))

	return b.String()
}

// viewSettings renders the settings view.
func (m Model) viewSettings() string {
	return m.theme.Title.Render("⚙ Settings") + `

` + m.theme.Accent.Render("General") + `
  • Theme: ` + m.theme.Normal.Render("default") + `
  • Language: ` + m.theme.Normal.Render("en") + `

` + m.theme.Accent.Render("AI") + `
  • Default Provider: ` + m.theme.Normal.Render("Anthropic") + `
  • Default Model: ` + m.theme.Normal.Render("claude-sonnet-4-20250514") + `

` + m.theme.Accent.Render("Local") + `
  • Ollama: ` + m.theme.Success.Render("Installed") + `
  • Auto-download models: ` + m.theme.Normal.Render("Yes") + `

` + m.theme.Subtle.Render("Press Esc to close")
}

// viewLogin renders the login view.
func (m Model) viewLogin() string {
	return m.theme.Title.Render("🔐 Login") + `

` + m.theme.Accent.Render("Select a provider to authenticate:") + `

  1. Anthropic (Claude)
  2. OpenAI (GPT)
  3. Google (Gemini)
  4. GitHub Copilot
  5. Azure OpenAI

` + m.theme.Subtle.Render("Note: Local models (Ollama) don't require authentication.") + `

` + m.theme.Subtle.Render("Press number to login or Esc to cancel")
}

// viewSetup renders the setup wizard.
func (m Model) viewSetup() string {
	return m.theme.Title.Render("🚀 Auto Setup Wizard") + `

` + m.theme.Accent.Render("Detecting system...") + `

  ` + m.theme.Success.Render("✓") + ` OS: macOS (arm64)
  ` + m.theme.Success.Render("✓") + ` Memory: 16 GB
  ` + m.theme.Success.Render("✓") + ` GPU: Apple Silicon (Metal)
  ` + m.theme.Success.Render("✓") + ` Tier: High

` + m.theme.Accent.Render("Recommended Models:") + `
  • llama3.2:13b (primary)
  • qwen2.5-coder:14b (coding)
  • deepseek-coder:6.7b (fast)

` + m.theme.Warning.Render("This will download ~20GB of models.") + `

` + m.theme.Subtle.Render("Press Enter to start or Esc to cancel")
}

// viewMCP renders the MCP management view.
func (m Model) viewMCP() string {
	return m.theme.Title.Render("🔌 MCP Server Management") + `

` + m.theme.Accent.Render("Active Servers:") + `
  ` + m.theme.Subtle.Render("No MCP servers connected.") + `

` + m.theme.Accent.Render("Available Servers:") + `
  • filesystem - File system access
  • git - Git operations
  • github - GitHub API
  • memory - Persistent memory

` + m.theme.Subtle.Render("Press number to toggle or Esc to close")
}

// viewSessions renders the sessions list view.
func (m Model) viewSessions() string {
	var b strings.Builder

	title := m.theme.Title.Render("📋 Sessions")
	b.WriteString(title + "\n\n")

	if len(m.sessions) == 0 {
		b.WriteString(m.theme.Subtle.Render("No sessions found."))
	} else {
		for i, s := range m.sessions {
			cursor := "  "
			style := m.theme.Normal
			if i == m.selectedIdx {
				cursor = "▶ "
				style = m.theme.Selected
			}

			line := fmt.Sprintf("%s%s (%d messages) - %s",
				cursor, s.Name, s.Messages, s.UpdatedAt)
			b.WriteString(style.Render(line) + "\n")
		}
	}

	b.WriteString("\n" + m.theme.Subtle.Render("Enter: Select | Esc: Back | ↑/↓: Navigate"))

	return b.String()
}

// viewHelp renders the help view.
func (m Model) viewHelp() string {
	help := `
╔══════════════════════════════════════════════════════════════════╗
║                       DevOrch Help                               ║
╠══════════════════════════════════════════════════════════════════╣
║                                                                  ║
║  SLASH COMMANDS (type / then use ↑↓ to select)                  ║
║  ───────────────────────────────────────────────                 ║
║  Session:   /new /clear /reset /save /export /share /compact    ║
║  Project:   /init /files /grep /ls /diff /git                   ║
║  Model:     /model /models /provider /providers /agent          ║
║  Context:   /context /memory /add                               ║
║  Settings:  /settings /theme /themes /config                    ║
║  Auth:      /login /logout /auth                                ║
║  Tools:     /tools /mcp /lsp                                    ║
║  System:    /status /setup /doctor /version /install /bench     ║
║  Edit:      /undo /redo                                         ║
║  Help:      /help /quit                                         ║
║                                                                  ║
║  KEYBOARD SHORTCUTS                                              ║
║  ──────────────────                                              ║
║  Ctrl+N     New session                                         ║
║  Ctrl+S     Toggle sessions panel                               ║
║  Ctrl+H     Toggle this help                                    ║
║  Ctrl+P     Open command palette                                ║
║  Ctrl+C/Q   Quit application                                    ║
║  ↑/↓        Navigate in command list / selection lists          ║
║  Enter      Send message / Select / Execute command             ║
║  Tab        Complete command name                               ║
║  Esc        Close dialogs / Cancel                              ║
║                                                                  ║
║  TIPS                                                            ║
║  ────                                                            ║
║  • Type / to see all commands with ↑↓ navigation                ║
║  • Use /init to analyze your project                            ║
║  • Use /model to switch AI models                               ║
║  • Use /compact when context gets full                          ║
║                                                                  ║
║  Press Esc to close this help                                   ║
╚══════════════════════════════════════════════════════════════════╝
`
	return m.theme.Help.Render(help)
}

// renderHeader renders the header bar.
func (m Model) renderHeader() string {
	title := "DevOrch"
	if m.sessionName != "" {
		title += " - " + m.sessionName
	}

	status := ""
	if m.isStreaming {
		status = m.spinner.View() + " Thinking..."
	}

	left := m.theme.Title.Render(title)
	right := m.theme.Subtle.Render(status)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	return left + strings.Repeat(" ", gap) + right
}

// renderMessages renders all chat messages.
func (m Model) renderMessages() string {
	var b strings.Builder

	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			b.WriteString(m.theme.UserMsg.Render("You: " + msg.Content))
		case "assistant":
			b.WriteString(m.theme.AssistantMsg.Render("AI: " + msg.Content))
		case "system":
			b.WriteString(m.theme.SystemMsg.Render("⚙ " + msg.Content))
		}
		b.WriteString("\n\n")
	}

	return b.String()
}

// renderInput renders the input area.
func (m Model) renderInput() string {
	return m.theme.InputBox.Render(m.input.View())
}

// renderFooter renders the footer bar.
func (m Model) renderFooter() string {
	shortcuts := "Ctrl+N: New | Ctrl+S: Sessions | Ctrl+H: Help | Ctrl+C: Quit"

	// Debug info (temporary)
	debug := fmt.Sprintf(" | mode=%d ac=%v sel=%d", m.mode, m.showCommandAutocomplete, len(m.selectionList))
	return m.theme.Footer.Render(shortcuts + debug)
}

// Message types for tea.Cmd

// StreamChunkMsg represents a streaming chunk from the LLM.
type StreamChunkMsg struct {
	Content string
}

// StreamDoneMsg indicates streaming is complete.
type StreamDoneMsg struct{}

// ErrorMsg represents an error.
type ErrorMsg struct {
	Err error
}

// SessionsLoadedMsg indicates sessions have been loaded.
type SessionsLoadedMsg struct {
	Sessions []SessionInfo
}

// Commands

// ActiveProvider holds the currently selected provider and model
type ActiveProvider struct {
	Provider string
	Model    string
}

// globalActiveProvider tracks the active provider/model
var globalActiveProvider = ActiveProvider{
	Provider: "ollama",
	Model:    "tinyllama:latest", // default to lightweight installed model
}

// SetActiveProvider sets the active provider
func SetActiveProvider(provider, model string) {
	globalActiveProvider.Provider = provider
	globalActiveProvider.Model = model
}

// GetActiveProvider returns the active provider
func GetActiveProvider() ActiveProvider {
	return globalActiveProvider
}

func (m Model) sendToLLM(text string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		active := GetActiveProvider()

		// Convert messages to the right format (skip system welcome message)
		var chatMsgs []Message
		for _, msg := range m.messages {
			if msg.Role == "system" && strings.Contains(msg.Content, "Welcome to DevOrch") {
				continue
			}
			chatMsgs = append(chatMsgs, msg)
		}
		chatMsgs = append(chatMsgs, Message{Role: "user", Content: text})

		// Use the unified ChatWithProvider function
		response, err := ChatWithProvider(ctx, active.Provider, active.Model, chatMsgs)
		if err != nil {
			// Provider-specific error messages
			switch active.Provider {
			case "ollama":
				if strings.Contains(err.Error(), "model") || strings.Contains(err.Error(), "not found") {
					return StreamChunkMsg{
						Content: fmt.Sprintf("❌ Model '%s' not found.\n\nPlease install it first:\n  ollama pull %s\n\nOr run 'devorch setup' to install essential models.", active.Model, active.Model),
					}
				}
				return StreamChunkMsg{
					Content: fmt.Sprintf("❌ Ollama error: %v\n\nMake sure Ollama is running: ollama serve", err),
				}
			case "anthropic", "openai", "google", "openrouter", "groq":
				if strings.Contains(err.Error(), "not set") {
					var envVar string
					switch active.Provider {
					case "anthropic":
						envVar = "ANTHROPIC_API_KEY"
					case "openai":
						envVar = "OPENAI_API_KEY"
					case "google":
						envVar = "GOOGLE_API_KEY"
					case "openrouter":
						envVar = "OPENROUTER_API_KEY"
					case "groq":
						envVar = "GROQ_API_KEY"
					}
					return StreamChunkMsg{
						Content: fmt.Sprintf("❌ %s not set.\n\nUse /login to get your API key, then set:\n  export %s=your_key_here\n\nOr add it to ~/.config/devorch/config.yaml", envVar, envVar),
					}
				}
				return StreamChunkMsg{
					Content: fmt.Sprintf("❌ %s error: %v", active.Provider, err),
				}
			default:
				return StreamChunkMsg{
					Content: fmt.Sprintf("❌ Error: %v", err),
				}
			}
		}

		return StreamChunkMsg{Content: response}
	}
}

func (m *Model) handleStreamChunk(msg StreamChunkMsg) {
	if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == "assistant" {
		// Append to existing assistant message
		m.messages[len(m.messages)-1].Content += msg.Content
	} else {
		// Create new assistant message
		m.messages = append(m.messages, Message{
			Role:    "assistant",
			Content: msg.Content,
		})
	}
}

func (m Model) newSession() tea.Cmd {
	return func() tea.Msg {
		// Initialize session store if needed
		if err := initSessionStore(); err != nil {
			return StreamChunkMsg{Content: fmt.Sprintf("❌ Failed to init session store: %v", err)}
		}

		// Create new session
		ctx := context.Background()
		cwd, _ := os.Getwd()
		sessID := fmt.Sprintf("sess_%d", time.Now().UnixNano())
		sess, err := globalSessionStore.Create(ctx, sessID, "New Chat", cwd)
		if err != nil {
			return StreamChunkMsg{Content: fmt.Sprintf("❌ Failed to create session: %v", err)}
		}

		return SessionCreatedMsg{
			SessionID:   sess.ID,
			SessionName: sess.Name,
		}
	}
}

// SessionCreatedMsg is sent when a new session is created
type SessionCreatedMsg struct {
	SessionID   string
	SessionName string
}

func (m Model) loadSessions() tea.Cmd {
	return func() tea.Msg {
		// Initialize session store if needed
		if err := initSessionStore(); err != nil {
			return SessionsLoadedMsg{
				Sessions: []SessionInfo{},
			}
		}

		// Load sessions from store
		ctx := context.Background()
		summaries, err := globalSessionStore.ListSummaries(ctx, "")
		if err != nil {
			return SessionsLoadedMsg{
				Sessions: []SessionInfo{},
			}
		}

		var sessions []SessionInfo
		for _, s := range summaries {
			sessions = append(sessions, SessionInfo{
				ID:        s.ID,
				Name:      s.Name,
				UpdatedAt: s.UpdatedAt.Format("2006-01-02 15:04"),
				Messages:  s.Messages,
			})
		}

		// If no sessions, add placeholder
		if len(sessions) == 0 {
			sessions = append(sessions, SessionInfo{
				ID:        "new",
				Name:      "(No sessions - start chatting to create one)",
				UpdatedAt: time.Now().Format("2006-01-02 15:04"),
				Messages:  0,
			})
		}

		return SessionsLoadedMsg{
			Sessions: sessions,
		}
	}
}

func (m Model) selectSession() (tea.Model, tea.Cmd) {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.sessions) {
		s := m.sessions[m.selectedIdx]

		// Handle "new session" placeholder
		if s.ID == "new" {
			m.mode = ViewModeChat
			return m, m.newSession()
		}

		m.sessionID = s.ID
		m.sessionName = s.Name
		m.mode = ViewModeChat

		// Load session messages
		return m, m.loadSessionMessages(s.ID)
	}
	return m, nil
}

func (m Model) loadSessionMessages(sessionID string) tea.Cmd {
	return func() tea.Msg {
		if globalSessionStore == nil {
			return nil
		}

		ctx := context.Background()
		sess, err := globalSessionStore.Get(ctx, sessionID)
		if err != nil {
			return StreamChunkMsg{Content: fmt.Sprintf("❌ Failed to load session: %v", err)}
		}

		// Convert session messages to TUI messages
		var messages []Message
		messages = append(messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("📂 Loaded session: %s (%d messages)", sess.Name, len(sess.Messages)),
		})

		for _, msg := range sess.Messages {
			messages = append(messages, Message{
				Role:    string(msg.Role),
				Content: msg.Content,
				Tokens:  msg.Tokens,
			})
		}

		return SessionMessagesLoadedMsg{Messages: messages}
	}
}

// SessionMessagesLoadedMsg is sent when session messages are loaded
type SessionMessagesLoadedMsg struct {
	Messages []Message
}
