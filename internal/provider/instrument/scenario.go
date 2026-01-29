package instrument

// Scenario = 벤치 집계의 키가 되는 "작업 유형"
// 절대 마음대로 문자열 바꾸지 말 것 (DB 집계가 깨짐)
type Scenario string

const (
	ScenarioChat       Scenario = "chat"
	ScenarioToolCall   Scenario = "tool-call"
	ScenarioCodeEdit   Scenario = "code-edit"
	ScenarioCodeReview Scenario = "code-review"
	ScenarioEmbedding  Scenario = "embedding"
	ScenarioRerank     Scenario = "rerank"
	ScenarioSummarize  Scenario = "summarize"
	ScenarioOther      Scenario = "other"
)

func NormalizeScenario(s string) Scenario {
	switch Scenario(s) {
	case ScenarioChat, ScenarioToolCall, ScenarioCodeEdit, ScenarioCodeReview,
		ScenarioEmbedding, ScenarioRerank, ScenarioSummarize:
		return Scenario(s)
	}
	if s == "" {
		return ScenarioOther
	}
	return ScenarioOther
}
