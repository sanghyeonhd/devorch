아래는 `https://opencode.ai/docs` 문서 전반(사이드바에 있는 주요 문서 페이지들 포함)을 읽고, **OpenCode(opencode)** 가 “무엇을 하는 제품인지 / 어떤 구성요소와 기능을 갖는지 / 실제 운영·확장·엔터프라이즈 적용까지 어떻게 설계되어 있는지”를 **프로젝트 관점에서 최대한 상세하게** 정리한 내용입니다. ([opencode.ai][1])

---

## 1) OpenCode란 무엇인가

**OpenCode는 오픈소스 AI 코딩 에이전트**입니다. 개발자가 “대화로 요청”하면 에이전트가 코드베이스를 읽고(컨텍스트), 계획을 세우고, 파일을 수정하고, 커맨드를 실행하면서 기능 추가/리팩터링/버그 수정 등을 수행하는 형태의 제품입니다. 사용 형태는 **터미널(TUI) 중심**이지만, 같은 백엔드 서버를 기반으로 **웹(Web UI)**, **데스크톱 앱**, **IDE 확장(VS Code 계열)** 등 여러 클라이언트를 지원합니다. ([opencode.ai][1])

---

## 2) 핵심 아키텍처: “서버(백엔드) + 다중 클라이언트”

OpenCode의 구조는 단순 “CLI 한 덩어리”가 아니라, 기본적으로:

* 사용자가 `opencode`를 실행하면 **TUI(클라이언트) + 로컬 서버(백엔드)** 가 함께 동작
* 서버는 **OpenAPI 3.1 스펙 엔드포인트**를 노출하며, 이 API를 기반으로 SDK도 생성/사용 가능
* 그래서 동일한 서버에 대해 TUI뿐 아니라 **웹 UI / 원격 attach / 다른 클라이언트**가 붙을 수 있는 구조

로 설계되어 있습니다. 이게 OpenCode가 “세션 공유”, “원격 접속”, “자동화/프로그래밍 방식 제어”를 자연스럽게 지원하는 기반입니다. ([opencode.ai][2])

---

## 3) 설치/배포 형태(크로스플랫폼)

문서 기준으로 OpenCode 설치는 크게 다음 루트가 있습니다.

* 공식 설치 스크립트(`curl ... | bash`)
* Node 패키지 매니저 기반 전역 설치(npm/bun/pnpm/yarn)
* macOS/Linux: Homebrew(tap 권장)
* Windows: Chocolatey, Scoop, npm, Mise, Docker, GitHub Releases 바이너리 등 ([opencode.ai][1])

즉 “개발자 로컬”과 “조직 배포” 모두 염두에 둔 설치 옵션 폭이 넓습니다.

---

## 4) LLM 연동 방식: Providers + Models + (선택) Zen

### 4.1 Providers / Models.dev 기반 다중 모델

OpenCode는 “특정 회사 API만”이 아니라, **AI SDK + Models.dev**를 통해 **75개+ LLM provider**를 지원하는 구조로 안내합니다.

* 키 등록: TUI에서 `/connect` 또는 CLI auth 명령으로 provider API key를 저장
* 인증정보 저장 위치: `~/.local/share/opencode/auth.json` ([opencode.ai][3])

### 4.2 모델 선택 / 기본 모델 / 변형(Variants)

Models 페이지에서는 “provider/model 형태로 모델을 지정”하고, 기본 모델 설정, 모델 구성, variants(빌트인/커스텀/순환) 같은 운영 개념을 제공합니다. ([opencode.ai][4])

### 4.3 OpenCode Zen(선택 기능)

“Zen”은 OpenCode 팀이 **검증한 모델 목록을 제공하는 큐레이션 provider**(베타)로 설명됩니다.

* `/connect`로 Zen 로그인/키 입력
* `/models`로 추천 모델 리스트 확인
* 과금은 요청 기반(크레딧 충전)으로 안내 ([opencode.ai][5])

또한 Zen 문서에는 “데이터/프라이버시(호스팅 위치, 보존 정책 예외 등)” 안내가 포함되어 있습니다. ([opencode.ai][6])

---

## 5) 설정 시스템: opencode.json(c) 중심(팀/개인/엔터프라이즈까지)

OpenCode는 **JSON 또는 JSONC** 설정 파일(`opencode.jsonc`)을 중심으로 동작하며, `$schema`를 제공해 구조화된 설정을 권장합니다. ([opencode.ai][7])

대표적으로 문서에서 다루는 설정 범주는 다음과 같습니다.

* theme, model, autoupdate 등 기본 설정 ([opencode.ai][7])
* 서버 설정(`server.port`, `server.hostname`, `mdns`, `cors`) ([opencode.ai][8])
* 포맷터(`formatter`) 내장/커스텀 구성 ([opencode.ai][8])
* 권한(`permission`)으로 도구 실행 허용/승인/차단 정책 제어 ([opencode.ai][9])

특히 `autoupdate`는 on/off뿐 아니라 `"notify"` 같은 모드도 안내하며, 패키지 매니저 설치 시 동작 제약도 언급합니다. ([opencode.ai][8])

---

## 6) 에이전트/모드 개념: Build/Plan + Subagents

OpenCode는 “에이전트(Agents)”를 핵심 UX로 설명합니다.

* **Primary agents**: 사용자가 직접 대화하는 주 에이전트 (기본적으로 Build/Plan 제공)
* **Subagents**: 주 에이전트가 특정 작업을 위해 호출하거나, 사용자가 `@` 멘션으로 직접 호출하는 보조 에이전트 (예: General/Explore 등) ([opencode.ai][10])

또한 과거 “mode” 설정이 “agent 설정으로 이동(Deprecated)”되었다는 안내가 있으며, Build는 기본적으로 도구가 활성화된 개발 모드, Plan은 변경 없이 분석/계획 중심으로 쓰는 흐름을 문서에서 강조합니다. ([opencode.ai][11])

---

## 7) 작업 실행 능력의 핵심: Tools + Permissions

### 7.1 Built-in tools(파일/명령 실행/패치/목록 등)

OpenCode는 LLM이 실제 작업을 수행할 수 있도록 다양한 “내장 도구(tools)”를 제공합니다. 예를 들어:

* 파일/디렉토리 목록(list)
* 파일 수정(edit/write/multiedit/patch 등)
* LSP 기반 코드 인텔리전스(lsp 도구, 실험 기능)
* skill 로딩
* todo 관리(todowrite/todoread) 등 ([opencode.ai][12])

### 7.2 Permissions로 “자동 실행 vs 승인 vs 차단” 제어

`permission` 설정은 각 도구 액션을:

* 자동 허용
* 실행 전 사용자 승인 요청
* 완전 차단

중 하나로 제어하는 메커니즘으로 설명됩니다. 또한 구버전의 `tools` boolean 설정이 `permission`으로 통합되며 deprecated 되었다고 명시합니다. ([opencode.ai][9])

> 이 구조는 사용자가 원했던 “승인/보안/폐쇄망 정책에 맞춘 통제”를 제품 레벨에서 제공하는 핵심 축입니다.

---

## 8) 컨텍스트 품질 강화: Rules(AGENTS.md) + Skills(SKILL.md)

### 8.1 Rules = 프로젝트 맞춤 지침(AGENTS.md)

OpenCode는 프로젝트 루트에 `AGENTS.md`를 두고, 여기에 “프로젝트 구조/규칙/코딩 표준”을 적어 LLM 컨텍스트로 포함하는 방식을 제공합니다.

* `/init`로 프로젝트를 스캔해 `AGENTS.md`를 생성/업데이트
* `AGENTS.md`를 Git에 커밋 권장
* 글로벌 규칙(`~/.config/opencode/AGENTS.md`)도 지원 ([opencode.ai][13])

### 8.2 Agent Skills = 재사용 가능한 스킬(SKILL.md)

Skills는 저장소 또는 홈 디렉토리에서 **SKILL.md 형태의 재사용 지침**을 발견하고, 필요 시 `skill` 도구로 “온디맨드 로딩”하는 개념입니다. ([opencode.ai][14])

---

## 9) 코드 이해 자동화: LSP Servers 연동

OpenCode는 LSP(Language Server Protocol)와의 통합을 통해:

* 진단(diagnostics) 기반 피드백
* 정의/참조/호버/심볼/콜 하이어라키 등 코드 인텔리전스

를 LLM 작업에 활용할 수 있도록 설계되어 있으며, 일부 언어/환경은 자동 설치/자동 감지 등의 테이블로 안내합니다. ([opencode.ai][15])

---

## 10) 외부 도구 확장: MCP servers + Custom Tools + Plugins

### 10.1 MCP servers (Model Context Protocol)

MCP를 통해 **로컬/원격 MCP 서버 도구를 추가**할 수 있고, 추가된 도구는 내장 도구와 함께 자동으로 LLM에 제공된다고 설명합니다. ([opencode.ai][16])

### 10.2 Custom Tools

프로젝트별 `.opencode/tools/` 또는 글로벌 `~/.config/opencode/tools/`에 **JS/TS로 도구 정의**를 추가해 LLM이 호출할 수 있는 커스텀 함수(툴)를 만들 수 있습니다. (정의는 JS/TS지만 실제 실행은 다른 언어 스크립트 호출도 가능) ([opencode.ai][17])

### 10.3 Plugins

Plugins는 OpenCode의 이벤트에 “훅”처럼 붙어 동작을 확장/변경하는 메커니즘으로 설명됩니다. 커뮤니티 예제도 연결합니다. ([opencode.ai][18])

---

## 11) 사용 인터페이스: TUI / Web / IDE / CLI / Server

### 11.1 TUI(터미널 UI)

TUI는 OpenCode의 대표 UX입니다. 문서에는:

* keybind(리더키 `ctrl+x` 기반)
* `/editor`, `/export`, `/models`, `/init`, `/share`, `/undo` 등 커맨드 팔레트 기반 워크플로우
* 스크롤 옵션 등 TUI 세부 설정
  이 정리되어 있습니다. ([opencode.ai][19])

### 11.2 Web UI

`opencode web` 형태로 브라우저에서 동일한 경험을 제공하며, 포트/호스트/mDNS/CORS/인증, 세션/서버 상태, 터미널 attach 등의 항목으로 구성되어 있습니다. ([opencode.ai][20])

### 11.3 IDE 확장

VS Code 및 Cursor/Windsurf/VSCodium 등 “VS Code 계열”에 대해, 통합 터미널에서 `opencode` 실행 시 확장이 자동 설치되는 흐름을 제공합니다. 또한 `/editor`·`/export`를 외부 에디터로 열기 위해 `EDITOR="code --wait"` 같은 설정도 안내합니다. ([opencode.ai][21])

### 11.4 CLI(비대화/자동화)

`opencode run "..."`처럼 비대화형 실행이 가능하고, `serve`, `attach`, `agent`, `auth`, `models --refresh` 등 “스크립팅/자동화/원격 연결”을 염두에 둔 커맨드 체계를 제공합니다. ([opencode.ai][22])

### 11.5 Headless Server + SDK

* `opencode serve`로 헤드리스 HTTP 서버 실행(OpenAPI 기반) ([opencode.ai][2])
* JS/TS SDK `@opencode-ai/sdk`로 서버를 타입세이프하게 제어 ([opencode.ai][23])

---

## 12) 공유(Share) 기능과 프라이버시 포인트

OpenCode는 대화를 링크로 공유(`/share`)할 수 있고, 공유 해제(`/unshare`)도 제공합니다. 또한 “공유 시 보존되는 데이터(대화 전체/메시지/세션 메타데이터)” 등 프라이버시 유의사항을 명시합니다. ([opencode.ai][1])

엔터프라이즈 환경에서는 공유 기능을:

* 완전 비활성화
* SSO 사용자로 제한
* 자체 호스팅
  같이 제어 가능하다고 언급합니다. ([opencode.ai][24])

---

## 13) 엔터프라이즈 대응: Network + Enterprise

### 13.1 Network (프록시/인증/사설 인증서)

기업망에서 필요한:

* 표준 프록시 환경변수 지원
* 프록시 인증
* 커스텀 인증서(사내 CA 등)
  구성을 공식 문서로 제공합니다. ([opencode.ai][25])

### 13.2 OpenCode Enterprise

OpenCode Enterprise는 “조직에서 코드/데이터가 인프라 밖으로 나가지 않게” 하기 위한 옵션으로 설명되며,

* 중앙 config
* SSO 연동
* 내부 AI 게이트웨이 연동
  같은 구성을 언급합니다. 또한 “OpenCode 자체는 코드/컨텍스트를 저장하지 않는다”는 점을 반복해서 강조하고, 실제 보안 이슈는 사용자가 선택한 provider(또는 내부 게이트웨이) 신뢰 모델에 달린다는 식으로 설명합니다. ([opencode.ai][26])

---

## 14) GitHub / GitLab 워크플로우 통합(이슈/PR 자동화)

### 14.1 GitHub

GitHub에서는 코멘트에 `/opencode` 또는 `/oc` 같은 멘션으로 작업을 트리거하면, **GitHub Actions runner 안에서** OpenCode가 실행되어:

* 이슈 트리아지
* 수정/기능 구현 후 PR 생성
  같은 자동화가 가능하다고 설명합니다. ([opencode.ai][27])

### 14.2 GitLab

GitLab은:

* CI 파이프라인 구성요소로 OpenCode를 붙이는 방식
* GitLab Duo 스타일로 코멘트 `@opencode` 트리거 방식
  둘 다 안내하며, 실행은 GitLab runner에서 이뤄진다는 점을 강조합니다. ([opencode.ai][28])

---

## 15) 업데이트/마이그레이션/트러블슈팅

* **1.0 마이그레이션**: TUI가 “완전 재작성”되었고(Go+bubbletea → 자체 프레임워크 OpenTUI: zig+solidjs), 업그레이드/다운그레이드 커맨드와 UX/키바인딩 변경점 등을 안내합니다. ([opencode.ai][29])
* **Troubleshooting**: 로그 위치(플랫폼별), 데스크톱 앱 서버 연결 문제(포트/호스트/환경변수), 플러그인 디렉토리 점검, 저장소 리셋(최후 수단) 등을 가이드합니다. ([opencode.ai][30])

---

## 16) 생태계(Ecosystem)

공식 문서에는 커뮤니티가 만든:

* 플러그인/프로젝트(예: 디스코드 봇, Neovim 플러그인, 모바일 UI, 템플릿 등)
* 에이전트/워크플로우 모음
  을 모아둔 “Ecosystem” 페이지가 있습니다. ([opencode.ai][31])

---

# 결론: OpenCode를 한 문장으로 요약하면

OpenCode는 **(1) 서버+다중 클라이언트 구조**를 기반으로, **(2) 다양한 LLM provider/models**를 연결하고, **(3) 권한/규칙/스킬/LSP/MCP/커스텀툴/플러그인**으로 “코딩 에이전트의 실행력과 통제력”을 동시에 제공하며, **(4) 개인 개발부터 엔터프라이즈(프록시·SSO·내부 게이트웨이·CI 러너 실행)까지** 확장 가능한 오픈소스 AI 코딩 플랫폼입니다. ([opencode.ai][2])

---

원하시면, 위 내용을 당신이 만들고 있는 **Devorch(무중단·다중 LLM 오케스트레이션)** 기준으로 바로 맵핑해서,

* OpenCode에서 “바로 벤치마킹해야 할 포인트(설계/구조/설정/권한/에이전트/툴링)”
* Devorch에서 “차별화 가능한 포인트(무중단 fallback, 승인 최소화, 폐쇄망 설치/배포, 프로젝트 자동 생성 파이프라인)”

이렇게 **비교표 + Devorch 적용 설계안**으로도 정리해드릴게요.

[1]: https://opencode.ai/docs/ "Intro | OpenCode"
[2]: https://opencode.ai/docs/server/ "Server | OpenCode"
[3]: https://opencode.ai/docs/providers/?utm_source=chatgpt.com "Providers"
[4]: https://opencode.ai/docs/models/ "Models | OpenCode"
[5]: https://opencode.ai/docs/zen/ "Zen | OpenCode"
[6]: https://opencode.ai/docs/zen/?utm_source=chatgpt.com "Zen"
[7]: https://opencode.ai/docs/config/ "Config | OpenCode"
[8]: https://opencode.ai/docs/config/?utm_source=chatgpt.com "Config"
[9]: https://opencode.ai/docs/permissions/ "Permissions | OpenCode"
[10]: https://opencode.ai/docs/agents/ "Agents | OpenCode"
[11]: https://opencode.ai/docs/modes/?utm_source=chatgpt.com "Modes"
[12]: https://opencode.ai/docs/tools/ "Tools | OpenCode"
[13]: https://opencode.ai/docs/rules/ "Rules | OpenCode"
[14]: https://opencode.ai/docs/skills/ "Agent Skills | OpenCode"
[15]: https://opencode.ai/docs/lsp/ "LSP Servers | OpenCode"
[16]: https://opencode.ai/docs/mcp-servers/ "MCP servers | OpenCode"
[17]: https://opencode.ai/docs/custom-tools/ "Custom Tools | OpenCode"
[18]: https://opencode.ai/docs/plugins/ "Plugins | OpenCode"
[19]: https://opencode.ai/docs/keybinds/ "Keybinds | OpenCode"
[20]: https://opencode.ai/docs/web/ "Web | OpenCode"
[21]: https://opencode.ai/docs/ide/ "IDE | OpenCode"
[22]: https://opencode.ai/docs/cli/ "CLI | OpenCode"
[23]: https://opencode.ai/docs/sdk/ "SDK | OpenCode"
[24]: https://opencode.ai/docs/share/ "Share | OpenCode"
[25]: https://opencode.ai/docs/network/ "Network | OpenCode"
[26]: https://opencode.ai/docs/enterprise/ "Enterprise | OpenCode"
[27]: https://opencode.ai/docs/github/?utm_source=chatgpt.com "GitHub"
[28]: https://opencode.ai/docs/gitlab/ "GitLab | OpenCode"
[29]: https://opencode.ai/docs/1-0/ "Migrating to 1.0 | OpenCode"
[30]: https://opencode.ai/docs/troubleshooting/?utm_source=chatgpt.com "Troubleshooting"
[31]: https://opencode.ai/docs/ecosystem/ "Ecosystem | OpenCode"
