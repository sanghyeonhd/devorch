좋습니다. 이전에 만들었던 Devorch 트리(daemon/core + CLI + IDE(WebView/VSCode) + provider/agent/hook/tool 구조)를 **그대로 유지**하면서, 방금 합의한

* **OkAON(로컬 이력/벤치/성공률 저장)**
* **OS/아키텍처별 로컬 LLM 자동 설치**
* **로컬+원격(API) 혼합 라우팅**
* **작업 완료 시 자동 벤치 누적**

이 4가지를 **누락 없이** 반영한 “업그레이드 소스 트리”를 아래에 제시합니다. (설명은 사용성 중심으로 짧게 붙였습니다)

---

# Devorch 업그레이드 전체 소스 트리 (OkAON + Auto Local LLM + Hybrid Router 포함)

```
devorch/
├─ README.md
├─ LICENSE
├─ Makefile
├─ go.mod
├─ go.sum
├─ .editorconfig
├─ .gitignore
├─ .gitattributes
├─ .env.example
├─ configs/
│  ├─ devorch.default.yaml                 # 기본 동작 정책(라우팅/벤치/저장소/보안)
│  ├─ devorch.schema.json                  # 설정 스키마(검증용)
│  ├─ routing.presets.yaml                 # 카테고리/태스크별 모델 후보 + 가중치 프리셋
│  ├─ install.presets.yaml                 # OS/아키텍처별 로컬 LLM 설치 후보 목록
│  ├─ providers.yaml                       # OpenRouter/로컬/OAuth 프로바이더 정의
│  ├─ oauth.yaml                           # OAuth 제공자 설정(구글/깃허브/애플 등)
│  └─ privacy.yaml                         # 텔레메트리/로그/마스킹 정책
│
├─ cmd/
│  ├─ devorch/
│  │  ├─ main.go                           # CLI 엔트리(서브커맨드 라우팅)
│  │  └─ commands/
│  │     ├─ serve.go                       # 데몬 실행(로컬 API + 이벤트 스트림)
│  │     ├─ run.go                         # 단일 태스크 실행(프로젝트/파일 지정 가능)
│  │     ├─ doctor.go                      # 환경 진단(OS/권한/의존성/로컬LLM 상태)
│  │     ├─ bench.go                       # 수동 벤치 실행(quick/extended/workload)
│  │     ├─ models.go                      # 설치된 로컬 모델 목록/상태
│  │     ├─ install.go                     # 로컬LLM 런타임+모델 자동 설치/제거
│  │     ├─ providers.go                   # API 프로바이더 상태/키 설정/OAuth 연결
│  │     ├─ okAON.go                       # OkAON 이력 조회(최근 태스크/성공률/성능)
│  │     ├─ project.go                     # 프로젝트 등록/스캔/워크트리 관리
│  │     └─ config.go                      # 설정 출력/검증/머지(우선순위)
│  │
│  └─ devorchd/
│     └─ main.go                           # 데몬 전용 바이너리(백그라운드 상주)
│
├─ internal/
│  ├─ app/
│  │  ├─ bootstrap.go                      # 설정 로딩→스토리지→라우터→서버 결선
│  │  ├─ lifecycle.go                      # 시작/종료/업데이트 훅
│  │  └─ version.go
│  │
│  ├─ api/
│  │  ├─ http/
│  │  │  ├─ server.go                      # 로컬 HTTP API(IDE/CLI 공용)
│  │  │  ├─ routes/
│  │  │  │  ├─ health.go                   # 상태/헬스/버전
│  │  │  │  ├─ task.go                     # 태스크 생성/조회/중단
│  │  │  │  ├─ session.go                  # 세션/컨텍스트 관리
│  │  │  │  ├─ router.go                   # 라우팅 결과/스코어/폴백 로그 조회
│  │  │  │  ├─ models.go                   # 로컬 모델 설치/상태/용량
│  │  │  │  ├─ bench.go                    # 벤치 실행/결과 조회
│  │  │  │  ├─ okAON.go                    # OkAON 통계/이력/리포트
│  │  │  │  ├─ oauth.go                    # OAuth 연결/토큰 상태
│  │  │  │  └─ admin.go                    # (옵션) 정책/차단/레이트 제한
│  │  │  └─ middleware/
│  │  │     ├─ auth.go                     # 로컬 접근 제어(토큰/비번/OS키체인)
│  │  │     ├─ rate_limit.go
│  │  │     ├─ request_id.go
│  │  │     └─ audit.go                    # 민감 작업 감사로그
│  │  │
│  │  └─ stream/
│  │     ├─ sse.go                         # IDE 실시간 스트림(SSE)
│  │     └─ events.go                      # 표준 이벤트(TaskStarted/ModelSwitched/BenchSaved…)
│  │
│  ├─ core/
│  │  ├─ task/
│  │  │  ├─ spec.go                        # TaskSpec 표준(카테고리/제약/게이트)
│  │  │  ├─ planner.go                     # (선택) 태스크 분해/단계 구성
│  │  │  ├─ pipeline.go                    # plan→implement→qa→review 동적 파이프라인
│  │  │  ├─ executor.go                    # 단계 실행(모델/툴/권한)
│  │  │  ├─ artifacts.go                   # 결과물(패치/파일/리포트) 모델
│  │  │  └─ history.go                     # 태스크 히스토리(OkAON 연동)
│  │  │
│  │  ├─ router/
│  │  │  ├─ router.go                      # 최종 모델/프로바이더 선택 + 폴백
│  │  │  ├─ classify.go                    # cheap 분류기(category/difficulty/vision/risk)
│  │  │  ├─ score/
│  │  │  │  ├─ score.go                    # 실측 기반 스코어링(성공률/시간/비용/안정성)
│  │  │  │  ├─ weights.go                  # 프리셋 가중치(quick/qa/refactor…)
│  │  │  │  ├─ constraints.go              # 금지 모델/비용 상한/로컬 우선 등
│  │  │  │  └─ health.go                   # 최근 실패율/지연으로 감점(자동 회피)
│  │  │  ├─ fallback.go                    # 3단계 fallback + 실시간 health 전환
│  │  │  ├─ hybrid.go                      # 로컬+원격 혼합 정책(오프로딩 규칙)
│  │  │  └─ explain.go                     # “왜 이 모델을 골랐는지” 설명 생성(UX용)
│  │  │
│  │  ├─ quality/
│  │  │  ├─ gates.go                       # quality gate 정의(테스트/린트/빌드)
│  │  │  ├─ runners/
│  │  │  │  ├─ go_test.go
│  │  │  │  ├─ node_test.go
│  │  │  │  ├─ java_gradle.go
│  │  │  │  ├─ lint.go
│  │  │  │  └─ build.go
│  │  │  ├─ normalize.go                   # 언어/툴별 결과 표준화
│  │  │  └─ retry_policy.go                # “같은 모델 2회→다른 모델 전환” 같은 규칙
│  │  │
│  │  ├─ bench/
│  │  │  ├─ bench.go                       # 벤치 실행 엔트리(quick/extended/workload)
│  │  │  ├─ machine/
│  │  │  │  ├─ detect.go                   # OS/arch/cpu/ram/disk/gpu 탐지
│  │  │  │  ├─ diskio.go                   # 모델 로딩 병목용 디스크 벤치
│  │  │  │  └─ report.go                   # 머신 벤치 리포트
│  │  │  ├─ model/
│  │  │  │  ├─ warmup.go                   # cold/warm 측정
│  │  │  │  ├─ ttft.go                     # TTFT 측정
│  │  │  │  ├─ tps.go                      # TPS 측정
│  │  │  │  ├─ ctxscale.go                 # 4k/16k/32k 스케일링
│  │  │  │  └─ stability.go                # timeout/ratelimit/oom 측정
│  │  │  ├─ workload/
│  │  │  │  ├─ scenarios.go                # “내 개발 워크로드” 시나리오 정의
│  │  │  │  ├─ repo_scan.go                # repo scan + 요약
│  │  │  │  ├─ bugfix.go                   # bugfix + 테스트 통과
│  │  │  │  ├─ refactor.go                 # 리팩토링 + 린트
│  │  │  │  └─ feature.go                  # 기능 추가 + 통합 테스트
│  │  │  └─ recorder.go                    # 결과를 OkAON으로 저장
│  │  │
│  │  ├─ okAON/
│  │  │  ├─ store.go                       # OkAON 저장소 인터페이스(SQLite/파일)
│  │  │  ├─ sqlite/
│  │  │  │  ├─ db.go                       # DB 연결/pragma/성능 설정
│  │  │  │  ├─ schema.sql                  # TaskRun/ModelRun/EnvSnapshot 테이블
│  │  │  │  ├─ migrate.go                  # 마이그레이션
│  │  │  │  ├─ insert.go                   # 실행 이력 저장
│  │  │  │  ├─ query.go                    # 통계/최근 실행/모델별 성과
│  │  │  │  └─ vacuum.go                   # 용량 최적화/정리
│  │  │  ├─ files/
│  │  │  │  ├─ jsonl.go                    # 경량 모드(JSONL) 저장(옵션)
│  │  │  │  └─ index.go                    # 인덱싱(옵션)
│  │  │  ├─ models.go                      # PerfProfile/TaskRun/ModelRun 구조체
│  │  │  └─ retention.go                   # TTL/정리 정책(30일/90일 등)
│  │  │
│  │  ├─ modelmgr/
│  │  │  ├─ manager.go                     # “자동 설치/업데이트/삭제/용량관리” 총괄
│  │  │  ├─ catalog/
│  │  │  │  ├─ catalog.go                  # OS/arch별 후보 모델 정의
│  │  │  │  ├─ presets.go                  # 저사양/중간/고사양 프리셋
│  │  │  │  └─ policy.go                   # 설치 정책(가용RAM/디스크 기준)
│  │  │  ├─ runtime/
│  │  │  │  ├─ runtime.go                  # 로컬 런타임 추상화(Ollama/llama.cpp/기타)
│  │  │  │  ├─ ollama/
│  │  │  │  │  ├─ install.go               # Ollama 설치(OS별)
│  │  │  │  │  ├─ pull.go                  # 모델 pull + 진행률
│  │  │  │  │  ├─ serve.go                 # 로컬 서버 관리
│  │  │  │  │  └─ health.go
│  │  │  │  ├─ llamacpp/
│  │  │  │  │  ├─ install.go               # llama.cpp 바이너리 설치/관리
│  │  │  │  │  ├─ model.go                 # gguf 관리
│  │  │  │  │  └─ health.go
│  │  │  │  └─ shared/
│  │  │  │     ├─ download.go              # 다운로드/재시도/미러
│  │  │  │     ├─ checksum.go              # sha 검증
│  │  │  │     └─ paths.go                 # OS별 경로(XDG/AppData/Library)
│  │  │  ├─ storage/
│  │  │  │  ├─ quota.go                    # 디스크 용량/캐시 제한
│  │  │  │  ├─ evict.go                    # LRU/TTL 모델 제거
│  │  │  │  └─ usage.go                    # 모델별 용량 계산
│  │  │  └─ warmup/
│  │  │     ├─ warmup.go                   # 설치 후 워밍업/마이크로벤치
│  │  │     └─ prompts.go
│  │  │
│  │  ├─ providers/
│  │  │  ├─ provider.go                    # 공통 Provider 인터페이스
│  │  │  ├─ local/
│  │  │  │  ├─ local.go                    # 로컬 모델 호출 공통(런타임 선택)
│  │  │  │  └─ tools.go                    # 로컬에서 가능한 툴 제약
│  │  │  ├─ openrouter/
│  │  │  │  ├─ client.go                   # OpenRouter API 클라이언트
│  │  │  │  ├─ models.go                   # 모델 목록 캐시/health
│  │  │  │  └─ limits.go                   # free/paid 정책
│  │  │  ├─ openai_compat/
│  │  │  │  ├─ client.go                   # OpenAI 호환 엔드포인트 일반화
│  │  │  │  └─ models.go
│  │  │  └─ oauth/
│  │  │     ├─ oauth.go                    # OAuth 토큰 취득/갱신/저장
│  │  │     ├─ providers/
│  │  │     │  ├─ github.go
│  │  │     │  ├─ google.go
│  │  │     │  ├─ apple.go
│  │  │     │  └─ custom.go
│  │  │     └─ keychain/
│  │  │        ├─ darwin.go                # macOS Keychain
│  │  │        ├─ windows.go               # Windows Credential Manager
│  │  │        └─ linux.go                 # Secret Service / fallback file
│  │  │
│  │  ├─ agents/
│  │  │  ├─ registry.go                    # 에이전트 등록/권한/모델 요구사항
│  │  │  ├─ orchestrator/
│  │  │  │  ├─ sisyphus.go                 # 메인 오케스트레이터(태스크 분해/위임)
│  │  │  │  ├─ atlas.go                    # 실행 오케스트레이터(단계 실행)
│  │  │  │  └─ prompts/                    # 기본 프롬프트 템플릿
│  │  │  ├─ specialists/
│  │  │  │  ├─ oracle.go                   # 아키텍처/리뷰/디버그
│  │  │  │  ├─ explore.go                  # 빠른 코드 탐색
│  │  │  │  ├─ librarian.go                # 문서/코드 검색
│  │  │  │  ├─ qa.go                       # 테스트/검증
│  │  │  │  └─ uiux.go                     # UI/UX 보조
│  │  │  └─ routing/
│  │  │     ├─ category.go                 # 카테고리 정의(quick/ultrabrain/visual/qa…)
│  │  │     ├─ requirements.go             # 에이전트/카테고리 요구사항(툴/컨텍스트)
│  │  │     └─ fallback.go                 # 에이전트별 폴백 체인
│  │  │
│  │  ├─ hooks/
│  │  │  ├─ registry.go                    # 훅 등록/순서/활성화
│  │  │  ├─ pre_tool_use.go                # 실행 전 검증/권한/정책 적용
│  │  │  ├─ post_tool_use.go               # 출력 정리/로그/벤치 기록 트리거
│  │  │  ├─ on_task_complete.go            # ✅ 작업 완료 시 OkAON 기록(핵심)
│  │  │  └─ on_model_switch.go             # 모델 스위칭 이벤트 기록
│  │  │
│  │  ├─ tools/
│  │  │  ├─ registry.go
│  │  │  ├─ fs/
│  │  │  │  ├─ read.go
│  │  │  │  ├─ write.go
│  │  │  │  ├─ edit.go
│  │  │  │  └─ patch.go
│  │  │  ├─ vcs/
│  │  │  │  ├─ git.go
│  │  │  │  └─ worktree.go
│  │  │  ├─ lsp/
│  │  │  │  ├─ client.go
│  │  │  │  └─ actions.go
│  │  │  ├─ bench/
│  │  │  │  └─ record.go                   # 태스크 중/후 측정값 수집
│  │  │  ├─ delegate/
│  │  │  │  ├─ delegate_task.go            # subagent/category 라우팅 실행
│  │  │  │  └─ background.go               # 백그라운드 태스크
│  │  │  └─ net/
│  │  │     └─ webfetch.go
│  │  │
│  │  ├─ session/
│  │  │  ├─ session.go                     # 세션/컨텍스트/압축
│  │  │  ├─ compaction.go                  # 긴 세션 요약(로컬→원격 오프로딩 가능)
│  │  │  └─ store.go
│  │  │
│  │  └─ util/
│  │     ├─ log/
│  │     │  ├─ log.go
│  │     │  └─ redact.go                   # 민감정보 마스킹
│  │     ├─ osinfo/
│  │     │  ├─ darwin.go
│  │     │  ├─ windows.go
│  │     │  └─ linux.go
│  │     ├─ paths/
│  │     │  ├─ xdg.go
│  │     │  ├─ appdata.go
│  │     │  └─ macos.go
│  │     ├─ jsonc/
│  │     └─ concurrency/
│  │        ├─ pool.go
│  │        └─ limiter.go
│  │
│  ├─ platform/
│  │  ├─ darwin/
│  │  │  ├─ install.go                      # mac Intel/Apple Silicon 분기 포함
│  │  │  └─ permissions.go
│  │  ├─ windows/
│  │  │  ├─ install.go                      # 서비스/작업스케줄러/권한
│  │  │  └─ permissions.go
│  │  └─ linux/
│  │     ├─ install.go                      # systemd 유닛/권한
│  │     └─ permissions.go
│  │
│  └─ ui/
│     ├─ vscode/
│     │  ├─ extension/
│     │  │  ├─ package.json                # VSCode 확장 메타
│     │  │  ├─ src/
│     │  │  │  ├─ extension.ts             # 확장 엔트리(daemon 연결)
│     │  │  │  ├─ commands.ts              # Run/Bench/Install/Explain Router
│     │  │  │  ├─ client.ts                # SSE/HTTP 연결
│     │  │  │  ├─ panels/
│     │  │  │  │  ├─ chat.ts               # 채팅/에이전트 UI
│     │  │  │  │  ├─ perf.ts               # ✅ “내 PC 실측 성능” 뷰
│     │  │  │  │  ├─ models.ts             # 설치된 로컬 모델 관리
│     │  │  │  │  └─ runs.ts               # OkAON 실행 이력
│     │  │  │  └─ security.ts              # 권한/감사로그 UI
│     │  │  └─ media/
│     │  │     └─ webview.bundle.js
│     │  └─ README.md
│     │
│     └─ webview/
│        ├─ app/                            # 웹뷰 UI(선택: Tauri/Electron 가능)
│        │  ├─ src/
│        │  │  ├─ pages/
│        │  │  │  ├─ Home.tsx
│        │  │  │  ├─ Session.tsx
│        │  │  │  ├─ Perf.tsx              # ✅ 실측 벤치/그래프/재측정
│        │  │  │  ├─ Models.tsx
│        │  │  │  └─ History.tsx           # OkAON 이력
│        │  │  ├─ components/
│        │  │  └─ api/
│        │  └─ package.json
│        └─ README.md
│
├─ assets/
│  ├─ prompts/
│  ├─ schemas/
│  └─ icons/
│
├─ scripts/
│  ├─ build_all.sh
│  ├─ release.sh
│  ├─ gen_schema.go
│  └─ smoke_test.sh
│
├─ docs/
│  ├─ architecture.md
│  ├─ routing.md
│  ├─ bench.md
│  ├─ okAON.md
│  ├─ security.md
│  ├─ install.md
│  └─ troubleshooting.md
│
└─ test/
   ├─ e2e/
   ├─ integration/
   └─ fixtures/
```

---

## 사용성 관점에서 “추가된 것”이 사용자에게 어떻게 보이냐

* 사용자는 그냥 **“작업 실행”**만 누름
* Devorch는 내부적으로:

  * 로컬 모델이 없으면 **OS별로 알아서 설치**
  * 작업 중 로컬이 버겁거나 실패가 반복되면 **원격 API로 일부 오프로딩**
  * 작업이 끝나면 **OkAON에 성능+성공률이 자동 저장**
  * 다음부터는 같은 유형 작업에서 **더 좋은 모델을 자동 선택**

---

## 빠짐없이 반영된 핵심 포인트 체크

* ✅ 작업 완료 기준 + 벤치 기록을 OkAON에 저장 (`core/bench/recorder.go`, `core/okAON/…`, `hooks/on_task_complete.go`)
* ✅ OS별 자동 설치/다운로드 (`core/modelmgr/runtime/*`, `internal/platform/*`)
* ✅ 로컬+원격 혼합 라우팅 (`core/router/hybrid.go`)
* ✅ 모델 선택이 실측에 의해 점점 개선 (`router/score/*` + OkAON 통계 기반)
* ✅ VSCode/WebView에서 성능/이력/모델관리 UI 제공 (`ui/vscode/panels/perf.ts`, `History.tsx` 등)

---

원래 트리에서 “이름/경로”만 당신이 이미 정해둔 게 있다면(예: `okAON`의 실제 명칭, DB 위치 규칙, daemon 포트/프로토콜), 그건 그대로 맞춰서 경로만 리네이밍하면 됩니다.
