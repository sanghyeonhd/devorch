# Phase 44: MCP 클라이언트 구현

## 목표
Model Context Protocol (MCP) 클라이언트를 구현하여 외부 MCP 서버의 도구, 리소스, 프롬프트를 사용할 수 있게 합니다.

## 구현 내용

### 1. MCP 프로토콜 타입

**internal/mcp/types.go**

JSON-RPC 2.0 기반:
```go
type Request struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      any             `json:"id,omitempty"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      any             `json:"id,omitempty"`
    Result  json.RawMessage `json:"result,omitempty"`
    Error   *ErrorObject    `json:"error,omitempty"`
}
```

MCP 타입:
- ServerInfo, ClientInfo
- InitializeParams, InitializeResult
- Capabilities (Client/Server)
- Tool, Resource, Prompt
- Content types (text, image, resource)

### 2. Transport 인터페이스

**internal/mcp/transport.go**
```go
type Transport interface {
    Send(ctx context.Context, req *Request) (*Response, error)
    Notify(ctx context.Context, notif *Notification) error
    Close() error
}

// StdioTransport - stdin/stdout 통신
type StdioTransport struct {
    cmd     *exec.Cmd
    stdin   io.WriteCloser
    stdout  io.ReadCloser
    scanner *bufio.Scanner
    pending map[any]chan *Response
}
```

### 3. MCP Client

**internal/mcp/client.go**
```go
type Client struct {
    transport Transport
    serverInfo *ServerInfo
    serverCaps *ServerCapabilities
}

// 초기화
func (c *Client) Initialize(ctx, clientInfo) error

// 도구
func (c *Client) ListTools(ctx) ([]Tool, error)
func (c *Client) CallTool(ctx, name, args) (*ToolCallResult, error)

// 리소스
func (c *Client) ListResources(ctx) ([]Resource, error)
func (c *Client) ReadResource(ctx, uri) (*ResourceReadResult, error)

// 프롬프트
func (c *Client) ListPrompts(ctx) ([]Prompt, error)
func (c *Client) GetPrompt(ctx, name, args) (*PromptGetResult, error)
```

### 4. MCP Manager

**internal/mcp/manager.go**
```go
type Manager struct {
    config  *Config
    clients map[string]*Client
}

// 설정 관리
func (m *Manager) LoadConfig(path string) error
func (m *Manager) AddServer(name string, config ServerConfig)
func (m *Manager) RemoveServer(name string)

// 클라이언트 관리
func (m *Manager) GetClient(ctx, name) (*Client, error)
func (m *Manager) ListServers() []string

// 통합 API
func (m *Manager) ListAllTools(ctx) (map[string][]Tool, error)
func (m *Manager) CallTool(ctx, server, tool, args) (*ToolCallResult, error)
```

### 5. 설정 파일

**mcp.json**
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/dir"],
      "enabled": true
    },
    "websearch": {
      "url": "https://mcp.exa.ai",
      "enabled": true
    }
  }
}
```

## 파일 목록

| 파일 | 라인 | 설명 |
|------|------|------|
| internal/mcp/types.go | 210 | MCP 프로토콜 타입 |
| internal/mcp/transport.go | 130 | Transport 인터페이스 |
| internal/mcp/client.go | 280 | MCP 클라이언트 |
| internal/mcp/manager.go | 230 | MCP 서버 관리자 |

## MCP 프로토콜 버전

`ProtocolVersion = "2024-11-05"`

## Capabilities

### Client Capabilities
- roots.listChanged: 루트 디렉토리 변경 알림
- sampling: 샘플링 지원

### Server Capabilities
- logging: 로깅 지원
- prompts.listChanged: 프롬프트 목록 변경 알림
- resources.subscribe: 리소스 구독
- resources.listChanged: 리소스 목록 변경 알림
- tools.listChanged: 도구 목록 변경 알림

## 사용 예시

```go
// 매니저 생성
manager := mcp.NewManager()
manager.LoadDefaultConfig()

// 서버 추가
manager.AddServer("myserver", mcp.ServerConfig{
    Command: "node",
    Args:    []string{"mcp-server.js"},
    Enabled: true,
})

// 도구 목록 조회
allTools, err := manager.ListAllTools(ctx)
for server, tools := range allTools {
    for _, tool := range tools {
        fmt.Printf("[%s] %s: %s\n", server, tool.Name, tool.Description)
    }
}

// 도구 호출
result, err := manager.CallTool(ctx, "myserver", "read_file", 
    json.RawMessage(`{"path": "/path/to/file"}`))
```

## 다음 단계

- TUI 기본 구조 (Phase 45)
- Provider Auth 개선 (Phase 46)
