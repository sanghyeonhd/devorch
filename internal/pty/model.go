// Package pty provides PTY components for TUI.
package pty

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// OutputMsg is sent when the terminal produces output
type OutputMsg struct {
	TerminalID string
	Data       []byte
}

// ExitMsg is sent when the terminal exits
type ExitMsg struct {
	TerminalID string
	ExitCode   int
}

// ResizeMsg is sent when the terminal should be resized
type ResizeMsg struct {
	TerminalID string
	Rows       uint16
	Cols       uint16
}

// TerminalModel represents a terminal component for Bubbletea
type TerminalModel struct {
	terminal *Terminal
	manager  *Manager
	id       string
	width    int
	height   int
	buffer   *ScreenBuffer
	style    TerminalStyle
	focused  bool
}

// TerminalStyle defines the visual style of the terminal
type TerminalStyle struct {
	Border      lipgloss.Border
	BorderColor lipgloss.Color
	Background  lipgloss.Color
	Foreground  lipgloss.Color
	CursorColor lipgloss.Color
}

// DefaultTerminalStyle returns the default terminal style
func DefaultTerminalStyle() TerminalStyle {
	return TerminalStyle{
		Border:      lipgloss.RoundedBorder(),
		BorderColor: lipgloss.Color("62"),
		Background:  lipgloss.Color("0"),
		Foreground:  lipgloss.Color("15"),
		CursorColor: lipgloss.Color("2"),
	}
}

// ScreenBuffer represents the terminal screen buffer
type ScreenBuffer struct {
	lines     []string
	cursorX   int
	cursorY   int
	width     int
	height    int
	scrollTop int
}

// NewScreenBuffer creates a new screen buffer
func NewScreenBuffer(width, height int) *ScreenBuffer {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = strings.Repeat(" ", width)
	}
	return &ScreenBuffer{
		lines:  lines,
		width:  width,
		height: height,
	}
}

// Write writes data to the screen buffer
func (b *ScreenBuffer) Write(data []byte) {
	for _, char := range string(data) {
		switch char {
		case '\n':
			b.cursorY++
			if b.cursorY >= b.height {
				b.scroll()
			}
		case '\r':
			b.cursorX = 0
		case '\b':
			if b.cursorX > 0 {
				b.cursorX--
			}
		case '\t':
			b.cursorX = ((b.cursorX / 8) + 1) * 8
			if b.cursorX >= b.width {
				b.cursorX = b.width - 1
			}
		case '\x1b':
			// Start of escape sequence - simplified handling
			continue
		default:
			if b.cursorX >= b.width {
				b.cursorX = 0
				b.cursorY++
				if b.cursorY >= b.height {
					b.scroll()
				}
			}
			if b.cursorY < len(b.lines) {
				line := []rune(b.lines[b.cursorY])
				for len(line) <= b.cursorX {
					line = append(line, ' ')
				}
				line[b.cursorX] = char
				b.lines[b.cursorY] = string(line)
			}
			b.cursorX++
		}
	}
}

// scroll scrolls the buffer up by one line
func (b *ScreenBuffer) scroll() {
	if len(b.lines) > 0 {
		b.lines = append(b.lines[1:], strings.Repeat(" ", b.width))
	}
	b.cursorY = b.height - 1
}

// Render returns the visible portion of the buffer
func (b *ScreenBuffer) Render() string {
	var sb strings.Builder
	for i, line := range b.lines {
		if i > 0 {
			sb.WriteRune('\n')
		}
		if len(line) > b.width {
			sb.WriteString(line[:b.width])
		} else {
			sb.WriteString(line)
		}
	}
	return sb.String()
}

// Clear clears the buffer
func (b *ScreenBuffer) Clear() {
	for i := range b.lines {
		b.lines[i] = strings.Repeat(" ", b.width)
	}
	b.cursorX = 0
	b.cursorY = 0
}

// Resize resizes the buffer
func (b *ScreenBuffer) Resize(width, height int) {
	newLines := make([]string, height)
	for i := range newLines {
		if i < len(b.lines) {
			if len(b.lines[i]) > width {
				newLines[i] = b.lines[i][:width]
			} else {
				newLines[i] = b.lines[i] + strings.Repeat(" ", width-len(b.lines[i]))
			}
		} else {
			newLines[i] = strings.Repeat(" ", width)
		}
	}
	b.lines = newLines
	b.width = width
	b.height = height
	if b.cursorX >= width {
		b.cursorX = width - 1
	}
	if b.cursorY >= height {
		b.cursorY = height - 1
	}
}

// NewTerminalModel creates a new terminal model
func NewTerminalModel(manager *Manager, id string, width, height int) (*TerminalModel, error) {
	cfg := DefaultConfig()
	if height > 2 {
		cfg.Rows = uint16(height - 2)
	}
	if width > 2 {
		cfg.Cols = uint16(width - 2)
	}

	t, err := manager.Create(id, cfg)
	if err != nil {
		return nil, err
	}

	m := &TerminalModel{
		terminal: t,
		manager:  manager,
		id:       id,
		width:    width,
		height:   height,
		buffer:   NewScreenBuffer(width-2, height-2),
		style:    DefaultTerminalStyle(),
		focused:  true,
	}

	return m, nil
}

// Init implements tea.Model
func (m *TerminalModel) Init() tea.Cmd {
	return m.listenForOutput()
}

// listenForOutput returns a command that listens for terminal output
func (m *TerminalModel) listenForOutput() tea.Cmd {
	return func() tea.Msg {
		select {
		case data, ok := <-m.terminal.Output():
			if !ok {
				return ExitMsg{TerminalID: m.id, ExitCode: 0}
			}
			return OutputMsg{TerminalID: m.id, Data: data}
		}
	}
}

// Update implements tea.Model
func (m *TerminalModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		var input []byte
		switch msg.Type {
		case tea.KeyEnter:
			input = []byte("\r")
		case tea.KeyBackspace:
			input = []byte{127}
		case tea.KeyTab:
			input = []byte("\t")
		case tea.KeyUp:
			input = []byte("\x1b[A")
		case tea.KeyDown:
			input = []byte("\x1b[B")
		case tea.KeyRight:
			input = []byte("\x1b[C")
		case tea.KeyLeft:
			input = []byte("\x1b[D")
		case tea.KeyCtrlC:
			input = []byte{3}
		case tea.KeyCtrlD:
			input = []byte{4}
		case tea.KeyCtrlZ:
			input = []byte{26}
		case tea.KeyCtrlL:
			input = []byte{12}
		case tea.KeyEscape:
			input = []byte{27}
		case tea.KeyRunes:
			input = []byte(string(msg.Runes))
		case tea.KeySpace:
			input = []byte(" ")
		}

		if len(input) > 0 {
			m.terminal.Write(input)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		newRows := uint16(msg.Height - 2)
		newCols := uint16(msg.Width - 2)
		m.terminal.Resize(newRows, newCols)
		m.buffer.Resize(int(newCols), int(newRows))
		return m, nil

	case OutputMsg:
		if msg.TerminalID == m.id {
			m.buffer.Write(msg.Data)
			return m, m.listenForOutput()
		}
		return m, nil

	case ExitMsg:
		if msg.TerminalID == m.id {
			// Terminal exited
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

// View implements tea.Model
func (m *TerminalModel) View() string {
	borderStyle := lipgloss.NewStyle().
		Border(m.style.Border).
		BorderForeground(m.style.BorderColor).
		Width(m.width - 2).
		Height(m.height - 2)

	if m.focused {
		borderStyle = borderStyle.BorderForeground(lipgloss.Color("2"))
	}

	content := m.buffer.Render()
	return borderStyle.Render(content)
}

// Focus sets the focus state
func (m *TerminalModel) Focus() {
	m.focused = true
}

// Blur removes focus
func (m *TerminalModel) Blur() {
	m.focused = false
}

// IsFocused returns whether the terminal is focused
func (m *TerminalModel) IsFocused() bool {
	return m.focused
}

// ID returns the terminal ID
func (m *TerminalModel) ID() string {
	return m.id
}

// Write writes data to the terminal
func (m *TerminalModel) Write(data []byte) error {
	return m.terminal.Write(data)
}

// Close closes the terminal
func (m *TerminalModel) Close() error {
	return m.manager.Close(m.id)
}

// SendCommand sends a command to the terminal
func (m *TerminalModel) SendCommand(cmd string) error {
	return m.terminal.Write([]byte(cmd + "\r"))
}

// Clear clears the terminal screen
func (m *TerminalModel) Clear() {
	m.buffer.Clear()
	m.terminal.Write([]byte("\x1b[2J\x1b[H"))
}

// Title returns the terminal title
func (m *TerminalModel) Title() string {
	return fmt.Sprintf("Terminal: %s", m.id)
}
