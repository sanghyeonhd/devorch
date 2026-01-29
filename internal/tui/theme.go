// Package tui provides terminal user interface components.
package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines the visual style for the TUI.
type Theme struct {
	// Layout
	Title    lipgloss.Style
	Footer   lipgloss.Style
	InputBox lipgloss.Style
	Border   lipgloss.Style

	// Messages
	UserMsg      lipgloss.Style
	AssistantMsg lipgloss.Style
	SystemMsg    lipgloss.Style

	// List
	Normal   lipgloss.Style
	Selected lipgloss.Style
	Subtle   lipgloss.Style
	Accent   lipgloss.Style

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

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),

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

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true),

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

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")),

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

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("55")).
			Bold(true),

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

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")),

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

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")).
			Bold(true),

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

// =============================================================================
// Popular Themes from the community
// =============================================================================

// DraculaTheme returns the Dracula color scheme theme.
// Based on https://draculatheme.com/
func DraculaTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ff79c6")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272a4")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272a4")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#bd93f9")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50fa7b")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f8f8f2")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272a4")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f8f8f2")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#282a36")).
			Background(lipgloss.Color("#bd93f9")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6272a4")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff79c6")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8be9fd")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff5555")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50fa7b")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffb86c")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#282a36")).
			Foreground(lipgloss.Color("#f8f8f2")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#44475a")).
			Foreground(lipgloss.Color("#ff79c6")),
	}
}

// NordTheme returns the Nord color scheme theme.
// Based on https://www.nordtheme.com/
func NordTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#88c0d0")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4c566a")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4c566a")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#5e81ac")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a3be8c")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#eceff4")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4c566a")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d8dee9")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#2e3440")).
			Background(lipgloss.Color("#88c0d0")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4c566a")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#81a1c1")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8fbcbb")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#bf616a")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a3be8c")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ebcb8b")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#2e3440")).
			Foreground(lipgloss.Color("#d8dee9")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#3b4252")).
			Foreground(lipgloss.Color("#88c0d0")),
	}
}

// GruvboxDarkTheme returns the Gruvbox Dark color scheme theme.
// Based on https://github.com/morhetz/gruvbox
func GruvboxDarkTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#fe8019")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#928374")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#504945")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#d79921")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b8bb26")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ebdbb2")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#928374")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ebdbb2")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#282828")).
			Background(lipgloss.Color("#fabd2f")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#928374")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fe8019")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#83a598")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fb4934")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b8bb26")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#fabd2f")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#282828")).
			Foreground(lipgloss.Color("#ebdbb2")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#3c3836")).
			Foreground(lipgloss.Color("#fe8019")),
	}
}

// GruvboxLightTheme returns the Gruvbox Light color scheme theme.
func GruvboxLightTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#af3a03")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#928374")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d5c4a1")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#b57614")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#79740e")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3c3836")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#928374")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3c3836")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#fbf1c7")).
			Background(lipgloss.Color("#b57614")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#928374")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#af3a03")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#076678")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9d0006")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#79740e")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b57614")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#fbf1c7")).
			Foreground(lipgloss.Color("#3c3836")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#ebdbb2")).
			Foreground(lipgloss.Color("#af3a03")),
	}
}

// TokyoNightTheme returns the Tokyo Night color scheme theme.
// Based on https://github.com/enkia/tokyo-night-vscode-theme
func TokyoNightTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#bb9af7")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3b4261")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7aa2f7")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ece6a")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c0caf5")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a9b1d6")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1a1b26")).
			Background(lipgloss.Color("#7aa2f7")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7dcfff")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7dcfff")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f7768e")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ece6a")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e0af68")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#1a1b26")).
			Foreground(lipgloss.Color("#a9b1d6")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#24283b")).
			Foreground(lipgloss.Color("#bb9af7")),
	}
}

// TokyoNightStormTheme returns the Tokyo Night Storm variant theme.
func TokyoNightStormTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#bb9af7")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3b4261")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7aa2f7")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ece6a")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c0caf5")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a9b1d6")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#24283b")).
			Background(lipgloss.Color("#7aa2f7")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2ac3de")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7dcfff")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f7768e")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ece6a")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e0af68")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#24283b")).
			Foreground(lipgloss.Color("#a9b1d6")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#1f2335")).
			Foreground(lipgloss.Color("#bb9af7")),
	}
}

// CatppuccinMochaTheme returns the Catppuccin Mocha color scheme theme.
// Based on https://github.com/catppuccin/catppuccin
func CatppuccinMochaTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#cba6f7")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6c7086")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#45475a")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#89b4fa")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6e3a1")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cdd6f4")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6c7086")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cdd6f4")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1e1e2e")).
			Background(lipgloss.Color("#89b4fa")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6c7086")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f5c2e7")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94e2d5")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f38ba8")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6e3a1")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f9e2af")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#1e1e2e")).
			Foreground(lipgloss.Color("#cdd6f4")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#313244")).
			Foreground(lipgloss.Color("#f5c2e7")),
	}
}

// CatppuccinLatteTheme returns the Catppuccin Latte (light) color scheme theme.
func CatppuccinLatteTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8839ef")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ca0b0")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ccd0da")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#1e66f5")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#40a02b")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4c4f69")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ca0b0")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4c4f69")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#eff1f5")).
			Background(lipgloss.Color("#1e66f5")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ca0b0")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ea76cb")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#179299")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d20f39")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#40a02b")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#df8e1d")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#eff1f5")).
			Foreground(lipgloss.Color("#4c4f69")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#e6e9ef")).
			Foreground(lipgloss.Color("#8839ef")),
	}
}

// MonokaiTheme returns the Monokai color scheme theme.
func MonokaiTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#f92672")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#75715e")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#49483e")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#a6e22e")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6e22e")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f8f8f2")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#75715e")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f8f8f2")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#272822")).
			Background(lipgloss.Color("#e6db74")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#75715e")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#66d9ef")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#66d9ef")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f92672")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6e22e")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e6db74")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#272822")).
			Foreground(lipgloss.Color("#f8f8f2")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#3e3d32")).
			Foreground(lipgloss.Color("#f92672")),
	}
}

// SolarizedDarkTheme returns the Solarized Dark color scheme theme.
// Based on https://ethanschoonover.com/solarized/
func SolarizedDarkTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#268bd2")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#586e75")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#073642")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2aa198")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#859900")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#839496")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#586e75")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#839496")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#002b36")).
			Background(lipgloss.Color("#268bd2")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#586e75")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cb4b16")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2aa198")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#dc322f")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#859900")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b58900")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#002b36")).
			Foreground(lipgloss.Color("#839496")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#073642")).
			Foreground(lipgloss.Color("#2aa198")),
	}
}

// SolarizedLightTheme returns the Solarized Light color scheme theme.
func SolarizedLightTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#268bd2")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#93a1a1")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#eee8d5")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2aa198")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#859900")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#657b83")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#93a1a1")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#657b83")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#fdf6e3")).
			Background(lipgloss.Color("#268bd2")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#93a1a1")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cb4b16")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2aa198")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#dc322f")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#859900")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b58900")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#fdf6e3")).
			Foreground(lipgloss.Color("#657b83")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#eee8d5")).
			Foreground(lipgloss.Color("#2aa198")),
	}
}

// RosePineTheme returns the Rose Pine color scheme theme.
// Based on https://rosepinetheme.com/
func RosePineTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c4a7e7")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e6a86")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#26233a")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#31748f")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ccfd8")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e0def4")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e6a86")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e0def4")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#191724")).
			Background(lipgloss.Color("#c4a7e7")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e6a86")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ebbcba")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#31748f")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#eb6f92")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ccfd8")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f6c177")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#191724")).
			Foreground(lipgloss.Color("#e0def4")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#26233a")).
			Foreground(lipgloss.Color("#c4a7e7")),
	}
}

// OneDarkTheme returns the One Dark color scheme theme.
// Based on Atom One Dark
func OneDarkTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c678dd")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3e4451")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#61afef")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98c379")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#abb2bf")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#abb2bf")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#282c34")).
			Background(lipgloss.Color("#61afef")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6370")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#56b6c2")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#56b6c2")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e06c75")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98c379")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e5c07b")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#282c34")).
			Foreground(lipgloss.Color("#abb2bf")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#3e4451")).
			Foreground(lipgloss.Color("#e06c75")),
	}
}

// KanagawaTheme returns the Kanagawa color scheme theme.
// Based on https://github.com/rebelot/kanagawa.nvim
func KanagawaTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#957fb8")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#54546d")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#363646")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7e9cd8")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98bb6c")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#dcd7ba")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#54546d")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#dcd7ba")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1f1f28")).
			Background(lipgloss.Color("#7e9cd8")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#54546d")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffa066")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7fb4ca")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e82424")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98bb6c")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e6c384")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#1f1f28")).
			Foreground(lipgloss.Color("#dcd7ba")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#2a2a37")).
			Foreground(lipgloss.Color("#957fb8")),
	}
}

// FlexokiDarkTheme returns the Flexoki Dark color scheme theme.
// Based on https://stephango.com/flexoki
func FlexokiDarkTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#d14d41")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#575653")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#343331")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4385be")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#879a39")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cecdc3")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#575653")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cecdc3")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#100f0f")).
			Background(lipgloss.Color("#4385be")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#575653")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#da702c")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3aa99f")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d14d41")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#879a39")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d0a215")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#100f0f")).
			Foreground(lipgloss.Color("#cecdc3")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#1c1b1a")).
			Foreground(lipgloss.Color("#8b7ec8")),
	}
}

// FlexokiLightTheme returns the Flexoki Light color scheme theme.
func FlexokiLightTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#af3029")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#878580")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e6e4d9")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#205ea6")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#66800b")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#100f0f")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#878580")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#100f0f")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#fffcf0")).
			Background(lipgloss.Color("#205ea6")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#878580")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#bc5215")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#24837b")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#af3029")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#66800b")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ad8301")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#fffcf0")).
			Foreground(lipgloss.Color("#100f0f")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#f2f0e5")).
			Foreground(lipgloss.Color("#5e409d")),
	}
}

// EverforestDarkTheme returns the Everforest Dark color scheme theme.
// Based on https://github.com/sainnhe/everforest
func EverforestDarkTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#e67e80")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#859289")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#374145")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#83c092")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a7c080")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d3c6aa")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#859289")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d3c6aa")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#2d353b")).
			Background(lipgloss.Color("#83c092")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#859289")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e69875")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7fbbb3")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e67e80")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a7c080")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#dbbc7f")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#2d353b")).
			Foreground(lipgloss.Color("#d3c6aa")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#343f44")).
			Foreground(lipgloss.Color("#d699b6")),
	}
}

// AyuDarkTheme returns the Ayu Dark color scheme theme.
// Based on https://github.com/ayu-theme/ayu-colors
func AyuDarkTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffb454")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6773")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#2d3640")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#39bae6")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c2d94c")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e6e1cf")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6773")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e6e1cf")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0a0e14")).
			Background(lipgloss.Color("#39bae6")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5c6773")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f29668")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#95e6cb")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff3333")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c2d94c")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffb454")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#0a0e14")).
			Foreground(lipgloss.Color("#e6e1cf")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#0d1017")).
			Foreground(lipgloss.Color("#ffb454")),
	}
}

// MaterialDarkTheme returns the Material Dark color scheme theme.
func MaterialDarkTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c792ea")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#546e7a")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#37474f")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#82aaff")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c3e88d")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#eeffff")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#546e7a")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#eeffff")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#263238")).
			Background(lipgloss.Color("#82aaff")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#546e7a")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f78c6c")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#89ddff")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f07178")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c3e88d")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffcb6b")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#263238")).
			Foreground(lipgloss.Color("#eeffff")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#37474f")).
			Foreground(lipgloss.Color("#c792ea")),
	}
}

// NightOwlTheme returns the Night Owl color scheme theme.
// Based on https://github.com/sdras/night-owl-vscode-theme
func NightOwlTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c792ea")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#637777")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#122d42")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#82aaff")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#addb67")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d6deeb")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#637777")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d6deeb")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#011627")).
			Background(lipgloss.Color("#82aaff")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#637777")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f78c6c")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7fdbca")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ef5350")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#addb67")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffcb8b")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#011627")).
			Foreground(lipgloss.Color("#d6deeb")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#0b2942")).
			Foreground(lipgloss.Color("#c792ea")),
	}
}

// GitHubDarkTheme returns the GitHub Dark color scheme theme.
func GitHubDarkTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#58a6ff")).
			Padding(0, 1),

		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e")).
			Padding(0, 1),

		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#30363d")),

		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#58a6ff")).
			Padding(0, 1),

		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7ee787")).
			Bold(true),

		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c9d1d9")),

		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e")).
			Italic(true),

		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c9d1d9")),

		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0d1117")).
			Background(lipgloss.Color("#58a6ff")),

		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8b949e")),

		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffa657")).
			Bold(true),

		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#79c0ff")),

		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f85149")).
			Bold(true),

		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7ee787")).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d29922")),

		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#0d1117")).
			Foreground(lipgloss.Color("#c9d1d9")).
			Padding(1, 2),

		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#161b22")).
			Foreground(lipgloss.Color("#a5d6ff")),
	}
}

// =============================================================================
// Theme Registry
// =============================================================================

// AvailableThemes returns a list of all available theme names
func AvailableThemes() []string {
	return []string{
		"default",
		"light",
		"high-contrast",
		"dracula",
		"nord",
		"gruvbox-dark",
		"gruvbox-light",
		"tokyo-night",
		"tokyo-night-storm",
		"catppuccin-mocha",
		"catppuccin-latte",
		"catppuccin-frappe",
		"catppuccin-macchiato",
		"monokai",
		"solarized-dark",
		"solarized-light",
		"rose-pine",
		"rose-pine-moon",
		"rose-pine-dawn",
		"one-dark",
		"kanagawa",
		"kanagawa-wave",
		"kanagawa-dragon",
		"flexoki-dark",
		"flexoki-light",
		"everforest-dark",
		"ayu-dark",
		"material-dark",
		"night-owl",
		"github-dark",
		"github-light",
		"cobalt2",
	}
}

// GetTheme returns a theme by name
func GetTheme(name string) Theme {
	switch name {
	case "light":
		return LightTheme()
	case "high-contrast":
		return HighContrastTheme()
	case "dracula":
		return DraculaTheme()
	case "nord":
		return NordTheme()
	case "gruvbox-dark":
		return GruvboxDarkTheme()
	case "gruvbox-light":
		return GruvboxLightTheme()
	case "tokyo-night":
		return TokyoNightTheme()
	case "tokyo-night-storm":
		return TokyoNightStormTheme()
	case "catppuccin-mocha":
		return CatppuccinMochaTheme()
	case "catppuccin-latte":
		return CatppuccinLatteTheme()
	case "monokai":
		return MonokaiTheme()
	case "solarized-dark":
		return SolarizedDarkTheme()
	case "solarized-light":
		return SolarizedLightTheme()
	case "rose-pine":
		return RosePineTheme()
	case "one-dark":
		return OneDarkTheme()
	case "kanagawa":
		return KanagawaTheme()
	case "flexoki-dark":
		return FlexokiDarkTheme()
	case "flexoki-light":
		return FlexokiLightTheme()
	case "everforest-dark":
		return EverforestDarkTheme()
	case "ayu-dark":
		return AyuDarkTheme()
	case "material-dark":
		return MaterialDarkTheme()
	case "night-owl":
		return NightOwlTheme()
	case "github-dark":
		return GitHubDarkTheme()
	case "catppuccin-frappe":
		return CatppuccinFrappeTheme()
	case "catppuccin-macchiato":
		return CatppuccinMacchiatoTheme()
	case "kanagawa-wave":
		return KanagawaWaveTheme()
	case "kanagawa-dragon":
		return KanagawaDragonTheme()
	case "rose-pine-moon":
		return RosePineMoonTheme()
	case "rose-pine-dawn":
		return RosePineDawnTheme()
	case "github-light":
		return GitHubLightTheme()
	case "cobalt2":
		return Cobalt2Theme()
	default:
		return DefaultTheme()
	}
}

// =============================================================================
// Additional Themes
// =============================================================================

// CatppuccinFrappeTheme returns the Catppuccin Frappé theme
func CatppuccinFrappeTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ca9ee6")).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#737994")).
			Padding(0, 1),
		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626880")),
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#8caaee")).
			Padding(0, 1),
		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8caaee")).
			Bold(true),
		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c6d0f5")),
		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#737994")).
			Italic(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c6d0f5")),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#303446")).
			Background(lipgloss.Color("#8caaee")),
		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#737994")),
		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ca9ee6")).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#99d1db")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e78284")).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6d189")).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e5c890")),
		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#303446")).
			Foreground(lipgloss.Color("#c6d0f5")).
			Padding(1, 2),
		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#414559")).
			Foreground(lipgloss.Color("#ca9ee6")),
	}
}

// CatppuccinMacchiatoTheme returns the Catppuccin Macchiato theme
func CatppuccinMacchiatoTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c6a0f6")).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e738d")).
			Padding(0, 1),
		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5b6078")),
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#8aadf4")).
			Padding(0, 1),
		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8aadf4")).
			Bold(true),
		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cad3f5")),
		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e738d")).
			Italic(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cad3f5")),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#24273a")).
			Background(lipgloss.Color("#8aadf4")),
		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e738d")),
		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c6a0f6")).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#91d7e3")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ed8796")).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a6da95")).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#eed49f")),
		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#24273a")).
			Foreground(lipgloss.Color("#cad3f5")).
			Padding(1, 2),
		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#363a4f")).
			Foreground(lipgloss.Color("#c6a0f6")),
	}
}

// KanagawaWaveTheme returns the Kanagawa Wave theme
func KanagawaWaveTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#957fb8")).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#727169")).
			Padding(0, 1),
		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#54546d")),
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7e9cd8")).
			Padding(0, 1),
		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7e9cd8")).
			Bold(true),
		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#dcd7ba")),
		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#727169")).
			Italic(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#dcd7ba")),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1f1f28")).
			Background(lipgloss.Color("#7e9cd8")),
		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#727169")),
		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#957fb8")).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7fb4ca")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c34043")).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98bb6c")).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e6c384")),
		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#1f1f28")).
			Foreground(lipgloss.Color("#dcd7ba")).
			Padding(1, 2),
		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#363646")).
			Foreground(lipgloss.Color("#957fb8")),
	}
}

// KanagawaDragonTheme returns the Kanagawa Dragon theme
func KanagawaDragonTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8992a7")).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#625e5a")).
			Padding(0, 1),
		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#393836")),
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#8ba4b0")).
			Padding(0, 1),
		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8ba4b0")).
			Bold(true),
		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c5c9c5")),
		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#625e5a")).
			Italic(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c5c9c5")),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#181616")).
			Background(lipgloss.Color("#8ba4b0")),
		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#625e5a")),
		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8992a7")).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8ea4a2")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c4746e")).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8a9a7b")).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c4b28a")),
		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#181616")).
			Foreground(lipgloss.Color("#c5c9c5")).
			Padding(1, 2),
		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#282727")).
			Foreground(lipgloss.Color("#8992a7")),
	}
}

// RosePineMoonTheme returns the Rosé Pine Moon theme
func RosePineMoonTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c4a7e7")).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e6a86")).
			Padding(0, 1),
		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#44415a")),
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#3e8fb0")).
			Padding(0, 1),
		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3e8fb0")).
			Bold(true),
		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e0def4")),
		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e6a86")).
			Italic(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#e0def4")),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#232136")).
			Background(lipgloss.Color("#3e8fb0")),
		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6e6a86")),
		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c4a7e7")).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ccfd8")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#eb6f92")).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ccfd8")).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f6c177")),
		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#232136")).
			Foreground(lipgloss.Color("#e0def4")).
			Padding(1, 2),
		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#393552")).
			Foreground(lipgloss.Color("#c4a7e7")),
	}
}

// RosePineDawnTheme returns the Rosé Pine Dawn theme (light)
func RosePineDawnTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#907aa9")).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9893a5")).
			Padding(0, 1),
		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#dfdad9")),
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#286983")).
			Padding(0, 1),
		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#286983")).
			Bold(true),
		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#575279")),
		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9893a5")).
			Italic(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#575279")),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#faf4ed")).
			Background(lipgloss.Color("#286983")),
		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9893a5")),
		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#907aa9")).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#56949f")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#b4637a")).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#56949f")).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ea9d34")),
		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#faf4ed")).
			Foreground(lipgloss.Color("#575279")).
			Padding(1, 2),
		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#f2e9e1")).
			Foreground(lipgloss.Color("#907aa9")),
	}
}

// GitHubLightTheme returns the GitHub Light theme
func GitHubLightTheme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0969da")).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#656d76")).
			Padding(0, 1),
		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d0d7de")),
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#0969da")).
			Padding(0, 1),
		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0969da")).
			Bold(true),
		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1f2328")),
		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#656d76")).
			Italic(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1f2328")),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#0969da")),
		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#656d76")),
		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8250df")).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0969da")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cf222e")).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a7f37")).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9a6700")),
		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#f6f8fa")).
			Foreground(lipgloss.Color("#1f2328")).
			Padding(1, 2),
		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#eaeef2")).
			Foreground(lipgloss.Color("#0550ae")),
	}
}

// Cobalt2Theme returns the Cobalt2 theme
func Cobalt2Theme() Theme {
	return Theme{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffc600")).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0d3a58")).
			Padding(0, 1),
		Border: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0d3a58")),
		InputBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#ffc600")).
			Padding(0, 1),
		UserMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0088ff")).
			Bold(true),
		AssistantMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")),
		SystemMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0d3a58")).
			Italic(true),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#193549")).
			Background(lipgloss.Color("#ffc600")),
		Subtle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0d3a58")),
		Accent: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff9d00")).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#0088ff")),
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff0000")).
			Bold(true),
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3ad900")).
			Bold(true),
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffc600")),
		CodeBlock: lipgloss.NewStyle().
			Background(lipgloss.Color("#193549")).
			Foreground(lipgloss.Color("#ffffff")).
			Padding(1, 2),
		InlineCode: lipgloss.NewStyle().
			Background(lipgloss.Color("#15232d")).
			Foreground(lipgloss.Color("#ffc600")),
	}
}
