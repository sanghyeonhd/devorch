// Package tui provides terminal user interface components.
package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Markdown renders markdown content with styling.
type Markdown struct {
	Content string
	Theme   Theme
	Width   int
}

// Render converts markdown to styled terminal output.
func (m Markdown) Render() string {
	if m.Width == 0 {
		m.Width = 80
	}

	lines := strings.Split(m.Content, "\n")
	var result strings.Builder
	inCodeBlock := false
	codeContent := strings.Builder{}
	codeLang := ""

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Code blocks
		if strings.HasPrefix(line, "```") {
			if inCodeBlock {
				// End code block
				result.WriteString(m.renderCodeBlock(codeContent.String(), codeLang))
				result.WriteString("\n")
				codeContent.Reset()
				codeLang = ""
				inCodeBlock = false
			} else {
				// Start code block
				inCodeBlock = true
				codeLang = strings.TrimPrefix(line, "```")
			}
			continue
		}

		if inCodeBlock {
			codeContent.WriteString(line + "\n")
			continue
		}

		// Headers
		if strings.HasPrefix(line, "# ") {
			result.WriteString(m.renderHeader(1, strings.TrimPrefix(line, "# ")))
			continue
		}
		if strings.HasPrefix(line, "## ") {
			result.WriteString(m.renderHeader(2, strings.TrimPrefix(line, "## ")))
			continue
		}
		if strings.HasPrefix(line, "### ") {
			result.WriteString(m.renderHeader(3, strings.TrimPrefix(line, "### ")))
			continue
		}

		// Horizontal rule
		if line == "---" || line == "***" || line == "___" {
			result.WriteString(strings.Repeat("─", m.Width) + "\n")
			continue
		}

		// Blockquote
		if strings.HasPrefix(line, "> ") {
			result.WriteString(m.renderBlockquote(strings.TrimPrefix(line, "> ")))
			continue
		}

		// Lists
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			result.WriteString(m.renderListItem(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")))
			continue
		}

		// Numbered lists
		if matched, _ := regexp.MatchString(`^\d+\.\s`, line); matched {
			parts := strings.SplitN(line, ". ", 2)
			if len(parts) == 2 {
				result.WriteString(m.renderNumberedItem(parts[0], parts[1]))
				continue
			}
		}

		// Regular paragraph
		result.WriteString(m.renderParagraph(line))
	}

	return result.String()
}

func (m Markdown) renderHeader(level int, text string) string {
	var style lipgloss.Style

	switch level {
	case 1:
		style = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)
	case 2:
		style = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			MarginBottom(1)
	case 3:
		style = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("219")).
			MarginBottom(1)
	default:
		style = lipgloss.NewStyle().Bold(true)
	}

	return style.Render(text) + "\n"
}

func (m Markdown) renderCodeBlock(code, lang string) string {
	style := m.Theme.CodeBlock.Width(m.Width - 4)

	header := ""
	if lang != "" {
		headerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
		header = headerStyle.Render(lang) + "\n"
	}

	return header + style.Render(strings.TrimSuffix(code, "\n"))
}

func (m Markdown) renderBlockquote(text string) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("241")).
		PaddingLeft(1)

	return style.Render(text) + "\n"
}

func (m Markdown) renderListItem(text string) string {
	bullet := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Render("•")

	return "  " + bullet + " " + m.renderInline(text) + "\n"
}

func (m Markdown) renderNumberedItem(num, text string) string {
	numStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205"))

	return "  " + numStyle.Render(num+".") + " " + m.renderInline(text) + "\n"
}

func (m Markdown) renderParagraph(text string) string {
	if text == "" {
		return "\n"
	}
	return m.renderInline(text) + "\n"
}

func (m Markdown) renderInline(text string) string {
	// Bold
	boldRe := regexp.MustCompile(`\*\*(.+?)\*\*`)
	text = boldRe.ReplaceAllStringFunc(text, func(match string) string {
		inner := strings.TrimPrefix(strings.TrimSuffix(match, "**"), "**")
		return lipgloss.NewStyle().Bold(true).Render(inner)
	})

	// Italic
	italicRe := regexp.MustCompile(`\*(.+?)\*`)
	text = italicRe.ReplaceAllStringFunc(text, func(match string) string {
		inner := strings.TrimPrefix(strings.TrimSuffix(match, "*"), "*")
		return lipgloss.NewStyle().Italic(true).Render(inner)
	})

	// Inline code
	codeRe := regexp.MustCompile("`(.+?)`")
	text = codeRe.ReplaceAllStringFunc(text, func(match string) string {
		inner := strings.TrimPrefix(strings.TrimSuffix(match, "`"), "`")
		return m.Theme.InlineCode.Render(inner)
	})

	// Links [text](url)
	linkRe := regexp.MustCompile(`\[(.+?)\]\((.+?)\)`)
	text = linkRe.ReplaceAllStringFunc(text, func(match string) string {
		parts := linkRe.FindStringSubmatch(match)
		if len(parts) == 3 {
			linkStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("33")).
				Underline(true)
			return linkStyle.Render(parts[1])
		}
		return match
	})

	return text
}

// RenderToolResult formats a tool execution result for display.
func RenderToolResult(name string, success bool, output string, theme Theme) string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().Bold(true)
	statusStyle := theme.Success
	status := "✓"
	if !success {
		statusStyle = theme.Error
		status = "✗"
	}

	b.WriteString(headerStyle.Render("Tool: "+name) + " ")
	b.WriteString(statusStyle.Render(status))
	b.WriteString("\n")

	// Output
	if output != "" {
		outputStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1).
			MarginTop(1)

		b.WriteString(outputStyle.Render(output))
	}

	return b.String()
}

// RenderThinking renders a thinking indicator.
func RenderThinking(message string, theme Theme) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	return style.Render("💭 " + message)
}

// RenderError renders an error message.
func RenderError(err error, theme Theme) string {
	style := theme.Error.Copy().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(0, 1)

	return style.Render("Error: " + err.Error())
}

// RenderUsage renders token usage statistics.
func RenderUsage(input, output, total int, theme Theme) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	return style.Render("Tokens: " +
		"input=" + intToStr(input) +
		" output=" + intToStr(output) +
		" total=" + intToStr(total))
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}

	s := ""
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}

	if negative {
		s = "-" + s
	}
	return s
}
