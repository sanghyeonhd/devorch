package learning

import "time"

// CompletionMetrics: "작업이 끝났을 때" 저장할 벤치 메트릭
type CompletionMetrics struct {
	CreatedAt time.Time `json:"createdAt"`

	OS                 string `json:"os"`
	Arch               string `json:"arch"`
	MachineFingerprint string `json:"machineFingerprint"`

	Provider string `json:"provider"`
	Model    string `json:"model"`

	Scenario string `json:"scenario"` // "chat"|"code-edit"|"tool-call"...

	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	LatencyMs    int64 `json:"latencyMs"`

	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`

	QualityScore *float64 `json:"qualityScore,omitempty"` // 0..1 (thumbs 등에서 변환)
	CostUSD      *float64 `json:"costUsd,omitempty"`

	Metadata map[string]any `json:"metadata,omitempty"`
}
