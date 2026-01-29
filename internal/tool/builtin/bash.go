package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"devorch/internal/tool"
)

const (
	bashDefaultTimeout = 2 * time.Minute
	bashMaxOutput      = 50 * 1024 // 50KB
)

// BashParams defines parameters for bash command execution
type BashParams struct {
	Command     string `json:"command"`
	Timeout     int    `json:"timeout,omitempty"`     // milliseconds
	WorkDir     string `json:"workdir,omitempty"`     // working directory
	Description string `json:"description,omitempty"` // command description
}

// BashTool executes shell commands
type BashTool struct {
	// AllowedCommands whitelist (empty = all allowed)
	AllowedCommands []string

	// DeniedCommands blacklist
	DeniedCommands []string

	// DefaultWorkDir if not specified
	DefaultWorkDir string
}

// Info returns tool metadata
func (t *BashTool) Info() tool.ToolInfo {
	return tool.ToolInfo{
		Name: "bash",
		Description: `Execute a shell command.
Use this tool to run shell commands in the terminal.
Commands are executed in the specified working directory (defaults to project root).
Output is captured and returned. Long-running commands will timeout.
For potentially dangerous commands (rm, mv, chmod, etc.), exercise caution.`,
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "The shell command to execute"
				},
				"timeout": {
					"type": "number",
					"description": "Optional timeout in milliseconds (default: 120000)"
				},
				"workdir": {
					"type": "string",
					"description": "Working directory for command execution"
				},
				"description": {
					"type": "string",
					"description": "Brief description of what this command does"
				}
			},
			"required": ["command"]
		}`),
	}
}

// Execute runs the bash command
func (t *BashTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
	var p BashParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}

	// Determine working directory
	workDir := p.WorkDir
	if workDir == "" {
		workDir = ctx.WorkDir
	}
	if workDir == "" {
		workDir = t.DefaultWorkDir
	}
	if workDir == "" {
		wd, _ := os.Getwd()
		workDir = wd
	}

	// Resolve to absolute path
	if !filepath.IsAbs(workDir) {
		if ctx.WorkDir != "" {
			workDir = filepath.Join(ctx.WorkDir, workDir)
		}
	}

	// Determine timeout
	timeout := bashDefaultTimeout
	if p.Timeout > 0 {
		timeout = time.Duration(p.Timeout) * time.Millisecond
	}

	// Create command context with timeout
	execCtx, cancel := context.WithTimeout(ctx.Ctx, timeout)
	defer cancel()

	// Determine shell
	shell, args := t.getShell()

	// Create command
	cmd := exec.CommandContext(execCtx, shell, append(args, p.Command)...)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	// Set up process group for cleanup (Unix only)
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	// Combine output
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	// Truncate if needed
	output, truncated := tool.TruncateOutput(output, bashMaxOutput)

	// Determine exit code
	exitCode := 0
	timedOut := false
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			timedOut = true
			exitCode = -1
			output += "\n\n(Command timed out after " + timeout.String() + ")"
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// Build result
	result := &tool.Result{
		Output:    output,
		Title:     p.Description,
		Truncated: truncated,
		ExitCode:  exitCode,
		Metadata: map[string]any{
			"command":     p.Command,
			"workdir":     workDir,
			"exit_code":   exitCode,
			"duration_ms": duration.Milliseconds(),
			"timed_out":   timedOut,
		},
	}

	if exitCode != 0 && err != nil {
		return result, nil // Return result even on error (with exit code)
	}

	return result, nil
}

// getShell returns shell command and args for current OS
func (t *BashTool) getShell() (string, []string) {
	switch runtime.GOOS {
	case "windows":
		// Prefer Git Bash, fall back to cmd
		if gitBash := findGitBash(); gitBash != "" {
			return gitBash, []string{"-c"}
		}
		return "cmd", []string{"/C"}
	default:
		// Prefer zsh, fall back to bash, then sh
		for _, shell := range []string{"/bin/zsh", "/bin/bash", "/bin/sh"} {
			if _, err := os.Stat(shell); err == nil {
				return shell, []string{"-c"}
			}
		}
		return "sh", []string{"-c"}
	}
}

// findGitBash locates Git Bash on Windows
func findGitBash() string {
	paths := []string{
		`C:\Program Files\Git\bin\bash.exe`,
		`C:\Program Files (x86)\Git\bin\bash.exe`,
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// Name returns tool name for legacy interface
func (t *BashTool) Name() string {
	return "bash"
}

// ValidateCommand checks if command is allowed
func (t *BashTool) ValidateCommand(cmd string) bool {
	// Extract first word (command name)
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}
	cmdName := parts[0]

	// Check blacklist
	for _, denied := range t.DeniedCommands {
		if cmdName == denied {
			return false
		}
	}

	// If whitelist is empty, allow all
	if len(t.AllowedCommands) == 0 {
		return true
	}

	// Check whitelist
	for _, allowed := range t.AllowedCommands {
		if cmdName == allowed {
			return true
		}
	}

	return false
}
