# DevOrch CLI vs TUI 명령어 및 기능 분석 보고서

## 🔍 CLI vs TUI 명령어 비교 분석

### 1. 명령어 구조 차이점

#### TUI 명령어 시스템
- **핸들러**: `GetSlashCommands()` - 89개 이상의 명령어
- **카테고리**: Quick, Main, Legacy, Session, Code, Context, Provider, Tools, Config, Edit
- **인터랙티브 지원**: 키보드 네비게이션(↑/↓/Enter), 자동완성, 명령어 팔레트

#### CLI 명령어 시스템  
- **핸들러**: 16개 핵심 그룹 + 레거시 명령어
- **그룹화**: session, model, provider, agent, code, context, tools, config, auth, system
- **인터랙티브 지원**: 숫자 선택, 텍스트 입력, help 시스템

### 2. 동일한 명령어들

✅ **완전히 동기화된 명령어들:**
```
/help, /exit, /quit, /clear, /cls, /new, /init, /history
/session, /model, /provider, /agent, /code, /context
/tools, /config, /auth, /system
/mode, /multimodel, /preset, /permissions
```

### 3. TUI 전용 기능들

#### 3.1 고급 인터랙션 기능
- **키보드 단축키**: 
  - `Ctrl+1/2/3/4`: 작업 모드 전환(Ask/Edit/Agent/Plan)
  - `Ctrl+N`: 새 세션
  - `Ctrl+S`: 세션 목록
  - `Ctrl+H`: 도움말
  - `Ctrl+P`: 명령어 팔레트

#### 3.2 고급 UI 컴포넌트
- **명령어 자동완성**: 실시간 필터링 및 선택
- **멀티모달 비교**: 여러 모델 동시 응답 비교
- **프리셋 관리**: 모델 설정 저장/로드
- **프로그레스 바**: 모델 설치/다운로드 진행률
- **스트리밍**: 실시간 응답 스트리밍

#### 3.3 시각적 선택 인터페이스
- **테마 선택**: 시각적 테마 미리보기
- **모델 선택**: 설치 상태 표시 (✓)
- **Provider 선택**: 연결 상태 표시 (○/●)
- **언어 선택**: 다국어 지원

### 4. CLI 전용 기능들

#### 4.1 권한 관리
```bash
/permissions allow <tool>    # 도구 항상 허용
/permissions ask <tool>      # 도구 사용시 확인
/permissions deny <tool>     # 도구 사용 거부
```

#### 4.2 대화형 설정
- **번호 기반 선택**: Provider/모델을 번호로 선택
- **필터링**: 카테고리별 provider 표시
- **실시간 상태**: 연결 상태 실시간 업데이트

#### 4.3 OAuth 플로우 지원
- **Device Code Flow**: GitHub, Anthropic OAuth
- **Browser Flow**: 자동 브라우저 열기
- **API Key Flow**: 대화형 API 키 입력

## 🚨 TUI의 주요 사용성 문제점

### 1. Provider 선택 UI 문제

#### 현재 구현의 문제점:
```go
// /Users/deniallee/devorch/internal/tui/model.go:904-976
case ViewModeProviderSelect:
    // Extract provider name
    providers := GetAvailableProviders()
    if m.selectionIdx < len(providers) {
        p := providers[m.selectionIdx]
        // Get first model for this provider
        models := GetModelsForProvider(p.Name)
        // ...
    }
```

**문제점:**
1. **키보드 네비게이션 불완전**: `handleSelectionKey`에서 `up/down` 키는 작동하지만 사용성이 떨어짐
2. **시각적 피드백 부족**: 현재 선택된 항목이 명확하지 않음  
3. **상태 표시 불완전**: 연결 상태가 명확하게 표시되지 않음
4. **필터링/검색 불가**: 많은 provider 중에서 검색 불가

### 2. Connect UI의 사용성 문제

#### 현재 Connect 모드:
```go
// viewConnect renders the OpenCode-style provider connect view
func (m Model) viewConnect() string {
    // If in API key input mode
    if m.connectInputMode {
        // API key 입력만 지원, OAuth flow 불완전
    }
}
```

**문제점:**
1. **OAuth 플로우 미완성**: CLI는 OAuth 지원하지만 TUI는 API key만
2. **검색 기능 부재**: 40+개 provider에서 검색 불가
3. **카테고리화 미흡**: Popular/Other 구분이 UI에서 불명확

### 3. 모델 설치 UI 문제

```go
case *ModelInstallProgressMsg:
    // Update progress indicator
    m.UpdateProgress(msg.Percent, msg.Bytes, msg.Total, msg.Speed, msg.ETA, msg.Status)
```

**문제점:**
1. **진행률 표시 불완전**: ETA, 속도 정보가 UI에서 제대로 표시되지 않음
2. **취소 기능 없음**: 설치 중 취소할 수 없음
3. **에러 처리 미흡**: 설치 실패 시 복구 옵션 없음

## 🔧 기능성 차이 분석

### 1. 인증 시스템

| 기능 | CLI | TUI | 차이점 |
|------|-----|-----|--------|
| OAuth 로그인 | ✅ 완전 지원 | ⚠️ 부분 지원 | TUI는 브라우저 열기만 가능 |
| API Key 입력 | ✅ 대화형 | ✅ 입력 필드 | CLI가 더 직관적 |
| 상태 확인 | ✅ 실시간 | ✅ 정적 | CLI가 더 정확 |

### 2. 모델 관리

| 기능 | CLI | TUI | 차이점 |
|------|-----|-----|--------|
| 모델 목록 | ✅ 텍스트 | ✅ 선택 UI | TUI가 더 사용자 친화적 |
| 모델 설치 | ✅ 진행률 | ✅ 프로그레스바 | TUI가 더 시각적 |
| 모델 전환 | ✅ 번호/이름 | ✅ 키보드 선택 | 각각 장단점 |

### 3. 세션 관리

| 기능 | CLI | TUI | 차이점 |
|------|-----|-----|--------|
| 세션 저장 | ✅ 기본 | ✅ 향상된 UI | TUI가 더 풍부 |
| 세션 로드 | ✅ 목록 | ✅ 인터랙티브 | TUI가 더 편리 |
| 히스토리 | ✅ 텍스트 | ✅ 스크롤 가능 | TUI가 더 사용하기 쉬움 |

## 📊 전체 코드 구조 분석

### 1. TUI 아키텍처 (`internal/tui/`)

```
model.go (3,259 lines)    - 핵심 상태 관리, 키 핸들링
command.go (2,663 lines)  - 89+ 명령어 핸들러
provider.go (627 lines)   - Provider/모델 관리  
cli_mode.go (3,366 lines) - CLI 모드 구현
theme.go (271+ lines)     - 테마/스타일 관리
```

**강점:**
- 풍부한 인터랙션 (키보드 네비게이션, 자동완성)
- 시각적 피드백 (프로그레스바, 선택 하이라이트)
- 모달 기반 UI (각 작업별 전용 화면)

**약점:**
- 복잡한 상태 관리 (ViewMode 15개)
- 키보드 네비게이션 일관성 부족
- OAuth 플로우 미완성

### 2. CLI 아키텍처

```
cli_mode.go - 3,366 lines 내에 모든 CLI 로직
```

**강점:**
- 간단하고 직관적인 명령어 구조
- 완전한 OAuth 플로우 지원
- 권한 관리 시스템

**약점:**
- 텍스트 기반으로 시각적 피드백 부족
- 동시 작업 지원 제한
- 고급 기능 (멀티모델 비교 등) 부재

## 🛠️ 개선 권장사항

### 1. TUI Provider 선택 개선
```go
// 추가해야 할 기능들
- 검색/필터링 (`/` 키로 검색 모드)
- 카테고리 구분 (Popular/Other 시각적 구분)
- 연결 상태 명확한 표시
- OAuth 플로우 완전 구현
```

### 2. 키보드 네비게이션 표준화
```go
// 모든 선택 UI에서 일관된 키 바인딩
- ↑/↓: 네비게이션
- Enter: 선택/실행
- Esc: 취소/뒤로
- /: 검색 모드
- Space: 다중 선택 토글 (해당하는 경우)
```

### 3. CLI 고급 기능 추가
```go
// TUI의 고급 기능을 CLI로 포팅
- 멀티모델 비교 지원
- 프리셋 관리 시스템
- 진행률 표시 개선
```

이 분석을 통해 TUI와 CLI 각각의 장단점이 명확해졌으며, 특히 TUI의 Provider 선택 UI 사용성 문제가 핵심 개선 포인트임을 확인했습니다.