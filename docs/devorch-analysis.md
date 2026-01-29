# DevOrch 프로젝트 완전 분석서

**작성일**: 2026-01-30  
**브랜치**: main  
**버전**: v1.0.0

---

## 1. 프로젝트 개요

DevOrch는 Go 기반의 AI 오케스트레이션 데몬/툴킷입니다. 다수의 LLM 프로바이더를 통합하고, 유연한 내장 도구 시스템, TUI/WebUI, LSP/진단 도구, 세션 기록, SQLite 스토리지, 플랫폼/하드웨어 감지, 국제화(i18n)를 제공합니다.

### 1.1 핵심 특징
- **멀티 프로바이더**: OpenAI, Anthropic, Google, Azure, AWS Bedrock, Ollama 등 17개 LLM 프로바이더 지원
- **내장 도구 시스템**: 파일 편집, 검색, 웹 검색, 코드 분석, 배치 작업, LSP 등 25개 이상의 도구
- **스마트 라우팅**: Thompson Sampling 기반 밴딧 알고리즘으로 최적의 모델 자동 선택
- **듀얼 인터페이스**: TUI (Terminal UI) + Web UI 동시 지원
- **ACP WebSocket**: IDE 통합을 위한 양방향 WebSocket 통신 지원
- **PTY 터미널**: 네이티브 PTY 터미널 에뮬레이터 내장 (creack/pty)
- **테마 시스템**: 31개의 사전 정의된 테마 (Catppuccin, Kanagawa, Rosé Pine 등)
- **로컬 모델**: Ollama 번들링 및 자동 시작 지원
- **국제화**: 영어, 한국어, 일본어, 중국어 지원

---

## 2. 프로젝트 구조 (전체 트리)

```
devorch/
├── .github/
│   └── workflows/
│       └── devorch.yml                    # CI/CD 워크플로우
├── bin/                                   # 빌드된 바이너리
│   ├── devorch                            # CLI 바이너리 (현재 OS)
│   ├── devorch-darwin-amd64               # macOS Intel
│   ├── devorch-darwin-arm64               # macOS Apple Silicon
│   ├── devorch-linux-amd64                # Linux x64
│   ├── devorch-linux-arm64                # Linux ARM64
│   ├── devorch-windows-amd64.exe          # Windows x64
│   ├── devorch-windows-arm64.exe          # Windows ARM64
│   ├── devorchd                           # 데몬 바이너리 (현재 OS)
│   ├── devorchd-darwin-amd64
│   ├── devorchd-darwin-arm64
│   ├── devorchd-linux-amd64
│   ├── devorchd-linux-arm64
│   ├── devorchd-windows-amd64.exe
│   └── devorchd-windows-arm64.exe
├── cmd/
│   ├── devorch/
│   │   └── main.go                        # CLI 엔트리포인트
│   └── devorchd/
│       ├── static/                        # 정적 파일
│       └── main.go                        # 데몬 엔트리포인트
├── docs/                                  # 문서
│   ├── compare/                           # 비교 분석 문서
│   ├── devorch-analysis.md                # 본 문서
│   └── phase*.md                          # 개발 단계별 문서
├── internal/                              # 핵심 내부 패키지
│   ├── agent/                             # 에이전트 오케스트레이션
│   │   ├── agent.go                       # Agent 정의 및 구성
│   │   ├── builtin.go                     # 내장 에이전트
│   │   ├── orchestrator.go                # 멀티에이전트 오케스트레이터
│   │   └── registry.go                    # 에이전트 레지스트리
│   ├── app/                               # 애플리케이션 부트스트랩
│   │   ├── app.go                         # Deps 구조체 정의
│   │   ├── wire.go                        # 의존성 주입 (WireMinimal)
│   │   ├── step11_wiring.go               # 추가 와이어링
│   │   └── wire_oauth_config.go           # OAuth 설정
│   ├── attribution/                       # 기여 분석
│   │   ├── analyzer.go                    # 분석 로직
│   │   ├── envsig.go                      # 환경 시그니처
│   │   ├── store.go                       # 저장소
│   │   └── types.go                       # 타입 정의
│   ├── auth/                              # 인증
│   │   ├── device_flow.go                 # OAuth Device Flow
│   │   ├── manager_workspace.go           # 워크스페이스 매니저
│   │   ├── oidc_discovery.go              # OIDC 디스커버리
│   │   ├── pkce.go                        # PKCE 구현
│   │   ├── providers.go                   # 인증 프로바이더
│   │   ├── providers_build.go             # 빌드 시 프로바이더
│   │   ├── refresh.go                     # 토큰 갱신
│   │   ├── state.go                       # 상태 관리
│   │   ├── store_sqlite.go                # SQLite 저장소
│   │   ├── store.go                       # 저장소 인터페이스
│   │   └── types.go                       # 타입 정의
│   ├── authz/                             # 권한 부여
│   │   ├── authorizer_sqlite.go           # SQLite 권한 부여자
│   │   ├── authz.go                       # 권한 로직
│   │   └── errors.go                      # 에러 정의
│   ├── autosetup/                         # 자동 설정
│   │   └── autosetup.go                   # 환경 자동 구성
│   ├── background/                        # 백그라운드 작업
│   │   ├── congestion.go                  # 혼잡 제어
│   │   ├── feedback.go                    # 피드백 처리
│   │   ├── manager.go                     # 작업 매니저
│   │   ├── task_manager.go                # 태스크 매니저
│   │   ├── task_store.go                  # 태스크 저장소
│   │   └── types_task.go                  # 태스크 타입
│   ├── bench/                             # 벤치마크
│   │   ├── eval/                          # 평가 도구
│   │   ├── bench.go                       # 벤치마크 코어
│   │   ├── cost_model.go                  # 비용 모델
│   │   ├── quality_eval.go                # 품질 평가
│   │   ├── recorder.go                    # 기록기
│   │   ├── reward_model.go                # 보상 모델
│   │   └── runner.go                      # 실행기
│   ├── billing/                           # 과금
│   │   └── pricing_table.go               # 가격 테이블
│   ├── bus/                               # 이벤트 버스
│   │   ├── hub.go                         # 이벤트 허브
│   │   └── sse.go                         # Server-Sent Events
│   ├── cli/                               # CLI
│   │   └── login.go                       # 로그인 명령
│   ├── command/                           # 명령 처리
│   │   └── command.go                     # 명령 정의
│   ├── compaction/                        # 컨텍스트 압축
│   │   └── compaction.go                  # 세션 요약/토큰 절감
│   ├── config/                            # 설정
│   │   ├── config.go                      # 설정 구조체
│   │   ├── jsonc.go                       # JSONC 파서
│   │   ├── load.go                        # 설정 로드
│   │   ├── load_oauth.go                  # OAuth 설정 로드
│   │   └── oauth.go                       # OAuth 설정
│   ├── delegate/                          # 위임 처리
│   │   ├── delegate.go                    # 위임 로직
│   │   ├── router.go                      # 위임 라우터
│   │   └── types.go                       # 타입 정의
│   ├── diagnostics/                       # 진단
│   │   ├── checks/                        # 진단 체크
│   │   ├── doctor.go                      # 시스템 진단
│   │   ├── report.go                      # 진단 리포트
│   │   └── run_doctor.go                  # 진단 실행
│   ├── exec/                              # 실행
│   │   └── runner.go                      # 명령 실행기
│   ├── experiment/                        # 실험
│   │   ├── assignment.go                  # 실험 할당
│   │   ├── evaluator.go                   # 평가기
│   │   ├── guard.go                       # 실험 가드
│   │   ├── hashing.go                     # 해싱
│   │   ├── selector.go                    # 선택기
│   │   ├── service.go                     # 실험 서비스
│   │   ├── store.go                       # 저장소
│   │   └── types.go                       # 타입 정의
│   ├── extension/                         # 확장
│   │   ├── doc.go                         # 문서
│   │   ├── loader.go                      # 확장 로더
│   │   └── loader_test.go                 # 테스트
│   ├── global/                            # 전역 설정
│   │   ├── env_fingerprint.go             # 환경 핑거프린트
│   │   ├── fingerprint.go                 # 핑거프린트 생성
│   │   ├── paths.go                       # 전역 경로
│   │   └── repo_fingerprint.go            # 저장소 핑거프린트
│   ├── history/                           # 히스토리
│   │   └── history.go                     # 변경 기록/백업/되돌리기
│   ├── hook/                              # 훅 시스템
│   │   ├── builtins/                      # 내장 훅
│   │   ├── chain.go                       # 훅 체인
│   │   ├── events.go                      # 이벤트 정의
│   │   ├── hook.go                        # 훅 인터페이스
│   │   └── registry.go                    # 훅 레지스트리
│   ├── i18n/                              # 국제화
│   │   ├── i18n.go                        # 번역기 코어
│   │   ├── translations_en.go             # 영어 번역
│   │   ├── translations_ja.go             # 일본어 번역
│   │   ├── translations_ko.go             # 한국어 번역
│   │   └── translations_zh.go             # 중국어 번역
│   ├── id/                                # ID 생성
│   │   └── ulid.go                        # ULID 생성기
│   ├── learning/                          # 학습
│   │   ├── features/                      # 특징 추출
│   │   ├── http/                          # HTTP 학습
│   │   ├── feedback.go                    # 피드백 처리
│   │   ├── metrics.go                     # 메트릭
│   │   └── recorder.go                    # 기록기
│   ├── log/                               # 로깅
│   │   └── log.go                         # 로거
│   ├── lsp/                               # Language Server Protocol
│   │   ├── client.go                      # LSP 클라이언트
│   │   ├── lsp.go                         # LSP 코어 (빌드 제외)
│   │   ├── manager.go                     # LSP 매니저
│   │   ├── operations.go                  # LSP 오퍼레이션
│   │   └── types.go                       # LSP 타입
│   ├── mcp/                               # Model Context Protocol
│   │   ├── client.go                      # MCP 클라이언트
│   │   ├── manager.go                     # MCP 매니저
│   │   ├── transport.go                   # MCP 전송
│   │   └── types.go                       # MCP 타입
│   ├── memory/                            # 메모리/컨텍스트
│   │   └── memory.go                      # DEVORCH.md 관리
│   ├── modelresolver/                     # 모델 해석기
│   │   ├── config_types.go                # 설정 타입
│   │   ├── fallback.go                    # 폴백 로직
│   │   ├── policy.go                      # 정책
│   │   ├── requirements.go                # 요구사항
│   │   ├── resolve.go                     # 해석 로직
│   │   ├── resolve_agent.go               # 에이전트 해석
│   │   └── types.go                       # 타입 정의
│   ├── okaon/                             # OkAON 통계
│   │   ├── sqlite/                        # SQLite 저장소
│   │   ├── aggregates.go                  # 통계 집계
│   │   ├── models.go                      # 데이터 모델
│   │   ├── retention.go                   # 데이터 보존
│   │   └── store.go                       # 저장소
│   ├── permission/                        # 권한
│   │   └── permission.go                  # 도구/경로 권한 규칙
│   ├── platform/                          # 플랫폼
│   │   └── detect/                        # 플랫폼 감지
│   ├── plugin/                            # 플러그인
│   │   ├── builtins.go                    # 내장 플러그인
│   │   ├── manager.go                     # 플러그인 매니저
│   │   └── plugin.go                      # 플러그인 인터페이스
│   ├── provider/                          # LLM 프로바이더
│   │   ├── anthropic/                     # Anthropic Claude
│   │   │   ├── anthropic.go               # 프로바이더 구현
│   │   │   ├── chat.go                    # 채팅 API
│   │   │   └── stream.go                  # 스트리밍
│   │   ├── azure/                         # Azure OpenAI
│   │   │   └── azure.go
│   │   ├── bedrock/                       # AWS Bedrock
│   │   │   └── bedrock.go
│   │   ├── cohere/                        # Cohere
│   │   │   └── cohere.go
│   │   ├── copilot/                       # GitHub Copilot
│   │   │   └── copilot.go
│   │   ├── gitlab/                        # GitLab AI
│   │   │   └── gitlab.go
│   │   ├── google/                        # Google AI
│   │   │   └── chat.go
│   │   ├── groq/                          # Groq
│   │   │   └── groq.go
│   │   ├── httpx/                         # HTTP 유틸
│   │   │   └── sse.go                     # SSE 처리
│   │   ├── instrument/                    # 계측
│   │   │   ├── instrument.go
│   │   │   └── scenario.go
│   │   ├── local/                         # 로컬 모델
│   │   │   ├── detect.go                  # 모델 감지
│   │   │   ├── installer.go               # 설치기
│   │   │   ├── inventory.go               # 인벤토리
│   │   │   ├── manifest_reader.go         # 매니페스트
│   │   │   └── runtime_limits.go          # 런타임 제한
│   │   ├── mistral/                       # Mistral AI
│   │   │   └── mistral.go
│   │   ├── observer/                      # 옵저버
│   │   │   ├── bench_recorder.go          # 벤치 기록
│   │   │   ├── chain.go                   # 체인
│   │   │   └── observer.go
│   │   ├── ollama/                        # Ollama
│   │   │   ├── health.go                  # 헬스 체크
│   │   │   ├── ollama.go                  # 프로바이더
│   │   │   ├── pull.go                    # 모델 풀
│   │   │   ├── serve.go                   # 서버 시작
│   │   │   └── tags_chat.go               # 태그/채팅
│   │   ├── openai/                        # OpenAI
│   │   │   ├── chat.go
│   │   │   ├── openai.go
│   │   │   └── stream.go
│   │   ├── openai_compat/                 # OpenAI 호환 레이어
│   │   │   └── client.go                  # 공용 클라이언트
│   │   ├── openrouter/                    # OpenRouter
│   │   │   └── openrouter.go
│   │   ├── perplexity/                    # Perplexity AI
│   │   │   └── perplexity.go
│   │   ├── pool/                          # 프로바이더 풀
│   │   │   ├── decay.go                   # 감쇠
│   │   │   ├── pool.go                    # 풀 관리
│   │   │   └── reliability.go             # 신뢰성
│   │   ├── streaming/                     # 스트리밍
│   │   │   ├── collector.go               # 수집기
│   │   │   ├── stream.go                  # 스트림
│   │   │   └── wrap.go                    # 래퍼
│   │   ├── together/                      # Together AI
│   │   │   └── together.go
│   │   ├── xai/                           # xAI (Grok)
│   │   │   └── xai.go
│   │   ├── auth_errors.go                 # 인증 에러
│   │   ├── errors.go                      # 에러 정의
│   │   ├── provider.go                    # Provider 인터페이스
│   │   ├── registry.go                    # 프로바이더 레지스트리
│   │   └── registry_auth.go               # 인증 레지스트리
│   ├── quality/                           # 품질 평가
│   │   ├── evaluator.go                   # 평가기
│   │   ├── evaluator_v2.go                # 평가기 v2
│   │   ├── recorder.go                    # 기록기
│   │   ├── signals.go                     # 신호
│   │   ├── signals_adapter.go             # 어댑터
│   │   └── types.go                       # 타입
│   ├── router/                            # 모델 라우터
│   │   ├── drift/                         # 드리프트 감지
│   │   │   ├── config.go
│   │   │   ├── detector.go
│   │   │   ├── stats.go
│   │   │   └── store.go
│   │   ├── http/                          # HTTP 라우트
│   │   │   └── router_routes.go
│   │   ├── learner/                       # 학습기
│   │   │   ├── decay.go
│   │   │   ├── features.go
│   │   │   ├── learner.go
│   │   │   ├── scoring.go
│   │   │   └── trainer.go
│   │   ├── rollback/                      # 롤백
│   │   │   ├── rollback.go
│   │   │   └── snapshot_store.go
│   │   ├── scope/                         # 스코프
│   │   │   ├── privacy.go
│   │   │   ├── resolve_scope.go
│   │   │   └── scope.go
│   │   ├── share/                         # 공유
│   │   │   ├── exporter.go
│   │   │   ├── importer.go
│   │   │   └── signer.go
│   │   ├── auto_update.go                 # 자동 업데이트
│   │   ├── bandit.go                      # 밴딧 알고리즘
│   │   ├── bench_router.go                # 벤치 라우터
│   │   ├── bench_scorer.go                # 벤치 스코어러
│   │   ├── bestarm_cache.go               # 최적 암 캐시
│   │   ├── coldstart.go                   # 콜드스타트
│   │   ├── enrich_okaon.go                # OkAON 강화
│   │   ├── explain.go                     # 설명
│   │   ├── learning.go                    # 학습
│   │   ├── learning_adapter.go            # 학습 어댑터
│   │   ├── policy.go                      # 정책
│   │   ├── policy_runtime.go              # 정책 런타임
│   │   ├── policy_store.go                # 정책 저장소
│   │   ├── reliability.go                 # 신뢰성
│   │   ├── reward_adjust.go               # 보상 조정
│   │   ├── router.go                      # 라우터 코어
│   │   ├── score.go                       # 스코어링
│   │   ├── scoring_v2.go                  # 스코어링 v2
│   │   ├── select_router.go               # 선택 라우터
│   │   ├── types.go                       # 타입
│   │   └── weights.go                     # 가중치
│   ├── runtime/                           # 런타임
│   │   └── version/                       # 버전 정보
│   ├── pty/                               # PTY 터미널 에뮬레이터
│   │   ├── pty.go                         # PTY 코어 (creack/pty)
│   │   └── model.go                       # Bubbletea PTY 모델
│   ├── server/                            # HTTP 서버
│   │   ├── middleware/                    # 미들웨어
│   │   │   ├── auth.go                    # 인증
│   │   │   └── workspace.go               # 워크스페이스
│   │   ├── routes/                        # 라우트
│   │   │   ├── acp.go                     # ACP SSE
│   │   │   ├── chat.go                    # 채팅 API
│   │   │   ├── health.go                  # 헬스 체크
│   │   │   ├── learning.go                # 학습 API
│   │   │   ├── provider_proxy.go          # 프록시
│   │   │   ├── providers.go               # 프로바이더 API
│   │   │   ├── register.go                # 등록
│   │   │   ├── stream.go                  # 스트림
│   │   │   ├── tasks.go                   # 태스크 API
│   │   │   ├── tools.go                   # 도구 API
│   │   │   └── websocket.go               # ACP WebSocket 허브
│   │   └── server.go                      # 서버 초기화
│   ├── session/                           # 세션 관리
│   │   ├── events.go                      # 이벤트
│   │   ├── llm_call.go                    # LLM 호출 기록
│   │   ├── processor.go                   # 프로세서
│   │   ├── session.go                     # 세션 구조체
│   │   ├── store.go                       # 저장소
│   │   └── stream.go                      # 스트림
│   ├── shell/                             # 셸
│   │   └── shell.go                       # 셸 유틸리티
│   ├── signal/                            # 시그널
│   │   ├── exec_runner.go                 # 실행 러너
│   │   ├── test_runner.go                 # 테스트 러너
│   │   ├── test_runner_os.go              # OS별 테스트
│   │   ├── tool_observer.go               # 도구 옵저버
│   │   └── types.go                       # 타입
│   ├── storage/                           # 저장소
│   │   └── sqlite/                        # SQLite
│   │       ├── boot.go                    # 부트
│   │       ├── migrations/                # 마이그레이션
│   │       └── sqlite.go                  # SQLite 연결
│   ├── tenancy/                           # 테넌시
│   │   ├── store.go                       # 저장소
│   │   └── workspace.go                   # 워크스페이스
│   ├── tool/                              # 도구 프레임워크
│   │   ├── builtin/                       # 내장 도구 (아래 상세)
│   │   ├── context.go                     # 도구 컨텍스트
│   │   ├── errors.go                      # 에러
│   │   ├── registry.go                    # 도구 레지스트리
│   │   ├── resultmeta.go                  # 결과 메타
│   │   ├── truncation.go                  # 출력 잘라내기
│   │   └── types.go                       # Tool 인터페이스
│   ├── tui/                               # Terminal UI
│   │   ├── app.go                         # TUI 앱
│   │   ├── components.go                  # 컴포넌트
│   │   ├── markdown.go                    # 마크다운 렌더
│   │   ├── model.go                       # Elm 모델
│   │   └── theme.go                       # 테마 (1800+ lines)
│   └── webui/                             # Web UI 서버
│       ├── static/                        # 정적 파일
│       ├── handlers.go                    # 핸들러
│       └── server.go                      # 서버
├── learning/                              # 학습 알고리즘
│   ├── bandit/                            # 밴딧
│   │   ├── bandit.go                      # 밴딧 인터페이스
│   │   ├── dist.go                        # 분포
│   │   └── thompson.go                    # Thompson Sampling
│   ├── regression/                        # 회귀
│   │   ├── model.go                       # 모델
│   │   └── trainer.go                     # 트레이너
│   └── learner.go                         # 학습기
├── pkg/                                   # 공개 패키지
│   └── sdk/
│       └── client_auth.go                 # SDK 클라이언트
├── platform/                              # 플랫폼 감지
│   ├── detect/
│   │   ├── cpu_mem.go                     # CPU/메모리
│   │   ├── env_fingerprint.go             # 환경 핑거프린트
│   │   ├── gpu_darwin.go                  # macOS GPU
│   │   ├── gpu_generic.go                 # 범용 GPU
│   │   ├── gpu_linux.go                   # Linux GPU
│   │   ├── gpu_windows.go                 # Windows GPU
│   │   ├── hw_profile.go                  # 하드웨어 프로필
│   │   ├── hw_profile_darwin.go           # macOS 프로필
│   │   ├── hw_profile_generic.go          # 범용 프로필
│   │   ├── hw_profile_linux.go            # Linux 프로필
│   │   ├── hw_profile_windows.go          # Windows 프로필
│   │   └── profile.go                     # 프로필 코어
│   └── tokenstore/
│       ├── secure_stub.go                 # 보안 스텁
│       └── tokenstore.go                  # 토큰 저장소
├── runtime/
│   └── llm/
│       └── models/                        # 로컬 모델 런타임
├── web/                                   # Web UI 프론트엔드
│   ├── public/
│   │   └── devorch.svg                    # 로고
│   ├── src/
│   │   ├── components/                    # React 컴포넌트
│   │   ├── App.tsx                        # 앱 루트
│   │   ├── index.css                      # 스타일
│   │   └── main.tsx                       # 엔트리
│   ├── index.html
│   ├── package.json
│   ├── postcss.config.js
│   ├── tailwind.config.js
│   ├── tsconfig.json
│   ├── tsconfig.node.json
│   └── vite.config.ts
├── .gitignore
├── devorch.db                             # SQLite 데이터베이스
├── go.mod                                 # Go 모듈
├── go.sum                                 # 의존성 체크섬
├── Project.md                             # 프로젝트 문서
└── README.md                              # README

116개 디렉토리, 297개 파일
```

---

## 3. 핵심 인터페이스 정의

### 3.1 Provider 인터페이스

**파일**: `internal/provider/provider.go`

```go
// Provider는 LLM 프로바이더의 기본 인터페이스
type Provider interface {
    Name() string                                    // 프로바이더 이름 (예: "openai")
    Kind() string                                    // 종류: "remote" 또는 "local"
    Endpoint() string                                // 기본 URL 또는 로컬 런타임 ID
    Health(ctx context.Context) error               // 헬스 체크
    ListModels(ctx context.Context) ([]Model, error) // 사용 가능한 모델 목록
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) // 채팅 완성
}

// ChatClient는 Phase 6 스타일 프로바이더용 인터페이스
type ChatClient interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    ChatStream(ctx context.Context, req ChatRequest, yield func(StreamChunk) error) (ChatResponse, error)
}

// ChatRequest 구조체
type ChatRequest struct {
    Provider    string         // 프로바이더 이름
    Model       string         // 모델 ID
    Messages    []ChatMessage  // 대화 메시지
    Temperature float64        // 온도 (0.0-2.0)
    MaxTokens   int           // 최대 토큰
    Stream      bool          // 스트리밍 여부
    Workspace   string        // 워크스페이스 ID
    SessionID   string        // 세션 ID
    TaskID      string        // 태스크 ID
}

// ChatResponse 구조체
type ChatResponse struct {
    RawText          string      // 원시 텍스트
    PromptTokens     int         // 프롬프트 토큰 수
    CompletionTokens int         // 완성 토큰 수
    TotalTokens      int         // 총 토큰 수
    CostMicroUSD     int64       // 비용 (마이크로 USD)
    ToolCalls        []ToolCall  // 도구 호출
    Usage_           Usage       // 사용량 상세
}

// Message 역할
const (
    RoleSystem Role = "system"     // 시스템 메시지
    RoleUser   Role = "user"       // 사용자 메시지
    RoleAssist Role = "assistant"  // 어시스턴트 메시지
    RoleTool   Role = "tool"       // 도구 결과 메시지
)
```

### 3.2 Tool 인터페이스

**파일**: `internal/tool/types.go`

```go
// Tool은 AI 에이전트가 사용할 수 있는 도구 인터페이스
type Tool interface {
    // Info는 도구의 메타데이터 반환
    Info() ToolInfo

    // Execute는 주어진 파라미터로 도구 실행
    Execute(ctx *Context, params json.RawMessage) (*Result, error)
}

// ToolInfo는 도구 정의
type ToolInfo struct {
    Name        string          `json:"name"`        // 고유 식별자
    Description string          `json:"description"` // AI가 이해할 설명
    Parameters  json.RawMessage `json:"parameters"`  // JSON 스키마
}

// Result는 도구 실행 결과
type Result struct {
    Output      string         `json:"output"`             // 주요 출력 텍스트
    Title       string         `json:"title,omitempty"`    // 표시용 제목
    Truncated   bool           `json:"truncated,omitempty"` // 잘림 여부
    ExitCode    int            `json:"exit_code,omitempty"` // 명령 종료 코드
    Metadata    map[string]any `json:"metadata,omitempty"`  // 추가 데이터
    Attachments []Attachment   `json:"attachments,omitempty"` // 첨부 파일
}

// Context는 도구 실행 컨텍스트
type Context struct {
    Ctx         context.Context // Go 컨텍스트
    SessionID   string          // 세션 ID
    Workspace   string          // 워크스페이스 경로
    WorkDir     string          // 작업 디렉토리
    // ... 추가 필드
}
```

### 3.3 Session 구조체

**파일**: `internal/session/session.go`

```go
// Session은 대화 세션
type Session struct {
    ID          string         `json:"id"`           // 세션 ID (ULID)
    Name        string         `json:"name"`         // 세션 이름
    Workspace   string         `json:"workspace"`    // 워크스페이스 경로
    Messages    []Message      `json:"messages"`     // 메시지 목록
    Metadata    map[string]any `json:"metadata"`     // 메타데이터
    Summary     string         `json:"summary"`      // 요약
    Tags        []string       `json:"tags"`         // 태그
    ParentID    string         `json:"parent_id"`    // 포크된 세션의 부모 ID
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    TotalTokens int            `json:"total_tokens"` // 총 토큰 수
}

// Message는 대화 메시지
type Message struct {
    ID          string         `json:"id"`
    Role        MessageRole    `json:"role"`         // user|assistant|system|tool
    Content     string         `json:"content"`
    Tokens      int            `json:"tokens"`
    ToolCalls   []ToolCall     `json:"tool_calls"`
    ToolResults []ToolResult   `json:"tool_results"`
    Metadata    map[string]any `json:"metadata"`
    CreatedAt   time.Time      `json:"created_at"`
}

// 메시지 역할
const (
    RoleUser      MessageRole = "user"
    RoleAssistant MessageRole = "assistant"
    RoleSystem    MessageRole = "system"
    RoleTool      MessageRole = "tool"
)
```

### 3.4 Agent 구조체

**파일**: `internal/agent/agent.go`

```go
// Agent는 특정 기능을 가진 AI 에이전트
type Agent struct {
    Name         string         `json:"name"`          // 고유 식별자
    Model        string         `json:"model"`         // 기본 모델 (예: "anthropic/claude-3-opus")
    Temperature  float64        `json:"temperature"`   // 응답 온도
    SystemPrompt string         `json:"system_prompt"` // 에이전트 행동 정의
    AllowedTools []string       `json:"allowed_tools"` // 허용 도구 (빈값=전체)
    DeniedTools  []string       `json:"denied_tools"`  // 거부 도구
    MaxTokens    int            `json:"max_tokens"`
    Metadata     map[string]any `json:"metadata"`
}

// CanUseTool은 특정 도구 사용 가능 여부 확인
func (a *Agent) CanUseTool(tool string) bool
```

### 3.5 Router 인터페이스

**파일**: `internal/router/router.go`

```go
// Router는 모델 선택 라우터
type Router struct {
    w        Weights           // 가중치
    bandit   Bandit            // 밴딧 알고리즘
    enricher *OkAONEnricher    // 통계 강화기
}

// Select는 최적의 프로바이더/모델 선택
func (r *Router) Select(ctx context.Context, wl Workload, candidates []Candidate) (Decision, error)

// Decision은 라우팅 결정
type Decision struct {
    Provider string  // 선택된 프로바이더
    Model    string  // 선택된 모델
    Endpoint string  // 엔드포인트
    Score    float64 // 스코어
    Why      string  // 선택 이유
}

// Weights는 라우팅 가중치
type Weights struct {
    Quality float64 // 품질 가중치
    Latency float64 // 지연시간 가중치
    Cost    float64 // 비용 가중치
}
```

---

## 4. 내장 도구 상세

**위치**: `internal/tool/builtin/`

### 4.1 파일 시스템 도구

| 도구 | 파일 | 설명 | 주요 파라미터 |
|------|------|------|--------------|
| `bash` | bash.go | 지속 셸 실행 (Windows 제외) | `command`, `timeout`, `workdir` |
| `read` | read.go | 파일 읽기 | `path`, `start_line`, `end_line` |
| `write` | write.go | 파일 쓰기 | `path`, `content`, `create_dirs` |
| `edit` | edit.go | 파일 편집 (old→new) | `path`, `old_text`, `new_text` |
| `multiedit` | multiedit.go | 다중 파일 편집 | `edits[]` (path, old, new) |
| `view` | view.go | 파일 보기 (라인 번호 표시) | `path`, `start`, `end` |
| `patch` | patch.go | 통합 diff 패치 적용 | `patch` (diff 형식) |
| `glob` | glob.go | 패턴 기반 파일 검색 | `pattern`, `root` |
| `grep` | grep.go | 텍스트 검색 | `pattern`, `path`, `regex` |
| `ls` | ls.go | 디렉토리 목록 | `path`, `recursive`, `hidden` |

### 4.2 웹/검색 도구

| 도구 | 파일 | 설명 | 주요 파라미터 |
|------|------|------|--------------|
| `websearch` | websearch.go | 웹 검색 (DuckDuckGo/Brave/Google) | `query`, `engine`, `max_results` |
| `webfetch` | webfetch.go | 웹 페이지 가져오기 | `url`, `selector` |
| `fetch` | fetch.go | 범용 HTTP 요청 | `url`, `method`, `headers` |
| `sourcegraph` | sourcegraph.go | Sourcegraph 코드 검색 | `query`, `repo` |
| `codesearch` | codesearch.go | 시맨틱 코드 검색 | `query`, `type` (text/symbol/ref) |

### 4.3 작업 관리 도구

| 도구 | 파일 | 설명 | 주요 파라미터 |
|------|------|------|--------------|
| `task` | task.go | 태스크 관리 | `action`, `id`, `title`, `status` |
| `todo` | todo.go | 할 일 관리 | `action`, `items[]` |
| `plan` | plan.go | 계획 수립/추적 | `action`, `steps[]` |

### 4.4 특수 도구

| 도구 | 파일 | 설명 | 주요 파라미터 |
|------|------|------|--------------|
| `agent` | agent.go | 읽기 전용 하위 에이전트 | `task`, `tools[]` |
| `diagnostics` | diagnostics.go | LSP 진단 수집 | `path`, `severity` |
| `question` | question.go | 대화형 사용자 질문 | `question`, `options[]`, `default` |
| `skill` | skill.go | 스킬 정의/호출 | `skill_name`, `params` |
| `batch` | batch.go | 다중 도구 배치 실행 | `operations[]`, `parallel`, `stop_on_error` |

### 4.5 도구 등록 (builtin.go)

```go
// DefaultTools는 모든 내장 도구 반환
func DefaultTools(rootDir string) []tool.Tool {
    return []tool.Tool{
        // 파일 시스템
        &BashTool{DefaultWorkDir: rootDir},
        &ReadTool{RootDir: rootDir, AllowExternal: false},
        &WriteTool{RootDir: rootDir, AllowOverwrite: true, CreateDirs: true},
        &EditTool{RootDir: rootDir},
        &MultiEditTool{RootDir: rootDir},
        &GlobTool{RootDir: rootDir},
        &GrepTool{RootDir: rootDir},
        &LsTool{RootDir: rootDir},
        NewViewTool(),
        NewPatchTool(),
        
        // 웹
        &WebFetchTool{},
        &WebSearchTool{BraveAPIKey: os.Getenv("BRAVE_API_KEY")},
        &FetchTool{},
        &SourcegraphTool{},
        
        // 코드 검색
        codeSearchTool,
        
        // 작업 관리 (싱글톤)
        taskTool,
        todoTool,
        planTool,
        
        // 특수
        agentTool,
        diagnosticsTool,
        questionTool,
        skillTool,
    }
}
```

---

## 5. 프로바이더 상세

### 5.1 지원 프로바이더 목록

| 프로바이더 | 디렉토리 | 환경 변수 | 설명 |
|-----------|---------|-----------|------|
| OpenAI | `openai/` | `OPENAI_API_KEY` | GPT-4, GPT-3.5 등 |
| Anthropic | `anthropic/` | `ANTHROPIC_API_KEY` | Claude 3 Opus/Sonnet |
| Google | `google/` | `GOOGLE_API_KEY` | Gemini Pro/Ultra |
| Azure | `azure/` | `AZURE_OPENAI_*` | Azure OpenAI Service |
| AWS Bedrock | `bedrock/` | `AWS_*` | Claude, Titan 등 |
| Groq | `groq/` | `GROQ_API_KEY` | 고속 추론 |
| Mistral | `mistral/` | `MISTRAL_API_KEY` | Mistral Large 등 |
| Together | `together/` | `TOGETHER_API_KEY` | 오픈소스 모델 |
| OpenRouter | `openrouter/` | `OPENROUTER_API_KEY` | 멀티 프로바이더 |
| Perplexity | `perplexity/` | `PERPLEXITY_API_KEY` | 검색 기반 LLM |
| xAI | `xai/` | `XAI_API_KEY` | Grok 모델 |
| Copilot | `copilot/` | `GITHUB_TOKEN` | GitHub Copilot |
| Cohere | `cohere/` | `COHERE_API_KEY` | Command 모델 |
| GitLab | `gitlab/` | `GITLAB_TOKEN` | GitLab AI |
| Ollama | `ollama/` | (로컬) | 로컬 모델 실행 |

### 5.2 환경 변수 전체 목록

```bash
# === LLM 프로바이더 ===
OPENAI_API_KEY=sk-...                    # OpenAI
ANTHROPIC_API_KEY=sk-ant-...             # Anthropic
GOOGLE_API_KEY=AIza...                   # Google AI
GROQ_API_KEY=gsk_...                     # Groq
MISTRAL_API_KEY=...                      # Mistral
TOGETHER_API_KEY=...                     # Together AI
OPENROUTER_API_KEY=sk-or-...             # OpenRouter
PERPLEXITY_API_KEY=pplx-...              # Perplexity
XAI_API_KEY=xai-...                      # xAI (Grok)
COHERE_API_KEY=...                       # Cohere

# === 클라우드 ===
AZURE_OPENAI_ENDPOINT=https://...        # Azure OpenAI
AZURE_OPENAI_API_KEY=...
AZURE_OPENAI_DEPLOYMENT=gpt-4

AWS_ACCESS_KEY_ID=AKIA...                # AWS Bedrock
AWS_SECRET_ACCESS_KEY=...
AWS_REGION=us-east-1
AWS_PROFILE=default                      # 또는 프로필 사용

# === 코드 호스팅 ===
GITHUB_TOKEN=ghp_...                     # GitHub Copilot
COPILOT_TOKEN=ghu_...                    # 또는 Copilot 토큰
GITLAB_TOKEN=glpat-...                   # GitLab AI
CI_JOB_TOKEN=...                         # GitLab CI

# === 검색 ===
BRAVE_API_KEY=BSA...                     # Brave Search
SERP_API_KEY=...                         # SerpAPI (Google)
```

### 5.3 프로바이더 구현 패턴

**openai_compat 클라이언트 사용** (`internal/provider/openai_compat/client.go`):

```go
// 대부분의 프로바이더는 OpenAI 호환 클라이언트 사용
type Client struct {
    baseURL string
    apiKey  string
    http    *http.Client
}

// 새 프로바이더 추가 예시 (xai/xai.go)
type XAI struct {
    client *openai_compat.Client
}

func New() *XAI {
    return &XAI{
        client: openai_compat.NewClient(
            "https://api.x.ai/v1",
            os.Getenv("XAI_API_KEY"),
        ),
    }
}

func (x *XAI) Name() string { return "xai" }
func (x *XAI) Kind() string { return "remote" }
func (x *XAI) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
    return x.client.Chat(ctx, req)
}
```

### 5.4 프로바이더 등록 (wire.go)

```go
func WireMinimal(cfg config.Config) (*Deps, error) {
    reg := provider.NewRegistry()
    
    // 환경 변수 기반 자동 등록
    if cfg.OpenAIKey != "" {
        reg.Register(openai.New(cfg.OpenAIKey))
    }
    if os.Getenv("ANTHROPIC_API_KEY") != "" {
        reg.Register(anthropic.New())
    }
    if os.Getenv("AWS_ACCESS_KEY_ID") != "" || os.Getenv("AWS_PROFILE") != "" {
        reg.Register(bedrock.New())
    }
    // ... 기타 프로바이더
    
    // Ollama는 항상 등록 (로컬)
    reg.Register(ollama.New(cfg.Ollama.Host, ollamaBin))
    
    return &Deps{ProviderRegistry: reg, ...}, nil
}
```

---

## 6. 라우터 및 모델 선택

### 6.1 라우팅 알고리즘

**Thompson Sampling 기반 밴딧**:

```go
// internal/router/bandit.go
type EpsilonGreedy struct {
    epsilon float64 // 탐색 확률 (기본 0.1)
}

func (e *EpsilonGreedy) Choose(bestIdx, n int) int {
    if rand.Float64() < e.epsilon {
        return rand.Intn(n)  // 탐색
    }
    return bestIdx  // 활용
}
```

**스코어링 공식**:
```
score = w_quality × quality + w_latency × (1 / (1 + latency/200)) + w_cost × cost_score
```

### 6.2 OkAON 통계

**파일**: `internal/okaon/models.go`

```go
// Work는 작업 단위
type Work struct {
    ID        string
    SessionID string
    Workspace string
    TaskType  string
    CreatedAt time.Time
}

// Run은 LLM 호출 기록
type Run struct {
    ID          string
    WorkID      string
    Provider    string
    Model       string
    LatencyMs   int64
    PromptTok   int
    CompleteTok int
    CostMicro   int64
    Error       string
    CreatedAt   time.Time
}

// Quality는 품질 평가
type Quality struct {
    RunID     string
    Accuracy  float64
    Relevance float64
    Score     float64
}
```

---

## 7. TUI (Terminal UI)

### 7.1 테마 시스템

**파일**: `internal/tui/theme.go` (1800+ 라인)

```go
type Theme struct {
    // 레이아웃
    Title    lipgloss.Style  // 제목 스타일
    Footer   lipgloss.Style  // 푸터 스타일
    InputBox lipgloss.Style  // 입력창 스타일
    Border   lipgloss.Style  // 테두리 스타일
    
    // 메시지
    UserMsg      lipgloss.Style  // 사용자 메시지
    AssistantMsg lipgloss.Style  // AI 응답
    SystemMsg    lipgloss.Style  // 시스템 메시지
    
    // 강조
    Help    lipgloss.Style  // 도움말
    Error   lipgloss.Style  // 에러
    Success lipgloss.Style  // 성공
    Warning lipgloss.Style  // 경고
    
    // 코드
    CodeBlock  lipgloss.Style  // 코드 블록
    InlineCode lipgloss.Style  // 인라인 코드
}

// 사전 정의된 테마들 (총 31개)
func DefaultTheme() Theme              // 기본 다크 테마
func LightTheme() Theme                // 라이트 테마
func MonochromeTheme() Theme           // 모노크롬
func DraculaTheme() Theme              // Dracula
func NordTheme() Theme                 // Nord
func SolarizedTheme() Theme            // Solarized Light
func SolarizedDarkTheme() Theme        // Solarized Dark
func GruvboxTheme() Theme              // Gruvbox Dark
func GruvboxLightTheme() Theme         // Gruvbox Light
func TokyoNightTheme() Theme           // Tokyo Night
func TokyoNightStormTheme() Theme      // Tokyo Night Storm
func TokyoNightDayTheme() Theme        // Tokyo Night Day
func OneDarkTheme() Theme              // One Dark
func OneLightTheme() Theme             // One Light
func MaterialTheme() Theme             // Material
func MaterialLightTheme() Theme        // Material Light
func CatppuccinTheme() Theme           // Catppuccin Mocha
func CatppuccinLatteTheme() Theme      // Catppuccin Latte
func CatppuccinFrappeTheme() Theme     // Catppuccin Frappé
func CatppuccinMacchiatoTheme() Theme  // Catppuccin Macchiato
func AyuDarkTheme() Theme              // Ayu Dark
func AyuLightTheme() Theme             // Ayu Light
func AyuMirageTheme() Theme            // Ayu Mirage
func RosePineTheme() Theme             // Rosé Pine
func RosePineMoonTheme() Theme         // Rosé Pine Moon
func RosePineDawnTheme() Theme         // Rosé Pine Dawn
func NightOwlTheme() Theme             // Night Owl
func GitHubDarkTheme() Theme           // GitHub Dark
func GitHubLightTheme() Theme          // GitHub Light
func KanagawaWaveTheme() Theme         // Kanagawa Wave
func KanagawaDragonTheme() Theme       // Kanagawa Dragon
func Cobalt2Theme() Theme              // Cobalt2
```

### 7.2 TUI 컴포넌트

```go
// internal/tui/model.go
type Model struct {
    messages  []Message       // 대화 메시지
    input     textinput.Model // 입력 필드
    viewport  viewport.Model  // 스크롤 뷰
    theme     Theme           // 현재 테마
    width     int
    height    int
    // ...
}

// Elm Architecture
func (m Model) Init() tea.Cmd
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m Model) View() string
```

---

## 8. 국제화 (i18n)

### 8.1 구조

**파일**: `internal/i18n/i18n.go`

```go
// 지원 언어
const (
    English  Language = "en"
    Korean   Language = "ko"
    Japanese Language = "ja"
    Chinese  Language = "zh"
)

// Translator 사용
type Translator struct {
    currentLang  Language
    translations map[Language]map[string]string
    fallback     Language
}

// 글로벌 인스턴스
var global *Translator

// 사용법
i18n.SetLanguage(i18n.Korean)
msg := i18n.T("welcome.message")
msg := i18n.T("error.file_not_found", filename)  // 포맷팅
```

### 8.2 번역 키 예시

```go
// translations_en.go
var englishTranslations = map[string]string{
    "app.name":               "DevOrch",
    "welcome.message":        "Welcome to DevOrch",
    "error.file_not_found":   "File not found: %s",
    "tool.bash.description":  "Execute shell commands",
    "tool.read.description":  "Read file contents",
    // ...
}

// translations_ko.go
var koreanTranslations = map[string]string{
    "app.name":               "DevOrch",
    "welcome.message":        "DevOrch에 오신 것을 환영합니다",
    "error.file_not_found":   "파일을 찾을 수 없습니다: %s",
    "tool.bash.description":  "셸 명령 실행",
    "tool.read.description":  "파일 내용 읽기",
    // ...
}
```

---

## 9. 서버 및 API

### 9.1 HTTP 서버

**파일**: `internal/server/server.go`

```go
type Server struct {
    Mux      *http.ServeMux
    Sessions *session.Processor
    Hub      *bus.Hub        // SSE 이벤트 허브
    Tools    *tool.Registry
}

// 라우트 등록
func New(hub *bus.Hub, proc *session.Processor) *Server {
    mux := http.NewServeMux()
    
    // 스트림 라우트
    sr := &routes.StreamRoutes{Hub: hub}
    sr.Register(mux)
    
    // 도구 라우트
    tr := &routes.ToolsRoutes{Registry: reg}
    tr.RegisterTools(mux)
    
    // ACP SSE 스트림
    ar := &routes.ACPRoutes{Hub: hub}
    ar.RegisterACP(mux)
    
    return &Server{...}
}
```

### 9.2 API 엔드포인트

| 경로 | 메서드 | 설명 |
|------|--------|------|
| `/health` | GET | 헬스 체크 |
| `/api/chat` | POST | 채팅 완성 |
| `/api/stream` | GET | SSE 스트림 |
| `/api/tools` | GET | 도구 목록 |
| `/api/tools/{name}` | POST | 도구 실행 |
| `/api/providers` | GET | 프로바이더 목록 |
| `/api/sessions` | GET/POST | 세션 관리 |
| `/api/tasks` | GET/POST | 태스크 관리 |

---

## 10. 데이터베이스 스키마

### 10.1 SQLite 테이블

**파일**: `internal/storage/sqlite/migrations/`

```sql
-- 세션 테이블
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    name TEXT,
    workspace TEXT,
    messages TEXT,  -- JSON
    metadata TEXT,  -- JSON
    summary TEXT,
    tags TEXT,      -- JSON array
    parent_id TEXT,
    created_at DATETIME,
    updated_at DATETIME,
    total_tokens INTEGER
);

-- OkAON 작업 테이블
CREATE TABLE okaon_works (
    id TEXT PRIMARY KEY,
    session_id TEXT,
    workspace TEXT,
    task_type TEXT,
    created_at DATETIME
);

-- OkAON 실행 테이블
CREATE TABLE okaon_runs (
    id TEXT PRIMARY KEY,
    work_id TEXT REFERENCES okaon_works(id),
    provider TEXT,
    model TEXT,
    latency_ms INTEGER,
    prompt_tok INTEGER,
    complete_tok INTEGER,
    cost_micro INTEGER,
    error TEXT,
    created_at DATETIME
);

-- 품질 평가 테이블
CREATE TABLE okaon_quality (
    run_id TEXT PRIMARY KEY REFERENCES okaon_runs(id),
    accuracy REAL,
    relevance REAL,
    score REAL
);
```

---

## 11. 빌드 및 실행

### 11.1 빌드 명령

```bash
# 데몬 빌드
go build -o bin/devorchd ./cmd/devorchd

# CLI 빌드
go build -o bin/devorch ./cmd/devorch

# 크로스 컴파일
GOOS=linux GOARCH=amd64 go build -o bin/devorchd-linux-amd64 ./cmd/devorchd
GOOS=darwin GOARCH=arm64 go build -o bin/devorchd-darwin-arm64 ./cmd/devorchd
GOOS=windows GOARCH=amd64 go build -o bin/devorchd-windows-amd64.exe ./cmd/devorchd

# 테스트
go test ./...
```

### 11.2 실행 명령

```bash
# 데몬 실행
./bin/devorchd

# 환경 변수 설정 후 실행
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
./bin/devorchd

# 웹 UI 개발 서버
cd web && npm install && npm run dev
```

### 11.3 빌드 태그

```go
// SQLite - OS 제한
//go:build darwin || linux || windows
// +build darwin linux windows

// Bash 도구 - Windows 제외
//go:build !windows
// +build !windows
```

---

## 12. 확장 및 커스터마이징

### 12.1 새 도구 추가

```go
// internal/tool/builtin/mytool.go
package builtin

import (
    "encoding/json"
    "devorch/internal/tool"
)

type MyTool struct{}

func (t *MyTool) Info() tool.ToolInfo {
    return tool.ToolInfo{
        Name:        "mytool",
        Description: "My custom tool description",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "input": {"type": "string", "description": "Input value"}
            },
            "required": ["input"]
        }`),
    }
}

func (t *MyTool) Execute(ctx *tool.Context, params json.RawMessage) (*tool.Result, error) {
    var p struct {
        Input string `json:"input"`
    }
    if err := json.Unmarshal(params, &p); err != nil {
        return nil, err
    }
    
    return &tool.Result{
        Output: "Processed: " + p.Input,
    }, nil
}
```

### 12.2 새 프로바이더 추가

```go
// internal/provider/myprovider/myprovider.go
package myprovider

import (
    "context"
    "os"
    "devorch/internal/provider"
    "devorch/internal/provider/openai_compat"
)

type MyProvider struct {
    client *openai_compat.Client
}

func New() *MyProvider {
    return &MyProvider{
        client: openai_compat.NewClient(
            "https://api.myprovider.com/v1",
            os.Getenv("MY_API_KEY"),
        ),
    }
}

func (p *MyProvider) Name() string     { return "myprovider" }
func (p *MyProvider) Kind() string     { return "remote" }
func (p *MyProvider) Endpoint() string { return "https://api.myprovider.com" }

func (p *MyProvider) Health(ctx context.Context) error {
    return p.client.Health(ctx)
}

func (p *MyProvider) ListModels(ctx context.Context) ([]provider.Model, error) {
    return []provider.Model{{ID: "my-model-v1"}}, nil
}

func (p *MyProvider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
    return p.client.Chat(ctx, req)
}
```

### 12.3 스킬 정의

**위치**: `~/.devorch/skills/`

```yaml
# ~/.devorch/skills/code_review.yaml
name: code_review
description: Perform thorough code review
prompt: |
  Review the following code for:
  - Bugs and potential issues
  - Performance improvements
  - Code style and best practices
  
  Code:
  {{.code}}
  
  Language: {{.language | default "auto"}}
```

---

## 13. 주요 의존성

```go
// go.mod 주요 의존성
require (
    github.com/charmbracelet/bubbletea v1.x    // TUI 프레임워크
    github.com/charmbracelet/lipgloss v1.x     // TUI 스타일링
    modernc.org/sqlite v1.x                    // 순수 Go SQLite
    github.com/aws/aws-sdk-go-v2 v1.x          // AWS SDK
    golang.org/x/net v0.x                      // 네트워킹
)
```

---

## 14. 개발 가이드

### 14.1 코드 스타일

- Go 표준 포맷팅 (`gofmt`, `goimports`)
- 패키지 단위 테스트 (`*_test.go`)
- 인터페이스 기반 설계
- 에러는 `fmt.Errorf`로 컨텍스트 추가

### 14.2 테스트

```bash
# 전체 테스트
go test ./...

# 특정 패키지
go test ./internal/tool/builtin/...

# 커버리지
go test -cover ./...
```

### 14.3 디버깅

```bash
# 로그 레벨 설정
export DEVORCH_LOG_LEVEL=debug
./bin/devorchd

# 특정 프로바이더만 활성화
export OPENAI_API_KEY=sk-...
# 다른 키는 설정하지 않음
./bin/devorchd
```

---

## 15. 파일 통계

- **전체 디렉토리**: 116개
- **전체 파일**: 297개
- **Go 소스 파일**: ~250개
- **TypeScript/React 파일**: ~15개
- **문서 파일**: ~50개

---

## 16. 향후 작업

- [ ] JS 플러그인 런타임 (goja 기반)
- [ ] 프로바이더 통합 테스트 강화
- [ ] 커스텀 테마 로더
- [ ] LSP 푸시 알림 통합
- [ ] 파일 히스토리/되돌리기 UX 개선
- [ ] README 프로바이더 설정 상세화

---

**본 문서는 2026년 1월 30일 main 브랜치 기준으로 작성되었습니다.**
**새로운 AI 모델이 이 문서만으로 개발을 계속할 수 있도록 모든 핵심 정보를 포함합니다.**
