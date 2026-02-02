# Phase 43: Agent 시스템 구현

## 목표
멀티 에이전트 오케스트레이션 시스템을 구현하여 복잡한 작업을 여러 전문화된 에이전트가 협력하여 처리합니다.

## 구현 내용

### 1. Agent 정의

**internal/agent/agent.go**
```go
type Agent struct {
    Name         string
    Model        string
    Temperature  float64
    SystemPrompt string
    AllowedTools []string
    DeniedTools  []string
    MaxTokens    int
    Metadata     map[string]any
}

func (a *Agent) CanUseTool(tool string) bool
func (a *Agent) Clone() *Agent
func (a *Agent) WithModel(model string) *Agent
```

### 2. 내장 에이전트

**internal/agent/builtin.go**

| 에이전트 | 역할 | 모델 | 도구 |
|---------|------|------|------|
| orchestrator | 플래닝/조정 | claude-3-opus | 전체 |
| coder | 코드 구현 | claude-3-opus | bash, read, write, edit, glob, grep |
| oracle | 전략 조언/디버깅 | gpt-4-turbo | read, glob, grep |
| explorer | 빠른 탐색 | gpt-3.5-turbo | read, glob, grep |
| documenter | 문서화 | claude-3-sonnet | read, write, edit, glob |
| reviewer | 코드 리뷰 | claude-3-opus | read, glob, grep |

### 3. Agent 레지스트리

**internal/agent/registry.go**
```go
type Registry struct {
    agents map[string]*Agent
}

func (r *Registry) Register(a *Agent) error
func (r *Registry) Get(name string) (*Agent, bool)
func (r *Registry) List() []*Agent
func (r *Registry) GetByRole(role string) []*Agent
```

### 4. Orchestrator

**internal/agent/orchestrator.go**
```go
type Orchestrator struct {
    Registry      *Registry
    ExecuteFunc   func(ctx, agent, task) (*TaskResult, error)
    MaxConcurrent int
    TaskTimeout   time.Duration
}

func (o *Orchestrator) Execute(ctx, task) (*TaskResult, error)
func (o *Orchestrator) ExecutePlan(ctx, plan) ([]*TaskResult, error)
func (o *Orchestrator) DelegateTask(ctx, parent, agent, desc, input) (*TaskResult, error)
func (o *Orchestrator) SelectAgent(description string) *Agent
```

### 5. Task 관리

```go
type Task struct {
    ID          string
    AgentName   string
    ParentID    string
    Description string
    Input       json.RawMessage
    Status      TaskStatus  // pending, running, completed, failed
    Priority    int
    CreatedAt   time.Time
    Result      any
    Error       string
}

type TaskResult struct {
    TaskID    string
    AgentName string
    Success   bool
    Output    any
    Error     string
    Duration  time.Duration
    Subtasks  []*TaskResult
}
```

## 파일 목록

| 파일 | 라인 | 설명 |
|------|------|------|
| internal/agent/agent.go | 100 | Agent 타입 정의 |
| internal/agent/builtin.go | 170 | 내장 에이전트 정의 |
| internal/agent/registry.go | 100 | 에이전트 레지스트리 |
| internal/agent/orchestrator.go | 200 | 오케스트레이터 |

## 에이전트 선택 로직

Orchestrator.SelectAgent()는 키워드 기반으로 적합한 에이전트를 선택:

| 키워드 | 에이전트 |
|--------|----------|
| write, implement, create, fix | coder |
| debug, analyze | oracle |
| review | reviewer |
| search, find | explorer |
| document, readme | documenter |
| (기본) | orchestrator |

## 사용 예시

```go
// 레지스트리 생성
registry := agent.DefaultRegistry()

// 오케스트레이터 생성
orch := agent.NewOrchestrator(registry)
orch.ExecuteFunc = func(ctx context.Context, agent *agent.Agent, task *agent.Task) (*agent.TaskResult, error) {
    // LLM 호출 로직
    return &agent.TaskResult{Success: true}, nil
}

// 태스크 생성 및 실행
task := &agent.Task{
    ID:          "task-1",
    AgentName:   "coder",
    Description: "Implement a new feature",
    Input:       json.RawMessage(`{"prompt": "Create a function..."}`),
}

result, err := orch.Execute(ctx, task)
```

## 다음 단계

- MCP 클라이언트 구현 (Phase 44)
- TUI 기본 구조 (Phase 45+)
