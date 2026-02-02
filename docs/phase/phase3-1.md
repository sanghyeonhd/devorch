좋습니다. 3단계(“자동 후보군 생성 + OkAON 기반 자동선택(밴딧) + 작업완료 후 실측 벤치/품질점수 반영 + 로컬모델 자동준비(profile)”) 전체 풀코드를 파일별로 제공합니다.
아래는 Step2 코드 기준으로 “추가/변경되는 전체 파일” 입니다.
0) 개요(3단계에서 추가되는 핵심)
Auto Candidates: workspace/카테고리/태스크에 맞춰 로컬(Ollama) + 원격(OpenAI/OpenRouter) 후보를 한 번에 생성
Auto Select: router.Select()가 OkAON(과거 성과) 기반으로 후보를 선택 (epsilon-greedy 유지 + UCB1 옵션 추가)
Bench After Task: “작업 완료 기준(테스트/빌드/린트)”을 실행해 품질점수(0~1) 를 산출 → OkAON에 기록
Local Model Ensure: HW/프로파일(low/mid/high)에 따라 Ollama 모델 자동 pull (오프라인이면 skip)
1) DB Migration 추가
internal/storage/sqlite/migrations/0005_quality_bench.sql (추가)
코드 복사
Sql
CREATE TABLE IF NOT EXISTS quality_benchmarks (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,

  workspace_id TEXT,
  user_id TEXT,
  project_id TEXT,

  category TEXT,
  task_type TEXT,

  suite TEXT NOT NULL,          -- e.g. "go_test", "lint", "build"
  command TEXT NOT NULL,
  timeout_sec INTEGER NOT NULL DEFAULT 0,

  exit_code INTEGER NOT NULL DEFAULT 0,
  duration_ms INTEGER NOT NULL DEFAULT 0,

  score REAL NOT NULL DEFAULT 0.0,   -- 0~1
  stdout TEXT,
  stderr TEXT
);

CREATE INDEX IF NOT EXISTS idx_quality_benchmarks_time
ON quality_benchmarks(created_at);
2) OkAON Store 확장 (벤치마크 저장)
internal/okaon/store.go (변경)
코드 복사
Go
package okaon

import (
	"context"
	"database/sql"
	"time"

	"github.com/oklog/ulid/v2"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) InsertRun(ctx context.Context, r Run) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO okaon_runs(
  run_id, created_at, policy_key,
  workspace_id, user_id, project_id,
  agent_type, category, task_type,
  provider, model, endpoint,
  latency_ms, ok, quality,
  prompt_tokens, completion_tokens, total_tokens, cost_micro_usd,
  error_code, error_message
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.RunID, r.CreatedAt, r.PolicyKey,
		r.WorkspaceID, r.UserID, r.ProjectID,
		r.AgentType, r.Category, r.TaskType,
		r.Provider, r.Model, r.Endpoint,
		r.LatencyMS, r.OK, r.Quality,
		r.PromptTokens, r.CompletionTokens, r.TotalTokens, r.CostMicroUSD,
		r.ErrorCode, r.ErrorMessage,
	)
	return err
}

type BenchRecord struct {
	ID        string
	CreatedAt string

	WorkspaceID string
	UserID      string
	ProjectID   string

	Category string
	TaskType string

	Suite      string
	Command    string
	TimeoutSec int

	ExitCode   int
	DurationMS int

	Score  float64
	Stdout string
	Stderr string
}

func (s *Store) InsertBench(ctx context.Context, b BenchRecord) error {
	if b.ID == "" {
		b.ID = ulid.Make().String()
	}
	if b.CreatedAt == "" {
		b.CreatedAt = time.Now().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO quality_benchmarks(
  id, created_at,
  workspace_id, user_id, project_id,
  category, task_type,
  suite, command, timeout_sec,
  exit_code, duration_ms,
  score, stdout, stderr
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		b.ID, b.CreatedAt,
		b.WorkspaceID, b.UserID, b.ProjectID,
		b.Category, b.TaskType,
		b.Suite, b.Command, b.TimeoutSec,
		b.ExitCode, b.DurationMS,
		b.Score, b.Stdout, b.Stderr,
	)
	return err
}
3) Router: UCB1 전략 추가(옵션) + 설정
internal/router/strategy.go (추가)
코드 복사
Go
package router

import (
	"context"
	"math"
)

type Strategy interface {
	Pick(ctx context.Context, wl Workload, scored []ScoredCandidate) (ScoredCandidate, error)
}

// EpsilonGreedy는 Step1/2에서 사용하던 전략 (유지)
type EpsilonGreedy struct {
	Epsilon float64
}

func NewEpsilonGreedy() *EpsilonGreedy { return &EpsilonGreedy{Epsilon: 0.10} }

// UCB1: 평균 보상 + 탐험 보정 (OkAON 통계 기반)
type UCB1 struct{}

func NewUCB1() *UCB1 { return &UCB1{} }

func (s *UCB1) Pick(ctx context.Context, wl Workload, scored []ScoredCandidate) (ScoredCandidate, error) {
	if len(scored) == 0 {
		return ScoredCandidate{}, Err("no_candidates", "no candidates")
	}

	// total pulls: sum of counts
	total := 0.0
	for i := range scored {
		total += float64(scored[i].Stats.Count)
	}
	if total < 1 {
		// cold start: pick highest base score
		best := scored[0]
		for i := 1; i < len(scored); i++ {
			if scored[i].Score > best.Score {
				best = scored[i]
			}
		}
		return best, nil
	}

	best := scored[0]
	bestU := ucb(total, scored[0])
	for i := 1; i < len(scored); i++ {
		u := ucb(total, scored[i])
		if u > bestU {
			bestU = u
			best = scored[i]
		}
	}
	return best, nil
}

func ucb(total float64, c ScoredCandidate) float64 {
	n := float64(c.Stats.Count)
	if n < 1 {
		// force exploration of never-tried arms
		return c.Score + 10.0
	}
	avg := c.Stats.AvgReward
	bonus := math.Sqrt(2.0 * math.Log(total) / n)
	return avg + bonus
}
internal/router/router.go (변경: 전략 인터페이스 적용 + 통계 포함)
코드 복사
Go
package router

import (
	"context"
	"time"

	"devorch/internal/log"
)

type Router struct {
	weights  Weights
	strategy Strategy
	enricher Enricher
}

func New(weights Weights, strategy Strategy, enricher Enricher) *Router {
	return &Router{weights: weights, strategy: strategy, enricher: enricher}
}

type Decision struct {
	Provider  string
	Model     string
	Endpoint  string
	PolicyKey string
}

func (r *Router) Select(ctx context.Context, wl Workload, cands []Candidate) (Decision, error) {
	if len(cands) == 0 {
		return Decision{}, Err("no_candidates", "no candidates provided")
	}

	stats, _ := r.enricher.GetStats(ctx, BuildPolicyKey(wl), cands)

	scored := make([]ScoredCandidate, 0, len(cands))
	for i, c := range cands {
		st := stats[i]
		base := ScoreCandidate(r.weights, c, st)
		scored = append(scored, ScoredCandidate{
			Candidate: c,
			Score:     base,
			Stats:     st,
		})
	}

	// Step3: Strategy(egreedy or ucb1)
	picked, err := r.strategy.Pick(ctx, wl, scored)
	if err != nil {
		return Decision{}, err
	}

	log.Infof("router.select provider=%s model=%s base=%.4f count=%d avg=%.4f",
		picked.Provider, picked.Model, picked.Score, picked.Stats.Count, picked.Stats.AvgReward,
	)
	return Decision{
		Provider:  picked.Provider,
		Model:     picked.Model,
		Endpoint:  picked.Endpoint,
		PolicyKey: BuildPolicyKey(wl),
	}, nil
}

type ScoredCandidate struct {
	Candidate
	Score float64
	Stats CandidateStats
}

type Enricher interface {
	GetStats(ctx context.Context, policyKey string, cands []Candidate) ([]CandidateStats, error)
}

type CandidateStats struct {
	Count     int
	AvgReward float64
	P95LatMS  int
	FailRate  float64
	// Step3: allow strategy to see raw stats quickly
	LastSeenAt time.Time
}

func Err(code, msg string) error { return &RouterError{Code: code, Msg: msg} }

type RouterError struct {
	Code string
	Msg  string
}

func (e *RouterError) Error() string { return e.Code + ": " + e.Msg }
4) OkAON Enricher: 통계 필드 확장
internal/router/enricher_okaon.go (변경)
코드 복사
Go
package router

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/okaon"
)

type OkAONEnricher struct {
	Store *okaon.Store
}

func (e *OkAONEnricher) GetStats(ctx context.Context, policyKey string, cands []Candidate) ([]CandidateStats, error) {
	out := make([]CandidateStats, len(cands))
	if e == nil || e.Store == nil {
		return out, nil
	}

	for i, c := range cands {
		st, err := e.queryOne(ctx, policyKey, c.Provider, c.Model)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		out[i] = st
	}
	return out, nil
}

func (e *OkAONEnricher) queryOne(ctx context.Context, policyKey, provider, model string) (CandidateStats, error) {
	var cnt int
	var avg sql.NullFloat64
	var p95 sql.NullInt64
	var fail sql.NullFloat64
	var last sql.NullString

	err := e.Store.DB().QueryRowContext(ctx, `
SELECT
  COUNT(*) as cnt,
  AVG(reward) as avg_reward,
  (SELECT latency_ms FROM okaon_rewards
     WHERE policy_key=? AND provider=? AND model=?
     ORDER BY latency_ms
     LIMIT 1 OFFSET (SELECT CAST(0.95*COUNT(*) AS INT) FROM okaon_rewards WHERE policy_key=? AND provider=? AND model=?)
  ) AS p95_latency_ms,
  AVG(CASE WHEN ok=0 THEN 1.0 ELSE 0.0 END) as fail_rate,
  MAX(created_at) as last_seen
FROM okaon_rewards
WHERE policy_key=? AND provider=? AND model=?`,
		policyKey, provider, model,
		policyKey, provider, model,
		policyKey, provider, model,
	).Scan(&cnt, &avg, &p95, &fail, &last)
	if err != nil {
		return CandidateStats{}, err
	}

	var t time.Time
	if last.Valid {
		tt, _ := time.Parse(time.RFC3339Nano, last.String)
		t = tt
	}

	st := CandidateStats{Count: cnt, LastSeenAt: t}
	if avg.Valid {
		st.AvgReward = avg.Float64
	}
	if p95.Valid {
		st.P95LatMS = int(p95.Int64)
	}
	if fail.Valid {
		st.FailRate = fail.Float64
	}
	return st, nil
}

// DB accessor for this query helper
func (s *okaon.Store) DB() *sql.DB { return s.RawDB() }
⚠️ 위 코드에서 okaon.Store에 RawDB()가 없으므로 아래를 추가합니다.
internal/okaon/store_db.go (추가)
코드 복사
Go
package okaon

import "database/sql"

func (s *Store) RawDB() *sql.DB { return s.db }
5) 후보군 자동 생성(Auto Candidates)
internal/autocand/generate.go (추가)
코드 복사
Go
package autocand

import (
	"context"
	"strings"

	"devorch/internal/config"
	"devorch/internal/provider"
	"devorch/internal/router"
)

type Generator struct {
	Cfg config.Config
	Reg *provider.Registry
}

func New(cfg config.Config, reg *provider.Registry) *Generator {
	return &Generator{Cfg: cfg, Reg: reg}
}

// Step3: workload + config 기반 후보를 “일괄 생성”
func (g *Generator) Generate(ctx context.Context, wl router.Workload) ([]router.Candidate, error) {
	out := make([]router.Candidate, 0, 16)

	// 1) 로컬(ollama): category/task에 맞는 모델 리스트 사용
	if p, ok := g.Reg.Get("ollama"); ok {
		// 빠른/코딩/일반에 따라 모델 선호
		for _, m := range defaultOllamaModels(wl) {
			out = append(out, router.Candidate{
				Provider:  "ollama",
				Model:     m,
				Endpoint:  p.Endpoint(),
				BaseCost:  0,
				BaseLatMS: 900,
				BaseQual:  0.55,
			})
		}
	}

	// 2) openai/openrouter: “원격 모델 후보” (API로 모델 list를 가져오는 대신, 설정/기본값 기반)
	if p, ok := g.Reg.Get("openai"); ok {
		for _, m := range defaultOpenAIModels(wl) {
			out = append(out, router.Candidate{
				Provider:  "openai",
				Model:     m,
				Endpoint:  p.Endpoint(),
				BaseCost:  30,
				BaseLatMS: 700,
				BaseQual:  0.70,
			})
		}
	}
	if p, ok := g.Reg.Get("openrouter"); ok {
		for _, m := range defaultOpenRouterModels(wl) {
			out = append(out, router.Candidate{
				Provider:  "openrouter",
				Model:     m,
				Endpoint:  p.Endpoint(),
				BaseCost:  20,
				BaseLatMS: 900,
				BaseQual:  0.65,
			})
		}
	}

	// 3) 중복 제거 (provider+model)
	uniq := make(map[string]router.Candidate, len(out))
	for _, c := range out {
		k := c.Provider + "::" + c.Model
		if _, exists := uniq[k]; !exists {
			uniq[k] = c
		}
	}
	out2 := make([]router.Candidate, 0, len(uniq))
	for _, v := range uniq {
		out2 = append(out2, v)
	}
	return out2, nil
}

func defaultOllamaModels(wl router.Workload) []string {
	// step3: 최소 룰
	switch strings.ToLower(wl.Category) {
	case "ultrabrain":
		return []string{"qwen2.5:14b", "llama3.1:8b"}
	case "quick":
		return []string{"llama3.1:8b"}
	default:
		return []string{"llama3.1:8b"}
	}
}

func defaultOpenAIModels(wl router.Workload) []string {
	switch strings.ToLower(wl.Category) {
	case "ultrabrain":
		return []string{"gpt-5.2-codex", "gpt-5.2"}
	case "quick":
		return []string{"gpt-5-nano", "gpt-5.2"}
	default:
		return []string{"gpt-5.2"}
	}
}

func defaultOpenRouterModels(wl router.Workload) []string {
	// 예시: openrouter에서는 "anthropic/..." 같은 ID도 가능
	switch strings.ToLower(wl.Category) {
	case "quick":
		return []string{"anthropic/claude-haiku-4.5"}
	default:
		return []string{"anthropic/claude-sonnet-4.5"}
	}
}
6) 로컬 모델 자동 준비(Ensure) – 프로파일 기반 ollama pull
internal/localensure/ensure.go (추가)
코드 복사
Go
package localensure

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"devorch/internal/config"
	"devorch/internal/provider"
)

type profile struct {
	Profile string `json:"profile"`
	Targets []struct {
		Runtime string   `json:"runtime"`
		Models  []string `json:"models"`
	} `json:"targets"`
}

func EnsureLocalModels(ctx context.Context, cfg config.Config, reg *provider.Registry, profileName string) error {
	if cfg.Local.OfflineMode {
		return nil // offline: skip download
	}
	if profileName == "" {
		profileName = "mid"
	}

	p, ok := reg.Get("ollama")
	if !ok {
		return nil // no local runtime
	}

	type puller interface{ Pull(context.Context, string) error }
	pl, ok := p.(puller)
	if !ok {
		return nil
	}

	pp := filepath.Join(cfg.Local.ModelsDir, "profiles", profileName+".json")
	b, err := os.ReadFile(pp)
	if err != nil {
		return err
	}
	var prof profile
	if err := json.Unmarshal(b, &prof); err != nil {
		return err
	}

	for _, t := range prof.Targets {
		if t.Runtime != "ollama" {
			continue
		}
		for _, m := range t.Models {
			if m == "" {
				continue
			}
			if err := pl.Pull(ctx, m); err != nil {
				// pull 실패는 “치명”이 될 수 있으니 그대로 반환
				return errors.New("ollama pull failed: " + err.Error())
			}
		}
	}
	return nil
}
7) 작업완료 후 품질 벤치(테스트/빌드) 실행
internal/benchmark/executor.go (추가)
코드 복사
Go
package benchmark

import (
	"bytes"
	"context"
	"os/exec"
	"time"
)

type ExecResult struct {
	ExitCode   int
	DurationMS int
	Stdout     string
	Stderr     string
}

func Run(ctx context.Context, cmd string, args []string, timeout time.Duration, workdir string) ExecResult {
	cctx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		cctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	c := exec.CommandContext(cctx, cmd, args...)
	if workdir != "" {
		c.Dir = workdir
	}

	var outb, errb bytes.Buffer
	c.Stdout = &outb
	c.Stderr = &errb

	start := time.Now()
	err := c.Run()
	dur := int(time.Since(start).Milliseconds())

	exit := 0
	if err != nil {
		// exit code best effort
		if ee, ok := err.(*exec.ExitError); ok && ee.ProcessState != nil {
			exit = ee.ProcessState.ExitCode()
		} else {
			exit = 127
		}
	}
	return ExecResult{
		ExitCode:   exit,
		DurationMS: dur,
		Stdout:     outb.String(),
		Stderr:     errb.String(),
	}
}
internal/benchmark/suite.go (추가)
코드 복사
Go
package benchmark

import (
	"context"
	"time"

	"devorch/internal/okaon"
	"devorch/internal/router"
)

type Suite struct {
	Name       string
	Command    string
	Args       []string
	TimeoutSec int
	Workdir    string
	// 점수 산정 함수 (exit, duration 등)
	ScoreFn func(r ExecResult) float64
}

func DefaultSuites(workdir string) []Suite {
	return []Suite{
		{
			Name:       "go_test",
			Command:    "go",
			Args:       []string{"test", "./..."},
			TimeoutSec: 600,
			Workdir:    workdir,
			ScoreFn: func(r ExecResult) float64 {
				if r.ExitCode == 0 {
					return 1.0
				}
				return 0.0
			},
		},
		{
			Name:       "go_build",
			Command:    "go",
			Args:       []string{"build", "./..."},
			TimeoutSec: 600,
			Workdir:    workdir,
			ScoreFn: func(r ExecResult) float64 {
				if r.ExitCode == 0 {
					return 0.8
				}
				return 0.0
			},
		},
	}
}

type Runner struct {
	Store *okaon.Store
}

func (r *Runner) RunAndStore(ctx context.Context, wl router.Workload, suites []Suite) (float64, error) {
	if len(suites) == 0 {
		return 0.6, nil
	}
	sum := 0.0
	for _, s := range suites {
		timeout := time.Duration(s.TimeoutSec) * time.Second
		res := Run(ctx, s.Command, s.Args, timeout, s.Workdir)
		score := 0.0
		if s.ScoreFn != nil {
			score = s.ScoreFn(res)
		}

		if r.Store != nil {
			_ = r.Store.InsertBench(ctx, okaon.BenchRecord{
				WorkspaceID: wl.WorkspaceID,
				UserID:      wl.UserID,
				ProjectID:   wl.ProjectID,
				Category:    wl.Category,
				TaskType:    wl.TaskType,
				Suite:       s.Name,
				Command:     s.Command,
				TimeoutSec:  s.TimeoutSec,
				ExitCode:    res.ExitCode,
				DurationMS:  res.DurationMS,
				Score:       score,
				Stdout:      trim(res.Stdout, 4000),
				Stderr:      trim(res.Stderr, 4000),
			})
		}
		sum += score
	}
	avg := sum / float64(len(suites))
	return avg, nil
}

func trim(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
8) SessionCaller: “벤치 점수”를 Run.Quality에 반영
internal/session/llm_call.go (변경)
코드 복사
Go
package session

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"

	"devorch/internal/benchmark"
	"devorch/internal/okaon"
	"devorch/internal/provider"
	"devorch/internal/router"
)

type Caller struct {
	Bench *benchmark.Runner
}

func NewCaller(bench *benchmark.Runner) *Caller { return &Caller{Bench: bench} }

func (c *Caller) CallAndRecord(
	ctx context.Context,
	reg *provider.Registry,
	rt *router.Router,
	okStore *okaon.Store,
	wl router.Workload,
	candidates []router.Candidate,
	req provider.ChatRequest,
	workdir string,
) (provider.ChatResponse, error) {
	start := time.Now()

	dec, err := rt.Select(ctx, wl, candidates)
	if err != nil {
		return provider.ChatResponse{}, err
	}

	p, ok := reg.Get(dec.Provider)
	if !ok {
		return provider.ChatResponse{}, provider.Err("provider_not_found", dec.Provider)
	}

	req.Model = dec.Model
	resp, callErr := p.Chat(ctx, req)
	lat := int(time.Since(start).Milliseconds())

	// 1) 기본 품질(성공/지연 기반)
	quality := estimateQuality(callErr == nil)
	if callErr == nil && dec.Provider == "ollama" {
		quality = qualityFromLatency(lat)
	}

	// 2) Step3: 작업완료 기준 벤치 실행(가능하면)
	if callErr == nil && c != nil && c.Bench != nil && workdir != "" {
		benchScore, _ := c.Bench.RunAndStore(ctx, wl, benchmark.DefaultSuites(workdir))
		// 매우 보수적으로 합성 (벤치가 0이면 품질 대폭 하락)
		quality = 0.5*quality + 0.5*benchScore
	}

	run := okaon.Run{
		RunID:     ulid.Make().String(),
		CreatedAt: time.Now().Format(time.RFC3339Nano),

		PolicyKey: router.BuildPolicyKey(wl),

		WorkspaceID: wl.WorkspaceID,
		UserID:      wl.UserID,
		ProjectID:   wl.ProjectID,

		AgentType: wl.AgentType,
		Category:  wl.Category,
		TaskType:  wl.TaskType,

		Provider: dec.Provider,
		Model:    dec.Model,
		Endpoint: dec.Endpoint,

		LatencyMS: lat,
		OK:        boolToInt(callErr == nil),
		Quality:   quality,

		PromptTokens:     resp.PromptTokens,
		CompletionTokens: resp.CompletionTokens,
		TotalTokens:      resp.TotalTokens,
		CostMicroUSD:     resp.CostMicroUSD,
	}

	if callErr != nil {
		run.ErrorCode = "provider_call_failed"
		run.ErrorMessage = callErr.Error()
	}

	if okStore != nil {
		_ = okStore.InsertRun(ctx, run)
	}

	if callErr != nil {
		return provider.ChatResponse{}, callErr
	}
	return resp, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func estimateQuality(ok bool) float64 {
	if !ok {
		return 0.0
	}
	return 0.60
}

func qualityFromLatency(ms int) float64 {
	if ms <= 300 {
		return 0.65
	}
	if ms <= 1200 {
		return 0.62
	}
	if ms <= 3000 {
		return 0.58
	}
	return 0.52
}
9) App wiring: BenchRunner 주입 + Strategy 선택(egreedy/ucb1)
internal/app/wire.go (변경)
코드 복사
Go
package app

import (
	"context"
	"path/filepath"
	"runtime"
	"time"

	"devorch/internal/benchmark"
	"devorch/internal/config"
	"devorch/internal/global"
	"devorch/internal/localensure"
	"devorch/internal/okaon"
	"devorch/internal/provider"
	"devorch/internal/provider/ollama"
	"devorch/internal/provider/openai"
	"devorch/internal/provider/openrouter"
	"devorch/internal/router"
	"devorch/internal/session"
	"devorch/internal/storage/sqlite"
)

func WireMinimal(cfg config.Config) (*Deps, error) {
	db, err := sqlite.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}

	reg := provider.NewRegistry()
	if cfg.OpenAIKey != "" {
		reg.Register(openai.New(cfg.OpenAIKey))
	}
	if cfg.OpenRouterKey != "" {
		reg.Register(openrouter.New(cfg.OpenRouterKey))
	}

	okStore := okaon.NewStore(db.SQL)

	ollamaBin := cfg.Ollama.BundleOverride
	if ollamaBin == "" {
		ollamaBin = bundledOllamaPath(cfg.Local.RuntimeDir)
	}

	if cfg.Ollama.AutoStart && ollamaBin != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if err := ollama.Health(ctx, cfg.Ollama.Host); err != nil {
			_ = global.EnsureDirs()
			stateDir := filepath.Join(global.DataDir(), "state")
			_ = ollama.StartBundled(ctx, ollamaBin, cfg.Ollama.Host, stateDir)
		}
	}

	reg.Register(ollama.New(cfg.Ollama.Host, ollamaBin))

	// Step3: 로컬 모델 자동 준비(profile=mid 기본)
	{
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		_ = localensure.EnsureLocalModels(ctx, cfg, reg, "mid")
	}

	enricher := &router.OkAONEnricher{Store: okStore}

	// 전략 선택: DEVORCH_STRATEGY=ucb1|egreedy
	strategy := router.NewEpsilonGreedy()
	if cfgStrategy := getenv("DEVORCH_STRATEGY"); cfgStrategy == "ucb1" {
		strategy = router.NewUCB1()
	}

	rt := router.New(router.DefaultWeights(), strategy, enricher)

	benchRunner := &benchmark.Runner{Store: okStore}
	caller := session.NewCaller(benchRunner)

	return &Deps{
		DB:               db,
		ProviderRegistry: reg,
		OkAONStore:       okStore,
		Router:           rt,
		SessionCaller:    caller,
	}, nil
}

func bundledOllamaPath(runtimeDir string) string {
	exe := "ollama"
	if runtime.GOOS == "windows" {
		exe = "ollama.exe"
	}
	osarch := runtime.GOOS + "-" + runtime.GOARCH
	return filepath.Join(runtimeDir, "llm", "ollama", osarch, exe)
}

func getenv(k string) string {
	// stdlib os.Getenv 분리(파일 간 import 순서 깔끔화)
	return ""
}
⚠️ 위 getenv는 빈 구현이면 안 됩니다. 아래 파일을 추가합니다.
internal/app/env.go (추가)
코드 복사
Go
package app

import "os"

func getenv(k string) string { return os.Getenv(k) }
10) CLI: auto-chat / auto-bench 추가 + 후보군 생성 연결
cmd/devorch/main.go (변경)
코드 복사
Go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"devorch/internal/app"
	"devorch/internal/autocand"
	"devorch/internal/config"
	"devorch/internal/log"
	"devorch/internal/provider"
	"devorch/internal/router"
)

func usage() {
	fmt.Println(`devorch (Step3)

Commands:
  devorch doctor
  devorch providers
  devorch models --provider openai|openrouter|ollama
  devorch ollama-pull --model <model>
  devorch chat --provider openai|openrouter|ollama --model <model> --prompt "hi"
  devorch auto-chat --category quick|ultrabrain --task chat|code --prompt "..."
      [--workdir .]  (벤치 품질 반영)
`)
}

func main() {
	log.Init()

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Errorf("config load failed: %v", err)
		os.Exit(1)
	}

	deps, err := app.WireMinimal(cfg)
	if err != nil {
		log.Errorf("wire failed: %v", err)
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "doctor":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for _, pinfo := range deps.ProviderRegistry.List() {
			p, _ := deps.ProviderRegistry.Get(pinfo.Name)
			if err := p.Health(ctx); err != nil {
				log.Errorf("provider %s health failed: %v", pinfo.Name, err)
				os.Exit(1)
			}
		}
		fmt.Println("OK")

	case "providers":
		for _, p := range deps.ProviderRegistry.List() {
			fmt.Printf("- %s (%s)\n", p.Name, p.Kind)
		}

	case "models":
		fs := flag.NewFlagSet("models", flag.ExitOnError)
		provName := fs.String("provider", "", "openai|openrouter|ollama")
		_ = fs.Parse(os.Args[2:])
		if *provName == "" {
			fmt.Println("missing --provider")
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		p, ok := deps.ProviderRegistry.Get(*provName)
		if !ok {
			fmt.Printf("provider not found: %s\n", *provName)
			os.Exit(2)
		}
		models, err := p.ListModels(ctx)
		if err != nil {
			log.Errorf("list models failed: %v", err)
			os.Exit(1)
		}
		for _, m := range models {
			fmt.Printf("- %s\n", m.ID)
		}

	case "ollama-pull":
		fs := flag.NewFlagSet("ollama-pull", flag.ExitOnError)
		model := fs.String("model", "", "ollama model name")
		_ = fs.Parse(os.Args[2:])
		if *model == "" {
			fmt.Println("missing --model")
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()

		p, ok := deps.ProviderRegistry.Get("ollama")
		if !ok {
			fmt.Println("ollama provider not registered")
			os.Exit(1)
		}
		type puller interface {
			Pull(ctx context.Context, model string) error
		}
		if pl, ok := p.(puller); ok {
			if err := pl.Pull(ctx, *model); err != nil {
				log.Errorf("ollama pull failed: %v", err)
				os.Exit(1)
			}
			fmt.Println("OK")
		} else {
			fmt.Println("ollama pull not supported")
			os.Exit(1)
		}

	case "chat":
		fs := flag.NewFlagSet("chat", flag.ExitOnError)
		provName := fs.String("provider", "", "openai|openrouter|ollama")
		model := fs.String("model", "", "model id")
		prompt := fs.String("prompt", "", "prompt text")
		_ = fs.Parse(os.Args[2:])
		if *provName == "" || *model == "" || *prompt == "" {
			fmt.Println("missing --provider/--model/--prompt")
			os.Exit(2)
		}

		p, ok := deps.ProviderRegistry.Get(*provName)
		if !ok {
			fmt.Printf("provider not found: %s\n", *provName)
			os.Exit(2)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		req := provider.ChatRequest{
			Model: *model,
			Messages: []provider.ChatMessage{
				{Role: "user", Content: *prompt},
			},
			Temperature: 0.2,
		}

		wl := router.Workload{
			WorkspaceID: "local",
			UserID:      "local",
			ProjectID:   "adhoc",
			AgentType:   "cli",
			Category:    "quick",
			TaskType:    "chat",
		}

		cands := []router.Candidate{
			{Provider: p.Name(), Model: *model, Endpoint: p.Endpoint()},
		}

		resp, err := deps.SessionCaller.CallAndRecord(ctx, deps.ProviderRegistry, deps.Router, deps.OkAONStore, wl, cands, req, "")
		if err != nil {
			log.Errorf("chat failed: %v", err)
			os.Exit(1)
		}
		fmt.Println(resp.Text())

	case "auto-chat":
		fs := flag.NewFlagSet("auto-chat", flag.ExitOnError)
		category := fs.String("category", "quick", "quick|ultrabrain|...")
		task := fs.String("task", "chat", "chat|code|refactor|...")
		prompt := fs.String("prompt", "", "prompt text")
		workdir := fs.String("workdir", ".", "project workdir for bench")
		_ = fs.Parse(os.Args[2:])
		if *prompt == "" {
			fmt.Println("missing --prompt")
			os.Exit(2)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		wl := router.Workload{
			WorkspaceID: "local",
			UserID:      "local",
			ProjectID:   "localproj",
			AgentType:   "cli-auto",
			Category:    *category,
			TaskType:    *task,
		}

		gen := autocand.New(cfg, deps.ProviderRegistry)
		cands, err := gen.Generate(ctx, wl)
		if err != nil {
			log.Errorf("generate candidates failed: %v", err)
			os.Exit(1)
		}

		req := provider.ChatRequest{
			Model: "",
			Messages: []provider.ChatMessage{
				{Role: "user", Content: *prompt},
			},
			Temperature: 0.2,
		}

		resp, err := deps.SessionCaller.CallAndRecord(ctx, deps.ProviderRegistry, deps.Router, deps.OkAONStore, wl, cands, req, *workdir)
		if err != nil {
			log.Errorf("auto-chat failed: %v", err)
			os.Exit(1)
		}
		fmt.Println(resp.Text())

	default:
		usage()
		os.Exit(2)
	}
}
11) provider.Registry: List() / Get() 유지 확인용(누락 방지)
(만약 Step1/2에서 Registry가 미완이면 아래로 교체하세요)
internal/provider/registry.go (전체 제공)
코드 복사
Go
package provider

import "sync"

type ProviderInfo struct {
	Name string
	Kind string
}

type Registry struct {
	mu sync.RWMutex
	m  map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{m: make(map[string]Provider)}
}

func (r *Registry) Register(p Provider) {
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

func (r *Registry) List() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ProviderInfo, 0, len(r.m))
	for _, p := range r.m {
		out = append(out, ProviderInfo{Name: p.Name(), Kind: p.Kind()})
	}
	return out
}
12) provider