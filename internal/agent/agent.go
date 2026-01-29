// Package agent provides multi-agent orchestration for AI tasks
package agent

import (
	"encoding/json"
)

// Agent defines an AI agent with specific capabilities
type Agent struct {
	// Name is the unique identifier
	Name string `json:"name"`

	// Model is the default model to use (e.g., "anthropic/claude-3-opus")
	Model string `json:"model"`

	// Temperature for response generation
	Temperature float64 `json:"temperature"`

	// SystemPrompt defines agent behavior
	SystemPrompt string `json:"system_prompt"`

	// AllowedTools whitelist (empty = all allowed)
	AllowedTools []string `json:"allowed_tools,omitempty"`

	// DeniedTools blacklist
	DeniedTools []string `json:"denied_tools,omitempty"`

	// MaxTokens for response
	MaxTokens int `json:"max_tokens,omitempty"`

	// Metadata for agent-specific configuration
	Metadata map[string]any `json:"metadata,omitempty"`
}

// CanUseTool checks if the agent can use a specific tool
func (a *Agent) CanUseTool(tool string) bool {
	// Check blacklist first
	for _, denied := range a.DeniedTools {
		if denied == tool || denied == "*" {
			return false
		}
	}

	// If whitelist is empty, allow all
	if len(a.AllowedTools) == 0 {
		return true
	}

	// Check whitelist
	for _, allowed := range a.AllowedTools {
		if allowed == tool || allowed == "*" {
			return true
		}
	}

	return false
}

// Clone creates a copy of the agent
func (a *Agent) Clone() *Agent {
	clone := *a
	if a.AllowedTools != nil {
		clone.AllowedTools = make([]string, len(a.AllowedTools))
		copy(clone.AllowedTools, a.AllowedTools)
	}
	if a.DeniedTools != nil {
		clone.DeniedTools = make([]string, len(a.DeniedTools))
		copy(clone.DeniedTools, a.DeniedTools)
	}
	if a.Metadata != nil {
		clone.Metadata = make(map[string]any)
		for k, v := range a.Metadata {
			clone.Metadata[k] = v
		}
	}
	return &clone
}

// WithModel returns a copy with different model
func (a *Agent) WithModel(model string) *Agent {
	clone := a.Clone()
	clone.Model = model
	return clone
}

// WithTemperature returns a copy with different temperature
func (a *Agent) WithTemperature(temp float64) *Agent {
	clone := a.Clone()
	clone.Temperature = temp
	return clone
}

// ToJSON serializes the agent to JSON
func (a *Agent) ToJSON() ([]byte, error) {
	return json.Marshal(a)
}

// FromJSON deserializes an agent from JSON
func FromJSON(data []byte) (*Agent, error) {
	var a Agent
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}
