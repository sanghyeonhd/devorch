# Devorch 3 — 채팅 우선 GUI와 코드 재사용으로 완주하는 방법

**작성일:** 2026-08-19
**상태:** 다음 스프린트의 실행 문서. `DEVORCH_3_RELEASE.md`가 **이미 착지한 것**을 기록한다면, 이 문서는 **더 만들지 말고 가져와서 끝내는 것**을 기록한다.
**선행:** `DEVORCH_3_RELEASE.md`, `DEVORCH_3_RELEASE_SCOPE_CHANGE.md`, `DEVORCH_3_RUST_MULTI_AGENT_ADE_IMPLEMENTATION.md` §25–28·§49–51, `docs/compare/opencode-analysis.md`, `docs/compare/productexplain.md`, `docs/compare/devorch-vs-opencode-analysis.md`

한 줄:

```text
코어는 이미 있다. 홈 화면을 미션 폼에서 채팅으로 바꾸고,
채팅·마크다운·세션·설치기·레이아웃은 허용 라이센스 코드를 인용한다.
미션/비교/병합은 채팅 위의 명령이지, 앱의 입구가 아니다.
```

---

## 0. 왜 이 문서가 필요한가

`DEVORCH_3_RELEASE.md`가 닫은 것:

- 네 벤더 어댑터, 정책, 검증, 평가
- 미션 루프 (워크트리 → 게이트 → 승자 병합 → 재검증)
- egui 데스크탑, 3D 별자리, macOS `.app`
- Go 퇴역, Apache-2.0

그게 **오케스트레이터**는 된다. **쓰는 제품**은 아직 아니다.

지금 GUI의 홈은 Mission Control이다. 목표·위험도·에이전트를 폼에 채우고 Run을 누른다. OpenCode, Codex, Claude Code, Orca를 쓰는 사람은 그렇게 시작하지 않는다. 텍스트를 치고, 스트림이 오고, 필요하면 비교를 켠다.

원 계획 §27의 레이아웃도 “Mission DAG / Agent panes”가 중앙이다. 그 판단은 ADE 콘솔을 염두에 둔 것이고, 일일 UX와 어긋난다. 이 문서가 그 한 줄을 뒤집는다.

원 계획 §49는 이미 같은 방향을 말해 두었다.

> After selecting native Rust GUI, **borrow UX patterns rather than frontend stacks.**

지금 할 일은 그 문장을 실행하는 것이다. 채팅창, 세션 탭, 마크다운 렌더, 설치기를 다시 발명하지 않는다.

---

## 1. 다시 만들지 말 것 (이미 있다)

에이전트 규칙 5번: *Do not add a second implementation of something a crate already does.*

| 이미 있는 것 | 크레이트 | 채팅 GUI에서의 역할 |
|---|---|---|
| 벤더 스트림 → `AgentEvent` | `devorch-agent-*` | 말풍선 소스 |
| 프로세스 스트리밍 + delta 합치기 | `devorch-process`, CLI/GUI coalesce | 타이핑 애니메이션 |
| 능력 탐지 | `devorch-agent::inspect` | 헤더의 에이전트 칩 |
| 워크트리 | `devorch-git`, `devorch-workspace` | `/compare`가 켤 때 |
| 결정론적 게이트 | `devorch-verify` | 채팅이 아니라 증거 |
| 평가·병합 | `devorch-evaluate`, `devorch-mission` | `/compare` 한 번의 뒷면 |
| 정책 | `devorch-policy` | 승인 카드 |
| 영속 | `devorch-store` | 세션·메시지 저장 |
| 라우터 | `devorch-router` | “누구한테 보낼지” 기본값 |
| Command → Core → Event | `devorch-ui::state` | **유지.** 뷰만 바뀐다 |

코어를 채팅 앱 안에 다시 짜지 않는다. 뷰가 `run_agent`와 `MissionRunner`를 호출한다.

---

## 2. 제품 결정 — 채팅이 홈이다

```
┌────────────┬──────────────────────────────────────┐
│ 세션 목록   │  대화                                 │
│            │  [user]  인증 테스트 깨졌어            │
│ 오늘        │  [grok]  failing test는 …             │
│  · 인증회귀  │  [tool]  edit src/auth.py              │
│  · 비교 4인  │  [verify] cargo test  exit 0           │
│            │                                      │
│ 에이전트    │  ┌──────────────────────────────────┐ │
│  ☑ grok    │  │ 메시지를 입력…        /compare    │ │
│  ☐ codex   │  └──────────────────────────────────┘ │
└────────────┴──────────────────────────────────────┘
```

동작:

1. 사용자는 텍스트를 친다. 그게 전부다.
2. 선택된 에이전트(기본 1명, 라우터 기본값 = `TaskRisk::Low`)가 같은 워크스페이스에서 스트리밍한다.
3. `AgentEvent`가 말풍선이 된다. `TextDelta`는 이미 coalesce된다.
4. `/compare` 또는 헤더의 “Compare”가 기존 미션 루프를 켠다. 결과는 채팅 안의 카드 + 기존 Compare/Constellation 화면.
5. `/policy`, `/agents`, `/workspaces`는 사이드 패널이지 홈이 아니다.

미션 폼은 고급 설정으로 내린다. 없애지 않는다. 입구에서 치운다.

이것은 OpenCode의 “Build agent에 말한다”, Codex TUI의 “프롬프트 한 줄”, Orca의 “프롬프트를 여러 에이전트에 팬아웃”을 한 제품에 겹친 것이다. 비교·병합은 Devorch만의 뒷면이고, 입구는 그들과 같아야 한다.

---

## 3. 라이센스 울타리 — 인용 전에 읽을 것

프로젝트는 Apache-2.0. `docs/upstream-sources.toml`에 커밋 없는 포트는 머지하지 않는다. 원 계획 §50, 에이전트 규칙 8번.

### 가져와도 되는 것 (허용적)

| 소스 | 라이센스 | 가져오는 방식 | 가져오는 것 | 가져오지 않는 것 |
|---|---|---|---|---|
| [openai/codex](https://github.com/openai/codex) `codex-rs/` | Apache-2.0 | **PORT** 가능. 커밋을 매니페스트에 적는다 | TUI 채팅 카드 구조, app-server JSON-RPC 모양, 승인 프롬프트 UX | ChatGPT 데스크탑 앱, 모델 가중치, 브랜드 |
| [anomalyco/opencode](https://github.com/anomalyco/opencode) | MIT | **UX 인용** 기본. 코드 PORT는 매니페스트 | 세션 목록 + 트랜스크립트 + 컴포저 3단, 탭, `/` 명령, 권한 프롬프트 UX | SolidJS/Tauri 스택 통째, Zen 과금, 클라우드 인프라 |
| [stablyai/orca](https://github.com/stablyai/orca) | MIT | **UX 인용** | 한 프롬프트 → N 워크트리 팬아웃,  Diff 주석, 에이전트 칩 | Electron, 임베디드 Chromium, VS Code 에디터, 모바일 앱 |
| [google-gemini/gemini-cli](https://github.com/google-gemini/gemini-cli) | Apache-2.0 | 관찰 + 필요 시 PORT | 헤드리스 스트림 스키마 (라이브 캡처가 막힌 부분) | 브랜드, 인증 UI |
| 우리 자신의 `devorch-go-final` | 우리 코드 | **가장 먼저 본다** | Bubbletea TUI의 세션·메시지·`/agent`·`/session` UX. 라이센스 문제 없음 | Go 런타임, 정적 웹 UI, 줄 단위 번역 (원 문서가 금지) |
| `egui_commonmark` | MIT OR Apache-2.0 | **의존성** | 마크다운 렌더 | 직접 짠 파서 |
| `syntect` / `egui_extras` | MIT/Apache | **의존성** (extras는 이미 있음) | 코드 펜스 하이라이트 | 에디터 |
| `cargo-packager` / `cargo-bundle` | MIT/Apache | **도구** | dmg/msi/appimage | 손수 짠 설치기 |

### 가져오면 안 되는 것

| 소스 | 이유 |
|---|---|
| Claude Code 저장소 | 오픈소스 재배포 라이센스가 아님. 어댑터는 **캡처한 stdout만** 사용한다. 이미 그렇게 되어 있다 (`upstream-sources.toml`). |
| Cursor, Windsurf, Copilot Chat UI | 독점. 스크린샷으로 레이아웃을 기억하는 것은 아이디어고, 코드를 가져오는 것은 아니다. |
| OpenCode의 SST/클라우드 인프라 | 로컬-퍼스트와 반대. 서버+다중 클라이언트 **아이디어**만 취한다. |
| Orca의 Electron+Chromium+VS Code | 원 계획이 Tauri/WebView를 폴백으로 둔 이유와 같다. Devorch는 에디터가 아니다. |
| GPL/AGPL 본체 (일부 OpenHands 구성, 일부 확장) | Apache-2.0과 섞지 않는다. 쓰기 전에 파일 단위 라이센스를 확인한다. |

인용 절차 (원 계획 §50 그대로):

```toml
[[source]]
name = "openai-codex-tui-transcript"
repo = "https://github.com/openai/codex"
commit = "<40자 sha>"
license = "Apache-2.0"
usage = "PORT"          # 또는 "UX" — 코드를 안 옮기면 usage = "UX" 와 source_path 생략
source_path = "codex-rs/tui/src/..."
target_crate = "devorch-ui"
```

`usage = "UX"`는 레이아웃·인터랙션만 보고 우리 코드로 다시 그린 것이다. 줄 단위 복사가 아니다. `usage = "PORT"`만 커밋이 필수다. 애매하면 PORT로 적는다.

---

## 4. 세 제품에서 그대로 가져올 UX (다시 그리지 말 것)

원 계획 §49 티어 C: Orca, OpenHands를 참고하고 프론트 스택은 빌리지 않는다.

### 4.1 OpenCode — 채팅 셸

`docs/compare/productexplain.md`가 이미 정리했다. OpenCode의 본체는 “대화로 요청하면 에이전트가 파일을 고친다”이고, 구조는 **로컬 서버 + 여러 클라이언트**다.

가져올 것:

- 왼쪽: 세션 리스트 (오늘 / 어제, 제목 = 첫 사용자 메시지)
- 가운데: 트랜스크립트. 사용자 / 어시스턴트 / 도구 / 오류가 다른 카드
- 아래: 여러 줄 컴포저, 붙여넣기, `@파일` 힌트, `/` 명령 팔레트
- 헤더: 모델·에이전트 전환, 새 세션

가져오지 말 것: SolidJS, Vite, Tauri 2, Tailwind, 75-provider Models.dev 레이어. Devorch는 설치된 CLI 네 개만 말한다.

구현: `Screen::Chat`이 기본. 세션은 `devorch-store` documents (`kind = "session"`). 메시지 한 줄은 이미 있는 이벤트 저널에 `workspace`/`session` 태그로 쌓인다.

### 4.2 Codex TUI — 이벤트 → 카드

`openai/codex`의 `codex-rs/tui`는 Apache-2.0이고 Rust다. Devorch와 언어가 같다.

가져올 것 (PORT 후보, 커밋 기록):

- 사용자 메시지 / 어시스턴트 마크다운 / 도구 호출 / 패치 / 승인 을 **카드 타입**으로 나눈 렌더
- 스트리밍 중 마지막 어시스턴트 카드만 다시 그리는 방식
- 승인 큐 UX (정책 엔진의 Ask/AskStrongly와 1:1)

가져오지 말 것: Codex의 샌드박스, 모델 클라이언트, ChatGPT 로그인. 우리는 이미 벤더 CLI를 띄운다.

`codex-rs/app-server`의 JSON-RPC 모양은 “CLI와 GUI가 같은 코어를 본다”는 우리 규칙과 같다. 새 프로토콜을 만들지 말고, 이미 있는 `Command` / `AgentEvent`를 그 역할에 쓴다.

### 4.3 Orca — 팬아웃은 명령이다

Orca는 MIT, Electron. 스택은 빌리지 않는다. 한 문장만 가져온다.

> 한 프롬프트를 여러 에이전트에 팬아웃하고, 각각 워크트리에서 돌리고, 승자를 고른다.

그게 이미 `MissionRunner`다. Orca는 그걸 **채팅에서** 켠다. 우리도 `/compare` 한 줄이면 된다.

```
/compare high  인증 테스트 고쳐
```

→ 기존 `MissionRequest { goal, risk: High, ... }` → 채팅에 “비교가 시작됐다” 카드 → 끝나면 승자 카드 + Compare 화면 링크.

폼을 다시 그리지 않는다.

### 4.4 우리 Go TUI — 가장 가까운 유산

태그 `devorch-go-final`. `docs/compare/devorch-vs-opencode-feature-analysis.md`에 `/session`, `/agent`, `/model`, `/export`가 이미 있었다. 그 TUI는 채팅이었다.

Go를 되살리지 않는다. 명령 이름과 정보 구조를 이식한다. 줄 단위 번역은 원 문서가 금지한다. 명령 표는 우리 것이므로 매니페스트가 필요 없다.

| Go TUI | Rust 채팅 |
|---|---|
| `/session` | 세션 목록 + `Ctrl+N` |
| `/agent` | 헤더 칩, 멀티셀렉트 |
| `/model` | 해당 CLI가 지원하면 전달, 없으면 숨김 |
| `/export` | 세션 JSON을 store에서 dump |
| 메시지 스크롤 | 트랜스크립트 |

---

## 5. 직접 짜지 말고 의존성으로 쓸 것

새 크레이트 요청은 인테그레이터가 `Cargo.toml`에 넣는다 (규칙 7). 이 문서가 요청을 기록한다.

| 필요 | 만들지 말 것 | 쓸 것 | 라이센스 |
|---|---|---|---|
| 채팅 마크다운 | CommonMark 파서 | `egui_commonmark` | MIT OR Apache-2.0 |
| 코드 펜스 | 하이라이터 | `egui_commonmark` + `syntect` 피처 | 동일 |
| 여러 줄 입력 | 커스텀 에디터 | `egui::TextEdit::multiline` + Enter 전송 / Shift+Enter 줄바꿈 | 이미 있음 |
| 세션 검색 | 검색 엔진 | store의 제목 LIKE. 나중이면 `nucleo` | MPL-2.0 — 쓰면 고지 |
| macOS/Windows/Linux 설치 파일 | 손수 번들 스크립트 확장 | `cargo-packager` 또는 기존 `xtask bundle-macos`를 리눅스/윈도만 보강 | MIT |
| 아이콘 | 독자 일러스트 | 단색 마크. Lucide 등 MIT 아이콘을 SVG로 | 아이콘 라이센스 확인 |
| 신택스 테마 | 독자 테마 | egui 기본 + 이미 있는 `theme.rs` | — |

`egui_commonmark`는 인테그레이터 추가 대상이다. 추가 전에 이 표가 요청이다.

3D 별자리는 **남긴다.** Compare 화면의 두 번째 모드다. 채팅 홈에 올리지 않는다.

---

## 6. 아키텍처 — 뷰만 바꾸고 코어는 그대로

```
사용자가 입력
    │
    ▼
Command::Send { text, agents, mode: Chat | Compare }
    │
    ├─ Chat ──────► run_agent() ──► AgentEvent* ──► 말풍선
    │
    └─ Compare ───► MissionRunner ──► MissionRecord ──► 카드 + Compare 화면
```

추가 크레이트는 없다. `devorch-ui`에 `chat.rs` (렌더)와 `state.rs`의 `Screen::Chat`, `ChatSession`, `ChatMessage`면 충분하다.

메시지 모델 (새 프로토콜 타입 금지에 가깝게, UI 전용):

```text
ChatMessage
  role: User | Agent | Tool | Verify | System
  agent: Option<AgentKind>
  text: String            // coalesce된 본문
  events: Vec<AgentEvent> // 원본, 접으면 펼칠 수 있음
```

`AgentEvent`를 다시 정의하지 않는다. 감싼다.

세션 저장: `devorch-store`의 documents (`kind = "chat_session"`). 스키마 마이그레이션이 필요하면 `0003_chat.sql`. Go `devorch.db`와 섞지 않는다.

CLI도 같은 입구를 갖는다. 폼을 복제하지 않는다.

```bash
devorch chat                         # REPL, 기본 에이전트 1
devorch chat --agents grok,codex     # 같은 프롬프트를 둘에게
echo "인증 테스트 고쳐" | devorch chat
devorch chat --compare --risk high
```

`devorch mission run`은 남긴다. 스크립트와 CI를 위해.

---

## 7. 화면 재배치

지금:

```
Missions | Agents | Workspaces | Compare | Policy | Settings
```

다음:

```
Chat(기본) | Sessions | Compare | Agents | Policy | Settings
```

Workspaces는 Chat 헤더의 경로 표시 + Settings. 사이드바에서 뺀다.

Mission Control 폼은 Compare 화면 상단 “New comparison”과 `devorch mission run`에만 남긴다.

원 계획 §27의 3단(레일 / 워크스페이스 / 인스펙터)은 **채팅 레이아웃으로 재해석**한다. 레일 = 세션, 중앙 = 대화, 인스펙터 = 선택 시 정책·비용·검증. 새 셸을 만들지 않는다.

---

## 8. 스프린트 — 다시 만들지 않는 순서

게이트는 범위 변경 문서와 같다. 네이티브 제어·브라우저는 여전히 없다.

### S1 — 채팅 홈 (목표: 하루)

만지는 곳: `devorch-ui`만.

- `Screen::Chat`을 기본으로
- 컴포저 + 트랜스크립트 + 에이전트 칩
- `Command::Send` → 기존 `run_agent` 스트리밍
- 이미 있는 coalesce를 말풍선에 연결
- 마크다운은 `egui_commonmark` (인테그레이터가 의존성 추가)

완료: 창을 열고 한 줄 치면 설치된 CLI가 대답한다. 미션 폼을 거치지 않는다.

### S2 — 세션 (목표: 하루)

- store documents로 세션 persist
- 왼쪽 목록, 새 세션, 제목 = 첫 메시지
- CLI `devorch chat`
- Go TUI 명령 표 이식 (`/session`, `/agent`, `/export`)

완료: 앱을 꺼도 대화가 남는다.

### S3 — 채팅에서 비교 (목표: 하루)

- `/compare` → 기존 `MissionRunner`
- 진행 카드는 이미 있는 progress log
- 끝나면 Compare / Constellation (이미 있음)
- 검증 카드는 `VerificationReport`를 접어 보여 준다. 새 엔진 없음

완료: 한 줄로 네 에이전트 비교가 시작되고, 승자 병합은 기존 경로.

### S4 — 설치와 다듬기 (목표: 하루)

- `xtask bundle-macos` 유지
- Windows/Linux는 `cargo-packager` 인용. NSIS/AppImage를 손수 짜지 않음
- OpenCode 데스크톱의 탭 UX를 세션 탭으로만
- README 첫 화면을 채팅으로 고침

완료: “설치하고 말 걸면 된다.”

하지 않는 것 (릴리스 문서 §11과 동일 + 이 문서의 추가):

- Electron/Tauri로 GUI 재작성
- 임베디드 터미널 (Orca Ghostty). 도구 출력은 카드면 충분
- 임베디드 에디터
- 브라우저 자동화, 네이티브 OS 제어
- OpenCode 서버를 Devorch 안에 띄우기
- 75개 프로바이더. 네 CLI가 제품이다

---

## 9. 파일 단위 — 만지는 곳, 만지지 않는 곳

만진다:

```
crates/devorch-ui/src/state.rs      Screen::Chat, ChatSession
crates/devorch-ui/src/views.rs      사이드바 순서, chat()
crates/devorch-ui/src/chat.rs       신규. 트랜스크립트·컴포저
crates/devorch-ui/src/app.rs        Send → run_agent
crates/devorch-store/migrations/    세션 문서가 기존 documents로 부족할 때만
bins/devorch/src/main.rs            chat 서브커맨드
docs/upstream-sources.toml          PORT가 생기는 즉시
README.md                           첫 사용 = 채팅
```

만지지 않는다 (이미 맞다):

```
crates/devorch-agent-*
crates/devorch-evaluate
crates/devorch-verify
crates/devorch-policy
crates/devorch-git
crates/devorch-mission          # 호출만
crates/devorch-ui/src/graph3d.rs
```

규칙 5를 어기는 신호: `chat.rs` 안에 워크트리 생성, 점수, 병합이 생기면 잘못이다. 그건 `MissionRunner`의 일이다.

---

## 10. 목표 문서 대비 이 스프린트가 닫는 것

| 원 목표 (Document 1 DoD) | alpha.1 | 이 스프린트 |
|---|---|---|
| Rust 런타임 | 닫힘 | — |
| Rust GUI | 창은 있음, 입구가 폼 | 채팅이 입구 |
| Go 불필요 | 닫힘 | — |
| 네 CLI 탐지 | 닫힘 | 칩으로 노출 |
| fixture 스위트 | 닫힘 | — |
| 라이브 스모크 4종 | Grok 미션 1, Gemini 실패 | 채팅에서 재실행. Gemini는 헤드리스 인증이 막힘. |
| 워크트리 격리 | 닫힘 | `/compare`가 재사용 |
| 인벤토리 | 닫힘 | — |
| 결정론적 평가 | 닫힘 | 채팅 카드로 보여 줌 |
| 병합 + 재검증 | 닫힘 | `/compare` |
| 재시작 생존 | 미션만 | 세션도 |
| 유휴 프로세스 0 | 닫힘 | 채팅도 턴이 끝나면 종료 |
| 3 OS CI | clippy는 수정됨. 줄바꿈은 이 패치 | — |

원 계획의 “Mission Control이 홈”은 이 문서가 대체한다. 기술 판단(증거 > 주장, 인벤토리, 정책, Rust GUI)은 대체하지 않는다.

---

## 11. GitHub의 “생성된 코드”를 쓸 때

GitHub에는 Codex·Claude·Gemini가 만든 에이전트 UI가 많다. 별 수가 허가가 아니다.

쓰기 전 세 질문:

1. **라이센스 파일이 저장소 루트에 있는가?** 없으면 인용 금지.
2. **Apache-2.0 / MIT / BSD / ISC 인가?** 아니면 법률 검토 없이 금지.
3. **그 파일이 우리 크레이트가 이미 하는 일인가?** 그러면 규칙 5. 확장하거나 버린다.

통과하면 `docs/upstream-sources.toml`에 적고 포팅한다. “데모 보고 기억해서 다시 짬”은 UX 인용이고, 줄을 가져오면 PORT다.

Claude/Gemini가 만든 gist·gist 아닌 스니펫은 라이센스가 없는 경우가 많다. **없는 라이센스는 가져오지 않는다.**

---

## 12. 완료의 정의 (이 문서)

다음이 한 머신에서 관측되면 이 스프린트는 끝이다.

```text
1. 앱을 열면 채팅이 보인다. 미션 폼이 아니다.
2. 한 줄을 치면 설치된 CLI가 스트림으로 대답한다.
3. 앱을 재시작해도 그 세션이 있다.
4. /compare 가 기존 미션 루프를 돌리고 승자를 병합한다.
5. 새 채팅 경로에 워크트리/점수/병합 구현이 없다.
6. PORT한 줄은 upstream-sources.toml 에 커밋이 있다.
7. 게이트 네 개 + verify-agent-rules 가 세 OS에서 통과한다.
```

관측하지 않은 성공을 보고하지 않는다. 실패하는 명령을 숨기지 않는다. 테스트를 지워 게이트를 통과시키지 않는다.

---

## 13. 바로 다음에 할 일

1. 인테그레이터: `egui_commonmark`를 워크스페이스 의존성으로 추가.
2. `devorch-ui`에 `Screen::Chat` + 컴포저 + `run_agent` 연결 (S1).
3. 첫 PORT가 생기면 `upstream-sources.toml`을 비어 있지 않게 만든다.

코어를 더 기다릴 필요는 없다. 대답하는 창이 없는 것뿐이다.
