# DevOrch 명령어 효율화 통합 계획

> **작성일**: 2026년 1월 30일  
> **현재 상태**: cli_mode.go에 44개 명령어 구현됨, 일부만 /help에 노출  
> **목표**: 52개 → 16개 핵심 명령어로 통합 (하위 명령어 방식)

---

## 1. 현재 상태 분석: 구현 vs 노출 불일치

### 1.1 문제점

`cli_mode.go`에 44개 명령어가 **실제로 구현되어 있지만**, `/help`에는 16개만 표시됩니다.

### 1.2 실제 구현된 명령어 (handleCommand 함수, 44개)

| 카테고리 | 명령어 | 핸들러 함수 | 상태 |
|----------|--------|-------------|------|
| **세션** | /new | `messages = []Message{}` | ✅ 동작 |
| | /sessions | `showSessions()` | ✅ 동작 |
| | /clear | `messages = []Message{}` | ✅ 동작 |
| | /reset | `resetSession()` | ✅ 동작 |
| | /save | `saveSession(args)` | ✅ 동작 |
| | /export | `exportSession(args)` | ✅ 동작 |
| | /share | `shareSession(args)` | ⚠️ coming soon |
| | /unshare | `unshareSession(args)` | ⚠️ placeholder |
| | /compact | `compactHistory()` | ✅ 동작 |
| | /details | `showDetails()` | ✅ 동작 |
| | /undo | `undoLast()` | ⚠️ placeholder |
| | /redo | `redoLast()` | ⚠️ placeholder |
| | /history | `showHistory()` | ✅ 동작 |
| **에디터/파일** | /editor | `showEditor()` | ⚠️ coming soon |
| | /init | `initProject()` | ✅ 동작 |
| | /files | `listFiles(args)` | ✅ 동작 |
| | /grep | `grepSearch(args)` | ✅ 동작 |
| | /ls | `listDirectory(args)` | ✅ 동작 |
| | /diff | `showDiff(args)` | ✅ 동작 |
| | /git | `gitCommand(args)` | ✅ 동작 |
| | /review | `reviewCode(args)` | ✅ 동작 |
| **모델/프로바이더** | /model | `setModel()/listModels()` | ✅ 동작 |
| | /models | `listModels()` | ✅ 동작 |
| | /provider | `setProvider()/showProviders()` | ✅ 동작 |
| | /providers | `showProviders()` | ✅ 동작 |
| | /connect | `showConnect()` | ✅ 동작 |
| | /disconnect | `disconnectProvider(name)` | ✅ 동작 |
| **에이전트** | /agent | `setAgent()/showAgents()` | ✅ 동작 |
| | /agents | `showAgents()` | ✅ 동작 |
| | /thinking | `toggleThinking()` | ✅ 동작 |
| **컨텍스트** | /context | `showContext()` | ✅ 동작 |
| | /memory | `showMemory()` | ✅ 동작 |
| | /add | `addToContext(args)` | ✅ 동작 |
| **설정** | /settings | `showSettings()` | ✅ 동작 |
| | /theme | `setTheme()/listThemes()` | ✅ 동작 |
| | /themes | `listThemes()` | ✅ 동작 |
| | /language | `setLanguage()/showLanguage()` | ✅ 동작 |
| | /config | `editConfig(args)` | ✅ 동작 |
| **인증** | /login | `loginProvider(args)` | ✅ 동작 |
| | /logout | `logoutProvider(args)` | ✅ 동작 |
| | /auth | `showAuthStatus()` | ✅ 동작 |
| **도구** | /tools | `showTools()` | ✅ 동작 |
| | /mcps, /mcp | `showMCPs()` | ⚠️ UI만 |
| | /lsp | `showLSP()` | ⚠️ coming soon |
| **시스템** | /status | `showStatus()` | ✅ 동작 |
| | /setup | `runSetup()` | ✅ 동작 |
| | /doctor | `runDoctor()` | ✅ 동작 |
| | /version | `showVersion()` | ✅ 동작 |
| | /install | `installModel(args)` | ✅ 동작 |
| | /bench | `runBenchmark(args)` | ✅ 동작 |
| | /help | `printHelp()` | ✅ 동작 |
| | /exit, /quit | `os.Exit(0)` | ✅ 동작 |

**동작 상태**:
- ✅ 완전 동작: 35개
- ⚠️ Placeholder/coming soon: 9개 (share, unshare, undo, redo, editor, lsp, mcp)

---

## 2. 효율적 명령어 통합 제안

### 2.1 목표

52개 → **16개 핵심 명령어**로 통합 (하위 명령어 방식)

### 2.2 통합 명령어 체계

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     DevOrch 통합 명령어 체계 (16개)                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  📝 세션 관리 (/session)                                                    │
│  ├─ /session new          새 세션 시작 (기존 /new)                          │
│  ├─ /session list         세션 목록 (기존 /sessions)                        │
│  ├─ /session save [name]  세션 저장 (기존 /save)                            │
│  ├─ /session export [fmt] 세션 내보내기 (기존 /export)                      │
│  ├─ /session reset        완전 초기화 (기존 /reset)                         │
│  ├─ /session compact      히스토리 압축 (기존 /compact)                     │
│  ├─ /session details      세션 상세 (기존 /details)                         │
│  └─ /session share        세션 공유 (기존 /share)                           │
│                                                                             │
│  🤖 모델 관리 (/model)                                                      │
│  ├─ /model                현재 모델 표시                                    │
│  ├─ /model list           모델 목록 (기존 /models)                          │
│  ├─ /model set <name>     모델 전환 (기존 /model <name>)                    │
│  ├─ /model install <name> 모델 설치 (기존 /install)                         │
│  └─ /model bench [name]   벤치마크 (기존 /bench)                            │
│                                                                             │
│  🔌 프로바이더 (/provider)                                                  │
│  ├─ /provider             현재 프로바이더 표시                              │
│  ├─ /provider list        프로바이더 목록 (기존 /providers)                 │
│  ├─ /provider set <name>  프로바이더 전환                                   │
│  ├─ /provider connect     연결 (기존 /connect)                              │
│  └─ /provider disconnect  연결 해제 (기존 /disconnect)                      │
│                                                                             │
│  🤖 에이전트 (/agent)                                                       │
│  ├─ /agent                현재 에이전트 표시                                │
│  ├─ /agent list           에이전트 목록 (기존 /agents)                      │
│  └─ /agent set <name>     에이전트 전환                                     │
│                                                                             │
│  📁 파일/코드 (/code)                                                       │
│  ├─ /code files [path]    파일 목록 (기존 /files)                           │
│  ├─ /code ls [path]       디렉토리 (기존 /ls)                               │
│  ├─ /code grep <pattern>  검색 (기존 /grep)                                 │
│  ├─ /code diff [file]     변경사항 (기존 /diff)                             │
│  ├─ /code review [file]   리뷰 (기존 /review)                               │
│  └─ /code git <cmd>       Git 명령 (기존 /git)                              │
│                                                                             │
│  🧠 컨텍스트 (/context)                                                     │
│  ├─ /context              현재 컨텍스트 표시                                │
│  ├─ /context add <file>   파일 추가 (기존 /add)                             │
│  ├─ /context memory       메모리 표시 (기존 /memory)                        │
│  └─ /context thinking     thinking 토글 (기존 /thinking)                    │
│                                                                             │
│  🔧 도구 (/tools)                                                           │
│  ├─ /tools                도구 목록                                         │
│  ├─ /tools mcp            MCP 서버 (기존 /mcps)                             │
│  └─ /tools lsp            LSP 상태 (기존 /lsp)                              │
│                                                                             │
│  ⚙️ 설정 (/config)                                                          │
│  ├─ /config               설정 보기 (기존 /settings)                        │
│  ├─ /config theme [name]  테마 (기존 /theme)                                │
│  ├─ /config lang [lang]   언어 (기존 /language)                             │
│  └─ /config edit [key]    설정 편집 (기존 /config)                          │
│                                                                             │
│  🔐 인증 (/auth)                                                            │
│  ├─ /auth                 인증 상태 (기존 /auth)                            │
│  ├─ /auth login [provider] 로그인 (기존 /login)                             │
│  └─ /auth logout [provider] 로그아웃 (기존 /logout)                         │
│                                                                             │
│  🛠️ 시스템 (/system)                                                        │
│  ├─ /system status        상태 (기존 /status)                               │
│  ├─ /system setup         초기 설정 (기존 /setup)                           │
│  ├─ /system doctor        진단 (기존 /doctor)                               │
│  └─ /system version       버전 (기존 /version)                              │
│                                                                             │
│  📌 단축 명령어 (자주 사용, 호환성 유지)                                     │
│  ├─ /help, /?             도움말                                            │
│  ├─ /exit, /quit, /q      종료                                              │
│  ├─ /clear, /cls          화면 지우기                                       │
│  ├─ /new                  새 세션 (=/session new)                           │
│  ├─ /history              히스토리 (=/context memory)                       │
│  └─ /init                 프로젝트 초기화                                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. 구현 코드 예시

### 3.1 handleCommand 통합 버전

```go
func (c *CLIMode) handleCommand(input string) bool {
    parts := strings.Fields(input)
    cmdName := strings.TrimPrefix(parts[0], "/")
    args := parts[1:]
    
    switch cmdName {
    // ====== 단축 명령어 (자주 사용) ======
    case "help", "h", "?":
        c.printHelp()
    case "exit", "quit", "q":
        fmt.Println("Goodbye!")
        os.Exit(0)
    case "clear", "cls":
        c.clearChat()
    case "new":
        c.sessionNew()
    case "history":
        c.showHistory()
    case "init":
        c.initProject()
    
    // ====== 주요 명령어 그룹 ======
    case "session":
        c.handleSessionCommand(args)
    case "model":
        c.handleModelCommand(args)
    case "provider":
        c.handleProviderCommand(args)
    case "agent":
        c.handleAgentCommand(args)
    case "code":
        c.handleCodeCommand(args)
    case "context":
        c.handleContextCommand(args)
    case "tools":
        c.handleToolsCommand(args)
    case "config":
        c.handleConfigCommand(args)
    case "auth":
        c.handleAuthCommand(args)
    case "system":
        c.handleSystemCommand(args)
    
    // ====== 기존 명령어 (하위 호환성) ======
    case "models":
        fmt.Println(c.theme.Subtle.Render("💡 Tip: /model list 사용을 권장합니다"))
        c.listModels()
    case "providers":
        fmt.Println(c.theme.Subtle.Render("💡 Tip: /provider list 사용을 권장합니다"))
        c.showProviders()
    // ... 기타 기존 명령어들
    
    default:
        fmt.Printf("⚠ Unknown command: /%s\n", cmdName)
    }
    return true
}
```

### 3.2 세션 하위 명령어 처리

```go
func (c *CLIMode) handleSessionCommand(args []string) {
    if len(args) == 0 {
        c.showSessions() // 기본: 세션 목록
        return
    }
    
    switch args[0] {
    case "new":
        c.messages = []Message{}
        fmt.Println(c.theme.Success.Render("✓ New session started"))
    case "list":
        c.showSessions()
    case "save":
        c.saveSession(args[1:])
    case "export":
        c.exportSession(args[1:])
    case "reset":
        c.resetSession()
    case "compact":
        c.compactHistory()
    case "details":
        c.showDetails()
    case "share":
        c.shareSession(args[1:])
    case "unshare":
        c.unshareSession(args[1:])
    case "undo":
        c.undoLast()
    case "redo":
        c.redoLast()
    case "help":
        c.printSessionHelp()
    default:
        fmt.Printf("⚠ Unknown session command: %s\n", args[0])
        fmt.Println("  Usage: /session [new|list|save|export|reset|compact|details|share|undo|redo]")
    }
}
```

### 3.3 모델 하위 명령어 처리

```go
func (c *CLIMode) handleModelCommand(args []string) {
    if len(args) == 0 {
        c.showCurrentModel() // 현재 모델 표시
        return
    }
    
    switch args[0] {
    case "list":
        c.listModels()
    case "set":
        if len(args) > 1 {
            c.setModel(args[1])
        } else {
            fmt.Println("⚠ Usage: /model set <name>")
            c.listModels()
        }
    case "install":
        c.installModel(args[1:])
    case "bench":
        c.runBenchmark(args[1:])
    case "help":
        c.printModelHelp()
    default:
        // 인자가 모델 이름이면 바로 설정 (편의성)
        c.setModel(args[0])
    }
}
```

### 3.4 코드 하위 명령어 처리

```go
func (c *CLIMode) handleCodeCommand(args []string) {
    if len(args) == 0 {
        c.listFiles(nil) // 기본: 파일 목록
        return
    }
    
    switch args[0] {
    case "files":
        c.listFiles(args[1:])
    case "ls":
        c.listDirectory(args[1:])
    case "grep":
        c.grepSearch(args[1:])
    case "diff":
        c.showDiff(args[1:])
    case "review":
        c.reviewCode(args[1:])
    case "git":
        c.gitCommand(args[1:])
    case "editor":
        c.showEditor()
    case "help":
        c.printCodeHelp()
    default:
        fmt.Printf("⚠ Unknown code command: %s\n", args[0])
        fmt.Println("  Usage: /code [files|ls|grep|diff|review|git|editor]")
    }
}
```

### 3.5 설정 하위 명령어 처리

```go
func (c *CLIMode) handleConfigCommand(args []string) {
    if len(args) == 0 {
        c.showSettings() // 기본: 설정 표시
        return
    }
    
    switch args[0] {
    case "theme":
        if len(args) > 1 {
            if args[1] == "list" {
                c.listThemes()
            } else {
                c.setTheme(args[1])
            }
        } else {
            c.listThemes()
        }
    case "lang", "language":
        if len(args) > 1 {
            c.setLanguage(args[1])
        } else {
            c.showLanguage()
        }
    case "edit":
        c.editConfig(args[1:])
    case "help":
        c.printConfigHelp()
    default:
        fmt.Printf("⚠ Unknown config command: %s\n", args[0])
    }
}
```

### 3.6 시스템 하위 명령어 처리

```go
func (c *CLIMode) handleSystemCommand(args []string) {
    if len(args) == 0 {
        c.showStatus() // 기본: 상태 표시
        return
    }
    
    switch args[0] {
    case "status":
        c.showStatus()
    case "setup":
        c.runSetup()
    case "doctor":
        c.runDoctor()
    case "version":
        c.showVersion()
    case "help":
        c.printSystemHelp()
    default:
        fmt.Printf("⚠ Unknown system command: %s\n", args[0])
    }
}
```

---

## 4. 마이그레이션 전략

### Phase 1: 하위 호환성 유지 (즉시 구현)

- 기존 명령어 **모두 유지**
- 통합 명령어 **추가** (병렬 지원)
- 기존 명령어 사용 시 안내 메시지 표시

```go
case "models": // 기존 명령어
    fmt.Println(c.theme.Subtle.Render("💡 Tip: /model list 사용을 권장합니다"))
    c.listModels()
```

### Phase 2: 점진적 전환 (2-4주)

- `/help`에서 통합 명령어 **우선 표시**
- 기존 명령어는 "Legacy" 섹션으로 이동
- 사용 통계 수집

### Phase 3: 기존 명령어 deprecated (8주 후)

- 기존 명령어 사용 시 **경고**
- 문서에서 제거
- 12주 후 완전 제거

---

## 5. 효율화 결과 비교

| 항목 | 현재 | 통합 후 | 개선 |
|------|------|---------|------|
| 최상위 명령어 | 44개 | 16개 | **-64%** |
| /help 출력 길이 | 52줄 | 25줄 | **-52%** |
| 학습 곡선 | 높음 | 낮음 | ⬆️ |
| 자동완성 효율 | 낮음 | 높음 | ⬆️ |
| 일관성 | 낮음 | 높음 | ⬆️ |

---

## 6. /help 출력 예시 (통합 후)

```
📚 DevOrch Commands

Quick:
  /help               Show this help
  /exit               Exit DevOrch  
  /clear              Clear chat
  /new                New session
  /init               Initialize project

Main Commands:
  /session            Session management (new, list, save, export...)
  /model              Model management (list, set, install, bench)
  /provider           Provider management (list, connect, disconnect)
  /agent              Agent management (list, set)
  /code               Code/file operations (files, grep, diff, review, git)
  /context            Context management (add, memory, thinking)
  /tools              Tool management (mcp, lsp)
  /config             Configuration (theme, lang, edit)
  /auth               Authentication (login, logout, status)
  /system             System (status, setup, doctor, version)

Examples:
  /model list                    List all models
  /model set llama3.2:7b         Switch to model
  /code grep "TODO"              Search in files
  /session save my-session       Save current session

Type "/command help" for subcommand details.
```

---

## 7. 기존 명령어 → 통합 명령어 매핑 테이블

| 기존 명령어 | 통합 명령어 | 비고 |
|-------------|-------------|------|
| /new | /session new 또는 /new (단축) | 단축 유지 |
| /sessions | /session list | 통합 |
| /save | /session save | 통합 |
| /export | /session export | 통합 |
| /reset | /session reset | 통합 |
| /compact | /session compact | 통합 |
| /details | /session details | 통합 |
| /share | /session share | 통합 |
| /unshare | /session unshare | 통합 |
| /undo | /session undo | 통합 |
| /redo | /session redo | 통합 |
| /models | /model list | 통합 |
| /model <name> | /model set <name> | 통합 |
| /install | /model install | 통합 |
| /bench | /model bench | 통합 |
| /providers | /provider list | 통합 |
| /provider <name> | /provider set <name> | 통합 |
| /connect | /provider connect | 통합 |
| /disconnect | /provider disconnect | 통합 |
| /agents | /agent list | 통합 |
| /agent <name> | /agent set <name> | 통합 |
| /files | /code files | 통합 |
| /ls | /code ls | 통합 |
| /grep | /code grep | 통합 |
| /diff | /code diff | 통합 |
| /review | /code review | 통합 |
| /git | /code git | 통합 |
| /editor | /code editor | 통합 |
| /context | /context | 유지 |
| /memory | /context memory | 통합 |
| /add | /context add | 통합 |
| /thinking | /context thinking | 통합 |
| /tools | /tools | 유지 |
| /mcps, /mcp | /tools mcp | 통합 |
| /lsp | /tools lsp | 통합 |
| /settings | /config | 통합 |
| /theme | /config theme | 통합 |
| /themes | /config theme list | 통합 |
| /language | /config lang | 통합 |
| /config | /config edit | 통합 |
| /auth | /auth | 유지 |
| /login | /auth login | 통합 |
| /logout | /auth logout | 통합 |
| /status | /system status | 통합 |
| /setup | /system setup | 통합 |
| /doctor | /system doctor | 통합 |
| /version | /system version | 통합 |
| /help | /help | 유지 |
| /exit, /quit | /exit, /quit | 유지 |
| /clear | /clear | 유지 |
| /history | /history 또는 /context memory | 단축 유지 |
| /init | /init | 유지 |

---

## 8. 실행 계획

### 8.1 즉시 (1주)

1. `handleSessionCommand()` 구현
2. `handleModelCommand()` 구현
3. `handleProviderCommand()` 구현
4. `handleCodeCommand()` 구현

### 8.2 단기 (2주)

5. `handleContextCommand()` 구현
6. `handleToolsCommand()` 구현
7. `handleConfigCommand()` 구현
8. `handleAuthCommand()` 구현
9. `handleSystemCommand()` 구현

### 8.3 중기 (3-4주)

10. `/help` 출력 개선
11. 자동완성 지원 추가
12. 기존 명령어 deprecated 경고 추가
13. 문서화 업데이트

---

## 9. 요약

**현재 문제**:
- 44개 명령어가 구현되어 있지만 `/help`에 16개만 노출
- 사용자가 기존 기능을 모름
- 명령어가 너무 많아 학습 곡선이 높음

**해결 방안**:
- 16개 핵심 명령어 그룹으로 통합
- 하위 명령어 방식 (`/model list`, `/session save` 등)
- 자주 사용하는 6개 단축 명령어 유지 (`/help`, `/exit`, `/clear`, `/new`, `/history`, `/init`)
- 기존 명령어 하위 호환성 유지 → 점진적 deprecated

**기대 효과**:
- 명령어 수 -64% (44개 → 16개)
- /help 출력 -52% (52줄 → 25줄)
- 일관성 있는 명령어 체계
- 자동완성 효율 향상
