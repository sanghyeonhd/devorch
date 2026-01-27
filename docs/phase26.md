아래는 26단계(서버 API 라우트에서 Runner 연결 + OkAON 조회 API + SSE 스트리밍(간소화)) 풀코드입니다.
핵심은 HTTP 요청 → 후보군 생성 → Runner.RunChat() 실행 → OkAON 기록/학습 반영 → 조회 API로 확인까지 “끊기지 않게” 연결하는 것입니다.

> ✅ 포함 범위 (26단계)



POST /v1/chat : 동기 응답(일반)

POST /v1/chat/stream : SSE 스트리밍(간소화: 토큰 단위가 아닌 “최종 결과”도 스트리밍으로 송출 가능)

GET /v1/learning/arms : OkAON arm_stats 조회

GET /v1/okaon/runs : OkAON runs 조회(최근 N개)

서버 구동: internal/server/server.go, 라우트 등록, JSON 에러 응답 표준화



---

1) Server Core

1-1) internal/server/server.go (신규)

package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"devorch/internal/app"
	"devorch/internal/server/routes"
)

type Server struct {
	App  *app.App
	HTTP *http.Server
}

type Config struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func New(app *app.App, cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:7777"
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 30 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 0 // SSE 고려: 0 or large
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = 2 * time.Minute
	}

	mux := http.NewServeMux()
	routes.Register(mux, app)

	s := &http.Server{
		Addr:         cfg.Addr,
		Handler:      withCommonMiddleware(mux),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return &Server{App: app, HTTP: s}
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.HTTP.Addr)
	if err != nil {
		return err
	}

	// ctx cancel 시 graceful shutdown
	go func() {
		<-ctx.Done()
		_ = s.Shutdown(context.Background(), 10*time.Second)
	}()

	return s.HTTP.Serve(ln)
}

func (s *Server) Shutdown(ctx context.Context, timeout time.Duration) error {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.HTTP.Shutdown(c)
}

// middleware는 최소만. (추후: cors, auth, rate-limit 등)
func withCommonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// JSON 기본 헤더는 응답 함수에서 설정
		// 간단한 health ping에 대해 빠르게 처리 가능
		next.ServeHTTP(w, r)
	})
}


---

2) Route Registration

2-1) internal/server/routes/register.go (신규)

package routes

import (
	"net/http"

	"devorch/internal/app"
)

func Register(mux *http.ServeMux, a *app.App) {
	// health
	mux.HandleFunc("/health", healthHandler())

	// chat
	mux.HandleFunc("/v1/chat", chatHandler(a))
	mux.HandleFunc("/v1/chat/stream", chatStreamHandler(a))

	// okAON / learning
	mux.HandleFunc("/v1/learning/arms", learningArmsHandler(a))
	mux.HandleFunc("/v1/okaon/runs", okaonRunsHandler(a))
}

2-2) internal/server/routes/health.go (신규)

package routes

import "net/http"

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}


---

3) Chat API (Runner 연결)

3-1) internal/server/routes/chat.go (신규)

package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"devorch/internal/app"
	"devorch/internal/global"
	"devorch/internal/provider"
	"devorch/internal/router"
)

type ChatRequest struct {
	// category(quick/ultrabrain/visual-engineering 등)
	Category string `json:"category"`

	// fingerprint(없으면 서버가 자동 계산)
	Fingerprint string `json:"fingerprint,omitempty"`

	Messages []provider.ChatMessage `json:"messages"`

	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`

	// 후보군을 클라이언트에서 직접 주고 싶으면 사용
	Candidates []router.Candidate `json:"candidates,omitempty"`

	// quality 측정 힌트(0..1). 없으면 0.5
	QualityHint01 float64 `json:"quality_hint_01,omitempty"`
}

type ChatResponse struct {
	RunID    string `json:"run_id"`
	Provider string `json:"provider"`
	Model    string `json:"model"`

	Text string `json:"text"`

	UsageTokensIn  int64 `json:"usage_tokens_in,omitempty"`
	UsageTokensOut int64 `json:"usage_tokens_out,omitempty"`
}

func chatHandler(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}

		var req ChatRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_json: "+err.Error())
			return
		}

		if req.Category == "" {
			req.Category = "quick"
		}
		if len(req.Messages) == 0 {
			writeJSONError(w, http.StatusBadRequest, "messages_required")
			return
		}

		fp := req.Fingerprint
		if fp == "" {
			fp = global.DefaultFingerprint(r)
		}

		cands := req.Candidates
		if len(cands) == 0 {
			// 후보군 자동 생성(가장 단순한 기본)
			cands = defaultCandidates(a, req.Category)
		}
		if len(cands) == 0 {
			writeJSONError(w, http.StatusServiceUnavailable, "no_candidates_available")
			return
		}

		pReq := provider.ChatRequest{
			Messages:    req.Messages,
			Temperature: req.Temperature,
			MaxTokens:   req.MaxTokens,
			TopP:        req.TopP,
			RequestID:   "", // 필요시 트레이스ID 넣기
			Stream:      false,
		}

		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()

		out, err := a.Runner.RunChat(ctx, execInput(fp, req.Category, cands, pReq, req.QualityHint01))
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, ChatResponse{
			RunID:          out.RunID,
			Provider:       out.Provider,
			Model:          out.Model,
			Text:           out.Response.Text,
			UsageTokensIn:  out.Response.UsageTokensIn,
			UsageTokensOut: out.Response.UsageTokensOut,
		})
	}
}

// Runner input 생성 (import 순환 방지용으로 여기서 struct 맞춰줌)
func execInput(fp, category string, cands []router.Candidate, pReq provider.ChatRequest, q float64) (in any) {
	// 내부 exec.RunInput과 동일 필드/타입을 직접 참조하려면 import 해야 하는데,
	// 프로젝트에서 패키지 레이아웃에 따라 순환이 날 수 있어 any로 넘기지 말고,
	// 실제로는 routes에서 exec를 import하는 구조를 권장합니다.
	// 여기서는 “명확한 풀코드”를 위해 exec를 직접 import합니다.
	return buildExecInput(fp, category, cands, pReq, q)
}

// 아래 buildExecInput은 같은 파일 하단에 두어 import 정리

> 위 코드에서 exec를 import해야 하므로, 같은 파일 하단에 실제 import가 들어간 버전을 제공합니다.
(Go는 파일 단위 import라서, 아래는 동일 파일의 완성형입니다.)



✅ internal/server/routes/chat.go 완성형(교체용)

package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"devorch/internal/app"
	"devorch/internal/exec"
	"devorch/internal/global"
	"devorch/internal/provider"
	"devorch/internal/router"
)

type ChatRequest struct {
	Category    string `json:"category"`
	Fingerprint string `json:"fingerprint,omitempty"`

	Messages []provider.ChatMessage `json:"messages"`

	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`

	Candidates []router.Candidate `json:"candidates,omitempty"`

	QualityHint01 float64 `json:"quality_hint_01,omitempty"`
}

type ChatResponse struct {
	RunID    string `json:"run_id"`
	Provider string `json:"provider"`
	Model    string `json:"model"`

	Text string `json:"text"`

	UsageTokensIn  int64 `json:"usage_tokens_in,omitempty"`
	UsageTokensOut int64 `json:"usage_tokens_out,omitempty"`
}

func chatHandler(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}

		var req ChatRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_json: "+err.Error())
			return
		}

		if req.Category == "" {
			req.Category = "quick"
		}
		if len(req.Messages) == 0 {
			writeJSONError(w, http.StatusBadRequest, "messages_required")
			return
		}

		fp := req.Fingerprint
		if fp == "" {
			fp = global.DefaultFingerprint(r)
		}

		cands := req.Candidates
		if len(cands) == 0 {
			cands = defaultCandidates(a, req.Category)
		}
		if len(cands) == 0 {
			writeJSONError(w, http.StatusServiceUnavailable, "no_candidates_available")
			return
		}

		pReq := provider.ChatRequest{
			Messages:    req.Messages,
			Temperature: req.Temperature,
			MaxTokens:   req.MaxTokens,
			TopP:        req.TopP,
			Stream:      false,
		}

		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()

		out, err := a.Runner.RunChat(ctx, exec.RunInput{
			Fingerprint:    fp,
			Category:       req.Category,
			Candidates:     cands,
			Request:        pReq,
			QualityHint01:  req.QualityHint01,
		})
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, ChatResponse{
			RunID:          out.RunID,
			Provider:       out.Provider,
			Model:          out.Model,
			Text:           out.Response.Text,
			UsageTokensIn:  out.Response.UsageTokensIn,
			UsageTokensOut: out.Response.UsageTokensOut,
		})
	}
}

func defaultCandidates(a *app.App, category string) []router.Candidate {
	// 26단계는 “최소 동작”을 위해,
	// provider registry에 등록된 provider 이름들을 순서대로 후보로 만들고,
	// 모델명은 category에 따라 대충 분기(추후 modelresolver로 교체)
	names := a.Providers.List()
	if len(names) == 0 {
		return nil
	}

	model := defaultModelByCategory(category)
	out := make([]router.Candidate, 0, len(names))
	for _, p := range names {
		out = append(out, router.Candidate{
			Provider: p,
			Model:    model,
		})
	}
	return out
}

func defaultModelByCategory(cat string) string {
	switch cat {
	case "visual-engineering", "artistry":
		return "gemini-3-flash"
	case "ultrabrain":
		return "gpt-4o-mini" // 예시. 실제는 config/modelresolver로 대체
	case "quick":
		return "claude-haiku"
	default:
		return "claude-sonnet"
	}
}

// --- json helpers (routes 공통) ---

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error": msg,
	})
}


---

4) SSE Stream API (간소화 버전)

4-1) internal/server/routes/chat_stream.go (신규)

package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"devorch/internal/app"
	"devorch/internal/exec"
	"devorch/internal/global"
	"devorch/internal/provider"
	"devorch/internal/router"
)

type ChatStreamRequest struct {
	Category    string `json:"category"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Messages    []provider.ChatMessage `json:"messages"`
	Candidates  []router.Candidate     `json:"candidates,omitempty"`
	QualityHint01 float64 `json:"quality_hint_01,omitempty"`
}

func chatStreamHandler(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSONError(w, http.StatusInternalServerError, "sse_not_supported")
			return
		}

		var req ChatStreamRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_json: "+err.Error())
			return
		}
		if req.Category == "" {
			req.Category = "quick"
		}
		if len(req.Messages) == 0 {
			writeJSONError(w, http.StatusBadRequest, "messages_required")
			return
		}

		fp := req.Fingerprint
		if fp == "" {
			fp = global.DefaultFingerprint(r)
		}

		cands := req.Candidates
		if len(cands) == 0 {
			cands = defaultCandidates(a, req.Category)
		}
		if len(cands) == 0 {
			writeJSONError(w, http.StatusServiceUnavailable, "no_candidates_available")
			return
		}

		// SSE headers
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		send := func(event string, payload any) {
			b, _ := json.Marshal(payload)
			_, _ = fmt.Fprintf(w, "event: %s\n", event)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
			flusher.Flush()
		}

		send("status", map[string]any{"state": "started", "category": req.Category})

		ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
		defer cancel()

		out, err := a.Runner.RunChat(ctx, exec.RunInput{
			Fingerprint: fp,
			Category:    req.Category,
			Candidates:  cands,
			Request: provider.ChatRequest{
				Messages: req.Messages,
				Stream:   false, // 현재 Provider 스트리밍이 없어도 SSE로 “상태/결과” 전달 가능
			},
			QualityHint01: req.QualityHint01,
		})
		if err != nil {
			send("error", map[string]any{"error": err.Error()})
			return
		}

		send("result", map[string]any{
			"run_id":   out.RunID,
			"provider": out.Provider,
			"model":    out.Model,
			"text":     out.Response.Text,
		})
		send("done", map[string]any{"ok": true})
	}
}


---

5) OkAON / Learning 조회 API

5-1) internal/server/routes/learning_arms.go (신규)

package routes

import (
	"net/http"
	"strconv"

	"devorch/internal/app"
)

func learningArmsHandler(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}

		fp := r.URL.Query().Get("fingerprint")
		cat := r.URL.Query().Get("category")
		limitStr := r.URL.Query().Get("limit")

		limit := 50
		if limitStr != "" {
			if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 500 {
				limit = v
			}
		}
		if fp == "" || cat == "" {
			writeJSONError(w, http.StatusBadRequest, "fingerprint_and_category_required")
			return
		}

		rows, err := a.Ok.QueryArmStats(r.Context(), fp, cat, limit)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"fingerprint": fp,
			"category":    cat,
			"items":       rows,
		})
	}
}

5-2) internal/server/routes/okaon_runs.go (신규)

package routes

import (
	"net/http"
	"strconv"

	"devorch/internal/app"
)

func okaonRunsHandler(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}

		fp := r.URL.Query().Get("fingerprint")
		cat := r.URL.Query().Get("category")
		limitStr := r.URL.Query().Get("limit")

		limit := 50
		if limitStr != "" {
			if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 500 {
				limit = v
			}
		}

		rows, err := a.Ok.QueryRuns(r.Context(), fp, cat, limit)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"fingerprint": fp,
			"category":    cat,
			"items":       rows,
		})
	}
}


---

6) sqlite Store에 Query API 추가 (OkAON 조회)

> 26단계에서 “학습이 실제로 쌓였는지” 확인하려면 조회가 있어야 합니다.
아래 2개를 sqlite.Store에 추가합니다.



6-1) internal/okAON/sqlite/query_arm_stats.go (신규)

package sqlite

import (
	"context"
)

type ArmStatRow struct {
	Fingerprint   string  `json:"fingerprint"`
	Category      string  `json:"category"`
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	CountRuns     int64   `json:"count_runs"`
	MeanReward01  float64 `json:"mean_reward01"`
	UpdatedTsUTC  int64   `json:"updated_ts_utc"`
}

func (s Store) QueryArmStats(ctx context.Context, fingerprint, category string, limit int) ([]ArmStatRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT fingerprint, category, provider, model, count_runs, mean_reward01, updated_ts_utc
FROM okaon_arm_stats
WHERE fingerprint=? AND category=?
ORDER BY mean_reward01 DESC, count_runs DESC
LIMIT ?
`, fingerprint, category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ArmStatRow, 0, limit)
	for rows.Next() {
		var r ArmStatRow
		if err := rows.Scan(&r.Fingerprint, &r.Category, &r.Provider, &r.Model, &r.CountRuns, &r.MeanReward01, &r.UpdatedTsUTC); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

6-2) internal/okAON/sqlite/query_runs.go (신규)

package sqlite

import (
	"context"
)

type RunRow struct {
	RunID        string  `json:"run_id"`
	Fingerprint  string  `json:"fingerprint"`
	Category     string  `json:"category"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	Success      bool    `json:"success"`
	LatencyMs    int64   `json:"latency_ms"`
	CostMicroUSD int64   `json:"cost_micro_usd"`
	Quality01    float64 `json:"quality01"`
	ErrorMsg     string  `json:"error_msg"`
	StartedTsUTC int64   `json:"started_ts_utc"`
	EndedTsUTC   int64   `json:"ended_ts_utc"`
}

func (s Store) QueryRuns(ctx context.Context, fingerprint, category string, limit int) ([]RunRow, error) {
	if limit <= 0 {
		limit = 50
	}

	// fingerprint/category가 비어있으면 전체에서 최근순 조회(개발/디버그 용)
	where := "1=1"
	args := make([]any, 0, 3)

	if fingerprint != "" {
		where += " AND fingerprint=?"
		args = append(args, fingerprint)
	}
	if category != "" {
		where += " AND category=?"
		args = append(args, category)
	}
	args = append(args, limit)

	q := `
SELECT run_id, fingerprint, category, provider, model,
       success, latency_ms, cost_micro_usd, quality01, error_msg,
       started_ts_utc, ended_ts_utc
FROM okaon_runs
WHERE ` + where + `
ORDER BY ended_ts_utc DESC
LIMIT ?
`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]RunRow, 0, limit)
	for rows.Next() {
		var r RunRow
		var successInt int
		if err := rows.Scan(
			&r.RunID, &r.Fingerprint, &r.Category, &r.Provider, &r.Model,
			&successInt, &r.LatencyMs, &r.CostMicroUSD, &r.Quality01, &r.ErrorMsg,
			&r.StartedTsUTC, &r.EndedTsUTC,
		); err != nil {
			return nil, err
		}
		r.Success = (successInt != 0)
		out = append(out, r)
	}
	return out, rows.Err()
}


---

7) Fingerprint 기본 구현 (서버가 자동 생성)

> “내 PC / 내 OS / 내 프로젝트” 최적화를 위해 fingerprint가 필요합니다.
26단계에서는 최소 버전으로 제공합니다(추후: hw_profile + project hash 결합).



7-1) internal/global/fingerprint.go (신규)

package global

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"runtime"
)

func DefaultFingerprint(r *http.Request) string {
	// 최소: os+arch+hostname
	host, _ := os.Hostname()
	raw := runtime.GOOS + "|" + runtime.GOARCH + "|" + host

	// (선택) 프로젝트별로 분리하고 싶으면 헤더/쿼리에 project_root 같은 걸 받도록 확장
	// project := r.Header.Get("X-DevOrch-Project")
	// raw += "|" + project

	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}


---

8) devorchd 엔트리포인트에서 서버 기동

8-1) cmd/devorchd/main.go (예시 교체)

package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"devorch/internal/app"
	"devorch/internal/okAON/sqlite"
	"devorch/internal/provider"
	"devorch/internal/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 종료 시그널
	go func() {
		ch := make(chan os.Signal, 2)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	// sqlite 연결(경로는 추후 XDG/paths.go로 교체)
	db, err := sql.Open("sqlite3", "./devorch.db?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ok := sqlite.Store{DB: db}

	// Provider registry (26단계에서는 더미/샘플 provider라도 등록돼 있어야 함)
	reg := provider.NewRegistry()

	// TODO: 실제 provider 등록 (openai/anthropic/google/ollama 등)
	// reg.Register(openai.New(...))
	// reg.Register(ollama.New(...))

	a := app.NewApp(ok, reg)
	if err := a.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer a.Stop()

	srv := server.New(a, server.Config{
		Addr: "127.0.0.1:7777",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  2 * time.Minute,
	})

	log.Println("devorchd listening on", srv.HTTP.Addr)
	if err := srv.ListenAndServe(ctx); err != nil {
		// 정상 종료면 http.ErrServerClosed 가능
		log.Println("server stopped:", err)
	}
}


---

26단계 동작 테스트 예시

1) 동기 채팅

curl -s http://127.0.0.1:7777/v1/chat \
  -H 'content-type: application/json' \
  -d '{
    "category":"quick",
    "messages":[{"role":"user","content":"hello"}]
  }'

2) SSE 스트리밍

curl -N http://127.0.0.1:7777/v1/chat/stream \
  -H 'content-type: application/json' \
  -d '{
    "category":"quick",
    "messages":[{"role":"user","content":"stream test"}]
  }'

3) 학습 arm_stats 조회

curl -s "http://127.0.0.1:7777/v1/learning/arms?fingerprint=...&category=quick&limit=20"

4) runs 조회

curl -s "http://127.0.0.1:7777/v1/okaon/runs?limit=50"


---

26단계에서 “누락 없이” 체크해야 하는 포인트

devorch.db에 0003_okAON_learning.sql이 적용돼 있어야 okaon_runs/okaon_arm_stats insert가 됩니다.

Provider registry에 최소 1개 provider가 등록돼 있어야 /v1/chat이 성공합니다.

Router의 학습 반영은 다음 요청부터 okaon_arm_stats가 쌓이면 점점 강해집니다.



---

다음 27단계(추천): “ModelResolver 실제 연결 + category→requirements→fallback 체인”

26단계는 후보군을 단순 생성했는데, 27단계에서

.devorch/devorch.jsonc + global config merge

category 요구사항(requirements) 기반 provider chain 생성

사용자 override → fallback → default 흐름을 실제로 구현
로 “oh-my-opencode급 모델 선택”이 완성됩니다.


원하면 27단계 풀코드 바로 이어서 작성하겠습니다.