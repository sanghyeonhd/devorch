// Package shell provides persistent shell session management
package shell

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Shell represents a persistent shell session
type Shell struct {
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	workingDir   string
	env          map[string]string
	history      []CommandRecord
	historyMu    sync.RWMutex
	outputBuf    strings.Builder
	outputMu     sync.RWMutex
	running      bool
	runningMu    sync.RWMutex
	lastExitCode int
}

// CommandRecord represents a recorded shell command
type CommandRecord struct {
	Command   string    `json:"command"`
	Output    string    `json:"output"`
	ExitCode  int       `json:"exit_code"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  float64   `json:"duration_ms"`
}

// Manager manages multiple shell sessions
type Manager struct {
	shells     map[string]*Shell
	shellsMu   sync.RWMutex
	defaultDir string
}

// NewManager creates a new shell manager
func NewManager(defaultDir string) *Manager {
	if defaultDir == "" {
		defaultDir, _ = os.Getwd()
	}
	return &Manager{
		shells:     make(map[string]*Shell),
		defaultDir: defaultDir,
	}
}

// GetOrCreate gets an existing shell or creates a new one
func (m *Manager) GetOrCreate(id string) (*Shell, error) {
	m.shellsMu.Lock()
	defer m.shellsMu.Unlock()

	if shell, ok := m.shells[id]; ok {
		return shell, nil
	}

	shell, err := NewShell(m.defaultDir)
	if err != nil {
		return nil, err
	}

	m.shells[id] = shell
	return shell, nil
}

// Get retrieves an existing shell
func (m *Manager) Get(id string) (*Shell, bool) {
	m.shellsMu.RLock()
	defer m.shellsMu.RUnlock()

	shell, ok := m.shells[id]
	return shell, ok
}

// Close closes a shell session
func (m *Manager) Close(id string) error {
	m.shellsMu.Lock()
	defer m.shellsMu.Unlock()

	if shell, ok := m.shells[id]; ok {
		err := shell.Close()
		delete(m.shells, id)
		return err
	}
	return nil
}

// CloseAll closes all shell sessions
func (m *Manager) CloseAll() {
	m.shellsMu.Lock()
	defer m.shellsMu.Unlock()

	for id, shell := range m.shells {
		shell.Close()
		delete(m.shells, id)
	}
}

// NewShell creates a new persistent shell
func NewShell(workingDir string) (*Shell, error) {
	if workingDir == "" {
		workingDir, _ = os.Getwd()
	}

	shell := &Shell{
		workingDir: workingDir,
		env:        make(map[string]string),
		history:    make([]CommandRecord, 0),
		running:    false,
	}

	// Copy current environment
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			shell.env[parts[0]] = parts[1]
		}
	}

	return shell, nil
}

// Execute executes a command in the shell
func (s *Shell) Execute(ctx context.Context, command string) (string, int, error) {
	s.runningMu.Lock()
	s.running = true
	s.runningMu.Unlock()

	defer func() {
		s.runningMu.Lock()
		s.running = false
		s.runningMu.Unlock()
	}()

	startTime := time.Now()

	// Handle cd command specially
	if strings.HasPrefix(strings.TrimSpace(command), "cd ") {
		return s.handleCd(command)
	}

	// Handle export command specially
	if strings.HasPrefix(strings.TrimSpace(command), "export ") {
		return s.handleExport(command)
	}

	// Determine shell
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/sh"
	}

	// Create command
	cmd := exec.CommandContext(ctx, shellPath, "-c", command)
	cmd.Dir = s.workingDir

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range s.env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Capture output
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	s.lastExitCode = exitCode

	// Record command
	record := CommandRecord{
		Command:   command,
		Output:    output,
		ExitCode:  exitCode,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  float64(duration),
	}

	s.historyMu.Lock()
	s.history = append(s.history, record)
	s.historyMu.Unlock()

	return output, exitCode, nil
}

// ExecuteStream executes a command with streaming output
func (s *Shell) ExecuteStream(ctx context.Context, command string, outputChan chan<- string) (int, error) {
	s.runningMu.Lock()
	s.running = true
	s.runningMu.Unlock()

	defer func() {
		s.runningMu.Lock()
		s.running = false
		s.runningMu.Unlock()
	}()

	startTime := time.Now()

	// Determine shell
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/sh"
	}

	cmd := exec.CommandContext(ctx, shellPath, "-c", command)
	cmd.Dir = s.workingDir

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range s.env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Create pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 1, err
	}

	if err := cmd.Start(); err != nil {
		return 1, err
	}

	var outputBuf strings.Builder
	var wg sync.WaitGroup

	// Read stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			outputBuf.WriteString(line)
			if outputChan != nil {
				select {
				case outputChan <- line:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// Read stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			outputBuf.WriteString(line)
			if outputChan != nil {
				select {
				case outputChan <- line:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	wg.Wait()
	err = cmd.Wait()

	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	s.lastExitCode = exitCode

	// Record command
	record := CommandRecord{
		Command:   command,
		Output:    outputBuf.String(),
		ExitCode:  exitCode,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  float64(duration),
	}

	s.historyMu.Lock()
	s.history = append(s.history, record)
	s.historyMu.Unlock()

	return exitCode, nil
}

func (s *Shell) handleCd(command string) (string, int, error) {
	parts := strings.Fields(command)
	if len(parts) < 2 {
		// cd without args goes to home
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", 1, err
		}
		s.workingDir = homeDir
		return "", 0, nil
	}

	targetDir := parts[1]

	// Handle ~ expansion
	if strings.HasPrefix(targetDir, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", 1, err
		}
		targetDir = filepath.Join(homeDir, targetDir[1:])
	}

	// Handle relative paths
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(s.workingDir, targetDir)
	}

	// Clean the path
	targetDir = filepath.Clean(targetDir)

	// Check if directory exists
	info, err := os.Stat(targetDir)
	if err != nil {
		return fmt.Sprintf("cd: %s: No such file or directory", parts[1]), 1, nil
	}

	if !info.IsDir() {
		return fmt.Sprintf("cd: %s: Not a directory", parts[1]), 1, nil
	}

	s.workingDir = targetDir
	return "", 0, nil
}

func (s *Shell) handleExport(command string) (string, int, error) {
	// Remove "export " prefix
	envPart := strings.TrimPrefix(strings.TrimSpace(command), "export ")

	// Parse NAME=VALUE
	parts := strings.SplitN(envPart, "=", 2)
	if len(parts) != 2 {
		return "export: Invalid format. Use: export NAME=VALUE", 1, nil
	}

	name := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Remove quotes if present
	if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
		(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
		value = value[1 : len(value)-1]
	}

	s.env[name] = value
	return "", 0, nil
}

// GetWorkingDir returns current working directory
func (s *Shell) GetWorkingDir() string {
	return s.workingDir
}

// SetWorkingDir sets the working directory
func (s *Shell) SetWorkingDir(dir string) {
	s.workingDir = dir
}

// GetEnv returns the environment variables
func (s *Shell) GetEnv() map[string]string {
	env := make(map[string]string)
	for k, v := range s.env {
		env[k] = v
	}
	return env
}

// SetEnv sets an environment variable
func (s *Shell) SetEnv(key, value string) {
	s.env[key] = value
}

// GetHistory returns command history
func (s *Shell) GetHistory() []CommandRecord {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	history := make([]CommandRecord, len(s.history))
	copy(history, s.history)
	return history
}

// GetLastExitCode returns the last command's exit code
func (s *Shell) GetLastExitCode() int {
	return s.lastExitCode
}

// IsRunning checks if a command is currently running
func (s *Shell) IsRunning() bool {
	s.runningMu.RLock()
	defer s.runningMu.RUnlock()
	return s.running
}

// Close closes the shell session
func (s *Shell) Close() error {
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}

// State returns the current shell state for persistence
func (s *Shell) State() ShellState {
	return ShellState{
		WorkingDir: s.workingDir,
		Env:        s.GetEnv(),
	}
}

// RestoreState restores shell state
func (s *Shell) RestoreState(state ShellState) {
	s.workingDir = state.WorkingDir
	for k, v := range state.Env {
		s.env[k] = v
	}
}

// ShellState represents the persistent state of a shell
type ShellState struct {
	WorkingDir string            `json:"working_dir"`
	Env        map[string]string `json:"env"`
}
