# DevOrch CLI 기능 응용성 분석 보고서

**날짜**: 2026-02-03  
**분석 범위**: 74개 CLI 메인 명령어, 161개 하위 명령어  
**목적**: 실제 개발 응용 가능 여부 확인

---

## 📊 Executive Summary

| 분류 | 개수 | 비율 | 응용 가능 여부 |
|------|------|------|---------------|
| ✅ 실제 백엔드 연동 | 12개 | 16% | **응용 가능** |
| ⚠️ 부분 연동 | 8개 | 11% | **제한적 응용** |
| ❌ 하드코딩만 (Mock) | 42개 | 57% | **응용 불가** |
| 🔴 백엔드 있으나 미연동 | 12개 | 16% | **연동 필요** |

**결론**: 현재 **27%만 실제 응용 가능**, 나머지 73%는 UI만 있고 실제 기능 없음

---

## ✅ 카테고리 1: 실제 백엔드 연동 (응용 가능)

### 완전 연동된 명령어 (12개)

| 명령어 | 백엔드 시스템 | 실제 호출 메서드 | 응용 예시 |
|--------|-------------|-----------------|----------|
| `/session` | `SessionManager` | `ListSessions()`, `CreateSession()`, `GetSession()` | 세션 생성/관리/전환 |
| `/tool` | `ToolRegistry` | `ListTools()`, `GetToolInfo()`, `Execute()` | 도구 실행 및 결과 획득 |
| `/ws` | `WebSocketServer` | `GetClients()`, `Subscribe()`, `Broadcast()` | 실시간 통신 |
| `/exec` | `ExecutionEngine` | `ExecuteCommand()`, `BenchmarkTask()`, `GetHistory()` | 명령 실행 및 벤치마크 |
| `/compact` | `CompactionEngine` | `AnalyzePotential()`, `CompactSession()` | 세션 압축 및 최적화 |
| `/proxy` | `ProxyManager` | `RegisterProxy()`, `RouteRequest()`, `InjectAuth()` | API 프록시 관리 |
| `/memory` | `ProjectMemory` | `Create()`, `Save()`, `UpdateContext()` | 프로젝트 메모리 관리 |
| `/config` | `APIKeyManager` | `ListKeys()`, `SetKey()`, `ValidateKey()` | API 키 관리 |
| `/hardware` | `platform.DetectHWProfile()` | 실제 하드웨어 감지 | 하드웨어 최적화 |
| `/agent orchestrate` | `AgentRegistry`, `Learner` | `SelectBestAgent()`, `RecordPerformance()` | 에이전트 오케스트레이션 |
| `/learn status` | `Learner` | 학습 상태 확인 | ML 모델 성능 모니터링 |
| `/router select` | `Learner` + `HWProfile` | 하드웨어 기반 모델 선택 | 최적 모델 자동 선택 |

### 실제 동작 확인 코드 예시

```go
// /session - 실제 SessionManager 사용
func (c *UnifiedCLI) listSessions(sessionManager *session.Manager) error {
    sessions, err := sessionManager.ListSessions()  // ✅ 실제 API 호출
    if err != nil {
        return fmt.Errorf("failed to list sessions: %w", err)
    }
    // ... 실제 데이터 출력
}

// /tool - 실제 ToolRegistry 사용
func (c *UnifiedCLI) handleToolExecute(toolName string, args []string) error {
    registry := c.core.GetToolRegistry()
    result, err := registry.Execute(toolName, args)  // ✅ 실제 도구 실행
    // ...
}
```

---

## ⚠️ 카테고리 2: 부분 연동 (제한적 응용)

### 백엔드 존재 확인만 하는 명령어 (8개)

| 명령어 | 백엔드 시스템 | 현재 상태 | 문제점 |
|--------|-------------|----------|--------|
| `/storage status` | `OkAONStore`, `SessionManager` | 존재 확인만 | 실제 쿼리 없음 |
| `/background status` | `BackgroundManager` | 존재 확인만 | 태스크 목록 미조회 |
| `/diag run` | `DiagnosticsDoctor` | 존재 확인만 | 실제 진단 실행 안함 |
| `/server status` | `WebSocketServer` | 존재 확인만 | 상세 정보 하드코딩 |
| `/learn status` | `Learner` | 존재 확인만 | 학습 통계 미조회 |
| `/router status` | `Learner` | 부분 연동 | 라우터 통계 하드코딩 |
| `/okaon status` | `OkAONStore` | 존재 확인만 | Run/Reward 데이터 미조회 |
| `/platform status` | `DetectHWProfile()` | 부분 연동 | 최적화 기능 하드코딩 |

### 부분 연동 코드 예시

```go
// 현재 상태 - 존재 확인만
func (c *UnifiedCLI) showBackgroundStatus() error {
    if c.core != nil && c.core.BackgroundManager != nil {
        fmt.Printf("Worker Pool: ✅ Active (Real Backend Connected)\n")  // 존재 확인
    }
    // 아래는 모두 하드코딩된 가짜 데이터
    fmt.Printf("Queue Depth: 0 pending\n")      // ❌ 실제 데이터 아님
    fmt.Printf("Completed Tasks: 127\n")        // ❌ 실제 데이터 아님
}

// 개선 필요 - 실제 데이터 조회
func (c *UnifiedCLI) showBackgroundStatus() error {
    if c.core != nil && c.core.BackgroundManager != nil {
        stats := c.core.BackgroundManager.GetStats()  // ✅ 실제 API 호출
        fmt.Printf("Queue Depth: %d pending\n", stats.QueueDepth)
        fmt.Printf("Completed Tasks: %d\n", stats.CompletedTasks)
    }
}
```

---

## ❌ 카테고리 3: 하드코딩만 (응용 불가) - 42개

### 완전 Mock 상태인 명령어

| 명령어 | 핸들러 | 문제점 | 백엔드 존재 여부 |
|--------|--------|--------|----------------|
| `/model` | `handleModel` | 모든 출력 하드코딩 | ❌ 모델 관리 시스템 없음 |
| `/provider` | `handleProvider` | 모든 출력 하드코딩 | ❌ 프로바이더 관리 시스템 없음 |
| `/context` | `handleContext` | 모든 출력 하드코딩 | ❌ 컨텍스트 관리 시스템 없음 |
| `/code` | `handleCode` | 모든 출력 하드코딩 | ❌ 코드 분석 시스템 없음 |
| `/auth` | `handleAuth` | 모든 출력 하드코딩 | 🟡 auth 패키지 있으나 미연동 |
| `/system` | `handleSystem` | 모든 출력 하드코딩 | ❌ 시스템 관리 없음 |
| `/autonomous` | `handleAutonomous` | TODO 주석, 하드코딩 | 🟡 orchestrator 있으나 미연동 |
| `/task` | `handleTask` | 모든 출력 하드코딩 | ❌ 태스크 시스템 없음 |
| `/delegate` | `handleDelegate` | 모든 출력 하드코딩 | 🟡 delegate 패키지 있으나 미연동 |
| `/analytics` | `handleAnalytics` | 모든 출력 하드코딩 | ❌ 분석 시스템 없음 |
| `/quality` | `handleQuality` | 모든 출력 하드코딩 | 🟡 quality 패키지 있으나 미연동 |
| `/benchmark` | `handleBenchmark` | 모든 출력 하드코딩 | 🟡 bench 패키지 있으나 미연동 |
| `/autosetup` | `handleAutoSetup` | 모든 출력 하드코딩 | 🟡 autosetup 패키지 있으나 미연동 |
| `/install` | `handleInstall` | 모든 출력 하드코딩 | ❌ 설치 시스템 없음 |
| `/lsp` | `handleLSP` | 모든 출력 하드코딩 | 🟡 lsp 패키지 있으나 미연동 |
| `/editor` | `handleEditor` | 모든 출력 하드코딩 | 🟡 editor 패키지 있으나 미연동 |
| `/terminal` | `handleTerminal` | 모든 출력 하드코딩 | ❌ 터미널 관리 없음 |
| `/shell` | `handleShell` | 모든 출력 하드코딩 | 🟡 shell 패키지 있으나 미연동 |
| `/workspace` | `handleWorkspace` | 모든 출력 하드코딩 | ❌ 워크스페이스 관리 없음 |
| `/history` | `handleHistory` | 모든 출력 하드코딩 | 🟡 history 패키지 있으나 미연동 |
| `/plugin` | `handlePlugin` | 모든 출력 하드코딩 | 🟡 plugin 패키지 있으나 미연동 |
| `/experiment` | `handleExperiment` | 모든 출력 하드코딩 | 🟡 experiment 패키지 있으나 미연동 |
| `/mcp` | `handleMCP` | 모든 출력 하드코딩 | 🟡 mcp 패키지 있으나 미연동 |
| `/api` | `handleAPI` | 모든 출력 하드코딩 | ❌ API 관리 시스템 없음 |
| `/events` | `handleEvents` | 모든 출력 하드코딩 | 🟡 bus 패키지로 대체 가능 |
| `/billing` | `handleBilling` | 모든 출력 하드코딩 | 🟡 billing 패키지 있으나 미연동 |
| `/budget` | `handleBudget` | 모든 출력 하드코딩 | ❌ 예산 시스템 없음 |
| `/usage` | `handleUsage` | 모든 출력 하드코딩 | ❌ 사용량 추적 없음 |
| `/doctor` | `handleDoctor` | 모든 출력 하드코딩 | 🟡 diagnostics로 대체 가능 |
| `/logs` | `handleLogs` | 모든 출력 하드코딩 | 🟡 log 패키지 있으나 미연동 |
| `/metrics` | `handleMetrics` | 모든 출력 하드코딩 | ❌ 메트릭 시스템 없음 |
| `/debug` | `handleDebug` | 모든 출력 하드코딩 | ❌ 디버그 시스템 없음 |
| `/i18n` | `handleI18n` | 모든 출력 하드코딩 | 🟡 i18n 패키지 있으나 미연동 |
| `/theme` | `handleTheme` | 모든 출력 하드코딩 | ❌ 테마 시스템 없음 |
| `/file` | `handleFile` | 모든 출력 하드코딩 | ❌ 파일 관리 없음 |
| `/permission` | `handlePermission` | 모든 출력 하드코딩 | 🟡 permission 패키지 있으나 미연동 |
| `/global` | `handleGlobal` | 모든 출력 하드코딩 | 🟡 global 패키지 있으나 미연동 |
| `/tenancy` | `handleTenancy` | 모든 출력 하드코딩 | 🟡 tenancy 패키지 있으나 미연동 |
| `/orchestrator` | `handleOrchestrator` | 모든 출력 하드코딩 | 🟡 orchestrator 패키지 있으나 미연동 |
| `/attribution` | `handleAttribution` | 모든 출력 하드코딩 | 🟡 attribution 패키지 있으나 미연동 |
| `/bus` | `handleBus` | 모든 출력 하드코딩 | 🟡 bus 패키지 있으나 미연동 |
| `/hook` | `handleHook` | 모든 출력 하드코딩 | 🟡 hook 패키지 있으나 미연동 |
| `/pty` | `handlePTY` | 모든 출력 하드코딩 | 🟡 pty 패키지 있으나 미연동 |
| `/signal` | `handleSignal` | 모든 출력 하드코딩 | 🟡 signal 패키지 있으나 미연동 |
| `/server` | `handleServer` | 대부분 하드코딩 | 🟡 server 패키지 있으나 미연동 |
| `/resolver` | `handleModelResolver` | 모든 출력 하드코딩 | 🟡 modelresolver 패키지 있으나 미연동 |
| `/extension` | `handleExtension` | 모든 출력 하드코딩 | 🟡 extension 패키지 있으나 미연동 |
| `/runtime` | `handleRuntime` | 모든 출력 하드코딩 | 🟡 runtime 패키지 있으나 미연동 |

### 하드코딩 코드 예시

```go
// ❌ 응용 불가 - 모든 데이터가 하드코딩
func (c *UnifiedCLI) showOrchestratorStatus() error {
    fmt.Println("🧠 AI Orchestrator Status:")
    fmt.Printf("Mode: Autonomous\n")              // 가짜 데이터
    fmt.Printf("Active Tier: Tier 2 (Mid-level)\n")  // 가짜 데이터
    fmt.Printf("Total Decisions: 1,247\n")        // 가짜 데이터
    fmt.Printf("Success Rate: 94.2%%\n")          // 가짜 데이터
    return nil
}

// ❌ 응용 불가 - 실제 MCP 서버 연결 안됨
func (c *UnifiedCLI) listMCPServers() error {
    fmt.Printf("%-15s %-10s %-20s %-15s\n", "Name", "Status", "URL", "Tools")
    // 아래 모두 하드코딩된 가짜 서버 목록
    fmt.Printf("%-15s %-10s %-20s %-15s\n", "filesystem", "✅ Active", "stdio://mcp-fs", "5 tools")
}
```

---

## 🔴 카테고리 4: 백엔드 있으나 미연동 (연동 필요)

### 즉시 연동 가능한 시스템 (12개)

| 명령어 | 백엔드 패키지 | 주요 인터페이스 | 연동 우선순위 |
|--------|-------------|----------------|-------------|
| `/storage` | `internal/storage` | `sqlite.DB`, `okaon.Store` | 🔴 HIGH |
| `/background` | `internal/background` | `Manager.Submit()`, `GetStats()` | 🔴 HIGH |
| `/diag` | `internal/diagnostics` | `Doctor.Run()`, `Report` | 🔴 HIGH |
| `/okaon` | `internal/okaon` | `Store.InsertRun()`, `GetArmStats()` | 🔴 HIGH |
| `/mcp` | `internal/mcp` | `Manager.ListServers()`, `CallTool()` | 🟡 MEDIUM |
| `/lsp` | `internal/lsp` | `Client.GetDiagnostics()`, `FindRefs()` | 🟡 MEDIUM |
| `/bus` | `internal/bus` | `Hub.Publish()`, `Subscribe()` | 🟡 MEDIUM |
| `/hook` | `internal/hook` | `Register()`, `Execute()` | 🟡 MEDIUM |
| `/pty` | `internal/pty` | `Create()`, `Attach()`, `Execute()` | 🟡 MEDIUM |
| `/plugin` | `internal/plugin` | `Manager.Load()`, `Enable()` | 🟢 LOW |
| `/extension` | `internal/extension` | `Manager.Install()`, `Enable()` | 🟢 LOW |
| `/orchestrator` | `internal/orchestrator` | `DecisionEngine`, `TaskQueue` | 🔴 HIGH |

### 연동 예시 코드

```go
// 현재 - 하드코딩
func (c *UnifiedCLI) listOkAONRuns() error {
    fmt.Printf("%-12s %-15s %-15s\n", "run-001", "anthropic", "claude-sonnet")  // 가짜
}

// 개선 - 실제 연동
func (c *UnifiedCLI) listOkAONRuns() error {
    if c.core == nil || c.core.OkAONStore == nil {
        return fmt.Errorf("OkAON store not initialized")
    }
    
    ctx := context.Background()
    aggregates, err := c.core.OkAONStore.GetModelAggregate(ctx, time.Now().Add(-24*time.Hour).Unix())
    if err != nil {
        return err
    }
    
    for _, agg := range aggregates {
        fmt.Printf("%-12s %-15s %-10.2f\n", agg.Model, agg.Provider, agg.AvgQuality)
    }
    return nil
}
```

### 백엔드 패키지별 사용 가능한 실제 API

#### 1. OkAON Store (`internal/okaon/store.go`)
```go
// Store 인터페이스 - 모두 실제 사용 가능
type Store interface {
    InsertWork(ctx, Work) error                    // 작업 기록 삽입
    InsertRun(ctx, Run) error                      // 실행 기록 삽입  
    InsertQuality(ctx, Quality) error              // 품질 기록 삽입
    InsertReward(ctx, Reward) error                // 보상 기록 삽입
    UpsertArmStats(ctx, ArmStatUpdate) error       // Arm 통계 업데이트
    GetArmStats(ctx, fingerprint, category, provider, model) (ArmStat, bool, error)
    GetModelAggregate(ctx, since int64) ([]ModelAggregate, error)      // ⭐ 모델별 집계
    GetCategoryAggregate(ctx, since int64) ([]CategoryAggregate, error) // ⭐ 카테고리별 집계
}
```

#### 2. Background Manager (`internal/background/manager.go`)
```go
// Manager 메서드 - 모두 실제 사용 가능
func (m *Manager) Start(ctx) error          // 워커 시작
func (m *Manager) Stop() error              // 워커 중지
func (m *Manager) Submit(task Task) error   // 태스크 제출
func (m *Manager) GetQueueDepth() int64     // ⭐ 큐 깊이 조회 가능
// Feedback.SetQueueDepth(), SetCongestion() 으로 실시간 모니터링 가능
```

#### 3. MCP Manager (`internal/mcp/manager.go`)
```go
// Manager 메서드 - 모두 실제 사용 가능
func (m *Manager) LoadConfig(path) error              // 설정 로드
func (m *Manager) ListServers() []ServerConfig        // ⭐ 서버 목록 조회
func (m *Manager) Connect(name) error                 // 서버 연결
func (m *Manager) Disconnect(name) error              // 서버 연결 해제
func (m *Manager) CallTool(server, tool, args) error  // ⭐ 도구 호출
```

#### 4. Diagnostics Doctor (`internal/diagnostics/doctor.go`)
```go
// Doctor 메서드 - 모두 실제 사용 가능
func (d *Doctor) Run(ctx, checks...) Report          // ⭐ 진단 실행
func (d *Doctor) CheckNetwork() CheckResult          // 네트워크 체크
func (d *Doctor) CheckStorage() CheckResult          // 스토리지 체크
func (d *Doctor) CheckProvider(name) CheckResult     // 프로바이더 체크
```

#### 5. Bus Hub (`internal/bus/hub.go`)
```go
// Hub 메서드 - 모두 실제 사용 가능
func (h *Hub) Publish(topic string, event any) error    // ⭐ 이벤트 발행
func (h *Hub) Subscribe(topic string, handler func())   // ⭐ 구독
func (h *Hub) GetStats() Stats                          // 통계 조회
```

---

## 📈 개선 우선순위 로드맵

### Phase 1: 핵심 시스템 연동 (1주)

| 우선순위 | 명령어 | 작업 내용 | 예상 시간 |
|---------|--------|----------|----------|
| 1 | `/storage` | `sqlite.DB` 실제 쿼리 연동 | 4시간 |
| 2 | `/background` | `BackgroundManager` 전체 연동 | 4시간 |
| 3 | `/diag` | `DiagnosticsDoctor.Run()` 연동 | 4시간 |
| 4 | `/okaon` | `OkAONStore` 전체 연동 | 4시간 |
| 5 | `/orchestrator` | `DecisionEngine` 연동 | 6시간 |

### Phase 2: 개발 도구 연동 (2주)

| 우선순위 | 명령어 | 작업 내용 | 예상 시간 |
|---------|--------|----------|----------|
| 6 | `/mcp` | MCP 서버 관리 연동 | 6시간 |
| 7 | `/lsp` | LSP 클라이언트 연동 | 6시간 |
| 8 | `/bus` | 이벤트 버스 연동 | 4시간 |
| 9 | `/hook` | 훅 시스템 연동 | 4시간 |
| 10 | `/pty` | PTY 관리 연동 | 4시간 |

### Phase 3: 확장 기능 연동 (3주)

| 우선순위 | 명령어 | 작업 내용 | 예상 시간 |
|---------|--------|----------|----------|
| 11 | `/plugin` | 플러그인 시스템 연동 | 4시간 |
| 12 | `/extension` | 확장 시스템 연동 | 4시간 |
| 13 | `/billing` | 빌링 시스템 연동 | 4시간 |
| 14 | `/quality` | 품질 평가 연동 | 4시간 |
| 15 | `/benchmark` | 벤치마크 연동 | 4시간 |

---

## 🔧 구체적 개선 방안

### 1. Core 구조체에 누락된 시스템 추가

```go
// internal/core/core.go
type DevOrchCore struct {
    // 기존
    SessionManager    *session.Manager
    ToolRegistry      *tool.Registry
    BackgroundManager *background.Manager
    DiagnosticsDoctor *diagnostics.Doctor
    OkAONStore        okaon.Store
    
    // 추가 필요
    MCPManager        *mcp.Manager           // MCP 서버 관리
    LSPClient         *lsp.Client            // LSP 클라이언트
    EventBus          *bus.Hub               // 이벤트 버스
    HookManager       *hook.Manager          // 훅 관리
    PTYManager        *pty.Manager           // PTY 관리
    PluginManager     *plugin.Manager        // 플러그인 관리
    ExtensionManager  *extension.Manager     // 확장 관리
    BillingTracker    *billing.Tracker       // 빌링 추적
    QualityChecker    *quality.Checker       // 품질 체크
    BenchRunner       *bench.Runner          // 벤치마크
    DecisionEngine    *orchestrator.DecisionEngine  // AI 결정 엔진
}
```

### 2. 핸들러 함수 개선 패턴

```go
// Before (하드코딩)
func (c *UnifiedCLI) showBusStatus() error {
    fmt.Printf("Status: Active\n")           // 가짜
    fmt.Printf("Active Subscriptions: 12\n") // 가짜
    return nil
}

// After (실제 연동)
func (c *UnifiedCLI) showBusStatus() error {
    if c.core == nil || c.core.EventBus == nil {
        return fmt.Errorf("event bus not initialized")
    }
    
    stats := c.core.EventBus.GetStats()
    fmt.Printf("Status: %s\n", stats.Status)
    fmt.Printf("Active Subscriptions: %d\n", stats.SubscriptionCount)
    fmt.Printf("Events Published: %d\n", stats.EventsPublished)
    return nil
}
```

---

## 📝 결론

### 현재 상태
- **74개 메인 명령어** 중 **12개(16%)만 실제 동작**
- **42개(57%)는 완전한 Mock 상태** (UI만 있고 기능 없음)
- **12개(16%)는 백엔드 존재하나 미연동**

### 문제점
1. 대부분의 CLI 명령어가 `fmt.Printf()`로 하드코딩된 가짜 데이터 출력
2. 백엔드 패키지는 구현되어 있으나 CLI와 연결 안됨
3. `DevOrchCore`에 필요한 시스템들이 모두 등록되지 않음

### 권장 조치
1. **즉시**: 핵심 5개 시스템 실제 연동 (storage, background, diag, okaon, orchestrator)
2. **1주 내**: 개발 도구 5개 연동 (mcp, lsp, bus, hook, pty)
3. **2주 내**: 확장 기능 5개 연동 (plugin, extension, billing, quality, benchmark)

### 예상 결과
- 연동 완료 시: **응용 가능 명령어 74개 (100%)**
- 현재: **응용 가능 명령어 20개 (27%)**

---

## 📊 명령어별 응용 가능 여부 상세 표

| # | 명령어 | 핸들러 | 상태 | 응용 가능 | 필요 작업 |
|---|--------|--------|------|----------|----------|
| 1 | `/session` | handleSession | ✅ 연동됨 | **가능** | - |
| 2 | `/tool` | handleTool | ✅ 연동됨 | **가능** | - |
| 3 | `/ws` | handleWebSocket | ✅ 연동됨 | **가능** | - |
| 4 | `/exec` | handleExecution | ✅ 연동됨 | **가능** | - |
| 5 | `/compact` | handleCompaction | ✅ 연동됨 | **가능** | - |
| 6 | `/proxy` | handleProxy | ✅ 연동됨 | **가능** | - |
| 7 | `/memory` | handleMemory | ✅ 연동됨 | **가능** | - |
| 8 | `/config` | handleConfig | ✅ 연동됨 | **가능** | - |
| 9 | `/hardware` | handleHardware | ✅ 연동됨 | **가능** | - |
| 10 | `/agent` | handleAgent | ⚠️ 부분 | 제한적 | 학습 API 연동 |
| 11 | `/learn` | handleLearn | ⚠️ 부분 | 제한적 | Learner 통계 연동 |
| 12 | `/router` | handleRouter | ⚠️ 부분 | 제한적 | 라우팅 로직 연동 |
| 13 | `/storage` | handleStorage | ⚠️ 부분 | 제한적 | OkAON API 연동 필요 |
| 14 | `/background` | handleBackground | ⚠️ 부분 | 제한적 | Manager.GetQueueDepth() |
| 15 | `/diag` | handleDiag | ⚠️ 부분 | 제한적 | Doctor.Run() 연동 |
| 16 | `/server` | handleServer | ⚠️ 부분 | 제한적 | 상세 정보 연동 |
| 17 | `/okaon` | handleOkAON | ⚠️ 부분 | 제한적 | GetModelAggregate() |
| 18 | `/platform` | handlePlatform | ⚠️ 부분 | 제한적 | 최적화 연동 |
| 19 | `/model` | handleModel | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 20 | `/provider` | handleProvider | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 21 | `/context` | handleContext | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 22 | `/code` | handleCode | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 23 | `/auth` | handleAuth | ❌ Mock | **불가** | auth 패키지 연동 |
| 24 | `/system` | handleSystem | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 25 | `/autonomous` | handleAutonomous | ❌ Mock | **불가** | orchestrator 연동 |
| 26 | `/task` | handleTask | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 27 | `/delegate` | handleDelegate | ❌ Mock | **불가** | delegate 연동 |
| 28 | `/analytics` | handleAnalytics | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 29 | `/quality` | handleQuality | ❌ Mock | **불가** | quality 연동 |
| 30 | `/benchmark` | handleBenchmark | ❌ Mock | **불가** | bench 연동 |
| 31 | `/autosetup` | handleAutoSetup | ❌ Mock | **불가** | autosetup 연동 |
| 32 | `/install` | handleInstall | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 33 | `/lsp` | handleLSP | ❌ Mock | **불가** | lsp 연동 |
| 34 | `/editor` | handleEditor | ❌ Mock | **불가** | editor 연동 |
| 35 | `/terminal` | handleTerminal | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 36 | `/shell` | handleShell | ❌ Mock | **불가** | shell 연동 |
| 37 | `/workspace` | handleWorkspace | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 38 | `/history` | handleHistory | ❌ Mock | **불가** | history 연동 |
| 39 | `/plugin` | handlePlugin | ❌ Mock | **불가** | plugin 연동 |
| 40 | `/experiment` | handleExperiment | ❌ Mock | **불가** | experiment 연동 |
| 41 | `/mcp` | handleMCP | ❌ Mock | **불가** | mcp 연동 필요 |
| 42 | `/api` | handleAPI | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 43 | `/events` | handleEvents | ❌ Mock | **불가** | bus 연동 |
| 44 | `/billing` | handleBilling | ❌ Mock | **불가** | billing 연동 |
| 45 | `/budget` | handleBudget | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 46 | `/usage` | handleUsage | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 47 | `/doctor` | handleDoctor | ❌ Mock | **불가** | diagnostics 연동 |
| 48 | `/logs` | handleLogs | ❌ Mock | **불가** | log 연동 |
| 49 | `/metrics` | handleMetrics | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 50 | `/debug` | handleDebug | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 51 | `/i18n` | handleI18n | ❌ Mock | **불가** | i18n 연동 |
| 52 | `/theme` | handleTheme | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 53 | `/file` | handleFile | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 54 | `/permission` | handlePermission | ❌ Mock | **불가** | permission 연동 |
| 55 | `/global` | handleGlobal | ❌ Mock | **불가** | global 연동 |
| 56 | `/tenancy` | handleTenancy | ❌ Mock | **불가** | tenancy 연동 |
| 57 | `/orchestrator` | handleOrchestrator | ❌ Mock | **불가** | orchestrator 연동 |
| 58 | `/attribution` | handleAttribution | ❌ Mock | **불가** | attribution 연동 |
| 59 | `/bus` | handleBus | ❌ Mock | **불가** | bus 연동 |
| 60 | `/hook` | handleHook | ❌ Mock | **불가** | hook 연동 |
| 61 | `/pty` | handlePTY | ❌ Mock | **불가** | pty 연동 |
| 62 | `/signal` | handleSignal | ❌ Mock | **불가** | signal 연동 |
| 63 | `/extension` | handleExtension | ❌ Mock | **불가** | extension 연동 |
| 64 | `/runtime` | handleRuntime | ❌ Mock | **불가** | runtime 연동 |
| 65 | `/workflow` | handleWorkflow | ❌ Mock | **불가** | 백엔드 구현 필요 |
| 66 | `/orchestrate` | handleOrchestrate | ⚠️ 부분 | 제한적 | 상세 연동 필요 |

---

## 🛠️ 즉시 개선 가능한 하드코딩 제거 예시

### `/okaon runs` 명령어 실제 연동 코드

**현재 코드 (하드코딩):**
```go
func (c *UnifiedCLI) listOkAONRuns() error {
    fmt.Println("📋 Recent OkAON Runs:")
    // 전부 가짜 데이터
    fmt.Printf("%-12s %-15s %-15s %-10s %-12s\n", "run-001", "anthropic", "claude-sonnet", "0.92", "1.2s")
    fmt.Printf("%-12s %-15s %-15s %-10s %-12s\n", "run-002", "openai", "gpt-4o-mini", "0.85", "0.8s")
    return nil
}
```

**개선된 코드 (실제 연동):**
```go
func (c *UnifiedCLI) listOkAONRuns() error {
    if c.core == nil || c.core.OkAONStore == nil {
        return fmt.Errorf("OkAON store not initialized")
    }
    
    ctx := context.Background()
    since := time.Now().Add(-24 * time.Hour).Unix()
    
    aggregates, err := c.core.OkAONStore.GetModelAggregate(ctx, since)
    if err != nil {
        return fmt.Errorf("failed to get model aggregates: %w", err)
    }
    
    fmt.Println("📋 Recent OkAON Model Performance (Last 24h):")
    fmt.Println()
    fmt.Printf("%-15s %-12s %-10s %-10s %-10s\n", "Model", "Provider", "Runs", "Avg Quality", "Avg Latency")
    fmt.Printf("%s\n", strings.Repeat("-", 60))
    
    for _, agg := range aggregates {
        fmt.Printf("%-15s %-12s %-10d %-10.3f %-10.2fs\n", 
            agg.Model, agg.Provider, agg.RunCount, agg.AvgQuality, agg.AvgLatencyMs/1000)
    }
    
    if len(aggregates) == 0 {
        fmt.Println("No runs recorded in the last 24 hours")
    }
    return nil
}
```

### `/background status` 명령어 실제 연동 코드

**현재 코드 (하드코딩):**
```go
func (c *UnifiedCLI) showBackgroundStatus() error {
    fmt.Printf("Queue Depth: 0 pending\n")      // 가짜
    fmt.Printf("Completed Tasks: 127\n")         // 가짜
    return nil
}
```

**개선된 코드 (실제 연동):**
```go
func (c *UnifiedCLI) showBackgroundStatus() error {
    if c.core == nil || c.core.BackgroundManager == nil {
        return fmt.Errorf("background manager not initialized")
    }
    
    depth := c.core.BackgroundManager.GetQueueDepth()
    congestion := c.core.BackgroundManager.GetCongestion()
    
    fmt.Println("🔄 Background Task Manager:")
    fmt.Printf("Queue Depth: %d pending\n", depth)
    fmt.Printf("Congestion Score: %.2f\n", congestion)
    fmt.Printf("Workers: %d active\n", c.core.BackgroundManager.GetWorkerCount())
    return nil
}
```
