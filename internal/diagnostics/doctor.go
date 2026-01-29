package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarn     Severity = "warn"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

type CheckResult struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Severity    Severity       `json:"severity"`
	Passed      bool           `json:"passed"`
	Summary     string         `json:"summary"`
	Details     map[string]any `json:"details,omitempty"`
	Suggestions []string       `json:"suggestions,omitempty"`
	StartedAt   time.Time      `json:"startedAt"`
	FinishedAt  time.Time      `json:"finishedAt"`
	DurationMs  int64          `json:"durationMs"`
}

type Check interface {
	ID() string
	Name() string
	Run(ctx context.Context, in Input) CheckResult
}

// Logger interface for diagnostics
type Logger interface {
	Infof(format string, args ...any)
	Errorf(format string, args ...any)
}

type Input struct {
	Log Logger

	// Optional fields - 존재하면 활용
	Version   string
	OS        string
	Arch      string
	DataDir   string
	ConfigDir string
	CacheDir  string
	StateDir  string
	LogDir    string

	// 아래 의존성은 Step 1~8에서 주입된다고 가정
	Bench BenchAPI
}

type BenchAPI interface {
	RunQuick(ctx context.Context) (QuickBenchSummary, error)
}

type QuickBenchSummary struct {
	CreatedAt time.Time `json:"createdAt"`
	Items     []any     `json:"items"`
}

type Doctor struct {
	logger Logger
	checks []Check
}

func NewDoctor(l Logger) *Doctor {
	return &Doctor{logger: l, checks: make([]Check, 0, 32)}
}

func (d *Doctor) Register(c Check) {
	d.checks = append(d.checks, c)
}

func (d *Doctor) Run(ctx context.Context, in Input) Report {
	start := time.Now()

	if in.Log == nil {
		in.Log = d.logger
	}

	// stable order
	sort.SliceStable(d.checks, func(i, j int) bool {
		return d.checks[i].ID() < d.checks[j].ID()
	})

	results := make([]CheckResult, 0, len(d.checks))
	var hasError bool
	var hasCritical bool

	for _, c := range d.checks {
		r := c.Run(ctx, in)
		results = append(results, r)
		if !r.Passed {
			if r.Severity == SeverityCritical {
				hasCritical = true
			}
			if r.Severity == SeverityError || r.Severity == SeverityCritical {
				hasError = true
			}
		}
	}

	status := "ok"
	if hasError {
		status = "degraded"
	}
	if hasCritical {
		status = "failed"
	}

	return Report{
		Status:     status,
		StartedAt:  start,
		FinishedAt: time.Now(),
		Results:    results,
	}
}

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func CombineErr(errs ...error) error {
	var out []error
	for _, e := range errs {
		if e != nil {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil
	}
	if len(out) == 1 {
		return out[0]
	}
	return errors.New(fmt.Sprintf("multiple errors: %v", out))
}
