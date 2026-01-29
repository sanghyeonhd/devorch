package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Client is an MCP protocol client
type Client struct {
	transport Transport

	// Server info after initialization
	serverInfo  *ServerInfo
	serverCaps  *ServerCapabilities
	initialized bool

	// Cached data
	tools     []Tool
	resources []Resource
	prompts   []Prompt

	mu sync.RWMutex
}

// NewClient creates a new MCP client
func NewClient(transport Transport) *Client {
	return &Client{
		transport: transport,
	}
}

// Initialize performs the MCP initialization handshake
func (c *Client) Initialize(ctx context.Context, clientInfo ClientInfo) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	params := InitializeParams{
		ProtocolVersion: ProtocolVersion,
		Capabilities: ClientCapabilities{
			Roots: &RootsCapability{
				ListChanged: true,
			},
		},
		ClientInfo: clientInfo,
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	req := &Request{
		JSONRPC: "2.0",
		Method:  "initialize",
		Params:  paramsJSON,
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s (code %d)", resp.Error.Message, resp.Error.Code)
	}

	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	c.serverInfo = &result.ServerInfo
	c.serverCaps = &result.Capabilities
	c.initialized = true

	// Send initialized notification
	notif := &Notification{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	c.transport.Notify(ctx, notif)

	return nil
}

// ServerInfo returns server information
func (c *Client) ServerInfo() *ServerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverInfo
}

// ServerCapabilities returns server capabilities
func (c *Client) ServerCapabilities() *ServerCapabilities {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverCaps
}

// ListTools returns available tools from the server
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	req := &Request{
		JSONRPC: "2.0",
		Method:  "tools/list",
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tools/list request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s", resp.Error.Message)
	}

	var result ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools list: %w", err)
	}

	c.mu.Lock()
	c.tools = result.Tools
	c.mu.Unlock()

	return result.Tools, nil
}

// CallTool invokes a tool on the server
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (*ToolCallResult, error) {
	params := ToolCallParams{
		Name:      name,
		Arguments: arguments,
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	req := &Request{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params:  paramsJSON,
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tools/call request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/call error: %s", resp.Error.Message)
	}

	var result ToolCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	return &result, nil
}

// ListResources returns available resources from the server
func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
	req := &Request{
		JSONRPC: "2.0",
		Method:  "resources/list",
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("resources/list request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("resources/list error: %s", resp.Error.Message)
	}

	var result ResourcesListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse resources list: %w", err)
	}

	c.mu.Lock()
	c.resources = result.Resources
	c.mu.Unlock()

	return result.Resources, nil
}

// ReadResource reads a resource from the server
func (c *Client) ReadResource(ctx context.Context, uri string) (*ResourceReadResult, error) {
	params := ResourceReadParams{
		URI: uri,
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	req := &Request{
		JSONRPC: "2.0",
		Method:  "resources/read",
		Params:  paramsJSON,
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("resources/read request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("resources/read error: %s", resp.Error.Message)
	}

	var result ResourceReadResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse resource content: %w", err)
	}

	return &result, nil
}

// ListPrompts returns available prompts from the server
func (c *Client) ListPrompts(ctx context.Context) ([]Prompt, error) {
	req := &Request{
		JSONRPC: "2.0",
		Method:  "prompts/list",
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("prompts/list request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("prompts/list error: %s", resp.Error.Message)
	}

	var result PromptsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse prompts list: %w", err)
	}

	c.mu.Lock()
	c.prompts = result.Prompts
	c.mu.Unlock()

	return result.Prompts, nil
}

// GetPrompt gets a prompt with arguments
func (c *Client) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*PromptGetResult, error) {
	params := PromptGetParams{
		Name:      name,
		Arguments: arguments,
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	req := &Request{
		JSONRPC: "2.0",
		Method:  "prompts/get",
		Params:  paramsJSON,
	}

	resp, err := c.transport.Send(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("prompts/get request failed: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("prompts/get error: %s", resp.Error.Message)
	}

	var result PromptGetResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse prompt result: %w", err)
	}

	return &result, nil
}

// Close closes the client and underlying transport
func (c *Client) Close() error {
	return c.transport.Close()
}
