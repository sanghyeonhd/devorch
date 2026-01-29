package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Transport defines the interface for MCP communication
type Transport interface {
	// Send sends a request and returns the response
	Send(ctx context.Context, req *Request) (*Response, error)

	// Notify sends a notification (no response expected)
	Notify(ctx context.Context, notif *Notification) error

	// Close closes the transport
	Close() error
}

// StdioTransport communicates with MCP server via stdin/stdout
type StdioTransport struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	scanner *bufio.Scanner

	mu      sync.Mutex
	nextID  int
	pending map[any]chan *Response
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(command string, args ...string) (*StdioTransport, error) {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	t := &StdioTransport{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		scanner: bufio.NewScanner(stdout),
		pending: make(map[any]chan *Response),
	}

	// Start response reader
	go t.readResponses()

	return t, nil
}

// Send sends a request and waits for response
func (t *StdioTransport) Send(ctx context.Context, req *Request) (*Response, error) {
	t.mu.Lock()

	// Assign ID if not set
	if req.ID == nil {
		t.nextID++
		req.ID = t.nextID
	}

	// Create response channel
	respChan := make(chan *Response, 1)
	t.pending[req.ID] = respChan

	// Send request
	data, err := json.Marshal(req)
	if err != nil {
		delete(t.pending, req.ID)
		t.mu.Unlock()
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	_, err = t.stdin.Write(append(data, '\n'))
	t.mu.Unlock()

	if err != nil {
		t.mu.Lock()
		delete(t.pending, req.ID)
		t.mu.Unlock()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.pending, req.ID)
		t.mu.Unlock()
		return nil, ctx.Err()
	case resp := <-respChan:
		return resp, nil
	}
}

// Notify sends a notification
func (t *StdioTransport) Notify(ctx context.Context, notif *Notification) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	_, err = t.stdin.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}

// Close closes the transport
func (t *StdioTransport) Close() error {
	t.stdin.Close()
	t.stdout.Close()
	return t.cmd.Process.Kill()
}

// readResponses reads responses from stdout
func (t *StdioTransport) readResponses() {
	for t.scanner.Scan() {
		line := t.scanner.Bytes()

		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // Skip invalid responses
		}

		// Find pending request
		t.mu.Lock()
		if ch, ok := t.pending[resp.ID]; ok {
			ch <- &resp
			delete(t.pending, resp.ID)
		}
		t.mu.Unlock()
	}
}
