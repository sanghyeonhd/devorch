package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CLIPresetManager manages model configuration presets in CLI mode
type CLIPresetManager struct {
	presetDir string
	presets   map[string]*CLIPreset
	loaded    bool
}

// CLIPreset represents a saved configuration preset
type CLIPreset struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Models      []string          `json:"models"`
	WorkMode    string            `json:"work_mode"`
	Context     map[string]string `json:"context"`
	Provider    string            `json:"provider"` // Primary provider
	Model       string            `json:"model"`    // Primary model
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	UsedCount   int               `json:"used_count"`
	LastUsed    time.Time         `json:"last_used"`
	Tags        []string          `json:"tags,omitempty"` // Optional tags
	Version     string            `json:"version"`        // Preset format version
}

// NewCLIPresetManager creates a new preset manager
func NewCLIPresetManager() *CLIPresetManager {
	homeDir, _ := os.UserHomeDir()
	presetDir := filepath.Join(homeDir, ".devorch", "presets")

	manager := &CLIPresetManager{
		presetDir: presetDir,
		presets:   make(map[string]*CLIPreset),
		loaded:    false,
	}

	// Ensure preset directory exists
	os.MkdirAll(presetDir, 0755)

	return manager
}

// LoadPresets loads all presets from disk
func (pm *CLIPresetManager) LoadPresets() error {
	if pm.loaded {
		return nil
	}

	files, err := filepath.Glob(filepath.Join(pm.presetDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to scan preset directory: %v", err)
	}

	for _, file := range files {
		preset, err := pm.loadPresetFromFile(file)
		if err != nil {
			// Log error but continue loading other presets
			fmt.Printf("Warning: failed to load preset from %s: %v\n", file, err)
			continue
		}

		pm.presets[preset.Name] = preset
	}

	pm.loaded = true
	return nil
}

// loadPresetFromFile loads a single preset from a JSON file
func (pm *CLIPresetManager) loadPresetFromFile(filename string) (*CLIPreset, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var preset CLIPreset
	if err := json.Unmarshal(data, &preset); err != nil {
		return nil, err
	}

	// Set version if not specified (for backwards compatibility)
	if preset.Version == "" {
		preset.Version = "1.0"
	}

	return &preset, nil
}

// SavePreset saves a preset to disk
func (pm *CLIPresetManager) SavePreset(preset *CLIPreset) error {
	if err := pm.LoadPresets(); err != nil {
		return err
	}

	// Update metadata
	preset.Version = "1.0"
	preset.UpdatedAt = time.Now()

	if preset.CreatedAt.IsZero() {
		preset.CreatedAt = preset.UpdatedAt
	}

	// Save to disk
	filename := filepath.Join(pm.presetDir, fmt.Sprintf("%s.json", preset.Name))
	data, err := json.MarshalIndent(preset, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal preset: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write preset file: %v", err)
	}

	// Update in-memory cache
	pm.presets[preset.Name] = preset

	return nil
}

// LoadPreset loads a specific preset by name
func (pm *CLIPresetManager) LoadPreset(name string) (*CLIPreset, error) {
	if err := pm.LoadPresets(); err != nil {
		return nil, err
	}

	preset, exists := pm.presets[name]
	if !exists {
		return nil, fmt.Errorf("preset '%s' not found", name)
	}

	// Update usage statistics
	preset.UsedCount++
	preset.LastUsed = time.Now()

	// Save updated statistics
	pm.SavePreset(preset)

	return preset, nil
}

// ListPresets returns all available presets
func (pm *CLIPresetManager) ListPresets() []*CLIPreset {
	if err := pm.LoadPresets(); err != nil {
		return []*CLIPreset{}
	}

	presets := make([]*CLIPreset, 0, len(pm.presets))
	for _, preset := range pm.presets {
		presets = append(presets, preset)
	}

	// Sort by name
	sort.Slice(presets, func(i, j int) bool {
		return presets[i].Name < presets[j].Name
	})

	return presets
}

// DeletePreset removes a preset
func (pm *CLIPresetManager) DeletePreset(name string) error {
	if err := pm.LoadPresets(); err != nil {
		return err
	}

	if _, exists := pm.presets[name]; !exists {
		return fmt.Errorf("preset '%s' not found", name)
	}

	// Remove from disk
	filename := filepath.Join(pm.presetDir, fmt.Sprintf("%s.json", name))
	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("failed to delete preset file: %v", err)
	}

	// Remove from memory
	delete(pm.presets, name)

	return nil
}

// PresetExists checks if a preset with the given name exists
func (pm *CLIPresetManager) PresetExists(name string) bool {
	if err := pm.LoadPresets(); err != nil {
		return false
	}

	_, exists := pm.presets[name]
	return exists
}

// CreatePresetFromCLI creates a preset from current CLI state
func (pm *CLIPresetManager) CreatePresetFromCLI(name, description string, cliMode *CLIMode) (*CLIPreset, error) {
	preset := &CLIPreset{
		Name:        name,
		Description: description,
		Provider:    cliMode.provider,
		Model:       cliMode.model,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		UsedCount:   0,
		Version:     "1.0",
	}

	// Include selected models if multi-model is active
	if cliMode.multiModel.IsActive() {
		preset.Models = cliMode.multiModel.GetSelectedModels()
	} else {
		preset.Models = []string{cliMode.model}
	}

	// Include work mode
	if cliMode.workModeManager != nil {
		preset.WorkMode = cliMode.workModeManager.GetCurrentMode().String()
		preset.Context = cliMode.workModeManager.GetContextMap()
	}

	return preset, pm.SavePreset(preset)
}

// ApplyPresetToCLI applies a preset configuration to CLI mode
func (pm *CLIPresetManager) ApplyPresetToCLI(preset *CLIPreset, cliMode *CLIMode) error {
	// Apply primary provider and model
	if preset.Provider != "" && preset.Model != "" {
		cliMode.provider = preset.Provider
		cliMode.model = preset.Model
		cliMode.client = cliMode.createClientForProvider(preset.Provider)
	}

	// Apply multi-model configuration if applicable
	if len(preset.Models) > 1 {
		cliMode.multiModel.ClearSelection()
		for _, model := range preset.Models {
			cliMode.multiModel.SelectModel(model)
		}
	}

	// Apply work mode
	if preset.WorkMode != "" && cliMode.workModeManager != nil {
		if workMode := parseWorkModeFromString(preset.WorkMode); workMode != -1 {
			cliMode.workModeManager.SwitchWorkMode(workMode, cliMode)

			// Apply context if available
			if preset.Context != nil {
				cliMode.workModeManager.LoadContext(preset.Context)
			}
		}
	}

	return nil
}

// GetPresetStats returns statistics about presets
func (pm *CLIPresetManager) GetPresetStats() map[string]interface{} {
	if err := pm.LoadPresets(); err != nil {
		return map[string]interface{}{"error": err.Error()}
	}

	stats := map[string]interface{}{
		"total_presets": len(pm.presets),
		"total_usage":   0,
	}

	var mostUsed *CLIPreset
	var recentlyUsed *CLIPreset

	for _, preset := range pm.presets {
		stats["total_usage"] = stats["total_usage"].(int) + preset.UsedCount

		if mostUsed == nil || preset.UsedCount > mostUsed.UsedCount {
			mostUsed = preset
		}

		if recentlyUsed == nil || preset.LastUsed.After(recentlyUsed.LastUsed) {
			recentlyUsed = preset
		}
	}

	if mostUsed != nil {
		stats["most_used"] = map[string]interface{}{
			"name":  mostUsed.Name,
			"count": mostUsed.UsedCount,
		}
	}

	if recentlyUsed != nil && !recentlyUsed.LastUsed.IsZero() {
		stats["recently_used"] = map[string]interface{}{
			"name": recentlyUsed.Name,
			"time": recentlyUsed.LastUsed.Format("2006-01-02 15:04:05"),
		}
	}

	return stats
}

// SearchPresets searches presets by name, description, or tags
func (pm *CLIPresetManager) SearchPresets(query string) []*CLIPreset {
	if err := pm.LoadPresets(); err != nil {
		return []*CLIPreset{}
	}

	query = strings.ToLower(query)
	var matches []*CLIPreset

	for _, preset := range pm.presets {
		// Search in name
		if strings.Contains(strings.ToLower(preset.Name), query) {
			matches = append(matches, preset)
			continue
		}

		// Search in description
		if strings.Contains(strings.ToLower(preset.Description), query) {
			matches = append(matches, preset)
			continue
		}

		// Search in tags
		for _, tag := range preset.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				matches = append(matches, preset)
				break
			}
		}
	}

	// Sort matches by relevance (exact name match first, then by usage count)
	sort.Slice(matches, func(i, j int) bool {
		// Exact name match gets highest priority
		if strings.EqualFold(matches[i].Name, query) && !strings.EqualFold(matches[j].Name, query) {
			return true
		}
		if !strings.EqualFold(matches[i].Name, query) && strings.EqualFold(matches[j].Name, query) {
			return false
		}

		// Otherwise sort by usage count
		return matches[i].UsedCount > matches[j].UsedCount
	})

	return matches
}

// DuplicatePreset creates a copy of an existing preset with a new name
func (pm *CLIPresetManager) DuplicatePreset(sourceName, newName, newDescription string) error {
	sourcePreset, err := pm.LoadPreset(sourceName)
	if err != nil {
		return err
	}

	if pm.PresetExists(newName) {
		return fmt.Errorf("preset '%s' already exists", newName)
	}

	// Create a copy
	newPreset := *sourcePreset
	newPreset.Name = newName
	newPreset.Description = newDescription
	newPreset.CreatedAt = time.Now()
	newPreset.UpdatedAt = time.Now()
	newPreset.UsedCount = 0
	newPreset.LastUsed = time.Time{}

	return pm.SavePreset(&newPreset)
}

// Helper function to parse work mode from string
func parseWorkModeFromString(modeStr string) WorkMode {
	switch strings.ToLower(modeStr) {
	case "ask":
		return WorkModeAsk
	case "edit":
		return WorkModeEdit
	case "agent":
		return WorkModeAgent
	case "plan":
		return WorkModePlan
	default:
		return -1 // Invalid work mode
	}
}
