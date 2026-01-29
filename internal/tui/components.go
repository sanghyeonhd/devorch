// Package tui provides terminal user interface components.
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Component is the interface for reusable UI components.
type Component interface {
	Render(width int) string
}

// Box renders content in a styled box.
type Box struct {
	Title   string
	Content string
	Style   lipgloss.Style
}

// Render renders the box component.
func (b Box) Render(width int) string {
	style := b.Style.Width(width)

	if b.Title != "" {
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
		content := titleStyle.Render(b.Title) + "\n" + b.Content
		return style.Render(content)
	}

	return style.Render(b.Content)
}

// Divider renders a horizontal divider.
type Divider struct {
	Char  string
	Style lipgloss.Style
}

// Render renders the divider.
func (d Divider) Render(width int) string {
	char := d.Char
	if char == "" {
		char = "─"
	}
	return d.Style.Render(strings.Repeat(char, width))
}

// Table renders a simple table.
type Table struct {
	Headers []string
	Rows    [][]string
	Style   lipgloss.Style
}

// Render renders the table.
func (t Table) Render(width int) string {
	if len(t.Headers) == 0 && len(t.Rows) == 0 {
		return ""
	}

	// Calculate column widths
	numCols := len(t.Headers)
	if numCols == 0 && len(t.Rows) > 0 {
		numCols = len(t.Rows[0])
	}

	colWidths := make([]int, numCols)
	for i, h := range t.Headers {
		if len(h) > colWidths[i] {
			colWidths[i] = len(h)
		}
	}
	for _, row := range t.Rows {
		for i, cell := range row {
			if i < numCols && len(cell) > colWidths[i] {
				colWidths[i] = len(cell)
			}
		}
	}

	var b strings.Builder

	// Render headers
	if len(t.Headers) > 0 {
		headerStyle := lipgloss.NewStyle().Bold(true)
		for i, h := range t.Headers {
			b.WriteString(headerStyle.Render(padRight(h, colWidths[i])))
			if i < len(t.Headers)-1 {
				b.WriteString(" │ ")
			}
		}
		b.WriteString("\n")

		// Separator
		for i, w := range colWidths {
			b.WriteString(strings.Repeat("─", w))
			if i < len(colWidths)-1 {
				b.WriteString("─┼─")
			}
		}
		b.WriteString("\n")
	}

	// Render rows
	for _, row := range t.Rows {
		for i, cell := range row {
			if i < numCols {
				b.WriteString(padRight(cell, colWidths[i]))
				if i < numCols-1 {
					b.WriteString(" │ ")
				}
			}
		}
		b.WriteString("\n")
	}

	return t.Style.Render(b.String())
}

// ProgressBar renders a progress bar.
type ProgressBar struct {
	Progress float64 // 0.0 to 1.0
	Width    int
	Style    lipgloss.Style
}

// Render renders the progress bar.
func (p ProgressBar) Render(width int) string {
	barWidth := p.Width
	if barWidth == 0 {
		barWidth = width - 10
	}
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int(float64(barWidth) * p.Progress)
	empty := barWidth - filled

	filledStyle := lipgloss.NewStyle().Background(lipgloss.Color("205"))
	emptyStyle := lipgloss.NewStyle().Background(lipgloss.Color("238"))

	bar := filledStyle.Render(strings.Repeat(" ", filled)) +
		emptyStyle.Render(strings.Repeat(" ", empty))

	percentage := int(p.Progress * 100)
	return p.Style.Render(bar + " " + padLeft(string(rune('0'+percentage/100))+string(rune('0'+percentage/10%10))+string(rune('0'+percentage%10))+"%", 4))
}

// Badge renders a status badge.
type Badge struct {
	Text  string
	Color string
}

// Render renders the badge.
func (b Badge) Render(width int) string {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color(b.Color)).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1)
	return style.Render(b.Text)
}

// StatusBadges for common statuses.
var (
	BadgeSuccess = Badge{Text: "SUCCESS", Color: "46"}
	BadgeError   = Badge{Text: "ERROR", Color: "196"}
	BadgeWarning = Badge{Text: "WARNING", Color: "214"}
	BadgeInfo    = Badge{Text: "INFO", Color: "33"}
	BadgePending = Badge{Text: "PENDING", Color: "241"}
)

// List renders a bullet list.
type List struct {
	Items  []string
	Bullet string
	Style  lipgloss.Style
}

// Render renders the list.
func (l List) Render(width int) string {
	bullet := l.Bullet
	if bullet == "" {
		bullet = "•"
	}

	var b strings.Builder
	for _, item := range l.Items {
		b.WriteString(bullet + " " + item + "\n")
	}

	return l.Style.Render(strings.TrimSuffix(b.String(), "\n"))
}

// Tabs renders tab headers.
type Tabs struct {
	Items       []string
	Active      int
	Style       lipgloss.Style
	ActiveStyle lipgloss.Style
}

// Render renders the tabs.
func (t Tabs) Render(width int) string {
	if t.ActiveStyle.Value() == "" {
		t.ActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("205"))
	}

	if t.Style.Value() == "" {
		t.Style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
	}

	var parts []string
	for i, item := range t.Items {
		if i == t.Active {
			parts = append(parts, t.ActiveStyle.Render(item))
		} else {
			parts = append(parts, t.Style.Render(item))
		}
	}

	return strings.Join(parts, "  ")
}

// TreeNode represents a node in a tree view.
type TreeNode struct {
	Text     string
	Children []TreeNode
	Expanded bool
}

// Tree renders a tree view.
type Tree struct {
	Root  TreeNode
	Style lipgloss.Style
}

// Render renders the tree.
func (t Tree) Render(width int) string {
	return t.Style.Render(t.renderNode(t.Root, "", true))
}

func (t Tree) renderNode(node TreeNode, prefix string, isLast bool) string {
	var b strings.Builder

	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if prefix == "" {
		connector = ""
	}

	b.WriteString(prefix + connector + node.Text + "\n")

	if node.Expanded {
		childPrefix := prefix
		if prefix != "" {
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
		}

		for i, child := range node.Children {
			b.WriteString(t.renderNode(child, childPrefix, i == len(node.Children)-1))
		}
	}

	return b.String()
}

// Helper functions

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func padLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// WrapText wraps text to the specified width.
func WrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for _, word := range words {
		wordLen := len(word)
		if lineLen+wordLen+1 > width && lineLen > 0 {
			result.WriteString("\n")
			lineLen = 0
		}
		if lineLen > 0 {
			result.WriteString(" ")
			lineLen++
		}
		result.WriteString(word)
		lineLen += wordLen
	}

	return result.String()
}
