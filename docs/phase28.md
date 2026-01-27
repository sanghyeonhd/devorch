아래는 28단계 풀코드입니다.
목표는 27단계의 “카테고리 기반 모델 해석”을 에이전트 기반(subagent_type)까지 완전히 확장하고, delegate_task가 category 또는 subagent_type(에이전트명) 에 따라 동일한 3단계 해석(override→fallback→default) 으로 후보를 만들도록 연결하는 것입니다.

> ✅ 28단계 결과



ModelResolver.ResolveCandidatesForAgent() 추가

delegate_task 입력이

category="quick" 이면 카테고리 해석

subagent_type="oracle" 이면 에이전트 해석


internal/delegate에 “작업 스펙/라우팅/실행” 골격 추가

서버 라우트에 /v1/tasks/delegate 추가 (CLI/VSCode/WebUI가 호출 가능)



---

A) 28단계 변경 파일 목록

신규

internal/modelresolver/resolve_agent.go

internal/delegate/types.go

internal/delegate/router.go

internal/delegate/delegate.go

internal/server/routes/task_delegate.go


수정

internal/app/app.go (Delegate 라우터 주입)

internal/server/server.go (라우트 등록)

internal/server/routes/routes.go (라우트 묶음이 있으면)

(선택) internal/tool/task/delegate_task.go 가 이미 있으면 그곳에서 서버 호출 대신 내부 delegate 호출로 연결



---

1) ModelResolver: Agent 해석 추가

1-1) internal/modelresolver/resolve_agent.go (신규)

package modelresolver

import (
	"context"
	"errors"

	"devorch/internal/config"
	"devorch/internal/provider"
	"devorch/internal/router"
)

// ResolveCandidatesForAgent:
// 1) agents.{agent}.model 이 namespaced면 provider 고정 후보 1개
// 2) agents.{agent}.providers 우선순위로 fallback 후보 생성
// 3) systemDefaultModel + global providers
func (r *Resolver) ResolveCandidatesForAgent(ctx context.Context, agent string) ([]router.Candidate, error) {
	cfg := r.Config.Get()

	req := RequirementForAgent(cfg, agent)

	// 1) namespaced override
	if p, full, ok := ParseMaybeNamespacedModel(req.BaseModel); ok {
		if r.providerAvailable(p) {
			return []router.Candidate{{Provider: p, Model: full}}, nil
		}
		// provider가 registry에 없으면 fallback 진행
	}

	// 2) fallback chain
	out := make([]router.Candidate, 0, len(req.Providers))
	for _, p := range req.Providers {
		if !r.providerAvailable(p) {
			continue
		}
		out = append(out, router.Candidate{
			Provider: p,
			Model:    NamespacedModel(cfg, p, req.BaseModel),
		})
	}
	if len(out) > 0 {
		return out, nil
	}

	// 3) default
	if cfg.SystemDefaultModel == "" {
		return nil, errors.New("no_system_default_model")
	}

	for _, p := range cfg.Providers.Priority {
		if !r.providerAvailable(p) {
			continue
		}
		out = append(out, router.Candidate{
			Provider: p,
			Model:    NamespacedModel(cfg, p, cfg.SystemDefaultModel),
		})
	}
	if len(out) == 0 {
		return nil, errors.New("no_provider_available")
	}
	return out, nil
}

// (28단계 보조) Provider를 조회해서 “실제 Provider 구현체”도 반환하는 helper
func (r *Resolver) ResolveProvider(ctx context.Context, name string) (provider.Provider, bool) {
	if r.Registry == nil {
		return nil, false
	}
	return r.Registry.Get(name)
}

> ✅ 27단계 파일(internal/modelresolver/resolve.go)은 그대로 두고, agent 전용 파일만 추가합니다.




---

2) Delegate: 작업 스펙/라우팅/실행 (핵심)

2-1) internal/delegate/types.go (신규)

package delegate

// DelegateRequest: oh-my-opencode의 delegate_task 형태를 DevOrch 서버에 맞게 최소화
type DelegateRequest struct {
	// 둘 중 하나는 반드시 필요:
	Category     string   `json:"category,omitempty"`
	SubagentType string   `json:"subagent_type,omitempty"` // 예: "oracle", "librarian"

	Description string   `json:"description,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`

	LoadSkills  []string `json:"load_skills,omitempty"` // 28단계에서는 저장만, 29단계에서 실제 로딩 연결
	RunInBackground bool `json:"run_in_background,omitempty"`

	// 실행 옵션(필요시 확장)
	MaxTurns int `json:"max_turns,omitempty"`
}

// DelegateResult: 최소 결과 (task_id는 background에서만 유효)
type DelegateResult struct {
	TaskID   string `json:"task_id,omitempty"`
	Provider string `json:"provider"`
	Model    string `json:"model"`

	// 최종 텍스트 응답(동기 실행 시)
	Output string `json:"output,omitempty"`
}

2-2) internal/delegate/router.go (신규)

package delegate

import (
	"context"
	"errors"

	"devorch/internal/modelresolver"
	"devorch/internal/router"
)

// Resolver는 model candidates를 만들고, Router가 최종 선택을 합니다.
type RouterDeps struct {
	Models *modelresolver.Resolver
	Router *router.Router // 26단계/이전에서 만든 Router 사용(없으면 첫 후보 사용)
}

type DelegateRouter struct {
	models *modelresolver.Resolver
	r      *router.Router
}

func NewDelegateRouter(d RouterDeps) *DelegateRouter {
	return &DelegateRouter{
		models: d.Models,
		r:      d.Router,
	}
}

// PickCandidates: category/subagent_type에 따라 후보군 생성
func (dr *DelegateRouter) PickCandidates(ctx context.Context, req DelegateRequest) ([]router.Candidate, error) {
	if dr.models == nil {
		return nil, errors.New("modelresolver_not_configured")
	}
	if req.SubagentType != "" {
		return dr.models.ResolveCandidatesForAgent(ctx, req.SubagentType)
	}
	if req.Category != "" {
		return dr.models.ResolveCandidates(ctx, req.Category)
	}
	return nil, errors.New("either category or subagent_type required")
}

// PickOne: 후보군을 라우터 정책으로 하나 선택
func (dr *DelegateRouter) PickOne(ctx context.Context, req DelegateRequest) (router.Candidate, error) {
	cands, err := dr.PickCandidates(ctx, req)
	if err != nil {
		return router.Candidate{}, err
	}
	if len(cands) == 0 {
		return router.Candidate{}, errors.New("no_candidates")
	}

	// Router가 있으면 점수 기반 선택, 없으면 0번 후보
	if dr.r == nil {
		return cands[0], nil
	}

	// 28단계: Router.Select(...)가 없을 수도 있으니 “BestCandidate”만 최소로 가정
	best, _, err := dr.r.Choose(ctx, router.RouteRequest{
		Category: req.Category,
		Prompt:   req.Prompt,
	}, cands)
	if err != nil {
		return cands[0], nil
	}
	return best, nil
}

> ⚠️ router.Router.Choose(...) / router.RouteRequest는 26단계 구현에 따라 이름이 다를 수 있습니다.

만약 26단계 라우터가 SelectCandidate(ctx, cands) 형태라면 그 함수명만 맞춰 바꾸면 됩니다.

“라우터가 없다면 첫 후보 선택”으로 동작하므로 안전합니다.




2-3) internal/delegate/delegate.go (신규)

package delegate

import (
	"context"
	"errors"

	"devorch/internal/provider"
)

type ExecutorDeps struct {
	Router    *DelegateRouter
	Providers *provider.Registry
	// Background manager는 29단계에서 연결(여기서는 동기만)
}

type Executor struct {
	dr        *DelegateRouter
	providers *provider.Registry
}

func NewExecutor(d ExecutorDeps) *Executor {
	return &Executor{
		dr:        d.Router,
		providers: d.Providers,
	}
}

// Execute:
// - 28단계는 “동기 실행”만 최소 구현 (run_in_background는 task_id만 리턴하고 실제 큐잉은 29단계에서)
func (e *Executor) Execute(ctx context.Context, req DelegateRequest) (DelegateResult, error) {
	if e.dr == nil {
		return DelegateResult{}, errors.New("delegate_router_not_configured")
	}

	chosen, err := e.dr.PickOne(ctx, req)
	if err != nil {
		return DelegateResult{}, err
	}

	p, ok := e.providers.Get(chosen.Provider)
	if !ok {
		return DelegateResult{}, errors.New("provider_not_registered: " + chosen.Provider)
	}

	// background는 29단계에서 실제 큐에 넣고 task_id 발급
	if req.RunInBackground {
		return DelegateResult{
			TaskID:   "bg_TODO_29",
			Provider: chosen.Provider,
			Model:    chosen.Model,
		}, nil
	}

	// Provider Chat 최소 호출 (provider.Provider 인터페이스는 기존 구현에 맞춰 수정 필요)
	out, err := p.Chat(ctx, provider.ChatRequest{
		Model:  chosen.Model,
		Prompt: req.Prompt,
		// Description/Skills는 29단계 이후 prompt assembler로 반영
	})
	if err != nil {
		return DelegateResult{}, err
	}

	return DelegateResult{
		Provider: chosen.Provider,
		Model:    chosen.Model,
		Output:   out.Text,
	}, nil
}


---

3) Provider 인터페이스 최소 가정 (없으면 추가/수정)

> 28단계에서 p.Chat(...)를 호출합니다.
만약 Provider 인터페이스가 아직 정리되지 않았다면 아래처럼 최소 정의가 필요합니다.



3-1) internal/provider/provider.go (있으면 확인/없으면 추가)

package provider

import "context"

type Provider interface {
	Name() string
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

type ChatRequest struct {
	Model  string
	Prompt string
}

type ChatResponse struct {
	Text string
}


---

4) App에 Delegate Executor 연결

4-1) internal/app/app.go (수정)

package app

import (
	"context"

	"devorch/internal/config"
	"devorch/internal/delegate"
	"devorch/internal/exec"
	"devorch/internal/modelresolver"
	"devorch/internal/okAON/sqlite"
	"devorch/internal/provider"
	"devorch/internal/router"
)

type App struct {
	Ok        sqlite.Store
	Providers *provider.Registry

	Config *config.Store
	Models *modelresolver.Resolver

	Router *router.Router // 기존 라우터가 있으면 주입
	Delegate *delegate.Executor

	Runner *exec.Runner
}

func NewApp(ok sqlite.Store, reg *provider.Registry) *App {
	cfg, err := config.LoadMerged(config.LoadOptions{})
	if err != nil {
		cfg = config.DefaultConfig()
	}
	cs := config.NewStore(cfg)
	res := modelresolver.NewResolver(cs, reg)

	a := &App{
		Ok:        ok,
		Providers: reg,
		Config:    cs,
		Models:    res,
	}

	// Router는 기존 단계 구현이 있으면 생성/주입
	// a.Router = router.NewRouter(...)

	// 28단계 Delegate 연결
	dr := delegate.NewDelegateRouter(delegate.RouterDeps{
		Models: res,
		Router: a.Router,
	})
	a.Delegate = delegate.NewExecutor(delegate.ExecutorDeps{
		Router:    dr,
		Providers: reg,
	})

	a.Runner = exec.NewRunner(exec.RunnerDeps{
		Ok:        ok,
		Providers: reg,
		Router:    a.Router,
	})

	return a
}

func (a *App) Start(ctx context.Context) error { return nil }
func (a *App) Stop()                           {}


---

5) 서버 라우트: /v1/tasks/delegate

5-1) internal/server/routes/task_delegate.go (신규)

package routes

import (
	"encoding/json"
	"net/http"

	"devorch/internal/app"
	"devorch/internal/delegate"
)

type TaskDelegateHandler struct {
	App *app.App
}

func NewTaskDelegateHandler(a *app.App) *TaskDelegateHandler {
	return &TaskDelegateHandler{App: a}
}

func (h *TaskDelegateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.App == nil || h.App.Delegate == nil {
		http.Error(w, "delegate_not_configured", http.StatusInternalServerError)
		return
	}

	var req delegate.DelegateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_json", http.StatusBadRequest)
		return
	}

	res, err := h.App.Delegate.Execute(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}


---

6) 서버에 라우트 등록

아래는 “서버 라우팅 구조”가 어떻게 되어있는지(파일명이) 정확히 모르므로, 가장 일반적인 형태로 제공합니다.
당신 코드에서 http.ServeMux에 라우트를 붙이는 위치에 그대로 추가하면 됩니다.

6-1) internal/server/server.go (수정 예시)

package server

import (
	"net/http"

	"devorch/internal/app"
	"devorch/internal/server/routes"
)

type Server struct {
	app *app.App
	mux *http.ServeMux
}

func New(a *app.App) *Server {
	s := &Server{
		app: a,
		mux: http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// 기존 라우트들...
	// s.mux.Handle("/v1/chat", routes.NewChatHandler(s.app))
	// s.mux.Handle("/v1/chat/stream", routes.NewChatStreamHandler(s.app))

	// ✅ 28단계: delegate_task
	s.mux.Handle("/v1/tasks/delegate", routes.NewTaskDelegateHandler(s.app))
}

func (s *Server) Handler() http.Handler { return s.mux }


---

7) 28단계 동작 확인용 요청 예시

category 기반

curl -X POST http://127.0.0.1:9901/v1/tasks/delegate \
  -H "Content-Type: application/json" \
  -d '{
    "category":"quick",
    "description":"fast code scan",
    "prompt":"Find where auth is implemented",
    "run_in_background": false
  }'

subagent_type 기반 (에이전트 override)

curl -X POST http://127.0.0.1:9901/v1/tasks/delegate \
  -H "Content-Type: application/json" \
  -d '{
    "subagent_type":"oracle",
    "description":"architecture decision",
    "prompt":"Suggest the best router strategy for local+remote mix",
    "run_in_background": false
  }'


---

8) .devorch/devorch.jsonc (28단계용 오버라이드 예시)

{
  "systemDefaultModel": "claude-sonnet",

  "agents": {
    "oracle": {
      "model": "openai/gpt-4o-mini",
      "providers": ["openai", "anthropic", "google"]
    },
    "librarian": {
      "model": "opencode/big-pickle",
      "providers": ["opencode", "openai"]
    }
  },

  "categories": {
    "quick": {
      "model": "claude-haiku",
      "providers": ["anthropic", "openai", "ollama"]
    }
  }
}


---

다음 29단계(바로 이어서): Background delegate_task + OkAON 기록 + reward emit 연결

28단계는 동기 실행까지 완성했고, run_in_background=true는 bg_TODO_29로 남겨뒀습니다.
29단계에서 아래를 “풀코드”로 연결합니다:

background.Manager에 delegate enqueue

task_id 발급 + /v1/tasks/{id} 조회

완료 시 okAON에 (prompt, chosen model/provider, latency, outcome) 저장

hook on_task_complete에서 reward emit


원하면 29단계 풀코드 바로 작성하겠습니다.