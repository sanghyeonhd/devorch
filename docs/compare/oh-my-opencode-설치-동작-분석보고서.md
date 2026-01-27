# OpenCode + OhMyOpenCode(oh-my-opencode) 설치/동작 분석 보고서 (macOS)

작성일: 2026-01-28  
업데이트: 2026-01-28 (v3.x 기반 전면 업그레이드)

이 문서는 `oh-my-opencode` 소스코드와 OpenCode 공식 문서를 근거로, "opencode + oh-my-opencode가 내부적으로 어떻게 동작하는지"를 가능한 한 상세히 정리합니다.

---

## 목차

1. [결론 요약](#1-결론-요약-핵심만)
2. [프로젝트 전체 구조](#2-프로젝트-전체-구조)
3. [실제 설치/데이터 경로](#3-실제-설치데이터-경로)
4. [OpenCode 설정 시스템](#4-opencode-설정-시스템)
5. [플러그인 로딩 메커니즘](#5-플러그인-로딩-메커니즘)
6. [oh-my-opencode 핵심 모듈별 분석](#6-oh-my-opencode-핵심-모듈별-분석)
7. [오케스트레이션 시스템](#7-오케스트레이션-시스템)
8. [다중 모델 선택 시스템](#8-다중-모델-선택-시스템)
9. [Hook 시스템 심층 분석](#9-hook-시스템-심층-분석)
10. [Tool 시스템 심층 분석](#10-tool-시스템-심층-분석)
11. [Feature 시스템 분석](#11-feature-시스템-분석)
12. [CLI 시스템](#12-cli-시스템)
13. [MCP 시스템](#13-mcp-시스템)
14. [점검/트러블슈팅](#14-점검트러블슈팅)
15. [참고 자료](#15-참고-자료)

---

## 1) 결론 요약 (핵심만)

### 1.1 아키텍처 개요

```
┌─────────────────────────────────────────────────────────────────────┐
│                           OpenCode                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │   TUI/웹    │◄─►│  HTTP 서버  │◄─►│     LLM Provider API    │  │
│  │  클라이언트   │  │  (SSE 이벤트) │  │  (Anthropic/OpenAI/...)  │  │
│  └──────────────┘  └──────┬───────┘  └──────────────────────────┘  │
│                           │                                          │
│                    ┌──────▼───────┐                                  │
│                    │   플러그인    │◄─── oh-my-opencode              │
│                    │   시스템      │                                  │
│                    └──────────────┘                                  │
└─────────────────────────────────────────────────────────────────────┘
```

- **OpenCode 실행 파일**: `/Users/deniallee/.opencode/bin/opencode` (버전 `1.1.36`)
- **TUI + 서버 구조**: `opencode` 실행 시 서버가 뜨고 TUI가 그 서버에 연결
- **플러그인 자동 설치**: `opencode.json`의 `plugin` 배열에 지정된 npm 패키지를 Bun으로 설치하여 `~/.cache/opencode/node_modules/`에 캐시

### 1.2 oh-my-opencode 핵심 구성요소

| 구성요소 | 파일 수 | 총 코드 라인 | 설명 |
|---------|--------|------------|------|
| **Agents** | 15개 | ~3,500줄 | Sisyphus, Oracle, Librarian, Explore, Prometheus, Metis, Momus, Atlas 등 |
| **Hooks** | 31개 | ~5,000줄 | PreToolUse, PostToolUse, UserPromptSubmit, Stop 이벤트 처리 |
| **Tools** | 20개+ | ~3,500줄 | LSP, AST-Grep, delegate_task, background_task 등 |
| **Features** | 15개+ | ~4,000줄 | Background Agent, Skill MCP, Context Injector 등 |
| **MCPs** | 3개 | ~200줄 | Exa(웹검색), Context7(문서), Grep.app(코드검색) |
| **CLI** | 4개 명령 | ~1,500줄 | install, doctor, run, get-local-version |
| **Shared** | 50개+ | ~2,500줄 | 유틸리티, 모델 리졸버, 설정 파서 등 |

### 1.3 다중 모델 오케스트레이션 요약

| 레이어 | 방식 | 예시 |
|-------|------|------|
| **에이전트별 모델** | 역할에 따른 전문화 | oracle=GPT-5.2, explore=Grok Code |
| **카테고리별 모델** | 작업 성격에 따른 프리셋 | ultrabrain=GPT-5.2-codex, visual=Gemini 3 Pro |
| **3단계 Fallback** | 가용성 기반 자동 선택 | 사용자 오버라이드 → 프로바이더 우선순위 → 시스템 기본값 |
| **delegate_task 라우팅** | 동적 에이전트 호출 | `category="quick"` 또는 `subagent_type="oracle"` |

---

## 2) 프로젝트 전체 구조

```
oh-my-opencode/
├── src/                           # 소스 코드 (총 489 파일)
│   ├── agents/                    # 🤖 10+ AI 에이전트 정의
│   │   ├── sisyphus.ts           # 메인 오케스트레이터 (615줄)
│   │   ├── atlas.ts              # 플랜 실행 오케스트레이터 (543줄)
│   │   ├── oracle.ts             # 전략적 조언 에이전트 (GPT-5.2)
│   │   ├── librarian.ts          # 문서/코드 검색 에이전트
│   │   ├── explore.ts            # 빠른 코드베이스 탐색 에이전트
│   │   ├── prometheus-prompt.ts  # 플래너 에이전트 (1,196줄)
│   │   ├── metis.ts              # 플랜 컨설턴트
│   │   ├── momus.ts              # 플랜 리뷰어
│   │   └── sisyphus-junior.ts    # 카테고리 기반 위임 실행자
│   │
│   ├── hooks/                     # 🪝 31개 라이프사이클 훅
│   │   ├── atlas/                 # 메인 오케스트레이션 (773줄)
│   │   ├── ralph-loop/           # 자기참조 개발 루프
│   │   ├── claude-code-hooks/    # Claude Code 호환 레이어
│   │   ├── comment-checker/      # AI 슬롭 방지
│   │   ├── rules-injector/       # 조건부 규칙 주입
│   │   ├── directory-agents-injector/ # AGENTS.md 자동 주입
│   │   ├── directory-readme-injector/ # README.md 자동 주입
│   │   └── todo-continuation-enforcer.ts # TODO 완료 강제
│   │
│   ├── tools/                     # 🔧 20+ 도구
│   │   ├── delegate-task/        # 카테고리 기반 라우팅 (1,039줄)
│   │   ├── lsp/                  # 6개 LSP 도구
│   │   ├── ast-grep/             # 2개 AST 검색/치환 도구
│   │   ├── session-manager/      # 4개 세션 관리 도구
│   │   ├── background-task/      # 백그라운드 작업 관리
│   │   └── skill/                # 스킬 실행 도구
│   │
│   ├── features/                  # ⚙️ 핵심 기능 모듈
│   │   ├── background-agent/     # 작업 라이프사이클 (1,335줄)
│   │   ├── builtin-skills/       # 내장 스킬 (1,203줄)
│   │   ├── skill-mcp-manager/    # MCP 클라이언트 관리
│   │   ├── context-injector/     # AGENTS.md/README.md 주입
│   │   ├── claude-code-*-loader/ # Claude Code 호환 로더 (5개)
│   │   └── boulder-state/        # TODO 상태 영속화
│   │
│   ├── cli/                       # 💻 CLI 인터페이스
│   │   ├── install.ts            # 대화형 설치 (520줄)
│   │   ├── config-manager.ts     # JSONC 설정 파싱 (641줄)
│   │   └── doctor/               # 14개 진단 검사
│   │
│   ├── mcp/                       # 🌐 내장 MCP 서버
│   │   ├── websearch.ts          # Exa AI 웹 검색
│   │   ├── context7.ts           # 라이브러리 문서
│   │   └── grep-app.ts           # GitHub 코드 검색
│   │
│   ├── shared/                    # 🔄 50+ 공유 유틸리티
│   │   ├── model-resolver.ts     # 3단계 모델 해석
│   │   ├── model-requirements.ts # 에이전트/카테고리 요구사항
│   │   ├── dynamic-truncator.ts  # 토큰 인식 잘라내기
│   │   └── jsonc-parser.ts       # JSONC 파싱
│   │
│   ├── config/                    # 📋 설정 스키마
│   │   └── schema.ts             # Zod 기반 검증
│   │
│   ├── plugin-handlers/          # 🔌 플러그인 핸들러
│   │   └── config-handler.ts     # OpenCode config 합성
│   │
│   └── index.ts                   # 메인 엔트리포인트 (593줄)
│
├── packages/                      # 📦 7개 플랫폼별 바이너리
│   ├── darwin-arm64/
│   ├── darwin-x64/
│   ├── linux-arm64/
│   ├── linux-arm64-musl/
│   ├── linux-x64/
│   ├── linux-x64-musl/
│   └── windows-x64/
│
├── docs/                          # 📚 문서
│   ├── guide/                    # 사용자 가이드
│   ├── features.md               # 기능 상세
│   ├── configurations.md         # 설정 가이드
│   └── orchestration-guide.md    # 오케스트레이션 가이드
│
├── script/                        # 🛠️ 빌드/배포 스크립트
│   ├── build-binaries.ts
│   ├── build-schema.ts
│   └── publish.ts
│
├── assets/                        # JSON 스키마
│   └── oh-my-opencode.schema.json
│
└── bin/                           # CLI 진입점
    └── oh-my-opencode.js
```

### 2.1 복잡도 핫스팟 (주요 대형 파일)

| 파일 | 라인 수 | 설명 |
|------|--------|------|
| `src/features/background-agent/manager.ts` | 1,335 | 작업 라이프사이클, 동시성 관리 |
| `src/features/builtin-skills/skills.ts` | 1,203 | 스킬 정의 |
| `src/agents/prometheus-prompt.ts` | 1,196 | 플래너 에이전트 프롬프트 |
| `src/tools/delegate-task/tools.ts` | 1,039 | 카테고리 기반 위임 |
| `src/hooks/atlas/index.ts` | 773 | 오케스트레이터 훅 |
| `src/cli/config-manager.ts` | 641 | JSONC 설정 파싱 |
| `src/agents/sisyphus.ts` | 615 | 메인 에이전트 프롬프트 |

---

## 3) 실제 설치/데이터 경로

`opencode debug paths --print-logs` 결과:

| 경로 유형 | 위치 | 용도 |
|----------|------|------|
| home | `/Users/deniallee` | 사용자 홈 |
| config | `/Users/deniallee/.config/opencode` | 설정 파일 |
| cache | `/Users/deniallee/.cache/opencode` | 플러그인 캐시 |
| data | `/Users/deniallee/.local/share/opencode` | 세션 데이터 |
| log | `/Users/deniallee/.local/share/opencode/log` | 로그 파일 |
| state | `/Users/deniallee/.local/state/opencode` | 상태 파일 |

### 3.1 oh-my-opencode 설정 파일 위치

| 우선순위 | 위치 | 용도 |
|---------|------|------|
| 1 (최고) | `.opencode/oh-my-opencode.json(c)` | 프로젝트별 설정 |
| 2 | `~/.config/opencode/oh-my-opencode.json(c)` | 사용자 전역 설정 |

> **참고**: `.jsonc` 파일이 `.json` 파일보다 우선합니다.

---

## 4) OpenCode 설정 시스템

### 4.1 설정 로딩 우선순위 (머지 방식)

```
Remote config (.well-known/opencode)
        ↓ merge
Global config (~/.config/opencode/opencode.json)
        ↓ merge
Custom config (OPENCODE_CONFIG env)
        ↓ merge
Project config (opencode.json in project)
        ↓ merge
.opencode directories
        ↓ merge
Inline config (OPENCODE_CONFIG_CONTENT env)
        = 최종 설정
```

### 4.2 주요 환경 변수

| 변수 | 용도 |
|-----|------|
| `OPENCODE_CONFIG` | 특정 설정 파일 경로 지정 |
| `OPENCODE_CONFIG_CONTENT` | JSON 문자열로 설정 인라인 주입 |
| `OPENCODE_CONFIG_DIR` | 추가 설정 디렉터리 지정 |
| `XDG_CONFIG_HOME` | config 루트 변경 |
| `XDG_DATA_HOME` | data 루트 변경 |

### 4.3 JSONC 지원

oh-my-opencode는 JSONC(JSON with Comments)를 완벽 지원합니다:
- 라인 주석: `// comment`
- 블록 주석: `/* comment */`
- 후행 쉼표: `{ "key": "value", }`

```jsonc
{
  "$schema": "https://raw.githubusercontent.com/code-yeongyu/oh-my-opencode/master/assets/oh-my-opencode.schema.json",

  /* 에이전트 오버라이드 - 특정 작업에 맞는 모델 커스터마이즈 */
  "agents": {
    "oracle": {
      "model": "openai/gpt-5.2"  // GPT로 전략적 추론
    },
  },
}
```

---

## 5) 플러그인 로딩 메커니즘

### 5.1 플러그인 소스

OpenCode는 두 가지 소스에서 플러그인을 로드합니다:

| 소스 | 위치 | 설명 |
|-----|------|------|
| 로컬 파일 | `.opencode/plugins/`, `~/.config/opencode/plugins/` | 직접 작성한 플러그인 |
| npm 패키지 | `opencode.json`의 `plugin` 배열 | OpenCode가 Bun으로 자동 설치 |

### 5.2 설치 캐시 위치

```
~/.cache/opencode/
└── node_modules/
    ├── oh-my-opencode/              # 메인 플러그인
    └── oh-my-opencode-darwin-x64/   # 플랫폼별 바이너리
```

### 5.3 플러그인 API 인터페이스

```typescript
type Plugin = (input: PluginInput) => Promise<Hooks>

interface Hooks {
  config?(config): Promise<Config>               // 설정 수정
  event?({ event }): Promise<void>              // 이벤트 수신
  "chat.message"?(message): Promise<Message>     // 메시지 변환
  "tool.execute.before"?(input): Promise<Input>  // 도구 실행 전
  "tool.execute.after"?(output): Promise<Output> // 도구 실행 후
  "permission.ask"?(): Promise<boolean>          // 권한 확인
  // ... 추가 훅들
}
```

---

## 6) oh-my-opencode 핵심 모듈별 분석

### 6.1 src/agents/ — AI 에이전트 정의

| 에이전트 | 모델 | Temperature | 용도 |
|---------|------|-------------|------|
| **Sisyphus** | `anthropic/claude-opus-4-5` | 0.1 | 메인 오케스트레이터, 32k 확장 사고 |
| **Atlas** | `anthropic/claude-opus-4-5` | 0.1 | 플랜 실행 오케스트레이터 |
| **oracle** | `openai/gpt-5.2` | 0.1 | 아키텍처 결정, 코드 리뷰, 디버깅 |
| **librarian** | `opencode/big-pickle` | 0.1 | 문서 조회, GitHub 검색 |
| **explore** | `opencode/gpt-5-nano` | 0.1 | 빠른 컨텍스트 grep |
| **multimodal-looker** | `google/gemini-3-flash` | 0.1 | PDF/이미지 분석 |
| **Prometheus** | `anthropic/claude-opus-4-5` | 0.1 | 전략적 플래닝 |
| **Metis** | `anthropic/claude-sonnet-4-5` | 0.3 | 사전 플래닝 분석 |
| **Momus** | `anthropic/claude-sonnet-4-5` | 0.1 | 플랜 검증 |
| **Sisyphus-Junior** | `anthropic/claude-sonnet-4-5` | 0.1 | 카테고리 기반 실행자 |

#### 에이전트별 도구 제한

| 에이전트 | 제한 방식 | 제한된 도구 |
|---------|----------|------------|
| oracle | Deny list | write, edit, task, delegate_task |
| librarian | Deny list | write, edit, task, delegate_task, call_omo_agent |
| explore | Deny list | write, edit, task, delegate_task, call_omo_agent |
| multimodal-looker | Allow list | read만 허용 |
| Sisyphus-Junior | Deny list | task, delegate_task |

### 6.2 src/hooks/ — 라이프사이클 훅

31개의 훅이 이벤트별로 실행됩니다:

#### 훅 이벤트 유형

| 이벤트 | 타이밍 | 차단 가능 | 용도 |
|--------|--------|----------|------|
| PreToolUse | 도구 실행 전 | ✅ | 입력 검증/수정 |
| PostToolUse | 도구 실행 후 | ❌ | 경고 추가, 출력 잘라내기 |
| UserPromptSubmit | 프롬프트 제출 시 | ✅ | 키워드 감지 |
| Stop | 세션 유휴 시 | ❌ | 자동 계속 |
| onSummarize | 압축 시 | ❌ | 상태 보존 |

#### 실행 순서

**chat.message 체인:**
```
keywordDetector → claudeCodeHooks → autoSlashCommand → startWork → ralphLoop
```

**tool.execute.before 체인:**
```
claudeCodeHooks → nonInteractiveEnv → commentChecker → directoryAgentsInjector → rulesInjector
```

**tool.execute.after 체인:**
```
editErrorRecovery → delegateTaskRetry → commentChecker → toolOutputTruncator → claudeCodeHooks
```

### 6.3 src/tools/ — 도구 정의

| 카테고리 | 도구 | 용도 |
|---------|------|------|
| **LSP** | lsp_goto_definition, lsp_find_references, lsp_symbols, lsp_diagnostics, lsp_prepare_rename, lsp_rename | 시맨틱 코드 인텔리전스 |
| **검색** | ast_grep_search, ast_grep_replace, grep, glob | 패턴 발견 |
| **세션** | session_list, session_read, session_search, session_info | 히스토리 탐색 |
| **에이전트** | delegate_task, call_omo_agent, background_output, background_cancel | 작업 오케스트레이션 |
| **시스템** | interactive_bash, look_at | CLI, 멀티모달 |
| **스킬** | skill, skill_mcp, slashcommand | 스킬 실행 |

### 6.4 src/features/ — 핵심 기능 모듈

| 기능 | 파일 | 용도 |
|-----|------|------|
| **background-agent** | manager.ts (1,335줄) | launch → poll → complete 라이프사이클 |
| **skill-mcp-manager** | manager.ts | MCP 클라이언트 지연 로딩 |
| **builtin-skills** | skills.ts (1,203줄) | playwright, git-master, frontend-ui-ux |
| **builtin-commands** | commands.ts | ralph-loop, refactor, init-deep, start-work |
| **claude-code-*-loader** | 5개 로더 | .mcp.json, agents/*.md, commands/*.md 호환 |
| **context-injector** | injector.ts | AGENTS.md/README.md 자동 주입 |
| **boulder-state** | storage.ts | TODO 상태 영속화 |

---

## 7) 오케스트레이션 시스템

### 7.1 핵심 철학: "계획과 실행의 분리"

```
┌─────────────────────────────────────────────────────────────────┐
│                      계획 단계                                   │
│  ┌────────────┐     ┌─────────────┐     ┌────────────────────┐ │
│  │ Prometheus │ ◄─► │    Metis    │     │ .sisyphus/plans/   │ │
│  │  (플래너)   │     │ (컨설턴트)  │     │     {name}.md      │ │
│  └────────────┘     └─────────────┘     └────────────────────┘ │
│         │                                         │             │
│         └─────────────────► Momus ◄───────────────┘             │
│                            (리뷰어)                              │
└─────────────────────────────────────────────────────────────────┘
                               │
                               ▼
                        /start-work
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────┐
│                      실행 단계                                   │
│  ┌────────────┐                                                 │
│  │   Atlas    │ ──► Oracle / Frontend / Explore / Librarian    │
│  │(오케스트레이터)│                                               │
│  └────────────┘                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 7.2 두 가지 작업 모드

#### 모드 1: Ultrawork (빠른 작업)

프롬프트에 `ultrawork` 또는 `ulw`를 포함하기만 하면 됩니다:

```
ulw add authentication to my Next.js app
```

에이전트가 자동으로:
1. 코드베이스를 탐색하여 기존 패턴 파악
2. 전문 에이전트를 통해 모범 사례 연구
3. 규칙을 따라 기능 구현
4. 진단 및 테스트로 검증
5. 완료될 때까지 계속 작업

#### 모드 2: Prometheus (정밀 작업)

복잡하거나 중요한 작업의 경우:

1. **Tab** 키를 눌러 Prometheus(플래너) 모드 진입
2. Prometheus가 인터뷰를 통해 요구사항 수집
3. `.sisyphus/plans/*.md`에 상세 계획 생성
4. `/start-work` 명령으로 Atlas가 실행

```
1. Tab 키 → Prometheus 모드 진입
2. 작업 설명 → Prometheus 인터뷰
3. 계획 확인 → .sisyphus/plans/*.md 검토
4. /start-work 실행 → 오케스트레이터 실행
```

### 7.3 관련 명령어

| 명령어 | 용도 |
|-------|------|
| `@plan [요청]` | Prometheus 호출하여 계획 세션 시작 |
| `/start-work` | 생성된 계획 실행 |
| `/ralph-loop` | 완료될 때까지 자기참조 개발 루프 |
| `/ulw-loop` | ultrawork 모드로 ralph-loop 실행 |
| `/cancel-ralph` | 활성 Ralph Loop 취소 |
| `/init-deep` | 계층적 AGENTS.md 지식 베이스 초기화 |
| `/refactor` | LSP, AST-grep, TDD 검증으로 지능적 리팩토링 |

---

## 8) 다중 모델 선택 시스템

### 8.1 3단계 모델 해석 흐름

```
┌─────────────────────────────────────────────────────────────────┐
│                     모델 해석 흐름                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   1단계: 사용자 오버라이드                                        │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │ oh-my-opencode.json에 모델 지정됨?                       │   │
│   │         YES → 지정된 모델 그대로 사용                    │   │
│   │         NO  → 2단계로 계속                              │   │
│   └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│                              ▼                                  │
│   2단계: 프로바이더 우선순위 Fallback                            │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │ requirement.providers 순서대로 각 프로바이더 시도:       │   │
│   │                                                         │   │
│   │ 예: Sisyphus의 경우                                     │   │
│   │ anthropic → github-copilot → opencode → antigravity     │   │
│   │     │            │              │            │          │   │
│   │     ▼            ▼              ▼            ▼          │   │
│   │ 시도: anthropic/claude-opus-4-5                         │   │
│   │ 시도: github-copilot/claude-opus-4-5                    │   │
│   │ 시도: opencode/claude-opus-4-5                          │   │
│   │ ...                                                     │   │
│   │                                                         │   │
│   │ 사용 가능한 모델 찾음? → 해당 모델 반환                  │   │
│   │ 못 찾음? → 다음 프로바이더 시도                          │   │
│   └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│                              ▼ (모든 프로바이더 소진)            │
│   3단계: 시스템 기본값                                           │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │ systemDefaultModel 반환 (opencode.json에서)              │   │
│   └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 8.2 에이전트별 프로바이더 체인

| 에이전트 | 모델 (접두사 없음) | 프로바이더 우선순위 |
|---------|-------------------|-------------------|
| **Sisyphus** | `claude-opus-4-5` | anthropic → github-copilot → opencode → antigravity → google |
| **oracle** | `gpt-5.2` | openai → anthropic → google → github-copilot → opencode |
| **librarian** | `big-pickle` | opencode → github-copilot → anthropic |
| **explore** | `gpt-5-nano` | opencode → anthropic → github-copilot |
| **multimodal-looker** | `gemini-3-flash` | google → openai → zai-coding-plan → anthropic → opencode |
| **Prometheus** | `claude-opus-4-5` | anthropic → github-copilot → opencode → antigravity → google |
| **Atlas** | `claude-sonnet-4-5` | anthropic → github-copilot → opencode → antigravity → google |

### 8.3 카테고리별 프로바이더 체인

| 카테고리 | 모델 (접두사 없음) | 프로바이더 우선순위 |
|---------|-------------------|-------------------|
| **visual-engineering** | `gemini-3-pro` | google → openai → anthropic → github-copilot → opencode |
| **ultrabrain** | `gpt-5.2-codex` | openai → anthropic → google → github-copilot → opencode |
| **artistry** | `gemini-3-pro` | google → openai → anthropic → github-copilot → opencode |
| **quick** | `claude-haiku-4-5` | anthropic → github-copilot → opencode → antigravity → google |
| **unspecified-low** | `claude-sonnet-4-5` | anthropic → github-copilot → opencode → antigravity → google |
| **unspecified-high** | `claude-opus-4-5` | anthropic → github-copilot → opencode → antigravity → google |
| **writing** | `gemini-3-flash` | google → openai → anthropic → github-copilot → opencode |

### 8.4 delegate_task 사용법

#### 카테고리 기반 (작업 성격으로 모델 선택)

```typescript
delegate_task(
  category="visual-engineering",
  load_skills=["frontend-ui-ux"],
  description="대시보드 차트 컴포넌트 추가",
  prompt="1. TASK: ...\n2. EXPECTED OUTCOME: ...",
  run_in_background=false
)
```

#### 에이전트 직접 호출 (역할로 모델 선택)

```typescript
delegate_task(
  subagent_type="oracle",
  load_skills=[],
  description="경합 버그 분석",
  prompt="...",
  run_in_background=false
)
```

---

## 9) Hook 시스템 심층 분석

### 9.1 주요 훅 목록

#### 컨텍스트 & 주입

| 훅 | 이벤트 | 설명 |
|---|-------|------|
| **directory-agents-injector** | PostToolUse | 파일 읽을 때 AGENTS.md 자동 주입 |
| **directory-readme-injector** | PostToolUse | 디렉터리 컨텍스트용 README.md 자동 주입 |
| **rules-injector** | PostToolUse | `.claude/rules/`에서 조건부 규칙 주입 |
| **compaction-context-injector** | Stop | 세션 압축 중 중요 컨텍스트 보존 |

#### 생산성 & 제어

| 훅 | 이벤트 | 설명 |
|---|-------|------|
| **keyword-detector** | UserPromptSubmit | `ultrawork`/`ulw`, `search`, `analyze` 모드 감지 |
| **think-mode** | UserPromptSubmit | "think deeply", "ultrathink" 감지하여 설정 조정 |
| **ralph-loop** | Stop | 자기참조 루프 계속 관리 |
| **todo-continuation-enforcer** | Stop | TODO 완료 강제 |
| **auto-slash-command** | UserPromptSubmit | 프롬프트에서 슬래시 명령 자동 실행 |

#### 품질 & 안전

| 훅 | 이벤트 | 설명 |
|---|-------|------|
| **comment-checker** | PostToolUse | 과도한 주석 감소 알림. BDD, 지시어, 독스트링은 무시 |
| **thinking-block-validator** | PreToolUse | 사고 블록 검증으로 API 오류 방지 |
| **edit-error-recovery** | PostToolUse | 편집 도구 실패에서 복구 |

#### 복구 & 안정성

| 훅 | 이벤트 | 설명 |
|---|-------|------|
| **session-recovery** | Stop | 충돌에서 자동 복구 |
| **anthropic-context-window-limit-recovery** | Stop | 컨텍스트 초과 시 자동 요약 |
| **delegate-task-retry** | PostToolUse | 위임 작업 재시도 |

### 9.2 훅 비활성화

`oh-my-opencode.json`에서:

```json
{
  "disabled_hooks": ["comment-checker", "agent-usage-reminder"]
}
```

---

## 10) Tool 시스템 심층 분석

### 10.1 LSP 도구

| 도구 | 용도 |
|-----|------|
| `lsp_goto_definition` | 정의로 이동 |
| `lsp_find_references` | 참조 찾기 |
| `lsp_symbols` | 심볼 검색 |
| `lsp_diagnostics` | 진단 정보 |
| `lsp_prepare_rename` | 이름 변경 준비 |
| `lsp_rename` | 이름 변경 실행 |

### 10.2 AST-Grep 도구

| 도구 | 용도 |
|-----|------|
| `ast_grep_search` | 25개 이상 언어에서 AST 기반 검색 |
| `ast_grep_replace` | AST 기반 치환 |

패턴 문법:
- `$VAR` — 단일 노드 매칭
- `$$$` — 다중 노드 매칭

### 10.3 백그라운드 작업 도구

```typescript
// 백그라운드에서 실행
delegate_task(agent="explore", background=true, prompt="인증 구현 찾기")

// 작업 계속...
// 완료 시 시스템이 알림

// 필요할 때 결과 조회
background_output(task_id="bg_abc123")

// 작업 취소
background_cancel(task_id="bg_abc123")
```

---

## 11) Feature 시스템 분석

### 11.1 Background Agent 라이프사이클

```
launch → poll (2초 간격) → complete

안정성: 3회 연속 poll = idle
동시성: 프로바이더/모델별 제한
정리: 30분 TTL, 3분 stale 타임아웃
```

설정:

```json
{
  "background_task": {
    "defaultConcurrency": 5,
    "providerConcurrency": {
      "anthropic": 3,
      "openai": 5,
      "google": 10
    },
    "modelConcurrency": {
      "anthropic/claude-opus-4-5": 2,
      "google/gemini-3-flash": 10
    }
  }
}
```

### 11.2 내장 스킬

| 스킬 | 트리거 | 설명 |
|-----|-------|------|
| **playwright** | 브라우저 작업, 테스트, 스크린샷 | Playwright MCP를 통한 브라우저 자동화 |
| **frontend-ui-ux** | UI/UX 작업, 스타일링 | 디자이너 관점의 UI/UX 개발 |
| **git-master** | commit, rebase, squash, blame | 원자적 커밋 분할, 히스토리 검색 |

### 11.3 로더 우선순위

| 유형 | 우선순위 (높은 순) |
|-----|------------------|
| Commands | `.opencode/command/` > `~/.config/opencode/command/` > `.claude/commands/` |
| Skills | `.opencode/skills/` > `~/.config/opencode/skills/` > `.claude/skills/` |
| MCPs | `.claude/.mcp.json` > `.mcp.json` > `~/.claude/.mcp.json` |

---

## 12) CLI 시스템

### 12.1 사용 가능한 명령어

| 명령어 | 용도 |
|-------|------|
| `bunx oh-my-opencode install` | 대화형 설정 |
| `bunx oh-my-opencode doctor` | 14개 상태 검사 |
| `bunx oh-my-opencode run` | 세션 실행 |
| `bunx oh-my-opencode get-local-version` | 버전 확인 |

### 12.2 Doctor 검사 카테고리

| 카테고리 | 검사 항목 |
|---------|---------|
| installation | opencode, plugin 버전 |
| configuration | JSONC 유효성, Zod 검증 |
| authentication | anthropic, openai, google 인증 |
| dependencies | ast-grep, comment-checker |
| tools | LSP, MCP 연결 |
| updates | 버전 비교 |

---

## 13) MCP 시스템

### 13.1 내장 MCP 서버

| 이름 | URL | 용도 | 인증 |
|-----|-----|------|------|
| websearch | mcp.exa.ai | 실시간 웹 검색 | EXA_API_KEY |
| context7 | mcp.context7.com | 라이브러리 문서 | 없음 |
| grep_app | mcp.grep.app | GitHub 코드 검색 | 없음 |

### 13.2 MCP 비활성화

```json
{
  "disabled_mcps": ["websearch", "context7", "grep_app"]
}
```

### 13.3 스킬 MCP 관리

- **지연 로딩**: 첫 호출 시 클라이언트 생성
- **전송**: stdio, http (SSE/Streamable)
- **정리**: 5분 유휴 후 정리

---

## 14) 점검/트러블슈팅

### 14.1 경로/설정/에이전트 확인

```bash
# 버전 확인
opencode --version

# 경로 확인
opencode debug paths --print-logs

# 설정 확인
opencode debug config --print-logs --log-level=INFO

# 에이전트 확인
opencode debug agent sisyphus --print-logs --log-level=INFO
```

### 14.2 플러그인 설치 캐시 확인

```bash
ls -la ~/.cache/opencode/node_modules/oh-my-opencode
cat ~/.cache/opencode/node_modules/oh-my-opencode/package.json
```

### 14.3 Doctor 실행

```bash
bunx oh-my-opencode doctor --verbose
```

"Model Resolution" 검사에서 확인 가능:
- 각 에이전트/카테고리의 모델 요구사항
- 프로바이더 fallback 체인
- 사용자 오버라이드 (설정된 경우)
- 효과적인 해석 경로

### 14.4 민감정보 주의

다음 위치에는 세션/로그/자격증명 등이 포함될 수 있습니다:
- `~/.config/opencode/.opencode/`
- `~/.local/share/opencode/`

**인증 관련 파일은 공유/커밋 금지!**

---

## 15) 참고 자료

### 15.1 공식 문서

| 문서 | URL |
|-----|-----|
| OpenCode 설정 | https://opencode.ai/docs/config/ |
| OpenCode 플러그인 | https://opencode.ai/docs/plugins/ |
| OpenCode 서버 | https://opencode.ai/docs/server/ |
| OpenCode 에이전트 | https://opencode.ai/docs/agents/ |

### 15.2 프로젝트 내 문서

| 문서 | 위치 |
|-----|------|
| 기능 가이드 | [docs/features.md](features.md) |
| 설정 가이드 | [docs/configurations.md](configurations.md) |
| 오케스트레이션 가이드 | [docs/orchestration-guide.md](orchestration-guide.md) |
| 카테고리/스킬 가이드 | [docs/category-skill-guide.md](category-skill-guide.md) |
| 개요 | [docs/guide/overview.md](guide/overview.md) |
| 설치 가이드 | [docs/guide/installation.md](guide/installation.md) |
| 오케스트레이션 이해하기 | [docs/guide/understanding-orchestration-system.md](guide/understanding-orchestration-system.md) |

### 15.3 소스 코드 내 지식 베이스 (AGENTS.md)

| 위치 | 내용 |
|-----|------|
| `AGENTS.md` (루트) | 프로젝트 전체 개요 |
| `src/agents/AGENTS.md` | 에이전트 모듈 가이드 |
| `src/hooks/AGENTS.md` | 훅 모듈 가이드 |
| `src/tools/AGENTS.md` | 도구 모듈 가이드 |
| `src/features/AGENTS.md` | 기능 모듈 가이드 |
| `src/shared/AGENTS.md` | 공유 유틸리티 가이드 |
| `src/cli/AGENTS.md` | CLI 모듈 가이드 |
| `src/mcp/AGENTS.md` | MCP 모듈 가이드 |

---

## 부록: 설정 예시

### A. 최소 설정 (대부분의 사용자)

```jsonc
{
  "$schema": "https://raw.githubusercontent.com/code-yeongyu/oh-my-opencode/master/assets/oh-my-opencode.schema.json"
  // 대부분의 설정은 자동으로 최적화됩니다.
  // 특별히 변경할 필요가 없습니다.
}
```

### B. 모델 커스터마이즈

```jsonc
{
  "$schema": "https://raw.githubusercontent.com/code-yeongyu/oh-my-opencode/master/assets/oh-my-opencode.schema.json",

  "agents": {
    "oracle": { "model": "openai/gpt-5.2" },
    "librarian": { "model": "zai-coding-plan/glm-4.7" },
    "explore": { "model": "opencode/gpt-5-nano" }
  },

  "categories": {
    "quick": { "model": "opencode/gpt-5-nano" },
    "visual-engineering": { "model": "google/gemini-3-pro" }
  }
}
```

### C. 기능 비활성화

```jsonc
{
  "$schema": "https://raw.githubusercontent.com/code-yeongyu/oh-my-opencode/master/assets/oh-my-opencode.schema.json",

  "disabled_agents": ["multimodal-looker"],
  "disabled_hooks": ["comment-checker", "auto-update-checker"],
  "disabled_mcps": ["websearch"],
  "disabled_skills": ["playwright"]
}
```

### D. 백그라운드 작업 동시성 조절

```jsonc
{
  "$schema": "https://raw.githubusercontent.com/code-yeongyu/oh-my-opencode/master/assets/oh-my-opencode.schema.json",

  "background_task": {
    "defaultConcurrency": 5,
    "providerConcurrency": {
      "anthropic": 3,
      "openai": 5,
      "google": 10
    },
    "modelConcurrency": {
      "anthropic/claude-opus-4-5": 2
    }
  }
}
```

### E. Git Master 설정

```jsonc
{
  "$schema": "https://raw.githubusercontent.com/code-yeongyu/oh-my-opencode/master/assets/oh-my-opencode.schema.json",

  "git_master": {
    "commit_footer": true,
    "include_co_authored_by": true
  }
}
```

---

*이 문서는 oh-my-opencode 소스 코드와 OpenCode 공식 문서를 기반으로 작성되었습니다.*
