# devorch 개선점 분석 보고서

> **분석 일자**: 2026년 1월 29일  
> **비교 대상**: OpenCode (v1.1.36), oh-my-opencode (v3.x)  
> **현재 버전**: devorch Phase 1-40 구현 완료

---

## 1. 개요

본 문서는 OpenCode와 oh-my-opencode 프로젝트를 분석하여 devorch의 개선점과 참고할 코드를 정리합니다.

### 1.1 현재 devorch 상태

| 항목 | 현재 상태 |
|------|----------|
| Go 소스 파일 | 248개 |
| SQL 마이그레이션 | 22개 |
| CLI 명령어 | 10개 (doctor, providers, models, chat, bench, stats, login, login-status, logout, ollama-pull) |
| 지원 프로바이더 | 5개 (OpenAI, Anthropic, Google, OpenRouter, Ollama) |
| 크로스 플랫폼 | 6개 (darwin/linux/windows × amd64/arm64) |

### 1.2 OpenCode/oh-my-opencode 규모

| 항목 | OpenCode | oh-my-opencode |
|------|----------|----------------|
| 코드 라인 | ~40,626줄 | ~20,000줄+ |
| 언어 | TypeScript (Bun) | TypeScript |
| AI 프로바이더 | 20+ | 프로바이더 라우팅 |
| 에이전트 | 6개 기본 | 10개+ (Sisyphus, Oracle, Atlas 등) |
| 훅 시스템 | 이벤트 기반 | 31개 라이프사이클 훅 |

---

## 2. 핵심 개선 영역

### 2.1 🔴 긴급 (High Priority)

#### 2.1.1 TUI (터미널 UI) 추가

**현재**: CLI 명령어만 지원  
**목표**: 대화형 터미널 UI

**참고 코드**:
- `opencode/packages/opencode/src/cli/cmd/tui/` - SolidJS 기반 TUI
- 32개 테마 지원 (`context/theme/*.json`)
- 키바인딩, 자동완성, 히스토리 기능

**구현 방안** (Go 기반):
```go
// 권장 라이브러리
// - github.com/charmbracelet/bubbletea (TUI 프레임워크)
// - github.com/charmbracelet/lipgloss (스타일링)
// - github.com/charmbracelet/bubbles (UI 컴포넌트)
```

**예상 작업량**: 2-3주

---

#### 2.1.2 에이전트 시스템 고도화

**현재**: 단순 채팅 (단일 LLM 호출)  
**목표**: 멀티 에이전트 오케스트레이션

**oh-my-opencode 에이전트 체계**:
| 에이전트 | 역할 | 모델 |
|---------|------|------|
| Sisyphus | 메인 오케스트레이터 | claude-opus-4-5 |
| Atlas | 플랜 실행 | claude-opus-4-5 |
| Oracle | 전략 조언, 디버깅 | gpt-5.2 |
| Librarian | 문서/코드 검색 | big-pickle |
| Explore | 빠른 탐색 | gpt-5-nano |
| Prometheus | 전략 플래닝 | claude-opus-4-5 |

**참고 코드**:
- `oh-my-opencode/src/agents/` - 에이전트 정의
- `oh-my-opencode/src/agents/sisyphus.ts` - 메인 오케스트레이터 (615줄)
- `oh-my-opencode/src/shared/model-resolver.ts` - 3단계 모델 해석

**devorch 개선안**:
```go
// internal/agent/agent.go
type Agent struct {
    Name        string
    Model       string
    Temperature float64
    SystemPrompt string
    AllowedTools []string
    DeniedTools  []string
}

// 에이전트 정의 예시
var Agents = map[string]Agent{
    "orchestrator": {Model: "anthropic/claude-3-opus", Temperature: 0.1},
    "oracle":       {Model: "openai/gpt-4-turbo", Temperature: 0.1},
    "explorer":     {Model: "openai/gpt-3.5-turbo", Temperature: 0.1},
}
```

**예상 작업량**: 3-4주

---

#### 2.1.3 MCP (Model Context Protocol) 지원

**현재**: 미구현  
**목표**: MCP 클라이언트/서버 지원

**OpenCode MCP 기능**:
- OAuth 콜백 지원
- 지연 로딩 (첫 호출 시 클라이언트 생성)
- stdio, http (SSE/Streamable) 전송

**참고 코드**:
- `opencode/packages/opencode/src/mcp/` - MCP 구현
  - `index.ts` - MCP 클라이언트
  - `auth.ts` - MCP 인증
  - `oauth-callback.ts` - OAuth 콜백

**oh-my-opencode 내장 MCP**:
| MCP | URL | 용도 |
|-----|-----|------|
| websearch | mcp.exa.ai | 웹 검색 |
| context7 | mcp.context7.com | 라이브러리 문서 |
| grep_app | mcp.grep.app | GitHub 코드 검색 |

**devorch 구현 방안**:
```go
// internal/mcp/client.go
type MCPClient struct {
    Transport MCPTransport  // stdio, http
    Tools     []Tool
}

// internal/mcp/server.go
type MCPServer struct {
    Addr     string
    Tools    []Tool
    OAuth    *OAuthConfig
}
```

**예상 작업량**: 2주

---

### 2.2 🟡 중요 (Medium Priority)

#### 2.2.1 훅 (Hook) 시스템

**현재**: 미구현  
**목표**: 라이프사이클 훅으로 확장성 제공

**oh-my-opencode 훅 이벤트**:
| 이벤트 | 타이밍 | 용도 |
|--------|--------|------|
| PreToolUse | 도구 실행 전 | 입력 검증/수정 |
| PostToolUse | 도구 실행 후 | 출력 변환 |
| UserPromptSubmit | 프롬프트 제출 시 | 키워드 감지 |
| Stop | 세션 유휴 시 | 자동 계속 |

**참고 코드**:
- `oh-my-opencode/src/hooks/` - 31개 훅 구현
  - `atlas/` - 오케스트레이션 (773줄)
  - `comment-checker/` - AI 슬롭 방지
  - `rules-injector/` - 조건부 규칙 주입

**devorch 구현안**:
```go
// internal/hook/hook.go
type HookEvent string

const (
    HookPreToolUse    HookEvent = "pre_tool_use"
    HookPostToolUse   HookEvent = "post_tool_use"
    HookPromptSubmit  HookEvent = "prompt_submit"
    HookSessionStop   HookEvent = "session_stop"
)

type Hook interface {
    Name() string
    Events() []HookEvent
    Execute(ctx context.Context, event HookEvent, data any) (any, error)
}
```

**예상 작업량**: 1-2주

---

#### 2.2.2 도구 (Tool) 시스템 확장

**현재**: 기본 채팅만 지원  
**목표**: AI가 사용할 수 있는 도구 체계

**OpenCode 도구 목록**:
| 도구 | 역할 |
|------|------|
| bash | 쉘 명령어 실행 |
| read | 파일 읽기 |
| write | 파일 쓰기 |
| edit | 파일 부분 수정 |
| glob | 파일 패턴 검색 |
| grep | 텍스트 검색 |
| lsp | 언어 서버 쿼리 |
| websearch | 웹 검색 |

**참고 코드**:
- `opencode/packages/opencode/src/tool/` - 도구 구현
  - `tool.ts` - 도구 인터페이스
  - `registry.ts` - 도구 레지스트리
  - `bash.ts`, `read.ts`, `write.ts` 등

**예상 작업량**: 2-3주

---

#### 2.2.3 LSP (Language Server Protocol) 통합

**현재**: 미구현  
**목표**: 시맨틱 코드 인텔리전스

**oh-my-opencode LSP 도구**:
- `lsp_goto_definition` - 정의로 이동
- `lsp_find_references` - 참조 찾기
- `lsp_symbols` - 심볼 검색
- `lsp_diagnostics` - 진단 정보
- `lsp_prepare_rename` - 이름 변경 준비
- `lsp_rename` - 이름 변경 실행

**참고 코드**:
- `opencode/packages/opencode/src/lsp/` - LSP 구현
- `oh-my-opencode/src/tools/lsp/` - LSP 도구

**예상 작업량**: 2주

---

#### 2.2.4 세션 관리 고도화

**현재**: 기본 세션 (devorch.db에 저장)  
**목표**: 세션 내보내기/가져오기, 압축

**OpenCode 세션 기능**:
- 컨텍스트 압축 (compaction)
- 세션 내보내기/가져오기
- 요약 생성
- TODO 관리

**참고 코드**:
- `opencode/packages/opencode/src/session/`
  - `compaction.ts` - 컨텍스트 압축
  - `summary.ts` - 요약 생성
  - `todo.ts` - TODO 관리

**예상 작업량**: 1주

---

### 2.3 🟢 개선 (Lower Priority)

#### 2.3.1 웹 UI

**현재**: CLI만 지원  
**목표**: 웹 기반 인터페이스

**참고 코드**:
- `opencode/packages/app/` - SolidJS 웹 앱
- `opencode/packages/ui/` - 공유 UI 컴포넌트

**구현 방안**: Go embed로 정적 파일 제공 + API 서버

**예상 작업량**: 3-4주

---

#### 2.3.2 데스크톱 앱

**현재**: 미구현  
**목표**: 네이티브 데스크톱 앱

**참고**:
- OpenCode: Tauri 2.x (Rust 기반)
- 대안: Wails (Go 기반 데스크톱 프레임워크)

**예상 작업량**: 4-6주

---

#### 2.3.3 플러그인 시스템

**현재**: 미구현  
**목표**: 사용자 확장 가능한 플러그인

**OpenCode 플러그인 API**:
```typescript
type Plugin = (input: PluginInput) => Promise<Hooks>

interface Hooks {
  config?(config): Promise<Config>
  event?({ event }): Promise<void>
  "tool.execute.before"?(input): Promise<Input>
  "tool.execute.after"?(output): Promise<Output>
}
```

**참고 코드**:
- `opencode/packages/plugin/` - 플러그인 API
- `opencode/packages/opencode/src/plugin/` - 플러그인 로더

**예상 작업량**: 2주

---

#### 2.3.4 GitHub Action 통합

**현재**: 미구현  
**목표**: PR/이슈에서 AI 에이전트 호출

**OpenCode GitHub Action**:
- `/opencode` 또는 `/oc` 명령어로 호출
- PR 코드 리뷰, 이슈 수정

**참고 코드**:
- `opencode/github/` - GitHub Action 패키지

**예상 작업량**: 1-2주

---

## 3. 참고할 핵심 코드

### 3.1 모델 라우팅 (3단계 해석)

**파일**: `oh-my-opencode/src/shared/model-resolver.ts`

```
1단계: 사용자 오버라이드 (설정 파일)
    ↓ (없으면)
2단계: 프로바이더 우선순위 Fallback
    ↓ (소진 시)
3단계: 시스템 기본값
```

**devorch 적용**: 현재 OkAON Thompson Sampling과 결합 가능

---

### 3.2 delegate_task 라우팅

**파일**: `oh-my-opencode/src/tools/delegate-task/tools.ts` (1,039줄)

**핵심 개념**:
- 카테고리 기반 라우팅 (`quick`, `ultrabrain`, `visual-engineering`)
- 에이전트 직접 호출 (`oracle`, `explore`, `librarian`)
- 백그라운드 실행 옵션

**devorch 적용**: `internal/delegate/` 패키지 확장

---

### 3.3 백그라운드 에이전트 관리

**파일**: `oh-my-opencode/src/features/background-agent/manager.ts` (1,335줄)

**라이프사이클**:
```
launch → poll (2초 간격) → complete

안정성: 3회 연속 poll = idle
동시성: 프로바이더/모델별 제한
정리: 30분 TTL, 3분 stale 타임아웃
```

**devorch 적용**: `internal/background/` 패키지에 이미 기본 구조 있음, 고도화 필요

---

### 3.4 컨텍스트 압축 (Compaction)

**파일**: `opencode/packages/opencode/src/session/compaction.ts`

**기능**:
- 긴 대화 히스토리 요약
- 토큰 인식 잘라내기
- 중요 컨텍스트 보존

**devorch 적용**: 긴 세션에서 컨텍스트 윈도우 초과 방지

---

### 3.5 훅 체인 실행

**파일**: `oh-my-opencode/src/hooks/atlas/index.ts` (773줄)

**실행 순서 예시**:
```
chat.message 체인:
keywordDetector → claudeCodeHooks → autoSlashCommand → startWork → ralphLoop

tool.execute.before 체인:
claudeCodeHooks → nonInteractiveEnv → commentChecker → directoryAgentsInjector
```

---

## 4. 구현 로드맵

### Phase 41-45: 도구 시스템 (2주)
- [ ] Tool 인터페이스 정의
- [ ] 기본 도구 구현 (bash, read, write, edit, grep)
- [ ] Tool Registry

### Phase 46-50: TUI (3주)
- [ ] bubbletea 기반 TUI 프레임워크
- [ ] 대화형 채팅 UI
- [ ] 세션 목록/선택
- [ ] 테마 지원

### Phase 51-55: 에이전트 시스템 (3주)
- [ ] Agent 정의 구조
- [ ] 멀티 에이전트 오케스트레이션
- [ ] delegate_task 구현
- [ ] 백그라운드 에이전트 고도화

### Phase 56-60: 훅 시스템 (2주)
- [ ] Hook 인터페이스
- [ ] 라이프사이클 이벤트
- [ ] 기본 훅 구현

### Phase 61-65: MCP/LSP (2주)
- [ ] MCP 클라이언트
- [ ] MCP 서버
- [ ] LSP 통합

### Phase 66-70: 웹 UI (3주)
- [ ] API 서버 확장
- [ ] 프론트엔드 구현
- [ ] 웹소켓/SSE 스트리밍

---

## 5. 결론

devorch는 Phase 1-40에서 핵심 인프라(OkAON, 라우팅, 벤치마크)를 완성했습니다.  
OpenCode/oh-my-opencode와 비교하여 다음 영역에서 개선이 필요합니다:

| 영역 | 우선순위 | 예상 기간 |
|------|----------|----------|
| TUI | 🔴 긴급 | 3주 |
| 에이전트 시스템 | 🔴 긴급 | 3-4주 |
| MCP 지원 | 🔴 긴급 | 2주 |
| 훅 시스템 | 🟡 중요 | 1-2주 |
| 도구 시스템 | 🟡 중요 | 2-3주 |
| LSP 통합 | 🟡 중요 | 2주 |
| 웹 UI | 🟢 개선 | 3-4주 |

**총 예상 기간**: 16-20주 (Phase 41-70)

Go 언어의 장점(컴파일 속도, 단일 바이너리, 크로스 플랫폼)을 유지하면서,  
TypeScript 기반 OpenCode의 기능을 점진적으로 구현하는 것을 권장합니다.
