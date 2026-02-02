package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Theme contains color and style definitions for CLI output
type Theme struct {
	Primary   lipgloss.Style
	Secondary lipgloss.Style
	Accent    lipgloss.Style
	Success   lipgloss.Style
	Warning   lipgloss.Style
	Error     lipgloss.Style
	Muted     lipgloss.Style
	Border    lipgloss.Style
	Title     lipgloss.Style
	Subtitle  lipgloss.Style
	Code      lipgloss.Style
	Highlight lipgloss.Style
}

// DefaultTheme returns the default CLI theme
func DefaultTheme() *Theme {
	return &Theme{
		Primary:   lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")),
		Secondary: lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B")),
		Accent:    lipgloss.NewStyle().Foreground(lipgloss.Color("#06B6D4")),
		Success:   lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")),
		Warning:   lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")),
		Error:     lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")),
		Muted:     lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")),
		Border:    lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")),
		Title:     lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true),
		Subtitle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB")),
		Code:      lipgloss.NewStyle().Foreground(lipgloss.Color("#F97316")).Background(lipgloss.Color("#1F2937")).Padding(0, 1),
		Highlight: lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Bold(true),
	}
}

// AvailableThemes returns a list of available theme names
func AvailableThemes() []string {
	return []string{
		"default",
		"dark",
		"light",
		"monokai",
		"solarized",
	}
}

// GetTheme returns a theme by name
func GetTheme(name string) *Theme {
	switch name {
	case "light":
		return lightTheme()
	case "monokai":
		return monokaiTheme()
	case "solarized":
		return solarizedTheme()
	default:
		return DefaultTheme()
	}
}

func lightTheme() *Theme {
	return &Theme{
		Primary:   lipgloss.NewStyle().Foreground(lipgloss.Color("#6366F1")),
		Secondary: lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B")),
		Accent:    lipgloss.NewStyle().Foreground(lipgloss.Color("#0891B2")),
		Success:   lipgloss.NewStyle().Foreground(lipgloss.Color("#059669")),
		Warning:   lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706")),
		Error:     lipgloss.NewStyle().Foreground(lipgloss.Color("#DC2626")),
		Muted:     lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")),
		Border:    lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB")),
		Title:     lipgloss.NewStyle().Foreground(lipgloss.Color("#111827")).Bold(true),
		Subtitle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#374151")),
		Code:      lipgloss.NewStyle().Foreground(lipgloss.Color("#DB2777")).Background(lipgloss.Color("#F9FAFB")).Padding(0, 1),
		Highlight: lipgloss.NewStyle().Foreground(lipgloss.Color("#7C2D12")).Bold(true),
	}
}

func monokaiTheme() *Theme {
	return &Theme{
		Primary:   lipgloss.NewStyle().Foreground(lipgloss.Color("#F92672")),
		Secondary: lipgloss.NewStyle().Foreground(lipgloss.Color("#75715E")),
		Accent:    lipgloss.NewStyle().Foreground(lipgloss.Color("#66D9EF")),
		Success:   lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E22E")),
		Warning:   lipgloss.NewStyle().Foreground(lipgloss.Color("#FD971F")),
		Error:     lipgloss.NewStyle().Foreground(lipgloss.Color("#F92672")),
		Muted:     lipgloss.NewStyle().Foreground(lipgloss.Color("#75715E")),
		Border:    lipgloss.NewStyle().Foreground(lipgloss.Color("#49483E")),
		Title:     lipgloss.NewStyle().Foreground(lipgloss.Color("#F8F8F2")).Bold(true),
		Subtitle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#CFCFC2")),
		Code:      lipgloss.NewStyle().Foreground(lipgloss.Color("#AE81FF")).Background(lipgloss.Color("#272822")).Padding(0, 1),
		Highlight: lipgloss.NewStyle().Foreground(lipgloss.Color("#E6DB74")).Bold(true),
	}
}

func solarizedTheme() *Theme {
	return &Theme{
		Primary:   lipgloss.NewStyle().Foreground(lipgloss.Color("#268BD2")),
		Secondary: lipgloss.NewStyle().Foreground(lipgloss.Color("#93A1A1")),
		Accent:    lipgloss.NewStyle().Foreground(lipgloss.Color("#2AA198")),
		Success:   lipgloss.NewStyle().Foreground(lipgloss.Color("#859900")),
		Warning:   lipgloss.NewStyle().Foreground(lipgloss.Color("#B58900")),
		Error:     lipgloss.NewStyle().Foreground(lipgloss.Color("#DC322F")),
		Muted:     lipgloss.NewStyle().Foreground(lipgloss.Color("#657B83")),
		Border:    lipgloss.NewStyle().Foreground(lipgloss.Color("#073642")),
		Title:     lipgloss.NewStyle().Foreground(lipgloss.Color("#EEE8D5")).Bold(true),
		Subtitle:  lipgloss.NewStyle().Foreground(lipgloss.Color("#93A1A1")),
		Code:      lipgloss.NewStyle().Foreground(lipgloss.Color("#D33682")).Background(lipgloss.Color("#002B36")).Padding(0, 1),
		Highlight: lipgloss.NewStyle().Foreground(lipgloss.Color("#CB4B16")).Bold(true),
	}
}

// PrintError prints an error with theme styling
func PrintError(err error, theme *Theme) {
	if theme == nil {
		theme = DefaultTheme()
	}
	fmt.Printf("%s %s\n", theme.Error.Render("Error:"), err.Error())
}
