package signal

import "time"

type ToolRun struct {
	ID        string
	Workspace string
	SessionID string
	TaskID    string

	ToolName   string
	StartedAt  time.Time
	EndedAt    time.Time
	DurationMs int64

	Success bool
	Error   string

	InputSummary  string
	OutputSummary string

	OutputBytes int64
	Tags        string // csv or json string (MVP)

	ExitCode int
	MetaJSON string
}

type TestRun struct {
	ID        string
	Workspace string
	ProjectID string
	SessionID string
	TaskID    string

	Runner     string
	Command    string
	StartedAt  time.Time
	EndedAt    time.Time
	DurationMs int64

	Success   bool
	ExitCode  int
	Error     string
	StdoutSum string
	StderrSum string
	Tags      string
}
