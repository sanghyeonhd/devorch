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
		"monokai",
		"solarized-dark",
		"solarized-light",
		"rose-pine",
		"one-dark",
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
	default:
		return DefaultTheme()
	}
}
