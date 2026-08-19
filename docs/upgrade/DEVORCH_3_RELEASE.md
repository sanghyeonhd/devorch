# Devorch 3 — 릴리스 종합

**버전:** `3.0.0-alpha.1`
**작성일:** 2026-08-19
**저장소:** `github.com/sanghyeonhd/devorch`
**선행 문서:** `DEVORCH_3_RUST_MULTI_AGENT_ADE_IMPLEMENTATION.md`, `DEVORCH_3_RUST_AGENT_OPERATING_LAYER_IMPLEMENTATION.md`, `DEVORCH_3_DEVELOPMENT_SIMULATION_VALIDATION.md`, `DEVORCH_3_RELEASE_SCOPE_CHANGE.md`, `docs/rust-migration/R0-R2-REPORT.md`

이 문서는 범위 변경 문서가 선언한 목표 — Rust 전면 전환, 네이티브 GUI, Go 퇴역, 라이센스, GitHub 릴리스 — 가 실제로 어디에 착지했는지를 기록한다. 계획이 아니라 관측이다.

채팅 표면 계획(TUI가 홈, egui는 동반)은 `DEVORCH_3_CHAT_GUI_AND_REUSE.md`다. 시뮬레이터가 이전 4일 egui 초안을 기각했다. 제품 구현은 그 문서가 승격되기 전에는 시작하지 않는다.

---

## 1. 한 줄 결과

Devorch 3은 Go 런타임 없이, Rust 코어와 `egui/eframe` 데스크탑 앱으로 미션을 돌리고, 결정론적 게이트로 후보를 가려 승자를 병합한다. 실제 Grok CLI로 그 루프가 한 번 끝까지 통과했다.

라이센스는 Apache-2.0. Go 트리는 태그 `devorch-go-final`에 보존되어 있다.

---

## 2. 원래 계획과 달라진 것

원 계획은 R0–R7을 웨이브마다 게이트를 두고 네 에이전트가 워크트리에서 병렬로 작업하는 형태였다. `DEVORCH_3_RELEASE_SCOPE_CHANGE.md`가 이를 단일 실행 주체의 릴리스 스프린트로 바꿨다. 기술적 판단은 유지했고, 속도만 바꿨다.

유지된 판단:

- Rust 단일 언어, 네이티브 `egui/eframe` (Tauri 아님)
- 후보 평가에서 `git diff` 단독 사용 금지, 완전한 변경 인벤토리
- 유휴 에이전트 프로세스 0, `max_parallel_agents` 기본 2
- 구조화된 헤드리스 출력 우선
- 결정론적 프로그램 상태가 에이전트의 주장보다 항상 우선
- 네이티브 마우스/키보드 제어는 이번 릴리스에 없음

바뀐 절차:

| 원 계획 | 이번 스프린트 |
|---|---|
| 웨이브마다 교차 배정 + 게이트 | 단일 릴리스 게이트 |
| Go 동작마다 parity 테스트 후 삭제 | Rust가 자체 테스트로 증명, Go는 태그로 보존 후 삭제 |
| 어댑터 작성자 ≠ 검증자 | 녹화된 fixture CI + 옵트인 라이브 스모크 |

Go parity를 버린 이유는 R0 실측이다. 97개 패키지 중 테스트 파일은 3개였다. 검증되지 않은 동작에 대한 parity는 Go를 Rust로 옮겨 적고 옳다고 선언하는 일이며, 원 문서가 금지한 줄 단위 번역과 같은 결과다.

---

## 3. 커밋 지도

모두 `main`에 착지했다. 기준점은 `9ecdc71` (Go v0.1.0 트리).

| SHA | 범위 | 내용 |
|---|---|---|
| `bb3465a` | R0–R2 | Go 베이스라인 측정, Rust 워크스페이스, 설정·저장소·git·워크트리 수직 슬라이스 |
| `0857f5f` | 절차 | 단계적 이관 → 릴리스 스프린트 범위 변경 |
| `f93feba` | R3 | 프로세스 스트리밍, 네 벤더 어댑터, 정책·검증·평가 |
| `dc6e316` | R4–R5 | 미션 실행, 라우터, 스케줄러, 승자 병합, fake agent, git E2E |
| `d6be16b` | R6 | CLI `mission` 명령, 네이티브 Rust GUI |
| `f542940` | 결함 | 라이브 CLI가 드러낸 어댑터 3건 + 미션 결함 2건 |
| `5072695` | GUI | 3D 별자리, macOS `.app` 번들, 데모 시더 |
| `5920932` | R7 준비 | Go SQLite 스키마 22개를 `docs/rust-migration/go-schema/`로 보존 |
| `ffbb0ee` | R7 | Go 런타임 전면 제거 (`devorch-go-final` 태그) |
| `017d59e` | 라이센스 | Apache-2.0, NOTICE, THIRD_PARTY_NOTICES, README 재작성 |
| *(이 패치)* | CI | Windows `clippy::result_large_err` — `ConfigError::Parse`의 `toml::de::Error` 박싱 |

태그:

- `devorch-go-final` — 삭제 직전의 완전한 Go 트리
- 제품 버전 태그는 CI가 세 플랫폼에서 통과한 뒤에 붙인다 (`v3.0.0-alpha.1`)

---

## 4. 착지한 제품

```
                    DEVORCH 3
                       │
          ┌────────────┴────────────┐
          │                         │
       Rust Core                Rust GUI
       (CLI: devorch)           egui/eframe
          │                     (devorch-ui)
          └────────────┬────────────┘
                       │
                  Mission loop
                       │
         router → worktrees → adapters
                       │
              verify → evaluate → merge
                       │
                  re-verify merge
```

워크스페이스 크레이트 (19) + 바이너리 2 + xtask:

| 크레이트 | 역할 |
|---|---|
| `devorch-protocol` | 공유 타입: 에이전트, 이벤트, 변경 인벤토리, ID |
| `devorch-config` | `~/.devorch/config.toml`, 위험도 → 후보 수 |
| `devorch-store` | SQLite (`devorch3.db`), 저널, 워크스페이스 기록 |
| `devorch-events` | 이벤트 저널 |
| `devorch-process` | 플랫폼 중립 프로세스 실행·스트리밍 |
| `devorch-git` | 워크트리와 **완전한** 변경 인벤토리 |
| `devorch-workspace` | 워크트리 ↔ 영속 기록 |
| `devorch-agent` | 어댑터 계약, 런타임 능력 탐지, 러너 |
| `devorch-agent-{codex,claude,grok,gemini}` | 벤더별 정규화 |
| `devorch-agents` | 레지스트리 + 계약 적합성 테스트 |
| `devorch-verify` | 결정론적 게이트 (빌드/테스트/린트) |
| `devorch-policy` | 능력 기반 정책, 벤더 플래그는 입력이고 권한은 아님 |
| `devorch-evaluate` | 하드 게이트 탈락은 점수와 거래되지 않음 |
| `devorch-router` | 후보 수와 에이전트 선택, 이유 포함 |
| `devorch-mission` | 루프 전체 + fake agent + 데모 시더 |
| `devorch-ui` | 데스크탑: 미션, 에이전트 보드, 비교, 3D 별자리 |

측정 (이 패치 기준, `*.rs`):

- Rust 파일 41개, 약 12,400줄
- `#[test]` 162개 이상
- 에이전트 fixture 11개, 90줄 (라이브 캡처 포함)

Go는 저장소에 남아 있지 않다. `find . -name '*.go'`는 비어 있다.

---

## 5. 미션이 실제로 하는 일

```
goal
 └─ router picks how many candidates, and which
     └─ one git worktree per candidate, branched from the same base
         └─ each agent runs in its own worktree, streaming normalized events
             └─ deterministic gate: build, tests, lint
                 └─ complete change inventory, including untracked files
                     └─ compare → winner
                         └─ merge onto an integration branch
                             └─ run the gate again on the merged result
                                 └─ losing worktrees removed, records kept
```

세 가지 부하를 지는 성질, 각각 테스트가 있다.

1. **병합 결과를 다시 검증한다.** 혼자 통과한 후보가 통합 브랜치를 깨면 병합은 롤백되고 미션은 실패한다.
2. **진 워크트리는 지우고, 기록은 남긴다.** 증거는 저널에 있다.
3. **상태가 재시작을 견딘다.** 정리 전에 미션과 후보가 persist된다.

평가기는 에이전트의 성공 주장을 기록만 하고 점수에 넣지 않는다. `agent_claimed_success`는 비교 화면의 주석이지 승패가 아니다.

라우터의 기본값:

| 위험도 | 후보 수 | 동시 실행 (기본 cap 2) |
|---|---|---|
| low | 1 | 1 |
| normal | 2 | 2 |
| high / explicit-compare-all | 4 | 2 |

설치된 에이전트가 4개여도 유휴 프로세스는 0이다. 미션이 시작할 때만 켜고, 끝나면 끈다.

---

## 6. 어댑터 — 문서가 아니라 캡처

네 어댑터는 하나의 `AgentEvent` 계약으로 수렴한다. CI는 `testdata/agents/<vendor>/`의 녹화 스트림을 재생한다. 기본 CI는 구독·네트워크·쿼터에 의존하지 않는다.

### 6.1 라이브 캡처가 바꾼 것

문서 스키마로 짠 fixture는 세 곳에서 빗나갔다.

**Grok.** 문서에는 청크 텍스트가 `text` 필드라고 되어 있다. 실제 세션은 `{"type":"text","data":"ok"}`이고 payload는 그냥 문자열이다. 어댑터는 어시스턴트 텍스트를 하나도 못 뽑았다. 세션 id와 cost도 `end`에만 온다. 캡처: `testdata/agents/grok/live-simple.jsonl` (32줄).

**Codex.** 아이템 수준 에러가 실행 중에 나온다 (스킬 설명 단축 같은 무해한 것 포함). 그걸 터미널로 취급하면 아직 돌아가는 세션을 죽여 버린다. 지금은 스트림 수준 에러만 터미널이다. 사용량 한도 메시지는 protocol error가 아니라 rate limit으로 분류한다 — 하나는 나중에 재시도할 가치가 있고, 다른 하나는 없다. 실패 분류는 네 어댑터가 공유한다. 캡처: `testdata/agents/codex/live-usage-limit.jsonl`.

**Claude.** 어댑터 변경은 필요 없었다. 공개 스키마에 없는 이벤트 타입 두 개와 subtype 없는 result 라인이 있다. 그게 조용히 통과한다는 테스트가 있다. 캡처: `testdata/agents/claude/live-simple.jsonl`.

**Gemini.** 비대화형 세션에서 인증 프롬프트를 띄우고 멈춘다. 이 머신에서 라이브 캡처는 불가능했다. fixture는 문서 스키마 + 실패 스트림이다. 라이브 검증은 Gemini가 헤드리스로 인증된 환경의 후속 작업이다.

### 6.2 라이브 미션이 바꾼 것

실제 Grok가 파일을 올바르게 고쳤는데 미션이 실패했다. 세 가지 제품 결함:

1. **검증 명령이 없으면 에이전트를 돌리고 전부 거절했다.** 테스트 파일은 있는데 `pyproject.toml`이 없는 저장소. 지금은 테스트 존재로 Python 스위트를 인식하고, 검증할 것이 없으면 워크트리를 만들기 **전에** `MissionError::NoChecks`로 거절한다.
2. **CLI에 `--check`가 없었다.** 게이트를 명시할 수 있어야 한다.
3. **토큰 단위 delta가 진행 로그를 도배했다.** CLI와 GUI 모두 coalesce한다.

수정된 빌드로 실제 Grok CLI 미션이 통과했다: 실행, 검증, 병합, 병합 후 재검증. `main`은 건드리지 않았다.

---

## 7. GUI

`devorch-ui`는 코어 상태의 클라이언트다. 워크트리를 만들거나 후보를 채점하거나 병합을 결정하지 않는다. `devorch_mission`에 요청하고 답을 그린다.

화면 여섯 개: Mission Control (라이브 로그), Agent Board, Workspaces, Compare (증거 표 + 3D 별자리), Policy, Settings.

3D 별자리는 GPU 씬 그래프가 아니라 egui painter 위의 직교 투영이다. 깊이에 따라 크기·밝기가 변하고, 승자만 굵은 선으로 merge에 연결되며, 거절된 후보는 남는다 — 거절도 비교의 일부이기 때문이다. 깊이 정렬, pitch clamp, yaw wrap은 테스트된다.

macOS에서 맨 Mach-O는 Dock·메뉴바·앱 권한이 없는 faceless process다. `cargo xtask bundle-macos`가 `.app`을 만든다.

데이터 없이 보려면:

```bash
cargo run -p devorch-mission --example seed_demo -- /tmp/devorch-demo
DEVORCH_HOME=/tmp/devorch-demo cargo run --release -p devorch-ui-bin
```

시더는 스크립트 에이전트로 **진짜 미션**을 돌린다. UI가 보는 데이터는 라이브 미션과 같은 코드 경로의 산물이다.

---

## 8. Go 퇴역

1. 태그 `devorch-go-final`로 전체 Go 트리 보존
2. 22개 SQLite 마이그레이션을 `docs/rust-migration/go-schema/`로 복사 (`5920932`)
3. Go 소스 359파일 / 58,685줄, 두 바이너리, 정적 웹 UI, 셸 연동 스크립트 삭제 (`ffbb0ee`)

사용자의 `devorch.db`는 삭제하지 않는다. Rust는 `~/.devorch/devorch3.db`를 쓴다. OkAON 22개 테이블의 처분은 `docs/rust-migration/sql-schema-map.md`에 있고, 데이터 이관 실행은 이번 릴리스에 없다.

---

## 9. 라이센스

- 프로젝트: Apache-2.0 (`LICENSE`, `NOTICE`)
- 의존성: 전부 허용적. `cargo xtask third-party-notices`가 `THIRD_PARTY_NOTICES.md`를 생성하며, 이전에 쓰던 Python 스크립트와 동일 결과를 내도록 고정했다.
- 업스트림 포트: 없음. `docs/upstream-sources.toml`의 `sources = []`는 누락이 아니라 선언이다. 어댑터는 녹화한 CLI 출력에서 나왔고, 벤더 소스에서 나오지 않았다. Claude Code 저장소는 오픈소스 재배포 라이센스가 아니므로 이 구분이 특히 중요하다.

---

## 10. 크로스 플랫폼 CI

`.github/workflows/rust.yml`이 `ubuntu-latest`, `macos-latest`, `windows-latest`에서 릴리스 게이트를 돌린다.

`017d59e` 기준:

| Job | 결과 |
|---|---|
| gate (ubuntu-latest) | 성공 |
| gate (macos-latest) | 성공 |
| gate (windows-latest) | **실패** — clippy만. fmt / check / test는 통과 |

실패는 `clippy::result_large_err` 하나다. `ConfigError::Parse`가 `PathBuf`와 `toml::de::Error`를 인라인으로 들고 있고, Windows에서 가장 큰 variant가 128바이트 이상이다.

이 머신 (macOS, 64-bit)에서 측정한 크기:

| 타입 | 바이트 |
|---|---|
| `toml::de::Error` | 96 |
| `std::path::PathBuf` | 24 |
| `std::io::Error` | 8 |
| `rusqlite::Error` | 64 |
| `serde_json::Error` | 8 |

macOS에서 Parse는 임계값 바로 아래라 clippy가 침묵하고, Windows 레이아웃에서만 넘는다. 수정은 `toml::de::Error`를 `Box`로 감싸는 것이다. `const _: () = assert!(size_of::<ConfigError>() < 128)`가 모든 타깃에서 회귀를 막는다. `allow`가 아니다.

Windows·Linux에서 프로세스를 “실제로 화를 내며” 돌린 적은 아직 없다. 프로세스 레이어는 플랫폼 중립으로 짜여 있고 CI가 빌드·테스트를 돌리지만, 라이브 에이전트 미션은 macOS에서만 관측했다. README가 이 한계를 이미 적고 있다.

---

## 11. 이번 릴리스에 없는 것

범위 변경 문서 §4와 같다. 발견에 맡기지 않는다.

- 네이티브 OS 제어 (마우스, 키보드, 스크린 캡처). 정책 엔진은 거부하고 런타임이 없다.
- 브라우저 자동화
- 자율 운영. 미션은 사람이 시작한다.
- OkAON 학습 데이터의 Go → Rust 이관 실행
- Gemini 라이브 캡처 (비대화형 인증 프롬프트)
- Windows/Linux 라이브 에이전트 미션

원 문서 §53의 순서가 맞다. 정책과 검증이 들어왔으니 다음 릴리스에서 네이티브 제어를 얹을 수 있는 상태다. 지금 얹으면 “즉시 거부” 패치다.

---

## 12. 쓰는 법

```bash
git clone https://github.com/sanghyeonhd/devorch
cd devorch
cargo build --release

devorch agent inspect
devorch mission run --repo . --goal "Fix the failing authentication test" --risk high
devorch mission run --repo . --goal "..." --dry-run
devorch mission run --repo . --goal "..." --check "cargo test --workspace"
devorch mission list
devorch mission show <id>

cargo run --release -p devorch-ui-bin          # 데스크탑
cargo xtask bundle-macos                       # macOS .app
```

게이트:

```bash
cargo fmt --all -- --check
cargo check --workspace
cargo test --workspace
cargo clippy --workspace --all-targets -- -D warnings
cargo xtask verify-agent-rules
```

라이브 스모크는 옵트인이다. 기본 테스트는 키와 구독이 필요 없다.

---

## 13. 위험 — 다루지 않은 것

- **Grok 스키마가 또 바뀔 수 있다.** 라이브 fixture가 현재 출력을 고정하지만, 벤더 CLI는 통지 없이 필드를 옮긴다. 능력 탐지(help 텍스트)가 플래그 이름에는 대응하고, 이벤트 스키마에는 대응하지 않는다.
- **Gemini 헤드리스 인증.** 어댑터는 compile되고 fixture 계약을 통과한다. 실제 Gemini 미션은 이 스프린트에서 돌리지 못했다.
- **병합 전략은 한 승자의 fast-forward/merge다.** 부분 채택(이 파일은 A, 저 파일은 B)은 없다.
- **OkAON이 비어 있다.** 라우터의 `success_rate`는 `None`이면 0으로 취급하지 않고, 부트스트랩 위험도 정책을 쓴다. 학습 루프는 다음 작업이다.
- **Windows PATH/홈.** `USERPROFILE`은 이미 본다. 실제 Windows에서 에이전트 CLI를 띄워 본 적은 없다.
- **앱 서명·공증.** macOS 번들은 만들어지지만 Apple 서명과 공증은 이 릴리스에 없다. Gatekeeper가 로컬 빌드를 막을 수 있다.

---

## 14. 이 문서가 닫는 것, 닫지 않는 것

닫는다: R0 측정, R1 워크스페이스, R2 수직 슬라이스, R3 어댑터·정책·검증·평가, R4–R5 미션 루프, R6 GUI, R7 Go 퇴역, 라이센스, Windows clippy 게이트.

닫지 않는다: GitHub Release 자산 업로드 (CI 통과 후), OkAON 이관, 네이티브 제어, Gemini 라이브, Windows/Linux 현장 미션.

원 문서의 “즉시 거부” 규칙 — 실패 숨기기, 테스트 삭제, 관측하지 않은 빌드 성공 보고, 웨이브를 앞지른 네이티브 제어 — 은 그대로다.
