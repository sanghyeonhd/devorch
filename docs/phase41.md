# Phase 41: 도구(Tool) 시스템 확장

## 목표
OpenCode 패턴을 참고하여 AI 에이전트가 사용할 수 있는 도구 시스템을 구현합니다.

## 구현 내용

### 1. 도구 인터페이스 개선

**internal/tool/types.go**
```go
type Tool interface {
    Info() ToolInfo           // 도구 메타데이터
    Execute(ctx *Context, params json.RawMessage) (*Result, error)
}

type ToolInfo struct {
    Name        string
    Description string
    Parameters  json.RawMessage  // JSON Schema
}

type Result struct {
    Output      string
    Title       string
    Truncated   bool
    ExitCode    int
    Metadata    map[string]any
    Attachments []Attachment
}
```

### 2. 도구 컨텍스트

**internal/tool/context.go**
```go
type Context struct {
    Ctx         context.Context
    SessionID   string
    MessageID   string
    WorkspaceID string
    WorkDir     string
    Abort       <-chan struct{}
    Extra       map[string]any
}
```

### 3. 내장 도구

| 도구 | 파일 | 설명 |
|------|------|------|
| bash | builtin/bash.go | 쉘 명령어 실행 |
| read | builtin/read.go | 파일 읽기 (이미지/PDF 지원) |
| write | builtin/write.go | 파일 쓰기 |
| edit | builtin/edit.go | 파일 부분 수정 |
| glob | builtin/glob.go | 파일 패턴 검색 |
| grep | builtin/grep.go | 텍스트 검색 |

### 4. 레지스트리 개선

**internal/tool/registry.go**
- 새로운 Tool 인터페이스와 레거시 인터페이스 모두 지원
- 도구 목록 조회 기능
- 도구별 메타데이터 관리

## 파일 목록

| 파일 | 라인 | 설명 |
|------|------|------|
| internal/tool/types.go | 60 | 도구 인터페이스 및 타입 |
| internal/tool/context.go | 40 | 실행 컨텍스트 |
| internal/tool/truncation.go | 90 | 출력 트런케이션 유틸 |
| internal/tool/registry.go | 150 | 도구 레지스트리 |
| internal/tool/builtin/bash.go | 220 | Bash 도구 |
| internal/tool/builtin/read.go | 310 | Read 도구 |
| internal/tool/builtin/write.go | 150 | Write 도구 |
| internal/tool/builtin/edit.go | 170 | Edit 도구 |
| internal/tool/builtin/glob.go | 200 | Glob 도구 |
| internal/tool/builtin/grep.go | 220 | Grep 도구 |

## 사용 예시

```go
// 레지스트리 생성
registry := tool.NewRegistry()
registry.WorkDir = "/path/to/project"

// 도구 등록
registry.Register(&builtin.BashTool{})
registry.Register(&builtin.ReadTool{})
registry.Register(&builtin.WriteTool{})
registry.Register(&builtin.EditTool{})
registry.Register(&builtin.GlobTool{})
registry.Register(&builtin.GrepTool{})

// 도구 실행
ctx := context.Background()
result, err := registry.Execute(ctx, "workspace", "session", "task", "bash", 
    json.RawMessage(`{"command": "ls -la"}`))
```

## 다음 단계

- Hook 시스템 구현 (Phase 42)
- Agent 시스템 구현 (Phase 43)
- MCP 클라이언트 구현 (Phase 44)
