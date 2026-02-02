# DevOrch CLI vs TUI 종합적 기능 분석 및 CLI-TUI 통합 로드맵

## 📋 1. 현재 CLI vs TUI 기능 구현 상태 완전 분석

### 🎯 1.1 TUI 전용 고급 기능들 (CLI에서 완전히 미지원)

#### A. Multi-Model 비교 시스템
- **TUI 구현 현황**:
  - `/multimodel` 명령으로 모델 선택 인터페이스 진입
  - `ViewModeMultiModelSelect`: 다중 모델 선택 화면
  - `ViewModeCompare`: 응답 비교 화면
  - 모델별 응답 레이팅 시스템 (`ResponseRating`)
  - 사이드바이사이드 비교 뷰

- **CLI 구현 현황**:
  - 기본적인 `/multimodel` 명령어만 존재
  - 실제 다중 모델 선택 불가
  - 응답 비교 기능 없음
  - 레이팅 시스템 없음

```go
// TUI에만 있는 고급 기능
type ModelSelection struct {
    Name     string
    Provider string
    Selected bool
    Rating   ResponseRating
}

type ModelResponse struct {
    Content  string
    Tokens   int
    Duration time.Duration
    Rating   ResponseRating
}
```

#### B. Preset 관리 시스템
- **TUI 구현 현황**:
  - 완전한 preset 관리자 (`PresetManager`)
  - `/preset save|load|list|delete` 전체 구현
  - 모델 조합 저장/복원
  - 사용량 통계 추적
  - preset 별 메타데이터 관리

- **CLI 구현 현황**:
  - 기본적인 명령어 구조만 존재
  - 실제 저장/로드 기능 없음
  - 단순 메시지만 출력
  - 백엔드 연동 없음

```go
// TUI 전용 preset 시스템
type Preset struct {
    Name        string           `json:"name"`
    Description string           `json:"description"`
    Models      []ModelSelection `json:"models"`
    CreatedAt   time.Time        `json:"created_at"`
    UpdatedAt   time.Time        `json:"updated_at"`
    UsageCount  int              `json:"usage_count"`
}
```

#### C. 고급 ViewMode 시스템 (21개 모드)
- **TUI 구현 현황**:
  - 21개의 전용 ViewMode (ViewModeChat부터 ViewModeCompare까지)
  - 모드별 전용 UI 렌더링
  - 모드간 네비게이션 스택
  - 모드별 키바인딩

- **CLI 구현 현황**:
  - 단순 텍스트 기반 메뉴만 존재
  - 모드 개념 없음
  - 선형적 명령 처리

```go
// TUI 전용 ViewMode 시스템
const (
    ViewModeChat             ViewMode = iota
    ViewModeCommands
    ViewModeSubCommands
    ViewModeModels
    ViewModeProviders
    ViewModeThemes
    ViewModeProviderSelect
    ViewModeThemeSelect
    ViewModeLogin
    ViewModeMCP
    ViewModeSetup
    ViewModeAgentSelect
    ViewModeInstallSelect
    ViewModeLanguageSelect
    ViewModeConnect
    ViewModeConfirm
    ViewModeProgress
    ViewModeMultiModelSelect  // CLI 미지원
    ViewModeCompare          // CLI 미지원
)
```

### 🎯 1.2 Work Mode 시스템 분석

#### A. TUI Work Mode 구현 (완전 구현)
```go
type WorkMode int

const (
    WorkModeAsk   WorkMode = iota // Quick Q&A mode
    WorkModeEdit                  // Code editing mode  
    WorkModeAgent                 // Autonomous agent mode
    WorkModePlan                  // Task planning mode
)

func (m *Model) switchWorkMode(mode WorkMode) tea.Cmd {
    m.workMode = mode
    
    // 모드별 컨텍스트 자동 설정
    switch mode {
    case WorkModeEdit:
        m.detectEditContext()  // 파일 컨텍스트 자동 감지
    case WorkModeAgent:
        // 자율 에이전트 모드 초기화
    case WorkModePlan:
        // 계획 모드 컨텍스트 설정
    }
    
    return nil
}

// 모드별 프롬프트 수정자
func (m *Model) getWorkModePromptModifier() string {
    switch m.workMode {
    case WorkModeAsk:
        return "Quick Q&A mode: Provide concise, direct answers."
    case WorkModeEdit:
        return "Code editing mode: Focus on code analysis and modifications."
    case WorkModeAgent:
        return "Agent mode: Break down tasks and execute step-by-step."
    case WorkModePlan:
        return "Planning mode: Create detailed task breakdowns and strategies."
    }
}
```

#### B. CLI Work Mode 구현 (기본 구현만)
```go
func (c *CLIMode) handleModeCommand(args []string) {
    // 단순 메시지만 출력
    switch args[0] {
    case "ask":
        fmt.Println("✓ Switched to Ask mode (💬 Quick Q&A)")
    case "edit":
        fmt.Println("✓ Switched to Edit mode (✏️ Code editing)")
    // 실제 상태 변경이나 컨텍스트 설정 없음
    }
}
```

### 🎯 1.3 UI 인터랙션 패러다임 차이

#### A. TUI: 모달 기반 인터랙션
- **키보드 네비게이션**: `Ctrl+1/2/3/4` 모드 전환
- **인터랙티브 선택**: 상하 화살표로 모델/provider 선택
- **실시간 UI 업데이트**: Bubbletea 기반 반응형 UI
- **시각적 피드백**: 프로그레스바, 스피너, 하이라이트

#### B. CLI: 선형 명령 처리
- **명령어 기반**: 모든 작업이 텍스트 명령으로만 가능
- **단방향 출력**: 명령 → 결과 출력의 단순 구조
- **상태 관리 부족**: 세션간 상태 보존 미흡

## 📊 2. 기능 구현 상태 매트릭스

| 기능 영역 | TUI 구현 | CLI 구현 | 구현율 | 비고 |
|-----------|----------|----------|---------|------|
| **기본 채팅** | ✅ 완전 | ✅ 완전 | 100% | 동일 기능 |
| **모델 관리** | ✅ 완전 | ✅ 완전 | 100% | 동일 기능 |
| **Provider 관리** | ✅ 완전 | ✅ 완전 | 100% | 동일 기능 |
| **세션 관리** | ✅ 완전 | ✅ 완전 | 100% | 동일 기능 |
| **설정 관리** | ✅ 완전 | ✅ 완전 | 100% | 동일 기능 |
| **Work Mode 시스템** | ✅ 완전 구현 | ⚠️ 메시지만 | 20% | TUI 독점 |
| **Multi-Model 비교** | ✅ 완전 구현 | ⚠️ 명령어만 | 10% | TUI 독점 |
| **Preset 관리** | ✅ 완전 구현 | ⚠️ 껍데기만 | 15% | TUI 독점 |
| **인터랙티브 UI** | ✅ 21개 ViewMode | ❌ 없음 | 0% | TUI 독점 |
| **키보드 단축키** | ✅ 풍부 | ❌ 없음 | 0% | TUI 독점 |

**전체 기능 구현율**: CLI는 TUI 대비 약 **65%** 수준

## 🎯 3. CLI에서 TUI 스타일 UI 구현 가능성 분석

### 🟢 3.1 구현 가능한 기능들 (80%)

#### A. Work Mode 시스템
- **구현 방안**: CLI 상태 관리 개선
- **필요 개발**: 2-3주
- **복잡도**: 중간

```go
type CLIWorkMode struct {
    current      WorkMode
    context      map[string]string
    promptMod    string
    initialized  bool
}

func (c *CLIMode) switchWorkMode(mode WorkMode) {
    c.workMode.current = mode
    c.workMode.promptMod = getWorkModePromptModifier(mode)
    
    switch mode {
    case WorkModeEdit:
        c.autoDetectFiles()
        c.showFileContext()
    case WorkModeAgent:
        c.initializeAgentSteps()
    }
}
```

#### B. Multi-Model 텍스트 기반 구현
- **구현 방안**: 텍스트 기반 선택 인터페이스
- **필요 개발**: 3-4주
- **복잡도**: 중상

```go
func (c *CLIMode) showMultiModelSelection() {
    fmt.Println("📦 Select Models for Comparison:")
    models := c.getAvailableModels()
    
    for i, model := range models {
        selected := ""
        if c.isModelSelected(model.Name) {
            selected = "✓"
        }
        fmt.Printf("  [%d] %s %s - %s\n", i+1, selected, model.Name, model.Description)
    }
    
    fmt.Print("\nEnter model numbers (comma-separated): ")
}

func (c *CLIMode) runMultiModelComparison(query string) {
    responses := make(map[string]string)
    
    for _, model := range c.selectedModels {
        fmt.Printf("🤔 Asking %s...\n", model)
        response := c.queryModel(model, query)
        responses[model] = response
    }
    
    c.displayComparison(responses)
}
```

#### C. Preset 관리 시스템
- **구현 방안**: 파일 기반 preset 저장
- **필요 개발**: 1-2주
- **복잡도**: 낮음

```go
type CLIPresetManager struct {
    presetFile string
    presets    map[string]CLIPreset
}

type CLIPreset struct {
    Name        string            `json:"name"`
    Models      []string          `json:"models"`
    WorkMode    WorkMode          `json:"work_mode"`
    Context     map[string]string `json:"context"`
    CreatedAt   time.Time         `json:"created_at"`
}

func (c *CLIMode) savePreset(name string) {
    preset := CLIPreset{
        Name:      name,
        Models:    c.selectedModels,
        WorkMode:  c.workMode.current,
        Context:   c.workMode.context,
        CreatedAt: time.Now(),
    }
    
    c.presetManager.Save(preset)
    fmt.Printf("✓ Saved preset '%s'\n", name)
}
```

### 🟡 3.2 부분 구현 가능한 기능들 (15%)

#### A. 단순화된 인터랙티브 메뉴
- **구현 방안**: 번호 선택 기반 메뉴
- **제약사항**: 실시간 업데이트 불가

```go
func (c *CLIMode) showInteractiveMenu(title string, options []string) int {
    fmt.Printf("\n%s\n", title)
    for i, option := range options {
        fmt.Printf("  [%d] %s\n", i+1, option)
    }
    fmt.Print("\nSelect option: ")
    
    var choice int
    fmt.Scanf("%d", &choice)
    return choice - 1
}
```

#### B. 텍스트 기반 프로그레스 표시
- **구현 방안**: ASCII 프로그레스바
- **제약사항**: 제한적 시각적 피드백

```go
func (c *CLIMode) showProgress(percent float64) {
    width := 50
    filled := int(percent * float64(width) / 100)
    bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
    fmt.Printf("\r[%s] %.1f%%", bar, percent)
}
```

### 🔴 3.3 구현 불가능한 기능들 (5%)

#### A. 실시간 키보드 인터랙션
- **제약사항**: CLI는 enter 기반 입력만 가능
- **대안**: 단축 명령어로 대체

#### B. 동시 다중 화면 렌더링
- **제약사항**: 터미널의 선형 출력 특성
- **대안**: 순차적 정보 표시

## 🚀 4. CLI-TUI 통합 업그레이드 상세 로드맵

### 📅 Phase 1: Work Mode 시스템 구현 (2-3주)

#### Week 1: 핵심 상태 관리
```go
// 파일: internal/tui/cli_workmode.go (신규)
type CLIWorkModeManager struct {
    current     WorkMode
    context     map[string]interface{}
    promptMod   string
    fileContext []string
    agentSteps  []string
    planTasks   []string
}

func (c *CLIMode) initWorkModeManager() {
    c.workModeManager = &CLIWorkModeManager{
        current:     WorkModeAsk,
        context:     make(map[string]interface{}),
        fileContext: []string{},
        agentSteps:  []string{},
        planTasks:   []string{},
    }
}
```

#### Week 2-3: 모드별 특화 기능
```go
func (c *CLIMode) switchWorkMode(mode WorkMode) {
    c.workModeManager.current = mode
    
    switch mode {
    case WorkModeEdit:
        c.enableEditMode()
        c.detectProjectFiles()
        c.showFileContext()
    case WorkModeAgent:
        c.enableAgentMode()
        c.initializeAgentSteps()
    case WorkModePlan:
        c.enablePlanMode()
        c.showPlanningInterface()
    }
    
    c.showModeStatus()
}

func (c *CLIMode) enableEditMode() {
    fmt.Println("🔧 Edit Mode Active - Auto-detecting project files...")
    files := c.scanProjectFiles()
    c.workModeManager.fileContext = files
    
    fmt.Println("📁 Files in context:")
    for _, file := range files {
        fmt.Printf("  • %s\n", file)
    }
}
```

### 📅 Phase 2: Multi-Model 비교 시스템 (3-4주)

#### Week 1: 모델 선택 인터페이스
```go
// 파일: internal/tui/cli_multimodel.go (신규)
type CLIMultiModel struct {
    selectedModels []string
    responses      map[string]CLIModelResponse
    lastQuery      string
}

type CLIModelResponse struct {
    Content   string
    Tokens    int
    Duration  time.Duration
    Timestamp time.Time
}

func (c *CLIMode) showMultiModelSelection() {
    fmt.Println("\n📦 Multi-Model Selection Interface")
    fmt.Println("────────────────────────────────────")
    
    models := c.getAvailableModels()
    for i, model := range models {
        status := "○"
        if c.multiModel.isSelected(model.Name) {
            status = "●"
        }
        fmt.Printf("  %s [%d] %s (%s)\n", status, i+1, model.Name, model.Provider)
    }
    
    c.showMultiModelCommands()
}

func (c *CLIMode) showMultiModelCommands() {
    fmt.Println("\n📋 Commands:")
    fmt.Println("  select <numbers>     - Select models (e.g., select 1,3,5)")
    fmt.Println("  deselect <numbers>   - Deselect models")
    fmt.Println("  compare \"<query>\"     - Run comparison")
    fmt.Println("  clear                - Clear all selections")
    fmt.Println("  exit                 - Back to chat")
}
```

#### Week 2: 병렬 처리 구현
```go
func (c *CLIMode) runMultiModelComparison(query string) {
    if len(c.multiModel.selectedModels) == 0 {
        fmt.Println("⚠️ No models selected. Use 'select <numbers>' first.")
        return
    }
    
    fmt.Printf("🤔 Querying %d models in parallel...\n", len(c.multiModel.selectedModels))
    
    // 병렬 처리를 위한 goroutine
    results := make(chan CLIModelResult, len(c.multiModel.selectedModels))
    
    for _, modelName := range c.multiModel.selectedModels {
        go func(model string) {
            start := time.Now()
            response := c.queryModel(model, query)
            
            results <- CLIModelResult{
                ModelName: model,
                Response:  response,
                Duration:  time.Since(start),
                Error:     nil,
            }
        }(modelName)
    }
    
    // 결과 수집 및 표시
    c.collectAndDisplayResults(results, len(c.multiModel.selectedModels))
}

func (c *CLIMode) displayComparison(responses map[string]CLIModelResponse) {
    fmt.Println("\n📊 Multi-Model Comparison Results")
    fmt.Println("════════════════════════════════════")
    
    for modelName, response := range responses {
        fmt.Printf("\n🤖 %s (%s)\n", modelName, response.Duration.Round(time.Millisecond))
        fmt.Println("────────────────────────────────────")
        fmt.Println(response.Content)
        fmt.Printf("\n📊 Tokens: %d | Time: %s\n", 
            response.Tokens, response.Duration.Round(time.Millisecond))
    }
    
    c.showComparisonCommands()
}
```

#### Week 3-4: 고급 비교 기능
```go
func (c *CLIMode) showComparisonCommands() {
    fmt.Println("\n📋 Comparison Commands:")
    fmt.Println("  rate <model> <1-5>   - Rate a response")
    fmt.Println("  export <format>      - Export comparison (json/md)")
    fmt.Println("  rerun                - Run same query again")
    fmt.Println("  save \"<name>\"         - Save comparison as preset")
    fmt.Println("  back                 - Return to selection")
}

func (c *CLIMode) exportComparison(format string) {
    filename := fmt.Sprintf("comparison_%s.%s", 
        time.Now().Format("20060102_150405"), format)
    
    switch format {
    case "json":
        c.exportAsJSON(filename)
    case "md":
        c.exportAsMarkdown(filename)
    default:
        fmt.Println("⚠️ Supported formats: json, md")
        return
    }
    
    fmt.Printf("✓ Exported to %s\n", filename)
}
```

### 📅 Phase 3: Preset 관리 시스템 (1-2주)

#### Week 1: 기본 저장/로드
```go
// 파일: internal/tui/cli_preset.go (신규)
type CLIPresetManager struct {
    presetDir string
    presets   map[string]*CLIPreset
}

type CLIPreset struct {
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Models      []string          `json:"models"`
    WorkMode    string            `json:"work_mode"`
    Context     map[string]string `json:"context"`
    CreatedAt   time.Time         `json:"created_at"`
    UsedCount   int               `json:"used_count"`
    LastUsed    time.Time         `json:"last_used"`
}

func (c *CLIMode) saveCurrentAsPreset(name, description string) {
    preset := &CLIPreset{
        Name:        name,
        Description: description,
        Models:      c.multiModel.selectedModels,
        WorkMode:    c.workModeManager.current.String(),
        Context:     c.workModeManager.getContextMap(),
        CreatedAt:   time.Now(),
        UsedCount:   0,
    }
    
    if err := c.presetManager.Save(preset); err != nil {
        fmt.Printf("❌ Failed to save preset: %v\n", err)
        return
    }
    
    fmt.Printf("✓ Saved preset '%s' with %d models\n", name, len(preset.Models))
}
```

#### Week 2: 고급 관리 기능
```go
func (c *CLIMode) showPresetList() {
    presets := c.presetManager.ListAll()
    
    fmt.Println("\n📋 Saved Presets")
    fmt.Println("════════════════════════════════════")
    
    if len(presets) == 0 {
        fmt.Println("No presets saved yet.")
        fmt.Println("Use '/preset save <name>' to create your first preset.")
        return
    }
    
    for i, preset := range presets {
        fmt.Printf("\n[%d] %s\n", i+1, preset.Name)
        if preset.Description != "" {
            fmt.Printf("    %s\n", preset.Description)
        }
        fmt.Printf("    Models: %v\n", preset.Models)
        fmt.Printf("    Mode: %s | Used: %d times\n", preset.WorkMode, preset.UsedCount)
        fmt.Printf("    Created: %s\n", preset.CreatedAt.Format("2006-01-02 15:04"))
    }
    
    c.showPresetCommands()
}

func (c *CLIMode) loadPreset(name string) {
    preset, err := c.presetManager.Load(name)
    if err != nil {
        fmt.Printf("❌ Failed to load preset '%s': %v\n", name, err)
        return
    }
    
    // Apply preset configuration
    c.multiModel.selectedModels = preset.Models
    c.workModeManager.loadContext(preset.Context)
    
    if workMode := parseWorkMode(preset.WorkMode); workMode != -1 {
        c.switchWorkMode(workMode)
    }
    
    // Update usage statistics
    preset.UsedCount++
    preset.LastUsed = time.Now()
    c.presetManager.Save(preset)
    
    fmt.Printf("✓ Loaded preset '%s'\n", preset.Name)
    fmt.Printf("  Models: %v\n", preset.Models)
    fmt.Printf("  Mode: %s\n", preset.WorkMode)
}
```

### 📅 Phase 4: 향상된 CLI UX (1-2주)

#### Week 1: 개선된 상태 표시
```go
func (c *CLIMode) showEnhancedStatus() {
    fmt.Println()
    fmt.Println("📊 Current DevOrch Status")
    fmt.Println("════════════════════════════════════")
    
    // Work Mode 상태
    mode := c.workModeManager.current
    fmt.Printf("🎯 Work Mode: %s %s\n", mode.Icon(), mode.String())
    
    // Multi-Model 상태
    if len(c.multiModel.selectedModels) > 0 {
        fmt.Printf("📦 Models: %v (%d selected)\n", 
            c.multiModel.selectedModels, len(c.multiModel.selectedModels))
    } else {
        fmt.Printf("📦 Model: %s @ %s\n", c.model, c.provider)
    }
    
    // Context 상태
    if len(c.workModeManager.fileContext) > 0 {
        fmt.Printf("📁 Files in context: %d\n", len(c.workModeManager.fileContext))
    }
    
    // Session 상태
    fmt.Printf("💬 Messages: %d\n", len(c.messages))
    
    fmt.Println("────────────────────────────────────")
}

func (c *CLIMode) showInlineStatus() {
    // 프롬프트에 상태 정보 포함
    mode := c.workModeManager.current.Icon()
    models := ""
    
    if len(c.multiModel.selectedModels) > 1 {
        models = fmt.Sprintf("[%d models]", len(c.multiModel.selectedModels))
    } else {
        models = fmt.Sprintf("[%s]", c.model)
    }
    
    return fmt.Sprintf("%s %s > ", mode, models)
}
```

#### Week 2: 자동화 기능
```go
func (c *CLIMode) enableAutoFeatures() {
    // 자동 파일 컨텍스트 감지
    if c.workModeManager.current == WorkModeEdit {
        c.autoDetectProjectFiles()
    }
    
    // 자동 preset 제안
    if c.shouldSuggestPreset() {
        c.suggestPresetCreation()
    }
    
    // 자동 모드 전환 제안
    if c.shouldSuggestModeSwitch() {
        c.suggestModeSwitch()
    }
}

func (c *CLIMode) autoDetectProjectFiles() {
    // Git repository 감지
    if c.isInGitRepo() {
        files := c.getRecentlyModifiedFiles()
        if len(files) > 0 {
            fmt.Printf("🔍 Auto-detected %d recently modified files\n", len(files))
            c.workModeManager.fileContext = files
        }
    }
}

func (c *CLIMode) suggestPresetCreation() {
    if len(c.multiModel.selectedModels) >= 2 {
        fmt.Println()
        fmt.Println("💡 Suggestion: Save current model selection as preset?")
        fmt.Println("   Command: /preset save <name>")
    }
}
```

## 📈 5. 개발 우선순위 및 예상 효과

### 🥇 High Priority (즉시 구현 권장)

1. **Work Mode 시스템** (2-3주)
   - **가치**: 매우 높음 (TUI의 핵심 기능)
   - **복잡도**: 중간
   - **ROI**: 높음
   - **사용자 체감**: 즉시 향상

2. **Multi-Model 비교** (3-4주)
   - **가치**: 높음 (차별화 기능)
   - **복잡도**: 중상
   - **ROI**: 중상
   - **사용자 체감**: 큰 향상

### 🥈 Medium Priority

3. **Preset 관리** (1-2주)
   - **가치**: 중간 (편의성 향상)
   - **복잡도**: 낮음
   - **ROI**: 중간
   - **사용자 체감**: 편의성 향상

4. **Enhanced CLI UX** (1-2주)
   - **가치**: 중간 (사용성 개선)
   - **복잡도**: 낮음
   - **ROI**: 중간
   - **사용자 체감**: 점진적 향상

### 🥉 Low Priority (추후 고려)

5. **고급 시각화** (2-3주)
   - **가치**: 낮음 (부가 기능)
   - **복잡도**: 높음
   - **ROI**: 낮음
   - **사용자 체감**: 제한적

## 🎯 6. 최종 권장사항

### 📋 개발 순서
1. **Week 1-3**: Work Mode 시스템 구현
2. **Week 4-7**: Multi-Model 비교 시스템
3. **Week 8-9**: Preset 관리 시스템
4. **Week 10-11**: Enhanced CLI UX

### 🎪 예상 최종 CLI 기능
구현 완료 후 CLI는 다음 기능들을 제공하게 됩니다:

- ✅ **Work Mode 시스템**: 4가지 모드 + 자동 컨텍스트 관리
- ✅ **Multi-Model 비교**: 병렬 처리 + 응답 비교 + 레이팅
- ✅ **Preset 관리**: 저장/로드 + 사용량 통계 + 자동 제안
- ✅ **Enhanced UX**: 상태 표시 + 자동화 기능 + 스마트 제안

### 📊 최종 기능 동등성
구현 완료 후 **CLI는 TUI 대비 95% 기능 동등성**을 달성하게 됩니다.

남은 5%는 실시간 키보드 인터랙션 등 CLI의 근본적 제약으로 인한 부분이며, 실제 사용성에는 큰 영향을 미치지 않습니다.

---

**총 개발 기간**: 11주 (약 3개월)
**개발 복잡도**: 중상
**예상 투입 인력**: 1명 (풀타임)
**ROI**: 높음 (CLI 사용자 경험 대폭 향상)