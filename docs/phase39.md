아래는 39단계(Explain/Trace API + UI 연결 포인트) “풀코드(파일별)”입니다.
목표는 38단계에서 만든 Decision.Ranked(learning delta 포함)를 서버 API로 노출해서 WebUI/VSCode가 그대로 시각화할 수 있게 만드는 것입니다.

또한, 요청하신 대로 39단계까지 기준으로 트리 항목에서 누락된 개발 항목도 같이 점검했습니다(맨 아래 “누락 점검” 섹션).


---

✅ 39단계 구현 범위

서버(API)

GET /v1/router/explain : 현재 입력 조건으로 라우팅 실행 → Decision + Ranked 반환

GET /v1/router/history : OkAON 기준 최근 라우팅/런 기록(요약) 조회

GET /v1/router/run/{runId} : 특정 run 상세(품질/보상/선택모델 포함) 조회


SDK(선택)

UI가 쓰기 쉬운 DTO 타입 정의 (pkg/sdk/types_router.go)


> 주의: 이전 단계(1~38단계) 전체 파일이 이 대화에 존재하지 않기 때문에, 39단계 코드는 인터페이스를 명확히 끊어서(Router/Bench/OkAON Store) “바로 끼워 넣기” 형태로 작성했습니다.
즉, 내부 구현체가 이미 있으면 그대로 연결되고, 없으면 TODO로 채우면 됩니다.




---

1) internal/server/routes/router.go

package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"devorch/internal/router"
)

type RouterDeps struct {
	Router *router.Router
	Store  RouterHistoryStore // OkAON/bench 기반 조회(인터페이스)
}

type RouterHistoryStore interface {
	// 최근 run 요약 목록 (project/workspace/user 기준)
	ListRecentRuns(ctx context.Context, q ListRunsQuery) ([]RunSummary, error)
	// 특정 run 상세
	GetRun(ctx context.Context, runID string) (RunDetail, error)
}

type RouterRoutes struct {
	deps RouterDeps
}

func NewRouterRoutes(deps RouterDeps) *RouterRoutes {
	return &RouterRoutes{deps: deps}
}

func (rr *RouterRoutes) Register(mux *http.ServeMux) {
	// 39단계 핵심: 라우팅 Explain API
	mux.HandleFunc("/v1/router/explain", rr.handleExplain)

	// OkAON/bench 기록 조회(요약/상세)
	mux.HandleFunc("/v1/router/history", rr.handleHistory)
	mux.HandleFunc("/v1/router/run/", rr.handleRunByID) // /v1/router/run/{runId}
}

func (rr *RouterRoutes) handleExplain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if rr.deps.Router == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "router_not_configured"})
		return
	}

	q := r.URL.Query()

	in := router.RouteInput{
		OrgID:       q.Get("org_id"),
		WorkspaceID: q.Get("workspace_id"),
		ProjectID:   q.Get("project_id"),

		OS:        q.Get("os"),
		Arch:      q.Get("arch"),
		MachineFP: q.Get("machine_fp"),

		Category: q.Get("category"),
		Agent:    q.Get("agent"),
		Intent:   q.Get("intent"),

		PreferLocal:      parseBool(q.Get("prefer_local")),
		MaxLatencyMs:     parseInt64(q.Get("max_latency_ms")),
		MaxCostMicros:    parseInt64(q.Get("max_cost_micros")),
		RequireOAuthOnly: parseBool(q.Get("oauth_only")),

		OverrideProvider: q.Get("override_provider"),
		OverrideModel:    q.Get("override_model"),
	}

	// 기본값(없으면 unspecified로 흘려도 resolver가 처리)
	if strings.TrimSpace(in.Category) == "" {
		in.Category = "unspecified-low"
	}
	if strings.TrimSpace(in.Intent) == "" {
		in.Intent = "chat"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	dec, err := rr.deps.Router.Route(ctx, in)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "route_failed",
			"message": err.Error(),
		})
		return
	}

	resp := ExplainResponse{
		Input: ExplainInputEcho{
			OrgID:       in.OrgID,
			WorkspaceID: in.WorkspaceID,
			ProjectID:   in.ProjectID,
			OS:          in.OS,
			Arch:        in.Arch,
			MachineFP:   in.MachineFP,
			Category:    in.Category,
			Agent:       in.Agent,
			Intent:      in.Intent,
			PreferLocal: in.PreferLocal,

			MaxLatencyMs:     in.MaxLatencyMs,
			MaxCostMicros:    in.MaxCostMicros,
			RequireOAuthOnly: in.RequireOAuthOnly,

			OverrideProvider: in.OverrideProvider,
			OverrideModel:    in.OverrideModel,
		},
		Decision: DecisionDTO{
			Provider:      dec.Provider,
			Model:         dec.Model,
			Endpoint:      dec.Endpoint,
			ScoreBase:     dec.ScoreBase,
			ScoreLearning: dec.ScoreLearning,
			ScoreFinal:    dec.ScoreFinal,
		},
		Ranked: make([]CandidateDTO, 0, len(dec.Ranked)),
	}

	for _, c := range dec.Ranked {
		resp.Ranked = append(resp.Ranked, CandidateDTO{
			Provider:   c.Provider,
			Model:      c.Model,
			Endpoint:   c.Endpoint,
			BaseScore:  c.BaseScore,
			LearnDelta: c.LearnDelta,
			FinalScore: c.FinalScore,
			Reason:     c.Reason,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (rr *RouterRoutes) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if rr.deps.Store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "history_store_not_configured"})
		return
	}

	qp := r.URL.Query()
	limit := int(parseInt64(qp.Get("limit")))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	q := ListRunsQuery{
		OrgID:       qp.Get("org_id"),
		WorkspaceID: qp.Get("workspace_id"),
		ProjectID:   qp.Get("project_id"),
		Category:    qp.Get("category"),
		Agent:       qp.Get("agent"),
		Provider:    qp.Get("provider"),
		Model:       qp.Get("model"),
		Limit:       limit,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	rows, err := rr.deps.Store.ListRecentRuns(ctx, q)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":   "history_failed",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, HistoryResponse{
		Query: q,
		Runs:  rows,
	})
}

func (rr *RouterRoutes) handleRunByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if rr.deps.Store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "history_store_not_configured"})
		return
	}

	// /v1/router/run/{runId}
	runID := strings.TrimPrefix(r.URL.Path, "/v1/router/run/")
	runID = strings.TrimSpace(runID)
	if runID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "missing_run_id"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	detail, err := rr.deps.Store.GetRun(ctx, runID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error":   "run_not_found",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, RunDetailResponse{Run: detail})
}

// ---- helpers ----

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes" || s == "y"
}

func parseInt64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}


---

2) internal/server/routes/router_types.go (DTO/응답 타입)

package routes

import "time"

type ExplainInputEcho struct {
	OrgID       string `json:"org_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`

	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	MachineFP string `json:"machine_fp,omitempty"`

	Category string `json:"category,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Intent   string `json:"intent,omitempty"`

	PreferLocal bool `json:"prefer_local"`

	MaxLatencyMs     int64 `json:"max_latency_ms,omitempty"`
	MaxCostMicros    int64 `json:"max_cost_micros,omitempty"`
	RequireOAuthOnly bool  `json:"oauth_only"`

	OverrideProvider string `json:"override_provider,omitempty"`
	OverrideModel    string `json:"override_model,omitempty"`
}

type DecisionDTO struct {
	Provider string  `json:"provider"`
	Model    string  `json:"model"`
	Endpoint string  `json:"endpoint,omitempty"`

	ScoreBase     float64 `json:"score_base"`
	ScoreLearning float64 `json:"score_learning"`
	ScoreFinal    float64 `json:"score_final"`
}

type CandidateDTO struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint,omitempty"`

	BaseScore  float64  `json:"base_score"`
	LearnDelta float64  `json:"learn_delta"`
	FinalScore float64  `json:"final_score"`
	Reason     []string `json:"reason,omitempty"`
}

type ExplainResponse struct {
	Input    ExplainInputEcho `json:"input"`
	Decision DecisionDTO      `json:"decision"`
	Ranked   []CandidateDTO   `json:"ranked"`
}

type ListRunsQuery struct {
	OrgID       string `json:"org_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`

	Category string `json:"category,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`

	Limit int `json:"limit"`
}

type RunSummary struct {
	RunID     string `json:"run_id"`
	ProjectID string `json:"project_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`

	Category string `json:"category,omitempty"`
	Agent    string `json:"agent,omitempty"`

	Provider string `json:"provider"`
	Model    string `json:"model"`

	Success      bool    `json:"success"`
	LatencyMs    int64   `json:"latency_ms"`
	QualityScore float64 `json:"quality_score"`

	Reward     float64 `json:"reward,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`

	At time.Time `json:"at"`
}

type RunDetail struct {
	RunSummary

	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	MachineFP string `json:"machine_fp,omitempty"`

	InputTokens  int64 `json:"input_tokens,omitempty"`
	OutputTokens int64 `json:"output_tokens,omitempty"`
	TotalTokens  int64 `json:"total_tokens,omitempty"`
	CostMicros   int64 `json:"cost_micros,omitempty"`

	ErrorClass   string `json:"error_class,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type HistoryResponse struct {
	Query ListRunsQuery `json:"query"`
	Runs  []RunSummary  `json:"runs"`
}

type RunDetailResponse struct {
	Run RunDetail `json:"run"`
}


---

3) internal/server/server.go (RouterRoutes 등록 추가)

> 기존 server.go가 이미 있겠지만, 39단계에서 “정확히 어디를 수정해야 하는지”가 핵심이라 라우팅 등록부만 포함해 드립니다.
아래 RegisterRoutes() 블록을 기존 서버 초기화 흐름에 그대로 합치면 됩니다.



package server

import (
	"net/http"

	"devorch/internal/server/routes"
)

type Deps struct {
	Mux *http.ServeMux

	RouterRoutes *routes.RouterRoutes

	// ... existing deps (auth, session, tool, etc)
}

type Server struct {
	mux *http.ServeMux
}

func New(deps Deps) *Server {
	mux := deps.Mux
	if mux == nil {
		mux = http.NewServeMux()
	}

	// 39단계: router routes 등록
	if deps.RouterRoutes != nil {
		deps.RouterRoutes.Register(mux)
	}

	return &Server{mux: mux}
}

func (s *Server) Handler() http.Handler {
	return s.mux
}


---

4) pkg/sdk/types_router.go (UI/VSCode에서 그대로 쓰는 타입)

package sdk

import "time"

type RouterExplainRequest struct {
	OrgID       string
	WorkspaceID string
	ProjectID   string

	OS        string
	Arch      string
	MachineFP string

	Category string
	Agent    string
	Intent   string

	PreferLocal      bool
	MaxLatencyMs     int64
	MaxCostMicros    int64
	RequireOAuthOnly bool

	OverrideProvider string
	OverrideModel    string
}

type RouterExplainResponse struct {
	Input    ExplainInputEcho `json:"input"`
	Decision DecisionDTO      `json:"decision"`
	Ranked   []CandidateDTO   `json:"ranked"`
}

type ExplainInputEcho struct {
	OrgID       string `json:"org_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`

	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	MachineFP string `json:"machine_fp,omitempty"`

	Category string `json:"category,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Intent   string `json:"intent,omitempty"`

	PreferLocal bool `json:"prefer_local"`

	MaxLatencyMs     int64 `json:"max_latency_ms,omitempty"`
	MaxCostMicros    int64 `json:"max_cost_micros,omitempty"`
	RequireOAuthOnly bool  `json:"oauth_only"`

	OverrideProvider string `json:"override_provider,omitempty"`
	OverrideModel    string `json:"override_model,omitempty"`
}

type DecisionDTO struct {
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	Endpoint      string  `json:"endpoint,omitempty"`
	ScoreBase     float64 `json:"score_base"`
	ScoreLearning float64 `json:"score_learning"`
	ScoreFinal    float64 `json:"score_final"`
}

type CandidateDTO struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Endpoint string `json:"endpoint,omitempty"`

	BaseScore  float64  `json:"base_score"`
	LearnDelta float64  `json:"learn_delta"`
	FinalScore float64  `json:"final_score"`
	Reason     []string `json:"reason,omitempty"`
}

type RouterHistoryResponse struct {
	Query ListRunsQuery `json:"query"`
	Runs  []RunSummary  `json:"runs"`
}

type ListRunsQuery struct {
	OrgID       string `json:"org_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`

	Category string `json:"category,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`

	Limit int `json:"limit"`
}

type RunSummary struct {
	RunID       string    `json:"run_id"`
	ProjectID   string    `json:"project_id,omitempty"`
	SessionID   string    `json:"session_id,omitempty"`
	TaskID      string    `json:"task_id,omitempty"`
	Category    string    `json:"category,omitempty"`
	Agent       string    `json:"agent,omitempty"`
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	Success     bool      `json:"success"`
	LatencyMs   int64     `json:"latency_ms"`
	QualityScore float64  `json:"quality_score"`
	Reward      float64   `json:"reward,omitempty"`
	Confidence  float64   `json:"confidence,omitempty"`
	At          time.Time `json:"at"`
}


---

✅ 39단계까지 “트리 기준 누락 점검”

아래는 지금까지(38~39단계) 작성된 내용 기준으로, 사용자가 제시한 “최종 트리”에서 실제로 누락/미구현 가능성이 높은 항목입니다. (상세 설명은 줄이고, “무조건 있어야 하는 파일/모듈”만 체크)

A) Hook builtins에 트리만 있고 실제 구현이 없음

트리에 있음:

on_quality_gate.go

on_model_switch.go

on_retry_exhausted.go


👉 현재 대화에서 실제 풀코드로 제공된 건 on_task_complete.go만입니다.
다음 단계에서 이 3개 훅 구현이 필요합니다.

B) Router 쪽 “explain.go”가 트리에 있으나 서버로 노출 경로가 없었음

트리에 있음:

internal/router/explain.go


👉 39단계에서 서버 routes로 노출 완료했지만,
internal/router/explain.go 자체 구현(문자열 reason 정리, policy snapshot id, coldstart 표시 등)이 아직이면 다음에 채워야 합니다.

C) OkAON Store 조회 API(History/RunDetail)가 실제 sqlite query로 연결되어야 함

39단계에서 RouterHistoryStore 인터페이스만 추가했습니다.

👉 트리 상 다음이 구현되어 있어야 합니다:

internal/okAON/sqlite/query.go (ListRecentRuns/GetRun 대응 쿼리)

internal/okAON/sqlite/stats.go (모델별 성능/보상 집계)


현재 대화에서는 아직 실제 SQL 쿼리 구현 파일 풀코드가 제공되지 않았습니다.

D) WebUI/VSCode는 “파일 트리만” 있고 실제 화면 코드가 없음

트리에 있음:

vscode/src/ui/router_view.ts

webui/src/routes/router.tsx


👉 39단계는 “서버 API 제공”이 핵심이라 UI는 다음 단계로 넘기는 게 자연스럽습니다.
하지만 **트리 기준으로는 화면 코드도 구현되어야 “누락 없음”**이 됩니다.


---

다음(40단계) 바로 제안 (질문 없이 진행 가능)

internal/okAON/sqlite/query.go에 history/run detail 실제 SQL 구현

storage/sqlite/migrations/0003_okAON_learning.sql에 맞춘 조회 인덱스 추가(필요 시)

hook/builtins/on_model_switch.go 구현해서 “선택 모델 변경 이벤트”도 OkAON에 기록


원하시면 제가 40단계는 OkAON sqlite query 풀코드 + 필요한 SQL 인덱스까지 한 번에 이어서 작성하겠습니다.