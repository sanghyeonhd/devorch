package signal

import (
	"context"
	"time"

	"devorch/internal/id"
)

// OSTestRunner uses RunCommand for cross-platform execution
type OSTestRunner struct {
	Store TestRunWriter
}

func NewOSTestRunner(store TestRunWriter) *OSTestRunner {
	return &OSTestRunner{Store: store}
}

func (r *OSTestRunner) Run(ctx context.Context, req RunTestsRequest) (TestRun, error) {
	start := time.Now()

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

	res, err := RunCommand(ctx, ExecSpec{
		Command: req.Command,
		Dir:     req.Dir,
		Env:     req.Env,
		Timeout: req.Timeout,
	})

	end := time.Now()
	run.EndedAt = end
	run.DurationMs = end.Sub(start).Milliseconds()
	run.ExitCode = res.ExitCode
	run.Success = (err == nil && res.ExitCode == 0 && !res.TimedOut)
	run.Error = errString(err)
	run.StdoutSum = summarize(string(res.Stdout), 2000)
	run.StderrSum = summarize(string(res.Stderr), 2000)

	if r.Store != nil {
		_, _ = r.Store.InsertTestRun(ctx, run)
	}

	return run, err
}
