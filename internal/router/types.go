package router

import "time"

type Workload struct {
	WorkspaceID string
	UserID      string
	ProjectID   string

	AgentType string
	Category  string
	TaskType  string
}

type Candidate struct {
	Provider string
	Model    string
	Endpoint string

	EstLatencyMS    int
	EstCostMicroUSD int64
	EstQuality      float64 // 0~1

	Tags map[string]string `json:"tags,omitempty"` // 예: {"tier":"local","cap":"code"}
}

type Decision struct {
	Provider string
	Model    string
	Endpoint string

	Score float64
	Why   string
}

// RequestCtx = 라우팅 평가에 필요한 "상황"
type RequestCtx struct {
	Scenario string `json:"scenario"` // "quick" | "chat" | "code-edit" | "tool-call" ...

	// Soft constraints
	PreferLocal bool          `json:"preferLocal"`
	MaxLatency  time.Duration `json:"maxLatency"` // 0이면 무시
	MaxCostUSD  float64       `json:"maxCostUsd"` // 0이면 무시

	// Machine identity (same box learn)
	OS                 string `json:"os"`
	Arch               string `json:"arch"`
	MachineFingerprint string `json:"machineFingerprint"`
}

// Scored = 후보 + 점수/근거
type Scored struct {
	Candidate Candidate `json:"candidate"`
	Score     float64   `json:"score"` // 높을수록 좋음
	Reasons   []string  `json:"reasons,omitempty"`
}
