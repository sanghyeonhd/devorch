# DevOrch 빌드 정보

## 버전
v0.1.0

## 빌드 날짜
2026-01-31

## 지원 플랫폼
모든 주요 운영체제를 지원합니다:

### macOS
- `devorch-darwin-amd64` - Intel Mac (13 MB)
- `devorch-darwin-arm64` - Apple Silicon (M1/M2/M3) (12 MB)

### Linux
- `devorch-linux-amd64` - Linux x64 (13 MB)
- `devorch-linux-arm64` - Linux ARM64 (Raspberry Pi 등) (12 MB)

### Windows
- `devorch-windows-amd64.exe` - Windows x64 (13 MB)
- `devorch-windows-arm64.exe` - Windows ARM64 (12 MB)

## 설치 방법

### macOS/Linux
```bash
# 실행 권한 부여
chmod +x devorch-darwin-arm64  # macOS M1/M2/M3
# 또는
chmod +x devorch-darwin-amd64  # macOS Intel
# 또는
chmod +x devorch-linux-amd64   # Linux

# 실행
./devorch-darwin-arm64
```

### Windows
PowerShell에서:
```powershell
.\devorch-windows-amd64.exe
```

## 사용 방법

### CLI 모드 (기본)
```bash
./devorch
```
- 직접 터미널에서 대화형 명령어 입력
- `/help` 명령어로 사용 가능한 명령어 확인
- 16개 핵심 명령어 그룹 지원

### TUI 모드 (Full Screen)
```bash
./devorch tui
```
- Bubbletea 기반 풀스크린 인터페이스
- 마우스 지원
- 시각적 UI

## 주요 명령어 그룹

1. `/session` - 세션 관리 (new, list, save, export...)
2. `/model` - 모델 관리 (list, set, install, bench)
3. `/provider` - 프로바이더 관리 (list, connect, disconnect)
4. `/agent` - 에이전트 관리 (list, set)
5. `/code` - 코드/파일 작업 (files, grep, diff, review)
6. `/context` - 컨텍스트 관리 (add, memory, thinking)
7. `/tools` - 도구 관리 (mcp, lsp)
8. `/config` - 설정 (theme, lang, edit)
9. `/auth` - 인증 (login, logout, status)
10. `/system` - 시스템 (status, setup, doctor, version)

## 빌드 방법

프로젝트를 직접 빌드하려면:

```bash
# 현재 플랫폼만 빌드
go build -o bin/devorch ./cmd/devorch

# 모든 플랫폼 빌드
./build.sh
```

## 시스템 요구사항

- 메모리: 최소 512MB (권장 1GB+)
- 디스크: 100MB 이상
- Ollama: 로컬 모델 사용 시 필요

## 지원 모델

### Ollama (로컬)
- tinyllama, phi3, llama3.2, qwen2.5-coder 등
- 모든 Ollama 호환 모델 지원

### 외부 API
- OpenAI (GPT-4, GPT-3.5)
- Anthropic (Claude)
- Google (Gemini)
- GitHub Copilot
- Groq
- OpenRouter

## 문의 및 지원

- GitHub: https://github.com/sanghyeonhd/devorch
- Issues: https://github.com/sanghyeonhd/devorch/issues

## 라이선스

MIT License
