# Phase 45-48: TUI (Terminal User Interface)

## 개요
Bubbletea 기반의 터미널 사용자 인터페이스를 구현했습니다. 인터랙티브한 채팅 경험과 세션 관리를 터미널에서 직접 제공합니다.

## 구현 파일

### internal/tui/model.go
- **Model**: 메인 TUI 모델 (bubbletea.Model 구현)
- **ViewMode**: Chat, Sessions, Settings, Help 모드
- **Message**: 채팅 메시지 (role, content, tokens)
- **SessionInfo**: 세션 메타데이터
- 키보드 단축키 처리 (Ctrl+N, Ctrl+S, Ctrl+H, Ctrl+C)
- 슬래시 명령어 (/help, /new, /sessions, /clear, /model, /quit)

### internal/tui/theme.go
- **Theme**: 테마 스타일 정의
- **DefaultTheme()**: 기본 다크 테마
- **LightTheme()**: 라이트 테마
- **HighContrastTheme()**: 고대비 테마 (접근성)
- **WithColors()**: 커스텀 컬러 테마

### internal/tui/components.go
- **Component**: 재사용 가능한 UI 컴포넌트 인터페이스
- **Box**: 스타일된 박스
- **Divider**: 수평 구분선
- **Table**: 테이블 렌더링
- **ProgressBar**: 진행률 표시
- **Badge**: 상태 뱃지
- **List**: 불릿 리스트
- **Tabs**: 탭 헤더
- **Tree**: 트리 뷰
- **WrapText()**: 텍스트 래핑

### internal/tui/markdown.go
- **Markdown**: 마크다운 렌더링
- 헤더, 코드 블록, 인용문, 리스트 지원
- 인라인 스타일 (볼드, 이탈릭, 코드, 링크)
- **RenderToolResult()**: 도구 실행 결과 포맷팅
- **RenderThinking()**: 사고 중 표시
- **RenderError()**: 에러 메시지
- **RenderUsage()**: 토큰 사용량

### internal/tui/app.go
- **App**: TUI 애플리케이션 래퍼
- **Option**: 설정 옵션 함수
- **Run()**: 애플리케이션 실행
- **RunOnce()**: 일회성 실행
- **Send()**: 메시지 전송
- **Quit()**: 종료
- CLI용 간편 출력 함수들

## 의존성
- github.com/charmbracelet/bubbletea v1.3.10
- github.com/charmbracelet/lipgloss v1.1.0
- github.com/charmbracelet/bubbles v0.21.0

## 사용 예시
```go
import "devorch/internal/tui"

// 기본 실행
app := tui.NewApp(
    tui.WithTheme(tui.DefaultTheme()),
    tui.WithSessionID("session-1"),
)
if err := app.Run(); err != nil {
    log.Fatal(err)
}

// 테마 변경
app := tui.NewApp(
    tui.WithTheme(tui.LightTheme()),
)
```

## 키보드 단축키
| 키 | 기능 |
|---|---|
| Ctrl+N | 새 세션 |
| Ctrl+S | 세션 목록 토글 |
| Ctrl+H | 도움말 토글 |
| Ctrl+C/Q | 종료 |
| Enter | 메시지 전송/선택 |
| Esc | 채팅 뷰로 돌아가기 |
| ↑/↓ | 리스트 탐색 |

## 슬래시 명령어
| 명령어 | 설명 |
|---|---|
| /help | 도움말 표시 |
| /new | 새 세션 시작 |
| /sessions | 세션 목록 표시 |
| /clear | 현재 채팅 지우기 |
| /model | 현재 모델 표시 |
| /quit | 종료 |
