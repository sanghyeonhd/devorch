package quality

import "errors"

type EvalRequest struct {
	Workspace string
	SessionID string
	TaskID    string
	Provider  string
	Model     string
	Answer    string
	Error     error
}

type EvalResult struct {
	Success bool
	Score   float64
	Notes   string
	Error   string
}

func errString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

var ErrNotImplemented = errors.New("quality evaluator not implemented")
