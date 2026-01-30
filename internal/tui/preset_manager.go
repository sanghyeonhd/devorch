package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ModelPreset represents a saved model combination
type ModelPreset struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Models      []ModelSelection `json:"models"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	UsageCount  int              `json:"usage_count"`
}

// PresetManager manages model presets
type PresetManager struct {
	presetsDir string
	presets    map[string]*ModelPreset
}

// NewPresetManager creates a new preset manager
func NewPresetManager() (*PresetManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	presetsDir := filepath.Join(homeDir, ".config", "devorch", "presets")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(presetsDir, 0755); err != nil {
		return nil, err
	}

	pm := &PresetManager{
		presetsDir: presetsDir,
		presets:    make(map[string]*ModelPreset),
	}

	// Load existing presets
	if err := pm.LoadAll(); err != nil {
		// Non-fatal error, just log it
		fmt.Printf("Warning: Could not load presets: %v\n", err)
	}

	return pm, nil
}

// SavePreset saves a model preset
func (pm *PresetManager) SavePreset(name, description string, models []ModelSelection) error {
	// Filter only selected models
	selectedModels := []ModelSelection{}
	for _, model := range models {
		if model.Selected {
			selectedModels = append(selectedModels, model)
		}
	}

	if len(selectedModels) == 0 {
		return fmt.Errorf("no models selected")
	}

	preset := &ModelPreset{
		Name:        name,
		Description: description,
		Models:      selectedModels,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		UsageCount:  0,
	}

	// Save to file
	filename := filepath.Join(pm.presetsDir, fmt.Sprintf("%s.json", name))
	data, err := json.MarshalIndent(preset, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}

	// Add to memory
	pm.presets[name] = preset

	return nil
}

// LoadPreset loads a specific preset
func (pm *PresetManager) LoadPreset(name string) (*ModelPreset, error) {
	if preset, ok := pm.presets[name]; ok {
		// Update usage count
		preset.UsageCount++
		preset.UpdatedAt = time.Now()
		pm.SavePreset(preset.Name, preset.Description, preset.Models)
		return preset, nil
	}

	filename := filepath.Join(pm.presetsDir, fmt.Sprintf("%s.json", name))
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var preset ModelPreset
	if err := json.Unmarshal(data, &preset); err != nil {
		return nil, err
	}

	pm.presets[name] = &preset
	return &preset, nil
}

// LoadAll loads all presets from disk
func (pm *PresetManager) LoadAll() error {
	files, err := os.ReadDir(pm.presetsDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		name := file.Name()[:len(file.Name())-5] // Remove .json

		data, err := os.ReadFile(filepath.Join(pm.presetsDir, file.Name()))
		if err != nil {
			continue
		}

		var preset ModelPreset
		if err := json.Unmarshal(data, &preset); err != nil {
			continue
		}

		pm.presets[name] = &preset
	}

	return nil
}

// DeletePreset deletes a preset
func (pm *PresetManager) DeletePreset(name string) error {
	filename := filepath.Join(pm.presetsDir, fmt.Sprintf("%s.json", name))

	if err := os.Remove(filename); err != nil {
		return err
	}

	delete(pm.presets, name)

	return nil
}

// ListPresets returns all presets
func (pm *PresetManager) ListPresets() []*ModelPreset {
	presets := make([]*ModelPreset, 0, len(pm.presets))
	for _, preset := range pm.presets {
		presets = append(presets, preset)
	}
	return presets
}

// ApplyPreset applies a preset to the model
func (m *Model) ApplyPreset(preset *ModelPreset) {
	// Clear current selection
	for i := range m.selectedModels {
		m.selectedModels[i].Selected = false
	}

	// Apply preset selections
	for _, presetModel := range preset.Models {
		for i := range m.selectedModels {
			if m.selectedModels[i].Provider == presetModel.Provider &&
				m.selectedModels[i].Model == presetModel.Model {
				m.selectedModels[i].Selected = true
			}
		}
	}

	// Enable multi-model mode
	m.multiModelEnabled = true

	// Show confirmation
	selectedCount := 0
	for _, model := range m.selectedModels {
		if model.Selected {
			selectedCount++
		}
	}

	m.messages = append(m.messages, Message{
		Role:    "system",
		Content: fmt.Sprintf("🔖 Preset '%s' applied.\n%d models selected: %s", preset.Name, selectedCount, preset.Description),
	})
}

// Built-in presets
func GetBuiltInPresets() []*ModelPreset {
	return []*ModelPreset{
		{
			Name:        "fast",
			Description: "Fast models for quick responses",
			Models: []ModelSelection{
				{Provider: "ollama", Model: "llama3.1:latest", DisplayName: "ollama - Llama 3.1", Selected: true},
				{Provider: "anthropic", Model: "claude-3-5-haiku-20241022", DisplayName: "anthropic - Claude 3.5 Haiku", Selected: true},
				{Provider: "google", Model: "gemini-2.0-flash-exp", DisplayName: "google - Gemini 2.0 Flash", Selected: true},
			},
		},
		{
			Name:        "quality",
			Description: "High-quality models for best results",
			Models: []ModelSelection{
				{Provider: "anthropic", Model: "claude-sonnet-4-20250514", DisplayName: "anthropic - Claude Sonnet 4", Selected: true},
				{Provider: "openai", Model: "gpt-4o", DisplayName: "openai - GPT-4o", Selected: true},
				{Provider: "google", Model: "gemini-1.5-pro", DisplayName: "google - Gemini 1.5 Pro", Selected: true},
			},
		},
		{
			Name:        "code",
			Description: "Code-specialized models",
			Models: []ModelSelection{
				{Provider: "ollama", Model: "codellama:latest", DisplayName: "ollama - CodeLlama", Selected: true},
				{Provider: "anthropic", Model: "claude-sonnet-4-20250514", DisplayName: "anthropic - Claude Sonnet 4", Selected: true},
				{Provider: "openai", Model: "gpt-4o", DisplayName: "openai - GPT-4o", Selected: true},
			},
		},
		{
			Name:        "local",
			Description: "Local models only (no API cost)",
			Models: []ModelSelection{
				{Provider: "ollama", Model: "llama3.1:latest", DisplayName: "ollama - Llama 3.1", Selected: true},
				{Provider: "ollama", Model: "mistral:latest", DisplayName: "ollama - Mistral", Selected: true},
				{Provider: "ollama", Model: "codellama:latest", DisplayName: "ollama - CodeLlama", Selected: true},
			},
		},
	}
}
