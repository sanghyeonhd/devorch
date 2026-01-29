package learner

import (
	"devorch/internal/provider/observer"
	"devorch/internal/router/scope"
)

type Features struct {
	ScopeType string
	ScopeID   string

	OS       string
	Arch     string
	Scenario string

	Provider string
	Model    string

	LatencyMs    int64
	Success      bool
	InputTokens  int64
	OutputTokens int64
	Cost         float64
}

func FromCall(key scope.Key, c observer.ProviderCall, cost float64) Features {
	return Features{
		ScopeType: string(key.Type),
		ScopeID:   key.ID,

		OS:       c.OS,
		Arch:     c.Arch,
		Scenario: c.Scenario,

		Provider: c.Provider,
		Model:    c.Model,

		LatencyMs:    c.LatencyMs,
		Success:      c.Success,
		InputTokens:  c.InputTokens,
		OutputTokens: c.OutputTokens,
		Cost:         cost,
	}
}
