package diagnostics

import (
	"encoding/json"
	"time"
)

type Report struct {
	Status     string        `json:"status"` // ok|degraded|failed
	StartedAt  time.Time     `json:"startedAt"`
	FinishedAt time.Time     `json:"finishedAt"`
	Results    []CheckResult `json:"results"`
}

func (r Report) DurationMs() int64 {
	return r.FinishedAt.Sub(r.StartedAt).Milliseconds()
}

func (r Report) JSON(pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(r, "", "  ")
	}
	return json.Marshal(r)
}

func (r Report) HasFailures() bool {
	for _, x := range r.Results {
		if !x.Passed && (x.Severity == SeverityError || x.Severity == SeverityCritical) {
			return true
		}
	}
	return false
}
