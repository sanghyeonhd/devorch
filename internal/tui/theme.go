// Package tui provides terminal user interface components.
package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines the visual style for the TUI.
type Theme struct {
	// Layout
	Title    lipgloss.Style
	Footer   lipgloss.Style
	InputBox lipgloss.Style

	// Messages
	UserMsg      lipgloss.Style
	AssistantMsg lipgloss.Style
	SystemMsg    lipgloss.Style

	// List
	Normal   lipgloss.Style
	Selected lipgloss.Style
	Subtle   lipgloss.Style

	// Special
	Help    lipgloss.Style
	Error   lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style

	// Code
	CodeBlock  lipgloss.Style
	InlineCode lipgloss.Style
}

// DefaultTheme returns the default dark theme.
func DefaultTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true),

		Normal: lipgloss.NewStyle(),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("215")),
	}
}

// LightTheme returns a light theme variant.
func LightTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("55")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Padding(0, 1),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("22")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("55")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true),

		Normal: lipgloss.NewStyle(),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("55")).
			Background(lipgloss.Color("254")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("22")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("124")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("28")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("166")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("254")).
			Foreground(lipgloss.Color("234")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("254")).
			Foreground(lipgloss.Color("124")),
	}
}

// HighContrastTheme returns a high contrast theme for accessibility.
func HighContrastTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("0")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("0")).
			Padding(0, 1),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("15")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color("15")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("0")).
			Foreground(lipgloss.Color("15")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("15")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("0")).
			Foreground(lipgloss.Color("14")).
			Bold(true),
	}
}

// WithColors creates a custom theme with specified colors.
func WithColors(primary, secondary, accent, background, text string) Theme {
	theme := DefaultTheme()

	theme.Title = theme.Title.Foreground(lipgloss.Color(primary))
	theme.UserMsg = theme.UserMsg.Foreground(lipgloss.Color(secondary))
	theme.AssistantMsg = theme.AssistantMsg.Foreground(lipgloss.Color(accent))
	theme.Selected = theme.Selected.Foreground(lipgloss.Color(primary))

	return theme
}
