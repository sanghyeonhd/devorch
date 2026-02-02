# Phase 53-54: Plugin System

## 개요
확장 가능한 플러그인 시스템을 구현하여 도구, 프로바이더, 훅, 에이전트를 동적으로 추가할 수 있습니다.

## 구현 파일

### internal/plugin/plugin.go
**Plugin 인터페이스:**
```go
type Plugin interface {
    Metadata() Metadata
    Initialize(ctx context.Context, host Host) error
    Shutdown(ctx context.Context) error
}
```

**Metadata 구조:**
```go
type Metadata struct {
    ID          string            // 고유 식별자
    Name        string            // 표시 이름
    Version     string            // 버전
    Description string            // 설명
    Author      string            // 작성자
    License     string            // 라이선스
    Homepage    string            // 홈페이지 URL
    Repository  string            // 저장소 URL
    Tags        []string          // 태그
    Provides    []string          // 제공하는 기능: "tool", "provider", "hook", "agent"
    Requires    []string          // 필요한 의존성
    Config      map[string]any    // 설정
}
```

**Host 인터페이스:**
플러그인이 DevOrch와 상호작용하기 위한 인터페이스:
```go
type Host interface {
    RegisterTool(name string, handler ToolHandler) error
    RegisterProvider(name string, provider LLMProvider) error
    RegisterHook(event string, handler HookHandler) error
    RegisterAgent(name string, agent Agent) error
    GetConfig(key string) (any, bool)
    SetConfig(key string, value any)
    Log(level, message string, args ...any)
    GetWorkspacePath() string
    GetDataPath() string
}
```

### internal/plugin/manager.go
**Manager 기능:**
```go
manager := plugin.NewManager(pluginsDir, dataDir, workspacePath)

// 전체 로드
manager.LoadAll(ctx)

// 단일 플러그인 로드
manager.Load(ctx, "/path/to/plugin")

// 플러그인 등록 (프로그래밍 방식)
manager.Register(ctx, myPlugin)

// 언로드
manager.Unload(ctx, "plugin-id")

// 활성화/비활성화
manager.Enable(ctx, "plugin-id")
manager.Disable(ctx, "plugin-id")

// 목록
plugins := manager.List()

// 도구/프로바이더/에이전트 접근
handler, ok := manager.GetTool("my_tool")
provider, ok := manager.GetProvider("my_provider")
agent, ok := manager.GetAgent("my_agent")

// 훅 디스패치
manager.DispatchHook(ctx, plugin.HookEvent{
    Type: "pre_tool_use",
    Data: map[string]any{"tool": "bash"},
})
```

### internal/plugin/builtins.go
내장 플러그인들:

**WebFetchPlugin:**
- ID: `devorch.web-fetch`
- 도구: `web_fetch` - URL에서 콘텐츠 가져오기

**FileWatchPlugin:**
- ID: `devorch.file-watch`
- 도구: `file_watch`, `file_unwatch` - 파일 변경 감시

**GitPlugin:**
- ID: `devorch.git`
- 도구: `git_status`, `git_diff`, `git_log`, `git_branch`

**ScriptPlugin:**
- ID: `devorch.script`
- 도구: `run_script` - 스크립트 실행

**JSONSchemaPlugin:**
- ID: `devorch.json-schema`
- 도구: `json_validate` - JSON 유효성 검사

## 플러그인 매니페스트
플러그인 디렉토리에 `plugin.json` 파일:
```json
{
    "metadata": {
        "id": "my-plugin",
        "name": "My Plugin",
        "version": "1.0.0",
        "description": "A custom plugin",
        "author": "Developer",
        "license": "MIT",
        "provides": ["tool"],
        "requires": []
    },
    "entry_point": "main.go",
    "permissions": ["network", "filesystem"]
}
```

## 플러그인 개발 예시
```go
package myplugin

import (
    "context"
    "devorch/internal/plugin"
)

type MyPlugin struct {
    host plugin.Host
}

func (p *MyPlugin) Metadata() plugin.Metadata {
    return plugin.Metadata{
        ID:          "my-plugin",
        Name:        "My Plugin",
        Version:     "1.0.0",
        Description: "Does something useful",
        Provides:    []string{"tool"},
    }
}

func (p *MyPlugin) Initialize(ctx context.Context, host plugin.Host) error {
    p.host = host
    
    // 도구 등록
    return host.RegisterTool("my_tool", p.myToolHandler)
}

func (p *MyPlugin) Shutdown(ctx context.Context) error {
    return nil
}

func (p *MyPlugin) myToolHandler(ctx context.Context, args map[string]any) (any, error) {
    input, _ := args["input"].(string)
    
    // 로깅
    p.host.Log("info", "Processing: %s", input)
    
    return map[string]any{
        "result": "processed: " + input,
    }, nil
}
```

## 내장 플러그인 사용
```go
import "devorch/internal/plugin"

// 매니저 생성
manager := plugin.NewManager(pluginsDir, dataDir, workspacePath)

// 내장 플러그인 등록
for _, p := range plugin.BuiltinPlugins() {
    manager.Register(ctx, p)
}

// 도구 사용
handler, ok := manager.GetTool("web_fetch")
if ok {
    result, err := handler(ctx, map[string]any{
        "url": "https://example.com",
    })
}
```

## 권한 모델
플러그인은 매니페스트에서 필요한 권한을 선언합니다:
- `network`: 네트워크 접근
- `filesystem`: 파일 시스템 접근
- `exec`: 외부 프로세스 실행
- `env`: 환경 변수 접근

## 확장 포인트
| 타입 | 설명 |
|---|---|
| tool | AI가 사용할 수 있는 도구 |
| provider | LLM 프로바이더 |
| hook | 이벤트 훅 |
| agent | 특화된 에이전트 |
