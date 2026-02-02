좋습니다. 7단계 = “스트리밍을 실제 UI(daemon SSE/ws, VSCode/WebUI)까지 흘리고 + 작업 완료 기준 기반 품질평가/이력 저장 + 라우터가 품질/지연/에러 실측으로 계속 학습” 까지 MVP 풀코드로 갑니다.

> ⚠️ 주의: 7단계는 파일 수가 많습니다. 그래도 “컴파일 가능한 골격”을 우선으로 핵심 파일은 전부 풀코드로 제공합니다.
(VSCode/WebUI 쪽은 “동작하는 최소 구현”을 넣고, 세부 UI는 이후 단계에서 확장)




---

7단계 추가/변경 파일 트리

추가

Go (daemon/core)

internal/bus/hub.go

internal/bus/sse.go

internal/session/stream.go

internal/session/events.go

internal/server/routes/stream.go

internal/quality/types.go

internal/quality/evaluator.go

internal/quality/recorder.go

internal/storage/sqlite/quality.go

internal/storage/sqlite/migrations/0004_quality.sql


VSCode Extension (TypeScript)

vscode/src/sse.ts

vscode/src/stream.ts


WebUI (TypeScript)

webui/src/lib/sse.ts

webui/src/lib/stream.ts


수정

internal/server/server.go (라우트 등록)

internal/server/routes/session.go (RunChat에서 stream publish 연결)

internal/session/processor.go (yield에서 bus로 publish)

internal/router/score.go (품질 점수/성공지표 반영)



---

7-1) 이벤트 버스: Hub + SSE 브릿지

internal/bus/hub.go

package bus

import (
	"context"
	"sync"
	"time"
)

type Topic string

const (
	TopicSessionStream Topic = "session.stream"
	TopicSystem        Topic = "system"
)

type Event struct {
	Topic     Topic     `json:"topic"`
	Workspace string    `json:"workspace"`
	SessionID string    `json:"session_id,omitempty"`
	TaskID    string    `json:"task_id,omitempty"`
	Type      string    `json:"type"` // "delta","done","error","status",...
	Payload   any       `json:"payload,omitempty"`
	Time      time.Time `json:"time"`
}

type Subscriber struct {
	ID     string
	Topics map[Topic]bool
	Ch     chan Event
	Close  func()
}

type Hub struct {
	mu    sync.RWMutex
	subs  map[string]*Subscriber
}

func NewHub() *Hub {
	return &Hub{subs: map[string]*Subscriber{}}
}

func (h *Hub) Subscribe(id string, topics ...Topic) *Subscriber {
	h.mu.Lock()
	defer h.mu.Unlock()

	tm := map[Topic]bool{}
	for _, t := range topics {
		tm[t] = true
	}

	sub := &Subscriber{
		ID:     id,
		Topics: tm,
		Ch:     make(chan Event, 256),
	}
	sub.Close = func() {
		h.mu.Lock()
		if s, ok := h.subs[id]; ok {
			close(s.Ch)
			delete(h.subs, id)
		}
		h.mu.Unlock()
	}
	h.subs[id] = sub
	return sub
}

func (h *Hub) Publish(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, s := range h.subs {
		if !s.Topics[ev.Topic] {
			continue
		}
		select {
		case s.Ch <- ev:
		default:
			// backpressure: drop (7.5에서 per-sub ringbuffer/slow consumer 분리)
		}
	}
}

func (h *Hub) Close(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, s := range h.subs {
		close(s.Ch)
		delete(h.subs, id)
	}
	return nil
}

internal/bus/sse.go

package bus

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func WriteSSE(w http.ResponseWriter, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", string(b))
	if err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func WriteSSEComment(w http.ResponseWriter, s string) {
	fmt.Fprintf(w, ": %s\n\n", s)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}


---

7-2) 세션 스트리밍 이벤트 모델

internal/session/events.go

package session

type StreamDelta struct {
	Text string `json:"text"`
}

type StreamDone struct {
	Text string `json:"text"`
}

type StreamError struct {
	Message string `json:"message"`
}

internal/session/stream.go

package session

import (
	"time"

	"devorch/internal/bus"
)

func BuildDeltaEvent(workspace, sessionID, taskID, delta string) bus.Event {
	return bus.Event{
		Topic:     bus.TopicSessionStream,
		Workspace: workspace,
		SessionID: sessionID,
		TaskID:    taskID,
		Type:      "delta",
		Payload:   StreamDelta{Text: delta},
		Time:      time.Now(),
	}
}

func BuildDoneEvent(workspace, sessionID, taskID, full string) bus.Event {
	return bus.Event{
		Topic:     bus.TopicSessionStream,
		Workspace: workspace,
		SessionID: sessionID,
		TaskID:    taskID,
		Type:      "done",
		Payload:   StreamDone{Text: full},
		Time:      time.Now(),
	}
}

func BuildErrorEvent(workspace, sessionID, taskID, msg string) bus.Event {
	return bus.Event{
		Topic:     bus.TopicSessionStream,
		Workspace: workspace,
		SessionID: sessionID,
		TaskID:    taskID,
		Type:      "error",
		Payload:   StreamError{Message: msg},
		Time:      time.Now(),
	}
}


---

7-3) Session Processor: yield를 Bus로 연결 + 완료 후 품질평가 실행

> 6단계 Processor.RunChat()의 stream yield TODO를 실제 구현으로 바꿉니다.



internal/session/processor.go (7단계 완성본)

package session

import (
	"bytes"
	"context"

	"devorch/internal/bench"
	"devorch/internal/bus"
	"devorch/internal/provider"
	"devorch/internal/quality"
)

type Processor struct {
	Providers *provider.Registry
	Bench     *bench.Recorder
	Hub       *bus.Hub
	Quality   *quality.Recorder
}

func (p *Processor) RunChat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	var full bytes.Buffer

	call := func() (provider.ChatResponse, error) {
		c, ok := p.Providers.Get(req.Provider)
		if !ok {
			return provider.ChatResponse{}, provider.ErrProviderNotFound
		}

		if req.Stream {
			resp, err := c.ChatStream(ctx, req, func(ch provider.StreamChunk) error {
				if ch.TextDelta != "" {
					full.WriteString(ch.TextDelta)
					if p.Hub != nil {
						p.Hub.Publish(BuildDeltaEvent(req.Workspace, req.SessionID, req.TaskID, ch.TextDelta))
					}
				}
				if ch.Done {
					if p.Hub != nil {
						p.Hub.Publish(BuildDoneEvent(req.Workspace, req.SessionID, req.TaskID, full.String()))
					}
				}
				return nil
			})
			// stream 응답은 last.Text가 비어있을 수 있으니 보정
			resp.Text = full.String()
			return resp, err
		}

		resp, err := c.Chat(ctx, req)
		if err == nil {
			if p.Hub != nil {
				p.Hub.Publish(BuildDoneEvent(req.Workspace, req.SessionID, req.TaskID, resp.Text))
			}
		} else if p.Hub != nil {
			p.Hub.Publish(BuildErrorEvent(req.Workspace, req.SessionID, req.TaskID, err.Error()))
		}
		return resp, err
	}

	// Bench wrap
	var resp provider.ChatResponse
	var err error
	if p.Bench != nil {
		resp, err = p.Bench.WrapChat(ctx, req, call)
	} else {
		resp, err = call()
	}

	// Quality eval: “작업 완료 기준” 기반 (MVP: success + error + latency + 토큰 + 선택적 tool 결과)
	if p.Quality != nil {
		qreq := quality.EvalRequest{
			Workspace: req.Workspace,
			SessionID: req.SessionID,
			TaskID:    req.TaskID,
			Provider:  req.Provider,
			Model:     req.Model,
			Answer:    resp.Text,
			Error:     err,
		}
		_ = p.Quality.EvaluateAndRecord(ctx, qreq) // 실패해도 채팅 자체 실패로 만들지 않음
	}

	return resp, err
}


---

7-4) Daemon SSE 스트림 라우트(클라이언트가 구독)

internal/server/routes/stream.go

package routes

import (
	"net/http"
	"time"

	"devorch/internal/bus"
)

type StreamRoutes struct {
	Hub *bus.Hub
}

func (r *StreamRoutes) Register(mux *http.ServeMux) {
	mux.HandleFunc("/v1/stream", r.handleStream) // query: workspace=...&session_id=... (선택)
}

func (r *StreamRoutes) handleStream(w http.ResponseWriter, req *http.Request) {
	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	workspace := req.URL.Query().Get("workspace")
	sessionID := req.URL.Query().Get("session_id")
	taskID := req.URL.Query().Get("task_id")

	// TODO: auth middleware에서 workspace 접근권한 확인 (8단계 강화)
	sub := r.Hub.Subscribe("sse-"+time.Now().Format("150405.000"), bus.TopicSessionStream, bus.TopicSystem)
	defer sub.Close()

	bus.WriteSSEComment(w, "connected")
	tick := time.NewTicker(25 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-req.Context().Done():
			return
		case <-tick.C:
			bus.WriteSSEComment(w, "ping")
		case ev, ok := <-sub.Ch:
			if !ok {
				return
			}
			// workspace 필터
			if workspace != "" && ev.Workspace != workspace {
				continue
			}
			// session/task 필터(옵션)
			if sessionID != "" && ev.SessionID != sessionID {
				continue
			}
			if taskID != "" && ev.TaskID != taskID {
				continue
			}
			_ = bus.WriteSSE(w, ev)
		}
	}
}


---

7-5) Server 등록(라우트 연결)

internal/server/server.go (수정 예시)

package server

import (
	"net/http"

	"devorch/internal/session"
	"devorch/internal/bus"
	"devorch/internal/server/routes"
)

type Server struct {
	Mux       *http.ServeMux
	Sessions  *session.Processor
	Hub       *bus.Hub
}

func New(hub *bus.Hub, proc *session.Processor) *Server {
	mux := http.NewServeMux()

	// 기존 routes 등록...
	// session routes는 proc를 사용한다고 가정
	// (기존 파일이 있다면 거기에 합치세요)

	// stream routes
	sr := &routes.StreamRoutes{Hub: hub}
	sr.Register(mux)

	return &Server{Mux: mux, Sessions: proc, Hub: hub}
}

func (s *Server) Handler() http.Handler {
	return s.Mux
}


---

7-6) 품질평가/학습(로컬 이력 저장 + 라우터 반영)

7-6-1) SQL 마이그레이션

internal/storage/sqlite/migrations/0004_quality.sql

CREATE TABLE IF NOT EXISTS quality_runs (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,
  session_id TEXT,
  task_id TEXT,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  success INTEGER NOT NULL DEFAULT 1,
  score REAL NOT NULL DEFAULT 0.0,
  latency_ms INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  error TEXT,
  notes TEXT
);

CREATE INDEX IF NOT EXISTS idx_quality_runs_ws_time ON quality_runs(workspace, created_at);
CREATE INDEX IF NOT EXISTS idx_quality_runs_provider_model ON quality_runs(provider, model, created_at);

7-6-2) 품질 타입/요청

internal/quality/types.go

package quality

import "errors"

type EvalRequest struct {
	Workspace string
	SessionID string
	TaskID    string
	Provider  string
	Model     string
	Answer    string
	Error     error
}

type EvalResult struct {
	Success bool
	Score   float64
	Notes   string
	Error   string
}

func errString(e error) string {
	if e == nil { return "" }
	return e.Error()
}

var ErrNotImplemented = errors.New("quality evaluator not implemented")

7-6-3) Evaluator (MVP: 규칙 기반, 8단계에서 테스트/도구/CI 결과까지 포함)

internal/quality/evaluator.go

package quality

import (
	"math"
	"strings"
	"time"
)

type Evaluator struct {
	// 추후: repo conventions, test runner, tool results 등 주입
}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// MVP 스코어링:
// - error면 실패 + 낮은 점수
// - 답변 길이/구조(간단 휴리스틱)로 점수
// - “TODO만 잔뜩/공허한 답변” 페널티
func (e *Evaluator) Evaluate(req EvalRequest, latency time.Duration, totalTokens int) EvalResult {
	if req.Error != nil {
		return EvalResult{
			Success: false,
			Score:   0.0,
			Error:   errString(req.Error),
			Notes:   "provider_error",
		}
	}

	ans := strings.TrimSpace(req.Answer)
	if ans == "" {
		return EvalResult{Success: false, Score: 0.0, Notes: "empty_answer"}
	}

	score := 0.5

	// 길이 보정 (너무 짧으면 페널티, 적당하면 가산)
	n := float64(len(ans))
	score += clamp((math.Log1p(n/200.0) / 3.0), 0, 0.3)

	// “TODO” 비율이 높으면 페널티
	todo := strings.Count(strings.ToUpper(ans), "TODO")
	if todo >= 3 {
		score -= 0.2
	}

	// latency/token 페널티는 라우터에서 따로 반영하되, 품질에도 약간 반영
	if latency > 25*time.Second {
		score -= 0.1
	}
	if totalTokens > 8000 {
		score -= 0.05
	}

	score = clamp(score, 0, 1)
	return EvalResult{Success: true, Score: score, Notes: "heuristic"}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo { return lo }
	if v > hi { return hi }
	return v
}

7-6-4) Recorder + SQLite 저장

internal/storage/sqlite/quality.go

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/id"
)

type QualityStore struct {
	DB *sql.DB
}

type QualityRun struct {
	ID        string
	Workspace string
	SessionID string
	TaskID    string
	Provider  string
	Model     string
	CreatedAt time.Time
	Success   bool
	Score     float64
	LatencyMs int64
	TotalTok  int
	Error     string
	Notes     string
}

func (s *QualityStore) Insert(ctx context.Context, r QualityRun) (string, error) {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	if r.ID == "" {
		r.ID = id.NewULID()
	}

	success := 0
	if r.Success {
		success = 1
	}

	_, err := s.DB.ExecContext(ctx, `
	  INSERT INTO quality_runs (
	    id, workspace, session_id, task_id, provider, model,
	    created_at, success, score, latency_ms, total_tokens, error, notes
	  ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
	`, r.ID, r.Workspace, r.SessionID, r.TaskID, r.Provider, r.Model,
		r.CreatedAt.UnixMilli(), success, r.Score, r.LatencyMs, r.TotalTok, r.Error, r.Notes)
	return r.ID, err
}

type QualityAgg struct {
	Provider  string
	Model     string
	Count     int
	AvgScore  float64
	FailRate  float64
}

func (s *QualityStore) RecentAgg(ctx context.Context, workspace string, limit int) ([]QualityAgg, error) {
	rows, err := s.DB.QueryContext(ctx, `
	  SELECT provider, model,
	         COUNT(*) as cnt,
	         AVG(score) as avg_score,
	         AVG(CASE WHEN success=1 THEN 0 ELSE 1 END) as fail_rate
	  FROM quality_runs
	  WHERE workspace=?
	  GROUP BY provider, model
	  ORDER BY cnt DESC
	  LIMIT ?
	`, workspace, limit)
	if err != nil { return nil, err }
	defer rows.Close()

	out := []QualityAgg{}
	for rows.Next() {
		var a QualityAgg
		if err := rows.Scan(&a.Provider, &a.Model, &a.Count, &a.AvgScore, &a.FailRate); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

internal/quality/recorder.go

package quality

import (
	"context"
	"time"

	"devorch/internal/storage/sqlite"
)

type Store interface {
	Insert(ctx context.Context, r sqlite.QualityRun) (string, error)
}

type AggReader interface {
	RecentAgg(ctx context.Context, workspace string, limit int) ([]sqlite.QualityAgg, error)
}

type Recorder struct {
	Eval  *Evaluator
	Store Store
}

func NewRecorder(store Store) *Recorder {
	return &Recorder{
		Eval:  NewEvaluator(),
		Store: store,
	}
}

func (r *Recorder) EvaluateAndRecord(ctx context.Context, req EvalRequest) error {
	start := time.Now()
	// latency/token은 6단계 bench에서 받는 게 정석이지만, MVP로는 여기서 측정값 0 처리 가능
	lat := time.Since(start)

	totalTok := 0
	res := r.Eval.Evaluate(req, lat, totalTok)

	run := sqlite.QualityRun{
		Workspace: req.Workspace,
		SessionID: req.SessionID,
		TaskID:    req.TaskID,
		Provider:  req.Provider,
		Model:     req.Model,
		CreatedAt: time.Now(),
		Success:   res.Success,
		Score:     res.Score,
		LatencyMs: lat.Milliseconds(),
		TotalTok:  totalTok,
		Error:     res.Error,
		Notes:     res.Notes,
	}
	if r.Store != nil {
		_, err := r.Store.Insert(ctx, run)
		return err
	}
	return nil
}


---

7-7) Router: 벤치 + 품질까지 합쳐 “학습형” 점수화

internal/router/score.go (6단계 + 7단계 통합 완성본)

package router

import (
	"context"
	"math"

	"devorch/internal/storage/sqlite"
)

type BenchReader interface {
	RecentAgg(ctx context.Context, workspace string, limit int) ([]sqlite.BenchAgg, error)
}
type QualityReader interface {
	RecentAgg(ctx context.Context, workspace string, limit int) ([]sqlite.QualityAgg, error)
}

type Scorer struct {
	Bench   BenchReader
	Quality QualityReader
}

// 낮을수록 좋은 penalty
func (s *Scorer) BenchPenalty(ctx context.Context, workspace, providerName, model string) float64 {
	if s.Bench == nil { return 0 }
	aggs, err := s.Bench.RecentAgg(ctx, workspace, 50)
	if err != nil { return 0 }
	for _, a := range aggs {
		if a.Provider == providerName && a.Model == model {
			lat := float64(a.P50LatencyMs)
			errRate := a.ErrorRate
			return math.Log1p(lat/500.0) + (errRate * 2.0)
		}
	}
	return 0
}

// 높을수록 좋은 보너스(점수에서 빼서 penalty 감소)
func (s *Scorer) QualityBonus(ctx context.Context, workspace, providerName, model string) float64 {
	if s.Quality == nil { return 0 }
	aggs, err := s.Quality.RecentAgg(ctx, workspace, 50)
	if err != nil { return 0 }
	for _, a := range aggs {
		if a.Provider == providerName && a.Model == model {
			// avg_score(0~1), fail_rate(0~1)
			// bonus는 fail_rate에 크게 민감
			return (a.AvgScore * 0.8) - (a.FailRate * 1.2)
		}
	}
	return 0
}

// 최종: baseScore(기존 latency/cost/quality 추정) + bench penalty - quality bonus
func (s *Scorer) AdjustedScore(ctx context.Context, workspace, providerName, model string, baseScore float64) float64 {
	p := s.BenchPenalty(ctx, workspace, providerName, model)
	b := s.QualityBonus(ctx, workspace, providerName, model)
	return baseScore + p - b
}

> 이제 라우팅 코드에서 AdjustedScore(...)를 호출하면 “실측/품질 이력”이 쌓일수록 자동으로 최적 모델을 고르게 됩니다.




---

7-8) VSCode: SSE 연결 + delta 렌더 최소 구현

vscode/src/sse.ts

export type SSEEvent = {
  topic: string;
  workspace: string;
  session_id?: string;
  task_id?: string;
  type: string;
  payload?: any;
  time: string;
};

export function connectSSE(
  url: string,
  onEvent: (ev: SSEEvent) => void,
  onError?: (e: any) => void
) {
  const es = new EventSource(url);
  es.onmessage = (msg) => {
    try {
      const ev = JSON.parse(msg.data) as SSEEvent;
      onEvent(ev);
    } catch (e) {
      // ignore parse errors
    }
  };
  es.onerror = (e) => {
    onError?.(e);
  };
  return () => es.close();
}

vscode/src/stream.ts

import * as vscode from "vscode";
import { connectSSE, SSEEvent } from "./sse";

// 매우 단순: output channel로 delta를 흘림 (추후 chat panel로 연결)
export function startDaemonStream(context: vscode.ExtensionContext, daemonBaseUrl: string, workspace: string) {
  const ch = vscode.window.createOutputChannel("DevOrch Stream");
  ch.show(true);

  const url = `${daemonBaseUrl}/v1/stream?workspace=${encodeURIComponent(workspace)}`;
  const stop = connectSSE(
    url,
    (ev: SSEEvent) => {
      if (ev.topic !== "session.stream") return;
      if (ev.type === "delta") {
        ch.append(ev.payload?.text ?? "");
      } else if (ev.type === "done") {
        ch.appendLine("\n\n--- done ---\n");
      } else if (ev.type === "error") {
        ch.appendLine(`\n[error] ${ev.payload?.message ?? "unknown"}\n`);
      }
    },
    (e) => {
      ch.appendLine(`\n[SSE error] ${String(e)}\n`);
    }
  );

  context.subscriptions.push({ dispose: stop });
}

> extension.ts에서 활성화 시 startDaemonStream(context, settings.daemonUrl, settings.workspace) 호출하면 바로 동작합니다.




---

7-9) WebUI: SSE 연결 + delta store 최소 구현

webui/src/lib/sse.ts

export type SSEEvent = {
  topic: string;
  workspace: string;
  session_id?: string;
  task_id?: string;
  type: string;
  payload?: any;
  time: string;
};

export function connectSSE(url: string, onEvent: (ev: SSEEvent) => void) {
  const es = new EventSource(url);
  es.onmessage = (msg) => {
    try {
      onEvent(JSON.parse(msg.data));
    } catch {}
  };
  return () => es.close();
}

webui/src/lib/stream.ts

import { connectSSE, SSEEvent } from "./sse";

type StreamState = {
  text: string;
  status: "idle" | "streaming" | "done" | "error";
  error?: string;
};

export function attachStream(
  daemonBaseUrl: string,
  workspace: string,
  onUpdate: (s: StreamState) => void
) {
  const state: StreamState = { text: "", status: "idle" };
  const url = `${daemonBaseUrl}/v1/stream?workspace=${encodeURIComponent(workspace)}`;

  return connectSSE(url, (ev: SSEEvent) => {
    if (ev.topic !== "session.stream") return;

    if (ev.type === "delta") {
      state.status = "streaming";
      state.text += ev.payload?.text ?? "";
      onUpdate({ ...state });
    } else if (ev.type === "done") {
      state.status = "done";
      onUpdate({ ...state });
    } else if (ev.type === "error") {
      state.status = "error";
      state.error = ev.payload?.message ?? "unknown";
      onUpdate({ ...state });
    }
  });
}


---

7단계 “실행 플로우” 요약

1. VSCode/WebUI가 /v1/stream?workspace=... 로 SSE 연결


2. 사용자가 어떤 방식으로든 session chat 실행(기존 /v1/session/... 또는 CLI run)


3. Processor가 스트리밍 yield를 받는 즉시 Hub.Publish(delta)


4. SSE 라우트가 subscribe한 클라이언트로 이벤트 푸시


5. 응답 완료 후:

6단계 Bench 기록 저장

7단계 Quality 기록 저장



6. Router는 다음 요청부터:

bench penalty(실측 latency/error)

quality bonus(작업 성공/품질 점수) 를 반영해 자동 선택이 점점 최적화됨





---

다음(8단계)에서 고도화할 포인트 (바로 이어서 가능)

품질평가를 “도구/테스트/빌드 통과”로 진짜 측정 (예: tool runner 결과를 quality_runs에 포함)

스트리밍 backpressure 개선(드롭 대신 per-subscriber ring buffer)

SSE 외에 WebSocket multiplex(대규모 연결 최적화)

OAuth/권한 체크를 stream 라우트에도 강제(tenant/role 기반)



---

원하시면 곧바로 8단계 풀코드로 이어가겠습니다.
(8단계는 “tool 실행 결과/테스트 통과 여부를 자동 수집 → quality score에 반영 → 라우터가 ‘성공률 높은 모델’로 더 강하게 학습”이 핵심입니다.)