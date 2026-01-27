# 🔍 OpenCode 프로젝트 분석 보고서

> **버전**: 1.1.36  
> **분석 일자**: 2026년 1월 28일  
> **저장소**: github.com/anomalyco/opencode  
> **기본 브랜치**: dev

---

## 1. 프로젝트 개요

OpenCode는 **100% 오픈소스 AI 코딩 에이전트**로, Claude Code의 대안으로 설계되었습니다. 특정 AI 프로바이더에 종속되지 않으며, TUI(터미널 UI), 웹, 데스크톱 앱을 모두 지원합니다.

### 핵심 특징
- 🔓 **프로바이더 독립성**: 20+ AI 프로바이더 지원
- 🖥️ **다양한 인터페이스**: CLI TUI, 웹, 데스크톱 앱
- 🔧 **개발자 도구 통합**: LSP, MCP 지원
- 📱 **원격 제어**: 클라이언트/서버 아키텍처
- 🔌 **ACP 지원**: Agent Client Protocol을 통한 외부 에디터 통합 (Zed 등)
- 🤖 **GitHub Action**: PR/이슈에서 `/opencode` 명령어로 AI 에이전트 호출

### 보안 모델

> ⚠️ **중요**: OpenCode는 **샌드박스가 없습니다**. 권한 시스템은 UX 기능으로, 에이전트가 수행하는 작업을 사용자가 인지할 수 있도록 확인 프롬프트를 제공합니다. 보안 격리가 필요하면 Docker 컨테이너나 VM에서 실행하세요.

서버 모드는 opt-in이며, `OPENCODE_SERVER_PASSWORD`를 설정하여 HTTP Basic Auth를 활성화할 수 있습니다.

---

## 2. 기술 스택

| 분류 | 기술 |
|------|------|
| **런타임** | Bun 1.3+ |
| **언어** | TypeScript 5.8 |
| **프론트엔드** | SolidJS |
| **빌드 도구** | Vite 7.x, Turbo |
| **인프라** | SST (Cloudflare Workers, PlanetScale) |
| **데스크톱** | Tauri 2.x (Rust) |
| **스타일링** | Tailwind CSS 4.x |

---

## 3. 프로젝트 루트 구조

```
opencode/
├── .git/                    # Git 저장소
├── .github/                 # GitHub 설정
│   ├── actions/             # 커스텀 GitHub Actions
│   ├── ISSUE_TEMPLATE/      # 이슈 템플릿
│   ├── workflows/           # CI/CD 워크플로우
│   ├── CODEOWNERS           # 코드 소유자 설정
│   └── pull_request_template.md
├── .husky/                  # Git 훅 (pre-push)
│   └── pre-push             # 푸시 전 실행 스크립트
├── .opencode/               # OpenCode 자체 설정
│   ├── agent/               # 커스텀 에이전트 정의
│   ├── command/             # 커스텀 명령어
│   ├── skill/               # 커스텀 스킬
│   ├── themes/              # 커스텀 테마
│   ├── tool/                # 커스텀 도구
│   └── opencode.jsonc       # 프로젝트별 설정 파일
├── .vscode/                 # VS Code 설정 예제
├── github/                  # GitHub Action 패키지
│   ├── script/              # 배포 스크립트
│   ├── action.yml           # Action 정의
│   ├── index.ts             # Action 진입점
│   └── package.json
├── infra/                   # SST 인프라 코드
│   ├── app.ts               # 메인 앱 인프라 (API, 웹, 앱)
│   ├── console.ts           # 관리 콘솔 인프라
│   ├── enterprise.ts        # 엔터프라이즈 기능
│   ├── secret.ts            # 시크릿 관리
│   └── stage.ts             # 스테이지별 설정
├── logs/                    # 로컬 로그 파일
├── nix/                     # Nix 패키지 설정
│   ├── scripts/             # Nix 관련 스크립트
│   ├── desktop.nix          # 데스크톱 앱 Nix 설정
│   ├── opencode.nix         # CLI Nix 설정
│   └── hashes.json          # 의존성 해시
├── packages/                # 🔷 모노레포 패키지들
├── patches/                 # npm 패키지 패치
│   └── ghostty-web@0.3.0.patch
├── script/                  # 빌드/배포 스크립트
│   ├── changelog.ts         # 체인지로그 생성
│   ├── duplicate-pr.ts      # PR 복제
│   ├── format.ts            # 코드 포맷팅
│   ├── generate.ts          # SDK/타입 생성
│   ├── publish-complete.ts  # 배포 완료 처리
│   ├── publish-start.ts     # 배포 시작 처리
│   ├── release              # 릴리스 스크립트
│   ├── stats.ts             # 통계 생성
│   └── sync-zed.ts          # Zed 확장 동기화
├── sdks/                    # 외부 SDK
│   └── vscode/              # VS Code 확장 SDK
├── specs/                   # 기술 명세서
│   ├── 01-persist-payload-limits.md
│   ├── 02-cache-eviction.md
│   ├── 03-request-throttling.md
│   ├── 04-scroll-spy-optimization.md
│   ├── 05-modularize-and-dedupe.md
│   ├── 06-app-i18n-audit.md
│   ├── 07-ui-i18n-audit.md
│   ├── 08-app-e2e-smoke-suite.md
│   ├── perf-roadmap.md      # 성능 로드맵
│   └── project.md           # 프로젝트 명세
├── themes/                  # 커스텀 테마
│   ├── deltarune.json
│   └── undertale.json
├── .editorconfig            # 에디터 설정
├── .gitignore               # Git 무시 파일
├── .prettierignore          # Prettier 무시 파일
├── AGENTS.md                # AI 에이전트 가이드라인
├── bun.lock                 # Bun 락파일
├── bunfig.toml              # Bun 설정
├── CONTRIBUTING.md          # 기여 가이드
├── flake.nix                # Nix Flake 설정
├── install                  # 설치 스크립트
├── LICENSE                  # MIT 라이선스
├── package.json             # 루트 패키지 설정
├── README.*.md              # 다국어 README (15개 언어)
├── SECURITY.md              # 보안 정책
├── sst-env.d.ts             # SST 타입 정의
├── sst.config.ts            # SST 설정
├── STATS.md                 # 프로젝트 통계
├── tsconfig.json            # TypeScript 설정
└── turbo.json               # Turborepo 설정
```

---

## 4. packages/ 상세 구조

### 4.1 `packages/opencode/` - 🔷 핵심 엔진 (40,626줄)

```
packages/opencode/
├── bin/
│   └── opencode             # CLI 진입점 (shebang 스크립트)
├── script/
│   ├── build.ts             # 빌드 스크립트
│   ├── postinstall.mjs      # 설치 후 스크립트
│   ├── publish-registries.ts # 레지스트리 배포
│   ├── publish.ts           # npm 배포
│   ├── schema.ts            # JSON 스키마 생성
│   └── seed-e2e.ts          # E2E 테스트 시드
├── src/
│   ├── acp/                 # Agent Communication Protocol
│   │   └── index.ts         # ACP 클라이언트/서버 구현
│   ├── agent/               # AI 에이전트 시스템
│   │   ├── agent.ts         # 에이전트 정의 (build, plan, general, explore)
│   │   ├── generate.txt     # 생성 프롬프트
│   │   └── prompt/          # 에이전트별 프롬프트
│   │       ├── compaction.txt   # 컨텍스트 압축 프롬프트
│   │       ├── explore.txt      # 탐색 에이전트 프롬프트
│   │       ├── summary.txt      # 요약 생성 프롬프트
│   │       └── title.txt        # 제목 생성 프롬프트
│   ├── auth/                # 인증 시스템
│   │   └── index.ts         # OAuth, API 키 관리
│   ├── bun/                 # Bun 런타임 유틸리티
│   │   └── index.ts         # Bun 프로세스 관리
│   ├── bus/                 # 이벤트 버스
│   │   ├── index.ts         # 이벤트 버스 구현
│   │   └── bus-event.ts     # 이벤트 정의
│   ├── cli/                 # CLI 인터페이스
│   │   ├── bootstrap.ts     # CLI 부트스트랩
│   │   ├── cmd/             # CLI 명령어들
│   │   │   ├── acp.ts       # ACP 명령어
│   │   │   ├── agent.ts     # 에이전트 관리
│   │   │   ├── auth.ts      # 인증 명령어
│   │   │   ├── debug/       # 디버그 명령어들
│   │   │   ├── export.ts    # 세션 내보내기
│   │   │   ├── generate.ts  # 코드 생성
│   │   │   ├── github.ts    # GitHub 통합
│   │   │   ├── import.ts    # 세션 가져오기
│   │   │   ├── mcp.ts       # MCP 관리
│   │   │   ├── models.ts    # 모델 목록
│   │   │   ├── pr.ts        # PR 명령어
│   │   │   ├── run.ts       # 단일 실행
│   │   │   ├── serve.ts     # API 서버
│   │   │   ├── session.ts   # 세션 관리
│   │   │   ├── stats.ts     # 통계
│   │   │   ├── tui/         # 터미널 UI
│   │   │   ├── uninstall.ts # 제거
│   │   │   ├── upgrade.ts   # 업그레이드
│   │   │   └── web.ts       # 웹 인터페이스
│   │   ├── error.ts         # 에러 포맷팅
│   │   ├── network.ts       # 네트워크 유틸
│   │   ├── ui.ts            # CLI UI 헬퍼
│   │   └── upgrade.ts       # 업그레이드 로직
│   ├── command/             # 커스텀 명령어 시스템
│   │   └── index.ts         # 명령어 레지스트리
│   ├── config/              # 설정 관리
│   │   └── config.ts        # 설정 로드/저장
│   ├── env/                 # 환경 변수
│   │   └── index.ts         # 환경 변수 관리
│   ├── file/                # 파일 시스템 유틸
│   │   └── index.ts         # 파일 읽기/쓰기
│   ├── flag/                # 기능 플래그
│   │   └── flag.ts          # 플래그 정의
│   ├── format/              # 코드 포맷터
│   │   └── index.ts         # 포맷터 통합
│   ├── global/              # 전역 상태
│   │   └── index.ts         # 전역 경로, 설정
│   ├── id/                  # ID 생성
│   │   └── index.ts         # ULID 기반 ID
│   ├── ide/                 # IDE 통합
│   │   └── index.ts         # VS Code, Cursor 등
│   ├── installation/        # 설치 관리
│   │   └── index.ts         # 버전, 경로 관리
│   ├── lsp/                 # Language Server Protocol
│   │   ├── index.ts         # LSP 메인 모듈
│   │   ├── client.ts        # LSP 클라이언트
│   │   ├── language.ts      # 언어별 설정
│   │   └── server.ts        # LSP 서버 정의
│   ├── mcp/                 # Model Context Protocol
│   │   ├── index.ts         # MCP 클라이언트
│   │   ├── auth.ts          # MCP 인증
│   │   ├── oauth-callback.ts # OAuth 콜백
│   │   └── oauth-provider.ts # OAuth 프로바이더
│   ├── patch/               # 패치 유틸
│   │   └── index.ts         # 코드 패치 적용
│   ├── permission/          # 권한 시스템
│   │   └── next.ts          # 권한 규칙 엔진
│   ├── plugin/              # 플러그인 시스템
│   │   └── index.ts         # 플러그인 로더
│   ├── project/             # 프로젝트 관리
│   │   ├── instance.ts      # 프로젝트 인스턴스
│   │   ├── bootstrap.ts     # 부트스트랩
│   │   └── vcs.ts           # 버전 관리 통합
│   ├── provider/            # LLM 프로바이더
│   │   ├── provider.ts      # 프로바이더 추상화
│   │   ├── auth.ts          # 프로바이더 인증
│   │   ├── models.ts        # 모델 정의
│   │   ├── transform.ts     # 응답 변환
│   │   └── sdk/             # 커스텀 SDK
│   ├── pty/                 # 터미널 에뮬레이션
│   │   └── index.ts         # PTY 관리
│   ├── question/            # 질문 시스템
│   │   └── index.ts         # 사용자 질문 처리
│   ├── scheduler/           # 작업 스케줄러
│   │   └── index.ts         # 비동기 작업 관리
│   ├── server/              # API 서버
│   │   ├── server.ts        # Hono 기반 서버
│   │   ├── error.ts         # 에러 처리
│   │   ├── event.ts         # SSE 이벤트
│   │   ├── mdns.ts          # mDNS 디스커버리
│   │   └── routes/          # API 라우트
│   │       ├── config.ts    # 설정 API
│   │       ├── experimental.ts # 실험적 기능
│   │       ├── file.ts      # 파일 API
│   │       ├── global.ts    # 전역 API
│   │       ├── mcp.ts       # MCP API
│   │       ├── permission.ts # 권한 API
│   │       ├── project.ts   # 프로젝트 API
│   │       ├── provider.ts  # 프로바이더 API
│   │       ├── pty.ts       # PTY API
│   │       ├── question.ts  # 질문 API
│   │       ├── session.ts   # 세션 API
│   │       └── tui.ts       # TUI API
│   ├── session/             # 세션 관리
│   │   ├── index.ts         # 세션 CRUD
│   │   ├── compaction.ts    # 컨텍스트 압축
│   │   ├── instruction.ts   # 지시사항 처리
│   │   ├── llm.ts           # LLM 호출
│   │   ├── message.ts       # 메시지 포맷
│   │   ├── message-v2.ts    # 메시지 v2 포맷
│   │   ├── processor.ts     # 메시지 처리
│   │   ├── prompt.ts        # 프롬프트 생성
│   │   ├── retry.ts         # 재시도 로직
│   │   ├── revert.ts        # 되돌리기
│   │   ├── status.ts        # 상태 관리
│   │   ├── summary.ts       # 요약 생성
│   │   ├── system.ts        # 시스템 프롬프트
│   │   └── todo.ts          # TODO 관리
│   ├── share/               # 세션 공유
│   │   └── index.ts         # 공유 기능
│   ├── shell/               # 쉘 통합
│   │   └── index.ts         # 쉘 명령어 실행
│   ├── skill/               # 스킬 시스템
│   │   └── skill.ts         # 스킬 정의/실행
│   ├── snapshot/            # 스냅샷 관리
│   │   └── index.ts         # 파일 스냅샷
│   ├── storage/             # 스토리지
│   │   └── storage.ts       # 로컬 스토리지
│   ├── tool/                # AI 도구들
│   │   ├── tool.ts          # 도구 인터페이스
│   │   ├── registry.ts      # 도구 레지스트리
│   │   ├── truncation.ts    # 출력 truncation
│   │   ├── apply_patch.ts   # 패치 적용 도구
│   │   ├── bash.ts          # 쉘 명령어 도구
│   │   ├── batch.ts         # 배치 실행 도구
│   │   ├── codesearch.ts    # 코드 검색 도구
│   │   ├── edit.ts          # 파일 편집 도구
│   │   ├── glob.ts          # 파일 패턴 도구
│   │   ├── grep.ts          # 텍스트 검색 도구
│   │   ├── ls.ts            # 디렉토리 목록 도구
│   │   ├── lsp.ts           # LSP 도구
│   │   ├── multiedit.ts     # 다중 편집 도구
│   │   ├── plan.ts          # 계획 모드 도구
│   │   ├── question.ts      # 질문 도구
│   │   ├── read.ts          # 파일 읽기 도구
│   │   ├── skill.ts         # 스킬 호출 도구
│   │   ├── task.ts          # 작업 도구
│   │   ├── todo.ts          # TODO 관리 도구
│   │   ├── webfetch.ts      # 웹 페이지 가져오기
│   │   ├── websearch.ts     # 웹 검색 도구
│   │   ├── write.ts         # 파일 쓰기 도구
│   │   └── *.txt            # 각 도구의 프롬프트
│   ├── util/                # 유틸리티
│   │   ├── log.ts           # 로깅
│   │   ├── iife.ts          # IIFE 헬퍼
│   │   ├── lazy.ts          # 지연 초기화
│   │   └── timeout.ts       # 타임아웃
│   ├── worktree/            # Git worktree
│   │   └── index.ts         # worktree 관리
│   └── index.ts             # 메인 진입점
├── test/                    # 테스트 파일
│   ├── acp/                 # ACP 테스트
│   ├── agent/               # 에이전트 테스트
│   ├── cli/                 # CLI 테스트
│   ├── config/              # 설정 테스트
│   ├── file/                # 파일 테스트
│   ├── fixture/             # 테스트 픽스처
│   ├── ide/                 # IDE 테스트
│   ├── lsp/                 # LSP 테스트
│   ├── mcp/                 # MCP 테스트
│   ├── patch/               # 패치 테스트
│   ├── permission/          # 권한 테스트
│   ├── plugin/              # 플러그인 테스트
│   ├── project/             # 프로젝트 테스트
│   ├── provider/            # 프로바이더 테스트
│   ├── question/            # 질문 테스트
│   ├── server/              # 서버 테스트
│   ├── session/             # 세션 테스트
│   ├── skill/               # 스킬 테스트
│   ├── snapshot/            # 스냅샷 테스트
│   ├── tool/                # 도구 테스트
│   ├── util/                # 유틸 테스트
│   └── preload.ts           # 테스트 프리로드
├── .gitignore
├── AGENTS.md                # 에이전트 가이드라인
├── bunfig.toml              # Bun 설정
├── Dockerfile               # Docker 이미지
├── package.json
├── parsers-config.ts        # Tree-sitter 파서 설정
├── README.md
└── tsconfig.json
```

### 4.2 `packages/app/` - 🌐 웹 UI

```
packages/app/
├── e2e/                     # E2E 테스트 (Playwright)
│   ├── context.spec.ts      # 컨텍스트 테스트
│   ├── file-open.spec.ts    # 파일 열기 테스트
│   ├── file-tree.spec.ts    # 파일 트리 테스트
│   ├── file-viewer.spec.ts  # 파일 뷰어 테스트
│   ├── home.spec.ts         # 홈 페이지 테스트
│   ├── model-picker.spec.ts # 모델 선택 테스트
│   ├── navigation.spec.ts   # 네비게이션 테스트
│   ├── palette.spec.ts      # 명령어 팔레트 테스트
│   ├── prompt.spec.ts       # 프롬프트 테스트
│   ├── session.spec.ts      # 세션 테스트
│   ├── settings.spec.ts     # 설정 테스트
│   ├── sidebar.spec.ts      # 사이드바 테스트
│   ├── terminal.spec.ts     # 터미널 테스트
│   └── fixtures.ts          # 테스트 픽스처
├── public/                  # 정적 파일
│   ├── favicon.*            # 파비콘 (심링크)
│   └── oc-theme-preload.js  # 테마 프리로드
├── script/
│   └── e2e-local.ts         # 로컬 E2E 스크립트
├── src/
│   ├── addons/              # 추가 기능
│   │   └── terminal/        # 터미널 애드온
│   ├── components/          # UI 컴포넌트
│   │   ├── app-theme.tsx    # 테마 컴포넌트
│   │   ├── breadcrumbs.tsx  # 브레드크럼
│   │   ├── command-dialog.tsx # 명령어 다이얼로그
│   │   ├── context-dialog.tsx # 컨텍스트 다이얼로그
│   │   ├── diff-viewer.tsx  # 디프 뷰어
│   │   ├── file-tree.tsx    # 파일 트리
│   │   ├── file-viewer.tsx  # 파일 뷰어
│   │   ├── header.tsx       # 헤더
│   │   ├── layout.tsx       # 레이아웃
│   │   ├── mcp-dialog.tsx   # MCP 다이얼로그
│   │   ├── model-picker.tsx # 모델 선택기
│   │   ├── permission-dialog.tsx # 권한 다이얼로그
│   │   ├── prompt.tsx       # 프롬프트 입력
│   │   ├── session-list.tsx # 세션 목록
│   │   ├── settings.tsx     # 설정 화면
│   │   ├── sidebar.tsx      # 사이드바
│   │   ├── terminal.tsx     # 터미널
│   │   └── ...
│   ├── context/             # React Context
│   │   ├── app.tsx          # 앱 컨텍스트
│   │   ├── sdk.tsx          # SDK 컨텍스트
│   │   └── theme.tsx        # 테마 컨텍스트
│   ├── hooks/               # 커스텀 훅
│   │   ├── use-keyboard.ts  # 키보드 훅
│   │   ├── use-session.ts   # 세션 훅
│   │   └── ...
│   ├── i18n/                # 국제화
│   │   ├── index.ts         # i18n 설정
│   │   └── locales/         # 언어별 번역
│   ├── pages/               # 페이지 컴포넌트
│   │   ├── home.tsx         # 홈 페이지
│   │   ├── session.tsx      # 세션 페이지
│   │   └── settings.tsx     # 설정 페이지
│   ├── utils/               # 유틸리티
│   ├── app.tsx              # 메인 앱 컴포넌트
│   ├── entry.tsx            # 진입점
│   ├── index.css            # 글로벌 스타일
│   └── index.ts             # 내보내기
├── index.html               # HTML 템플릿
├── package.json
├── playwright.config.ts     # Playwright 설정
├── vite.config.ts           # Vite 설정
└── tsconfig.json
```

### 4.3 `packages/desktop/` - 🖥️ 데스크톱 앱 (Tauri)

```
packages/desktop/
├── scripts/
│   ├── copy-bundles.ts      # 번들 복사
│   ├── predev.ts            # 개발 전 설정
│   ├── prepare.ts           # 빌드 준비
│   └── utils.ts             # 유틸리티
├── src/
│   ├── cli.ts               # CLI 인터페이스
│   ├── index.tsx            # 앱 진입점
│   ├── menu.ts              # 앱 메뉴
│   ├── styles.css           # 스타일
│   ├── updater.ts           # 자동 업데이트
│   └── webview-zoom.ts      # 웹뷰 줌
├── src-tauri/               # Rust 네이티브 코드
│   ├── assets/              # NSIS 설치 이미지
│   ├── capabilities/        # Tauri 권한
│   │   └── default.json     # 기본 권한
│   ├── icons/               # 앱 아이콘
│   │   ├── dev/             # 개발용 아이콘
│   │   └── prod/            # 프로덕션 아이콘
│   ├── release/             # 릴리스 메타데이터
│   │   └── appstream.metainfo.xml
│   ├── src/
│   │   ├── main.rs          # Rust 진입점
│   │   ├── lib.rs           # 라이브러리
│   │   ├── cli.rs           # CLI 처리
│   │   ├── job_object.rs    # Windows 작업 객체
│   │   ├── markdown.rs      # 마크다운 처리
│   │   └── window_customizer.rs # 윈도우 커스텀
│   ├── build.rs             # 빌드 스크립트
│   ├── Cargo.toml           # Rust 의존성
│   ├── tauri.conf.json      # 개발 설정
│   └── tauri.prod.conf.json # 프로덕션 설정
├── index.html
├── package.json
├── vite.config.ts
└── tsconfig.json
```

### 4.4 `packages/ui/` - 🎨 공유 UI 컴포넌트

```
packages/ui/
├── script/
│   ├── colors.txt           # 색상 정의
│   └── tailwind.ts          # Tailwind 설정 생성
├── src/
│   ├── assets/              # 정적 자산
│   │   ├── audio/           # 사운드 파일
│   │   ├── favicon/         # 파비콘
│   │   ├── fonts/           # 폰트 파일
│   │   └── images/          # 이미지
│   ├── components/          # 공유 컴포넌트
│   │   ├── accordion.tsx    # 아코디언
│   │   ├── avatar.tsx       # 아바타
│   │   ├── badge.tsx        # 뱃지
│   │   ├── button.tsx       # 버튼
│   │   ├── checkbox.tsx     # 체크박스
│   │   ├── code-block.tsx   # 코드 블록
│   │   ├── dialog.tsx       # 다이얼로그
│   │   ├── dropdown.tsx     # 드롭다운
│   │   ├── input.tsx        # 인풋
│   │   ├── markdown.tsx     # 마크다운 렌더러
│   │   ├── select.tsx       # 셀렉트
│   │   ├── tabs.tsx         # 탭
│   │   ├── toast.tsx        # 토스트
│   │   ├── tooltip.tsx      # 툴팁
│   │   ├── file-icons/      # 파일 아이콘
│   │   └── provider-icons/  # 프로바이더 아이콘
│   ├── context/             # 컨텍스트
│   │   └── index.tsx        # 공통 컨텍스트
│   ├── hooks/               # 공통 훅
│   │   └── index.ts
│   ├── i18n/                # 국제화
│   │   └── *.ts             # 언어별 번역
│   ├── pierre/              # Pierre 렌더링 엔진
│   │   ├── index.ts         # 메인
│   │   ├── diff.ts          # 디프 렌더링
│   │   └── syntax.ts        # 구문 강조
│   ├── styles/              # 스타일
│   │   ├── index.css        # 메인 스타일
│   │   └── tailwind/        # Tailwind 설정
│   └── theme/               # 테마 시스템
│       ├── index.ts         # 테마 정의
│       └── context.tsx      # 테마 컨텍스트
├── package.json
├── vite.config.ts
└── tsconfig.json
```

### 4.5 `packages/sdk/` - 📦 JavaScript SDK

```
packages/sdk/
├── js/
│   ├── example/
│   │   └── example.ts       # 사용 예제
│   ├── script/
│   │   ├── build.ts         # 빌드 스크립트
│   │   └── publish.ts       # 배포 스크립트
│   ├── src/
│   │   ├── gen/             # 자동 생성된 코드
│   │   │   └── *.ts         # OpenAPI 기반 클라이언트
│   │   ├── v2/              # v2 API
│   │   │   ├── client.ts    # v2 클라이언트
│   │   │   └── index.ts
│   │   ├── client.ts        # 클라이언트
│   │   ├── index.ts         # 진입점
│   │   └── server.ts        # 서버 타입
│   ├── package.json
│   └── tsconfig.json
└── openapi.json             # OpenAPI 스펙
```

### 4.6 `packages/console/` - ☁️ 관리 콘솔

```
packages/console/
├── app/                     # 콘솔 웹 앱
│   ├── src/                 # SolidJS 앱
│   ├── public/              # 정적 파일
│   └── package.json
├── core/                    # 비즈니스 로직
│   ├── migrations/          # DB 마이그레이션
│   ├── src/                 # 핵심 로직
│   │   ├── account/         # 계정 관리
│   │   ├── billing/         # 결제 처리
│   │   ├── subscription/    # 구독 관리
│   │   └── user/            # 사용자 관리
│   └── drizzle.config.ts    # Drizzle ORM 설정
├── function/                # 서버리스 함수
│   └── src/
│       └── auth.ts          # 인증 함수
├── mail/                    # 이메일 템플릿
│   └── emails/              # React Email 템플릿
└── resource/                # 리소스 정의
    ├── resource.cloudflare.ts # Cloudflare 리소스
    └── resource.node.ts     # Node 리소스
```

### 4.7 `packages/plugin/` - 🔌 플러그인 API

```
packages/plugin/
├── script/
│   └── publish.ts           # 배포 스크립트
├── src/
│   ├── index.ts             # 플러그인 API
│   └── tool.ts              # 도구 정의 API
├── package.json
└── tsconfig.json
```

### 4.8 기타 패키지

| 패키지 | 역할 |
|--------|------|
| `packages/web/` | 문서 & 랜딩 페이지 (Astro) |
| `packages/docs/` | API 문서 (MDX) |
| `packages/function/` | 서버리스 API 함수 |
| `packages/util/` | 공통 유틸리티 |
| `packages/slack/` | Slack 통합 |
| `packages/enterprise/` | 엔터프라이즈 기능 |
| `packages/extensions/` | 에디터 확장 (Zed) |
| `packages/identity/` | 아이덴티티 관리 |

---

## 5. TUI (터미널 UI) 상세 구조

```
packages/opencode/src/cli/cmd/tui/
├── component/               # UI 컴포넌트
│   ├── prompt/              # 프롬프트 컴포넌트
│   │   ├── index.tsx        # 메인 프롬프트
│   │   ├── autocomplete.tsx # 자동완성
│   │   ├── frecency.tsx     # 최근 사용 기록
│   │   ├── history.tsx      # 명령어 히스토리
│   │   └── stash.tsx        # 스태시 기능
│   ├── border.tsx           # 테두리
│   ├── dialog-agent.tsx     # 에이전트 선택 다이얼로그
│   ├── dialog-command.tsx   # 명령어 다이얼로그
│   ├── dialog-mcp.tsx       # MCP 다이얼로그
│   ├── dialog-model.tsx     # 모델 선택 다이얼로그
│   ├── dialog-provider.tsx  # 프로바이더 다이얼로그
│   ├── dialog-session-list.tsx  # 세션 목록
│   ├── dialog-session-rename.tsx # 세션 이름 변경
│   ├── dialog-stash.tsx     # 스태시 다이얼로그
│   ├── dialog-status.tsx    # 상태 다이얼로그
│   ├── dialog-tag.tsx       # 태그 다이얼로그
│   ├── dialog-theme-list.tsx # 테마 목록
│   ├── logo.tsx             # 로고 렌더링
│   ├── textarea-keybindings.ts # 키바인딩
│   ├── tips.tsx             # 팁 표시
│   └── todo-item.tsx        # TODO 아이템
├── context/                 # 컨텍스트
│   ├── theme/               # 테마 JSON 파일들 (32개)
│   │   ├── opencode.json    # 기본 테마
│   │   ├── catppuccin.json  # Catppuccin 테마
│   │   ├── dracula.json     # Dracula 테마
│   │   ├── gruvbox.json     # Gruvbox 테마
│   │   ├── nord.json        # Nord 테마
│   │   ├── tokyonight.json  # Tokyo Night 테마
│   │   └── ...              # 기타 테마들
│   ├── args.tsx             # 인자 컨텍스트
│   ├── directory.ts         # 디렉토리 컨텍스트
│   ├── exit.tsx             # 종료 컨텍스트
│   ├── helper.tsx           # 헬퍼 컨텍스트
│   ├── keybind.tsx          # 키바인딩 컨텍스트
│   ├── kv.tsx               # KV 스토어 컨텍스트
│   ├── local.tsx            # 로컬 상태 컨텍스트
│   ├── prompt.tsx           # 프롬프트 컨텍스트
│   ├── route.tsx            # 라우팅 컨텍스트
│   ├── sdk.tsx              # SDK 컨텍스트
│   ├── sync.tsx             # 동기화 컨텍스트
│   └── theme.tsx            # 테마 컨텍스트
├── routes/                  # 라우트
│   ├── session/             # 세션 라우트
│   │   ├── index.tsx        # 메인 세션 뷰
│   │   ├── header.tsx       # 세션 헤더
│   │   ├── footer.tsx       # 세션 푸터
│   │   ├── sidebar.tsx      # 세션 사이드바
│   │   ├── permission.tsx   # 권한 요청
│   │   ├── question.tsx     # 질문 처리
│   │   ├── dialog-*.tsx     # 세션 다이얼로그들
│   │   └── ...
│   └── home.tsx             # 홈 라우트
├── ui/                      # 공통 UI
│   ├── dialog.tsx           # 다이얼로그 베이스
│   ├── dialog-alert.tsx     # 알림 다이얼로그
│   ├── dialog-confirm.tsx   # 확인 다이얼로그
│   ├── dialog-export-options.tsx # 내보내기 옵션
│   ├── dialog-help.tsx      # 도움말
│   ├── dialog-prompt.tsx    # 프롬프트 다이얼로그
│   ├── dialog-select.tsx    # 선택 다이얼로그
│   ├── link.tsx             # 링크
│   ├── spinner.ts           # 스피너
│   └── toast.tsx            # 토스트
├── util/                    # 유틸리티
│   ├── clipboard.ts         # 클립보드
│   ├── editor.ts            # 에디터 연동
│   ├── signal.ts            # 시그널 처리
│   ├── terminal.ts          # 터미널 유틸
│   └── transcript.ts        # 트랜스크립트
├── app.tsx                  # TUI 앱 진입점
├── attach.ts                # 세션 연결
├── event.ts                 # 이벤트 처리
├── thread.ts                # 스레드 명령어
└── worker.ts                # 워커 스레드
```

---

## 6. 도구(Tool) 상세 설명

| 도구 | 파일 | 역할 |
|------|------|------|
| **bash** | `bash.ts` | 쉘 명령어 실행 |
| **read** | `read.ts` | 파일 내용 읽기 |
| **write** | `write.ts` | 파일 생성/덮어쓰기 |
| **edit** | `edit.ts` | 파일 부분 수정 |
| **multiedit** | `multiedit.ts` | 다중 파일 수정 |
| **glob** | `glob.ts` | 파일 패턴 검색 |
| **grep** | `grep.ts` | 텍스트 검색 |
| **ls** | `ls.ts` | 디렉토리 목록 |
| **lsp** | `lsp.ts` | 언어 서버 쿼리 |
| **apply_patch** | `apply_patch.ts` | 패치 적용 |
| **codesearch** | `codesearch.ts` | 시맨틱 코드 검색 |
| **webfetch** | `webfetch.ts` | 웹 페이지 가져오기 |
| **websearch** | `websearch.ts` | 웹 검색 |
| **question** | `question.ts` | 사용자에게 질문 |
| **task** | `task.ts` | 서브에이전트 실행 |
| **todo** | `todo.ts` | TODO 목록 관리 |
| **batch** | `batch.ts` | 배치 실행 |
| **plan** | `plan.ts` | 계획 모드 전환 |
| **skill** | `skill.ts` | 스킬 호출 |

---

## 7. 지원 프로바이더

```typescript
// packages/opencode/src/provider/provider.ts

const BUNDLED_PROVIDERS = {
  "@ai-sdk/amazon-bedrock": createAmazonBedrock,
  "@ai-sdk/anthropic": createAnthropic,
  "@ai-sdk/azure": createAzure,
  "@ai-sdk/google": createGoogleGenerativeAI,
  "@ai-sdk/google-vertex": createVertex,
  "@ai-sdk/google-vertex/anthropic": createVertexAnthropic,
  "@ai-sdk/openai": createOpenAI,
  "@ai-sdk/openai-compatible": createOpenAICompatible,
  "@openrouter/ai-sdk-provider": createOpenRouter,
  "@ai-sdk/xai": createXai,
  "@ai-sdk/mistral": createMistral,
  "@ai-sdk/groq": createGroq,
  "@ai-sdk/deepinfra": createDeepInfra,
  "@ai-sdk/cerebras": createCerebras,
  "@ai-sdk/cohere": createCohere,
  "@ai-sdk/gateway": createGateway,
  "@ai-sdk/togetherai": createTogetherAI,
  "@ai-sdk/perplexity": createPerplexity,
  "@ai-sdk/vercel": createVercel,
  "@gitlab/gitlab-ai-provider": createGitLab,
  "@ai-sdk/github-copilot": createGitHubCopilotOpenAICompatible,
}
```

---

## 8. 프로젝트 통계

| 항목 | 수치 |
|------|------|
| TypeScript 파일 | 784개 |
| 핵심 코드 라인 | ~40,626줄 |
| 패키지 수 | 16+ |
| 지원 AI 프로바이더 | 20+ |
| 지원 언어 (README) | 15개 |
| TUI 테마 | 32개 |
| AI 도구 | 19개 |
| 내장 에이전트 | 6개 |

---

## 9. 코드 스타일 가이드

> 출처: [AGENTS.md](AGENTS.md)

### 핵심 원칙

- **한 함수 내 유지**: 재사용/조합이 필요한 경우에만 분리
- **불필요한 구조 분해 금지**: `const { a, b } = obj` 대신 `obj.a`, `obj.b` 사용
- **try/catch 회피**: 가능하면 `.catch(...)` 패턴 사용
- **`any` 타입 금지**: 정확한 타입 사용
- **단일 단어 변수명 선호**: 가능하면 `foo`, `bar` 사용
- **Bun API 사용**: `Bun.file()`, `Bun.write()` 등 활용
- **타입 추론 의존**: 불필요한 타입 어노테이션 회피

### let 문 회피

```ts
// ✅ Good
const foo = condition ? 1 : 2

// ❌ Bad
let foo
if (condition) foo = 1
else foo = 2
```

### else 문 회피

```ts
// ✅ Good - early return
function foo() {
  if (condition) return 1
  return 2
}

// ❌ Bad
function foo() {
  if (condition) return 1
  else return 2
}
```

### 테스트 원칙

- **Mock 최소화**: 가능하면 실제 구현 테스트
- **로직 중복 금지**: 테스트에 구현 로직을 복제하지 않음

---

## 10. 아키텍처 패턴

> 출처: [packages/opencode/AGENTS.md](packages/opencode/AGENTS.md)

### 핵심 패턴

| 패턴 | 설명 |
|------|------|
| **Tool** | `Tool.Info` 인터페이스 구현, `execute()` 메서드 |
| **Context** | `sessionID`를 도구 컨텍스트로 전달, `App.provide()` 사용 |
| **Validation** | 모든 입력을 Zod 스키마로 검증 |
| **Logging** | `Log.create({ service: "name" })` 패턴 |
| **Storage** | `Storage` 네임스페이스로 영속화 |
| **Namespace** | 네임스페이스 기반 조직 (`Tool.define()`, `Session.create()`) |

### API 클라이언트 통신

TypeScript TUI(SolidJS + OpenTUI)는 `@opencode-ai/sdk`를 통해 OpenCode 서버와 통신합니다. 서버 엔드포인트 추가/수정 시 `./script/generate.ts`를 실행하여 SDK를 재생성해야 합니다.

---

## 11. ACP (Agent Client Protocol)

> 출처: [packages/opencode/src/acp/README.md](packages/opencode/src/acp/README.md)

ACP는 외부 에디터(Zed 등)와 OpenCode를 통합하는 프로토콜입니다.

### 아키텍처

```
┌─────────────┐     JSON-RPC     ┌─────────────┐
│   Zed IDE   │ ◄──────────────► │   OpenCode  │
│  (Client)   │     stdio        │  ACP Server │
└─────────────┘                  └─────────────┘
```

### 핵심 컴포넌트

| 컴포넌트 | 파일 | 역할 |
|----------|------|------|
| **Agent** | `agent.ts` | ACP 프로토콜 인터페이스, 세션 관리 |
| **Client** | `client.ts` | 파일 작업, 권한 요청 |
| **Session** | `session.ts` | ACP ↔ OpenCode 세션 매핑 |
| **Server** | `server.ts` | JSON-RPC over stdio 서버 |

### Zed 통합 설정

```json
// ~/.config/zed/settings.json
{
  "agent_servers": {
    "OpenCode": {
      "command": "opencode",
      "args": ["acp"]
    }
  }
}
```

### 프로토콜 지원

- ✅ `initialize` - 프로토콜 버전 협상
- ✅ `session/new` - 새 대화 세션 생성
- ✅ `session/load` - 기존 세션 복원
- ✅ `session/prompt` - 사용자 메시지 처리
- ✅ 파일 읽기/쓰기 작업
- 🔄 스트리밍 응답 (진행 중)

---

## 12. GitHub Action 통합

> 출처: [github/README.md](github/README.md)

GitHub PR/이슈에서 `/opencode` 또는 `/oc` 명령어로 AI 에이전트를 호출할 수 있습니다.

### 사용 예시

```bash
# 이슈 설명 요청
/opencode explain this issue

# 이슈 수정 요청 (새 브랜치 생성 + PR 오픈)
/opencode fix this

# PR 코드 리뷰 및 수정 요청
Delete the attachment from S3 when the note is removed /oc

# 특정 코드 라인에 댓글
/oc add error handling here
```

### 설치

```bash
opencode github install
```

이 명령어는 GitHub App 설치, 워크플로우 생성, 시크릿 설정을 안내합니다.

---

## 13. 성능 최적화 로드맵

> 출처: [specs/perf-roadmap.md](specs/perf-roadmap.md)

### Phase 0: 기준선 + 플래그 준비

- 기능 플래그 인프라 구축
- 개발용 카운터/로그 추가

### Phase 1: "Jank Generator" 제거

- 파일 검색 디바운스 + 오래된 결과 보호
- 영속화 페이로드 크기 체크
- 프롬프트 히스토리 이미지 dataUrl 제거

### Phase 2: 메모리 성장 제한

- 공유 LRU/TTL 캐시 헬퍼 도입
- 파일 내용 캐시에 eviction 적용
- global-sync 디렉토리별 자식 스토어 eviction

### Phase 3: 대형 세션 스크롤 확장성

- scroll-spy 로직 모듈 분리
- IntersectionObserver 기반 추적 구현
- 비관찰자 환경용 이진 검색 폴백

### Phase 4: 모듈화 및 중복 제거

- 공유 scoped-cache 유틸리티 도입
- mega-component 분할 (session.tsx, prompt-input.tsx)

---

## 14. 기술 명세서 요약

### 14.1 페이로드 제한 (`specs/01-persist-payload-limits.md`)

**문제**: 큰 페이로드(base64 이미지, 터미널 버퍼)가 localStorage에 저장되어 메인 스레드 차단

**해결책**:
- per-key 영속화 정책 (`warnBytes`, `maxBytes`)
- 대용량 데이터용 Blob 스토어 (웹: IndexedDB, 데스크톱: 파일시스템)
- 이미지를 참조로 저장 (dataUrl 대신 blobID)

### 14.2 캐시 Eviction (`specs/02-cache-eviction.md`)

**문제**: 인메모리 캐시가 제한 없이 증가

**해결책**:
```ts
type CacheOpts = {
  maxEntries: number
  ttlMs?: number
  maxBytes?: number
  sizeOf?: (value: unknown) => number
}

function createLruCache<T>(opts: CacheOpts) {
  // get, set, delete, clear, evictExpired, stats
}
```

적용 대상: 파일 내용 캐시, global-sync 디렉토리 스토어, 세션/메시지 캐시

### 14.3 요청 쓰로틀링 (`specs/03-request-throttling.md`)

**문제**: 빠른 타이핑 시 서버 요청 폭주

**해결책**:
```ts
function createLatestOnlyAsync<TArgs extends unknown[], TResult>(
  fn: (args: { input: TArgs; signal?: AbortSignal }) => Promise<TResult>,
) {
  let id = 0
  let controller: AbortController | undefined

  return async (...input: TArgs) => {
    id += 1
    const current = id
    controller?.abort()
    controller = new AbortController()

    const result = await fn({ input, signal: controller.signal })
    if (current !== id) return
    return result
  }
}
```

### 14.4 Scroll Spy 최적화 (`specs/04-scroll-spy-optimization.md`)

**문제**: `querySelectorAll('[data-message-id]')` 반복 호출로 O(N) DOM 스캔

**해결책**:
- IntersectionObserver로 가시성 추적
- 사전 계산된 오프셋으로 이진 검색 폴백
- ResizeObserver로 레이아웃 변경 감지

### 14.5 컴포넌트 모듈화 (`specs/05-modularize-and-dedupe.md`)

**대상 파일**:
- `packages/app/src/pages/session.tsx` → 800 LOC 미만으로
- `packages/app/src/components/prompt-input.tsx` → 900 LOC 미만으로

**공유 유틸리티**:
```ts
type ScopedOpts = { maxEntries?: number; ttlMs?: number }

function createScopedCache<T>(createValue: (key: string) => T, opts: ScopedOpts) {
  // store + eviction + dispose hooks
}
```

---

## 15. 멀티 프로젝트 세션 API

> 출처: [specs/project.md](specs/project.md)

OpenCode는 단일 인스턴스에서 여러 프로젝트와 worktree별 세션을 관리할 수 있습니다.

### 주요 API 엔드포인트

```
GET  /project                              # 프로젝트 목록
POST /project/init                         # 프로젝트 초기화

GET  /project/:projectID/session           # 세션 목록
POST /project/:projectID/session           # 새 세션 생성
GET  /project/:projectID/session/:id       # 세션 조회
DELETE /project/:projectID/session/:id     # 세션 삭제

POST /project/:projectID/session/:id/init  # 세션 초기화
POST /project/:projectID/session/:id/abort # 세션 중단
POST /project/:projectID/session/:id/share # 세션 공유
POST /project/:projectID/session/:id/compact # 컨텍스트 압축

GET  /project/:projectID/session/:id/message     # 메시지 목록
POST /project/:projectID/session/:id/message     # 새 메시지
POST /project/:projectID/session/:id/revert      # 되돌리기
POST /project/:projectID/session/:id/permission  # 권한 처리
```

---

## 16. 커스텀 확장 시스템

### 16.1 커스텀 에이전트 (`.opencode/agent/`)

```yaml
# .opencode/agent/triage.md
---
mode: primary
hidden: true
model: opencode/claude-haiku-4-5
color: "#44BA81"
tools:
  "*": false
  "github-triage": true
---

You are a triage agent responsible for triaging github issues.
```

### 16.2 커스텀 명령어 (`.opencode/command/`)

```yaml
# .opencode/command/commit.md
---
description: git commit and push
model: opencode/glm-4.7
subtask: true
---

commit and push with appropriate prefix (docs:, tui:, core:, ci:, etc.)

## GIT DIFF
!`git diff`
```

### 16.3 커스텀 스킬 (`.opencode/skill/`)

```yaml
# .opencode/skill/bun-file-io/SKILL.md
---
name: bun-file-io
description: Use this when working on file operations
---

## Bun file APIs
- `Bun.file(path)` is lazy; call `text`, `json`, `exists` to read
- `Bun.write(dest, input)` writes strings, buffers, Blobs
- `Bun.Glob` + `Array.fromAsync(glob.scan(...))` for scans
```

---

## 17. 개발 환경 설정

### 로컬 개발

```bash
# 백엔드 + TUI
bun dev

# 웹 UI 개발 (별도 터미널)
# 1. 백엔드 시작
bun run --cwd packages/opencode --conditions=browser ./src/index.ts serve --port 4096

# 2. 앱 개발 서버 시작
bun run --cwd packages/app dev -- --port 4444

# 3. http://localhost:4444 에서 확인
```

### 데스크톱 앱 개발

```bash
# Tauri 개발 모드 (네이티브 윈도우)
bun run --cwd packages/desktop tauri dev

# 웹 개발 서버만 (네이티브 쉘 없이)
bun run --cwd packages/desktop dev

# 프로덕션 빌드
bun run --cwd packages/desktop tauri build
```

### 디버거 설정

```bash
# 서버 디버깅
bun run --inspect=ws://localhost:6499/ --cwd packages/opencode ./src/index.ts serve --port 4096

# TUI 디버깅
bun run --inspect=ws://localhost:6499/ --cwd packages/opencode --conditions=browser ./src/index.ts
```

---

## 18. E2E 테스트

> 출처: [specs/08-app-e2e-smoke-suite.md](specs/08-app-e2e-smoke-suite.md)

### 실행 방법

```bash
bunx playwright install
bun run --cwd packages/app test:e2e:local
bun run --cwd packages/app test:e2e:local -- --grep "settings"
```

### 테스트 커버리지

| 테스트 | 파일 |
|--------|------|
| Settings dialog | `settings.spec.ts` |
| Prompt `/open` | `prompt-slash-open.spec.ts` |
| Prompt `@mention` | `prompt-mention.spec.ts` |
| Model picker | `model-picker.spec.ts` |
| File viewer | `file-viewer.spec.ts` |
| Terminal init | `terminal-init.spec.ts` |

### 환경 변수

- `PLAYWRIGHT_SERVER_HOST` / `PLAYWRIGHT_SERVER_PORT`: 백엔드 주소 (기본: `localhost:4096`)
- `PLAYWRIGHT_PORT`: Vite 개발 서버 포트 (기본: `3000`)
- `PLAYWRIGHT_BASE_URL`: 기본 URL 오버라이드

---

## 19. 다운로드 통계

> 출처: [STATS.md](STATS.md)

| 날짜 | GitHub | npm | 총합 |
|------|--------|-----|------|
| 2025-10-03 | 446,829 | 359,937 | **806,766** |

- 일일 평균 증가: ~10,000+ 다운로드
- 주요 마일스톤: 2025-07-08 첫 10만 다운로드 돌파

---

## 20. 개발 명령어

```bash
# 의존성 설치
bun install

# 개발 서버 시작
bun dev

# 특정 디렉토리에서 실행
bun dev <directory>

# API 서버만 시작
bun dev serve

# 웹 인터페이스 시작
bun dev web

# 빌드
./packages/opencode/script/build.ts --single

# 테스트
bun run --cwd packages/opencode test

# 타입 체크
bun turbo typecheck

# SDK 재생성
./packages/sdk/js/script/build.ts

# 포맷팅
bun run script/format.ts

# ACP 서버 시작
opencode acp

# GitHub Action 설치
opencode github install
```

---

## 21. 결론

OpenCode는 **잘 구조화된 모노레포 프로젝트**로:

1. **핵심 엔진** (`packages/opencode`): 40,000줄 이상의 TypeScript 코드로 AI 에이전트, 도구, 세션 관리 등 핵심 기능 구현
2. **다중 인터페이스**: TUI, 웹, 데스크톱 앱을 공유 컴포넌트로 지원
3. **확장성**: 플러그인 시스템, 커스텀 에이전트/도구/스킬/명령어 지원
4. **프로바이더 독립**: 20+ AI 프로바이더 지원으로 벤더 락인 방지
5. **개발자 경험**: LSP, MCP, ACP 통합으로 IDE급 기능 제공
6. **GitHub 통합**: PR/이슈에서 직접 AI 에이전트 호출 가능
7. **성능 최적화**: 상세한 성능 로드맵과 기술 명세서 기반 개선 진행 중

프로젝트는 터미널 기반 개발 도구의 한계를 넓히는 것을 목표로 활발히 개발되고 있으며, 80만+ 다운로드를 달성한 성장하는 오픈소스 프로젝트입니다.

---

## 부록 A: 파일 I/O 패턴

> 출처: [.opencode/skill/bun-file-io/SKILL.md](.opencode/skill/bun-file-io/SKILL.md)

### Bun 파일 API

```ts
// 파일 읽기 (lazy)
const file = Bun.file(path)
await file.text()
await file.json()
await file.exists()
file.size
file.type

// 파일 쓰기
await Bun.write(dest, content)

// 파일 삭제
await Bun.file(path).delete()

// 글로브 스캔
const glob = new Bun.Glob("**/*.ts")
const files = await Array.fromAsync(glob.scan({ cwd, absolute: true }))

// 바이너리 찾기 및 실행
const path = Bun.which("ripgrep")
const proc = Bun.spawn([path, ...args])
const output = await Bun.readableStreamToText(proc.stderr)
```

### 디렉토리 작업 (node:fs 사용)

```ts
import { mkdir, readdir } from "node:fs/promises"
await mkdir(path, { recursive: true })
const entries = await readdir(path)
```

---

## 부록 B: 국제화 (i18n) 현황

> 출처: [specs/06-app-i18n-audit.md](specs/06-app-i18n-audit.md)

### 지원 언어

- 영어 (`en.ts`) - 373 키
- 중국어 간체 (`zh.ts`) - 373 키

### 번역 완료 영역

- 페이지: home, layout, session, error
- 컴포넌트: prompt-input, dialog-connect-provider, session-header
- 컨텍스트: notification, global-sync, file, local, terminal

### 번역 패턴

```ts
const { t, locale } = useLanguage()

// 텍스트 번역
<span>{t("session.header.search.placeholder")}</span>

// 날짜/숫자 로케일
Luxon.DateTime.fromISO(date).toRelative({ locale: locale() })
new Intl.NumberFormat(locale()).format(number)
```
