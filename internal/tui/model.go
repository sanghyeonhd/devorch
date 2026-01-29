// Package tui provides terminal user interface components using bubbletea.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ViewMode represents the current view mode.
type ViewMode int

const (
	ViewModeChat ViewMode = iota
	ViewModeSessions
	ViewModeSettings
	ViewModeHelp
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
	ti.Placeholder = "Type your message..."
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 80

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return Model{
		mode:    ViewModeChat,
		input:   ti,
		spinner: sp,
		theme:   DefaultTheme(),
		messages: []Message{
			{Role: "system", Content: "Welcome to DevOrch! Type /help for commands."},
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

	case "enter":
		if m.mode == ViewModeChat && !m.isStreaming {
			return m.sendMessage()
		}
		if m.mode == ViewModeSessions {
			return m.selectSession()
		}

	case "up", "k":
		if m.mode == ViewModeSessions && m.selectedIdx > 0 {
			m.selectedIdx--
		}

	case "down", "j":
		if m.mode == ViewModeSessions && m.selectedIdx < len(m.sessions)-1 {
			m.selectedIdx++
		}

	case "esc":
		if m.mode != ViewModeChat {
			m.mode = ViewModeChat
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
func (m Model) handleCommand(cmd string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return m, nil
	}

	switch parts[0] {
	case "/help":
		m.messages = append(m.messages, Message{
			Role: "system",
			Content: `Available commands:
/help      - Show this help
/new       - Start new session
/sessions  - List sessions
/clear     - Clear current chat
/model     - Show current model
/quit      - Exit application

Shortcuts:
Ctrl+N     - New session
Ctrl+S     - Toggle sessions
Ctrl+H     - Toggle help
Ctrl+C     - Quit`,
		})

	case "/new":
		return m, m.newSession()

	case "/sessions":
		m.mode = ViewModeSessions
		return m, m.loadSessions()

	case "/clear":
		m.messages = []Message{
			{Role: "system", Content: "Chat cleared."},
		}

	case "/model":
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: "Current model: claude-sonnet-4-20250514",
		})

	case "/quit":
		m.quitting = true
		return m, tea.Quit
	}

	m.input.SetValue("")
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
	default:
		return m.viewChat()
	}
}

// viewChat renders the chat view.
func (m Model) viewChat() string {
	header := m.renderHeader()
	content := m.viewport.View()
	input := m.renderInput()
	footer := m.renderFooter()

	return fmt.Sprintf("%s\n%s\n%s\n%s", header, content, input, footer)
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
╔════════════════════════════════════════════════════════════╗
║                    DevOrch Help                            ║
╠════════════════════════════════════════════════════════════╣
║                                                            ║
║  COMMANDS                                                  ║
║  ────────                                                  ║
║  /help      Show this help message                        ║
║  /new       Start a new chat session                      ║
║  /sessions  List all saved sessions                       ║
║  /clear     Clear current chat history                    ║
║  /model     Show current AI model                         ║
║  /quit      Exit the application                          ║
║                                                            ║
║  KEYBOARD SHORTCUTS                                        ║
║  ──────────────────                                        ║
║  Ctrl+N     New session                                   ║
║  Ctrl+S     Toggle sessions panel                         ║
║  Ctrl+H     Toggle this help                              ║
║  Ctrl+C     Quit application                              ║
║  Ctrl+Q     Quit application                              ║
║  Enter      Send message / Select item                    ║
║  Esc        Return to chat view                           ║
║  ↑/↓        Navigate in lists                             ║
║                                                            ║
║  Press Esc to close this help                             ║
╚════════════════════════════════════════════════════════════╝
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
	return m.theme.Footer.Render(shortcuts)
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

func (m Model) sendToLLM(text string) tea.Cmd {
	return func() tea.Msg {
		// TODO: Integrate with actual LLM provider
		// For now, return a mock response
		return StreamChunkMsg{
			Content: "This is a placeholder response. The LLM integration will be connected here.",
		}
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
		// TODO: Create new session via session manager
		return nil
	}
}

func (m Model) loadSessions() tea.Cmd {
	return func() tea.Msg {
		// TODO: Load sessions from session manager
		return SessionsLoadedMsg{
			Sessions: []SessionInfo{
				{ID: "1", Name: "Debug session", UpdatedAt: "2025-01-01", Messages: 10},
				{ID: "2", Name: "Feature planning", UpdatedAt: "2025-01-02", Messages: 25},
			},
		}
	}
}

func (m Model) selectSession() (tea.Model, tea.Cmd) {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.sessions) {
		s := m.sessions[m.selectedIdx]
		m.sessionID = s.ID
		m.sessionName = s.Name
		m.mode = ViewModeChat
		// TODO: Load session messages
	}
	return m, nil
}
