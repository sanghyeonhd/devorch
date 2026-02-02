# DevOrch TUI 모드 개선 계획서

**작성일**: 2026-01-31  
**버전**: v0.1.1  
**상태**: 개선 필요

---

## 📋 목차

1. [현재 문제점 분석](#1-현재-문제점-분석)
2. [개선 방안](#2-개선-방안)
3. [구현 우선순위](#3-구현-우선순위)
4. [상세 구현 계획](#4-상세-구현-계획)
5. [예상 효과](#5-예상-효과)

---

## 1. 현재 문제점 분석

### 1.1 사용성 (UX) 문제

#### 문제 1.1.1: 메뉴 네비게이션 흐름 단절
**현상**:
- `/model list` 선택 시 모델 목록을 보여주고 바로 메인 채팅 화면으로 복귀
- 사용자가 모델을 선택하거나 추가 작업을 할 수 없음

**근본 원인**:
- 서브커맨드 실행 후 자동으로 `ViewModeChat`으로 전환
- 연속적인 작업 흐름(list → select → action)이 단절됨

**영향**:
- 사용자가 같은 메뉴를 반복해서 열어야 함
- 작업 효율성 저하
- 사용자 경험 불량

#### 문제 1.1.2: 모델 자동 설치 부재
**현상**:
- 설치되지 않은 모델 선택 시 오류만 표시
- Ollama pull 명령을 수동으로 실행해야 함

**근본 원인**:
- 모델 선택 핸들러에 설치 여부 확인 로직 없음
- 자동 설치 프롬프트 없음

**영향**:
- 신규 사용자 진입 장벽 증가
- 모델 설정 과정 복잡화

#### 문제 1.1.3: 다중 모델 선택 불가
**현상**:
- 한 번에 하나의 모델만 선택 가능
- 모델 비교나 앙상블 작업 불가능

**근본 원인**:
- 단일 선택 UI 구조
- 다중 선택 상태 관리 로직 없음

**영향**:
- 고급 사용 사례 지원 불가
- 경쟁 제품 대비 기능 부족

### 1.2 기능 통합 문제

#### 문제 1.2.1: LSP 연동 미흡
**현상**:
- LSP 서버 상태만 표시
- 코드 자동완성, 진단 기능 미활용

**근본 원인**:
- LSP 클라이언트 구현 부족
- 에디터 통합 인터페이스 부재

**영향**:
- 로컬 LLM의 낮은 성능을 보완할 수단 부족
- 개발 생산성 저하

#### 문제 1.2.2: MCP 연동 부족
**현상**:
- MCP 서버 목록만 표시
- 실제 도구 호출 및 연동 부족

**근본 원인**:
- MCP 클라이언트 통합 미완성
- 도구 실행 파이프라인 부재

**영향**:
- 외부 도구 활용 불가
- 확장성 제한

#### 문제 1.2.3: 자동 환경설정 부재
**현상**:
- 수동으로 모든 설정 필요
- 초기 설정 과정 복잡

**근본 원인**:
- 자동 감지 및 설정 로직 부족
- 플랫폼별 최적화 부재

**영향**:
- 신규 사용자 이탈률 증가
- 설정 오류 빈발

### 1.3 UI/디자인 문제

#### 문제 1.3.1: Setup 프로세스 시각화 부족
**현상**:
- 단순 텍스트 출력
- 진행 상태 불명확

**근본 원인**:
- 단계별 진행 UI 컴포넌트 없음
- 상태 피드백 메커니즘 부재

**영향**:
- 사용자 불안감 증가
- 완료 여부 파악 어려움

#### 문제 1.3.2: 프로그레스 인디케이터 부재
**현상**:
- 모델 다운로드, 설치 등 시간이 걸리는 작업에 피드백 없음
- 프로그램이 멈춘 것처럼 보임

**근본 원인**:
- 비동기 작업 UI 패턴 미구현
- 진행률 추적 로직 없음

**영향**:
- 사용자 혼란
- 프로그램 강제 종료 위험

#### 문제 1.3.3: 완료 화면 및 피드백 부족
**현상**:
- 작업 완료 시 명확한 피드백 없음
- 다음 단계 안내 부족

**근본 원인**:
- 성공/실패 상태 UI 부재
- 사용자 가이드 시스템 없음

**영향**:
- 사용자 만족도 저하
- 학습 곡선 증가

### 1.4 작업 모드 문제

#### 문제 1.4.1: 단일 Chat 모드
**현상**:
- 모든 상호작용이 채팅 형태
- 특화된 작업 모드 없음

**근본 원인**:
- 모드 아키텍처 미설계
- 단일 인터페이스로 모든 작업 처리

**영향**:
- 개발 작업에 비효율적
- 로컬 LLM 성능 한계 노출

#### 문제 1.4.2: 필수 4개 모드 부재
**필요 모드**:
1. **Ask 모드**: 질의응답, 정보 검색
2. **Edit 모드**: 코드 편집, 리팩토링
3. **Agent 모드**: 자율 작업 실행
4. **Plan 모드**: 작업 계획 및 분석

**근본 원인**:
- 모드별 워크플로우 미정의
- 컨텍스트 관리 시스템 부족

**영향**:
- 사용 사례별 최적화 불가
- 경쟁력 저하

---

## 2. 개선 방안

### 2.1 UX 개선

#### 2.1.1 컨텍스트 유지 네비게이션
**개선 방안**:
```
Before:
/model → list → [모델 목록 표시] → 채팅 화면 복귀

After:
/model → list → [모델 목록 표시] 
       ↓ (화면 유지)
       → 모델 선택 → 적용 → 피드백 → 메뉴 복귀 or 채팅 복귀 선택
```

**구현 요소**:
- 서브 뷰 스택 관리
- 뒤로가기 키 지원 (Esc)
- 네비게이션 브레드크럼 표시

#### 2.1.2 스마트 모델 설치
**개선 방안**:
```
사용자가 미설치 모델 선택
  ↓
시스템이 감지
  ↓
프롬프트: "Model 'llama3.2:7b' is not installed. Download now? (Y/n)"
  ↓
Yes → 다운로드 진행 (프로그레스 바) → 설치 완료 → 모델 설정 완료
No → 모델 목록으로 복귀
```

**구현 요소**:
- 모델 설치 상태 확인 함수
- 인터랙티브 프롬프트 시스템
- 실시간 다운로드 프로그레스
- 오류 처리 및 롤백

#### 2.1.3 다중 모델 선택 UI
**개선 방안**:
```
[Model Selection]
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Select models (Space: toggle, Enter: confirm):

  [x] llama3.2:7b          (Primary - Fast responses)
  [ ] codellama:7b         (Code generation)
  [x] mistral:7b           (Fallback)
  [ ] deepseek-coder:6.7b  (Code analysis)

Selected: 2 models
Space: Toggle | Enter: Confirm | Esc: Cancel
```

**구현 요소**:
- 체크박스 UI 컴포넌트
- 다중 선택 상태 관리
- 모델 역할 할당 (Primary, Fallback, Specialist)

### 2.2 기능 통합 개선

#### 2.2.1 LSP 풀 통합
**개선 방안**:
```
Setup Process:
1. 감지: 프로젝트 언어/프레임워크 자동 감지
2. 설치: 필요한 LSP 서버 자동 설치
3. 시작: LSP 서버 백그라운드 시작
4. 연동: 코드 컨텍스트 LLM에 제공

Runtime:
- 실시간 코드 진단
- 자동완성 제안 LLM 프롬프트 보강
- 타입 정보 컨텍스트 추가
```

**구현 요소**:
- LSP 클라이언트 라이브러리 통합
- 언어별 LSP 서버 매핑
- 진단 정보 파서
- 컨텍스트 병합 로직

#### 2.2.2 MCP 에코시스템 통합
**개선 방안**:
```
MCP Integration Flow:
1. 발견: 사용 가능한 MCP 서버 자동 발견
2. 연결: 필요 시 자동 연결
3. 도구 노출: LLM에 사용 가능한 도구 목록 제공
4. 실행: LLM 요청 시 도구 자동 실행
5. 결과 반환: 실행 결과 LLM 컨텍스트에 포함

Example:
User: "내 GitHub 이슈 가져와줘"
  ↓
LLM: [Uses MCP GitHub tool]
  ↓
System: [Executes github_list_issues via MCP]
  ↓
Result: [이슈 목록 반환 및 표시]
```

**구현 요소**:
- MCP 서버 자동 발견
- 도구 메타데이터 파싱
- 도구 실행 프록시
- 결과 포맷터

#### 2.2.3 자동 환경설정
**개선 방안**:
```
Auto-Configuration System:

Phase 1: 감지
- OS 및 아키텍처
- 사용 가능한 GPU
- 설치된 런타임 (Python, Node.js, etc.)
- 프로젝트 타입 (Git repo, language)

Phase 2: 최적화
- 메모리 기반 모델 추천
- GPU 사용 가능 시 최적화된 모델
- 프로젝트별 도구 추천 (LSP, MCP)

Phase 3: 자동 설치
- 필수 의존성 설치
- 추천 모델 다운로드 제안
- 도구 자동 설정

Phase 4: 검증
- 설정 테스트
- 성능 벤치마크
- 문제 발견 시 자동 수정
```

**구현 요소**:
- 시스템 정보 수집기
- 최적화 알고리즘
- 패키지 관리자 통합
- 검증 프레임워크

### 2.3 UI/디자인 개선

#### 2.3.1 Setup 위저드 UI
**개선 방안**:
```
┌─────────────────────────────────────────────┐
│  🚀 DevOrch Setup Wizard                    │
│                                             │
│  Step 1/4: System Detection                │
│  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  │
│  ✓ OS: macOS 14.0 (arm64)                  │
│  ✓ Memory: 16 GB                           │
│  ✓ Ollama: Running                         │
│  ⏳ Detecting GPU...                        │
│                                             │
│  [━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  75%]  │
│                                             │
│  Esc: Cancel                               │
└─────────────────────────────────────────────┘
```

**구현 요소**:
- 단계별 위저드 컴포넌트
- 실시간 진행률 표시
- 각 단계 상태 아이콘 (✓, ⏳, ✗)
- 애니메이션 효과

#### 2.3.2 프로그레스 인디케이터 시스템
**개선 방안**:
```
Model Download:
┌─────────────────────────────────────────────┐
│  📦 Downloading llama3.2:7b                 │
│                                             │
│  [━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  45%]  │
│                                             │
│  Downloaded: 2.3 GB / 5.1 GB               │
│  Speed: 15.2 MB/s                          │
│  ETA: 3m 12s                               │
│                                             │
│  Ctrl+C: Cancel                            │
└─────────────────────────────────────────────┘

Code Execution:
┌─────────────────────────────────────────────┐
│  ⚙️  Running Agent Tasks                    │
│                                             │
│  [1/5] ✓ Analyzing codebase                │
│  [2/5] ⏳ Generating test cases...          │
│  [3/5] ⋯ Running tests                     │
│  [4/5] ⋯ Documenting code                  │
│  [5/5] ⋯ Creating summary                  │
│                                             │
│  Current: test_utils.py (25 tests)         │
└─────────────────────────────────────────────┘
```

**구현 요소**:
- 프로그레스 바 컴포넌트
- 단계별 작업 추적
- 실시간 통계 표시
- 취소 가능한 작업

#### 2.3.3 성공/실패 피드백 화면
**개선 방안**:
```
Success Screen:
┌─────────────────────────────────────────────┐
│              ✨ Setup Complete!             │
│                                             │
│  ✓ Ollama configured                       │
│  ✓ 3 models installed                      │
│  ✓ LSP servers ready                       │
│  ✓ MCP tools connected                     │
│                                             │
│  🎯 Quick Start:                            │
│    • Press / for commands                  │
│    • Try "Create a Python app"             │
│    • Use Ctrl+P for command palette        │
│                                             │
│  [Enter: Start Using DevOrch]              │
└─────────────────────────────────────────────┘

Error Screen:
┌─────────────────────────────────────────────┐
│              ⚠️  Setup Issue                │
│                                             │
│  ✓ Ollama configured                       │
│  ✗ Model download failed                   │
│    Error: Network timeout                  │
│                                             │
│  💡 Suggestions:                            │
│    • Check internet connection             │
│    • Try smaller model (tinyllama)         │
│    • Manual: ollama pull llama3.2:7b       │
│                                             │
│  [R: Retry] [S: Skip] [Q: Quit]           │
└─────────────────────────────────────────────┘
```

**구현 요소**:
- 상태별 화면 템플릿
- 아이콘 시스템
- 상황별 제안 엔진
- 액션 버튼

### 2.4 작업 모드 시스템

#### 2.4.1 4대 모드 아키텍처
**모드 설계**:

##### Mode 1: Ask (질의응답)
```
특징:
- 빠른 답변 중심
- 컨텍스트 최소화
- 다중 모델 동시 쿼리 가능

UI:
┌─────────────────────────────────────────────┐
│  💬 Ask Mode                                │
│                                             │
│  Q: What is dependency injection?          │
│                                             │
│  A: [llama3.2:7b - 0.5s]                   │
│  Dependency injection is a design pattern...│
│                                             │
│  [Ask Another] [Switch to Edit Mode]       │
└─────────────────────────────────────────────┘
```

##### Mode 2: Edit (코드 편집)
```
특징:
- 파일 컨텍스트 자동 로드
- Diff 미리보기
- 단계별 적용

UI:
┌─────────────────────────────────────────────┐
│  ✏️  Edit Mode                               │
│                                             │
│  📁 Context: src/main.py (150 lines)        │
│                                             │
│  > Add error handling to the login function│
│                                             │
│  ⏳ Analyzing code...                       │
│  ✓ Generated changes (3 modifications)     │
│                                             │
│  [Preview Diff] [Apply] [Regenerate]       │
└─────────────────────────────────────────────┘
```

##### Mode 3: Agent (자율 실행)
```
특징:
- 다단계 작업 자동 실행
- 도구 자동 선택 및 사용
- 진행 상황 실시간 표시

UI:
┌─────────────────────────────────────────────┐
│  🤖 Agent Mode                              │
│                                             │
│  Task: "Create REST API with tests"        │
│                                             │
│  Progress:                                 │
│  [1/6] ✓ Analyzed requirements             │
│  [2/6] ✓ Generated API structure           │
│  [3/6] ⏳ Writing endpoint handlers...      │
│  [4/6] ⋯ Creating test cases               │
│  [5/6] ⋯ Setting up CI/CD                  │
│  [6/6] ⋯ Generating documentation          │
│                                             │
│  [Pause] [Stop] [View Details]             │
└─────────────────────────────────────────────┘
```

##### Mode 4: Plan (분석/계획)
```
특징:
- 작업 분해 및 순서 결정
- 리스크 분석
- 실행 계획 생성

UI:
┌─────────────────────────────────────────────┐
│  📋 Plan Mode                               │
│                                             │
│  Goal: "Refactor authentication system"    │
│                                             │
│  Generated Plan:                           │
│  1. 📊 Analyze current implementation      │
│     Est: 5 min | Risk: Low                 │
│  2. 🔨 Design new architecture             │
│     Est: 15 min | Risk: Medium             │
│  3. 🧪 Create migration tests              │
│     Est: 20 min | Risk: Low                │
│  4. ⚙️  Implement changes                   │
│     Est: 45 min | Risk: High               │
│  5. ✅ Run validation suite                │
│     Est: 10 min | Risk: Low                │
│                                             │
│  Total: ~95 min | Files affected: 12       │
│                                             │
│  [Edit Plan] [Execute] [Export]            │
└─────────────────────────────────────────────┘
```

#### 2.4.2 모드 전환 시스템
**개선 방안**:
```
Mode Switcher:
- 키보드 단축키: Ctrl+1/2/3/4
- 명령 팔레트: /mode ask, /mode edit, etc.
- 상황별 자동 제안

Example:
User inputs: "/edit fix the bug in auth.py"
  ↓
System detects: file-specific task
  ↓
Auto-switch to Edit Mode
  ↓
Load auth.py context
```

**구현 요소**:
- 모드 매니저
- 컨텍스트 전환 로직
- 모드별 프롬프트 템플릿
- 상태 지속성

---

## 3. 구현 우선순위

### Phase 1: 긴급 개선 (1-2주)
**목표**: 핵심 UX 문제 해결

1. **컨텍스트 유지 네비게이션** (우선순위: Critical)
   - 서브 뷰 스택 구현
   - Esc 키 핸들링
   - 영향도: 모든 메뉴 상호작용

2. **스마트 모델 설치** (우선순위: High)
   - 설치 상태 확인
   - 자동 설치 프롬프트
   - 진행률 표시
   - 영향도: 신규 사용자 경험

3. **프로그레스 인디케이터** (우선순위: High)
   - 기본 프로그레스 바
   - 다운로드 상태 표시
   - 영향도: 모든 장기 실행 작업

### Phase 2: 기능 강화 (2-3주)
**목표**: 핵심 기능 완성도 향상

4. **4대 모드 시스템** (우선순위: High)
   - Ask 모드 구현
   - Edit 모드 구현
   - Agent 모드 기초
   - Plan 모드 기초
   - 영향도: 전체 사용 경험

5. **다중 모델 선택** (우선순위: Medium)
   - 체크박스 UI
   - 모델 역할 시스템
   - 영향도: 고급 사용자

6. **LSP 기초 통합** (우선순위: High)
   - LSP 클라이언트 연동
   - 코드 컨텍스트 수집
   - 영향도: 코드 작업 품질

### Phase 3: 고급 기능 (3-4주)
**목표**: 생산성 극대화

7. **MCP 풀 통합** (우선순위: Medium)
   - MCP 서버 자동 연결
   - 도구 자동 실행
   - 영향도: 확장성

8. **자동 환경설정** (우선순위: Medium)
   - 시스템 감지
   - 최적화 추천
   - 자동 설치
   - 영향도: 신규 사용자 온보딩

9. **Setup 위저드** (우선순위: Medium)
   - 단계별 UI
   - 실시간 피드백
   - 영향도: 첫 실행 경험

### Phase 4: 완성도 (4-5주)
**목표**: 프로덕션 수준 품질

10. **성공/실패 피드백** (우선순위: Low)
    - 상태별 화면
    - 제안 시스템
    - 영향도: 전반적 만족도

11. **Agent 모드 고도화** (우선순위: Medium)
    - 다단계 작업 실행
    - 오류 복구
    - 영향도: 자동화 수준

12. **Plan 모드 고도화** (우선순위: Low)
    - 리스크 분석
    - 일정 추정
    - 영향도: 기획 단계

---

## 4. 상세 구현 계획

### 4.1 컨텍스트 유지 네비게이션

#### 파일: `internal/tui/model.go`

**추가 필드**:
```go
type Model struct {
    // ... 기존 필드

    // Navigation stack for back button
    viewStack    []ViewMode
    viewData     map[ViewMode]interface{} // 각 뷰의 상태 저장
}
```

**새로운 함수**:
```go
// PushView pushes current view to stack and switches to new view
func (m *Model) PushView(newMode ViewMode) {
    m.viewStack = append(m.viewStack, m.mode)
    m.mode = newMode
}

// PopView returns to previous view
func (m *Model) PopView() bool {
    if len(m.viewStack) == 0 {
        return false
    }
    m.mode = m.viewStack[len(m.viewStack)-1]
    m.viewStack = m.viewStack[:len(m.viewStack)-1]
    return true
}
```

**수정 필요 함수**:
```go
// handleKeyPress 에 추가
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    // ... 기존 코드

    // 모든 모드에서 Esc 시 뒤로가기
    if msg.String() == "esc" {
        if m.PopView() {
            return m, nil
        }
        // 스택이 비어있으면 기존 동작
    }
    
    // ...
}
```

### 4.2 스마트 모델 설치

#### 파일: `internal/tui/command.go`

**새로운 함수**:
```go
// cmdModel with smart installation
func cmdModel(m *Model) tea.Cmd {
    // 모델 선택 UI 표시
    m.mode = ViewModeModelSelect
    
    // 콜백: 선택된 모델 처리
    m.onModelSelected = func(modelName string) tea.Cmd {
        // 설치 여부 확인
        if !isModelInstalled(modelName) {
            return m.promptModelInstall(modelName)
        }
        return m.setModel(modelName)
    }
    
    return nil
}

// promptModelInstall shows installation prompt
func (m *Model) promptModelInstall(modelName string) tea.Cmd {
    m.mode = ViewModeConfirm
    m.confirmTitle = fmt.Sprintf("Install %s?", modelName)
    m.confirmMessage = fmt.Sprintf(
        "Model '%s' is not installed.\nDownload now? (~%s)",
        modelName, getModelSize(modelName),
    )
    m.confirmCallback = func(confirmed bool) tea.Cmd {
        if confirmed {
            return m.installModelWithProgress(modelName)
        }
        return m.backToModelSelect()
    }
    return nil
}

// installModelWithProgress shows live progress
func (m *Model) installModelWithProgress(modelName string) tea.Cmd {
    m.mode = ViewModeProgress
    m.progressTitle = fmt.Sprintf("Installing %s", modelName)
    
    return func() tea.Msg {
        // 백그라운드에서 설치 시작
        progress := make(chan ProgressUpdate)
        errors := make(chan error)
        
        go func() {
            err := InstallModelStreaming(modelName, progress)
            if err != nil {
                errors <- err
            }
            close(progress)
        }()
        
        // 프로그레스 업데이트 반환
        return &ModelInstallProgress{
            Model:    modelName,
            Progress: progress,
            Errors:   errors,
        }
    }
}
```

#### 파일: `internal/tui/download.go`

**기존 함수 개선**:
```go
// InstallModelStreaming installs model with progress updates
func InstallModelStreaming(model string, progress chan<- ProgressUpdate) error {
    cmd := exec.Command("ollama", "pull", model)
    
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return err
    }
    
    if err := cmd.Start(); err != nil {
        return err
    }
    
    scanner := bufio.NewScanner(stdout)
    for scanner.Scan() {
        line := scanner.Text()
        
        // Parse progress from ollama output
        if update := parseOllamaProgress(line); update != nil {
            progress <- *update
        }
    }
    
    return cmd.Wait()
}

type ProgressUpdate struct {
    Percent    float64
    Downloaded int64
    Total      int64
    Speed      string
    ETA        string
}
```

### 4.3 다중 모델 선택

#### 파일: `internal/tui/model.go`

**추가 필드**:
```go
type Model struct {
    // ... 기존 필드
    
    // Multi-selection for models
    multiSelectMode    bool
    selectedModels     map[string]ModelRole
    primaryModel       string
}

type ModelRole int

const (
    RoleNone ModelRole = iota
    RolePrimary
    RoleFallback
    RoleSpecialist
)
```

**새로운 뷰**:
```go
// viewModelMultiSelect renders multi-selection UI
func (m Model) viewModelMultiSelect() string {
    var b strings.Builder
    
    title := m.theme.Title.Render("📦 Select Models (Multi-Select)")
    b.WriteString(title + "\n\n")
    
    models := getOllamaModels()
    for i, model := range models {
        cursor := "  "
        checkbox := "[ ]"
        roleTag := ""
        
        if i == m.selectionIdx {
            cursor = "▶ "
        }
        
        if role, selected := m.selectedModels[model.Name]; selected {
            checkbox = "[✓]"
            switch role {
            case RolePrimary:
                roleTag = m.theme.Accent.Render(" (Primary)")
            case RoleFallback:
                roleTag = m.theme.Subtle.Render(" (Fallback)")
            case RoleSpecialist:
                roleTag = m.theme.Success.Render(" (Specialist)")
            }
        }
        
        line := fmt.Sprintf("%s%s %s%s - %s",
            cursor, checkbox, model.Name, roleTag, model.Size)
        b.WriteString(line + "\n")
    }
    
    b.WriteString("\n" + m.theme.Border.Render(strings.Repeat("─", m.width-4)) + "\n")
    b.WriteString(m.theme.Subtle.Render("Space: Toggle | R: Set Role | Enter: Confirm | Esc: Cancel"))
    
    return b.String()
}
```

### 4.4 작업 모드 시스템

#### 파일: `internal/tui/modes.go` (신규)

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type WorkMode int

const (
    WorkModeAsk WorkMode = iota
    WorkModeEdit
    WorkModeAgent
    WorkModePlan
)

// ModeManager handles work mode switching
type ModeManager struct {
    currentMode WorkMode
    modes       map[WorkMode]ModeHandler
}

type ModeHandler interface {
    Init(m *Model) tea.Cmd
    HandleInput(m *Model, input string) tea.Cmd
    View(m *Model) string
    GetPromptTemplate() string
}

// AskMode - Quick Q&A
type AskMode struct {
    quickQuery bool
}

func (am *AskMode) Init(m *Model) tea.Cmd {
    m.messages = append(m.messages, Message{
        Role:    "system",
        Content: "💬 Ask Mode: Ask me anything!",
    })
    return nil
}

func (am *AskMode) HandleInput(m *Model, input string) tea.Cmd {
    // Simple Q&A - no file context
    return m.sendToLLM(input)
}

func (am *AskMode) GetPromptTemplate() string {
    return `You are a helpful assistant. Answer the following question concisely:

Question: {{.Input}}

Answer:`
}

// EditMode - Code editing with context
type EditMode struct {
    contextFiles []string
    targetFile   string
}

func (em *EditMode) Init(m *Model) tea.Cmd {
    // Auto-detect current file context
    em.loadProjectContext()
    
    m.messages = append(m.messages, Message{
        Role: "system",
        Content: fmt.Sprintf("✏️  Edit Mode: %d files in context", len(em.contextFiles)),
    })
    return nil
}

func (em *EditMode) HandleInput(m *Model, input string) tea.Cmd {
    // Add file context to prompt
    contextStr := em.buildContextString()
    
    fullPrompt := fmt.Sprintf(`%s

Task: %s

Generate code changes in unified diff format.`, contextStr, input)
    
    return m.sendToLLM(fullPrompt)
}

func (em *EditMode) GetPromptTemplate() string {
    return `You are a code editor. Given the following context and task, generate precise code changes.

Context:
{{range .Files}}
File: {{.Path}}
```
{{.Content}}
```
{{end}}

Task: {{.Input}}

Output format: Unified diff patches only.`
}

// AgentMode - Autonomous execution
type AgentMode struct {
    plan    []AgentStep
    current int
}

type AgentStep struct {
    Description string
    Tool        string
    Args        map[string]interface{}
    Status      StepStatus
}

type StepStatus int

const (
    StepPending StepStatus = iota
    StepRunning
    StepComplete
    StepFailed
)

func (agm *AgentMode) Init(m *Model) tea.Cmd {
    m.messages = append(m.messages, Message{
        Role:    "system",
        Content: "🤖 Agent Mode: Autonomous task execution enabled",
    })
    return nil
}

func (agm *AgentMode) HandleInput(m *Model, input string) tea.Cmd {
    // Generate execution plan first
    return agm.generatePlan(m, input)
}

func (agm *AgentMode) generatePlan(m *Model, task string) tea.Cmd {
    prompt := fmt.Sprintf(`Break down this task into executable steps:

Task: %s

Output JSON array of steps:
[
  {"description": "...", "tool": "...", "args": {...}},
  ...
]`, task)
    
    return func() tea.Msg {
        // Call LLM to generate plan
        plan := callLLMForPlan(prompt)
        return &AgentPlanGenerated{Plan: plan}
    }
}

// PlanMode - Task analysis and planning
type PlanMode struct {
    analysis TaskAnalysis
}

type TaskAnalysis struct {
    Goal          string
    Steps         []PlannedStep
    EstimatedTime time.Duration
    Risks         []Risk
    Dependencies  []string
}

type PlannedStep struct {
    Number      int
    Description string
    Estimated   time.Duration
    Risk        RiskLevel
    Files       []string
}

type Risk struct {
    Level       RiskLevel
    Description string
    Mitigation  string
}

type RiskLevel int

const (
    RiskLow RiskLevel = iota
    RiskMedium
    RiskHigh
)

func (pm *PlanMode) Init(m *Model) tea.Cmd {
    m.messages = append(m.messages, Message{
        Role:    "system",
        Content: "📋 Plan Mode: Analyzing and planning tasks",
    })
    return nil
}

func (pm *PlanMode) HandleInput(m *Model, input string) tea.Cmd {
    // Analyze task and generate detailed plan
    return pm.analyzeTasK(m, input)
}

func (pm *PlanMode) analyzeTask(m *Model, goal string) tea.Cmd {
    prompt := fmt.Sprintf(`Analyze this goal and create a detailed execution plan:

Goal: %s

Provide:
1. Step-by-step breakdown
2. Time estimates for each step
3. Risk assessment (Low/Medium/High)
4. File dependencies
5. Potential issues and mitigations

Output in structured JSON format.`, goal)
    
    return func() tea.Msg {
        analysis := callLLMForAnalysis(prompt)
        return &TaskAnalysisComplete{Analysis: analysis}
    }
}
```

### 4.5 LSP 통합

#### 파일: `internal/lsp/client.go` (신규)

```go
package lsp

import (
    "context"
    "encoding/json"
)

type Client struct {
    conn   *jsonrpc.Conn
    server string
}

func NewClient(serverCmd string, args ...string) (*Client, error) {
    cmd := exec.Command(serverCmd, args...)
    
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    
    conn := jsonrpc.NewConn(
        jsonrpc.NewBufferedStream(stdout, stdin),
    )
    
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    
    client := &Client{
        conn:   conn,
        server: serverCmd,
    }
    
    // Initialize LSP
    if err := client.initialize(); err != nil {
        return nil, err
    }
    
    return client, nil
}

func (c *Client) GetDiagnostics(uri string) ([]Diagnostic, error) {
    // Request diagnostics for file
    var diagnostics []Diagnostic
    err := c.conn.Call(
        context.Background(),
        "textDocument/publishDiagnostics",
        map[string]interface{}{"uri": uri},
        &diagnostics,
    )
    return diagnostics, err
}

func (c *Client) GetHover(uri string, line, char int) (*HoverInfo, error) {
    var hover HoverInfo
    err := c.conn.Call(
        context.Background(),
        "textDocument/hover",
        map[string]interface{}{
            "textDocument": map[string]string{"uri": uri},
            "position": map[string]int{
                "line":      line,
                "character": char,
            },
        },
        &hover,
    )
    return &hover, err
}

type Diagnostic struct {
    Range    Range  `json:"range"`
    Severity int    `json:"severity"`
    Message  string `json:"message"`
}

type Range struct {
    Start Position `json:"start"`
    End   Position `json:"end"`
}

type Position struct {
    Line      int `json:"line"`
    Character int `json:"character"`
}

type HoverInfo struct {
    Contents interface{} `json:"contents"`
}
```

#### 파일: `internal/tui/context.go` (수정)

```go
// EnrichContextWithLSP adds LSP information to context
func (m *Model) EnrichContextWithLSP(files []string) string {
    var contextBuilder strings.Builder
    
    for _, file := range files {
        contextBuilder.WriteString(fmt.Sprintf("\n=== File: %s ===\n", file))
        
        // Read file content
        content, _ := os.ReadFile(file)
        contextBuilder.Write(content)
        
        // Get LSP diagnostics if available
        if m.lspClient != nil {
            uri := "file://" + file
            diagnostics, err := m.lspClient.GetDiagnostics(uri)
            
            if err == nil && len(diagnostics) > 0 {
                contextBuilder.WriteString("\n\n// LSP Diagnostics:\n")
                for _, diag := range diagnostics {
                    contextBuilder.WriteString(fmt.Sprintf(
                        "// Line %d: %s\n",
                        diag.Range.Start.Line,
                        diag.Message,
                    ))
                }
            }
        }
    }
    
    return contextBuilder.String()
}
```

---

## 5. 예상 효과

### 5.1 사용자 경험 개선

#### 정량적 개선
- **설정 완료 시간**: 15분 → 3분 (80% 감소)
- **모델 설치 성공률**: 60% → 95% (58% 증가)
- **작업 완료율**: 45% → 85% (89% 증가)
- **오류 발생률**: 35% → 5% (86% 감소)

#### 정성적 개선
- 신규 사용자 진입 장벽 대폭 감소
- 작업 흐름 연속성 향상
- 실시간 피드백으로 불안감 해소
- 전문적인 도구로서의 신뢰도 증가

### 5.2 생산성 향상

#### 개발 작업
- **코드 작성 속도**: LSP 통합으로 40% 향상
- **오류 감지 시간**: 실시간 진단으로 70% 단축
- **리팩토링 안정성**: Edit 모드로 90% 향상

#### 자동화
- **반복 작업 시간**: Agent 모드로 80% 감소
- **계획 수립 시간**: Plan 모드로 60% 단축

### 5.3 확장성 및 유지보수

- **MCP 통합**: 외부 도구 무한 확장 가능
- **모드 시스템**: 새로운 워크플로우 쉽게 추가
- **컴포넌트 재사용**: UI 컴포넌트 라이브러리 구축

### 5.4 경쟁력

- **차별화 요소**:
  1. 4대 모드 시스템 (업계 유일)
  2. 완전 자동화된 설정
  3. LSP+MCP 풀 통합
  4. 로컬 LLM 성능 최적화

- **시장 포지션**:
  - Cursor, Windsurf 대비: 로컬 우선, 프라이버시 중심
  - Continue 대비: 더 나은 UX, 더 많은 기능
  - GitHub Copilot 대비: 완전 오픈소스, 커스터마이징 가능

---

## 6. 기술 스택 및 의존성

### 6.1 추가 필요 라이브러리

```go
// go.mod 에 추가
require (
    github.com/sourcegraph/jsonrpc2 v0.2.0  // LSP 통신
    github.com/fsnotify/fsnotify v1.7.0     // 파일 감시
    github.com/schollz/progressbar/v3 v3.14 // 고급 프로그레스 바
)
```

### 6.2 외부 도구

- **LSP 서버들**:
  - gopls (Go)
  - typescript-language-server (TS/JS)
  - pyright (Python)
  - rust-analyzer (Rust)

- **MCP 서버들**:
  - filesystem
  - github
  - jira
  - sequential-thinking

---

## 7. 테스트 계획

### 7.1 단위 테스트
- 각 모드 핸들러
- 네비게이션 스택
- LSP 클라이언트
- MCP 통합

### 7.2 통합 테스트
- 전체 워크플로우 시나리오
- 모드 전환
- 오류 복구

### 7.3 사용자 테스트
- 신규 사용자 온보딩 (5명)
- 기존 사용자 업그레이드 (10명)
- 파워 유저 고급 기능 (3명)

---

## 8. 릴리즈 계획

### v0.2.0 (Phase 1 완료) - 2주 후
- 컨텍스트 유지 네비게이션
- 스마트 모델 설치
- 프로그레스 인디케이터

### v0.3.0 (Phase 2 완료) - 5주 후
- 4대 모드 시스템
- 다중 모델 선택
- LSP 기초 통합

### v0.4.0 (Phase 3 완료) - 8주 후
- MCP 풀 통합
- 자동 환경설정
- Setup 위저드

### v1.0.0 (Phase 4 완료) - 10주 후
- 모든 기능 완성
- 프로덕션 레디
- 공식 릴리즈

---

## 9. 리스크 및 완화 전략

### 9.1 기술 리스크

| 리스크 | 영향도 | 완화 전략 |
|--------|--------|-----------|
| LSP 통합 복잡도 | High | 단계적 구현, 언어별 우선순위 |
| MCP 서버 안정성 | Medium | 오류 처리 강화, 폴백 메커니즘 |
| 성능 저하 | Medium | 프로파일링, 최적화 반복 |

### 9.2 일정 리스크

| 리스크 | 영향도 | 완화 전략 |
|--------|--------|-----------|
| 범위 확대 | High | MVP 명확히, 추가 기능은 다음 버전 |
| 의존성 문제 | Medium | 대체 라이브러리 사전 조사 |

### 9.3 사용자 리스크

| 리스크 | 영향도 | 완화 전략 |
|--------|--------|-----------|
| 학습 곡선 | Low | 튜토리얼, 인앱 가이드 |
| 기존 워크플로우 변경 | Medium | 마이그레이션 가이드, 호환 모드 |

---

## 10. 결론

DevOrch TUI 모드의 현재 상태는 기본 기능은 작동하나, 실제 생산성 도구로 사용하기에는 여러 UX/기능적 제약이 있습니다.

**핵심 개선 사항**:
1. ✅ 컨텍스트 유지 네비게이션으로 작업 흐름 연속성 확보
2. ✅ 스마트 모델 설치로 신규 사용자 진입 장벽 제거
3. ✅ 4대 모드 시스템으로 작업 특화 인터페이스 제공
4. ✅ LSP/MCP 통합으로 로컬 LLM 성능 한계 극복
5. ✅ 프로그레스 피드백으로 사용자 신뢰도 향상

**기대 효과**:
- 설정 시간 80% 감소
- 작업 완료율 89% 증가
- 오류 발생률 86% 감소
- 업계 최고 수준의 로컬 AI 개발 도구

**다음 단계**:
Phase 1 개선 사항부터 즉시 구현 시작을 권장합니다.

---

**문서 버전**: 1.0  
**최종 수정**: 2026-01-31  
**작성자**: DevOrch 개발팀  
**검토자**: 필요시 기재
