Devorch 프로젝트 전체 설명서 (Master Overview)

1. 프로젝트 개요

Devorch는 다중 LLM(Multi-LLM) 환경에서 개발, 기획, 디자인, 프론트엔드, 백엔드, QA, 문서화, 운영 자동화를 통합적으로 오케스트레이션하는 차세대 AI 기반 무중단 개발 플랫폼입니다.

특히 다음과 같은 환경을 핵심 타깃으로 설계되었습니다:

폐쇄망 / 보안망 환경

외부 API 제한 또는 부분 허용 환경

대규모 엔터프라이즈 SI / 공공기관 개발 환경

멀티 모델 혼합 (Cloud + Local LLM)

무중단 개발(Non-Stop Development)


Devorch는 단순한 LLM 호출 툴이 아니라, **AI 에이전트 기반 개발 운영체제(Development Operating System)**에 가까운 개념입니다.


---

2. 핵심 비전 (Vision)

> "사람이 하던 개발 역할을, AI 에이전트들이 자동으로 분업하고 최적화하여, 개발 생산성을 10배 이상 향상시키는 시스템"



Devorch의 비전:

개발 프로세스 전체 자동화

최적의 LLM을 자동 선택

토큰, 성능, 비용, 안정성 기반 지능형 라우팅

모델 장애 시 자동 Fallback

Self-Learning을 통한 지속적 성능 개선



---

3. 전체 아키텍처 개요

[User / IDE / WebUI / CLI]
        |
        v
[Devorch Orchestrator Core]
        |
        +-- Router (Model Selection)
        +-- Agent Manager
        +-- Policy Engine
        +-- Reward / Bandit Engine
        +-- OkAON Controller
        +-- Self-Learning Engine
        |
        v
[LLM Providers]
  - Local LLM (Ollama, GGUF, llama.cpp, LM Studio 등)
  - Cloud LLM (OpenAI, Anthropic, Google, Azure 등)

        |
        v
[Bench / Telemetry / Logs / Metrics]


---

4. 핵심 구성 요소

4.1 Orchestrator Core

Devorch의 중앙 제어부로, 모든 요청과 작업 흐름을 통제합니다.

역할:

작업 분해(Task Decomposition)

에이전트 할당

워크플로우 실행

실패 복구 및 재시도



---

4.2 Multi-LLM Router

가장 중요한 핵심 모듈입니다.

기능:

작업 유형별 최적 LLM 자동 선택

성능, 비용, 지연시간, 성공률 기반 라우팅

토큰 부족 시 자동 모델 전환

Cloud → Local 자동 Fallback


선정 기준 예:

코드 생성 → Code 특화 모델

대용량 문서 분석 → Long Context 모델

이미지 생성 → Multimodal 모델

폐쇄망 → Local LLM 우선



---

4.3 Agent System (AI 에이전트)

Devorch는 단일 LLM이 아닌, 역할 기반 AI 에이전트 구조를 사용합니다.

대표 에이전트:

Planner Agent (기획)

Architect Agent (아키텍처 설계)

Frontend Agent

Backend Agent

QA/Test Agent

Security Agent

DevOps Agent

Documentation Agent


각 에이전트는 독립적으로 LLM을 사용하며, 협업합니다.


---

4.4 OkAON (Always-On Orchestration)

OkAON은 Devorch의 무중단 개발 철학을 구현하는 핵심 컨트롤러입니다.

기능:

작업 중단 방지

모델 장애 자동 감지

자동 모델 교체

세션 유지

작업 상태 복구


즉, 개발이 절대 멈추지 않도록 보장합니다.


---

4.5 Self-Learning Engine

Devorch는 정적인 시스템이 아니라, 스스로 학습합니다.

학습 대상:

모델별 성공률

작업 유형별 성능

사용자 선호

결과 품질


이를 통해:

시간이 지날수록 더 똑똑해짐

자동 정책 개선

라우팅 품질 향상



---

4.6 Reward / Bandit / Policy Engine

강화학습(RL) 개념을 차용한 최적화 엔진입니다.

구성:

Reward Engine: 결과 품질 점수화

Multi-Armed Bandit: 모델 선택 최적화

Policy Engine: 라우팅 정책 관리


효과:

최적의 모델 조합 자동 탐색

비용 대비 성능 극대화



---

5. 폐쇄망 / 보안 환경 지원

Devorch는 폐쇄망 환경을 1급 시민(First-Class Citizen)으로 지원합니다.

지원 기능:

완전 로컬 LLM 운용

외부 API 차단 모드

승인된 모델만 사용

파일 기반 모델 반입

Offline Bench 실행


특히 공공기관/금융권/SI 환경에 최적화되어 있습니다.


---

6. 개발 워크플로우 예시

6.1 신규 프로젝트 자동 생성

1. 사용자 요구사항 입력


2. Planner Agent → 요구사항 분석


3. Architect Agent → 전체 아키텍처 생성


4. Source Tree 자동 생성


5. README / 문서 자동 생성


6. Front/Back/QA 에이전트 병렬 개발




---

6.2 무중단 개발 시나리오

Cloud LLM 장애 발생

Router가 자동으로 Local LLM 전환

작업 지속

Cloud 복구 시 자동 재전환



---

7. DevOrch vs 기존 개발 방식

구분	기존 개발	Devorch

개발자 역할	사람 중심	AI 에이전트 중심
모델 선택	수동	자동
장애 대응	수동	자동
폐쇄망	제한적	완전 지원
확장성	낮음	매우 높음
자동화 수준	낮음	매우 높음



---

8. IDE / CLI / WebUI 통합

지원 채널:

VSCode Plugin

Eclipse 연동 (엔터프라이즈)

CLI (oh-my-opencode 연계)

WebUI Dashboard


기능:

실시간 에이전트 상태

모델 선택 현황

비용/성능 모니터링

작업 트래킹



---

9. 보안 및 감사(Audit)

모든 LLM 호출 로깅

작업 이력 추적

모델별 사용 기록

감사 로그(Audit Log)


금융/공공 기준 충족 가능 구조


---

10. 확장 로드맵

향후 확장:

3D GUI 기반 에이전트 협업 뷰

멀티 에이전트 시각화

조직 단위 정책 관리

온프레미스 전용 엔터프라이즈 에디션



---

11. 결론

Devorch는 단순한 AI 도구가 아니라,

> AI가 중심이 되는 차세대 개발 운영체제



입니다.

특히 SI, 공공, 폐쇄망, 대규모 엔터프라이즈 환경에서 기존 개발 방식의 한계를 근본적으로 해결하는 것을 목표로 합니다.


---

(본 문서는 Devorch Master Overview 문서로, README / 제안서 / 내부 공유 문서로 바로 사용 가능하도록 작성되었습니다.)