//go:build ignore

// Package lsp provides Language Server Protocol client implementation
package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Client represents an LSP client connection
type Client struct {
	cmd           *exec.Cmd
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	requestID     int64
	pendingReqs   map[int64]chan *Response
	pendingMu     sync.RWMutex
	notifications chan *Notification
	initialized   bool
	rootPath      string
	language      string
}

// Message represents a JSON-RPC message
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Response represents an LSP response
type Response struct {
	ID     int64
	Result json.RawMessage
	Error  *Error
}

// Notification represents an LSP notification
type Notification struct {
	Method string
	Params json.RawMessage
}

// Diagnostic represents an LSP diagnostic
type Diagnostic struct {
	Range       Range                `json:"range"`
	Severity    int                  `json:"severity"`
	Code        string               `json:"code,omitempty"`
	Source      string               `json:"source,omitempty"`
	Message     string               `json:"message"`
	RelatedInfo []RelatedInformation `json:"relatedInformation,omitempty"`
}

// Range represents a text range
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a text position
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// RelatedInformation represents related diagnostic information
type RelatedInformation struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}

// Location represents a location in a file
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// CompletionItem represents a completion suggestion
type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind"`
	Detail        string `json:"detail,omitempty"`
	Documentation any    `json:"documentation,omitempty"`
	InsertText    string `json:"insertText,omitempty"`
}

// Hover represents hover information
type Hover struct {
	Contents any    `json:"contents"`
	Range    *Range `json:"range,omitempty"`
}

// Symbol represents a document symbol
type Symbol struct {
	Name           string   `json:"name"`
	Kind           int      `json:"kind"`
	Range          Range    `json:"range"`
	SelectionRange Range    `json:"selectionRange"`
	Children       []Symbol `json:"children,omitempty"`
}

// DiagnosticSeverity constants
const (
	SeverityError       = 1
	SeverityWarning     = 2
	SeverityInformation = 3
	SeverityHint        = 4
)

// LanguageServer maps languages to their LSP servers
var LanguageServers = map[string][]string{
	"go":         {"gopls"},
	"python":     {"pylsp"},
	"typescript": {"typescript-language-server", "--stdio"},
	"javascript": {"typescript-language-server", "--stdio"},
	"rust":       {"rust-analyzer"},
	"c":          {"clangd"},
	"cpp":        {"clangd"},
	"java":       {"jdtls"},
	"ruby":       {"solargraph", "stdio"},
	"php":        {"phpactor", "language-server"},
}

// NewClient creates a new LSP client for a language
func NewClient(language, rootPath string) (*Client, error) {
	serverArgs, ok := LanguageServers[language]
	if !ok {
		return nil, fmt.Errorf("no LSP server configured for %s", language)
	}

	// Check if server exists
	serverPath, err := exec.LookPath(serverArgs[0])
	if err != nil {
		return nil, fmt.Errorf("LSP server not found: %s", serverArgs[0])
	}

	var args []string
	if len(serverArgs) > 1 {
		args = serverArgs[1:]
	}

	cmd := exec.Command(serverPath, args...)
	cmd.Dir = rootPath

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	client := &Client{
		cmd:           cmd,
		stdin:         stdin,
		stdout:        stdout,
		pendingReqs:   make(map[int64]chan *Response),
		notifications: make(chan *Notification, 100),
		rootPath:      rootPath,
		language:      language,
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Start reading responses
	go client.readMessages()

	return client, nil
}

// Initialize initializes the LSP server
func (c *Client) Initialize(ctx context.Context) error {
	absPath, _ := filepath.Abs(c.rootPath)

	params := map[string]any{
		"processId": os.Getpid(),
		"rootUri":   "file://" + absPath,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"completion": map[string]any{
					"completionItem": map[string]any{
						"snippetSupport": true,
					},
				},
				"hover": map[string]any{
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"publishDiagnostics": map[string]any{
					"relatedInformation": true,
				},
			},
			"workspace": map[string]any{
				"workspaceFolders": true,
			},
		},
		"workspaceFolders": []map[string]string{
			{"uri": "file://" + absPath, "name": filepath.Base(absPath)},
		},
	}

	_, err := c.call(ctx, "initialize", params)
	if err != nil {
		return err
	}

	// Send initialized notification
	c.notify("initialized", struct{}{})

	c.initialized = true
	return nil
}

// Shutdown shuts down the LSP server
func (c *Client) Shutdown(ctx context.Context) error {
	if !c.initialized {
		return nil
	}

	_, err := c.call(ctx, "shutdown", nil)
	if err != nil {
		return err
	}

	c.notify("exit", nil)
	return c.cmd.Wait()
}

// DidOpen notifies the server that a document was opened
func (c *Client) DidOpen(uri, languageID, content string) {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": languageID,
			"version":    1,
			"text":       content,
		},
	}
	c.notify("textDocument/didOpen", params)
}

// DidChange notifies the server of document changes
func (c *Client) DidChange(uri, content string, version int) {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri":     uri,
			"version": version,
		},
		"contentChanges": []map[string]any{
			{"text": content},
		},
	}
	c.notify("textDocument/didChange", params)
}

// DidClose notifies the server that a document was closed
func (c *Client) DidClose(uri string) {
	params := map[string]any{
		"textDocument": map[string]any{
			"uri": uri,
		},
	}
	c.notify("textDocument/didClose", params)
}

// GetDiagnostics returns diagnostics for a file
func (c *Client) GetDiagnostics(ctx context.Context, uri string) ([]Diagnostic, error) {
	// Diagnostics are typically pushed via notifications
	// This is a placeholder for direct diagnostic requests if supported
	return nil, fmt.Errorf("diagnostics are received via notifications")
}

// Completion requests completions at a position
func (c *Client) Completion(ctx context.Context, uri string, line, character int) ([]CompletionItem, error) {
	params := map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": character},
	}

	result, err := c.call(ctx, "textDocument/completion", params)
	if err != nil {
		return nil, err
	}

	// Handle both array and CompletionList responses
	var items []CompletionItem

	// Try direct array first
	if err := json.Unmarshal(result, &items); err == nil {
		return items, nil
	}

	// Try CompletionList
	var list struct {
		Items []CompletionItem `json:"items"`
	}
	if err := json.Unmarshal(result, &list); err != nil {
		return nil, err
	}

	return list.Items, nil
}

// Hover requests hover information at a position
func (c *Client) Hover(ctx context.Context, uri string, line, character int) (*Hover, error) {
	params := map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": character},
	}

	result, err := c.call(ctx, "textDocument/hover", params)
	if err != nil {
		return nil, err
	}

	var hover Hover
	if err := json.Unmarshal(result, &hover); err != nil {
		return nil, err
	}

	return &hover, nil
}

// Definition requests the definition of a symbol
func (c *Client) Definition(ctx context.Context, uri string, line, character int) ([]Location, error) {
	params := map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": character},
	}

	result, err := c.call(ctx, "textDocument/definition", params)
	if err != nil {
		return nil, err
	}

	// Handle single location or array
	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		var loc Location
		if err := json.Unmarshal(result, &loc); err != nil {
			return nil, err
		}
		locations = []Location{loc}
	}

	return locations, nil
}

// References requests references to a symbol
func (c *Client) References(ctx context.Context, uri string, line, character int, includeDeclaration bool) ([]Location, error) {
	params := map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": character},
		"context":      map[string]any{"includeDeclaration": includeDeclaration},
	}

	result, err := c.call(ctx, "textDocument/references", params)
	if err != nil {
		return nil, err
	}

	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		return nil, err
	}

	return locations, nil
}

// DocumentSymbols requests symbols in a document
func (c *Client) DocumentSymbols(ctx context.Context, uri string) ([]Symbol, error) {
	params := map[string]any{
		"textDocument": map[string]any{"uri": uri},
	}

	result, err := c.call(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}

	var symbols []Symbol
	if err := json.Unmarshal(result, &symbols); err != nil {
		return nil, err
	}

	return symbols, nil
}

// Notifications returns the channel for receiving notifications
func (c *Client) Notifications() <-chan *Notification {
	return c.notifications
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.requestID, 1)

	msg := Message{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
	}

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		msg.Params = data
	}

	// Create response channel
	respChan := make(chan *Response, 1)
	c.pendingMu.Lock()
	c.pendingReqs[id] = respChan
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pendingReqs, id)
		c.pendingMu.Unlock()
	}()

	// Send request
	if err := c.writeMessage(msg); err != nil {
		return nil, err
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("LSP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *Client) notify(method string, params any) error {
	msg := Message{
		JSONRPC: "2.0",
		Method:  method,
	}

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return err
		}
		msg.Params = data
	}

	return c.writeMessage(msg)
}

func (c *Client) writeMessage(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	_, err = c.stdin.Write([]byte(header))
	if err != nil {
		return err
	}

	_, err = c.stdin.Write(data)
	return err
}

func (c *Client) readMessages() {
	reader := bufio.NewReader(c.stdout)

	for {
		// Read headers
		contentLength := 0
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				break
			}

			if strings.HasPrefix(line, "Content-Length:") {
				lenStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
				contentLength, _ = strconv.Atoi(lenStr)
			}
		}

		if contentLength == 0 {
			continue
		}

		// Read body
		body := make([]byte, contentLength)
		_, err := io.ReadFull(reader, body)
		if err != nil {
			return
		}

		var msg Message
		if err := json.Unmarshal(body, &msg); err != nil {
			continue
		}

		// Handle response or notification
		if msg.ID != nil {
			c.pendingMu.RLock()
			respChan, ok := c.pendingReqs[*msg.ID]
			c.pendingMu.RUnlock()

			if ok {
				respChan <- &Response{
					ID:     *msg.ID,
					Result: msg.Result,
					Error:  msg.Error,
				}
			}
		} else if msg.Method != "" {
			select {
			case c.notifications <- &Notification{
				Method: msg.Method,
				Params: msg.Params,
			}:
			default:
				// Drop notification if channel is full
			}
		}
	}
}

// Manager manages multiple LSP clients
type Manager struct {
	clients  map[string]*Client
	clientMu sync.RWMutex
	rootPath string
}

// NewManager creates a new LSP manager
func NewManager(rootPath string) *Manager {
	return &Manager{
		clients:  make(map[string]*Client),
		rootPath: rootPath,
	}
}

// GetClient gets or creates an LSP client for a language
func (m *Manager) GetClient(language string) (*Client, error) {
	m.clientMu.Lock()
	defer m.clientMu.Unlock()

	if client, ok := m.clients[language]; ok {
		return client, nil
	}

	client, err := NewClient(language, m.rootPath)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := client.Initialize(ctx); err != nil {
		return nil, err
	}

	m.clients[language] = client
	return client, nil
}

// Shutdown shuts down all LSP clients
func (m *Manager) Shutdown() {
	m.clientMu.Lock()
	defer m.clientMu.Unlock()

	ctx := context.Background()
	for _, client := range m.clients {
		client.Shutdown(ctx)
	}
	m.clients = make(map[string]*Client)
}

// DetectLanguage detects the language from a file path
func DetectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs":
		return "javascript"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	default:
		return ""
	}
}

// FileURI converts a file path to a URI
func FileURI(path string) string {
	absPath, _ := filepath.Abs(path)
	return "file://" + absPath
}

// SeverityString returns a string representation of a severity
func SeverityString(severity int) string {
	switch severity {
	case SeverityError:
		return "Error"
	case SeverityWarning:
		return "Warning"
	case SeverityInformation:
		return "Information"
	case SeverityHint:
		return "Hint"
	default:
		return "Unknown"
	}
}
