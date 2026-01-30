// Package editor provides external editor integration
package editor

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Editor represents an external editor
type Editor struct {
	Name       string
	Command    string
	Args       []string
	Supported  bool
	Detected   bool
	DetectFunc func() bool
}

// Manager manages external editor integration
type Manager struct {
	editors     []*Editor
	defaultEdit string
	tempDir     string
}

// NewManager creates a new editor manager
func NewManager() *Manager {
	m := &Manager{
		tempDir: filepath.Join(os.TempDir(), "devorch-editor"),
	}
	os.MkdirAll(m.tempDir, 0755)

	m.initEditors()
	m.detectEditors()

	return m
}

// initEditors initializes supported editors
func (m *Manager) initEditors() {
	m.editors = []*Editor{
		{
			Name:      "VS Code",
			Command:   "code",
			Args:      []string{"--wait"},
			Supported: true,
			DetectFunc: func() bool {
				_, err := exec.LookPath("code")
				return err == nil
			},
		},
		{
			Name:      "Cursor",
			Command:   "cursor",
			Args:      []string{"--wait"},
			Supported: true,
			DetectFunc: func() bool {
				_, err := exec.LookPath("cursor")
				return err == nil
			},
		},
		{
			Name:      "Neovim",
			Command:   "nvim",
			Args:      []string{},
			Supported: true,
			DetectFunc: func() bool {
				_, err := exec.LookPath("nvim")
				return err == nil
			},
		},
		{
			Name:      "Vim",
			Command:   "vim",
			Args:      []string{},
			Supported: true,
			DetectFunc: func() bool {
				_, err := exec.LookPath("vim")
				return err == nil
			},
		},
		{
			Name:      "Nano",
			Command:   "nano",
			Args:      []string{},
			Supported: true,
			DetectFunc: func() bool {
				_, err := exec.LookPath("nano")
				return err == nil
			},
		},
		{
			Name:      "Emacs",
			Command:   "emacs",
			Args:      []string{},
			Supported: true,
			DetectFunc: func() bool {
				_, err := exec.LookPath("emacs")
				return err == nil
			},
		},
		{
			Name:      "Sublime Text",
			Command:   "subl",
			Args:      []string{"--wait"},
			Supported: true,
			DetectFunc: func() bool {
				_, err := exec.LookPath("subl")
				return err == nil
			},
		},
		{
			Name:      "IntelliJ IDEA",
			Command:   "idea",
			Args:      []string{"--wait"},
			Supported: true,
			DetectFunc: func() bool {
				_, err := exec.LookPath("idea")
				return err == nil
			},
		},
	}
}

// detectEditors detects available editors
func (m *Manager) detectEditors() {
	// Check $EDITOR environment variable first
	m.defaultEdit = os.Getenv("EDITOR")
	if m.defaultEdit == "" {
		m.defaultEdit = os.Getenv("VISUAL")
	}

	// Detect each editor
	for _, e := range m.editors {
		if e.DetectFunc != nil {
			e.Detected = e.DetectFunc()
		}
	}
}

// GetAvailableEditors returns list of detected editors
func (m *Manager) GetAvailableEditors() []*Editor {
	var available []*Editor
	for _, e := range m.editors {
		if e.Detected {
			available = append(available, e)
		}
	}
	return available
}

// GetDefaultEditor returns the default editor
func (m *Manager) GetDefaultEditor() *Editor {
	// Check $EDITOR first
	if m.defaultEdit != "" {
		for _, e := range m.editors {
			if e.Command == m.defaultEdit || strings.Contains(m.defaultEdit, e.Command) {
				return e
			}
		}
		// Return custom editor
		return &Editor{
			Name:      "Custom ($EDITOR)",
			Command:   m.defaultEdit,
			Args:      []string{},
			Supported: true,
			Detected:  true,
		}
	}

	// Return first detected editor
	for _, e := range m.editors {
		if e.Detected {
			return e
		}
	}

	// Fallback to vim
	return &Editor{
		Name:      "vim",
		Command:   "vim",
		Args:      []string{},
		Supported: true,
		Detected:  false,
	}
}

// EditContent opens content in an external editor and returns the modified content
func (m *Manager) EditContent(ctx context.Context, content string, extension string) (string, error) {
	editor := m.GetDefaultEditor()
	return m.EditContentWithEditor(ctx, content, extension, editor)
}

// EditContentWithEditor opens content in a specific editor
func (m *Manager) EditContentWithEditor(ctx context.Context, content string, extension string, editor *Editor) (string, error) {
	if extension == "" {
		extension = ".md"
	}
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}

	// Create temp file
	tmpfile, err := os.CreateTemp(m.tempDir, fmt.Sprintf("devorch-*%s", extension))
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpfile.Name()
	defer os.Remove(tmpPath)

	// Write content
	if _, err := tmpfile.WriteString(content); err != nil {
		tmpfile.Close()
		return "", fmt.Errorf("failed to write content: %w", err)
	}
	tmpfile.Close()

	// Get file modification time
	info, err := os.Stat(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}
	modTime := info.ModTime()

	// Build command
	args := append(editor.Args, tmpPath)
	cmd := exec.CommandContext(ctx, editor.Command, args...)

	// For terminal editors, connect to stdin/stdout/stderr
	if isTerminalEditor(editor.Command) {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	// Run editor
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}

	// Check if file was modified
	info, err = os.Stat(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file after edit: %w", err)
	}
	if !info.ModTime().After(modTime) {
		// File wasn't modified
		return content, nil
	}

	// Read modified content
	modified, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read modified content: %w", err)
	}

	return string(modified), nil
}

// EditFile opens a file in an external editor
func (m *Manager) EditFile(ctx context.Context, filePath string) error {
	editor := m.GetDefaultEditor()
	return m.EditFileWithEditor(ctx, filePath, editor)
}

// EditFileWithEditor opens a file in a specific editor
func (m *Manager) EditFileWithEditor(ctx context.Context, filePath string, editor *Editor) error {
	args := append(editor.Args, filePath)
	cmd := exec.CommandContext(ctx, editor.Command, args...)

	if isTerminalEditor(editor.Command) {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// OpenInEditor opens multiple files in an editor (non-blocking)
func (m *Manager) OpenInEditor(files ...string) error {
	editor := m.GetDefaultEditor()
	args := append(editor.Args, files...)

	// Remove --wait flag for non-blocking open
	for i, arg := range args {
		if arg == "--wait" || arg == "-w" {
			args = append(args[:i], args[i+1:]...)
			break
		}
	}

	cmd := exec.Command(editor.Command, args...)
	return cmd.Start()
}

// OpenFolder opens a folder in an editor
func (m *Manager) OpenFolder(folderPath string) error {
	editor := m.GetDefaultEditor()

	// VS Code and Cursor support folder opening
	if editor.Command == "code" || editor.Command == "cursor" {
		cmd := exec.Command(editor.Command, folderPath)
		return cmd.Start()
	}

	return fmt.Errorf("folder opening not supported for %s", editor.Name)
}

// isTerminalEditor returns true if the editor runs in the terminal
func isTerminalEditor(cmd string) bool {
	terminalEditors := []string{"vim", "nvim", "vi", "nano", "emacs", "pico", "micro"}
	for _, e := range terminalEditors {
		if cmd == e || strings.HasSuffix(cmd, "/"+e) {
			return true
		}
	}
	return false
}

// ComposePrompt opens an editor to compose a multi-line prompt
func (m *Manager) ComposePrompt(ctx context.Context, initialContent string) (string, error) {
	header := `# DevOrch Prompt Editor
# Write your prompt below. Lines starting with # are comments and will be ignored.
# Save and close the editor to submit your prompt.
# Close without saving to cancel.
# ─────────────────────────────────────────────────────────────────

`

	content := header + initialContent

	result, err := m.EditContent(ctx, content, ".md")
	if err != nil {
		return "", err
	}

	// Remove header comments
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(result))
	inHeader := true
	for scanner.Scan() {
		line := scanner.Text()
		if inHeader && (strings.HasPrefix(line, "#") || line == "") {
			continue
		}
		if strings.HasPrefix(line, "# ─") {
			inHeader = false
			continue
		}
		inHeader = false
		lines = append(lines, line)
	}

	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}

// ExportSession exports a session to a file and optionally opens in editor
func (m *Manager) ExportSession(sessionContent string, format string, openEditor bool) (string, error) {
	var ext string
	switch format {
	case "markdown", "md":
		ext = ".md"
	case "json":
		ext = ".json"
	case "txt", "text":
		ext = ".txt"
	default:
		ext = ".md"
	}

	// Create export file
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("devorch-session-%s%s", timestamp, ext)

	// Get exports directory
	var exportDir string
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		homeDir, _ := os.UserHomeDir()
		exportDir = filepath.Join(homeDir, ".config", "devorch", "exports")
	} else {
		exportDir = filepath.Join(os.TempDir(), "devorch-exports")
	}
	os.MkdirAll(exportDir, 0755)

	exportPath := filepath.Join(exportDir, filename)

	if err := os.WriteFile(exportPath, []byte(sessionContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write export file: %w", err)
	}

	if openEditor {
		if err := m.OpenInEditor(exportPath); err != nil {
			return exportPath, fmt.Errorf("exported but failed to open: %w", err)
		}
	}

	return exportPath, nil
}

// Status returns the editor integration status
type Status struct {
	DefaultEditor    *Editor
	AvailableEditors []*Editor
	EnvEditor        string
	EnvVisual        string
}

// GetStatus returns the current editor integration status
func (m *Manager) GetStatus() Status {
	return Status{
		DefaultEditor:    m.GetDefaultEditor(),
		AvailableEditors: m.GetAvailableEditors(),
		EnvEditor:        os.Getenv("EDITOR"),
		EnvVisual:        os.Getenv("VISUAL"),
	}
}
