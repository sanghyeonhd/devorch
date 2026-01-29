package share

import (
	"devorch/internal/router"
)

// Exporter: 로컬 policy를 export 가능한 JSON/Envelope 형태로 변환
type Exporter struct{}

type ExportEnvelope struct {
	Version  string          `json:"version"`
	Policies []ExportedEntry `json:"policies"`
}

type ExportedEntry struct {
	OS       string  `json:"os"`
	Arch     string  `json:"arch"`
	Scenario string  `json:"scenario"`
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	Weight   float64 `json:"weight"`

	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	AvgCost      float64 `json:"avg_cost"`
	SampleCount  int     `json:"sample_count"`
}

func NewExporter() *Exporter {
	return &Exporter{}
}

func (e *Exporter) Export(policies []router.Policy) ExportEnvelope {
	entries := make([]ExportedEntry, 0, len(policies))
	for _, p := range policies {
		entries = append(entries, ExportedEntry{
			OS:           p.OS,
			Arch:         p.Arch,
			Scenario:     p.Scenario,
			Provider:     p.Provider,
			Model:        p.Model,
			Weight:       p.Weight,
			SuccessRate:  p.SuccessRate,
			AvgLatencyMs: p.AvgLatencyMs,
			AvgCost:      p.AvgCost,
			SampleCount:  p.SampleCount,
		})
	}
	return ExportEnvelope{
		Version:  "1.0",
		Policies: entries,
	}
}
