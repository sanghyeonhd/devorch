// Package lsp provides Language Server Protocol integration for code intelligence.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Client represents an LSP client that communicates with a language server.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	mu        sync.Mutex
	requestID atomic.Int64
	pending   map[int64]chan *Response

	initialized  bool
	capabilities ServerCapabilities

	rootURI string
}

// NewClient creates a new LSP client for the given language server command.
func NewClient(command string, args ...string) *Client {
	return &Client{
		cmd:     exec.Command(command, args...),
		pending: make(map[int64]chan *Response),
	}
}

// Start starts the language server process.
func (c *Client) Start(ctx context.Context) error {
	var err error

	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start language server: %w", err)
	}

	// Start reading responses
	go c.readResponses(ctx)

	return nil
}

// Initialize sends the initialize request to the server.
func (c *Client) Initialize(ctx context.Context, rootURI string) error {
	c.rootURI = rootURI

	params := InitializeParams{
		ProcessID: nil, // null means the server should exit when client exits
		RootURI:   rootURI,
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				Synchronization: TextDocumentSyncClientCapabilities{
					DynamicRegistration: true,
					WillSave:            true,
					WillSaveWaitUntil:   true,
					DidSave:             true,
				},
				Completion: CompletionClientCapabilities{
					DynamicRegistration: true,
					CompletionItem: CompletionItemCapabilities{
						SnippetSupport:      true,
						DocumentationFormat: []string{"markdown", "plaintext"},
						DeprecatedSupport:   true,
						ResolveSupport:      &CompletionItemResolveSupport{Properties: []string{"documentation", "detail"}},
					},
				},
				Hover: HoverClientCapabilities{
					DynamicRegistration: true,
					ContentFormat:       []string{"markdown", "plaintext"},
				},
				Definition: DefinitionClientCapabilities{
					DynamicRegistration: true,
					LinkSupport:         true,
				},
				References: ReferencesClientCapabilities{
					DynamicRegistration: true,
				},
				DocumentSymbol: DocumentSymbolClientCapabilities{
					DynamicRegistration:               true,
					HierarchicalDocumentSymbolSupport: true,
				},
			},
			Workspace: WorkspaceClientCapabilities{
				ApplyEdit: true,
				WorkspaceEdit: WorkspaceEditClientCapabilities{
					DocumentChanges: true,
				},
				Symbol: WorkspaceSymbolClientCapabilities{
					DynamicRegistration: true,
				},
			},
		},
	}

	resp, err := c.request(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("failed to unmarshal initialize result: %w", err)
	}

	c.capabilities = result.Capabilities

	// Send initialized notification
	if err := c.notify("initialized", struct{}{}); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	c.initialized = true
	return nil
}

// Shutdown sends the shutdown request to the server.
func (c *Client) Shutdown(ctx context.Context) error {
	_, err := c.request(ctx, "shutdown", nil)
	return err
}

// Exit sends the exit notification to the server.
func (c *Client) Exit() error {
	return c.notify("exit", nil)
}

// Close shuts down and closes the connection.
func (c *Client) Close(ctx context.Context) error {
	if c.initialized {
		_ = c.Shutdown(ctx)
		_ = c.Exit()
	}

	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

// request sends a request and waits for response.
func (c *Client) request(ctx context.Context, method string, params any) (*Response, error) {
	id := c.requestID.Add(1)

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	respChan := make(chan *Response, 1)
	c.mu.Lock()
	c.pending[id] = respChan
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.send(req); err != nil {
		return nil, err
	}

	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("LSP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// notify sends a notification (no response expected).
func (c *Client) notify(method string, params any) error {
	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.send(req)
}

// send writes a message to the server.
func (c *Client) send(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := c.stdin.Write(data); err != nil {
		return err
	}

	return nil
}

// readResponses continuously reads responses from the server.
func (c *Client) readResponses(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		resp, err := c.readMessage()
		if err != nil {
			return
		}

		// If it has an ID, it's a response to our request
		if resp.ID > 0 {
			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			c.mu.Unlock()

			if ok {
				ch <- resp
			}
		}
		// Otherwise it might be a notification from server
	}
}

// readMessage reads a single LSP message from stdout.
func (c *Client) readMessage() (*Response, error) {
	// Read headers
	var contentLength int
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}

		if line == "" {
			break // End of headers
		}

		var n int
		if _, err := fmt.Sscanf(line, "Content-Length: %d", &n); err == nil {
			contentLength = n
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length header")
	}

	// Read body
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.stdout, body); err != nil {
		return nil, err
	}

	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// readLine reads a single line from stdout.
func (c *Client) readLine() (string, error) {
	var line []byte
	buf := make([]byte, 1)

	for {
		_, err := c.stdout.Read(buf)
		if err != nil {
			return "", err
		}

		if buf[0] == '\n' {
			break
		}
		if buf[0] != '\r' {
			line = append(line, buf[0])
		}
	}

	return string(line), nil
}
