package okaon

import "time"

// Work은 하나의 작업 단위(세션/태스크 단위)
type Work struct {
	ID          string
	ProjectID   string
	WorkspaceID string
	UserID      string

	TaskType    string // "chat", "edit", "code", etc.
	Category    string // "general", "code", "debug", etc.
	Agent       string // agent identifier if applicable
	PromptHash  string // SHA256 of prompt (not raw)
	ContextHash string // SHA256 of context (not raw)
	OS          string
	Arch        string
	Fingerprint string // HW/환경 fingerprint
	TagsJSON    string // JSON array of tags

	CreatedAt time.Time
}

// Run은 하나의 LLM 호출 기록
type Run struct {
	ID       string
	WorkID   string
	Provider string
	Model    string

	RoutePolicy string
	WasFallback bool
	RetryCount  int

	LatencyMs    int64
	InputTokens  int64
	OutputTokens int64
	CostMicroUSD int64

	ErrorCode string
	ErrorMsg  string

	CreatedAt time.Time
}

// Quality는 품질 평가 기록
type Quality struct {
	ID          string
	RunID       string
	QualityBin  int // 0-10 or similar scale
	ScoreJSON   string
	EvaluatedAt time.Time
}

// Reward는 최종 보상값 기록
type Reward struct {
	ID        string
	RunID     string
	Reward01  float64 // normalized 0-1
	CreatedAt time.Time
}

// ArmStats는 밴딧 arm 통계
type ArmStats struct {
	Fingerprint string
	Category    string
	Provider    string
	Model       string

	CountRuns    int64
	SumReward01  float64
	SumRewardSq  float64
	MeanReward01 float64
	StdDevReward float64

	UpdatedAt time.Time
}

// ArmStat는 단일 arm 통계 조회용
type ArmStat struct {
	Fingerprint string
	Category    string
	Provider    string
	Model       string

	CountRuns    int64
	MeanReward01 float64
	UpdatedTsUTC int64
}

// RunRecord는 학습용 실행 기록
type RunRecord struct {
	Fingerprint  string
	Category     string
	Provider     string
	Model        string
	Reward01     float64
	LatencyMs    int64
	CostMicroUSD int64
	CreatedAt    time.Time
}

// ArmStatUpdate는 arm 통계 업데이트용
type ArmStatUpdate struct {
	Fingerprint string
	Category    string
	Provider    string
	Model       string

	DeltaCount     int64
	DeltaSumReward float64
	DeltaSumSq     float64
}
