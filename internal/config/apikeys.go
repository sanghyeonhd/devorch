package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// APIKeyManager handles API key configuration and management
type APIKeyManager struct {
	configPath string
	keys       map[string]string
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager() *APIKeyManager {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".devorch", "api_keys.env")

	return &APIKeyManager{
		configPath: configPath,
		keys:       make(map[string]string),
	}
}

// LoadKeys loads API keys from environment and config file
func (akm *APIKeyManager) LoadKeys() error {
	// Ensure config directory exists
	configDir := filepath.Dir(akm.configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Load from environment variables first
	akm.loadFromEnv()

	// Load from config file if exists
	if _, err := os.Stat(akm.configPath); err == nil {
		if err := akm.loadFromFile(); err != nil {
			return fmt.Errorf("failed to load from config file: %w", err)
		}
	}

	return nil
}

// loadFromEnv loads API keys from environment variables
func (akm *APIKeyManager) loadFromEnv() {
	envKeys := map[string]string{
		"OPENAI_API_KEY":     "openai",
		"ANTHROPIC_API_KEY":  "anthropic",
		"GOOGLE_API_KEY":     "google",
		"OPENROUTER_API_KEY": "openrouter",
		"GROQ_API_KEY":       "groq",
		"GEMINI_API_KEY":     "gemini",
		"CLAUDE_API_KEY":     "claude",
	}

	for envVar, provider := range envKeys {
		if value := os.Getenv(envVar); value != "" {
			akm.keys[provider] = value
		}
	}
}

// loadFromFile loads API keys from config file
func (akm *APIKeyManager) loadFromFile() error {
	content, err := os.ReadFile(akm.configPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}

		// Map environment variable names to provider names
		switch key {
		case "OPENAI_API_KEY":
			akm.keys["openai"] = value
		case "ANTHROPIC_API_KEY":
			akm.keys["anthropic"] = value
		case "GOOGLE_API_KEY":
			akm.keys["google"] = value
		case "OPENROUTER_API_KEY":
			akm.keys["openrouter"] = value
		case "GROQ_API_KEY":
			akm.keys["groq"] = value
		case "GEMINI_API_KEY":
			akm.keys["gemini"] = value
		case "CLAUDE_API_KEY":
			akm.keys["claude"] = value
		}
	}

	return nil
}

// SetKey sets an API key for a provider
func (akm *APIKeyManager) SetKey(provider, key string) error {
	akm.keys[provider] = key
	return akm.saveToFile()
}

// GetKey gets an API key for a provider
func (akm *APIKeyManager) GetKey(provider string) (string, bool) {
	key, exists := akm.keys[provider]
	return key, exists
}

// RemoveKey removes an API key for a provider
func (akm *APIKeyManager) RemoveKey(provider string) error {
	delete(akm.keys, provider)
	return akm.saveToFile()
}

// ListKeys lists all configured providers
func (akm *APIKeyManager) ListKeys() map[string]bool {
	result := make(map[string]bool)
	providers := []string{"openai", "anthropic", "google", "openrouter", "groq", "gemini", "claude"}

	for _, provider := range providers {
		_, exists := akm.keys[provider]
		result[provider] = exists
	}

	return result
}

// saveToFile saves API keys to config file
func (akm *APIKeyManager) saveToFile() error {
	var content strings.Builder
	content.WriteString("# DevOrch API Keys Configuration\n")
	content.WriteString("# Generated automatically - do not edit manually\n\n")

	for provider, key := range akm.keys {
		var envVar string
		switch provider {
		case "openai":
			envVar = "OPENAI_API_KEY"
		case "anthropic":
			envVar = "ANTHROPIC_API_KEY"
		case "google":
			envVar = "GOOGLE_API_KEY"
		case "openrouter":
			envVar = "OPENROUTER_API_KEY"
		case "groq":
			envVar = "GROQ_API_KEY"
		case "gemini":
			envVar = "GEMINI_API_KEY"
		case "claude":
			envVar = "CLAUDE_API_KEY"
		default:
			envVar = strings.ToUpper(provider) + "_API_KEY"
		}
		content.WriteString(fmt.Sprintf("%s=\"%s\"\n", envVar, key))
	}

	return os.WriteFile(akm.configPath, []byte(content.String()), 0600)
}

// ValidateKey validates an API key format (basic validation)
func (akm *APIKeyManager) ValidateKey(provider, key string) error {
	if key == "" {
		return fmt.Errorf("API key cannot be empty")
	}

	// Basic format validation
	switch provider {
	case "openai":
		if !strings.HasPrefix(key, "sk-") {
			return fmt.Errorf("OpenAI API key must start with 'sk-'")
		}
	case "anthropic", "claude":
		if !strings.HasPrefix(key, "sk-ant-") {
			return fmt.Errorf("Anthropic API key must start with 'sk-ant-'")
		}
	case "google", "gemini":
		if len(key) < 20 {
			return fmt.Errorf("Google API key seems too short")
		}
	case "openrouter":
		if !strings.HasPrefix(key, "sk-or-") {
			return fmt.Errorf("OpenRouter API key must start with 'sk-or-'")
		}
	}

	return nil
}

// SetupPredefinedKeys sets up the predefined API keys
func (akm *APIKeyManager) SetupPredefinedKeys() error {
	predefinedKeys := map[string]string{
		"openai":     "sk-proj-jH3PiDo4L50uCUnHkzOrvOqBtNECRH_2hgXQJrJjjqT6Nm5gTrvvbKDPQ96eKtYLu5lrMJeR4eT3BlbkFJPu2llmXNxN4MZmJaKhv_DvmJ1PvITWAO7Jw3s4HFwNDwzPLdvNDwzPLdvNDwzPLdvNDwzPL",
		"anthropic":  "sk-ant-api03-PWaG0FKtppvU5E_WDMbxNJJnSDVMxzzE8vGGnDvgdKYgGFklvNJbzGk8VxPHvpwfQUzKzqwqwqwqwqwqwqwqwqwqwqwqAA",
		"gemini":     "AIzaSyA2PMJJjjqT6Nm5gTrvvbKDPQ96eKtYLu5lrMJeR4eT3BlbkFJPu2llmXNxN4MZmJaKhv_DvmJ1PvITWAO7Jw3s4HFwN",
		"openrouter": "sk-or-v1-jH3PiDo4L50uCUnHkzOrvOqBtNECRH_2hgXQJrJjjqT6Nm5gTrvvbKDPQ96eKtYLu5lrMJeR4eT3BlbkFJPu2llmXNxN4MZm",
	}

	for provider, key := range predefinedKeys {
		if err := akm.SetKey(provider, key); err != nil {
			return fmt.Errorf("failed to set key for %s: %w", provider, err)
		}
	}

	return nil
}
