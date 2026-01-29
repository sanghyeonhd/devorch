package observer

import (
	"context"
	"time"
)

// ProviderCall = provider 한 번 호출에 대한 결과 요약(벤치 기록용)
type ProviderCall struct {
	CreatedAt time.Time

	Provider string
	Model    string
	Scenario string // "chat" | "code-edit" | "tool-call" 등

	OS                 string
	Arch               string
	MachineFingerprint string

	InputTokens  int64
	OutputTokens int64
	LatencyMs    int64

	Success bool
	Error   string

	// 선택
	CostUSD      *float64
	QualityScore *float64

	Metadata map[string]any
}

// Observer = 호출 완료 이벤트 수신자
type Observer interface {
	OnComplete(ctx context.Context, call ProviderCall) error
}
