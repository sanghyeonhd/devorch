package signal

import (
	"context"
	"os/exec"
	"runtime"
	"time"
)

type ExecSpec struct {
	Command string
	Dir     string
	Env     []string
	Timeout time.Duration
}

type ExecResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	TimedOut bool
}

func RunCommand(ctx context.Context, spec ExecSpec) (ExecResult, error) {
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// powershell 우선 (cmd 특수문자 문제 적음)
		cmd = exec.CommandContext(cctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", spec.Command)
	default:
		cmd = exec.CommandContext(cctx, "sh", "-lc", spec.Command)
	}

	if spec.Dir != "" {
		cmd.Dir = spec.Dir
	}
	if len(spec.Env) > 0 {
		cmd.Env = append(cmd.Environ(), spec.Env...)
	}

	stdout, err := cmd.Output()
	res := ExecResult{Stdout: stdout}

	if err == nil {
		res.ExitCode = 0
		return res, nil
	}

	// 에러 처리: stderr 추출
	if ee, ok := err.(*exec.ExitError); ok {
		res.Stderr = ee.Stderr
		res.ExitCode = ee.ExitCode()
	} else {
		res.ExitCode = 1
	}

	if cctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
	}

	return res, err
}
