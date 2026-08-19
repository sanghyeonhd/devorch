# Devorch 3 — 채팅 표면: 재사용, Rust 판단, 시뮬레이션 개정

**작성일:** 2026-08-19
**상태:** 실행 금지. 이 문서는 구현 지시가 아니라, 시뮬레이터가 이전 초안을 **개정한 계획**이다.
**시뮬레이터:** `docs/upgrade/devorch_chat_surface_simulation.py` → `devorch_chat_surface_simulation.json`
**선행:** `DEVORCH_3_RELEASE.md`, 이 파일의 이전 초안(egui 채팅 홈, 4일), `DEVORCH_3_RUST_MULTI_AGENT_ADE_IMPLEMENTATION.md` §25–28·§49–51, Codex `codex-rs/tui` (Apache-2.0)

한 줄:

```text
Rust는 고집할 필요가 없다. 그러나 Codex/Claude 급 터미널을 빠르게
만들면서 부담을 줄이려면 Rust를 유지하는 편이 맞다.

유지하는 곳은 코어와 TUI다. egui 채팅 홈은 버린다.
데스크탑은 Compare/정책 동반 창이다. Claude.app 비주얼은 목표가 아니다.
```

시뮬레이터 결과: **`REVISE THE PLAN — do not execute S1–S4 as written`**

품질 막대 11개 중 지금 1, 이전 초안 S1으로 3, TUI-first로 9.

---

## 0. 무엇을 시뮬레이트했나

이전 개발 시뮬레이션이 git 워크트리와 평가기를 실제로 돌린 것처럼, 이번에도 산문이 아니라 프로그램을 돌렸다. 구현은 하지 않았다.

검사한 것 (저장소에서 읽음):

| 실측 | 값 |
|---|---|
| CLI | clap 서브커맨드. 기본 진입이 TUI가 아님. `chat` 명령 없음 |
| GUI 기본 화면 | `Screen::MissionControl` |
| 컴포저 | `TextEdit::singleline` 목표 필드 |
| `ratatui` / `egui_commonmark` | Cargo.toml에 없음 |
| `AgentEvent` | TextDelta, Tool*, PermissionRequired 있음 |
| `run_agent` | 원샷 `AgentRun` |
| `ExecuteRequest` | `prompt`, `workdir`, `model`, `timeout`. **세션 id 없음** |
| 이벤트 저널 | 있음. `chat_session` 마이그레이션은 없음 |
| 이전 초안 | egui 채팅 홈 4일, CLI는 `devorch chat` REPL |

그다음 Codex TUI / Claude TUI의 일일 동선 11개를 현재·이전 초안·TUI-first에 대입하고, 네 아키텍처의 부담·속도·천장 점수를 매겼다.

첫 워크트리 시뮬레이션이 “테스트 후 병합”이 `__pycache__`에 깨진 것과 같은 종류의 발견이다. 이번에는 “egui에 말풍선을 붙이면 Codex가 된다”가 깨진다.

---

## 1. 판단 — Rust를 유지하는가

**유지한다. 이데올로기가 아니라 부담과 품질 막대 때문이다.**

사용자가 낮추고 싶은 부담은 런타임 수, CI 세계, 두 개의 채팅 구현이다. 사용자가 맞추고 싶은 막대는 Codex와 Claude의 **데스크탑과 터미널**이다.

그 막대를 정직하게 읽으면:

- Codex의 공개된 높은 완성도는 **`codex-rs/tui` (ratatui, Apache-2.0)** 이다. `codex`만 치면 TUI가 열린다. ChatGPT 데스크탑은 이 저장소에 없다.
- Claude Code의 높은 완성도는 **터미널 TUI**다. Claude.app / VS Code 확장은 오픈소스가 아니다.
- 따라서 “Codex 또는 Claude 급”을 **가져올 수 있는 형태**는 TUI다. 데스크탑 크롬은 가져올 수 없다.

네 옵션:

| 옵션 | Rust? | 쓸 수 있는 채팅 | Codex TUI 근접 | 막대 충족 | 부담 |
|---|---|---|---|---|---|
| A. 이전 초안: egui 홈 4일 + clap REPL | 예 | 4일 | 도달 안 함 | 아니오 | 툴체인은 가볍고, 나중에 TUI를 또 짜는 빚 |
| **B. 코어 + ratatui TUI가 제품. egui는 동반** | **예** | **~10일** | **~30일** | **예 (터미널)** | **언어 하나, 크레이트 가족 하나** |
| C. 코어 + OpenCode/Tauri 셸 | 아니오 | ~14일 | 데스크탑은 더 예쁨 | 예 (비주얼) | Bun+Rust, CI 둘, 움직이는 타깃 PORT |
| D. egui 채팅과 TUI를 따로 | 예 | ~20일 | 가능 | 아니오 | 규칙 5 위반. 컴포저가 둘 |

**B를 고른다.**

이유, 시뮬레이터가 적은 그대로:

1. 품질 막대로 지목된 Codex가 Rust TUI다. Rust를 유지하는 것이 그 막대를 복사하는 방법이다.
2. 코어는 이미 Rust다. Bun/Tauri를 얹으면 부담이 늘고, “Rust로 부담을 덜고 싶다”와 반대다.
3. egui는 주 채팅 표면이 못 된다. TUI가 없는 데다, 나중에 TUI를 넣으면 `chat.rs`와 중복된다.
4. Claude.app 비주얼을 나흘에 맞추겠다는 말은, 미추적 파일을 무시한 채 “테스트 후 병합”과 같은 계획 실패다.
5. `/compare` → `MissionRunner`만은 이전 초안에서 범위가 맞다.

Rust를 버리는 조건 (지금은 해당 없음):

- 데스크탑이 Claude.app과 **픽셀 단위로** 같아야 하고, 터미널은 부차다 → 그때만 C (OpenCode MIT 셸). 부담이 올라가는 것을 명시적으로 산다.

---

## 2. 이전 초안이 틀린 곳 (PLAN-1 … PLAN-7)

시뮬레이터 여정에서 나온 결함. 구현으로 고치지 않았고, 계획만 고친다.

**PLAN-1.** Codex/Claude의 일일 드라이버는 터미널 바이너리다. 이전 초안은 GUI 창을 입구로 둔다. 뒤집혀 있다.

적용: `devorch` 인자 없음 → TUI. `devorch-ui`는 동반 앱.

**PLAN-2.** `devorch chat` REPL은 Codex 급이 아니다. 터미널 표면은 여러 줄 컴포저·붙여넣기·슬래시 명령이 있는 TUI여야 한다.

적용: clap `chat` 서브커맨드를 만들지 않는다. 기본 커맨드가 TUI다. `devorch mission` 등 기존 서브커맨드는 스크립트용으로 남긴다.

**PLAN-3.** 멀티턴은 뷰 변경이 아니다. `ExecuteRequest`에 벤더 세션 id가 없다. 보내기마다 `run_agent()` 원샷이면 스레드가 끊긴다.

적용: `ExecuteRequest`에 `resume: Option<String>` (벤더 세션 id). 어댑터가 `--resume` / 동등 플래그를 붙인다. 능력 탐지가 이미 `caps.resume`를 본다. 뷰보다 **먼저**.

**PLAN-4.** 승인은 Settings 화면이 아니다. `PermissionRequired`가 이미 이벤트에 있다. 턴을 막는 프로토콜인데 S1–S4에 이벤트 루프가 없다.

적용: TUI 트랜스크립트에 승인 카드. Allow/Deny가 자식에게 전달되거나, 거절 후 턴 종료. 정책 엔진은 그대로 호출만.

**PLAN-5.** Esc로 자식 취소가 없으면 Codex/Claude 급 채팅이 아니다.

적용: `devorch-process`에 사용자 취소. TUI Esc → kill. 타임아웃만으로는 부족하다.

**PLAN-6.** 저널이 이미 트랜스크립트다. `chat_session` 문서로 대화를 한 번 더 저장하지 않는다.

적용: 세션 메타만 documents (`kind = "session_meta"`: 제목, repo, agents, vendor_session_id). 본문은 `SessionRecorder` 재재생.

**PLAN-7.** 독점 데스크탑 크롬은 범위 밖. TUI 인터랙션이 범위 안이고, 그건 Rust 모양이다.

적용: 데스크탑 DoD에서 “Claude.app처럼 보인다”를 삭제. “같은 세션을 Compare/정책으로 본다”로 교체.

이전 초안의 **유효한 부분** (유지):

- 코어를 채팅 안에 다시 짜지 않는다.
- 라이센스 울타리와 `upstream-sources.toml`.
- `/compare`는 `MissionRunner`.
- 미션 폼은 입구에서 내린다 (다만 채팅 홈은 TUI다).
- 3D 별자리는 Compare에 남긴다.
- Electron/임베디드 에디터/브라우저 자동화는 하지 않는다.

---

## 3. 다시 만들지 말 것 (코어 — 유지)

에이전트 규칙 5번.

| 이미 있는 것 | 크레이트 | 채팅 표면에서의 역할 |
|---|---|---|
| 벤더 스트림 → `AgentEvent` | `devorch-agent-*` | 카드 소스 |
| 스트리밍 + delta 합치기 | `devorch-process`, GUI coalesce | TUI 마지막 어시스턴트 카드 |
| 능력 탐지, `resume` 플래그 | `devorch-agent::inspect` | 헤더 칩, `--resume` 가능 여부 |
| 워크트리 | `devorch-git`, `devorch-workspace` | `/compare` |
| 게이트 / 평가 / 병합 | `devorch-verify`, `evaluate`, `mission` | `/compare` 뒷면 |
| 정책 | `devorch-policy` | 승인 카드의 결정 |
| 이벤트 저널 | `devorch-events`, `store::journal` | 트랜스크립트 본문 |
| Command → Core → Event | `devorch-ui::state` | 데스크탑 동반 창만 |

TUI도 같은 `run_agent` / `MissionRunner` / 저널을 쓴다. 두 번째 오케스트레이터를 만들지 않는다.

---

## 4. 제품 모양 — TUI가 홈, 데스크탑은 동반

```
$ devorch
┌──────────────────────────────────────────────────────────┐
│ grok  ·  ~/proj  ·  session 01HZX…                       │
│──────────────────────────────────────────────────────────│
│ you   인증 테스트 깨졌어                                   │
│ grok  failing test는 test_auth.py:41 …                   │
│ tool  edit src/auth.py                              ok   │
│ need  process.run cargo test   [allow] [deny]            │
│──────────────────────────────────────────────────────────│
│ >  메시지를 입력…                           /compare  ?   │
└──────────────────────────────────────────────────────────┘
```

`devorch-ui` (egui):

```
Agents | Compare | Constellation | Policy | Settings
```

채팅 탭을 넣지 않는다. 라이브 로그는 같은 저널의 **읽기 전용** 뷰일 수 있다. 컴포저를 두 개 두지 않는다 (옵션 D, 규칙 5).

동작:

1. `devorch` → TUI. 텍스트를 친다.
2. 기본 1 에이전트 (`TaskRisk::Low` 라우터).
3. `AgentEvent`가 카드. `TextDelta` coalesce.
4. 두 번째 턴은 `ExecuteRequest.resume`로 같은 벤더 스레드.
5. Esc는 턴 취소.
6. `PermissionRequired`는 인라인 승인.
7. `/compare` → 기존 미션 루프. 결과는 카드 + 원하면 데스크탑 Compare.
8. 종료 후 `devorch` 다시 → 세션 피커 (저널 + session_meta).

---

## 5. 라이센스 울타리 (유지, TUI PORT를 연다)

프로젝트는 Apache-2.0. 커밋 없는 PORT는 머지하지 않는다.

| 소스 | 라이센스 | 방식 | 가져오는 것 | 가져오지 않는 것 |
|---|---|---|---|---|
| [openai/codex](https://github.com/openai/codex) `codex-rs/tui` | Apache-2.0 | **PORT** 1순위. 커밋 필수 | 트랜스크립트/컴포저/슬래시/승인 카드 구조 | ChatGPT 데스크탑, 브랜드, 모델 클라이언트 |
| `codex-rs/app-server` 모양 | Apache-2.0 | UX. 새 JSON-RPC를 만들지 말고 있는 `Command`/`AgentEvent`를 그 역할에 쓴다 | CLI와 GUI가 같은 코어 | 별도 서버 프로세스 (당장은 필요 없음) |
| [anomalyco/opencode](https://github.com/anomalyco/opencode) | MIT | UX만 | `/` 팔레트, 세션 리스트 정보구조 | SolidJS/Tauri 통째 (옵션 C가 되기 전에는) |
| [stablyai/orca](https://github.com/stablyai/orca) | MIT | UX | `/compare` 팬아웃 | Electron, Chromium, VS Code 에디터 |
| `devorch-go-final` TUI | 우리 코드 | 명령 표 | `/session` `/agent` `/export` | Go 런타임, 줄 단위 번역 |
| `ratatui` + `crossterm` | MIT | **의존성** (인테그레이터) | TUI 런타임 | 위젯을 처음부터 |
| `tui-textarea` 또는 Codex 컴포저 PORT | MIT / Apache-2.0 | 의존성 또는 PORT | 여러 줄 입력 | 손수 짠 에디터 |
| `egui_commonmark` | MIT OR Apache | **당장은 요청하지 않음** | — | 채팅 홈을 egui로 만들기 위해 넣지 않는다 |

Claude Code 소스, Cursor, 라이센스 없는 gist: 금지. 이전과 같다.

Codex TUI를 PORT하기 전에:

```toml
[[source]]
name = "openai-codex-tui-transcript"
repo = "https://github.com/openai/codex"
commit = "<40자 sha>"
license = "Apache-2.0"
usage = "PORT"
source_path = "codex-rs/tui/src/"
target_crate = "devorch-tui"
```

---

## 6. 아키텍처 — 표면은 둘, 채팅 구현은 하나

```
사용자 키입력 (TUI)
    │
    ▼
devorch-tui  (유일한 컴포저)
    │
    ├─ Send ──── resume? ──► run_agent() ──► AgentEvent* ──► 저널 ──► 카드
    ├─ Esc  ────────────────► process cancel
    ├─ Allow/Deny ──────────► 정책 결정 후 턴 계속/종료
    └─ /compare ────────────► MissionRunner ──► 저널 + MissionRecord

devorch-ui (egui) 는 저널/미션을 읽기만 한다
```

새 크레이트: `devorch-tui` (바이너리 `devorch`가 기본으로 연다). `devorch-ui`에 `chat.rs`를 추가하지 않는다.

인테그레이터 의존성 요청 (규칙 7, 시뮬레이터 `integrator_dependency_requests`):

```
ratatui
crossterm
```

컴포저는 PORT 또는 `tui-textarea`. 고르기 전에 라이센스와 커밋을 매니페스트에 적는다.

`ExecuteRequest` 확장 (뷰보다 먼저, `devorch-agent`):

```text
resume: Option<String>   // vendor session id from SessionStarted
cancel: watch / CancellationToken  // PLAN-5
```

프로토콜 타입을 새로 만들지 않는다. `AgentEvent`가 이미 충분하다.

---

## 7. 개정 스프린트 (실행은 이 문서가 승격된 다음)

나흘에 Codex 급을 끝낸다는 타임박스는 시뮬레이터가 기각했다. 아래는 **순서**이지 달력이 아니다. 각 단계는 관측 가능한 DoD가 있다.

### T0 — 스레드가 되게 (코어, UI 없음)

만지는 곳: `devorch-agent` `ExecuteRequest`, 각 어댑터의 `build_spec`, 프로세스 취소.

- `resume`가 스펙에 붙는다. fixture로 `--resume` / 동등이 생기는지 검사.
- 사용자 취소가 자식을 죽인다.
- 멀티턴을 fake agent + 저널로 테스트.

완료: 뷰 없이, 두 번째 `run_agent`가 같은 벤더 세션을 잇는다.

이전 초안 S1이 이것을 건너뛰었다. PLAN-3/5.

### T1 — `devorch`가 TUI를 연다

만지는 곳: `bins/devorch`, 신규 `devorch-tui`.

- 인자 없음 → TUI. `--help` / 서브커맨드는 그대로.
- 컴포저 + 트랜스크립트 + 한 에이전트 칩.
- `TextDelta` 카드. 마크다운은 나중. 먼저 모노 텍스트.

완료: `devorch`를 치고 한 줄 보내면 설치된 CLI가 스트림으로 대답한다. GUI를 열 필요 없다.

Codex: `codex` 인자 없음. Claude: `claude` 인자 없음.

### T2 — 세션, 슬래시, 승인, Esc

- session_meta + 저널 재재생 (PLAN-6).
- `/agent` `/export` `/compare` `/session`.
- `PermissionRequired` 인라인 (PLAN-4).
- Esc 취소 (PLAN-5).

완료: 껐다 켜서 어제 대화를 고르고, 승인을 거절할 수 있고, 폭주하는 턴을 죽일 수 있다.

### T3 — `/compare`와 데스크탑 동반

- `/compare` → 기존 `MissionRunner` (이전 초안에서 유일한 맞는 조각).
- egui 기본 화면을 Mission 폼에서 Compare/Agents로. **채팅 화면을 추가하지 않음.**
- 데스크탑은 같은 미션/저널을 보여 준다.

완료: 터미널에서 비교가 돌아가고, 창에서 별자리와 증거 표를 본다.

### T4 — 마크다운·붙여넣기·설치

- Codex TUI에서 컴포저/붙여넣기/CRLF 처리를 PORT (0.148 체인지로그가 그 버그를 고친 이유 — 우리도 그 함정을 사지 않는다).
- `/export` 마크다운.
- `cargo-packager`는 TUI 바이너리와 `devorch-ui` 동반 앱.

완료: 붙여넣기와 코드 펜스가 터미널에서 깨지지 않는다.

하지 않는 것:

- egui를 채팅 홈으로
- `devorch chat` REPL
- OpenCode/Tauri 셸 (옵션 C를 선택하기 전에는)
- 임베디드 터미널, 임베디드 에디터
- 네이티브 OS 제어, 브라우저 자동화
- 75 프로바이더
- Claude.app 비주얼 패리티를 DoD에 넣는 것

---

## 8. 파일 단위

만진다 (실행 단계가 시작되면):

```
crates/devorch-agent/src/adapter.rs     ExecuteRequest.resume, cancel
crates/devorch-agent/src/runner.rs      취소 전파
crates/devorch-agent-*                  build_spec resume 플래그
crates/devorch-process                  사용자 취소
crates/devorch-tui/                     신규
bins/devorch/src/main.rs                기본 진입 = TUI
crates/devorch-store                    session_meta 문서만
crates/devorch-ui/src/state.rs          기본 화면을 MissionControl에서 내림
docs/upstream-sources.toml              Codex TUI PORT 즉시
```

만지지 않는다:

```
crates/devorch-evaluate
crates/devorch-verify
crates/devorch-policy          # 호출만
crates/devorch-git
crates/devorch-mission         # /compare 가 호출만
crates/devorch-ui/src/graph3d.rs
crates/devorch-ui/src/chat.rs  # 만들지 않음
```

규칙 5 신호: `devorch-tui` 안에 워크트리 생성·점수·병합이 있거나, `devorch-ui`에 두 번째 컴포저가 생기면 잘못이다.

---

## 9. 품질 막대 — 시뮬레이터가 센 것

11개. 지금 1 (`PermissionRequired` 타입이 존재). 이전 S1로 3. TUI-first로 9. 남은 2개:

- 멀티턴 벤더 세션 — T0가 닫는다. 뷰가 아니다.
- 독점 데스크탑 비주얼 — 닫지 않는다. PLAN-7.

Codex TUI 0.148이 최근에 고친 것 (가져올 때 같이 가져온다):

- `/export` 마크다운
- resume가 cwd·정책·트랜스크립트 미리보기를 복구
- 컴포저 CRLF / 긴 URL / wrap
- 시작 시 버퍼된 입력이 프롬프트를 건드리지 않음

이것을 나흘짜리 egui 말풍선이 포함하지 못한 것이 초안 실패의 증거다.

---

## 10. 완료의 정의 (개정)

한 머신에서 관측:

```text
1. `devorch` 인자 없이 실행하면 TUI 채팅이 열린다. clap 도움말이 아니다.
2. 한 줄을 치면 설치된 CLI가 스트림으로 대답한다.
3. 두 번째 줄은 같은 벤더 세션을 잇는다 (resume id가 스펙에 있다).
4. Esc가 턴을 죽인다.
5. PermissionRequired 가 인라인 승인이다.
6. 재시작 후 세션 피커가 저널을 재생한다.
7. /compare 가 기존 미션 루프를 돌린다.
8. devorch-ui 에 컴포저가 없다.
9. PORT 줄은 upstream-sources.toml 에 커밋이 있다.
10. 게이트 네 개 + verify-agent-rules 가 세 OS에서 통과한다.
```

“창이 Claude처럼 보인다”는 이 목록에 없다.

---

## 11. 지금 하지 말 것

제품 코드를 이 문서대로 짜지 마라. 시뮬레이터가 초안을 개정하는 것이 이번 작업의 전부였다.

다음에 구현을 시작할 때:

1. 인테그레이터가 `ratatui` / `crossterm`를 워크스페이스에 넣는다.
2. T0 (`ExecuteRequest.resume` + 취소)를 T1보다 먼저.
3. Codex TUI PORT 전에 `upstream-sources.toml`에 커밋을 적는다.
4. `crates/devorch-ui/src/chat.rs`를 열지 않는다.

코어는 준비되어 있다. 부족한 것은 egui 말풍선이 아니라, `codex`와 `claude`가 하는 일 — **바이너리를 치면 대화가 시작되는 것** — 이다.
