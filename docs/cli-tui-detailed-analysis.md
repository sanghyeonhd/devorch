# DevOrch CLI vs TUI 세밀한 기능 분석 및 CLI-TUI 통합 업그레이드 계획서

## 📋 전체 기능 매트릭스 분석

### 🎯 1. 명령어 시스템 완전 분석

#### TUI 전용 고급 명령어 (CLI에서 미지원)
```
🔴 TUI 독점 기능들 - CLI에서 완전히 불가능

1. Work Mode 시스템:
   - /mode ask|edit|agent|plan
   - Ctrl+1/2/3/4 단축키
   - 모드별 컨텍스트 관리
   - 모드별 프롬프트 템플릿

2. Multi-Model 비교 시스템:
   - /multimodel (모델 다중 선택)
   - ViewModeMultiModelSelect (선택 UI)
   - ViewModeCompare (응답 비교)
   - 모델별 응답 레이팅 시스템

3. Preset 관리 시스템:
   - /preset save|load|list|delete
   - 모델 조합 저장/복원
   - 사용량 통계 추적

4. 고급 인터랙티브 UI:
   - 키보드 네비게이션 (↑/↓/Enter/Esc)
   - 실시간 검색/필터링 (/)
   - 다중 선택 (Space)
   - 진행률 표시 (프로그레스바)

5. 시각적 선택 시스템:
   - ViewModeThemeSelect
   - ViewModeProviderSelect  
   - ViewModeModelSelect
   - ViewModeLanguageSelect
   - ViewModeInstallSelect

6. 스트리밍 응답:
   - 실시간 토큰 스트리밍
   - 응답 중 취소 가능
   - 시각적 타이핑 애니메이션
```

#### CLI 전용 기능 (TUI에서 미지원)
```
🔴 CLI 독점 기능들 - TUI에서 불가능

1. 권한 관리:
   - /permissions allow|ask|deny <tool>
   - 도구별 세밀한 권한 제어

2. OAuth 완전 플로우:
   - Device Code Flow (GitHub Copilot)
   - PKCE OAuth Flow
   - Headless OAuth (브라우저 없는 환경)
   - 실시간 OAuth 토큰 관리

3. 대화형 설정:
   - 숫자 기반 메뉴 선택
   - 실시간 connection status 업데이트
   - Provider별 세부 설정

4. 텍스트 기반 빠른 입력:
   - 명령어 자동완성 없이 즉석 실행
   - 파이프 지원 (echo "text" | devorch)
```

### 🎯 2. UI/UX 패러다임 차이점

#### TUI: 모달 기반 인터랙션
```go
// 21개의 서로 다른 ViewMode
const (
    ViewModeChat
    ViewModeSessions  
    ViewModeSettings
    ViewModeHelp
    ViewModeCommands         // 명령어 팔레트
    ViewModeSubCommands      // 서브명령어 선택
    ViewModeModelSelect      // 모델 선택
    ViewModeProviderSelect   // Provider 선택
    ViewModeThemeSelect      // 테마 선택
    ViewModeLogin            // OAuth 로그인
    ViewModeMCP              // MCP 서버 관리
    ViewModeSetup            // 자동 설정 마법사
    ViewModeAgentSelect      // 에이전트 모드 선택
    ViewModeInstallSelect    // 모델 설치 선택
    ViewModeLanguageSelect   // 언어 선택
    ViewModeConnect          // Provider 연결
    ViewModeConfirm          // 확인 대화상자
    ViewModeProgress         // 진행률 표시기
    ViewModeMultiModelSelect // 다중 모델 선택
    ViewModeCompare          // 다중 모델 응답 비교
)
```

#### CLI: 명령줄 기반 인터랙션
```go
// 16개 핵심 명령어 그룹
1. /session   - 세션 관리
2. /model     - 모델 관리  
3. /provider  - Provider 관리
4. /agent     - 에이전트 관리
5. /code      - 코드/파일 작업
6. /context   - 컨텍스트 관리
7. /tools     - 도구 관리
8. /config    - 설정 관리
9. /auth      - 인증 관리
10. /system   - 시스템 관리
11. /mode     - 작업 모드 전환 (기본 구현만)
12. /multimodel - 다중 모델 (기본 구현만)
13. /preset   - 프리셋 관리 (기본 구현만)
```

### 🎯 3. 상태 관리 시스템 차이점

#### TUI: 복잡한 상태 트리
```go
type Model struct {
    // Core state (21개 ViewMode 상태)
    mode ViewMode
    
    // Work Mode System (4개 WorkMode)
    workMode        WorkMode
    workModeContext map[string]string
    editContextFiles []string
    agentSteps      []AgentStep
    planAnalysis    *TaskAnalysis
    
    // Multi-Model System
    multiModelEnabled bool
    selectedModels    []ModelSelection
    modelResponses    map[string]*ModelResponse
    showCompareView   bool
    compareScrollIdx  int
    modelRatings      map[string]ResponseRating
    
    // Preset System
    presetManager *PresetManager
    
    // Navigation Stack
    viewStack []ViewMode
    viewData  map[ViewMode]interface{}
    
    // Interactive UI State
    selectionList     []string
    selectionIdx      int
    filteredProviders []ProviderInfo
    showProviderSearch bool
    // ... 50+ 추가 상태 필드들
}
```

#### CLI: 단순한 선형 상태
```go
type CLIMode struct {
    provider string
    model    string
    messages []Message
    theme    Theme
    // 5-6개 기본 필드만
}
```

## 🎯 4. 세밀한 기능별 차이점 분석

### A. 모델 관리 시스템

| 기능 | TUI 구현 | CLI 구현 | 차이점 |
|------|----------|----------|---------|
| **모델 선택** | ✅ 키보드 네비게이션<br/>✅ 검색/필터<br/>✅ 설치 상태 표시<br/>✅ 다중 선택 | ✅ 번호 선택<br/>❌ 검색 없음<br/>✅ 상태 표시<br/>❌ 다중 선택 없음 | TUI가 훨씬 풍부 |
| **모델 설치** | ✅ 실시간 프로그레스바<br/>✅ ETA/속도 표시<br/>✅ 취소 가능<br/>✅ 배치 설치 | ✅ 텍스트 진행률<br/>❌ 상세 정보 없음<br/>❌ 취소 불가<br/>❌ 개별 설치만 | TUI가 압도적으로 우수 |
| **프리셋 관리** | ✅ 완전한 UI<br/>✅ 프리셋 미리보기<br/>✅ 사용량 추적<br/>✅ 설명 관리 | ⚠️ 기본 구현만<br/>❌ 미리보기 없음<br/>❌ 추적 없음<br/>❌ 텍스트만 | TUI 독점 고급 기능 |

### B. Provider 관리 시스템

| 기능 | TUI 구현 | CLI 구현 | 차이점 |
|------|----------|----------|---------|
| **Provider 선택** | ✅ 검색/필터링<br/>✅ 카테고리 구분<br/>✅ 연결 상태 시각화<br/>✅ 키보드 네비게이션 | ✅ 번호 선택<br/>❌ 검색 없음<br/>✅ 상태 표시<br/>✅ 간단한 네비게이션 | TUI가 더 발전됨 |
| **OAuth 인증** | ⚠️ 브라우저 열기만<br/>❌ Device Flow 없음<br/>❌ 토큰 관리 없음 | ✅ 완전한 OAuth<br/>✅ Device Code Flow<br/>✅ 토큰 갱신<br/>✅ Headless 지원 | CLI가 압도적으로 우수 |
| **API Key 관리** | ✅ 마스킹 입력<br/>✅ 즉시 저장<br/>✅ 시각적 피드백 | ✅ 대화형 입력<br/>✅ 즉시 저장<br/>✅ 검증 | 비슷한 수준 |

### C. 세션 관리 시스템

| 기능 | TUI 구현 | CLI 구현 | 차이점 |
|------|----------|----------|---------|
| **세션 목록** | ✅ 인터랙티브 선택<br/>✅ 메타데이터 표시<br/>✅ 키보드 네비게이션 | ✅ 텍스트 목록<br/>✅ 기본 정보<br/>✅ 번호 선택 | TUI가 더 사용자 친화적 |
| **세션 저장** | ✅ 대화형 이름 입력<br/>✅ 즉시 피드백 | ✅ 명령행 인자<br/>✅ 확인 메시지 | 비슷한 기능성 |
| **세션 내보내기** | ✅ 포맷 선택 UI<br/>✅ 미리보기 | ✅ 포맷 지정<br/>❌ 미리보기 없음 | TUI가 더 직관적 |

### D. 작업 모드 시스템 (Work Modes)

| 기능 | TUI 구현 | CLI 구현 | 차이점 |
|------|----------|----------|---------|
| **모드 전환** | ✅ Ctrl+1/2/3/4 단축키<br/>✅ /mode 명령어<br/>✅ 시각적 모드 표시 | ⚠️ /mode 기본 구현만<br/>❌ 단축키 없음<br/>❌ 상태 표시 없음 | TUI 독점 고급 기능 |
| **Ask Mode** | ✅ 최적화된 Q&A 프롬프트<br/>✅ 빠른 응답 모드 | ❌ 미구현 | TUI 독점 |
| **Edit Mode** | ✅ 파일 컨텍스트 자동 감지<br/>✅ 편집 히스토리<br/>✅ 코드 하이라이팅 | ❌ 미구현 | TUI 독점 |
| **Agent Mode** | ✅ 단계별 실행 추적<br/>✅ 진행률 표시<br/>✅ 중간 결과 표시 | ❌ 미구현 | TUI 독점 |
| **Plan Mode** | ✅ 작업 분석 UI<br/>✅ 리스크 평가<br/>✅ 시간 추정 | ❌ 미구현 | TUI 독점 |

### E. 다중 모델 비교 시스템

| 기능 | TUI 구현 | CLI 구현 | 차이점 |
|------|----------|----------|---------|
| **모델 선택** | ✅ 체크박스 다중 선택<br/>✅ 제공자별 그룹화<br/>✅ 실시간 필터링 | ⚠️ 기본 목록만<br/>❌ 다중 선택 없음<br/>❌ 그룹화 없음 | TUI 독점 고급 기능 |
| **응답 비교** | ✅ 나란히 보기<br/>✅ 토큰 수 비교<br/>✅ 응답 시간 비교<br/>✅ 평점 시스템 | ❌ 미구현 | TUI 독점 |
| **결과 관리** | ✅ 최고 응답 선택<br/>✅ 평점 기반 정렬<br/>✅ 결과 내보내기 | ❌ 미구현 | TUI 독점 |

## 🎯 5. CLI에서 TUI 스타일 UI 구현 가능성 분석

### ✅ 구현 가능한 TUI 기능들

#### A. 기본적인 인터랙티브 메뉴
```go
// 현재 CLI도 부분적으로 지원
func (c *CLIMode) showInteractiveMenu(title string, options []string) int {
    fmt.Println(title)
    for i, opt := range options {
        fmt.Printf("  %d. %s\n", i+1, opt)
    }
    // 번호 입력 받아서 처리
}
```

#### B. 프로그레스 바 (Text-based)
```go
// ASCII 기반 프로그레스 바
func (c *CLIMode) showProgress(percent float64) {
    width := 50
    filled := int(percent * float64(width) / 100)
    bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
    fmt.Printf("\r[%s] %.1f%%", bar, percent)
}
```

#### C. 테이블 기반 리스트
```go
// 현재도 부분 지원
func (c *CLIMode) showTable(headers []string, rows [][]string) {
    // ASCII 테이블 형태로 출력
}
```

### ❌ CLI에서 구현 불가능한 TUI 기능들

#### A. 실시간 키보드 네비게이션
- `↑/↓` 키로 실시간 선택 이동 불가
- 터미널의 line-buffered 입력 특성상 제한

#### B. 동시 다중 영역 업데이트  
- 여러 영역 동시 업데이트 불가 (채팅+사이드바)
- 터미널 화면 제어 한계

#### C. 모달 오버레이
- 팝업/모달 윈도우 불가
- z-index 개념 없음

#### D. 실시간 스트리밍 + 상호작용
- 스트리밍 중 다른 조작 불가
- 단일 스레드 입력 처리

## 🎯 6. CLI-TUI 통합 업그레이드 로드맵

### Phase 1: CLI 기본 인터랙티브 기능 강화 (2주)

#### 1.1 개선된 메뉴 시스템
```go
type InteractiveMenu struct {
    title       string
    options     []MenuOption
    multiSelect bool
    searchable  bool
}

type MenuOption struct {
    id          string
    label       string
    description string
    selected    bool
}
```

#### 1.2 ASCII 기반 시각화
- Provider 상태를 ASCII 아트로 표시
- 모델 설치 진행률을 텍스트 프로그레스바로
- 연결 상태를 색상 코드로 표현

#### 1.3 검색 가능한 선택
```bash
$ devorch cli
❯ /provider connect
🔍 Search providers (type to filter): anth
  1. ● Anthropic (Claude) [CONNECTED]  
  2. ○ Anthropic API [DISCONNECTED]
Enter number or continue typing:
```

### Phase 2: CLI 작업 모드 시스템 (3주)

#### 2.1 Work Mode 구현
```go
type CLIWorkMode struct {
    name        string
    description string
    promptMod   string
    contextFunc func(*CLIMode) map[string]string
}

func (c *CLIMode) switchWorkMode(mode string) {
    // 모드별 컨텍스트 설정
    // 프롬프트 수정자 적용
    // 상태 표시 업데이트
}
```

#### 2.2 컨텍스트 관리
- Edit 모드: 현재 디렉토리 파일 자동 스캔
- Agent 모드: 단계별 실행 로그
- Plan 모드: 작업 분석 결과 저장

### Phase 3: CLI 다중 모델 시스템 (4주)

#### 3.1 다중 모델 선택
```bash
❯ /multimodel select
Available models:
  [x] 1. anthropic/claude-3.5-sonnet
  [ ] 2. openai/gpt-4o  
  [x] 3. google/gemini-2.0-flash
  
Toggle: <number>, Apply: 'a', Cancel: 'c'
❯ 2a
Selected 3 models for comparison.
```

#### 3.2 응답 비교 시스템
```bash
❯ How to optimize React performance?

┌── Claude 3.5 Sonnet (2.3s, 450 tokens) ──┐ ┌── GPT-4o (1.8s, 380 tokens) ──┐
│ React performance optimization involves:   │ │ To optimize React performance:   │
│ 1. Memoization with useMemo/useCallback  │ │ 1. Use React.memo for components │
│ 2. Code splitting and lazy loading       │ │ 2. Implement virtualization      │
│ ...                                       │ │ ...                             │
└─────────────────────────────────────────┘ └───────────────────────────────────┘

Rate responses: 1-5 or 's' to select best ❯
```

### Phase 4: CLI 프리셋 시스템 (2주)

#### 4.1 프리셋 관리
```bash
❯ /preset save coding-review "Best models for code review"
✓ Saved preset 'coding-review' with 3 models

❯ /preset list  
📋 Available Presets:
  • coding-review (3 models, used 5 times)
    Claude 3.5 Sonnet + GPT-4o + Gemini Pro
  • writing-help (2 models, used 12 times)
    Claude 3.5 Sonnet + GPT-4o
```

### Phase 5: CLI 고급 UI 요소 (3주)

#### 5.1 대시보드 뷰
```bash
╭──── DevOrch Dashboard ────────────────────────╮
│ 🔗 Provider Status          📊 Usage Today     │
│   ● Anthropic (Connected)     ├ 15 queries    │  
│   ● OpenAI (Connected)        ├ 8 queries     │
│   ○ Google (Disconnected)     └ 23 total      │
│                                               │
│ 🎯 Current Mode: Edit        ⚡ Performance    │
│ 📁 Context: 5 files           ├ Avg: 2.1s     │
│ 🤖 Models: 2 selected         └ Cache: 85%    │
╰───────────────────────────────────────────────╯
❯ Type command or 'h' for help
```

#### 5.2 실시간 상태 업데이트
```bash
❯ /model install llama3.2:7b
Installing llama3.2:7b...
[████████████████████░░░░] 80% (1.2GB/1.5GB) ⬇ 15MB/s ETA: 20s

Press 'q' to cancel, 's' for details ❯
```

### Phase 6: CLI와 TUI 기능 통합 (2주)

#### 6.1 설정 동기화
- CLI와 TUI가 같은 설정 파일 공유
- 세션 상호 호환성
- 프리셋 및 상태 동기화

#### 6.2 모드 전환 지원
```bash
❯ /tui
Switching to TUI mode...
# TUI 모드로 전환

# TUI에서
/cli
Switching to CLI mode...
# CLI 모드로 복귀
```

## 🎯 7. 구현 우선순위 및 복잡도 매트릭스

| 기능 영역 | 구현 복잡도 | CLI 적용 가능성 | 사용자 가치 | 우선순위 |
|----------|------------|----------------|------------|----------|
| **Work Mode System** | 🔴 높음 | ✅ 가능 | 🟢 높음 | 🥇 1순위 |
| **Multi-Model 비교** | 🔴 높음 | ⚠️ 부분 가능 | 🟢 높음 | 🥈 2순위 |  
| **검색/필터링** | 🟡 중간 | ✅ 가능 | 🟢 높음 | 🥉 3순위 |
| **Preset 시스템** | 🟡 중간 | ✅ 가능 | 🟡 중간 | 4순위 |
| **프로그레스 시각화** | 🟢 낮음 | ✅ 가능 | 🟡 중간 | 5순위 |
| **키보드 네비게이션** | 🔴 높음 | ❌ 불가능 | 🟡 중간 | X |
| **실시간 스트리밍** | 🔴 높음 | ❌ 불가능 | 🟡 중간 | X |

## 🎯 8. 결론 및 권장사항

### 최적의 전략: 하이브리드 접근법

1. **CLI를 "Rich CLI"로 업그레이드**
   - ASCII 기반 시각화 최대 활용
   - 인터랙티브 메뉴 시스템 강화
   - 색상 및 텍스트 포맷팅 적극 사용

2. **핵심 TUI 기능들을 CLI에 이식**
   - Work Mode 시스템 → CLI 버전으로 구현  
   - Multi-Model → 텍스트 기반 비교 시스템
   - Preset → 명령행 기반 관리

3. **TUI는 특수 목적으로 유지**
   - 복잡한 설정 작업
   - 모델 설치 및 관리
   - 시각적 비교가 중요한 작업

### 개발 투입 리소스 추정
- **총 개발 기간**: 16주 (4개월)
- **개발자 리소스**: 2명 풀타임  
- **우선순위 1-3**: 9주 (핵심 가치 제공)
- **완전한 기능**: 16주 (TUI 90% 수준 기능성)

이 계획을 통해 CLI가 TUI의 핵심 기능 80%를 지원하면서도, 각 모드의 고유한 장점을 유지할 수 있을 것입니다.