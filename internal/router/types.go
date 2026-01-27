package router

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
}

type Decision struct {
	Provider string
	Model    string
	Endpoint string

	Score float64
	Why   string
}
