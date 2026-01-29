// Package memory provides OPENCODE.md style memory file management
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultFileName is the default memory file name
	DefaultFileName = "DEVORCH.md"
	// DeprecatedFileName is the legacy file name
	DeprecatedFileName = "OPENCODE.md"
)

// Memory represents the project memory file
type Memory struct {
	Path     string
	Content  string
	Sections map[string]string
}

// DefaultTemplate is the default DEVORCH.md template
const DefaultTemplate = `# DEVORCH.md

> This file is used by DevOrch to understand your project.
> Add context about your codebase, architecture decisions, and preferences.
> DevOrch will read this file at the start of each session.

## Project Overview

[Describe your project here]

## Architecture

[Describe your architecture here]

## Tech Stack

- **Language**: 
- **Framework**: 
- **Database**: 
- **Other tools**: 

## Code Conventions

- [Add your coding conventions here]

## Important Files

- [List important files and their purposes]

## Common Commands

` + "```bash" + `
# Build
# Test
# Run
` + "```" + `

## Notes

[Add any other important notes here]

---
*Last updated: %s*
`

// Load loads or creates the memory file
func Load(projectDir string) (*Memory, error) {
	// Try DEVORCH.md first
	path := filepath.Join(projectDir, DefaultFileName)
	content, err := os.ReadFile(path)
	if err == nil {
		return parseMemory(path, string(content))
	}

	// Try legacy OPENCODE.md
	legacyPath := filepath.Join(projectDir, DeprecatedFileName)
	content, err = os.ReadFile(legacyPath)
	if err == nil {
		return parseMemory(legacyPath, string(content))
	}

	// Return empty memory (file doesn't exist)
	return &Memory{
		Path:     path,
		Content:  "",
		Sections: make(map[string]string),
	}, nil
}

// Exists checks if a memory file exists
func Exists(projectDir string) bool {
	path := filepath.Join(projectDir, DefaultFileName)
	if _, err := os.Stat(path); err == nil {
		return true
	}

	legacyPath := filepath.Join(projectDir, DeprecatedFileName)
	if _, err := os.Stat(legacyPath); err == nil {
		return true
	}

	return false
}

// Create creates a new memory file with the default template
func Create(projectDir string) (*Memory, error) {
	path := filepath.Join(projectDir, DefaultFileName)

	content := fmt.Sprintf(DefaultTemplate, time.Now().Format("2006-01-02 15:04:05"))

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return nil, err
	}

	return parseMemory(path, content)
}

// Save saves the memory file
func (m *Memory) Save() error {
	return os.WriteFile(m.Path, []byte(m.Content), 0644)
}

// GetSection returns a specific section content
func (m *Memory) GetSection(name string) string {
	return m.Sections[strings.ToLower(name)]
}

// SetSection sets a section content
func (m *Memory) SetSection(name, content string) {
	m.Sections[strings.ToLower(name)] = content
	m.rebuildContent()
}

// AddNote adds a note to the Notes section
func (m *Memory) AddNote(note string) {
	notes := m.GetSection("notes")
	if notes == "" {
		notes = "[Add any other important notes here]\n"
	}

	timestamp := time.Now().Format("2006-01-02 15:04")
	notes = strings.TrimSuffix(notes, "\n")
	notes += fmt.Sprintf("\n- [%s] %s\n", timestamp, note)

	m.SetSection("notes", notes)
}

// UpdateTimestamp updates the last updated timestamp
func (m *Memory) UpdateTimestamp() {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	m.Content = strings.TrimSpace(m.Content)

	// Find and replace the timestamp line
	lines := strings.Split(m.Content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "*Last updated:") {
			lines[i] = fmt.Sprintf("*Last updated: %s*", timestamp)
			m.Content = strings.Join(lines, "\n")
			return
		}
	}

	// Add timestamp if not found
	m.Content += fmt.Sprintf("\n\n---\n*Last updated: %s*", timestamp)
}

// GetPromptContext returns the content formatted for inclusion in prompts
func (m *Memory) GetPromptContext() string {
	if m.Content == "" {
		return ""
	}

	return fmt.Sprintf(`## Project Context (from %s)

%s

---
`, filepath.Base(m.Path), m.Content)
}

func parseMemory(path, content string) (*Memory, error) {
	m := &Memory{
		Path:     path,
		Content:  content,
		Sections: make(map[string]string),
	}

	// Parse sections (## headings)
	lines := strings.Split(content, "\n")
	var currentSection string
	var sectionContent strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			if currentSection != "" {
				m.Sections[strings.ToLower(currentSection)] = strings.TrimSpace(sectionContent.String())
			}

			// Start new section
			currentSection = strings.TrimPrefix(line, "## ")
			sectionContent.Reset()
		} else if currentSection != "" {
			sectionContent.WriteString(line)
			sectionContent.WriteString("\n")
		}
	}

	// Save last section
	if currentSection != "" {
		m.Sections[strings.ToLower(currentSection)] = strings.TrimSpace(sectionContent.String())
	}

	return m, nil
}

func (m *Memory) rebuildContent() {
	var sb strings.Builder

	// Write header
	sb.WriteString("# DEVORCH.md\n\n")
	sb.WriteString("> This file is used by DevOrch to understand your project.\n")
	sb.WriteString("> Add context about your codebase, architecture decisions, and preferences.\n")
	sb.WriteString("> DevOrch will read this file at the start of each session.\n\n")

	// Write sections in order
	sectionOrder := []string{
		"project overview",
		"architecture",
		"tech stack",
		"code conventions",
		"important files",
		"common commands",
		"notes",
	}

	for _, section := range sectionOrder {
		if content, ok := m.Sections[section]; ok {
			sb.WriteString(fmt.Sprintf("## %s\n\n", strings.Title(section)))
			sb.WriteString(content)
			sb.WriteString("\n\n")
		}
	}

	// Write any additional sections
	for section, content := range m.Sections {
		found := false
		for _, s := range sectionOrder {
			if s == section {
				found = true
				break
			}
		}
		if !found {
			sb.WriteString(fmt.Sprintf("## %s\n\n", strings.Title(section)))
			sb.WriteString(content)
			sb.WriteString("\n\n")
		}
	}

	// Add timestamp
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("*Last updated: %s*\n", time.Now().Format("2006-01-02 15:04:05")))

	m.Content = sb.String()
}

// Migrate migrates from OPENCODE.md to DEVORCH.md
func Migrate(projectDir string) error {
	oldPath := filepath.Join(projectDir, DeprecatedFileName)
	newPath := filepath.Join(projectDir, DefaultFileName)

	// Check if old file exists
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return nil // Nothing to migrate
	}

	// Check if new file already exists
	if _, err := os.Stat(newPath); err == nil {
		return nil // Already migrated
	}

	// Read old file
	content, err := os.ReadFile(oldPath)
	if err != nil {
		return err
	}

	// Replace OPENCODE with DEVORCH in content
	newContent := strings.ReplaceAll(string(content), "OPENCODE.md", "DEVORCH.md")
	newContent = strings.ReplaceAll(newContent, "OpenCode", "DevOrch")
	newContent = strings.ReplaceAll(newContent, "opencode", "devorch")

	// Write new file
	if err := os.WriteFile(newPath, []byte(newContent), 0644); err != nil {
		return err
	}

	return nil
}
