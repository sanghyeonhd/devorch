좋습니다. 8단계 목표를 “실제로 품질이 좋아지게 만드는” 방향으로 고정하고, 7단계 구조 위에 (1) Tool 실행 결과/에러/시간/산출물 요약을 자동 수집, (2) 테스트/빌드/정적분석 같은 “완료 기준(Definition of Done)” 시그널을 수집, (3) 품질 점수에 반영 + 라우터 학습에 반영, (4) 스트림도 OAuth 권한 체크까지 MVP 풀코드로 넣겠습니다.

> ⚠️ 8단계는 파일 수가 늘어납니다. 아래는 “컴파일 가능한 최소 완전체” 기준으로 추가/수정되는 파일을 전부 풀코드로 제공합니다.
(실제 프로젝트에 이미 있는 코드와 충돌할 수 있는 부분은 “수정본”을 그대로 붙여넣을 수 있게 구성했습니다.)




---

8단계 추가/수정 트리(전체)

추가(Go)

신호 수집(툴/테스트)

internal/signal/types.go

internal/signal/tool_observer.go

internal/signal/test_runner.go


저장소(SQLite)

internal/storage/sqlite/toolruns.go

internal/storage/sqlite/testruns.go

internal/storage/sqlite/migrations/0005_toolruns.sql

internal/storage/sqlite/migrations/0006_testruns.sql


품질평가 강화(DoD 반영)

internal/quality/signals.go (신호 조회 인터페이스)

internal/quality/evaluator_v2.go (기존 evaluator.go를 유지해도 되지만, 여기서는 교체 버전 제공)

internal/quality/recorder.go (수정: signals/bench token 등 반영)


인증/권한

internal/authz/authz.go

internal/server/middleware/auth.go


수정(Go)

internal/server/routes/stream.go (OAuth 인증 + workspace 권한 체크 + backpressure 개선)

internal/tool/registry.go (tool observer hook 추가)

internal/session/processor.go (bench 결과(토큰/latency)를 Quality로 전달)



---

8-1) DB 마이그레이션 (툴 실행/테스트 실행)

internal/storage/sqlite/migrations/0005_toolruns.sql

CREATE TABLE IF NOT EXISTS tool_runs (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,
  session_id TEXT,
  task_id TEXT,

  tool_name TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at INTEGER NOT NULL,
  duration_ms INTEGER NOT NULL,

  success INTEGER NOT NULL DEFAULT 1,
  error TEXT,

  input_summary TEXT,
  output_summary TEXT,

  output_bytes INTEGER NOT NULL DEFAULT 0,
  tags TEXT
);

CREATE INDEX IF NOT EXISTS idx_tool_runs_ws_time ON tool_runs(workspace, started_at);
CREATE INDEX IF NOT EXISTS idx_tool_runs_tool ON tool_runs(tool_name, started_at);

internal/storage/sqlite/migrations/0006_testruns.sql

CREATE TABLE IF NOT EXISTS test_runs (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,
  project_id TEXT,
  session_id TEXT,
  task_id TEXT,

  runner TEXT NOT NULL,          -- "go test", "npm test", "gradle test" 등
  command TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  ended_at INTEGER NOT NULL,
  duration_ms INTEGER NOT NULL,

  success INTEGER NOT NULL DEFAULT 1,
  exit_code INTEGER NOT NULL DEFAULT 0,
  error TEXT,

  stdout_summary TEXT,
  stderr_summary TEXT,
  tags TEXT
);

CREATE INDEX IF NOT EXISTS idx_test_runs_ws_time ON test_runs(workspace, started_at);
CREATE INDEX IF NOT EXISTS idx_test_runs_runner ON test_runs(runner, started_at);


---

8-2) Signal 공통 타입

internal/signal/types.go

package signal

import "time"

type ToolRun struct {
	ID        string
	Workspace string
	SessionID string
	TaskID    string

	ToolName   string
	StartedAt  time.Time
	EndedAt    time.Time
	DurationMs int64

	Success bool
	Error   string

	InputSummary  string
	OutputSummary string

	OutputBytes int64
	Tags        string // csv or json string (MVP)
}

type TestRun struct {
	ID        string
	Workspace string
	ProjectID string
	SessionID string
	TaskID    string

	Runner     string
	Command    string
	StartedAt  time.Time
	EndedAt    time.Time
	DurationMs int64

	Success   bool
	ExitCode  int
	Error     string
	StdoutSum string
	StderrSum string
	Tags      string
}


---

8-3) Tool Observer: tool 실행 전/후 자동 기록

internal/signal/tool_observer.go

package signal

import (
	"context"
	"strings"
	"time"

	"devorch/internal/id"
)

type ToolRunWriter interface {
	InsertToolRun(ctx context.Context, r ToolRun) (string, error)
}

type ToolObserver struct {
	Store ToolRunWriter
}

type ToolStart struct {
	Workspace string
	SessionID string
	TaskID    string
	ToolName  string
	Input     string // raw json or stringified (MVP)
	Time      time.Time
}

type ToolEnd struct {
	Start   ToolStart
	Success bool
	Error   error
	Output  []byte // raw output bytes
	Time    time.Time
}

func NewToolObserver(store ToolRunWriter) *ToolObserver {
	return &ToolObserver{Store: store}
}

func (o *ToolObserver) OnToolEnd(ctx context.Context, end ToolEnd) {
	if o == nil || o.Store == nil {
		return
	}

	inSum := summarize(end.Start.Input, 800)
	outSum := summarize(string(end.Output), 1200)

	tr := ToolRun{
		ID:           id.NewULID(),
		Workspace:    end.Start.Workspace,
		SessionID:    end.Start.SessionID,
		TaskID:       end.Start.TaskID,
		ToolName:     end.Start.ToolName,
		StartedAt:    end.Start.Time,
		EndedAt:      end.Time,
		DurationMs:   end.Time.Sub(end.Start.Time).Milliseconds(),
		Success:      end.Success,
		Error:        errString(end.Error),
		InputSummary: inSum,
		OutputSummary: outSum,
		OutputBytes:  int64(len(end.Output)),
		Tags:         "",
	}

	_, _ = o.Store.InsertToolRun(ctx, tr)
}

func summarize(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func errString(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}


---

8-4) Test Runner: “완료 기준” 시그널 수집 (MVP)

> DevOrch가 “작업 완료”를 판단할 때, 단순히 LLM 답변이 아니라 테스트/빌드/린트 결과를 시그널로 남깁니다.
여기서는 MVP로 “명령 실행 + 요약 저장”만 제공합니다. (세부 파서/리포트는 9단계)



internal/signal/test_runner.go

package signal

import (
	"bytes"
	"context"
	"os/exec"
	"time"

	"devorch/internal/id"
)

type TestRunWriter interface {
	InsertTestRun(ctx context.Context, r TestRun) (string, error)
}

type TestRunner struct {
	Store TestRunWriter
}

func NewTestRunner(store TestRunWriter) *TestRunner {
	return &TestRunner{Store: store}
}

type RunTestsRequest struct {
	Workspace string
	ProjectID string
	SessionID string
	TaskID    string

	Runner  string // label
	Command string // full shell-like, but we will split minimal
	Dir     string
	Env     []string
	Timeout time.Duration
}

func (r *TestRunner) Run(ctx context.Context, req RunTestsRequest) (TestRun, error) {
	start := time.Now()
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	// MVP: exec.CommandContext with "sh -lc" (windows는 9단계에서 분기)
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, "sh", "-lc", req.Command)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}
	if len(req.Env) > 0 {
		cmd.Env = append(cmd.Environ(), req.Env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	run := TestRun{
		ID:        id.NewULID(),
		Workspace: req.Workspace,
		ProjectID: req.ProjectID,
		SessionID: req.SessionID,
		TaskID:    req.TaskID,
		Runner:    req.Runner,
		Command:   req.Command,
		StartedAt: start,
	}

	err := cmd.Run()
	end := time.Now()

	exitCode := 0
	success := err == nil

	if err != nil {
		// exit code best-effort
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = 1
		}
	}

	run.EndedAt = end
	run.DurationMs = end.Sub(start).Milliseconds()
	run.Success = success
	run.ExitCode = exitCode
	run.Error = errString(err)
	run.StdoutSum = summarize(stdout.String(), 2000)
	run.StderrSum = summarize(stderr.String(), 2000)

	if r.Store != nil {
		_, _ = r.Store.InsertTestRun(ctx, run)
	}

	return run, err
}


---

8-5) SQLite 저장 구현

internal/storage/sqlite/toolruns.go

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/signal"
)

type ToolRunStore struct {
	DB *sql.DB
}

func (s *ToolRunStore) InsertToolRun(ctx context.Context, r signal.ToolRun) (string, error) {
	_, err := s.DB.ExecContext(ctx, `
	  INSERT INTO tool_runs (
	    id, workspace, session_id, task_id,
	    tool_name, started_at, ended_at, duration_ms,
	    success, error, input_summary, output_summary, output_bytes, tags
	  ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		r.ID, r.Workspace, r.SessionID, r.TaskID,
		r.ToolName, r.StartedAt.UnixMilli(), r.EndedAt.UnixMilli(), r.DurationMs,
		boolToInt(r.Success), r.Error, r.InputSummary, r.OutputSummary, r.OutputBytes, r.Tags,
	)
	return r.ID, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type ToolAgg struct {
	ToolName string
	Count    int
	FailRate float64
	AvgMs    float64
}

func (s *ToolRunStore) RecentToolAgg(ctx context.Context, workspace string, limit int) ([]ToolAgg, error) {
	rows, err := s.DB.QueryContext(ctx, `
	  SELECT tool_name,
	         COUNT(*) as cnt,
	         AVG(CASE WHEN success=1 THEN 0 ELSE 1 END) as fail_rate,
	         AVG(duration_ms) as avg_ms
	  FROM tool_runs
	  WHERE workspace=?
	  GROUP BY tool_name
	  ORDER BY cnt DESC
	  LIMIT ?
	`, workspace, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ToolAgg{}
	for rows.Next() {
		var a ToolAgg
		if err := rows.Scan(&a.ToolName, &a.Count, &a.FailRate, &a.AvgMs); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// helper for reading recent runs (quality evaluator needs latest)
func (s *ToolRunStore) RecentRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.ToolRun, error) {
	rows, err := s.DB.QueryContext(ctx, `
	  SELECT id, workspace, session_id, task_id, tool_name,
	         started_at, ended_at, duration_ms, success, error,
	         input_summary, output_summary, output_bytes, tags
	  FROM tool_runs
	  WHERE workspace=? AND (session_id=? OR ?="") AND (task_id=? OR ?="")
	  ORDER BY started_at DESC
	  LIMIT ?
	`, workspace, sessionID, sessionID, taskID, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []signal.ToolRun
	for rows.Next() {
		var r signal.ToolRun
		var startedMs, endedMs int64
		var successInt int
		if err := rows.Scan(
			&r.ID, &r.Workspace, &r.SessionID, &r.TaskID, &r.ToolName,
			&startedMs, &endedMs, &r.DurationMs, &successInt, &r.Error,
			&r.InputSummary, &r.OutputSummary, &r.OutputBytes, &r.Tags,
		); err != nil {
			return nil, err
		}
		r.Success = successInt == 1
		r.StartedAt = time.UnixMilli(startedMs)
		r.EndedAt = time.UnixMilli(endedMs)
		out = append(out, r)
	}
	return out, nil
}

internal/storage/sqlite/testruns.go

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/signal"
)

type TestRunStore struct {
	DB *sql.DB
}

func (s *TestRunStore) InsertTestRun(ctx context.Context, r signal.TestRun) (string, error) {
	_, err := s.DB.ExecContext(ctx, `
	  INSERT INTO test_runs (
	    id, workspace, project_id, session_id, task_id,
	    runner, command, started_at, ended_at, duration_ms,
	    success, exit_code, error, stdout_summary, stderr_summary, tags
	  ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		r.ID, r.Workspace, r.ProjectID, r.SessionID, r.TaskID,
		r.Runner, r.Command, r.StartedAt.UnixMilli(), r.EndedAt.UnixMilli(), r.DurationMs,
		boolToInt(r.Success), r.ExitCode, r.Error, r.StdoutSum, r.StderrSum, r.Tags,
	)
	return r.ID, err
}

func (s *TestRunStore) RecentRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.TestRun, error) {
	rows, err := s.DB.QueryContext(ctx, `
	  SELECT id, workspace, project_id, session_id, task_id,
	         runner, command, started_at, ended_at, duration_ms,
	         success, exit_code, error, stdout_summary, stderr_summary, tags
	  FROM test_runs
	  WHERE workspace=? AND (session_id=? OR ?="") AND (task_id=? OR ?="")
	  ORDER BY started_at DESC
	  LIMIT ?
	`, workspace, sessionID, sessionID, taskID, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []signal.TestRun
	for rows.Next() {
		var r signal.TestRun
		var startedMs, endedMs int64
		var successInt int
		if err := rows.Scan(
			&r.ID, &r.Workspace, &r.ProjectID, &r.SessionID, &r.TaskID,
			&r.Runner, &r.Command, &startedMs, &endedMs, &r.DurationMs,
			&successInt, &r.ExitCode, &r.Error, &r.StdoutSum, &r.StderrSum, &r.Tags,
		); err != nil {
			return nil, err
		}
		r.Success = successInt == 1
		r.StartedAt = time.UnixMilli(startedMs)
		r.EndedAt = time.UnixMilli(endedMs)
		out = append(out, r)
	}
	return out, nil
}


---

8-6) 품질평가에 “툴/테스트 시그널” 반영

internal/quality/signals.go

package quality

import (
	"context"

	"devorch/internal/signal"
)

type SignalReader interface {
	RecentToolRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.ToolRun, error)
	RecentTestRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.TestRun, error)
}

internal/quality/evaluator_v2.go

package quality

import (
	"context"
	"strings"
	"time"

	"devorch/internal/signal"
)

type EvaluatorV2 struct {
	Signals SignalReader
}

func NewEvaluatorV2(signals SignalReader) *EvaluatorV2 {
	return &EvaluatorV2{Signals: signals}
}

// 8단계 핵심: “완료 기준” 기반 점수
// - 테스트 성공(가산), 테스트 실패(대감점)
// - tool 실패율/주요 tool 실패(감점)
// - 에러 없고, 테스트 통과하고, tool 에러 적으면 높은 점수
func (e *EvaluatorV2) Evaluate(ctx context.Context, req EvalRequest, latency time.Duration, totalTokens int) EvalResult {
	if req.Error != nil {
		return EvalResult{Success: false, Score: 0.0, Notes: "provider_error", Error: errString(req.Error)}
	}

	ans := strings.TrimSpace(req.Answer)
	if ans == "" {
		return EvalResult{Success: false, Score: 0.0, Notes: "empty_answer"}
	}

	score := 0.45 // base

	// 응답 품질(간단 휴리스틱)
	if len(ans) > 600 {
		score += 0.15
	}
	if strings.Count(strings.ToUpper(ans), "TODO") >= 3 {
		score -= 0.15
	}

	// latency/token 페널티
	if latency > 30*time.Second {
		score -= 0.08
	}
	if totalTokens > 8000 {
		score -= 0.05
	}

	// Signals: ToolRuns + TestRuns
	var toolRuns []signal.ToolRun
	var testRuns []signal.TestRun
	if e.Signals != nil {
		toolRuns, _ = e.Signals.RecentToolRuns(ctx, req.Workspace, req.SessionID, req.TaskID, 30)
		testRuns, _ = e.Signals.RecentTestRuns(ctx, req.Workspace, req.SessionID, req.TaskID, 10)
	}

	// 테스트 우선
	if len(testRuns) == 0 {
		// 테스트가 없으면: 보수적으로 약간 감점 (DoD 부족)
		score -= 0.07
	} else {
		latest := testRuns[0]
		if latest.Success {
			score += 0.25
		} else {
			score -= 0.35
		}
	}

	// tool 실패율 반영
	if len(toolRuns) > 0 {
		fail := 0
		for _, tr := range toolRuns {
			if !tr.Success {
				fail++
			}
		}
		failRate := float64(fail) / float64(len(toolRuns))
		score -= (failRate * 0.20)

		// 주요 도구 실패(예: patch/apply, edit, exec 등) 추가 감점
		for _, tr := range toolRuns {
			if tr.Success {
				continue
			}
			if strings.Contains(tr.ToolName, "apply_patch") || strings.Contains(tr.ToolName, "edit") {
				score -= 0.08
				break
			}
		}
	}

	// success 판단: 점수 기반
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	success := score >= 0.55
	notes := "dod_signals"
	return EvalResult{Success: success, Score: score, Notes: notes}
}

internal/quality/recorder.go (8단계 수정본)

package quality

import (
	"context"
	"time"

	"devorch/internal/storage/sqlite"
)

type Store interface {
	Insert(ctx context.Context, r sqlite.QualityRun) (string, error)
}

type Recorder struct {
	Store   Store
	EvalV2  *EvaluatorV2
	Signals SignalReader
}

func NewRecorder(store Store, signals SignalReader) *Recorder {
	return &Recorder{
		Store:   store,
		Signals: signals,
		EvalV2:  NewEvaluatorV2(signals),
	}
}

type EvalMeta struct {
	Latency    time.Duration
	TotalTokens int
}

func (r *Recorder) EvaluateAndRecord(ctx context.Context, req EvalRequest, meta EvalMeta) error {
	lat := meta.Latency
	totalTok := meta.TotalTokens

	res := r.EvalV2.Evaluate(ctx, req, lat, totalTok)

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

8-7) Tool Registry에 Observer Hook 추가 (핵심)

> 기존 internal/tool/registry.go가 tool execute를 담당한다고 가정하고, “실행 후 observer 기록”을 추가합니다.



internal/tool/registry.go (8단계 완성본 예시)

package tool

import (
	"context"
	"encoding/json"
	"time"

	"devorch/internal/signal"
)

type Tool interface {
	Name() string
	Execute(ctx context.Context, input json.RawMessage) (any, error)
}

type Registry struct {
	tools map[string]Tool

	Observer *signal.ToolObserver
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *Registry) Execute(ctx context.Context, workspace, sessionID, taskID, name string, input json.RawMessage) (any, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, ErrToolNotFound
	}

	start := time.Now()
	ts := signal.ToolStart{
		Workspace: workspace,
		SessionID: sessionID,
		TaskID:    taskID,
		ToolName:  name,
		Input:     string(input),
		Time:      start,
	}

	out, err := t.Execute(ctx, input)

	// marshal output to bytes for summary
	var outBytes []byte
	if out != nil {
		if b, mErr := json.Marshal(out); mErr == nil {
			outBytes = b
		}
	}

	if r.Observer != nil {
		r.Observer.OnToolEnd(ctx, signal.ToolEnd{
			Start:   ts,
			Success: err == nil,
			Error:   err,
			Output:  outBytes,
			Time:    time.Now(),
		})
	}

	return out, err
}

internal/tool/errors.go (없다면 추가)

package tool

import "errors"

var ErrToolNotFound = errors.New("tool not found")


---

8-8) Session Processor: Bench 메타를 Quality로 전달

> 7단계에서는 latency/token을 quality에 제대로 전달하지 않았습니다. 8단계에서 전달합니다.
(Bench.Recorder가 WrapChat에서 latency/tokens를 반환하도록 이미 6단계에서 설계했다고 가정)



internal/session/processor.go (8단계 수정본)

package session

import (
	"bytes"
	"context"
	"time"

	"devorch/internal/bus"
	"devorch/internal/provider"
	"devorch/internal/quality"
)

type Bench interface {
	WrapChat(ctx context.Context, req provider.ChatRequest, fn func() (provider.ChatResponse, error)) (provider.ChatResponse, time.Duration, int, error)
}

type Processor struct {
	Providers *provider.Registry
	Bench     Bench
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
				if ch.Done && p.Hub != nil {
					p.Hub.Publish(BuildDoneEvent(req.Workspace, req.SessionID, req.TaskID, full.String()))
				}
				return nil
			})
			resp.Text = full.String()
			return resp, err
		}

		resp, err := c.Chat(ctx, req)
		if err == nil && p.Hub != nil {
			p.Hub.Publish(BuildDoneEvent(req.Workspace, req.SessionID, req.TaskID, resp.Text))
		}
		if err != nil && p.Hub != nil {
			p.Hub.Publish(BuildErrorEvent(req.Workspace, req.SessionID, req.TaskID, err.Error()))
		}
		return resp, err
	}

	var (
		resp provider.ChatResponse
		err  error
		lat  time.Duration
		tok  int
	)

	if p.Bench != nil {
		resp, lat, tok, err = p.Bench.WrapChat(ctx, req, call)
	} else {
		start := time.Now()
		resp, err = call()
		lat = time.Since(start)
		tok = 0
	}

	// Quality eval: DoD signals 반영
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
		_ = p.Quality.EvaluateAndRecord(ctx, qreq, quality.EvalMeta{
			Latency:     lat,
			TotalTokens: tok,
		})
	}

	return resp, err
}


---

8-9) OAuth 인증 + Workspace 권한 체크 (Stream 포함)

internal/authz/authz.go

package authz

import "context"

type Principal struct {
	UserID string
	Email  string
}

type Authorizer interface {
	Authenticate(ctx context.Context, bearer string) (*Principal, error)
	CanAccessWorkspace(ctx context.Context, p *Principal, workspace string) (bool, error)
}

internal/server/middleware/auth.go

package middleware

import (
	"net/http"
	"strings"

	"devorch/internal/authz"
)

type key int

const principalKey key = 1

func WithAuthz(a authz.Authorizer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authorization: Bearer xxx
		h := r.Header.Get("Authorization")
		if h == "" {
			http.Error(w, "missing authorization", http.StatusUnauthorized)
			returns(w)
			return
		}
		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "invalid authorization", http.StatusUnauthorized)
			return
		}
		p, err := a.Authenticate(r.Context(), parts[1])
		if err != nil || p == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := contextWithPrincipal(r.Context(), p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func contextWithPrincipal(ctx context.Context, p *authz.Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

func PrincipalFromContext(r *http.Request) (*authz.Principal, bool) {
	p, ok := r.Context().Value(principalKey).(*authz.Principal)
	return p, ok
}

> 위 코드에서 context import가 필요합니다. 아래 “완성본”으로 제공합니다.



internal/server/middleware/auth.go (완성 import 포함)

package middleware

import (
	"context"
	"net/http"
	"strings"

	"devorch/internal/authz"
)

type key int

const principalKey key = 1

func WithAuthz(a authz.Authorizer, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			http.Error(w, "missing authorization", http.StatusUnauthorized)
			return
		}
		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "invalid authorization", http.StatusUnauthorized)
			return
		}
		p, err := a.Authenticate(r.Context(), parts[1])
		if err != nil || p == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), principalKey, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func PrincipalFromRequest(r *http.Request) (*authz.Principal, bool) {
	p, ok := r.Context().Value(principalKey).(*authz.Principal)
	return p, ok
}


---

8-10) Stream Route: OAuth + 권한 + backpressure(드롭 최소화)

internal/server/routes/stream.go (8단계 완성본)

package routes

import (
	"net/http"
	"time"

	"devorch/internal/authz"
	"devorch/internal/bus"
	"devorch/internal/server/middleware"
)

type StreamRoutes struct {
	Hub  *bus.Hub
	Auth authz.Authorizer
}

func (r *StreamRoutes) Register(mux *http.ServeMux) {
	// /v1/stream 은 반드시 인증
	mux.Handle("/v1/stream", middleware.WithAuthz(r.Auth, http.HandlerFunc(r.handleStream)))
}

func (r *StreamRoutes) handleStream(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	p, ok := middleware.PrincipalFromRequest(req)
	if !ok || p == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	workspace := req.URL.Query().Get("workspace")
	if workspace == "" {
		http.Error(w, "missing workspace", http.StatusBadRequest)
		return
	}
	allowed, err := r.Auth.CanAccessWorkspace(req.Context(), p, workspace)
	if err != nil || !allowed {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	sessionID := req.URL.Query().Get("session_id")
	taskID := req.URL.Query().Get("task_id")

	sub := r.Hub.Subscribe("sse-"+time.Now().Format("150405.000"), bus.TopicSessionStream, bus.TopicSystem)
	defer sub.Close()

	// backpressure: subscriber channel을 그대로 쓰면 느린 클라이언트에서 누락이 생길 수 있음
	// MVP: local buffer(최대 N)로 흡수 후 flush
	buffer := make([]bus.Event, 0, 64)

	bus.WriteSSEComment(w, "connected")
	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()

	flush := func() {
		for _, ev := range buffer {
			_ = bus.WriteSSE(w, ev)
		}
		buffer = buffer[:0]
	}

	for {
		select {
		case <-req.Context().Done():
			return
		case <-tick.C:
			bus.WriteSSEComment(w, "ping")
			flush()
		case ev, ok := <-sub.Ch:
			if !ok {
				return
			}
			if ev.Workspace != workspace {
				continue
			}
			if sessionID != "" && ev.SessionID != sessionID {
				continue
			}
			if taskID != "" && ev.TaskID != taskID {
				continue
			}

			if len(buffer) < cap(buffer) {
				buffer = append(buffer, ev)
			} else {
				// buffer 가득 찼으면 즉시 flush 후 추가
				flush()
				buffer = append(buffer, ev)
			}

			// done/error는 바로 flush
			if ev.Type == "done" || ev.Type == "error" {
				flush()
			}
		}
	}
}


---

8-11) “Signals Reader”를 Quality에 연결하기 위한 구현

> quality.SignalReader는 저장소를 통해 tool_runs/test_runs를 읽어야 합니다.
아래는 “어댑터”입니다.



internal/quality/signals_adapter.go (추가)

package quality

import (
	"context"

	"devorch/internal/signal"
)

type ToolRunReader interface {
	RecentRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.ToolRun, error)
}
type TestRunReader interface {
	RecentRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.TestRun, error)
}

type SignalAdapter struct {
	Tools ToolRunReader
	Tests TestRunReader
}

func (a *SignalAdapter) RecentToolRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.ToolRun, error) {
	if a == nil || a.Tools == nil {
		return nil, nil
	}
	return a.Tools.RecentRuns(ctx, workspace, sessionID, taskID, limit)
}

func (a *SignalAdapter) RecentTestRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.TestRun, error) {
	if a == nil || a.Tests == nil {
		return nil, nil
	}
	return a.Tests.RecentRuns(ctx, workspace, sessionID, taskID, limit)
}


---

8단계 “조립 포인트”(DI/앱 초기화에서 연결해야 하는 것)

8단계에서 “실제로 동작”하려면 초기화 시 아래 연결이 필요합니다:

1. SQLite DB에 0005_toolruns.sql, 0006_testruns.sql 마이그레이션 실행


2. ToolRunStore, TestRunStore 생성


3. signal.ToolObserver를 tool.Registry.Observer에 연결


4. quality.SignalAdapter(Tools=ToolRunStore, Tests=TestRunStore) 생성


5. quality.NewRecorder(qualityStore, signalAdapter)로 Recorder 생성


6. routes.StreamRoutes.Auth에 실제 authz.Authorizer 구현체 연결



> authz.Authorizer 구현은 9단계에서 “실제 OAuth 세션/토큰 저장소(auth/session.go)”와 완전 결합해서 제공합니다.
8단계 MVP에서는 인터페이스까지 포함했고, 연결만 하면 됩니다.




---

여기까지가 8단계 “풀코드” 범위입니다

원하시면 바로 9단계로 이어서:

Windows에서도 TestRunner가 powershell -Command로 동작하도록 OS 분기

tool 결과를 “정확한 구조화” (예: patch 적용 성공/실패, 파일 변경 수, 진단 수)

테스트 리포트 파싱(JUnit/Go test JSON 등) → quality 점수 더 정교화

OAuth 세션/토큰 + workspace/role 기반 authz.Authorizer 실제 구현

라우터가 quality/test/tool 시그널을 모델 선택에 더 강하게 반영


라고 가면 됩니다.

다음은 9단계로 진행할까요, 아니면 8단계에서 바로 필요한 “앱 초기화(wire/app.go) 조립 코드”까지 이번 단계에 추가로 같이 붙여드릴까요?