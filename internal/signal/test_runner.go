package signal

import (
	"bytes"
	"context"
	"os/exec"
	"time"

	"devorch/internal/id"
)

type TestRunWriter interface {
	InsertTestRun(ctx context.Context, r TestRun) (string, error)
}

type TestRunner struct {
	Store TestRunWriter
}

func NewTestRunner(store TestRunWriter) *TestRunner {
	return &TestRunner{Store: store}
}

type RunTestsRequest struct {
	Workspace string
	ProjectID string
	SessionID string
	TaskID    string

	Runner  string // label
	Command string // full shell-like
	Dir     string
	Env     []string
	Timeout time.Duration
}

func (r *TestRunner) Run(ctx context.Context, req RunTestsRequest) (TestRun, error) {
	start := time.Now()
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	// MVP: exec.CommandContext with "sh -lc" (windows는 9단계에서 분기)
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, "sh", "-lc", req.Command)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	if len(req.Env) > 0 {
		cmd.Env = append(cmd.Environ(), req.Env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	run := TestRun{
		ID:        id.NewULID(),
		Workspace: req.Workspace,
		ProjectID: req.ProjectID,
		SessionID: req.SessionID,
		TaskID:    req.TaskID,
		Runner:    req.Runner,
		Command:   req.Command,
		StartedAt: start,
	}

	err := cmd.Run()
	end := time.Now()

	exitCode := 0
	success := err == nil

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 1
		}
	}

	run.EndedAt = end
	run.DurationMs = end.Sub(start).Milliseconds()
	run.Success = success
	run.ExitCode = exitCode
	run.Error = errString(err)
	run.StdoutSum = summarize(stdout.String(), 2000)
	run.StderrSum = summarize(stderr.String(), 2000)

	if r.Store != nil {
		_, _ = r.Store.InsertTestRun(ctx, run)
	}

	return run, err
}
