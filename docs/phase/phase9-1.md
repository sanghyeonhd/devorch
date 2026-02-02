좋아요. “계속 개발”의 다음 묶음(= Step 9)은 Doctor/자가진단 + 로컬 벤치마크 이력 저장( SQLite ) 파트를 풀코드로 드리겠습니다.
(전체 레포 “전부”를 한 번에 출력하기엔 분량이 너무 커서, 이번 턴은 Step 9 범위만 완성본으로 제공합니다. 다음 턴에 Step 10 묶음으로 이어가면 됩니다.)

아래 코드는 당신이 올려준 devorch 트리 기준 경로 그대로입니다.


---

1) 추가되는 파일/경로 (Step 9)

internal/diagnostics/doctor.go

internal/diagnostics/report.go

internal/diagnostics/checks/*.go (필수 체크들)

internal/storage/sqlite/migrations/0003_benchmarks.sql

internal/storage/sqlite/benchmarks.go

internal/bench/bench.go

internal/bench/runner.go


> 전제: 이미 Step 1~8에서 internal/storage/sqlite/sqlite.go, internal/log, internal/provider의 최소 인터페이스가 존재한다고 가정합니다.
만약 아직 없다면, 제가 다음 메시지에서 누락된 인터페이스/스텁을 먼저 채우는 패치로 이어갈게요.




---

2) 코드

2.1 internal/diagnostics/doctor.go

package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"devorch/internal/log"
)

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarn    Severity = "warn"
	SeverityError   Severity = "error"
	SeverityCritical Severity = "critical"
)

type CheckResult struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Severity    Severity               `json:"severity"`
	Passed      bool                   `json:"passed"`
	Summary     string                 `json:"summary"`
	Details     map[string]any         `json:"details,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	StartedAt   time.Time              `json:"startedAt"`
	FinishedAt  time.Time              `json:"finishedAt"`
	DurationMs  int64                  `json:"durationMs"`
}

type Check interface {
	ID() string
	Name() string
	Run(ctx context.Context, in Input) CheckResult
}

type Input struct {
	Log log.Logger

	// Optional fields - 존재하면 활용
	Version     string
	OS          string
	Arch        string
	DataDir     string
	ConfigDir   string
	CacheDir    string
	StateDir    string
	LogDir      string

	// 아래 의존성은 Step 1~8에서 주입된다고 가정
	Bench BenchAPI
}

type BenchAPI interface {
	RunQuick(ctx context.Context) (QuickBenchSummary, error)
}

type QuickBenchSummary struct {
	CreatedAt time.Time `json:"createdAt"`
	Items     []any     `json:"items"`
}

type Doctor struct {
	logger log.Logger
	checks []Check
}

func NewDoctor(l log.Logger) *Doctor {
	return &Doctor{logger: l, checks: make([]Check, 0, 32)}
}

func (d *Doctor) Register(c Check) {
	d.checks = append(d.checks, c)
}

func (d *Doctor) Run(ctx context.Context, in Input) Report {
	start := time.Now()

	if in.Log == nil {
		in.Log = d.logger
	}

	// stable order
	sort.SliceStable(d.checks, func(i, j int) bool {
		return d.checks[i].ID() < d.checks[j].ID()
	})

	results := make([]CheckResult, 0, len(d.checks))
	var hasError bool
	var hasCritical bool

	for _, c := range d.checks {
		r := c.Run(ctx, in)
		results = append(results, r)
		if !r.Passed {
			if r.Severity == SeverityCritical {
				hasCritical = true
			}
			if r.Severity == SeverityError || r.Severity == SeverityCritical {
				hasError = true
			}
		}
	}

	status := "ok"
	if hasError {
		status = "degraded"
	}
	if hasCritical {
		status = "failed"
	}

	return Report{
		Status:     status,
		StartedAt:  start,
		FinishedAt: time.Now(),
		Results:    results,
	}
}

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func CombineErr(errs ...error) error {
	var out []error
	for _, e := range errs {
		if e != nil {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil
	}
	if len(out) == 1 {
		return out[0]
	}
	return errors.New(fmt.Sprintf("multiple errors: %v", out))
}


---

2.2 internal/diagnostics/report.go

package diagnostics

import (
	"encoding/json"
	"time"
)

type Report struct {
	Status     string        `json:"status"` // ok|degraded|failed
	StartedAt  time.Time     `json:"startedAt"`
	FinishedAt time.Time     `json:"finishedAt"`
	Results    []CheckResult `json:"results"`
}

func (r Report) DurationMs() int64 {
	return r.FinishedAt.Sub(r.StartedAt).Milliseconds()
}

func (r Report) JSON(pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(r, "", "  ")
	}
	return json.Marshal(r)
}

func (r Report) HasFailures() bool {
	for _, x := range r.Results {
		if !x.Passed && (x.Severity == SeverityError || x.Severity == SeverityCritical) {
			return true
		}
	}
	return false
}


---

2.3 Checks

2.3.1 internal/diagnostics/checks/installation.go

package checks

import (
	"context"
	"os"
	"time"

	"devorch/internal/diagnostics"
)

type Installation struct{}

func (c Installation) ID() string   { return "installation" }
func (c Installation) Name() string { return "Installation / Paths" }

func (c Installation) Run(ctx context.Context, in diagnostics.Input) diagnostics.CheckResult {
	start := time.Now()
	defer func() { _ = ctx }()

	details := map[string]any{
		"os":      in.OS,
		"arch":    in.Arch,
		"version": in.Version,
		"paths": map[string]any{
			"config": in.ConfigDir,
			"data":   in.DataDir,
			"cache":  in.CacheDir,
			"state":  in.StateDir,
			"log":    in.LogDir,
		},
	}

	missing := make([]string, 0, 8)
	checkDir := func(label, p string) {
		if p == "" {
			missing = append(missing, label+":<empty>")
			return
		}
		st, err := os.Stat(p)
		if err != nil || !st.IsDir() {
			missing = append(missing, label+":"+p)
		}
	}

	checkDir("config", in.ConfigDir)
	checkDir("data", in.DataDir)
	checkDir("cache", in.CacheDir)
	checkDir("state", in.StateDir)
	checkDir("log", in.LogDir)

	passed := len(missing) == 0
	severity := diagnostics.SeverityInfo
	summary := "All required directories exist"
	suggestions := []string(nil)

	if !passed {
		severity = diagnostics.SeverityError
		summary = "Some required directories are missing or not accessible"
		suggestions = []string{
			"Run `devorch install` to create required directories",
			"Check filesystem permissions",
		}
		details["missing"] = missing
	}

	finish := time.Now()
	return diagnostics.CheckResult{
		ID:          c.ID(),
		Name:        c.Name(),
		Severity:    severity,
		Passed:      passed,
		Summary:     summary,
		Details:     details,
		Suggestions: suggestions,
		StartedAt:   start,
		FinishedAt:  finish,
		DurationMs:  finish.Sub(start).Milliseconds(),
	}
}

2.3.2 internal/diagnostics/checks/config.go

package checks

import (
	"context"
	"time"

	"devorch/internal/diagnostics"
)

type Config struct{}

func (c Config) ID() string   { return "config" }
func (c Config) Name() string { return "Configuration" }

func (c Config) Run(ctx context.Context, in diagnostics.Input) diagnostics.CheckResult {
	start := time.Now()
	defer func() { _ = ctx }()

	// Step 1~8에서 config.Validate(...) 같은 게 존재한다면 여기서 호출하도록 확장하세요.
	// 지금은 “경로 존재 여부” 수준의 최소 체크만 수행합니다.
	passed := in.ConfigDir != ""

	severity := diagnostics.SeverityInfo
	summary := "Configuration looks okay"
	suggestions := []string(nil)

	if !passed {
		severity = diagnostics.SeverityError
		summary = "Config directory is empty"
		suggestions = []string{
			"Set XDG_CONFIG_HOME or ensure config directory resolution works",
			"Run `devorch debug paths` to verify resolved paths",
		}
	}

	finish := time.Now()
	return diagnostics.CheckResult{
		ID:          c.ID(),
		Name:        c.Name(),
		Severity:    severity,
		Passed:      passed,
		Summary:     summary,
		Details:     map[string]any{"configDir": in.ConfigDir},
		Suggestions: suggestions,
		StartedAt:   start,
		FinishedAt:  finish,
		DurationMs:  finish.Sub(start).Milliseconds(),
	}
}

2.3.3 internal/diagnostics/checks/oauth.go

package checks

import (
	"context"
	"time"

	"devorch/internal/diagnostics"
)

type OAuth struct{}

func (c OAuth) ID() string   { return "oauth" }
func (c OAuth) Name() string { return "OAuth / Token Store" }

func (c OAuth) Run(ctx context.Context, in diagnostics.Input) diagnostics.CheckResult {
	start := time.Now()
	defer func() { _ = ctx }()

	// Step 1~8에서 token store health 검사 함수가 있으면 여기에 연결하세요.
	// 지금은 “토큰 저장 경로(stateDir)”만 최소 확인.
	passed := in.StateDir != ""

	severity := diagnostics.SeverityInfo
	summary := "Token store path is available"
	suggestions := []string(nil)

	if !passed {
		severity = diagnostics.SeverityError
		summary = "State directory is empty; token store may not work"
		suggestions = []string{
			"Ensure state directory is resolved and writable",
			"On macOS/Windows/Linux, verify keychain/credman/libsecret integrations",
		}
	}

	finish := time.Now()
	return diagnostics.CheckResult{
		ID:          c.ID(),
		Name:        c.Name(),
		Severity:    severity,
		Passed:      passed,
		Summary:     summary,
		Details:     map[string]any{"stateDir": in.StateDir},
		Suggestions: suggestions,
		StartedAt:   start,
		FinishedAt:  finish,
		DurationMs:  finish.Sub(start).Milliseconds(),
	}
}

2.3.4 internal/diagnostics/checks/performance.go

package checks

import (
	"context"
	"time"

	"devorch/internal/diagnostics"
)

type Performance struct{}

func (c Performance) ID() string   { return "performance" }
func (c Performance) Name() string { return "Performance / Quick Bench" }

func (c Performance) Run(ctx context.Context, in diagnostics.Input) diagnostics.CheckResult {
	start := time.Now()

	details := map[string]any{}
	passed := true
	severity := diagnostics.SeverityInfo
	summary := "Quick bench succeeded"
	suggestions := []string(nil)

	if in.Bench == nil {
		passed = false
		severity = diagnostics.SeverityWarn
		summary = "Bench runner not configured"
		suggestions = []string{
			"Wire BenchRunner into diagnostics input",
			"Enable benchmark history storage to improve routing over time",
		}
	} else {
		sum, err := in.Bench.RunQuick(ctx)
		if err != nil {
			passed = false
			severity = diagnostics.SeverityWarn
			summary = "Quick bench failed"
			details["error"] = err.Error()
			suggestions = []string{
				"Verify local runtime (ollama/llama.cpp) is reachable",
				"Verify remote providers (openrouter/openai/anthropic) credentials",
				"Try again with `devorch bench --quick --verbose`",
			}
		} else {
			details["createdAt"] = sum.CreatedAt
			details["items"] = sum.Items
		}
	}

	finish := time.Now()
	return diagnostics.CheckResult{
		ID:          c.ID(),
		Name:        c.Name(),
		Severity:    severity,
		Passed:      passed,
		Summary:     summary,
		Details:     details,
		Suggestions: suggestions,
		StartedAt:   start,
		FinishedAt:  finish,
		DurationMs:  finish.Sub(start).Milliseconds(),
	}
}


---

2.4 Bench + SQLite 저장

2.4.1 internal/storage/sqlite/migrations/0003_benchmarks.sql

-- Benchmarks history (local machine oriented)
-- Stores “task-complete benchmark result” to improve future routing.

CREATE TABLE IF NOT EXISTS benchmarks (
  id TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  os TEXT NOT NULL,
  arch TEXT NOT NULL,
  machine_fingerprint TEXT NOT NULL,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,

  scenario TEXT NOT NULL,          -- e.g. "quick", "code-edit", "chat", "tool-call"
  input_tokens INTEGER NOT NULL,
  output_tokens INTEGER NOT NULL,

  latency_ms INTEGER NOT NULL,
  success INTEGER NOT NULL,         -- 1/0
  error TEXT,

  quality_score REAL,               -- optional 0..1
  cost_usd REAL,                    -- optional
  metadata_json TEXT                -- optional JSON
);

CREATE INDEX IF NOT EXISTS idx_benchmarks_created_at ON benchmarks(created_at);
CREATE INDEX IF NOT EXISTS idx_benchmarks_model ON benchmarks(provider, model);
CREATE INDEX IF NOT EXISTS idx_benchmarks_machine ON benchmarks(machine_fingerprint);

2.4.2 internal/storage/sqlite/benchmarks.go

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type BenchmarkRow struct {
	ID                string
	CreatedAt         time.Time
	OS                string
	Arch              string
	MachineFingerprint string

	Provider string
	Model    string

	Scenario     string
	InputTokens  int64
	OutputTokens int64

	LatencyMs int64
	Success   bool
	Error     string

	QualityScore *float64
	CostUSD      *float64
	Metadata     map[string]any
}

type BenchmarksStore struct {
	db *sql.DB
}

func NewBenchmarksStore(db *sql.DB) *BenchmarksStore {
	return &BenchmarksStore{db: db}
}

func (s *BenchmarksStore) Insert(ctx context.Context, r BenchmarkRow) error {
	if s.db == nil {
		return errors.New("sqlite: db is nil")
	}

	meta := ""
	if r.Metadata != nil {
		b, err := json.Marshal(r.Metadata)
		if err != nil {
			return err
		}
		meta = string(b)
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO benchmarks(
  id, created_at, os, arch, machine_fingerprint,
  provider, model,
  scenario, input_tokens, output_tokens,
  latency_ms, success, error,
  quality_score, cost_usd, metadata_json
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, r.ID, r.CreatedAt.UnixMilli(), r.OS, r.Arch, r.MachineFingerprint,
		r.Provider, r.Model,
		r.Scenario, r.InputTokens, r.OutputTokens,
		r.LatencyMs, boolToInt(r.Success), r.Error,
		r.QualityScore, r.CostUSD, meta)
	return err
}

func (s *BenchmarksStore) RecentByModel(ctx context.Context, provider, model string, limit int) ([]BenchmarkRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, created_at, os, arch, machine_fingerprint,
       provider, model,
       scenario, input_tokens, output_tokens,
       latency_ms, success, error,
       quality_score, cost_usd, metadata_json
FROM benchmarks
WHERE provider = ? AND model = ?
ORDER BY created_at DESC
LIMIT ?
`, provider, model, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]BenchmarkRow, 0, limit)
	for rows.Next() {
		var (
			id, os, arch, fp, prov, mdl, scenario, errStr, metaStr string
			createdAtMs                                           int64
			inTok, outTok, latMs                                   int64
			successInt                                             int
			quality, cost                                          sql.NullFloat64
		)

		if scanErr := rows.Scan(&id, &createdAtMs, &os, &arch, &fp,
			&prov, &mdl,
			&scenario, &inTok, &outTok,
			&latMs, &successInt, &errStr,
			&quality, &cost, &metaStr,
		); scanErr != nil {
			return nil, scanErr
		}

		var meta map[string]any
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &meta)
		}

		var qPtr *float64
		if quality.Valid {
			v := quality.Float64
			qPtr = &v
		}

		var cPtr *float64
		if cost.Valid {
			v := cost.Float64
			cPtr = &v
		}

		out = append(out, BenchmarkRow{
			ID:                id,
			CreatedAt:         time.UnixMilli(createdAtMs),
			OS:                os,
			Arch:              arch,
			MachineFingerprint: fp,
			Provider:          prov,
			Model:             mdl,
			Scenario:          scenario,
			InputTokens:       inTok,
			OutputTokens:      outTok,
			LatencyMs:         latMs,
			Success:           successInt == 1,
			Error:             errStr,
			QualityScore:      qPtr,
			CostUSD:           cPtr,
			Metadata:          meta,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}


---

2.5 Bench Runner

2.5.1 internal/bench/bench.go

package bench

import (
	"context"
	"time"
)

type Item struct {
	Provider   string         `json:"provider"`
	Model      string         `json:"model"`
	Scenario   string         `json:"scenario"`
	LatencyMs  int64          `json:"latencyMs"`
	Success    bool           `json:"success"`
	Error      string         `json:"error,omitempty"`
	InputTok   int64          `json:"inputTokens"`
	OutputTok  int64          `json:"outputTokens"`
	Quality    *float64       `json:"qualityScore,omitempty"`
	CostUSD    *float64       `json:"costUsd,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type QuickSummary struct {
	CreatedAt time.Time `json:"createdAt"`
	Items     []Item    `json:"items"`
}

type Runner interface {
	RunQuick(ctx context.Context) (QuickSummary, error)
}

2.5.2 internal/bench/runner.go

package bench

import (
	"context"
	"errors"
	"time"

	"devorch/internal/id"
	"devorch/internal/log"
	"devorch/internal/storage/sqlite"
)

type ProviderProbe interface {
	// “아주 짧은 작업”으로 RTT/응답성을 측정하는 최소 단위
	// Step 1~8 provider abstraction과 연결하세요.
	Probe(ctx context.Context, provider string, model string) (latency time.Duration, inTok, outTok int64, costUSD *float64, err error)
}

type MachineInfo interface {
	OS() string
	Arch() string
	Fingerprint() string
}

type RunnerImpl struct {
	log    log.Logger
	probe  ProviderProbe
	store  *sqlite.BenchmarksStore
	machine MachineInfo

	// 벤치할 대상 (local+remote)
	Targets []Target
}

type Target struct {
	Provider string
	Model    string
	Scenario string // "quick"
}

func NewRunner(l log.Logger, probe ProviderProbe, store *sqlite.BenchmarksStore, machine MachineInfo) *RunnerImpl {
	return &RunnerImpl{
		log:     l,
		probe:   probe,
		store:   store,
		machine: machine,
		Targets: []Target{},
	}
}

func (r *RunnerImpl) RunQuick(ctx context.Context) (QuickSummary, error) {
	if r.probe == nil {
		return QuickSummary{}, errors.New("bench: probe is nil")
	}
	if r.machine == nil {
		return QuickSummary{}, errors.New("bench: machine info is nil")
	}
	if len(r.Targets) == 0 {
		// 기본값: 최소 2개(로컬/원격) 정도만.
		r.Targets = []Target{
			{Provider: "ollama", Model: "llama3.1:8b", Scenario: "quick"},
			{Provider: "openrouter", Model: "openrouter/auto", Scenario: "quick"},
		}
	}

	now := time.Now()
	items := make([]Item, 0, len(r.Targets))

	for _, t := range r.Targets {
		lat, inTok, outTok, cost, err := r.probe.Probe(ctx, t.Provider, t.Model)
		it := Item{
			Provider:  t.Provider,
			Model:     t.Model,
			Scenario:  t.Scenario,
			LatencyMs: lat.Milliseconds(),
			Success:   err == nil,
			InputTok:  inTok,
			OutputTok: outTok,
		}
		if err != nil {
			it.Error = err.Error()
		}
		if cost != nil {
			it.CostUSD = cost
		}
		items = append(items, it)

		// DB에 저장 (store가 없으면 skip)
		if r.store != nil {
			_ = r.store.Insert(ctx, sqlite.BenchmarkRow{
				ID:                 id.NewULID(),
				CreatedAt:          now,
				OS:                 r.machine.OS(),
				Arch:               r.machine.Arch(),
				MachineFingerprint: r.machine.Fingerprint(),
				Provider:           t.Provider,
				Model:              t.Model,
				Scenario:           t.Scenario,
				InputTokens:        inTok,
				OutputTokens:       outTok,
				LatencyMs:          lat.Milliseconds(),
				Success:            err == nil,
				Error:              errString(err),
				CostUSD:            cost,
				Metadata: map[string]any{
					"type": "quick_probe",
				},
			})
		}
	}

	return QuickSummary{CreatedAt: now, Items: items}, nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}


---

3) Step 9 적용 방법 (연결 포인트)

devorch doctor 명령이 이미 있다면:

internal/diagnostics.NewDoctor(...)

Register(checks.Installation{}), Register(checks.Config{}), Register(checks.OAuth{}), Register(checks.Performance{})

Input{ ... Bench: <benchRunnerAdapter> } 주입


Bench 실행이 이미 있다면:

RunnerImpl을 daemon에 주입하고

storage/sqlite 마이그레이션에 0003_benchmarks.sql 추가




---

4) 다음 Step 10 예고 (다음 메시지에서 바로 이어서 가능)

당신이 원하는 “작업 완료 기준 벤치마크를 이력으로 저장하고, 라우팅이 점점 좋아지는 구조”를 제대로 만들려면 다음이 이어져야 합니다.

router/score.go에서 benchmarks 최근 결과를 가져와

동일 OS/Arch/FP 가중치

scenario별 지표(예: code-edit vs quick) 분리

latency/err-rate/품질점수/비용을 합쳐 스코어링


그리고 session/processor.go의 “작업 완료 시점”에

응답 품질(간단한 휴리스틱 + 사용자가 thumbs up/down) 저장

tool 성공률 포함




---

원하면 바로 다음 턴에 Step 10: Router 스코어링(benchmarks 기반) + 세션 완료 훅에서 벤치 저장까지 “풀코드”로 이어서 드릴게요.