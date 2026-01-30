# Phase 54: Interactive Selection UI for All Commands

## Overview
모든 슬래시 명령어를 텍스트 입력 대신 키보드 화살표 선택 기반 UI로 전환하여 사용성을 극대화했습니다.

## Changes Made

### 1. `/install` 명령어 완전 재설계

#### 기존 문제점
- 하드코딩된 3개 모델만 자동 설치
- 사용자가 모델을 선택할 수 없음
- 전체 모델 목록을 볼 수 없음
- 시스템 사양 표시만 하고 활용 안 함

#### 새로운 기능
```
🖥️  시스템 사양 자동 감지 및 표시
- RAM, CPU 코어 수 자동 감지
- 사양 등급 자동 판정 (Low/Medium/High/Very High)

📦 전체 모델 카탈로그 제공
⭐ Recommended (시스템 사양 기반 자동 추천)
  [◉] phi3:mini (1.3GB) - Best for low-end (Microsoft)
  [◉] qwen2.5:3b (2GB) - Fast general purpose
  [ ] llama3.2:1b (1.3GB) - Tiny but capable (Meta)

💻 Coding (코딩 특화 모델)
  [◉] qwen2.5-coder:3b (2GB) - Qwen2.5 Coder 3B
  [ ] codellama:7b (3.8GB) - CodeLlama 7B
  [ ] codellama:13b (7.3GB) - CodeLlama 13B
  [ ] deepseek-coder:6.7b (3.8GB) - DeepSeek Coder

🌐 General (범용 모델)
  [ ] llama3.2:1b (1.3GB) - Meta Llama 3.2 1B
  [ ] llama3.2:3b (2GB) - Meta Llama 3.2 3B
  [ ] llama3.1:8b (4.7GB) - Meta Llama 3.1 8B
  [ ] mistral:7b (4.1GB) - Mistral 7B
  ...

🔰 Small (저사양용)
  [ ] phi3:mini (1.3GB) - Microsoft Phi-3 Mini
  [ ] deepseek-r1:1.5b (1GB) - DeepSeek R1 1.5B
  [ ] tinyllama:1.1b (637MB) - TinyLlama 1.1B

🚀 Large (고성능 시스템용, 16GB+ RAM만 표시)
  [ ] mixtral:8x7b (26GB) - Mixtral 8x7B MoE
  [ ] qwen2.5:14b (9GB) - Qwen2.5 14B
  [ ] llama3.1:70b (40GB) - Meta Llama 3.1 70B
```

#### 인터랙티브 조작
```
Space   : 개별 모델 선택/해제
A       : 추천 모델 전체 선택
N       : 선택 전체 해제
↑/↓ (k/j) : 모델 간 이동
Enter   : 선택한 모델 설치 시작
Esc/Q   : 취소하고 채팅으로 돌아가기
```

#### 스마트 추천 로직
```go
// RAM 기반 추천
< 8GB   → 1-3B 모델 (phi3:mini, qwen2.5:3b, llama3.2:1b)
8-16GB  → 3-7B 모델 (qwen2.5:3b, mistral:7b, qwen2.5-coder:3b)
16-32GB → 7-13B 모델 (mistral:7b, codellama:7b, qwen2.5:7b)
> 32GB  → 13B+ 모델 (llama3.1:8b, mixtral:8x7b, qwen2.5:14b)
```

#### 실시간 정보 표시
- 현재 선택된 모델 수
- 선택된 모델의 총 다운로드 크기
- 이미 설치된 모델 표시 ([✓] 아이콘)
- 카테고리별 그룹화
- 스크롤 가능 (15개씩 표시)

### 2. `/language` 명령어 선택 UI 추가

#### 개선
```
📋 Select Language

  ▶ English (en)
    한국어 (ko)
    日本語 (ja)
    中文 (zh)
    Español (es)
    Français (fr)
    Deutsch (de)

Enter: Select | Esc: Cancel | ↑/↓: Navigate
```
화살표로 선택하고 Enter로 즉시 적용

### 3. `/themes` 명령어 통합

기존의 `/themes` (목록 표시) → `/theme` (선택) 이중 구조를
`/themes` 실행 시 바로 선택 UI로 통합

### 4. 새로운 ViewMode 추가

```go
const (
    ...
    ViewModeInstallSelect  // Model installation selection
    ViewModeLanguageSelect // Language selection
)
```

## Benefits

### 사용자 경험 향상
1. **직관적 조작**: 모든 명령어가 화살표 + Enter 패턴
2. **시각적 피드백**: 선택 상태, 설치 여부 즉시 확인
3. **스마트 추천**: 시스템에 최적화된 모델 자동 선택
4. **유연한 선택**: Space로 개별 선택, A로 일괄 선택, N으로 전체 해제

### 일관성
- 모든 선택 기반 명령어가 동일한 UI 패턴
- `/theme`, `/model`, `/provider`, `/language`, `/install` 모두 통일

### 정보 투명성
- 시스템 사양 명시
- 모델 크기 및 설명 제공
- 현재 설치 상태 표시
- 다운로드 크기 미리 계산

## Testing

```bash
# 빌드
cd /Users/deniallee/devorch
go build -o bin/devorch ./cmd/devorch

# 실행 및 테스트
./bin/devorch

# 테스트할 명령어
/install      # 모델 선택 UI 확인
/language     # 언어 선택 UI 확인
/themes       # 테마 선택 UI 확인
```

## Summary

모든 주요 명령어가 이제 **키보드 화살표 기반 선택 UI**를 지원합니다:

| 명령어 | 이전 | 현재 |
|--------|------|------|
| `/install` | 하드코딩 3개 자동 설치 | ✅ 전체 모델 선택 UI |
| `/language` | 텍스트 목록만 표시 | ✅ 선택 UI |
| `/themes` | 텍스트 목록 표시 | ✅ 선택 UI로 통합 |
| `/theme` | ✅ 선택 UI | ✅ 선택 UI |
| `/model` | ✅ 선택 UI | ✅ 선택 UI |
| `/provider` | ✅ 선택 UI | ✅ 선택 UI |
| `/agent` | ✅ 선택 UI | ✅ 선택 UI |

**DevOrch는 이제 OpenCode 스타일의 완전한 인터랙티브 TUI를 제공합니다!** 🎉
