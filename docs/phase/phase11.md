좋습니다. **Step 11 = Step 10에서 만든 “bench 기반 라우팅 + 학습(quality)”을 실제 런타임 흐름에 “연결”**하는 단계입니다.

아래는 Step 11 풀코드(파일별) 입니다.
(이 단계는 “기존 코드가 어떤 라우터/서버 프레임워크를 쓰는지”에 따라 통합 방식이 달라지므로, Step 11에서는 표준 net/http + http.ServeMux 기준으로 “연결 레이어”를 완성했습니다. 기존 서버가 다른 라우터를 쓰면, Register*() 함수만 그 라우터에 맞게 연결하면 됩니다.)


---

Step 11 변경/추가 파일 트리

devorch/
└─ internal/
   ├─ app/
   │  └─ step11_wiring.go                 # (NEW) bench/learning/router wiring
   │
   ├─ provider/
   │  └─ observer/
   │     ├─ observer.go                   # (NEW) provider-call observer interface
   │     └─ bench_recorder.go             # (NEW) provider-call 완료 시 benchmarks 기록
   │
   ├─ learning/
   │  └─ http/
   │     └─ feedback_routes.go            # (NEW) POST /v1/feedback
   │
   ├─ router/
   │  └─ http/
   │     └─ router_routes.go              # (NEW) GET /v1/router/choose (디버그)
   │
   └─ storage/sqlite/
      ├─ feedback.go                      # (NEW) feedback store 구현
      └─ migrations/
         └─ 0004_feedback.sql             # (Step10에서 추가한 것 그대로 사용)


---

1) Wiring: BenchmarksStore / Router / Recorder / FeedbackService 구성

1.1 internal/app/step11_wiring.go

package app

import (
	"database/sql"
	"errors"

	"devorch/internal/learning"
	learninghttp "devorch/internal/learning/http"
	"devorch/internal/router"
	routerhttp "devorch/internal/router/http"
	sqlstore "devorch/internal/storage/sqlite"
)

// Step11Deps: Step11에서 “학습/라우팅”에 필요한 의존성 묶음
type Step11Deps struct {
	BenchStore   *sqlstore.BenchmarksStore
	FeedbackStore *sqlstore.FeedbackStore

	Recorder *learning.Recorder
	Feedback *learning.FeedbackService

	Router *router.Router

	// HTTP 등록기(선택)
	HTTP struct {
		FeedbackRoutes *learninghttp.FeedbackRoutes
		RouterRoutes   *routerhttp.RouterRoutes
	}
}

// BuildStep11: sqlite DB만 있으면 Step11 핵심을 구성
func BuildStep11(db *sql.DB) (*Step11Deps, error) {
	if db == nil {
		return nil, errors.New("step11: db is nil")
	}

	bench := sqlstore.NewBenchmarksStore(db)
	fbStore := sqlstore.NewFeedbackStore(db)

	rec := learning.NewRecorder(bench)
	fbSvc := &learning.FeedbackService{
		Store: fbStore,
		Bench: rec,
	}

	rt := router.NewRouter(router.SQLiteBenchQuery{S: bench}, nil)

	deps := &Step11Deps{
		BenchStore:    bench,
		FeedbackStore: fbStore,
		Recorder:      rec,
		Feedback:      fbSvc,
		Router:        rt,
	}

	deps.HTTP.FeedbackRoutes = &learninghttp.FeedbackRoutes{Service: fbSvc}
	deps.HTTP.RouterRoutes = &routerhttp.RouterRoutes{Router: rt}

	return deps, nil
}


---

2) Provider-call 완료 시 자동 Bench 기록(“작업 완료 기준 벤치 저장”)

> 이 레이어는 “세션/도구/에이전트 어디서 호출하든” LLM 호출이 끝난 순간을 잡아 기록할 수 있게 해줍니다.
기존 provider 구현에서 “LLM 호출 완료 시점”에 Observer.OnComplete(...) 한 줄만 호출하면 됩니다.



2.1 internal/provider/observer/observer.go

package observer

import (
	"context"
	"time"
)

// ProviderCall = provider 한 번 호출에 대한 결과 요약(벤치 기록용)
type ProviderCall struct {
	CreatedAt time.Time

	Provider string
	Model    string
	Scenario string // "chat" | "code-edit" | "tool-call" 등

	OS                 string
	Arch               string
	MachineFingerprint string

	InputTokens  int64
	OutputTokens int64
	LatencyMs    int64

	Success bool
	Error   string

	// 선택
	CostUSD      *float64
	QualityScore *float64

	Metadata map[string]any
}

// Observer = 호출 완료 이벤트 수신자
type Observer interface {
	OnComplete(ctx context.Context, call ProviderCall) error
}

2.2 internal/provider/observer/bench_recorder.go

package observer

import (
	"context"
	"time"

	"devorch/internal/learning"
)

// BenchRecorderObserver: ProviderCall -> learning.Recorder로 기록
type BenchRecorderObserver struct {
	Recorder *learning.Recorder
}

func NewBenchRecorderObserver(r *learning.Recorder) *BenchRecorderObserver {
	return &BenchRecorderObserver{Recorder: r}
}

func (o *BenchRecorderObserver) OnComplete(ctx context.Context, call ProviderCall) error {
	if o == nil || o.Recorder == nil {
		return nil
	}
	created := call.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}

	m := learning.CompletionMetrics{
		CreatedAt: created,

		OS:                 call.OS,
		Arch:               call.Arch,
		MachineFingerprint: call.MachineFingerprint,

		Provider: call.Provider,
		Model:    call.Model,
		Scenario: call.Scenario,

		InputTokens:  call.InputTokens,
		OutputTokens: call.OutputTokens,
		LatencyMs:    call.LatencyMs,

		Success: call.Success,
		Error:   call.Error,

		QualityScore: call.QualityScore,
		CostUSD:      call.CostUSD,

		Metadata: call.Metadata,
	}
	return o.Recorder.RecordCompletion(ctx, m)
}


---

3) Feedback 저장소(SQLite) 구현 + HTTP API(thumbs/quality)

3.1 internal/storage/sqlite/feedback.go

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"devorch/internal/learning"
)

type FeedbackStore struct {
	db *sql.DB
}

func NewFeedbackStore(db *sql.DB) *FeedbackStore {
	return &FeedbackStore{db: db}
}

// InsertFeedback implements learning.FeedbackStore
func (s *FeedbackStore) InsertFeedback(ctx context.Context, f learning.Feedback) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite: feedback db is nil")
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO feedback(
  id, created_at,
  session_id, message_id,
  provider, model, scenario,
  thumbs_up, note,
  quality_score
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, f.ID, f.CreatedAt.UnixMilli(),
		f.SessionID, f.MessageID,
		f.Provider, f.Model, f.Scenario,
		boolToInt(f.ThumbsUp), f.Note,
		f.QualityScore,
	)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

3.2 internal/learning/http/feedback_routes.go

package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"devorch/internal/learning"
)

type FeedbackRoutes struct {
	Service *learning.FeedbackService
}

// Register registers feedback endpoints to mux
func (r *FeedbackRoutes) Register(mux *http.ServeMux) {
	if mux == nil {
		return
	}
	mux.HandleFunc("POST /v1/feedback", r.handleSubmit)
}

// --- request/response DTOs ---

type submitFeedbackReq struct {
	SessionID string `json:"sessionId"`
	MessageID string `json:"messageId"`

	Provider string `json:"provider"`
	Model    string `json:"model"`
	Scenario string `json:"scenario"`

	ThumbsUp *bool   `json:"thumbsUp,omitempty"`
	Note     string  `json:"note,omitempty"`

	// optional explicit qualityScore (0..1)
	QualityScore *float64 `json:"qualityScore,omitempty"`
}

type submitFeedbackResp struct {
	Feedback learning.Feedback `json:"feedback"`
}

func (r *FeedbackRoutes) handleSubmit(w http.ResponseWriter, req *http.Request) {
	if r == nil || r.Service == nil {
		http.Error(w, "feedback service not configured", http.StatusServiceUnavailable)
		return
	}

	var body submitFeedbackReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	body.SessionID = strings.TrimSpace(body.SessionID)
	body.MessageID = strings.TrimSpace(body.MessageID)
	body.Provider = strings.TrimSpace(body.Provider)
	body.Model = strings.TrimSpace(body.Model)
	body.Scenario = strings.TrimSpace(body.Scenario)

	if body.SessionID == "" || body.MessageID == "" || body.Provider == "" || body.Model == "" || body.Scenario == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	// build feedback
	var fb learning.Feedback
	if body.QualityScore != nil {
		qs := *body.QualityScore
		if qs < 0 {
			qs = 0
		}
		if qs > 1 {
			qs = 1
		}
		thumb := false
		if body.ThumbsUp != nil {
			thumb = *body.ThumbsUp
		} else {
			thumb = qs >= 0.5
		}
		fb = learning.Feedback{
			SessionID:     body.SessionID,
			MessageID:     body.MessageID,
			Provider:      body.Provider,
			Model:         body.Model,
			Scenario:      body.Scenario,
			ThumbsUp:      thumb,
			Note:          body.Note,
			QualityScore:  qs,
		}
	} else {
		thumb := false
		if body.ThumbsUp != nil {
			thumb = *body.ThumbsUp
		}
		fb = learning.NewThumbFeedback(body.SessionID, body.MessageID, body.Provider, body.Model, body.Scenario, thumb, body.Note)
	}

	out, err := r.Service.Submit(req.Context(), fb)
	if err != nil {
		http.Error(w, "failed to store feedback: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, submitFeedbackResp{Feedback: out})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}


---

4) Router 디버그 API(“왜 이 모델이 선택됐는지” 확인용)

4.1 internal/router/http/router_routes.go

package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"devorch/internal/router"
)

type RouterRoutes struct {
	Router *router.Router
}

func (r *RouterRoutes) Register(mux *http.ServeMux) {
	if mux == nil {
		return
	}
	mux.HandleFunc("GET /v1/router/choose", r.handleChoose)
}

type chooseResp struct {
	Chosen router.Scored   `json:"chosen"`
	All    []router.Scored `json:"all"`
}

func (r *RouterRoutes) handleChoose(w http.ResponseWriter, req *http.Request) {
	if r == nil || r.Router == nil {
		http.Error(w, "router not configured", http.StatusServiceUnavailable)
		return
	}

	q := req.URL.Query()
	scenario := strings.TrimSpace(q.Get("scenario"))
	if scenario == "" {
		scenario = "chat"
	}

	// candidates: provider/model 쌍을 query로 받거나, 실제 구현에서는 registry에서 후보군을 구성하면 됨
	// 예) ?c=openai:gpt-5.2&c=ollama:llama3.2
	cs := q["c"]
	if len(cs) == 0 {
		http.Error(w, "missing candidates (?c=provider:model)", http.StatusBadRequest)
		return
	}

	cands := make([]router.Candidate, 0, len(cs))
	for _, c := range cs {
		parts := strings.SplitN(c, ":", 2)
		if len(parts) != 2 {
			continue
		}
		cands = append(cands, router.Candidate{
			Provider: strings.TrimSpace(parts[0]),
			Model:    strings.TrimSpace(parts[1]),
			Tags:     map[string]string{},
		})
	}
	if len(cands) == 0 {
		http.Error(w, "no valid candidates", http.StatusBadRequest)
		return
	}

	rc := router.RequestCtx{
		Scenario: scenario,

		PreferLocal: q.Get("preferLocal") == "1",
		MaxLatency:  parseDurationMs(q.Get("maxLatencyMs")),
		MaxCostUSD:  parseFloat(q.Get("maxCostUsd")),

		OS:                 strings.TrimSpace(q.Get("os")),
		Arch:               strings.TrimSpace(q.Get("arch")),
		MachineFingerprint: strings.TrimSpace(q.Get("machineFp")),
	}

	chosen, all := r.Router.Choose(req.Context(), cands, rc)
	writeJSON(w, http.StatusOK, chooseResp{Chosen: chosen, All: all})
}

func parseDurationMs(s string) time.Duration {
	if s == "" {
		return 0
	}
	v, err := time.ParseDuration(s + "ms")
	if err != nil {
		return 0
	}
	return v
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	var f float64
	_ = json.Unmarshal([]byte(s), &f)
	return f
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}


---

5) “서버에 실제로 붙이는” 2줄짜리 연결 코드(필수)

> 당신 기존 internal/server/server.go 또는 internal/app/app.go에서 ServeMux를 만드는 곳이 있을 겁니다.
거기에 아래 2줄만 추가하면 Step11이 “진짜로 살아납니다”.



deps, _ := app.BuildStep11(db)
deps.HTTP.FeedbackRoutes.Register(mux)
deps.HTTP.RouterRoutes.Register(mux)


---

Step 11에서 “남은 실제 통합 포인트”(중요)

Step 11까지 하면 API와 저장소, 라우터 선택 엔진, 벤치 기록기가 준비됩니다.
이제 “진짜 자동 학습”이 되려면, LLM 호출이 끝나는 곳(provider 호출 반환 지점)에 아래 1줄을 넣어야 합니다:

observer.OnComplete(ctx, observer.ProviderCall{ ... })

✅ 다음 Step 12에서 제가 바로 해드릴 것:

당신 internal/provider/*(openai/anthropic/ollama 등)에서 streaming 포함 “호출 종료 시점”을 잡아

BenchRecorderObserver를 전 provider에 공통으로 적용(누락 없이)

session/tool/task에서 scenario 분류 기준(chat/code/tool-call 등)까지 고정


원하시면 **Step 12 “풀코드”**로 이어서 바로 작성하겠습니다.