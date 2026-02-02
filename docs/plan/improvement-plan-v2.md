# DevOrch 개선 분석 및 구현 계획

**작성일**: 2026-01-29  
**기준**: OpenCode 프로덕트 분석 (productexplain.md)

---

## 1. 현재 상태 분석

### 1.1 OpenCode 핵심 특징 vs DevOrch 현재 상태

| 기능 | OpenCode | DevOrch 현재 | 개선 필요 |
|------|----------|-------------|-----------|
| CLI 실행 시 TUI | ✅ 멋진 대화형 TUI 자동 실행 | ❌ 도움말만 출력 | **High** |
| 서버 Web UI | ✅ 완성된 웹 인터페이스 | ❌ 404 Not Found | **Critical** |
| 하드웨어 감지 | ✅ 자동 감지 및 모델 추천 | ⚠️ 감지만 있음, 추천 미연동 | **High** |
| OAuth/API키 연동 | ✅ `/connect` 명령으로 간편 설정 | ⚠️ 부분 구현 | **Medium** |
| 모델 자동 설치 | ✅ 자동 설치/업데이트 | ❌ 수동 설치만 | **High** |
| 세션 관리 | ✅ 세션 공유/복원 | ⚠️ 기본 구현 | **Medium** |

### 1.2 발견된 문제점

1. **TUI 미연동**: `devorch` 실행 시 도움말만 출력, 대화형 인터페이스 없음
2. **데몬 Web UI 404**: `devorchd` 실행 후 `http://127.0.0.1:8787/` 접속 시 404
3. **모델 자동 추천 미연동**: HWProfile 감지 코드는 있으나 실제 모델 추천/설치 연동 안됨
4. **프로바이더 인증 미완성**: OAuth 플레이스홀더만 있음

---

## 2. 구현 계획

### Phase A: TUI 화면 완성 (Priority: Critical)

**목표**: `devorch` 실행 시 OpenCode처럼 멋진 대화형 TUI 표시

**구현 내용**:
```
┌─────────────────────────────────────────────────────────────────┐
│  DevOrch v1.0 - AI Coding Agent                                │
│  ───────────────────────────────────────────────────────────── │
│                                                                 │
│  🖥️  System: macOS (arm64) | 32GB RAM | Metal GPU             │
│  🤖 Model: ollama/llama3.2:3b (auto-selected)                  │
│  📁 Project: /Users/user/myproject                             │
│                                                                 │
│  ───────────────────────────────────────────────────────────── │
│                                                                 │
│  > How can I help you today?                                   │
│                                                                 │
│  [Messages will appear here...]                                │
│                                                                 │
│  ───────────────────────────────────────────────────────────── │
│  > Type your message...                                        │
│                                                                 │
│  [ctrl+c] quit  [ctrl+r] reset  [/help] commands              │
└─────────────────────────────────────────────────────────────────┘
```

**파일 수정**:
- `cmd/devorch/main.go`: 기본 실행 시 TUI 모드 활성화
- `internal/tui/app.go`: 완성된 대화형 인터페이스
- `internal/tui/model.go`: 상태 관리 개선
- `internal/tui/views.go`: 화면 렌더링 (새 파일)

---

### Phase B: 하드웨어 감지 및 모델 자동 추천/설치 (Priority: High)

**목표**: 사용자 시스템 성능 파악 후 최적 모델 자동 설치

**로직**:
```
1. HWProfile 감지 (CPU, RAM, GPU)
2. 시스템 티어 분류:
   - High: GPU + 32GB+ RAM → llama3.2:13b
   - Mid: 16GB+ RAM → llama3.2:7b  
   - Low: 8GB+ RAM → llama3.2:3b
   - Minimal: <8GB → phi3:mini
3. Ollama 설치 확인 → 미설치 시 자동 설치
4. 추천 모델 자동 pull
```

**파일 수정/생성**:
- `internal/autosetup/autosetup.go`: 자동 설정 로직
- `internal/autosetup/model_recommend.go`: 모델 추천 로직
- `internal/autosetup/ollama_install.go`: Ollama 자동 설치

---

### Phase C: 데몬 Web UI 수정 (Priority: Critical)

**문제**: `devorchd` 실행 후 루트 경로 접속 시 404

**원인**: 
- `cmd/devorchd/main.go`에서 webui 패키지 미사용
- static 파일 라우팅 누락

**해결**:
- devorchd에서 webui 서버 통합
- 또는 기본 HTML 페이지 제공

---

### Phase D: OAuth/API키 연동 완성 (Priority: Medium)

**구현 내용**:
- `devorch connect <provider>`: 대화형 인증 설정
- API 키 저장소 (keychain/keyring 연동)
- 프로바이더별 상태 표시

---

## 3. 구현 순서

```
1. [Critical] Phase C - 데몬 Web UI 404 수정
2. [Critical] Phase A - TUI 화면 완성
3. [High] Phase B - 하드웨어 감지 및 모델 자동 추천
4. [Medium] Phase D - OAuth/API키 연동
```

---

## 4. 상세 구현

### 4.1 devorchd Web UI 수정

**현재 문제**:
```go
// cmd/devorchd/main.go 현재 코드
mux.HandleFunc("/health", ...)      // ✅ 동작
mux.HandleFunc("/api/providers", ...) // ✅ 동작
// "/" 라우트 없음 → 404
```

**해결 방안**:
```go
// 기본 페이지 추가
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    // 대시보드 HTML 렌더링
})
```

### 4.2 자동 모델 추천 매핑

| 시스템 티어 | RAM | GPU | 추천 모델 | 대안 모델 |
|------------|-----|-----|----------|----------|
| Ultra | 64GB+ | CUDA/Metal | llama3.3:70b | deepseek-coder:33b |
| High | 32GB+ | CUDA/Metal | llama3.2:13b | codellama:13b |
| Mid | 16GB+ | Any | llama3.2:7b | qwen2.5-coder:7b |
| Low | 8GB+ | Any | llama3.2:3b | phi3:medium |
| Minimal | <8GB | None | phi3:mini | qwen2.5-coder:1.5b |

---

## 5. 예상 결과

### 5.1 `devorch` 실행 (TUI 모드)
```bash
$ devorch

╭──────────────────────────────────────────────────────────────╮
│                    🚀 DevOrch v1.0                           │
│            Local-First Multi-LLM Orchestrator                │
├──────────────────────────────────────────────────────────────┤
│  System: macOS arm64 | 32GB RAM | Apple Metal               │
│  Model:  ollama/llama3.2:7b (auto-selected)                 │
│  Status: ● Connected                                         │
╰──────────────────────────────────────────────────────────────╯

  How can I help you today?

  ────────────────────────────────────────────────────────────

  > _

  [Ctrl+C] Quit  [Ctrl+N] New Session  [/help] Commands
```

### 5.2 `devorchd` Web UI
```
http://127.0.0.1:8787/ → 대시보드 표시
- 시스템 상태
- 활성 세션
- 프로바이더 목록
- 통계
```

### 5.3 자동 모델 설치
```bash
$ devorch

🔍 Detecting hardware...
  ✓ macOS arm64
  ✓ 32GB RAM
  ✓ Apple Metal GPU

📦 Checking Ollama...
  ✓ Ollama installed (v0.5.7)

🤖 Selecting optimal model...
  → Recommended: llama3.2:7b (based on your hardware)

⬇️  Installing model... (this may take a few minutes)
  ████████████████████████░░░░░░ 78% llama3.2:7b

✅ Setup complete! Starting DevOrch...
```

---

## 6. 파일 변경 목록

### 신규 파일
- `internal/autosetup/autosetup.go`
- `internal/autosetup/model_recommend.go`
- `internal/autosetup/ollama_install.go`
- `internal/tui/views.go`
- `internal/tui/commands.go`
- `internal/webui/static/dashboard.html`

### 수정 파일
- `cmd/devorch/main.go`
- `cmd/devorchd/main.go`
- `internal/tui/app.go`
- `internal/tui/model.go`
- `internal/webui/server.go`

---

*다음 단계: 구현 시작*
