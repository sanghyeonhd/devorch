// Package tui provides terminal user interface components.
package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// App represents the main TUI application.
type App struct {
	model   Model
	program *tea.Program
}

// NewApp creates a new TUI application.
func NewApp(opts ...Option) *App {
	model := New()

	for _, opt := range opts {
		opt(&model)
	}

	return &App{
		model: model,
	}
}

// Option is a function that configures the TUI model.
type Option func(*Model)

// WithTheme sets the theme for the TUI.
func WithTheme(theme Theme) Option {
	return func(m *Model) {
		m.theme = theme
	}
}

// WithSessionID sets the initial session ID.
func WithSessionID(id string) Option {
	return func(m *Model) {
		m.sessionID = id
	}
}

// WithSessionName sets the initial session name.
func WithSessionName(name string) Option {
	return func(m *Model) {
		m.sessionName = name
	}
}

// WithMessages sets initial messages.
func WithMessages(messages []Message) Option {
	return func(m *Model) {
		m.messages = messages
	}
}

// Run starts the TUI application.
func (a *App) Run() error {
	a.program = tea.NewProgram(
		a.model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := a.program.Run()
	return err
}

// RunOnce runs the TUI until it quits.
func (a *App) RunOnce() (Model, error) {
	a.program = tea.NewProgram(
		a.model,
		tea.WithAltScreen(),
	)

	finalModel, err := a.program.Run()
	if err != nil {
		return Model{}, err
	}

	return finalModel.(Model), nil
}

// Send sends a message to the running program.
func (a *App) Send(msg tea.Msg) {
	if a.program != nil {
		a.program.Send(msg)
	}
}

// Quit signals the program to quit.
func (a *App) Quit() {
	if a.program != nil {
		a.program.Quit()
	}
}

// RunInteractive is the main entry point for interactive mode.
func RunInteractive(sessionID string) error {
	app := NewApp(
		WithSessionID(sessionID),
		WithTheme(DefaultTheme()),
	)

	return app.Run()
}

// Simple output functions for non-interactive use

// Print prints a message to stdout.
func Print(msg string) {
	fmt.Println(msg)
}

// PrintStyled prints a styled message.
func PrintStyled(msg string, theme Theme) {
	fmt.Println(theme.Normal.Render(msg))
}

// PrintError prints an error message.
func PrintError(err error, theme Theme) {
	fmt.Fprintln(os.Stderr, theme.Error.Render("Error: "+err.Error()))
}

// PrintSuccess prints a success message.
func PrintSuccess(msg string, theme Theme) {
	fmt.Println(theme.Success.Render("✓ " + msg))
}

// PrintWarning prints a warning message.
func PrintWarning(msg string, theme Theme) {
	fmt.Println(theme.Warning.Render("⚠ " + msg))
}

// PrintInfo prints an info message.
func PrintInfo(msg string, theme Theme) {
	fmt.Println(theme.Subtle.Render("ℹ " + msg))
}

// Spinner functions for CLI progress indication

// StartSpinner creates and returns a spinner program.
func StartSpinner(message string) *tea.Program {
	m := spinnerModel{message: message}
	p := tea.NewProgram(m)
	go func() {
		_, _ = p.Run()
	}()
	return p
}

// StopSpinner stops a running spinner.
func StopSpinner(p *tea.Program) {
	if p != nil {
		p.Quit()
	}
}

// spinnerModel is a simple spinner for CLI use.
type spinnerModel struct {
	message string
	done    bool
}

func (m spinnerModel) Init() tea.Cmd {
	return nil
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.QuitMsg:
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m spinnerModel) View() string {
	if m.done {
		return ""
	}
	return "⠋ " + m.message
}
