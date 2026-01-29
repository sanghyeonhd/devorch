package router

type ExplainItem struct {
	Key   string
	Value string
	Score float64
}

type Explain struct {
	ChosenModel    string
	ChosenProvider string
	BaseScore      float64
	FinalScore     float64
	Reasons        []ExplainItem
}

func (e *Explain) Add(key, value string, score float64) {
	e.Reasons = append(e.Reasons, ExplainItem{Key: key, Value: value, Score: score})
}
