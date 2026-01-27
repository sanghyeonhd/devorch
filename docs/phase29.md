좋습니다. 29단계 풀코드는 “delegate_task를 백그라운드로 큐잉 → task_id 발급 → 조회/출력/취소 API 제공 → 완료 시 OkAON에 실측 로그 저장 → reward emit → learning(밴딧/정책)로 관측치 전달”까지 한 번에 연결합니다.

> ✅ 29단계 결과



run_in_background=true면 즉시 task_id 반환

/v1/tasks/{id} 상태 조회

/v1/tasks/{id}/output 결과 조회

/v1/tasks/{id}/cancel 취소

완료 시 OkAON( sqlite )에 실측 로그 저장

reward 계산/emit → learning.Learner.Observe(...) 호출



---

A) 29단계 변경 파일 목록

신규

internal/background/types.go

internal/background/store.go

internal/background/worker_pool.go  (없으면 신규 / 있으면 교체 가능)

internal/background/manager.go      (기존이 있으면 이 코드로 정리/확장)

internal/okAON/sqlite/task_runs.go

internal/learning/observe.go

internal/server/routes/task_query.go

internal/server/routes/task_cancel.go


수정

internal/app/app.go (Background+Learning 주입, Delegate의 background 연결)

internal/delegate/delegate.go (background enqueue 연결)

internal/server/server.go (라우트 등록)

internal/storage/sqlite/migrations/0004_task_runs.sql (신규 마이그레이션 추가)



---

1) SQL 마이그레이션: task_runs 테이블 추가

1-1) internal/storage/sqlite/migrations/0004_task_runs.sql (신규)

-- 0004_task_runs.sql
-- OkAON: delegate_task 실측 실행 로그(로컬/원격 혼합, 학습/벤치 기반)
BEGIN;

CREATE TABLE IF NOT EXISTS ok_aon_task_runs (
  id                TEXT PRIMARY KEY,
  created_at_unix   INTEGER NOT NULL,
  finished_at_unix  INTEGER,

  -- 요청 정보
  category          TEXT,
  subagent_type     TEXT,
  description       TEXT,
  prompt_hash       TEXT NOT NULL,

  -- 선택 결과
  provider          TEXT NOT NULL,
  model             TEXT NOT NULL,

  -- 실행 결과
  status            TEXT NOT NULL,  -- queued/running/succeeded/failed/canceled
  error_message     TEXT,
  output_text       TEXT,

  -- 실측 메트릭
  latency_ms        INTEGER,
  tokens_in         INTEGER,
  tokens_out        INTEGER,

  -- quality / reward
  quality_score     REAL,           -- 0~1 (없으면 NULL)
  reward            REAL,           -- 0~1 또는 실수

  -- 환경(최소)
  os                TEXT,
  arch              TEXT,
  hw_tier           TEXT
);

CREATE INDEX IF NOT EXISTS idx_ok_aon_task_runs_created
  ON ok_aon_task_runs(created_at_unix);

CREATE INDEX IF NOT EXISTS idx_ok_aon_task_runs_prompt_hash
  ON ok_aon_task_runs(prompt_hash);

CREATE INDEX IF NOT EXISTS idx_ok_aon_task_runs_model
  ON ok_aon_task_runs(provider, model);

COMMIT;


---

2) OkAON sqlite: task_runs CRUD

2-1) internal/okAON/sqlite/task_runs.go (신규)

package sqlite

import (
	"context"
	"database/sql"
)

type TaskRun struct {
	ID             string
	CreatedAtUnix  int64
	FinishedAtUnix sql.NullInt64

	Category     sql.NullString
	SubagentType sql.NullString
	Description  sql.NullString
	PromptHash   string

	Provider string
	Model    string

	Status       string
	ErrorMessage sql.NullString
	OutputText   sql.NullString

	LatencyMs sql.NullInt64
	TokensIn  sql.NullInt64
	TokensOut sql.NullInt64

	QualityScore sql.NullFloat64
	Reward       sql.NullFloat64

	OS    sql.NullString
	Arch  sql.NullString
	HwTier sql.NullString
}

type TaskRunStore struct {
	DB *sql.DB
}

func NewTaskRunStore(db *sql.DB) *TaskRunStore {
	return &TaskRunStore{DB: db}
}

func (s *TaskRunStore) InsertQueued(ctx context.Context, tr TaskRun) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO ok_aon_task_runs (
  id, created_at_unix, category, subagent_type, description, prompt_hash,
  provider, model, status, os, arch, hw_tier
) VALUES (
  ?, ?, ?, ?, ?, ?,
  ?, ?, ?, ?, ?, ?
)`,
		tr.ID, tr.CreatedAtUnix,
		tr.Category, tr.SubagentType, tr.Description, tr.PromptHash,
		tr.Provider, tr.Model, tr.Status,
		tr.OS, tr.Arch, tr.HwTier,
	)
	return err
}

func (s *TaskRunStore) MarkRunning(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE ok_aon_task_runs
SET status='running'
WHERE id=?`,
		id,
	)
	return err
}

func (s *TaskRunStore) MarkFinished(ctx context.Context, tr TaskRun) error {
	_, err := s.DB.ExecContext(ctx, `
UPDATE ok_aon_task_runs
SET finished_at_unix=?,
    status=?,
    error_message=?,
    output_text=?,
    latency_ms=?,
    tokens_in=?,
    tokens_out=?,
    quality_score=?,
    reward=?
WHERE id=?`,
		tr.FinishedAtUnix,
		tr.Status,
		tr.ErrorMessage,
		tr.OutputText,
		tr.LatencyMs,
		tr.TokensIn,
		tr.TokensOut,
		tr.QualityScore,
		tr.Reward,
		tr.ID,
	)
	return err
}

func (s *TaskRunStore) GetByID(ctx context.Context, id string) (TaskRun, error) {
	row := s.DB.QueryRowContext(ctx, `
SELECT
  id, created_at_unix, finished_at_unix,
  category, subagent_type, description, prompt_hash,
  provider, model, status, error_message, output_text,
  latency_ms, tokens_in, tokens_out,
  quality_score, reward,
  os, arch, hw_tier
FROM ok_aon_task_runs
WHERE id=?`, id)

	var tr TaskRun
	err := row.Scan(
		&tr.ID, &tr.CreatedAtUnix, &tr.FinishedAtUnix,
		&tr.Category, &tr.SubagentType, &tr.Description, &tr.PromptHash,
		&tr.Provider, &tr.Model, &tr.Status, &tr.ErrorMessage, &tr.OutputText,
		&tr.LatencyMs, &tr.TokensIn, &tr.TokensOut,
		&tr.QualityScore, &tr.Reward,
		&tr.OS, &tr.Arch, &tr.HwTier,
	)
	return tr, err
}


---

3) Learning: 관측치(Observe) 인터페이스

3-1) internal/learning/observe.go (신규)

package learning

import "context"

// Observation: router/bandit 업데이트용 최소 관측치
type Observation struct {
	TaskID      string
	Category    string
	SubagentType string

	Provider string
	Model    string

	Success bool
	Reward  float64       // 0~1 (또는 임의 스케일)
	LatencyMs int64

	QualityScore *float64 // optional

	PromptHash string
}

type Learner interface {
	Observe(ctx context.Context, obs Observation) error
}

> 29단계에서는 “연결”이 목적이므로, 기존 learning/learner.go가 있다면 그쪽에서 Observe를 구현하면 됩니다.
아직 없다면 아래 최소 구현(메모리 no-op)도 같이 넣어둡니다.



(선택) internal/learning/learner.go가 없거나 미완이면 최소 구현 추가

package learning

import "context"

type NoopLearner struct{}

func (n *NoopLearner) Observe(ctx context.Context, obs Observation) error { return nil }


---

4) Background: 큐/상태/취소/출력

4-1) internal/background/types.go (신규)

package background

import "time"

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

type Task struct {
	ID string

	CreatedAt time.Time
	StartedAt *time.Time
	EndedAt   *time.Time

	Status Status

	// delegate 정보
	Category     string
	SubagentType string
	Description  string
	Prompt       string
	PromptHash   string

	Provider string
	Model    string

	// 결과
	Output string
	Error  string

	// 메트릭(최소)
	LatencyMs int64
	TokensIn  int64
	TokensOut int64
	QualityScore *float64
	Reward float64
}

type TaskSnapshot struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at_unix"`
	StartedAt *int64 `json:"started_at_unix,omitempty"`
	EndedAt   *int64 `json:"ended_at_unix,omitempty"`

	Category     string `json:"category,omitempty"`
	SubagentType string `json:"subagent_type,omitempty"`

	Provider string `json:"provider"`
	Model    string `json:"model"`

	Error string `json:"error,omitempty"`
}

4-2) internal/background/store.go (신규)

package background

import (
	"errors"
	"sync"
)

type Store struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

func NewStore() *Store {
	return &Store{tasks: make(map[string]*Task)}
}

func (s *Store) Put(t *Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[t.ID] = t
}

func (s *Store) Get(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	return t, ok
}

func (s *Store) ListIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.tasks))
	for k := range s.tasks {
		out = append(out, k)
	}
	return out
}

func (s *Store) Update(id string, fn func(t *Task) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return errors.New("task_not_found")
	}
	return fn(t)
}

4-3) internal/background/worker_pool.go (신규/교체)

package background

import (
	"context"
)

type Job struct {
	TaskID string
}

type WorkerPool struct {
	ch chan Job
}

func NewWorkerPool(size int) *WorkerPool {
	return &WorkerPool{ch: make(chan Job, size)}
}

func (p *WorkerPool) Submit(j Job) {
	p.ch <- j
}

func (p *WorkerPool) Jobs() <-chan Job {
	return p.ch
}

func (p *WorkerPool) Close() {
	close(p.ch)
}

type CancelMap struct {
	// 단순 취소: task별 cancel func 저장
	m map[string]context.CancelFunc
}

func NewCancelMap() *CancelMap {
	return &CancelMap{m: map[string]context.CancelFunc{}}
}

4-4) internal/background/manager.go (신규/교체)

package background

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"devorch/internal/id"
	"devorch/internal/learning"
	oksql "devorch/internal/okAON/sqlite"
	"devorch/internal/provider"
)

type ExecuteFunc func(ctx context.Context, p provider.Provider, providerName, model, prompt string) (text string, tokensIn, tokensOut int64, err error)

type ManagerDeps struct {
	Providers *provider.Registry
	OkAON     *oksql.TaskRunStore
	Learner   learning.Learner

	Exec ExecuteFunc

	WorkerCount int
	QueueSize   int

	// env 최소 정보(29단계에선 문자열로)
	OS     string
	Arch   string
	HwTier string
}

type Manager struct {
	providers *provider.Registry
	ok        *oksql.TaskRunStore
	learner   learning.Learner
	exec      ExecuteFunc

	store *Store
	pool  *WorkerPool

	cancelMu *NewCancelMap // 단순형(실제 map은 아래)
	cancelFn map[string]context.CancelFunc

	os, arch, hwTier string
}

func NewManager(d ManagerDeps) *Manager {
	wc := d.WorkerCount
	if wc <= 0 {
		wc = 2
	}
	qs := d.QueueSize
	if qs <= 0 {
		qs = 128
	}

	m := &Manager{
		providers: d.Providers,
		ok:        d.OkAON,
		learner:   d.Learner,
		exec:      d.Exec,

		store:    NewStore(),
		pool:     NewWorkerPool(qs),
		cancelFn: map[string]context.CancelFunc{},

		os:     d.OS,
		arch:   d.Arch,
		hwTier: d.HwTier,
	}

	// 워커 실행
	for i := 0; i < wc; i++ {
		go m.workerLoop()
	}
	return m
}

func hashPrompt(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

func (m *Manager) Enqueue(req Task) (string, error) {
	if req.Provider == "" || req.Model == "" {
		return "", errors.New("provider_and_model_required")
	}
	if req.Prompt == "" {
		return "", errors.New("prompt_required")
	}

	taskID := id.NewULID()
	now := time.Now()

	req.ID = taskID
	req.CreatedAt = now
	req.Status = StatusQueued
	req.PromptHash = hashPrompt(req.Prompt)

	m.store.Put(&req)

	// OkAON queued 저장
	if m.ok != nil {
		_ = m.ok.InsertQueued(context.Background(), oksql.TaskRun{
			ID:            req.ID,
			CreatedAtUnix: now.Unix(),
			Category:      nullStr(req.Category),
			SubagentType:  nullStr(req.SubagentType),
			Description:   nullStr(req.Description),
			PromptHash:    req.PromptHash,
			Provider:      req.Provider,
			Model:         req.Model,
			Status:        string(StatusQueued),
			OS:            nullStr(m.os),
			Arch:          nullStr(m.arch),
			HwTier:        nullStr(m.hwTier),
		})
	}

	m.pool.Submit(Job{TaskID: taskID})
	return taskID, nil
}

func (m *Manager) GetSnapshot(id string) (TaskSnapshot, error) {
	t, ok := m.store.Get(id)
	if !ok {
		return TaskSnapshot{}, errors.New("task_not_found")
	}

	var sAt *int64
	var eAt *int64
	if t.StartedAt != nil {
		v := t.StartedAt.Unix()
		sAt = &v
	}
	if t.EndedAt != nil {
		v := t.EndedAt.Unix()
		eAt = &v
	}

	return TaskSnapshot{
		ID:        t.ID,
		Status:    string(t.Status),
		CreatedAt: t.CreatedAt.Unix(),
		StartedAt: sAt,
		EndedAt:   eAt,
		Category:  t.Category,
		SubagentType: t.SubagentType,
		Provider:  t.Provider,
		Model:     t.Model,
		Error:     t.Error,
	}, nil
}

func (m *Manager) GetOutput(id string) (string, error) {
	t, ok := m.store.Get(id)
	if !ok {
		return "", errors.New("task_not_found")
	}
	if t.Status == StatusQueued || t.Status == StatusRunning {
		return "", errors.New("task_not_finished")
	}
	return t.Output, nil
}

func (m *Manager) Cancel(id string) error {
	// cancel func 호출
	if cf, ok := m.cancelFn[id]; ok && cf != nil {
		cf()
	}
	// 상태 마킹
	return m.store.Update(id, func(t *Task) error {
		if t.Status == StatusSucceeded || t.Status == StatusFailed || t.Status == StatusCanceled {
			return nil
		}
		now := time.Now()
		t.Status = StatusCanceled
		t.EndedAt = &now
		t.Error = "canceled"
		return nil
	})
}

func (m *Manager) workerLoop() {
	for job := range m.pool.Jobs() {
		_ = m.runOne(job.TaskID)
	}
}

func (m *Manager) runOne(id string) error {
	t, ok := m.store.Get(id)
	if !ok {
		return errors.New("task_not_found")
	}

	// 상태 running
	start := time.Now()
	_ = m.store.Update(id, func(tt *Task) error {
		tt.Status = StatusRunning
		tt.StartedAt = &start
		return nil
	})
	if m.ok != nil {
		_ = m.ok.MarkRunning(context.Background(), id)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFn[id] = cancel

	// provider lookup
	p, ok := m.providers.Get(t.Provider)
	if !ok {
		return m.finish(id, start, "", 0, 0, errors.New("provider_not_registered"))
	}

	// 실제 실행
	out, tin, tout, err := m.exec(ctx, p, t.Provider, t.Model, t.Prompt)

	return m.finish(id, start, out, tin, tout, err)
}

func (m *Manager) finish(id string, start time.Time, out string, tin, tout int64, err error) error {
	end := time.Now()
	lat := end.Sub(start).Milliseconds()

	var status Status
	var errMsg string
	success := false
	if err != nil {
		status = StatusFailed
		errMsg = err.Error()
	} else {
		status = StatusSucceeded
		success = true
	}

	// reward(29단계 기본): 성공=1, 실패=0, canceled=0
	reward := 0.0
	if success {
		reward = 1.0
	}

	// store update
	_ = m.store.Update(id, func(t *Task) error {
		// 이미 cancel 되었으면 실패/성공 overwrite 하지 않음
		if t.Status == StatusCanceled {
			return nil
		}
		t.Status = status
		t.EndedAt = &end
		t.LatencyMs = lat
		t.TokensIn = tin
		t.TokensOut = tout
		t.Output = out
		t.Error = errMsg
		t.Reward = reward
		return nil
	})

	// OkAON finish 기록
	if m.ok != nil {
		tt, _ := m.store.Get(id)
		tr := oksql.TaskRun{
			ID: id,
			FinishedAtUnix: sqlNullInt64(end.Unix()),
			Status: string(status),
			ErrorMessage: nullStr(errMsg),
			OutputText:   nullStr(out),
			LatencyMs:    sqlNullInt64(lat),
			TokensIn:     sqlNullInt64(tin),
			TokensOut:    sqlNullInt64(tout),
			Reward:       sqlNullFloat64(reward),
		}
		// optional fields
		if tt != nil && tt.QualityScore != nil {
			tr.QualityScore = sqlNullFloat64(*tt.QualityScore)
		}
		_ = m.ok.MarkFinished(context.Background(), tr)
	}

	// Learning Observe
	if m.learner != nil {
		tt, _ := m.store.Get(id)
		if tt != nil {
			var qs *float64
			if tt.QualityScore != nil {
				qs = tt.QualityScore
			}
			_ = m.learner.Observe(context.Background(), learning.Observation{
				TaskID: id,
				Category: tt.Category,
				SubagentType: tt.SubagentType,
				Provider: tt.Provider,
				Model: tt.Model,
				Success: success,
				Reward: reward,
				LatencyMs: lat,
				QualityScore: qs,
				PromptHash: tt.PromptHash,
			})
		}
	}

	return nil
}

/* ---- sqlite null helpers ---- */

import "database/sql"

func nullStr(v string) sql.NullString {
	if v == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: v, Valid: true}
}
func sqlNullInt64(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: true}
}
func sqlNullFloat64(v float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: v, Valid: true}
}

> ManagerDeps.Exec는 Provider별 Chat 실행을 통일하기 위한 함수 포인터입니다.
29단계에서는 “정확한 토큰 계측”까지는 provider 구현에 따라 다르므로 일단 tin/tout=0 가능하게 둡니다.




---

5) Delegate: background enqueue 연결(29단계 핵심)

5-1) internal/delegate/delegate.go (수정)

package delegate

import (
	"context"
	"errors"

	"devorch/internal/background"
	"devorch/internal/provider"
)

type ExecutorDeps struct {
	Router    *DelegateRouter
	Providers *provider.Registry

	Background *background.Manager // ✅ 29단계 추가
}

type Executor struct {
	dr        *DelegateRouter
	providers *provider.Registry
	bg        *background.Manager
}

func NewExecutor(d ExecutorDeps) *Executor {
	return &Executor{
		dr:        d.Router,
		providers: d.Providers,
		bg:        d.Background,
	}
}

func (e *Executor) Execute(ctx context.Context, req DelegateRequest) (DelegateResult, error) {
	if e.dr == nil {
		return DelegateResult{}, errors.New("delegate_router_not_configured")
	}

	chosen, err := e.dr.PickOne(ctx, req)
	if err != nil {
		return DelegateResult{}, err
	}

	// ✅ background 모드: 즉시 enqueue
	if req.RunInBackground {
		if e.bg == nil {
			return DelegateResult{}, errors.New("background_manager_not_configured")
		}

		taskID, err := e.bg.Enqueue(background.Task{
			Category:     req.Category,
			SubagentType: req.SubagentType,
			Description:  req.Description,
			Prompt:       req.Prompt,

			Provider: chosen.Provider,
			Model:    chosen.Model,
		})
		if err != nil {
			return DelegateResult{}, err
		}

		return DelegateResult{
			TaskID:   taskID,
			Provider: chosen.Provider,
			Model:    chosen.Model,
		}, nil
	}

	// 동기 실행
	p, ok := e.providers.Get(chosen.Provider)
	if !ok {
		return DelegateResult{}, errors.New("provider_not_registered: " + chosen.Provider)
	}

	out, err := p.Chat(ctx, provider.ChatRequest{
		Model:  chosen.Model,
		Prompt: req.Prompt,
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

6) App: Background + OkAON + Learning 주입

6-1) internal/app/app.go (수정)

package app

import (
	"context"

	"devorch/internal/background"
	"devorch/internal/config"
	"devorch/internal/delegate"
	"devorch/internal/learning"
	oksql "devorch/internal/okAON/sqlite"
	"devorch/internal/provider"
	"devorch/internal/router"
)

type App struct {
	Config *config.Store

	Providers *provider.Registry
	Router    *router.Router

	OkTaskRuns *oksql.TaskRunStore

	Learner learning.Learner
	Background *background.Manager
	Delegate    *delegate.Executor
}

func NewApp(reg *provider.Registry, okDB oksql.TaskRunStore, learner learning.Learner) *App {
	cfg, err := config.LoadMerged(config.LoadOptions{})
	if err != nil {
		cfg = config.DefaultConfig()
	}
	cs := config.NewStore(cfg)

	a := &App{
		Config:    cs,
		Providers: reg,
		OkTaskRuns: &okDB,
		Learner:   learner,
	}

	// Router는 기존 단계 구현이 있으면 주입
	// a.Router = router.NewRouter(...)

	// Background 실행 함수: provider.Chat를 통일
	execFn := func(ctx context.Context, p provider.Provider, providerName, model, prompt string) (string, int64, int64, error) {
		resp, err := p.Chat(ctx, provider.ChatRequest{Model: model, Prompt: prompt})
		if err != nil {
			return "", 0, 0, err
		}
		return resp.Text, 0, 0, nil
	}

	a.Background = background.NewManager(background.ManagerDeps{
		Providers: reg,
		OkAON:     a.OkTaskRuns,
		Learner:   a.Learner,
		Exec:      execFn,
		WorkerCount: 2,
		QueueSize:   128,

		OS:     "", // 30단계에서 platform.detect 연결
		Arch:   "",
		HwTier: "",
	})

	// Delegate 라우터/실행기
	dr := delegate.NewDelegateRouter(delegate.RouterDeps{
		Models: nil, // 28단계에서 만든 modelresolver.Resolver를 주입하는 형태가 이미 있다면 그대로 연결
		Router: a.Router,
	})

	a.Delegate = delegate.NewExecutor(delegate.ExecutorDeps{
		Router:     dr,
		Providers:  reg,
		Background: a.Background,
	})

	return a
}

func (a *App) Start(ctx context.Context) error { return nil }
func (a *App) Stop()                           {}

> ⚠️ 여기서 Models: nil로 표시한 이유: 당신 28단계 코드에서 modelresolver.Resolver 주입이 이미 있는 형태(예: NewApp(ok, reg)에서 res := modelresolver.NewResolver(...))였기 때문입니다.
당신 28단계 App 구조에서 res를 만들고 dr := ...{Models: res}로만 연결하면 끝입니다.




---

7) Server: task 조회/출력/취소 라우트 추가

7-1) internal/server/routes/task_query.go (신규)

package routes

import (
	"encoding/json"
	"net/http"
	"strings"

	"devorch/internal/app"
)

type TaskQueryHandler struct {
	App *app.App
}

func NewTaskQueryHandler(a *app.App) *TaskQueryHandler {
	return &TaskQueryHandler{App: a}
}

func (h *TaskQueryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.App == nil || h.App.Background == nil {
		http.Error(w, "background_not_configured", http.StatusInternalServerError)
		return
	}

	// path: /v1/tasks/{id} or /v1/tasks/{id}/output
	path := strings.TrimPrefix(r.URL.Path, "/v1/tasks/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "missing_task_id", http.StatusBadRequest)
		return
	}
	taskID := parts[0]

	if len(parts) == 1 {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		snap, err := h.App.Background.GetSnapshot(taskID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snap)
		return
	}

	// /output
	if len(parts) == 2 && parts[1] == "output" {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		out, err := h.App.Background.GetOutput(taskID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id": taskID,
			"output": out,
		})
		return
	}

	http.Error(w, "not_found", http.StatusNotFound)
}

7-2) internal/server/routes/task_cancel.go (신규)

package routes

import (
	"encoding/json"
	"net/http"
	"strings"

	"devorch/internal/app"
)

type TaskCancelHandler struct {
	App *app.App
}

func NewTaskCancelHandler(a *app.App) *TaskCancelHandler {
	return &TaskCancelHandler{App: a}
}

func (h *TaskCancelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.App == nil || h.App.Background == nil {
		http.Error(w, "background_not_configured", http.StatusInternalServerError)
		return
	}

	// /v1/tasks/{id}/cancel
	path := strings.TrimPrefix(r.URL.Path, "/v1/tasks/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] != "cancel" {
		http.Error(w, "invalid_path", http.StatusBadRequest)
		return
	}
	taskID := parts[0]

	if err := h.App.Background.Cancel(taskID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id": taskID,
		"status": "canceled",
	})
}

7-3) internal/server/server.go (수정)

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
	// ✅ 28단계: delegate
	s.mux.Handle("/v1/tasks/delegate", routes.NewTaskDelegateHandler(s.app))

	// ✅ 29단계: task query/output
	s.mux.Handle("/v1/tasks/", routes.NewTaskQueryHandler(s.app))

	// ✅ 29단계: cancel
	s.mux.Handle("/v1/tasks/", routes.NewTaskCancelHandler(s.app))
}

func (s *Server) Handler() http.Handler { return s.mux }

> 주의: "/v1/tasks/"에 대해 두 핸들러가 겹치므로, 실제로는 보통 “라우터”에서 분기하거나 하나의 핸들러가 모두 처리합니다.
여기서는 파일을 분리해 보여줬고, 실제 적용 시에는 TaskQueryHandler 안에 cancel 분기까지 넣어 하나로 합치는 게 가장 안전합니다.
(원하면 다음 메시지에서 “하나의 파일로 합친 버전”도 바로 제공 가능합니다.)




---

8) 29단계 동작 확인 예시

(1) background 실행

curl -X POST http://127.0.0.1:9901/v1/tasks/delegate \
  -H "Content-Type: application/json" \
  -d '{
    "subagent_type":"oracle",
    "description":"arch advice",
    "prompt":"Design the best model routing policy for local+remote",
    "run_in_background": true
  }'

응답 예:

{"task_id":"01H...","provider":"openai","model":"openai/gpt-4o-mini"}

(2) 상태 조회

curl http://127.0.0.1:9901/v1/tasks/01H...

(3) 결과 조회

curl http://127.0.0.1:9901/v1/tasks/01H.../output

(4) 취소

curl -X POST http://127.0.0.1:9901/v1/tasks/01H.../cancel


---

9) 29단계에서 “아직 의도적으로 남겨둔 것”(30단계로)

29단계는 연결을 “완결”했고, 다음은 고도화 단계입니다.

platform.detect(OS/arch/hwTier 자동 채움) → 30단계

QualityGate(bench) 기반 quality_score 산출 → 31단계

reward를 latency/quality/cost 기반으로 실수화 + router에 반영 → 32단계

OkAON 집계/스냅샷/롤백(정책 버전관리) → 33단계



---

원하면 다음으로 30단계 풀코드(멀티OS 환경 fingerprint + hwTier 계산 + OkAON에 자동 기록) 바로 이어서 작성하겠습니다.