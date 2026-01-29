package experiment

type Status string
type Kind string
type Strategy string

const (
	StatusDraft     Status = "draft"
	StatusRunning   Status = "running"
	StatusStopped   Status = "stopped"
	StatusCompleted Status = "completed"
)

const (
	KindPrompt  Kind = "prompt"
	KindRouting Kind = "routing"
	KindPolicy  Kind = "policy"
)

const (
	StrategyAB            Strategy = "ab"
	StrategyEpsilonGreedy Strategy = "epsilon_greedy"
)

type Experiment struct {
	ID       string
	Name     string
	Kind     Kind
	Status   Status
	Strategy Strategy

	Traffic float64
	Seed    string
}

type Variant struct {
	ID           string
	ExperimentID string
	Key          string
	Weight       float64
	PayloadJSON  string
	IsWinner     bool
}

type Assignment struct {
	ID           string
	ExperimentID string
	Subject      string
	VariantKey   string
}

type Outcome struct {
	ID           string
	ExperimentID string
	VariantKey   string

	BenchRunID string
	SessionID  string
	MessageID  string

	Success      bool
	LatencyMS    int
	CostMicroUSD int
	QualityScore float64
	TokensIn     int
	TokensOut    int
}

type Decision struct {
	ID           string
	ExperimentID string
	VariantKey   string
	Reason       string
}
