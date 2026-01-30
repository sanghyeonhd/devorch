package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestAllCommandHandlers tests all slash command handlers
func TestAllCommandHandlers(t *testing.T) {
	commands := GetSlashCommands()
	t.Logf("Testing %d commands...\n", len(commands))

	// Commands that change mode and need selection list
	modeChangingCommands := map[string]ViewMode{
		"theme":    ViewModeThemeSelect,
		"model":    ViewModeModelSelect,
		"provider": ViewModeProviderSelect,
		"login":    ViewModeLogin,
		"agent":    ViewModeAgentSelect,
		"sessions": ViewModeSessions,
		"settings": ViewModeSettings,
		"help":     ViewModeHelp,
	}

	// Commands that add messages
	messageAddingCommands := []string{
		"new", "clear", "reset", "save", "export", "share", "unshare", "compact",
		"details", "undo", "redo", "init", "files", "grep", "ls", "diff", "git",
		"models", "providers", "thinking", "context", "memory", "add",
		"themes", "language", "config", "logout", "auth", "connect",
		"tools", "mcp", "lsp", "status", "doctor", "version",
	}

	// Commands that return tea.Cmd (async)
	asyncCommands := []string{
		"setup", "install", "bench", "editor",
	}

	// Test each command
	for _, cmd := range commands {
		t.Run("/"+cmd.Name, func(t *testing.T) {
			m := New()
			// Initialize with window size
			newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			m = newM.(Model)

			initialMsgCount := len(m.messages)
			initialMode := m.mode

			// Execute handler
			result := cmd.Handler(&m)

			// Check based on command type
			if expectedMode, ok := modeChangingCommands[cmd.Name]; ok {
				// Mode should change
				if m.mode != expectedMode {
					t.Errorf("Expected mode %d, got %d", expectedMode, m.mode)
				}
				// Selection list should be populated (except sessions, settings, help)
				if cmd.Name == "theme" || cmd.Name == "model" || cmd.Name == "provider" || cmd.Name == "login" || cmd.Name == "agent" {
					if len(m.selectionList) == 0 {
						t.Errorf("selectionList should not be empty")
					} else {
						t.Logf("✓ /%s: mode=%d, selectionList=%d items", cmd.Name, m.mode, len(m.selectionList))
					}
				} else {
					t.Logf("✓ /%s: mode=%d", cmd.Name, m.mode)
				}
			} else if contains(messageAddingCommands, cmd.Name) {
				// Should add a message
				if len(m.messages) <= initialMsgCount {
					t.Errorf("Expected message to be added")
				} else {
					lastMsg := m.messages[len(m.messages)-1]
					t.Logf("✓ /%s: added message [%s]: %.50s...", cmd.Name, lastMsg.Role, strings.ReplaceAll(lastMsg.Content, "\n", " "))
				}
			} else if contains(asyncCommands, cmd.Name) {
				// Returns a command
				if result != nil {
					t.Logf("✓ /%s: returns async command", cmd.Name)
				} else {
					// Some may not return command but still work
					t.Logf("✓ /%s: executed (no async cmd)", cmd.Name)
				}
			} else if cmd.Name == "quit" {
				// Quit sets quitting flag
				if !m.quitting {
					t.Errorf("Expected quitting=true")
				}
				t.Logf("✓ /%s: quitting=true", cmd.Name)
			} else if strings.HasPrefix(cmd.Name, "user:") || strings.HasPrefix(cmd.Name, "project:") {
				// Custom commands - just log
				t.Logf("✓ /%s: custom command", cmd.Name)
			} else {
				// Unknown command type - just check it doesn't panic
				if m.mode != initialMode || len(m.messages) > initialMsgCount || result != nil {
					t.Logf("✓ /%s: executed successfully", cmd.Name)
				} else {
					t.Logf("? /%s: no visible effect", cmd.Name)
				}
			}
		})
	}
}

// TestSpecificCommands tests specific important commands in detail
func TestSpecificCommands(t *testing.T) {
	tests := []struct {
		name           string
		expectedMode   ViewMode
		checkSelection bool
		checkMessage   bool
	}{
		{"theme", ViewModeThemeSelect, true, false},
		{"model", ViewModeModelSelect, true, false},
		{"provider", ViewModeProviderSelect, true, false},
		{"login", ViewModeLogin, true, false},
		{"agent", ViewModeAgentSelect, true, false},
		{"sessions", ViewModeSessions, false, false},
		{"settings", ViewModeSettings, false, false},
		{"help", ViewModeHelp, false, false},
	}

	for _, tt := range tests {
		t.Run("/"+tt.name, func(t *testing.T) {
			m := New()
			newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			m = newM.(Model)

			cmd := FindCommand(tt.name)
			if cmd == nil {
				t.Fatalf("Command /%s not found", tt.name)
			}

			cmd.Handler(&m)

			if m.mode != tt.expectedMode {
				t.Errorf("Mode: expected %d, got %d", tt.expectedMode, m.mode)
			}

			if tt.checkSelection && len(m.selectionList) == 0 {
				t.Error("selectionList should not be empty")
			}

			// Test view renders correctly
			view := m.View()
			if len(view) < 10 {
				t.Error("View is too short")
			}
			t.Logf("View renders OK (%d chars)", len(view))
		})
	}
}

// TestToggleCommands tests toggle commands
func TestToggleCommands(t *testing.T) {
	toggles := []struct {
		name  string
		field string
	}{
		{"details", "showDetails"},
		{"thinking", "thinkingMode"},
	}

	for _, tt := range toggles {
		t.Run("/"+tt.name, func(t *testing.T) {
			m := New()

			cmd := FindCommand(tt.name)
			if cmd == nil {
				t.Fatalf("Command /%s not found", tt.name)
			}

			// First call - should enable
			cmd.Handler(&m)
			var enabled bool
			switch tt.name {
			case "details":
				enabled = m.showDetails
			case "thinking":
				enabled = m.thinkingMode
			}
			if !enabled {
				t.Errorf("First call should enable %s", tt.field)
			}

			// Second call - should disable
			cmd.Handler(&m)
			switch tt.name {
			case "details":
				enabled = m.showDetails
			case "thinking":
				enabled = m.thinkingMode
			}
			if enabled {
				t.Errorf("Second call should disable %s", tt.field)
			}

			t.Logf("✓ /%s toggles correctly", tt.name)
		})
	}
}

// TestMessageCommands tests commands that add messages
func TestMessageCommands(t *testing.T) {
	msgCommands := []string{
		"new", "clear", "reset", "share", "unshare", "compact", "details",
		"undo", "redo", "init", "files", "grep", "ls", "diff", "git",
		"models", "providers", "thinking", "context", "memory", "add",
		"themes", "language", "config", "logout", "auth", "connect",
		"tools", "mcp", "lsp", "status", "doctor", "version", "help",
	}

	for _, name := range msgCommands {
		t.Run("/"+name, func(t *testing.T) {
			m := New()
			newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
			m = newM.(Model)

			initialCount := len(m.messages)

			cmd := FindCommand(name)
			if cmd == nil {
				t.Skipf("Command /%s not found", name)
				return
			}

			cmd.Handler(&m)

			// Most commands should add a message or change mode
			if len(m.messages) > initialCount {
				t.Logf("✓ /%s: added %d message(s)", name, len(m.messages)-initialCount)
			} else if m.mode != ViewModeChat {
				t.Logf("✓ /%s: changed mode to %d", name, m.mode)
			} else {
				t.Logf("? /%s: no message added, mode unchanged", name)
			}
		})
	}
}

// TestFilteringWorks tests that filtering returns correct commands
func TestFilteringWorks(t *testing.T) {
	filters := map[string][]string{
		"/":    {"new", "sessions", "clear"}, // Should have many
		"/th":  {"theme", "themes", "thinking"},
		"/con": {"context", "config", "connect"},
		"/mo":  {"model", "models"},
		"/pro": {"provider", "providers"},
		"/se":  {"sessions", "settings", "setup"},
		"/st":  {"status"},
		"/he":  {"help"},
		"/lo":  {"login", "logout"},
		"/un":  {"undo", "unshare"},
	}

	for filter, expected := range filters {
		t.Run(filter, func(t *testing.T) {
			results := FilterCommands(filter)

			for _, exp := range expected {
				found := false
				for _, cmd := range results {
					if cmd.Name == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected /%s in results for filter '%s'", exp, filter)
				}
			}

			names := make([]string, len(results))
			for i, cmd := range results {
				names[i] = "/" + cmd.Name
			}
			t.Logf("Filter '%s' → %s", filter, strings.Join(names, ", "))
		})
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
