package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestAutocompleteScrolling tests the autocomplete scrolling fix
func TestAutocompleteScrolling(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 40

	// Simulate typing "/"
	m.input.SetValue("/")
	m.filteredCommands = FilterCommands("/")
	m.showCommandAutocomplete = true
	m.commandSelectedIdx = 0

	t.Logf("Total commands: %d", len(m.filteredCommands))

	if len(m.filteredCommands) < 10 {
		t.Fatalf("Expected more than 10 commands, got %d", len(m.filteredCommands))
	}

	// Test scrolling down past 10 items
	for i := 0; i < 15; i++ {
		if m.commandSelectedIdx < len(m.filteredCommands)-1 {
			m.commandSelectedIdx++
		}
	}

	t.Logf("Selected index after 15 down presses: %d", m.commandSelectedIdx)

	// Render and check that selection is visible
	rendered := m.renderCommandAutocomplete()
	t.Logf("Rendered autocomplete:\n%s", rendered)

	// Should contain scroll indicator
	if !strings.Contains(rendered, "more below") && !strings.Contains(rendered, "more above") {
		t.Logf("Warning: No scroll indicator found")
	}

	// Should contain the selection pointer
	if !strings.Contains(rendered, "▶") {
		t.Error("Selection pointer ▶ not found in rendered output")
	}

	// The selected command should be visible
	selectedCmd := m.filteredCommands[m.commandSelectedIdx]
	if !strings.Contains(rendered, selectedCmd.Name) {
		t.Errorf("Selected command '%s' not visible in rendered output", selectedCmd.Name)
	}
}

// TestThemeSelection tests the theme command
func TestThemeSelection(t *testing.T) {
	m := New()

	// Execute /theme command
	cmd := cmdTheme(&m)
	if cmd != nil {
		t.Log("cmdTheme returned a command")
	}

	// Check mode changed
	if m.mode != ViewModeThemeSelect {
		t.Errorf("Expected mode ViewModeThemeSelect (%d), got %d", ViewModeThemeSelect, m.mode)
	}

	// Check selection list is populated
	if len(m.selectionList) == 0 {
		t.Error("selectionList is empty after /theme command")
	} else {
		t.Logf("Theme selection list: %v", m.selectionList)
	}

	// Check selection title
	if m.selectionTitle == "" {
		t.Error("selectionTitle is empty after /theme command")
	} else {
		t.Logf("Selection title: %s", m.selectionTitle)
	}
}

// TestNewCommands tests the newly added commands
func TestNewCommands(t *testing.T) {
	commands := GetSlashCommands()

	// Check for new commands
	newCmds := []string{"unshare", "details", "editor", "thinking", "connect"}

	for _, name := range newCmds {
		found := false
		for _, cmd := range commands {
			if cmd.Name == name {
				found = true
				t.Logf("✓ Found command: /%s - %s", cmd.Name, cmd.Description)
				break
			}
		}
		if !found {
			t.Errorf("✗ Command /%s not found", name)
		}
	}
}

// TestDetailsCommand tests the /details toggle
func TestDetailsCommand(t *testing.T) {
	m := New()

	// Initial state
	if m.showDetails {
		t.Error("showDetails should be false initially")
	}

	// Execute /details
	cmdDetails(&m)

	if !m.showDetails {
		t.Error("showDetails should be true after /details")
	}

	// Toggle again
	cmdDetails(&m)

	if m.showDetails {
		t.Error("showDetails should be false after second /details")
	}
}

// TestThinkingCommand tests the /thinking toggle
func TestThinkingCommand(t *testing.T) {
	m := New()

	// Initial state
	if m.thinkingMode {
		t.Error("thinkingMode should be false initially")
	}

	// Execute /thinking
	cmdThinking(&m)

	if !m.thinkingMode {
		t.Error("thinkingMode should be true after /thinking")
	}
}

// TestFilterCommands tests command filtering
func TestFilterCommands(t *testing.T) {
	// Test various filters
	tests := []struct {
		filter   string
		expected []string
	}{
		{"/th", []string{"theme", "themes", "thinking"}},
		{"/con", []string{"connect", "context", "config"}},
		{"/de", []string{"details"}},
		{"/un", []string{"undo", "unshare"}},
	}

	for _, tt := range tests {
		filtered := FilterCommands(tt.filter)
		t.Logf("Filter '%s' returned %d commands:", tt.filter, len(filtered))
		for _, cmd := range filtered {
			t.Logf("  - /%s", cmd.Name)
		}

		for _, exp := range tt.expected {
			found := false
			for _, cmd := range filtered {
				if cmd.Name == exp {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected /%s in filter results for '%s'", exp, tt.filter)
			}
		}
	}
}

// TestCommandCount verifies we have the expected number of commands
func TestCommandCount(t *testing.T) {
	commands := GetSlashCommands()
	t.Logf("Total commands: %d", len(commands))

	// Count by category
	categories := make(map[string]int)
	for _, cmd := range commands {
		categories[cmd.Category]++
	}

	for cat, count := range categories {
		t.Logf("  %s: %d commands", cat, count)
	}

	// We should have at least 35 commands now
	if len(commands) < 35 {
		t.Errorf("Expected at least 35 commands, got %d", len(commands))
	}
}

// TestHandleCommandAutocompleteKey tests keyboard navigation
func TestHandleCommandAutocompleteKey(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 40
	m.input.SetValue("/")
	m.filteredCommands = FilterCommands("/")
	m.showCommandAutocomplete = true
	m.commandSelectedIdx = 0

	// Test down arrow
	newModel, _ := m.handleCommandAutocompleteKey(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)

	if m.commandSelectedIdx != 1 {
		t.Errorf("Expected index 1 after down, got %d", m.commandSelectedIdx)
	}

	// Test up arrow
	newModel, _ = m.handleCommandAutocompleteKey(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(Model)

	if m.commandSelectedIdx != 0 {
		t.Errorf("Expected index 0 after up, got %d", m.commandSelectedIdx)
	}

	// Test escape
	newModel, _ = m.handleCommandAutocompleteKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(Model)

	if m.showCommandAutocomplete {
		t.Error("Autocomplete should be hidden after escape")
	}
}

// TestRenderCommandAutocomplete tests the rendering function directly
func TestRenderCommandAutocomplete(t *testing.T) {
	m := New()
	m.width = 100
	m.height = 40
	m.filteredCommands = FilterCommands("/")
	m.showCommandAutocomplete = true

	// Test with index 0
	m.commandSelectedIdx = 0
	output := m.renderCommandAutocomplete()
	t.Logf("Render at index 0:\n%s", output)

	if !strings.Contains(output, "▶") {
		t.Error("Missing selection pointer at index 0")
	}

	// Test with index 15 (should scroll)
	m.commandSelectedIdx = 15
	output = m.renderCommandAutocomplete()
	t.Logf("\nRender at index 15:\n%s", output)

	if !strings.Contains(output, "▶") {
		t.Error("Missing selection pointer at index 15")
	}

	// Should show "more above" indicator when scrolled down
	if !strings.Contains(output, "more above") {
		t.Error("Should show 'more above' when scrolled past first 10")
	}
}

// TestAvailableThemes verifies themes are available
func TestAvailableThemes(t *testing.T) {
	themes := AvailableThemes()
	t.Logf("Available themes: %v", themes)

	if len(themes) == 0 {
		t.Error("No themes available")
	}

	// Check for default theme
	hasDefault := false
	for _, th := range themes {
		if th == "default" {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		t.Error("Missing 'default' theme")
	}
}

// TestFullThemeFlow simulates the complete /theme flow
func TestFullThemeFlow(t *testing.T) {
	m := New()
	m.width = 120
	m.height = 40

	// Step 1: Type "/"
	m.input.SetValue("/")

	// Step 2: Process the input to trigger autocomplete
	input := m.input.Value()
	if strings.HasPrefix(input, "/") {
		m.showCommandAutocomplete = true
		m.filteredCommands = FilterCommands(input)
	}

	t.Logf("After '/': showCommandAutocomplete=%v, commands=%d", m.showCommandAutocomplete, len(m.filteredCommands))

	// Step 3: Type "theme"
	m.input.SetValue("/theme")
	m.filteredCommands = FilterCommands("/theme")
	t.Logf("After '/theme': filteredCommands=%d", len(m.filteredCommands))

	for _, cmd := range m.filteredCommands {
		t.Logf("  - /%s", cmd.Name)
	}

	// Step 4: Press Enter (execute the theme command)
	if len(m.filteredCommands) > 0 {
		selected := m.filteredCommands[m.commandSelectedIdx]
		t.Logf("Selected command: /%s", selected.Name)

		// Execute handler
		m.input.SetValue("")
		m.showCommandAutocomplete = false
		selected.Handler(&m)

		t.Logf("After handler: mode=%d (expected %d for ViewModeThemeSelect)", m.mode, ViewModeThemeSelect)
		t.Logf("  selectionList=%d items", len(m.selectionList))
		t.Logf("  selectionTitle=%s", m.selectionTitle)
	}

	// Verify results
	if m.mode != ViewModeThemeSelect {
		t.Errorf("Expected ViewModeThemeSelect (%d), got %d", ViewModeThemeSelect, m.mode)
	}

	if len(m.selectionList) == 0 {
		t.Error("selectionList should not be empty")
	}

	if m.selectionTitle != "Select Theme" {
		t.Errorf("Expected title 'Select Theme', got '%s'", m.selectionTitle)
	}
}

// TestKeyEventFlow tests the actual Update flow with key events
func TestKeyEventFlow(t *testing.T) {
	m := New()

	// Simulate window size message to set ready=true
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = newM.(Model)

	t.Logf("After WindowSize: ready=%v, width=%d, height=%d", m.ready, m.width, m.height)

	// Type "/" character
	m.input.SetValue("/")

	// Manually trigger autocomplete check (normally done in Update after key press)
	input := m.input.Value()
	if strings.HasPrefix(input, "/") {
		m.showCommandAutocomplete = true
		m.filteredCommands = FilterCommands(input)
	}

	t.Logf("State after '/': autocomplete=%v, commands=%d, mode=%d",
		m.showCommandAutocomplete, len(m.filteredCommands), m.mode)

	// Navigate down with arrow keys
	for i := 0; i < 5; i++ {
		newM, _ := m.handleCommandAutocompleteKey(tea.KeyMsg{Type: tea.KeyDown})
		m = newM.(Model)
	}
	t.Logf("After 5 down: selectedIdx=%d", m.commandSelectedIdx)

	// Find theme command
	themeIdx := -1
	for i, cmd := range m.filteredCommands {
		if cmd.Name == "theme" {
			themeIdx = i
			break
		}
	}
	t.Logf("Theme command at index: %d", themeIdx)

	// Navigate to theme
	if themeIdx >= 0 {
		m.commandSelectedIdx = themeIdx
	}

	// Press Enter
	t.Logf("Before Enter: mode=%d, showCommandAutocomplete=%v", m.mode, m.showCommandAutocomplete)
	newM, _ = m.handleCommandAutocompleteKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(Model)

	t.Logf("After Enter: mode=%d, showCommandAutocomplete=%v, selectionList=%d",
		m.mode, m.showCommandAutocomplete, len(m.selectionList))

	if m.mode != ViewModeThemeSelect {
		t.Errorf("Expected ViewModeThemeSelect, got mode=%d", m.mode)
	}

	if len(m.selectionList) == 0 {
		t.Error("Theme list should be populated")
	}

	// Now test View() - it should show selection list, not chat
	view := m.View()
	t.Logf("View length: %d chars", len(view))

	// Selection list should contain theme names
	if !strings.Contains(view, "Select Theme") && !strings.Contains(view, "default") {
		t.Error("View should show theme selection, not chat view")
		t.Logf("View output:\n%s", view[:min(500, len(view))])
	} else {
		t.Log("✓ View correctly shows theme selection!")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestMain(m *testing.M) {
	fmt.Println("=== Running TUI Command Tests ===")
	m.Run()
}
