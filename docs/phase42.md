# Phase 42: Hook 시스템 구현

## 목표
라이프사이클 훅 시스템을 구현하여 도구 실행, 세션, 메시지 등의 이벤트를 처리합니다.

## 구현 내용

### 1. 이벤트 타입

**internal/hook/events.go**
```go
const (
    EventPreToolUse      EventType = "pre_tool_use"
    EventPostToolUse     EventType = "post_tool_use"
    EventPromptSubmit    EventType = "prompt_submit"
    EventMessageCreate   EventType = "message_create"
    EventSessionStart    EventType = "session_start"
    EventSessionStop     EventType = "session_stop"
    EventModelSelect     EventType = "model_select"
    EventOnModelSwitch   EventType = "on_model_switch"
    EventResponseStream  EventType = "response_stream"
    EventOnRetryExhausted EventType = "on_retry_exhausted"
    EventOnError         EventType = "on_error"
)
```

### 2. Hook 인터페이스

**internal/hook/hook.go**
```go
type Hook interface {
    Name() string
    Events() []EventType
    Execute(ctx context.Context, event EventType, data any) (any, error)
}

// 이벤트별 데이터 타입
type PreToolUseData struct { ... }
type PostToolUseData struct { ... }
type PromptSubmitData struct { ... }
type SessionStartData struct { ... }
type SessionStopData struct { ... }
type ModelSelectData struct { ... }
```

### 3. Hook 레지스트리

**internal/hook/registry.go**
```go
type Registry struct {
    hooks map[EventType][]Hook
}

func (r *Registry) Register(h Hook)
func (r *Registry) Dispatch(ctx, event, data) (any, error)
func (r *Registry) DispatchAsync(event, data)
```

### 4. 내장 훅

| 훅 | 파일 | 설명 |
|-----|------|------|
| comment_checker | builtins/comment_checker.go | AI 슬롭 패턴 감지 |
| logging | builtins/logging.go | 이벤트 로깅 |
| on_model_switch | builtins/on_model_switch.go | 모델 전환 기록 |
| on_retry_exhausted | builtins/on_retry_exhausted.go | 재시도 실패 기록 |

## 파일 목록

| 파일 | 라인 | 설명 |
|------|------|------|
| internal/hook/events.go | 65 | 이벤트 타입 정의 |
| internal/hook/hook.go | 80 | Hook 인터페이스 및 데이터 타입 |
| internal/hook/registry.go | 130 | Hook 레지스트리 |
| internal/hook/chain.go | 130 | Hook 체인 (기존 + 신규) |
| internal/hook/builtins/comment_checker.go | 110 | AI 슬롭 감지 훅 |
| internal/hook/builtins/logging.go | 70 | 로깅 훅 |

## 사용 예시

```go
// 레지스트리 생성
registry := hook.NewRegistry()

// 훅 등록
registry.Register(builtins.NewCommentCheckerHook(logger))
registry.Register(builtins.NewLoggingHook(logger))

// 함수형 훅 등록
registry.RegisterFunc("custom", []hook.EventType{hook.EventPreToolUse}, 
    func(ctx context.Context, event hook.EventType, data any) (any, error) {
        // 커스텀 로직
        return data, nil
    })

// 이벤트 디스패치
result, err := registry.Dispatch(ctx, hook.EventPreToolUse, &hook.PreToolUseData{
    SessionID: "session-123",
    ToolName:  "bash",
    Input:     json.RawMessage(`{"command": "ls"}`),
})
```

## AI 슬롭 감지 패턴

comment_checker 훅은 다음 패턴을 감지합니다:
- `// TODO: implement`
- `// add your code here`
- `// implementation goes here`
- `raise NotImplementedError`
- `throw new Error('Not implemented')`
- `// ... (existing code)`

## 다음 단계

- Agent 시스템 구현 (Phase 43)
- MCP 클라이언트 구현 (Phase 44)
