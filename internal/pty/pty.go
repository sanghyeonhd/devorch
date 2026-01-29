// Package pty provides a pseudo-terminal (PTY) emulator for DevOrch.
package pty

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// Terminal represents a PTY terminal session
type Terminal struct {
	id       string
	cmd      *exec.Cmd
	ptmx     *os.File
	rows     uint16
	cols     uint16
	mu       sync.RWMutex
	output   chan []byte
	input    chan []byte
	done     chan struct{}
	isClosed bool
	onOutput func([]byte)
	onExit   func(int)
}

// Config holds terminal configuration
type Config struct {
	Shell   string
	Rows    uint16
	Cols    uint16
	Env     []string
	WorkDir string
}

// DefaultConfig returns default terminal configuration
func DefaultConfig() Config {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	return Config{
		Shell:   shell,
		Rows:    24,
		Cols:    80,
		Env:     os.Environ(),
		WorkDir: "",
	}
}

// New creates a new PTY terminal
func New(id string, cfg Config) (*Terminal, error) {
	if cfg.Shell == "" {
		cfg.Shell = "/bin/zsh"
	}
	if cfg.Rows == 0 {
		cfg.Rows = 24
	}
	if cfg.Cols == 0 {
		cfg.Cols = 80
	}

	t := &Terminal{
		id:     id,
		rows:   cfg.Rows,
		cols:   cfg.Cols,
		output: make(chan []byte, 256),
		input:  make(chan []byte, 64),
		done:   make(chan struct{}),
	}

	// Create command
	cmd := exec.Command(cfg.Shell)
	cmd.Env = cfg.Env
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}

	// Start PTY
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: cfg.Rows,
		Cols: cfg.Cols,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	t.cmd = cmd
	t.ptmx = ptmx

	// Start I/O goroutines
	go t.readLoop()
	go t.writeLoop()
	go t.waitLoop()

	return t, nil
}

// ID returns the terminal ID
func (t *Terminal) ID() string {
	return t.id
}

// readLoop reads output from the PTY
func (t *Terminal) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := t.ptmx.Read(buf)
		if err != nil {
			if err != io.EOF {
				// Log error but don't break
			}
			break
		}

		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			// Send to output channel
			select {
			case t.output <- data:
			default:
				// Buffer full, skip
			}

			// Call output callback
			if t.onOutput != nil {
				t.onOutput(data)
			}
		}
	}
}

// writeLoop writes input to the PTY
func (t *Terminal) writeLoop() {
	for {
		select {
		case data, ok := <-t.input:
			if !ok {
				return
			}
			if _, err := t.ptmx.Write(data); err != nil {
				return
			}
		case <-t.done:
			return
		}
	}
}

// waitLoop waits for the command to exit
func (t *Terminal) waitLoop() {
	exitCode := 0
	if err := t.cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	t.mu.Lock()
	t.isClosed = true
	t.mu.Unlock()

	close(t.done)

	if t.onExit != nil {
		t.onExit(exitCode)
	}
}

// Write sends input to the terminal
func (t *Terminal) Write(data []byte) error {
	t.mu.RLock()
	if t.isClosed {
		t.mu.RUnlock()
		return fmt.Errorf("terminal is closed")
	}
	t.mu.RUnlock()

	select {
	case t.input <- data:
		return nil
	case <-t.done:
		return fmt.Errorf("terminal is closed")
	case <-time.After(time.Second):
		return fmt.Errorf("write timeout")
	}
}

// Read returns the output channel
func (t *Terminal) Output() <-chan []byte {
	return t.output
}

// Resize resizes the terminal
func (t *Terminal) Resize(rows, cols uint16) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.isClosed {
		return fmt.Errorf("terminal is closed")
	}

	t.rows = rows
	t.cols = cols

	return pty.Setsize(t.ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

// Size returns the current terminal size
func (t *Terminal) Size() (rows, cols uint16) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.rows, t.cols
}

// OnOutput sets the output callback
func (t *Terminal) OnOutput(fn func([]byte)) {
	t.onOutput = fn
}

// OnExit sets the exit callback
func (t *Terminal) OnExit(fn func(int)) {
	t.onExit = fn
}

// Close closes the terminal
func (t *Terminal) Close() error {
	t.mu.Lock()
	if t.isClosed {
		t.mu.Unlock()
		return nil
	}
	t.isClosed = true
	t.mu.Unlock()

	// Send SIGHUP to the process group
	if t.cmd.Process != nil {
		t.cmd.Process.Signal(syscall.SIGHUP)
	}

	// Close PTY
	if t.ptmx != nil {
		t.ptmx.Close()
	}

	// Close input channel
	close(t.input)

	return nil
}

// IsClosed returns whether the terminal is closed
func (t *Terminal) IsClosed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.isClosed
}

// Manager manages multiple terminal sessions
type Manager struct {
	terminals map[string]*Terminal
	mu        sync.RWMutex
}

// NewManager creates a new terminal manager
func NewManager() *Manager {
	return &Manager{
		terminals: make(map[string]*Terminal),
	}
}

// Create creates a new terminal
func (m *Manager) Create(id string, cfg Config) (*Terminal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.terminals[id]; exists {
		return nil, fmt.Errorf("terminal %s already exists", id)
	}

	t, err := New(id, cfg)
	if err != nil {
		return nil, err
	}

	m.terminals[id] = t
	return t, nil
}

// Get returns a terminal by ID
func (m *Manager) Get(id string) (*Terminal, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.terminals[id]
	return t, ok
}

// List returns all terminal IDs
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.terminals))
	for id := range m.terminals {
		ids = append(ids, id)
	}
	return ids
}

// Close closes a terminal by ID
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.terminals[id]
	if !ok {
		return fmt.Errorf("terminal %s not found", id)
	}

	delete(m.terminals, id)
	return t.Close()
}

// CloseAll closes all terminals
func (m *Manager) CloseAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, t := range m.terminals {
		t.Close()
		delete(m.terminals, id)
	}
	return nil
}

// Count returns the number of active terminals
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.terminals)
}
