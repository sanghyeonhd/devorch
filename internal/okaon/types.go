package okaon

type Run struct {
	RunID     string
	CreatedAt string

	PolicyKey string

	WorkspaceID string
	UserID      string
	ProjectID   string

	AgentType string
	Category  string
	TaskType  string

	Provider string
	Model    string
	Endpoint string

	LatencyMS int
	OK        int     // 0/1
	Quality   float64 // 0~1

	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CostMicroUSD     int64

	TraceID      string
	ErrorCode    string
	ErrorMessage string
}
