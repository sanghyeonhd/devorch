package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// handleMultiModelSelectKey handles keys in multi-model selection mode
func (m Model) handleMultiModelSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectionIdx > 0 {
			m.selectionIdx--
		}
		return m, nil

	case "down", "j":
		if m.selectionIdx < len(m.selectedModels)-1 {
			m.selectionIdx++
		}
		return m, nil

	case " ": // Space to toggle selection
		if m.selectionIdx >= 0 && m.selectionIdx < len(m.selectedModels) {
			m.selectedModels[m.selectionIdx].Selected = !m.selectedModels[m.selectionIdx].Selected
		}
		return m, nil

	case "a": // Toggle all
		allSelected := true
		for _, model := range m.selectedModels {
			if !model.Selected {
				allSelected = false
				break
			}
		}
		// If all selected, deselect all; otherwise select all
		for i := range m.selectedModels {
			m.selectedModels[i].Selected = !allSelected
		}
		return m, nil

	case "enter":
		// Count selected models
		selectedCount := 0
		for _, model := range m.selectedModels {
			if model.Selected {
				selectedCount++
			}
		}

		if selectedCount == 0 {
			m.messages = append(m.messages, Message{
				Role:    "system",
				Content: "⚠️  Please select at least one model.",
			})
			return m, nil
		}

		// Enable multi-model mode and return to chat
		m.multiModelEnabled = true
		m.PopView()
		m.messages = append(m.messages, Message{
			Role:    "system",
			Content: fmt.Sprintf("🔀 Multi-model mode enabled with %d models.\n\nYour next message will be sent to all selected models for comparison.", selectedCount),
		})
		return m, nil

	case "esc":
		m.PopView()
		return m, nil
	}

	return m, nil
}

// handleCompareKey handles keys in comparison view mode
func (m Model) handleCompareKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1", "2": // Rate responses
		// Find which model is at the current scroll position
		// For simplicity, let's rate the first model
		if len(m.modelResponses) > 0 {
			// Get first model key
			for key, resp := range m.modelResponses {
				if !resp.InProgress && resp.Error == nil {
					rating := 1
					if msg.String() == "2" {
						rating = 2
					}
					m.rateModelResponse(key, rating)
					return m, nil
				}
			}
		}
		return m, nil

	case "esc", "q":
		// Return to chat view
		m.showCompareView = false
		m.mode = ViewModeChat
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil

	case "r": // Retry with same models
		// Get the last user message
		var lastUserMsg string
		for i := len(m.messages) - 1; i >= 0; i-- {
			if m.messages[i].Role == "user" {
				lastUserMsg = m.messages[i].Content
				break
			}
		}

		if lastUserMsg != "" {
			return m, m.sendToMultipleModels(lastUserMsg)
		}
		return m, nil
	}

	return m, nil
}
