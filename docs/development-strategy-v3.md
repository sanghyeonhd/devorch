# DevOrch 개발 전략 통합 문서 v3

> **작성일**: 2026년 1월 30일  
> **기준**: OpenCode 분석 + Phase 1-57 구현 완료 + 전체 코드 전수 조사  
> **OpenCode 참조**: `docs/compare/opencode-analysis.md` (1,316줄), `docs/compare/productexplain.md` (302줄)

---

## 📊 현재 상태 분석 (2026.01.30 기준)

### 1. 프로젝트 규모 비교

| 항목 | DevOrch | OpenCode | 비고 |
|------|---------|----------|------|
| 소스 파일 | 330개 (.go) | 784개 (.ts) | - |
| 핵심 코드 | 11,580줄 (internal/tui/) | 40,626줄 (packages/opencode/) | 28% |
| CLI 명령어 | **52개** | 9개 | 5.7배 많음 |
| AI 프로바이더 | 40+개 | 20+개 | - |
| 테마 | 32개 | 32개 | 동일 |
| AI 도구 | 25개 (internal/tool/builtin/) | 19개 | - |
| LSP 통합 | ✅ 구현됨 | ✅ 구현됨 | - |
| WebSocket | ✅ 구현됨 | ✅ 구현됨 | - |

### 1.1 internal/tui/ 파일별 코드량

| 파일 | 줄 수 | 역할 |
|------|-------|------|
| cli_mode.go | 2,448 | CLI 모드 핵심 |
| theme.go | 2,326 | 32개 테마 정의 |
| command.go | 2,198 | TUI 명령어 핸들러 |
| model.go | 1,926 | TUI 상태 관리 |
| provider.go | 1,125 | 프로바이더 연결 |
| components.go | 323 | UI 컴포넌트 |
| **합계** | **11,580** | - |

### 2. OpenCode 9개 명령어 구현 상태

| 명령어 | 상태 | 구현 위치 | 완성도 | OpenCode 기능 |
|--------|------|-----------|--------|---------------|
| `/agents` | ✅ 완료 | `showAgents()` | 90% | 에이전트 선택 (build, plan, general, explore) |
| `/connect` | ✅ 완료 | `showConnect()` | 95% | 프로바이더 연결 + OAuth |
| `/editor` | ⚠️ 부분 | `showEditor()` | 20% | VS Code/Cursor 통합, 외부 에디터 열기 |
| `/init` | ✅ 완료 | `initProject()` | 85% | AGENTS.md 생성, 프로젝트 스캔 |
| `/mcps` | ⚠️ 부분 | `showMCPs()` | 40% | MCP 서버 목록/연결/OAuth |
| `/new` | ✅ 완료 | `startNewSession()` | 90% | 새 세션 생성 |
| `/review` | ✅ 완료 | `reviewCode()` | 85% | git diff + AI 리뷰 |
| `/sessions` | ✅ 완료 | `showSessions()` | 90% | 세션 목록/전환/내보내기 |
| `/status` | ✅ 완료 | `showStatus()` | 95% | 시스템 상태 (모델, 토큰, 프로젝트) |

### 3. 미구현/Placeholder 기능 (전수 조사 결과)

```
❌ "coming soon" 항목 (cli_mode.go):
1. Line 1270: showEditor() - "Editor integration coming soon"
2. Line 1963: showLSP() - "LSP integration coming soon"  
3. Line 2186: shareSession() - "Session sharing requires cloud sync - coming soon"

⚠️ 부분 구현 (기능은 있으나 OpenCode 수준 미달):
1. showMCPs() - UI 표시만, 실제 MCP 서버 연결 미구현
2. showEditor() - VS Code/Neovim 연결 미구현
3. showLSP() - LSP 클라이언트 코드는 있으나 CLI 연동 미완
```

### 4. 이미 구현된 기반 인프라 (전수 조사)

| 인프라 | 파일 | 상태 | 설명 |
|--------|------|------|------|
| **LSP 클라이언트** | internal/lsp/client.go | ✅ 321줄 | Initialize, GotoDefinition, Hover, References |
| **AI 도구 25개** | internal/tool/builtin/*.go | ✅ 완전 | bash, read, write, edit, grep, glob, lsp, websearch 등 |
| **WebSocket 허브** | internal/server/routes/websocket.go | ✅ 404줄 | 실시간 채팅, 스트리밍, 이벤트 |
| **SSE 스트리밍** | internal/server/routes/stream.go | ✅ 62줄 | Server-Sent Events |
| **백그라운드 작업** | internal/background/task_manager.go | ✅ 235줄 | 진행률 콜백, 취소, 이벤트 |
| **ProgressBar 컴포넌트** | internal/tui/components.go | ✅ 323줄 | UI 진행률 표시 |
| **Spinner 컴포넌트** | internal/tui/app.go | ✅ | 로딩 애니메이션 |

### 5. 중첩/중복 기능 분석

| 기능 | 위치1 | 위치2 | 상태 |
|------|-------|-------|------|
| Agent 전환 | `/agents` (showAgents) | `/agent` (setAgent) | ✅ 정상 - 다른 용도 |
| 모델 목록 | `/models` (listModels) | `/model` (setModel) | ✅ 정상 - 다른 용도 |
| 테마 목록 | `/themes` (listThemes) | `/theme` (setTheme) | ✅ 정상 - 다른 용도 |
| formatSize | cli_mode.go (cliFormatSize) | provider.go (formatSize) | ⚠️ 중복 - 이름 변경으로 해결 |

---

## 🏗️ 기존 인프라 분석

### 1. Multi-LLM 오케스트레이션 (internal/learning/)

**이미 구현됨** - 771줄의 `multi_llm.go`:

```go
// 전략 패턴 지원
StrategyBestSingle    // 최적 단일 모델 사용
StrategyParallel      // 병렬 실행, 최상 응답 선택
StrategyChain         // 모델 체인으로 정제
StrategyVoting        // 다중 모델 투표
StrategySpecialized   // 작업별 전문가 라우팅
StrategyAdaptive      // 동적 전략 선택
```

**Thompson Sampling** 기반 모델 선택:
- `Alpha/Beta` 분포로 탐색/활용 균형
- 작업별 성능 추적 (`TaskScores`)
- 사용자 프로필 기반 최적화

### 2. 앙상블 실행 (internal/learning/ensemble.go)

**584줄** 구현:
- `executeSingle()` - 단일 모델 실행
- `executeParallel()` - 병렬 실행 (goroutine)
- `executeChain()` - 순차 체인 실행
- `executeVoting()` - 합의 기반 투표

### 3. 피드백 시스템 (internal/learning/feedback.go)

```go
type Feedback struct {
    ThumbsUp     bool    // 사용자 평가
    QualityScore float64 // 0-1 품질 점수
}
```

### 4. 자동 설정 (internal/autosetup/)

**이미 구현됨**:
- 하드웨어 감지 (OS, Arch, RAM, GPU)
- 5단계 티어 분류 (ultra/high/mid/low/minimal)
- 모델 자동 추천
- Ollama 자동 설치 (macOS/Linux)
- 모델 자동 pull (`DEVORCH_AUTO_INSTALL=1`)

---

## 🎨 UX/UI 개선 필요 사항 분석

### 1. 모델 다운로드 진행률 표시

**현재 구현 상태** (internal/tui/provider.go):
```go
func InstallEssentialModels(progressCh chan<- string) error {
    // progressCh <- fmt.Sprintf("⬇ Downloading %s (%s)...", model.ID, model.Size)
}
```

**문제점**:
- 텍스트 기반 진행 메시지만 표시
- 실제 다운로드 퍼센트/속도 미표시
- Ollama의 pull progress를 파싱하지 않음

**OpenCode 방식** (참고: `productexplain.md`):
- 실시간 진행률 바 표시
- 예상 남은 시간 계산
- 중단/재시작 지원

**개선안**:
```go
// internal/tui/download_progress.go
type DownloadProgress struct {
    Model       string
    TotalBytes  int64
    Downloaded  int64
    Speed       float64  // bytes/sec
    ETA         time.Duration
}

func (c *CLIMode) installModelWithProgress(model string) {
    cmd := exec.Command("ollama", "pull", model)
    stdout, _ := cmd.StdoutPipe()
    
    // Ollama outputs: "pulling manifest..." "downloading sha256:xxx... 45% ▕████░░░░░░░░░░▏ 1.2 GB/2.7 GB"
    scanner := bufio.NewScanner(stdout)
    for scanner.Scan() {
        line := scanner.Text()
        if percent, speed := parseOllamaProgress(line); percent > 0 {
            fmt.Printf("\r⬇ %s: %s [%d%%] %.1f MB/s", 
                model, renderProgressBar(percent, 30), percent, speed/1e6)
        }
    }
}
```

### 2. 이미 구현된 UI 컴포넌트 (internal/tui/components.go)

| 컴포넌트 | 상태 | 용도 |
|----------|------|------|
| `ProgressBar` | ✅ 구현됨 | 진행률 표시 |
| `Spinner` | ✅ 구현됨 | 로딩 애니메이션 |
| `Table` | ✅ 구현됨 | 데이터 테이블 |
| `Box` | ✅ 구현됨 | 콘텐츠 박스 |
| `Divider` | ✅ 구현됨 | 구분선 |
| `Badge` | ✅ 구현됨 | 상태 뱃지 |
| `List` | ✅ 구현됨 | 목록 |
| `Tabs` | ✅ 구현됨 | 탭 헤더 |

**문제**: 구현되어 있지만 CLI 모드에서 활용 안 됨

### 3. 스트리밍 응답 개선

**현재 구현** (internal/tui/cli_mode.go):
```go
c.isStreaming = true
err := c.client.ChatStream(ctx, c.model, c.messages, func(chunk string) {
    fmt.Print(chunk)  // 단순 출력
})
```

**개선안**:
- 타이핑 효과 (문자별 딜레이)
- 마크다운 실시간 렌더링
- 코드 블록 구문 강조

### 4. 백그라운드 작업 시스템 (internal/background/)

**이미 구현됨**:
```go
type TaskManager struct {
    // 작업 제출, 취소, 진행률 콜백
}

type TaskEvent struct {
    Type     string // "task.progress", "task.completed"
    Progress int    // 0-100
}
```

**CLI 연동 필요**:
```bash
devorch> /install llama3.2:7b
📦 Installing llama3.2:7b in background (task-1234)
   Check progress: /task status

devorch> /task list
ID          Model           Progress  Status
task-1234   llama3.2:7b     67%       downloading
```

---

## 🔍 OpenCode 코드 단위 세밀 분석

> 참조: `docs/compare/opencode-analysis.md` (1,316줄)

### 1. OpenCode TUI 구조 (packages/opencode/src/cli/cmd/tui/)

| 디렉토리 | 역할 | DevOrch 대응 |
|----------|------|-------------|
| `component/prompt/` | 프롬프트 입력 + 자동완성 | ✅ input.go |
| `component/dialog-*.tsx` | 각종 다이얼로그 | ⚠️ 부분 |
| `context/theme/` | 32개 테마 JSON | ✅ theme.go |
| `context/keybind.tsx` | 키바인딩 | ⚠️ 부분 |
| `routes/session/` | 세션 뷰 | ✅ cli_mode.go |
| `ui/spinner.ts` | 스피너 | ✅ app.go |
| `ui/toast.tsx` | 토스트 알림 | ❌ 미구현 |

### 2. OpenCode 도구 목록 vs DevOrch

| OpenCode 도구 | DevOrch (internal/tool/builtin/) | 상태 |
|---------------|--------------------------------|------|
| bash.ts | bash.go | ✅ |
| read.ts | read.go | ✅ |
| write.ts | write.go | ✅ |
| edit.ts | edit.go | ✅ |
| multiedit.ts | multiedit.go | ✅ |
| glob.ts | glob.go | ✅ |
| grep.ts | grep.go | ✅ |
| ls.ts | ls.go | ✅ |
| lsp.ts | lsp.go | ✅ |
| apply_patch.ts | patch.go | ✅ |
| codesearch.ts | codesearch.go | ✅ |
| webfetch.ts | webfetch.go | ✅ |
| websearch.ts | websearch.go | ✅ |
| question.ts | question.go | ✅ |
| task.ts | task.go | ✅ |
| todo.ts | todo.go | ✅ |
| batch.ts | batch.go | ✅ |
| plan.ts | plan.go | ✅ |
| skill.ts | skill.go | ✅ |
| - | agent.go | ✅ DevOrch 추가 |
| - | diagnostics.go | ✅ DevOrch 추가 |
| - | sourcegraph.go | ✅ DevOrch 추가 |
| - | view.go | ✅ DevOrch 추가 |
| - | fetch.go | ✅ DevOrch 추가 |

**결론**: DevOrch가 OpenCode보다 **6개 더 많은 도구** 보유

### 3. OpenCode MCP 구현 (packages/opencode/src/mcp/)

```typescript
// OpenCode MCP 구조
src/mcp/
├── index.ts         // MCP 클라이언트 (지연 로딩)
├── auth.ts          // MCP OAuth 인증
├── oauth-callback.ts // OAuth 콜백 핸들러
└── oauth-provider.ts // OAuth 프로바이더

// 지원 기능
- stdio, http (SSE/Streamable) 전송
- OAuth 콜백 지원
- 첫 호출 시 클라이언트 생성 (지연 로딩)
```

**DevOrch 현황**:
- ❌ MCP 클라이언트 미구현
- ⚠️ showMCPs()는 UI만 표시

### 4. OpenCode 세션 관리 (packages/opencode/src/session/)

**OpenCode 세션 디렉토리 구조** (실제 소스 코드 확인):
```
session/
├── compaction.ts    // 컨텍스트 압축 (226줄) - PRUNE_MINIMUM=20_000 토큰
├── summary.ts       // 요약 생성
├── todo.ts          // TODO 관리
├── retry.ts         // 재시도 로직
├── revert.ts        // 되돌리기
├── prompt.ts        // 프롬프트 생성
├── message-v2.ts    // 메시지 모델
├── processor.ts     // 세션 처리
├── status.ts        // 상태 관리
└── llm.ts           // LLM 호출 래퍼
```

**OpenCode 컨텍스트 압축 알고리즘** (compaction.ts):
```typescript
// PRUNE_MINIMUM = 20,000 토큰: 이 이상 pruning
// PRUNE_PROTECT = 40,000 토큰: 최신 40k 토큰 보호
// PRUNE_PROTECTED_TOOLS = ["skill"]: 보호되는 도구

// 로직: 뒤에서부터 40k 토큰까지 보호, 그 이전 도구 출력 삭제
async function prune(input: { sessionID: string }) {
    for (let msgIndex = msgs.length - 1; msgIndex >= 0; msgIndex--) {
        // ... 40k 이전의 tool output 압축
        part.state.time.compacted = Date.now()
    }
}
```

**DevOrch 현황**:
- ✅ 세션 저장/로드 (`~/.config/devorch/sessions/*.json`)
- ⚠️ 압축 미구현 (`/compact` placeholder)
- ⚠️ 요약 미구현
- ❌ TODO 관리 미구현

**DevOrch 구현 필요**:
```go
// internal/session/compaction.go
const (
    PruneMinimum = 20000 // 토큰
    PruneProtect = 40000 // 보호 토큰
)

func (s *Session) Compact() error {
    // 40k 토큰 이후의 도구 출력 압축
    for i := len(s.Messages) - 1; i >= 0; i-- {
        if totalTokens > PruneProtect {
            s.Messages[i].CompactedAt = time.Now()
        }
    }
}
```

### 5. OpenCode 권한 시스템 (packages/opencode/src/permission/)

**OpenCode 권한 모델** (permission/index.ts, 211줄):
```typescript
// 권한 요청 구조
export const Info = z.object({
    id: z.string(),
    type: z.string(),           // "bash", "write", "read"
    pattern: z.string() | z.array(z.string()), // 파일 패턴
    sessionID: z.string(),
    messageID: z.string(),
    callID: z.string().optional(),
    message: z.string(),        // 사용자에게 표시할 메시지
    metadata: z.record(z.string(), z.any()),
})

// 응답 유형
export const Response = z.enum(["once", "always", "reject"])

// 승인된 권한 저장 (세션별)
const approved: {
    [sessionID: string]: {
        [permissionID: string]: boolean
    }
} = {}

// Wildcard 패턴 매칭
function covered(keys: string[], approved: Record<string, boolean>): boolean {
    const pats = Object.keys(approved)
    return keys.every((k) => pats.some((p) => Wildcard.match(k, p)))
}
```

**OpenCode 권한 설정** (config에서):
```yaml
permission:
  bash: "ask"      # 실행 전 확인
  write: "allow"   # 자동 허용
  read: "allow"
  edit: "deny"     # 차단
```

**DevOrch 현황**:
- ❌ 권한 시스템 미구현
- 모든 도구 자동 실행 (위험)

**DevOrch 구현 필요**:
```go
// internal/permission/permission.go
type Permission struct {
    Type    string   // "bash", "write", "read"
    Pattern []string // 파일 패턴
    Mode    Mode     // Ask, Allow, Deny
}

type Mode string
const (
    ModeAsk   Mode = "ask"   // 사용자 확인
    ModeAllow Mode = "allow" // 자동 허용
    ModeDeny  Mode = "deny"  // 차단
)

func (p *PermissionManager) Check(tool string, args map[string]any) (bool, error) {
    mode := p.GetMode(tool)
    switch mode {
    case ModeDeny:
        return false, ErrDenied
    case ModeAllow:
        return true, nil
    case ModeAsk:
        return p.AskUser(tool, args)
    }
}
```

### 6. OpenCode MCP 구현 (packages/opencode/src/mcp/, 935줄)

**실제 MCP 구현 분석** (mcp/index.ts):
```typescript
// 지원 전송 방식
const transports = [
    { name: "StreamableHTTP", transport: new StreamableHTTPClientTransport(...) },
    { name: "SSE", transport: new SSEClientTransport(...) },
]

// stdio 전송도 별도 지원
new StdioClientTransport({ command: mcp.command, args: mcp.args, env: mcp.env })

// OAuth 인증 흐름
if (error instanceof UnauthorizedError) {
    if (lastError.message.includes("registration")) {
        status = { status: "needs_client_registration", error: "..." }
    } else {
        pendingOAuthTransports.set(key, transport)
        status = { status: "needs_auth" }
    }
}

// 도구 변환 (MCP → AI SDK)
async function convertMcpTool(mcpTool: MCPToolDef, client: MCPClient): Promise<Tool> {
    return dynamicTool({
        description: mcpTool.description,
        inputSchema: jsonSchema(schema),
        execute: async (args) => client.callTool({ name: mcpTool.name, arguments: args })
    })
}
```

**OpenCode MCP 상태 모델**:
```typescript
export const Status = z.discriminatedUnion("status", [
    z.object({ status: z.literal("connected") }),
    z.object({ status: z.literal("disabled") }),
    z.object({ status: z.literal("failed"), error: z.string() }),
    z.object({ status: z.literal("needs_auth") }),
    z.object({ status: z.literal("needs_client_registration"), error: z.string() }),
])
```

**DevOrch 현황**:
- ⚠️ `showMCPs()` UI만 표시
- ❌ MCP 클라이언트 미구현

### 7. OpenCode ACP (Agent Client Protocol)

```
┌─────────────┐     JSON-RPC     ┌─────────────┐
│   Zed IDE   │ ◄──────────────► │   OpenCode  │
│  (Client)   │     stdio        │  ACP Server │
└─────────────┘                  └─────────────┘
```

**DevOrch 현황**:
- ✅ WebSocket 허브 구현 (internal/server/routes/websocket.go)
- ⚠️ ACP 라우트는 있으나 (internal/server/routes/acp.go) 기능 미완

### 8. OpenCode Toast 알림 시스템 (ui/toast.tsx, 101줄)

**OpenCode Toast 구현**:
```tsx
type ToastOptions = {
    title?: string,
    message: string,
    variant: "info" | "success" | "warning" | "error",
    duration?: number  // 기본 5000ms
}

// 우측 상단 고정 위치
<box position="absolute" top={2} right={2} maxWidth={60}
    borderColor={theme[current().variant]}>
    <text>{current().message}</text>
</box>
```

**DevOrch 현황**: ❌ 미구현 (단순 fmt.Println 사용)

### 9. OpenCode 키바인딩 시스템 (context/keybind.tsx)

```typescript
const defaultBindings = {
    "ctrl+c": "interrupt",    "ctrl+d": "exit",
    "ctrl+l": "clear",        "ctrl+r": "retry",
    "ctrl+u": "clear-input",  "tab": "autocomplete",
    "up": "history-prev",     "down": "history-next",
}
```

**DevOrch 현황**: ✅ readline 기본 지원, ⚠️ 커스터마이징 불가

---

## 🎯 개발 우선순위 (Phase 58+)

### Phase A: 핵심 기능 완성 (High Priority)

#### A1. 모델 다운로드 진행률 UI 구현 ⭐ NEW

**현재**: 텍스트 메시지만 (`"⬇ Downloading..."`)  
**목표**: 실시간 퍼센트, 속도, 예상 시간 표시

```go
// internal/tui/download.go
type DownloadUI struct {
    Model      string
    Total      int64
    Downloaded int64
    Speed      float64
    StartTime  time.Time
}

func (d *DownloadUI) Render() string {
    percent := float64(d.Downloaded) / float64(d.Total) * 100
    eta := time.Duration(float64(d.Total-d.Downloaded) / d.Speed) * time.Second
    
    bar := renderProgressBar(percent, 40)
    return fmt.Sprintf(
        "⬇ %s\n%s %.1f%% | %.1f MB/s | ETA: %s",
        d.Model, bar, percent, d.Speed/1e6, eta.Round(time.Second),
    )
}

// CLI 사용 예
devorch> /install llama3.2:7b
⬇ llama3.2:7b
████████████████░░░░░░░░░░░░░░░░░░░░░░░░ 42.3% | 15.2 MB/s | ETA: 2m 15s
```

**구현 단계**:
1. Ollama `pull` 출력 파싱 (`"45% ▕████░░░░░░░░░░▏ 1.2 GB/2.7 GB"`)
2. ProgressBar 컴포넌트 활용 (이미 구현됨)
3. 백그라운드 작업 시스템 연동

#### A2. Editor Integration 실제 구현

**현재**: UI만 있음, "coming soon"  
**목표**: VS Code, Neovim, IntelliJ 연동

**OpenCode 방식** (참조: `opencode-analysis.md` Line 600+):
```typescript
// OpenCode는 $EDITOR 환경변수 활용
// /editor 명령으로 외부 에디터에서 프롬프트 편집
// /export로 세션을 에디터에서 열기
```

**DevOrch 구현안**:
```go
// internal/editor/editor.go
func OpenInEditor(content string) (string, error) {
    editor := os.Getenv("EDITOR")
    if editor == "" {
        editor = "vim"
    }
    
    // 임시 파일에 내용 저장
    tmpfile, _ := os.CreateTemp("", "devorch-*.md")
    tmpfile.WriteString(content)
    tmpfile.Close()
    
    // 에디터 실행
    cmd := exec.Command(editor, tmpfile.Name())
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Run()
    
    // 수정된 내용 반환
    return os.ReadFile(tmpfile.Name())
}
```

#### A3. LSP CLI 연동

**현재**: LSP 클라이언트 코드는 있으나 CLI에서 "coming soon"  
**목표**: `/lsp` 명령어로 실제 기능 제공

**이미 구현된 LSP 인프라** (internal/lsp/):
- `client.go` (321줄): Initialize, GotoDefinition, Hover, References
- `operations.go`: Completion, DocumentSymbol, Diagnostics
- `manager.go`: 언어별 서버 관리

**CLI 연동 필요**:
```bash
devorch> /lsp status
Language Servers:
  Go (gopls): ● Running
  TypeScript: ○ Not started
  Python: ○ Not started

devorch> /lsp definition main.go:42
Found: /path/to/pkg/handler.go:156

devorch> /lsp references Config
Found 12 references:
  - config.go:23
  - server.go:45
  ...
```

#### A4. MCP 실제 연동

**현재**: 상태 표시만  
**목표**: MCP 서버 실제 연결/도구 호출

**OpenCode MCP 기능** (참조: `productexplain.md` Section 10):
- stdio, http 전송
- OAuth 콜백
- 지연 로딩

```go
// internal/mcp/client.go
type MCPClient struct {
    Transport MCPTransport // stdio, http
    Tools     []Tool
}

func (c *MCPClient) Connect(serverPath string) error
func (c *MCPClient) ListTools() ([]Tool, error)
func (c *MCPClient) CallTool(name string, args map[string]any) (any, error)
```

---

### Phase B: 브라우저 기반 AI 연동 (Medium Priority)

#### B1. Web UI 대시보드

**현재**: 404 Not Found  
**목표**: `http://localhost:8787`에서 DevOrch 웹 UI

**이미 구현된 인프라**:
- WebSocket 허브 (`internal/server/routes/websocket.go`, 404줄)
- SSE 스트리밍 (`internal/server/routes/stream.go`, 62줄)
- REST API 라우트 (`internal/server/routes/`)

```
┌─────────────────────────────────────────────────────────────┐
│  DevOrch Dashboard                                    [⚙️]  │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
│  │ 🤖 Models   │ │ 📊 Stats    │ │ 💬 Chat     │           │
│  │ llama3.2:7b │ │ 1,234 calls │ │ Active: 3   │           │
│  │ qwen2.5:14b │ │ $0.42 today │ │ Sessions    │           │
│  └─────────────┘ └─────────────┘ └─────────────┘           │
│                                                             │
│  [Chat Interface]                                           │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ User: 코드 리뷰해줘                                  │   │
│  │ AI: 코드를 분석해보겠습니다...                       │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

**기술 스택**:
- Backend: Go (기존 `devorchd`)
- Frontend: HTMX + Alpine.js (경량) 또는 React
- WebSocket: 실시간 스트리밍

#### B2. Provider OAuth 브라우저 인증

**현재**: 부분 구현 (device flow)  
**목표**: 브라우저 기반 원클릭 인증

```go
// OAuth 콜백 핸들러
// GET /auth/callback?code=xxx&state=yyy
func (s *Server) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
    code := r.URL.Query().Get("code")
    token, err := exchangeCode(code)
    // 토큰 저장 후 브라우저 닫기
    renderSuccessPage(w)
}
```

**지원 프로바이더**:
- ✅ GitHub Copilot (device flow 구현됨)
- ⚠️ OpenAI (API 키 방식)
- ⚠️ Anthropic (API 키 방식)
- 🆕 Google OAuth

---

### Phase C: 다중 LLM 효과적 활용 (Medium Priority)

#### C1. CLI에서 Multi-LLM 사용

**목표**: `/ensemble` 명령어로 다중 모델 활용

```bash
devorch> /ensemble parallel
# 모든 연결된 모델에 동시 질문, 최상 응답 선택

devorch> /ensemble vote "버그가 있나요?"
# 3개 모델이 투표, 합의된 답변 제공

devorch> /ensemble chain review
# Model1(분석) -> Model2(요약) -> Model3(추천)
```

**CLI 구현**:
```go
func (c *CLIMode) runEnsemble(args []string) {
    strategy := args[0] // parallel, vote, chain
    
    executor := learning.NewEnsembleExecutor(c.orchestrator, c.clients)
    result, err := executor.ExecuteRequest(ctx, &learning.LLMRequest{
        Messages: c.messages,
        TaskType: strategy,
    })
    
    // 결과 표시
    fmt.Printf("Best: %s (confidence: %.1f%%)\n", 
        result.SelectedModel, result.Confidence*100)
}
```

#### C2. 모델 성능 대시보드

```
devorch> /perf

📊 Model Performance (Last 24h)
────────────────────────────────────────
Provider         Model           Quality  Speed   Cost
─────────────────────────────────────────
ollama           llama3.2:7b     0.82     45t/s   $0
anthropic        claude-3-opus   0.95     12t/s   $0.15
openai           gpt-4-turbo     0.91     18t/s   $0.08

🏆 Best for:
  Code: ollama/qwen2.5-coder:7b (0.89)
  Chat: anthropic/claude-3-sonnet (0.94)
  Analysis: openai/gpt-4-turbo (0.92)
```

#### C3. 자동 라우팅

```go
// 작업 유형별 자동 모델 선택
func (o *Orchestrator) RouteRequest(req Request) string {
    switch req.TaskType {
    case "code_generation":
        return o.getBestModel("code", req.Complexity)
    case "chat":
        return o.getBestModel("chat", req.Complexity)
    case "analysis":
        return o.getBestModel("analysis", req.Complexity)
    default:
        return o.getBestOverall()
    }
}
```

---

### Phase D: 자기 발전 학습 시스템 (Low Priority - 장기)

#### D1. 피드백 기반 학습

**현재**: `Feedback` 구조체 정의됨  
**목표**: 실시간 피드백 반영

```
devorch> 이 코드가 맞아?
AI: [응답]

devorch> /feedback good  # 👍
# Thompson Sampling Alpha += 1

devorch> /feedback bad "더 자세히 설명해줘"  # 👎
# Thompson Sampling Beta += 1
# 피드백 저장 -> 향후 학습에 반영
```

#### D2. 자동 프롬프트 최적화

```go
type PromptOptimizer struct {
    Templates    map[string]*PromptTemplate
    Performance  map[string]float64
}

// A/B 테스트로 최적 프롬프트 발견
func (o *PromptOptimizer) Optimize(taskType string) *PromptTemplate {
    // Thompson Sampling으로 템플릿 선택
    // 성능 측정 후 업데이트
}
```

#### D3. 모델 파인튜닝 파이프라인

```
1. 피드백 데이터 수집 (thumbs up/down)
2. 고품질 데이터 필터링 (quality > 0.8)
3. GGUF 포맷으로 내보내기
4. Ollama에서 fine-tune (미래 기능)
5. 개선된 모델 자동 배포
```

---

## 🚀 /setup 명령어 확장

### 현재 구현 (`internal/autosetup/autosetup.go`)

```go
func Run() (*Result, error) {
    // 1. 하드웨어 감지
    // 2. 티어 분류 (ultra/high/mid/low/minimal)
    // 3. 모델 추천
    // 4. Ollama 설치 (자동)
    // 5. 모델 pull (DEVORCH_AUTO_INSTALL=1 필요)
}
```

### 확장 계획

```bash
devorch> /setup

🔧 DevOrch Setup Wizard
────────────────────────────────────────

Step 1/5: Hardware Detection
  ✓ OS: macOS (arm64)
  ✓ RAM: 32 GB
  ✓ GPU: Apple Metal (M2 Pro)
  ✓ Tier: HIGH

Step 2/5: Ollama Installation
  ✓ Ollama v0.5.1 already installed

Step 3/5: Model Selection
  Recommended models for your system:
  [x] qwen2.5:7b (4.5GB) - General purpose
  [x] qwen2.5-coder:7b (4.5GB) - Code generation
  [ ] llama3.1:8b (4.7GB) - Conversational
  [ ] phi4 (2.5GB) - Fast reasoning
  
  Install selected? [Y/n] 

Step 4/5: Provider Connection
  Connect cloud providers for better results:
  [ ] OpenAI (API key)
  [ ] Anthropic (API key)
  [ ] GitHub Copilot (OAuth)
  
  Skip for now? [Y/n]

Step 5/5: Configuration
  ✓ Created ~/.config/devorch/config.yml
  ✓ Default model: qwen2.5:7b
  ✓ Default provider: ollama

🎉 Setup Complete!
Run 'devorch' to start chatting.
```

### 구현 코드

```go
// internal/autosetup/wizard.go
type SetupWizard struct {
    reader  *bufio.Reader
    profile *global.HWProfile
}

func (w *SetupWizard) Run() error {
    fmt.Println("🔧 DevOrch Setup Wizard")
    
    // Step 1: Hardware
    w.detectHardware()
    
    // Step 2: Ollama
    w.ensureOllama()
    
    // Step 3: Models (interactive)
    models := w.selectModels()
    w.installModels(models)
    
    // Step 4: Providers (optional)
    if w.askYesNo("Connect cloud providers?") {
        w.setupProviders()
    }
    
    // Step 5: Config
    w.createConfig()
    
    return nil
}
```

---

## 📋 구현 로드맵

### 즉시 (1-2주)

1. **Editor Integration 기초**
   - VS Code Extension 스켈레톤
   - WebSocket 통신 구현
   
2. **MCP 실제 연동**
   - stdio 전송 구현
   - 내장 MCP (filesystem, shell) 활성화

3. **/setup 마법사 완성**
   - 인터랙티브 모델 선택
   - 프로바이더 연결 가이드

### 단기 (3-4주)

4. **LSP Integration**
   - gopls 연동
   - 기본 기능 (definition, references)

5. **Web UI 대시보드**
   - 기본 채팅 인터페이스
   - 모델 상태 모니터링

6. **Multi-LLM CLI**
   - `/ensemble` 명령어
   - `/perf` 성능 대시보드

### 중기 (1-2개월)

7. **브라우저 OAuth**
   - 원클릭 프로바이더 연결
   - 토큰 자동 갱신

8. **자동 라우팅**
   - 작업별 최적 모델 선택
   - 비용/품질 최적화

9. **피드백 학습**
   - Thumbs up/down
   - 모델 성능 자동 조정

### 장기 (3-6개월)

10. **프롬프트 최적화**
    - A/B 테스트
    - 자동 템플릿 개선

11. **모델 파인튜닝**
    - 커스텀 데이터 수집
    - Ollama fine-tune 연동

12. **에이전트 오케스트레이션**
    - 멀티 에이전트 협업
    - 복잡한 작업 자동 분해

---

## 📁 파일 구조 (신규 추가 예정)

```
internal/
├── editor/
│   ├── vscode.go      # VS Code 연동
│   ├── neovim.go      # Neovim 연동
│   └── protocol.go    # 에디터 프로토콜
├── lsp/
│   ├── client.go      # LSP 클라이언트
│   ├── gopls.go       # Go LSP
│   └── typescript.go  # TypeScript LSP
├── mcp/
│   ├── client.go      # MCP 클라이언트
│   ├── stdio.go       # stdio 전송
│   └── builtin/
│       ├── filesystem.go
│       ├── shell.go
│       └── git.go
├── webui/
│   ├── server.go      # HTTP 서버
│   ├── handlers.go    # API 핸들러
│   ├── websocket.go   # 실시간 통신
│   └── static/        # 프론트엔드
└── autosetup/
    └── wizard.go      # 셋업 마법사
```

---

## 🎯 핵심 메트릭 목표

| 메트릭 | 현재 | 목표 (3개월) |
|--------|------|-------------|
| 명령어 완성도 | 80% | 95% |
| 프로바이더 연동 | API키만 | OAuth + API키 |
| 에디터 연동 | 0 | VS Code + Neovim |
| MCP 연동 | UI만 | 3+ 내장 MCP |
| Web UI | 404 | 완전한 대시보드 |
| Multi-LLM | 구조만 | CLI + 자동 라우팅 |

---

## 요약

**DevOrch는 이미 강력한 기반을 갖추고 있습니다:**

1. ✅ 52개 CLI 명령어 (OpenCode 5.7배)
2. ✅ 40+ 프로바이더 지원
3. ✅ Multi-LLM 오케스트레이션 코드 (771줄)
4. ✅ Thompson Sampling 기반 모델 선택
5. ✅ 자동 하드웨어 감지 및 모델 추천

**다음 단계는:**

1. 🔧 **완성도 향상**: Editor, LSP, MCP 실제 연동
2. 🌐 **접근성 확대**: Web UI 대시보드
3. 🤖 **지능화**: Multi-LLM 자동 라우팅
4. 📈 **자기 발전**: 피드백 기반 학습

이 전략을 따르면 DevOrch는 OpenCode를 넘어서는 **완전한 AI 코딩 오케스트레이터**가 될 것입니다.
