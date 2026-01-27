아래는 25단계(“실제 요청 처리 흐름에 꽂는 단계”) 풀코드입니다.
24단계까지 “객체 연결”을 끝냈다면, 25단계는 진짜로 다음을 파이프라인 호출 지점에 연결합니다.

✅ 25단계에서 연결되는 것들

Provider 호출 성공/실패/지연시간 → Pool.Observe() 기록

모델/프로바이더 fallback(스위치) 발생 → HookChain on_model_switch 이벤트 발행(+Reward 저장)

재시도 소진 → HookChain on_retry_exhausted 이벤트 발행(+Reward 저장)

Bench 결과(quality/latency/cost) → okAON 저장 + arm_stats 갱신 → Router가 다음 선택에 반영


> 이번 단계도 추가/교체되는 파일만 “파일별 전체 코드”로 제공합니다.




---

A) 파이프라인 런(Execution) 모듈 추가

A-1) internal/exec/runner.go (신규)

package exec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"devorch/internal/hook"
	"devorch/internal/id"
	"devorch/internal/provider"
	"devorch/internal/provider/pool"
	"devorch/internal/router"
)

type BenchRecorder interface {
	// 25단계에서는 “실측 벤치 결과 저장 + arm_stats 갱신”까지 수행한다고 가정
	Record(ctx context.Context, in BenchInput) error
}

type BenchInput struct {
	RunID       string
	Fingerprint string
	Category    string

	Provider string
	Model    string

	Success     bool
	LatencyMs   int64
	CostMicroUSD int64

	Quality01 float64 // 0..1
	ErrorMsg  string
}

type RewardSink interface {
	Dispatch(ctx context.Context, in hook.DispatchInput) error
}

type Runner struct {
	Router *router.Router
	Pool   *pool.Pool

	Providers *provider.Registry

	Hooks RewardSink
	Bench BenchRecorder

	// retry 정책
	MaxAttempts int
	BaseBackoff time.Duration
}

func NewRunner(rt *router.Router, pl *pool.Pool, reg *provider.Registry, hooks RewardSink, bench BenchRecorder) *Runner {
	return &Runner{
		Router:      rt,
		Pool:        pl,
		Providers:   reg,
		Hooks:       hooks,
		Bench:       bench,
		MaxAttempts: 3,
		BaseBackoff: 250 * time.Millisecond,
	}
}

type RunInput struct {
	Fingerprint string
	Category    string

	// Router 후보 생성용(모델리졸버 결과)
	Candidates []router.Candidate

	// 실제 요청(표준화된 provider request)
	Request provider.ChatRequest

	// Quality 측정이 가능하면(테스트/정답셋/정적 eval) 넣고, 없으면 0.5 기본
	QualityHint01 float64
}

type RunOutput struct {
	RunID    string
	Provider string
	Model    string

	Response provider.ChatResponse
}

func (r *Runner) RunChat(ctx context.Context, in RunInput) (RunOutput, error) {
	runID := id.NewULID()

	quality := in.QualityHint01
	if quality <= 0 {
		quality = 0.5
	}

	// 1) Router 선택
	sel, err := r.Router.Select(ctx, router.SelectInput{
		Fingerprint: in.Fingerprint,
		Category:    in.Category,
		Candidates:  in.Candidates,
	})
	if err != nil {
		return RunOutput{}, err
	}

	cur := sel.Chosen
	curCtxHash := hashContext(in.Request)

	// 2) 실제 실행(재시도 + fallback)
	var lastErr error
	attempts := clampInt(r.MaxAttempts, 1, 10)

	for attempt := 1; attempt <= attempts; attempt++ {
		start := time.Now()
		resp, costMicro, err := r.callOnce(ctx, cur.Provider, cur.Model, in.Request)
		latMs := time.Since(start).Milliseconds()

		success := (err == nil)

		// 2-a) Pool 관측 기록
		if r.Pool != nil {
			r.Pool.Observe(cur.Provider, cur.Model, success, latMs)
		}

		// 2-b) Bench 기록(실측)
		if r.Bench != nil {
			_ = r.Bench.Record(ctx, BenchInput{
				RunID:        runID,
				Fingerprint:  in.Fingerprint,
				Category:     in.Category,
				Provider:     cur.Provider,
				Model:        cur.Model,
				Success:      success,
				LatencyMs:    latMs,
				CostMicroUSD: costMicro,
				Quality01:    quality,
				ErrorMsg:     errString(err),
			})
		}

		if err == nil {
			return RunOutput{
				RunID:    runID,
				Provider: cur.Provider,
				Model:    cur.Model,
				Response: resp,
			}, nil
		}

		lastErr = err

		// 2-c) fallback: 다음 후보로 스위치 시도
		next, ok := pickNextCandidate(in.Candidates, cur)
		if ok {
			// Hook: model switch 이벤트 발행(=Reward 저장 트리거)
			if r.Hooks != nil {
				_ = r.Hooks.Dispatch(ctx, hook.DispatchInput{
					Type:        hook.EventOnModelSwitch,
					RunID:       runID,
					FromModel:   cur.Provider + "/" + cur.Model,
					ToModel:     next.Provider + "/" + next.Model,
					Reason:      "provider_error_or_retry",
					ContextHash: curCtxHash,
				})
			}
			cur = next
			// backoff는 스위치 후에도 적용
			sleepBackoff(ctx, r.BaseBackoff, attempt)
			continue
		}

		// 후보가 더 없으면 retry만
		sleepBackoff(ctx, r.BaseBackoff, attempt)
	}

	// 3) retry exhausted 이벤트
	if r.Hooks != nil {
		_ = r.Hooks.Dispatch(ctx, hook.DispatchInput{
			Type:    hook.EventOnRetryExhausted,
			RunID:   runID,
			Message: "retry_exhausted: " + errString(lastErr),
		})
	}
	return RunOutput{}, lastErr
}

func (r *Runner) callOnce(ctx context.Context, providerName, model string, req provider.ChatRequest) (provider.ChatResponse, int64, error) {
	if r.Providers == nil {
		return provider.ChatResponse{}, 0, errors.New("provider registry nil")
	}
	p, ok := r.Providers.Get(providerName)
	if !ok {
		return provider.ChatResponse{}, 0, errors.New("unknown provider: "+providerName)
	}
	return p.Chat(ctx, model, req)
}

func pickNextCandidate(list []router.Candidate, cur router.Candidate) (router.Candidate, bool) {
	// 단순: list 순서 기준으로 현재 다음
	found := false
	for _, c := range list {
		if !found {
			if c.Provider == cur.Provider && c.Model == cur.Model {
				found = true
			}
			continue
		}
		return c, true
	}
	return router.Candidate{}, false
}

func sleepBackoff(ctx context.Context, base time.Duration, attempt int) {
	if base <= 0 {
		base = 200 * time.Millisecond
	}
	d := time.Duration(attempt) * base
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return
	case <-t.C:
		return
	}
}

func hashContext(req provider.ChatRequest) string {
	// 최소: messages role/content만 해시
	h := sha256.New()
	for _, m := range req.Messages {
		h.Write([]byte(m.Role))
		h.Write([]byte{0})
		h.Write([]byte(m.Content))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}


---

B) Provider 표준 인터페이스에 “비용 반환”까지 포함(실측 기록용)

B-1) internal/provider/provider.go (교체)

package provider

import "context"

type ChatMessage struct {
	Role    string
	Content string
}

type ChatRequest struct {
	Messages     []ChatMessage
	Temperature  float64
	MaxTokens    int
	TopP         float64
	Stop         []string
	Stream       bool
	RequestID    string
}

type ChatResponse struct {
	Text string

	// 옵션: provider가 주면 사용
	UsageTokensIn  int64
	UsageTokensOut int64
}

type Provider interface {
	// costMicroUSD: 비용을 마이크로달러 단위로 계산/추정해 반환(없으면 0)
	Chat(ctx context.Context, model string, req ChatRequest) (resp ChatResponse, costMicroUSD int64, err error)
	Name() string
}

B-2) internal/provider/registry.go (교체)

package provider

import "sync"

type Registry struct {
	mu sync.RWMutex
	m  map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{m: map[string]Provider{}}
}

func (r *Registry) Register(p Provider) {
	if p == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[p.Name()] = p
}

func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.m[name]
	return p, ok
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.m))
	for k := range r.m {
		out = append(out, k)
	}
	return out
}


---

C) Bench Recorder: OkAON 저장 + arm_stats 갱신(핵심)

> 25단계에서 Bench가 단순 기록이 아니라
okaon_runs + okaon_arm_stats를 함께 갱신해야 Router 학습이 실제로 먹습니다.



C-1) internal/bench/recorder.go (신규/교체)

package bench

import (
	"context"
	"time"

	"devorch/internal/okAON"
	"devorch/internal/okAON/sqlite"
)

type Recorder struct {
	Store sqlite.Store
}

func NewRecorder(store sqlite.Store) *Recorder {
	return &Recorder{Store: store}
}

type RecordInput struct {
	RunID       string
	Fingerprint string
	Category    string
	Provider    string
	Model       string

	Success       bool
	LatencyMs     int64
	CostMicroUSD  int64
	Quality01     float64
	ErrorMsg      string

	StartedAt time.Time
	EndedAt   time.Time
}

func (r *Recorder) Record(ctx context.Context, in RecordInput) error {
	if in.StartedAt.IsZero() {
		in.StartedAt = time.Now().UTC()
	}
	if in.EndedAt.IsZero() {
		in.EndedAt = time.Now().UTC()
	}

	run := okAON.Run{
		RunID:        in.RunID,
		Fingerprint:  in.Fingerprint,
		Category:     in.Category,
		Provider:     in.Provider,
		Model:        in.Model,
		Success:      in.Success,
		LatencyMs:    in.LatencyMs,
		CostMicroUSD: in.CostMicroUSD,
		Quality01:    clamp01(in.Quality01),
		ErrorMsg:     in.ErrorMsg,
		StartedTsUTC: in.StartedAt.Unix(),
		EndedTsUTC:   in.EndedAt.Unix(),
	}

	// 1) runs insert
	if err := r.Store.InsertRun(ctx, run); err != nil {
		return err
	}

	// 2) arm_stats upsert (평균/카운트 갱신)
	return r.Store.UpsertArmStats(ctx, okAON.ArmStatUpdate{
		Fingerprint:  in.Fingerprint,
		Category:     in.Category,
		Provider:     in.Provider,
		Model:        in.Model,
		Reward01:     clamp01(in.Quality01),
		OccurredTsUTC: in.EndedAt.Unix(),
	})
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}


---

D) OkAON: Run insert + ArmStats Upsert 구현(+SQL)

D-1) internal/okAON/models_run.go (신규)

package okAON

type Run struct {
	RunID       string
	Fingerprint string
	Category    string
	Provider    string
	Model       string

	Success      bool
	LatencyMs    int64
	CostMicroUSD int64
	Quality01    float64
	ErrorMsg     string

	StartedTsUTC int64
	EndedTsUTC   int64
}

type ArmStatUpdate struct {
	Fingerprint   string
	Category      string
	Provider      string
	Model         string
	Reward01      float64
	OccurredTsUTC int64
}

D-2) internal/okAON/sqlite/run_insert.go (신규)

package sqlite

import (
	"context"

	"devorch/internal/okAON"
)

func (s Store) InsertRun(ctx context.Context, r okAON.Run) error {
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO okaon_runs(
  run_id, fingerprint, category, provider, model,
  success, latency_ms, cost_micro_usd, quality01, error_msg,
  started_ts_utc, ended_ts_utc
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)
`, r.RunID, r.Fingerprint, r.Category, r.Provider, r.Model,
		boolToInt(r.Success), r.LatencyMs, r.CostMicroUSD, r.Quality01, r.ErrorMsg,
		r.StartedTsUTC, r.EndedTsUTC)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

D-3) internal/okAON/sqlite/arm_stats_upsert.go (신규)

package sqlite

import (
	"context"
	"database/sql"
	"math"

	"devorch/internal/okAON"
)

// Welford 없이 “증분 평균”으로 처리(간단/안정)
// mean_new = mean_old + (x - mean_old) / (n+1)
func (s Store) UpsertArmStats(ctx context.Context, u okAON.ArmStatUpdate) error {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
SELECT count_runs, mean_reward01
FROM okaon_arm_stats
WHERE fingerprint=? AND category=? AND provider=? AND model=?
`, u.Fingerprint, u.Category, u.Provider, u.Model)

	var count sql.NullInt64
	var mean sql.NullFloat64
	scanErr := row.Scan(&count, &mean)

	oldCount := int64(0)
	oldMean := 0.0
	if scanErr == nil {
		oldCount = nzI64(count)
		oldMean = nzF64(mean)
	} else if scanErr != sql.ErrNoRows {
		return scanErr
	}

	x := clamp01(u.Reward01)
	newCount := oldCount + 1
	newMean := x
	if newCount > 1 {
		newMean = oldMean + (x-oldMean)/float64(newCount)
	}

	// sqlite upsert
	_, err = tx.ExecContext(ctx, `
INSERT INTO okaon_arm_stats(
  fingerprint, category, provider, model,
  count_runs, mean_reward01, updated_ts_utc
) VALUES(?,?,?,?,?,?,?)
ON CONFLICT(fingerprint, category, provider, model)
DO UPDATE SET
  count_runs=excluded.count_runs,
  mean_reward01=excluded.mean_reward01,
  updated_ts_utc=excluded.updated_ts_utc
`, u.Fingerprint, u.Category, u.Provider, u.Model,
		newCount, newMean, u.OccurredTsUTC)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0.5
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}


---

E) SQL: 0003_okAON_learning.sql 보강(arm_stats + runs)

> 22단계에 이미 0003이 있었다면, 아래 내용으로 교체하면 됩니다.
(이번 단계가 “진짜 저장/학습”이라 이 스키마가 기준입니다.)



E-1) internal/storage/sqlite/migrations/0003_okAON_learning.sql (교체)

-- OkAON: 실측 러닝/벤치 데이터

CREATE TABLE IF NOT EXISTS okaon_runs (
  run_id            TEXT PRIMARY KEY,
  fingerprint       TEXT NOT NULL,
  category          TEXT NOT NULL,
  provider          TEXT NOT NULL,
  model             TEXT NOT NULL,

  success           INTEGER NOT NULL,
  latency_ms        INTEGER NOT NULL,
  cost_micro_usd    INTEGER NOT NULL,
  quality01         REAL NOT NULL,
  error_msg         TEXT NOT NULL,

  started_ts_utc    INTEGER NOT NULL,
  ended_ts_utc      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_runs_fp_cat_time
ON okaon_runs(fingerprint, category, ended_ts_utc);

CREATE INDEX IF NOT EXISTS idx_okaon_runs_provider_model_time
ON okaon_runs(provider, model, ended_ts_utc);

-- arm stats: fingerprint+category 기준에서 provider/model arm 성능 통계
CREATE TABLE IF NOT EXISTS okaon_arm_stats (
  fingerprint       TEXT NOT NULL,
  category          TEXT NOT NULL,
  provider          TEXT NOT NULL,
  model             TEXT NOT NULL,

  count_runs        INTEGER NOT NULL,
  mean_reward01     REAL NOT NULL,
  updated_ts_utc    INTEGER NOT NULL,

  PRIMARY KEY (fingerprint, category, provider, model)
);

CREATE INDEX IF NOT EXISTS idx_okaon_arm_stats_fp_cat
ON okaon_arm_stats(fingerprint, category);

-- (선택) reward 이벤트(모델스위치/리트라이소진 등) 저장용
CREATE TABLE IF NOT EXISTS okaon_rewards (
  id               TEXT PRIMARY KEY,
  run_id           TEXT NOT NULL,
  kind             TEXT NOT NULL,   -- model_switch | retry_exhausted | ...
  payload_json     TEXT NOT NULL,
  created_ts_utc   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_rewards_run
ON okaon_rewards(run_id, created_ts_utc);


---

F) Hook Reward 저장(모델스위치/리트라이소진 이벤트 → okaon_rewards에 저장)

> 24단계에서 HookChain이 ok.InsertReward() 호출하게 되어 있으니,
sqlite.Store가 InsertReward를 실제로 구현해줘야 합니다.



F-1) internal/okAON/sqlite/reward_insert.go (신규)

package sqlite

import (
	"context"
	"encoding/json"
	"time"

	"devorch/internal/id"
)

type RewardEvent struct {
	RunID string `json:"run_id"`
	Kind  string `json:"kind"`
	Data  any    `json:"data"`
}

func (s Store) InsertReward(ctx context.Context, kind string, runID string, data any) error {
	payload, _ := json.Marshal(RewardEvent{
		RunID: runID,
		Kind:  kind,
		Data:  data,
	})

	_, err := s.DB.ExecContext(ctx, `
INSERT INTO okaon_rewards(id, run_id, kind, payload_json, created_ts_utc)
VALUES(?,?,?,?,?)
`, id.NewULID(), runID, kind, string(payload), time.Now().UTC().Unix())
	return err
}


---

G) Hook builtins: Store 인터페이스를 sqlite.Store에 맞춰 “kind+data”로 호출

G-1) internal/hook/builtins/on_model_switch.go (교체)

package builtins

import "context"

type SwitchStore interface {
	InsertReward(ctx context.Context, kind string, runID string, data any) error
}

type ModelSwitchInput struct {
	RunID       string
	FromModel   string
	ToModel     string
	Reason      string
	ContextHash string
}

func OnModelSwitch(ctx context.Context, store SwitchStore, in ModelSwitchInput) error {
	if store == nil {
		return nil
	}
	return store.InsertReward(ctx, "model_switch", in.RunID, in)
}

G-2) internal/hook/builtins/on_retry_exhausted.go (교체)

package builtins

import "context"

type RetryStore interface {
	InsertReward(ctx context.Context, kind string, runID string, data any) error
}

type RetryExhaustedInput struct {
	RunID   string
	Message string
}

func OnRetryExhausted(ctx context.Context, store RetryStore, in RetryExhaustedInput) error {
	if store == nil {
		return nil
	}
	return store.InsertReward(ctx, "retry_exhausted", in.RunID, in)
}


---

H) App wiring: Runner + BenchRecorder를 실제로 생성해서 쓰도록 연결

H-1) internal/app/app.go (교체 – Runner 추가)

package app

import (
	"context"
	"time"

	"devorch/internal/background"
	"devorch/internal/bench"
	"devorch/internal/exec"
	"devorch/internal/hook"
	"devorch/internal/okAON/sqlite"
	"devorch/internal/provider"
	"devorch/internal/provider/pool"
	"devorch/internal/router"
)

type App struct {
	Ok sqlite.Store

	Pool   *pool.Pool
	BgFB   *background.Feedback
	BgMgr  *background.Manager

	Providers *provider.Registry
	Router    *router.Router
	Hooks     *hook.Chain

	Bench  *bench.Recorder
	Runner *exec.Runner
}

func NewApp(ok sqlite.Store, provReg *provider.Registry) *App {
	p := pool.NewPool(24 * time.Hour)

	fb := &background.Feedback{}
	bg := background.NewManager(fb, 4)

	la := router.DefaultLearningAdapter(ok)
	rt := router.NewRouter(p, la, fb)

	hc := &hook.Chain{
		Store: ok, // sqlite.Store가 InsertReward 구현
	}

	br := bench.NewRecorder(ok)

	rn := exec.NewRunner(rt, p, provReg, hc, benchAdapter{br})

	return &App{
		Ok:        ok,
		Pool:      p,
		BgFB:      fb,
		BgMgr:     bg,
		Providers: provReg,
		Router:    rt,
		Hooks:     hc,
		Bench:     br,
		Runner:    rn,
	}
}

func (a *App) Start(ctx context.Context) error {
	return a.BgMgr.Start(ctx)
}

func (a *App) Stop() {
	a.BgMgr.Stop()
}

type benchAdapter struct{ r *bench.Recorder }

func (b benchAdapter) Record(ctx context.Context, in exec.BenchInput) error {
	if b.r == nil {
		return nil
	}
	return b.r.Record(ctx, bench.RecordInput{
		RunID:         in.RunID,
		Fingerprint:   in.Fingerprint,
		Category:      in.Category,
		Provider:      in.Provider,
		Model:         in.Model,
		Success:       in.Success,
		LatencyMs:     in.LatencyMs,
		CostMicroUSD:  in.CostMicroUSD,
		Quality01:     in.Quality01,
		ErrorMsg:      in.ErrorMsg,
	})
}


---

I) (필수) sqlite Store 타입 정의가 없으면 보강

I-1) internal/okAON/sqlite/sqlite.go (필요 시 교체/보강)

package sqlite

import "database/sql"

type Store struct {
	DB *sql.DB
}

I-2) internal/okAON/sqlite/nz.go (없으면 추가)

package sqlite

import "database/sql"

func nzI64(v sql.NullInt64) int64 {
	if v.Valid {
		return v.Int64
	}
	return 0
}

func nzF64(v sql.NullFloat64) float64 {
	if v.Valid {
		return v.Float64
	}
	return 0
}


---

25단계 완료 체크리스트(동작 기준)

1. Runner.RunChat() 실행하면



Router가 모델 선택

Provider Chat 호출

성공/실패에 따라 Pool.Observe() 기록

bench가 okaon_runs insert + okaon_arm_stats upsert 수행

실패 시 후보가 있으면 Hook model_switch 이벤트 저장

끝까지 실패하면 Hook retry_exhausted 이벤트 저장


2. 다음 호출부터 Router의 LearningAdapter가



GetArmStats()를 읽어서 score 보정
→ “쓸수록 내 PC/내 프로젝트에 최적화”가 실제로 동작합니다.



---

다음 26단계(추천): “서버 API 라우트에 Runner 연결”

25단계는 코어 파이프라인이 완성됐고, 26단계는

/v1/chat 같은 HTTP route에서 Runner.RunChat() 호출

SSE 스트리밍(선택)과 cost/usage 응답 정리

VSCode/WebUI에서 learning/perf/history 화면에 붙이기
를 진행하면 됩니다.


원하면 26단계 풀코드 바로 이어서 작성하겠습니다.