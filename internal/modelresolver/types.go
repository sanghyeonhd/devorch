package modelresolver

// Requirement는 모델 선택 요구사항
type Requirement struct {
	BaseModel string
	Providers []string

	// 확장 필드
	AgentID        string
	Category       string
	Model          string
	ContextLength  int
	MaxTokens      int
	NeedVision     bool
	NeedTool       bool
	ForcedProvider string
}

// Resolution은 모델 선택 결과
type Resolution struct {
	Provider string
	Model    string
	Source   string // "override" | "fallback" | "default"
	Reason   string // 선택 이유 (추가)
}
