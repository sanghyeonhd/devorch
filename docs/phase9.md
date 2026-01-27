좋습니다. **9단계는 “8단계 MVP를 실제 운영급으로 완성”**하는 단계로 보고, 아래 4가지를 풀코드로 넣습니다.
OAuth 세션/토큰 기반 authz.Authorizer 실제 구현 (+ SQLite 세션 저장)
TestRunner 멀티 OS 지원(mac/linux: sh, windows: powershell/cmd)
Tool 실행 결과 구조화(메타/exit_code/changed_files/bytes/latency) → tool_runs에 저장
Router가 “품질 점수(quality_runs) + DoD 시그널(test/tool)”을 학습해 모델 선택에 반영(+ model_stats 테이블)
아래는 9단계 추가/수정 전체 파일 풀코드입니다. (SQL 포함)
9단계 변경 트리(전체)
추가
AuthZ (실구현)
internal/authz/errors.go
internal/authz/authorizer_sqlite.go
internal/storage/sqlite/auth_sessions.go
internal/storage/sqlite/migrations/0007_auth_sessions.sql
Signal 실행(멀티 OS)
internal/signal/exec_runner.go
internal/signal/test_runner_os.go (기존 test_runner.go 대체)
Tool 결과 구조화
internal/tool/resultmeta.go
internal/storage/sqlite/migrations/0008_toolruns_meta.sql (tool_runs 확장)
Router 학습(품질 기반)
internal/router/learning.go
internal/storage/sqlite/model_stats.go
internal/storage/sqlite/migrations/0009_model_stats.sql
수정
internal/signal/tool_observer.go
internal/tool/registry.go
internal/quality/recorder.go
internal/server/routes/stream.go (이미 8단계에서 인증 적용 — 여기선 workspace 토큰 만료 처리 강화만)
internal/app/wire.go (조립 코드 예시 추가)
9-1) OAuth 세션 저장 (SQLite)
internal/storage/sqlite/migrations/0007_auth_sessions.sql
코드 복사
Sql
CREATE TABLE IF NOT EXISTS auth_sessions (
  id TEXT PRIMARY KEY,              -- session id (ulid)
  user_id TEXT NOT NULL,
  workspace TEXT NOT NULL,

  access_token_hash TEXT NOT NULL,  -- bearer hash
  expires_at INTEGER NOT NULL,       -- unix ms

  email TEXT,
  provider TEXT,                    -- "github", "openai", "google", etc
  created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_token ON auth_sessions(access_token_hash);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_ws_user ON auth_sessions(workspace, user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_exp ON auth_sessions(expires_at);
internal/storage/sqlite/auth_sessions.go
코드 복사
Go
package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"
)

type AuthSession struct {
	ID              string
	UserID          string
	Workspace       string
	AccessTokenHash string
	ExpiresAt       time.Time
	Email           string
	Provider        string
	CreatedAt       time.Time
}

type AuthSessionStore struct {
	DB *sql.DB
}

func HashBearerToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *AuthSessionStore) Insert(ctx context.Context, a AuthSession) (string, error) {
	_, err := s.DB.ExecContext(ctx, `
	  INSERT INTO auth_sessions (
	    id, user_id, workspace, access_token_hash, expires_at,
	    email, provider, created_at
	  ) VALUES (?,?,?,?,?,?,?,?)
	`, a.ID, a.UserID, a.Workspace, a.AccessTokenHash, a.ExpiresAt.UnixMilli(),
		a.Email, a.Provider, a.CreatedAt.UnixMilli(),
	)
	return a.ID, err
}

func (s *AuthSessionStore) FindByBearer(ctx context.Context, bearer string) (*AuthSession, error) {
	h := HashBearerToken(bearer)
	row := s.DB.QueryRowContext(ctx, `
	  SELECT id, user_id, workspace, access_token_hash, expires_at, email, provider, created_at
	  FROM auth_sessions
	  WHERE access_token_hash=?
	  LIMIT 1
	`, h)

	var a AuthSession
	var expMs, createdMs int64
	if err := row.Scan(&a.ID, &a.UserID, &a.Workspace, &a.AccessTokenHash, &expMs, &a.Email, &a.Provider, &createdMs); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	a.ExpiresAt = time.UnixMilli(expMs)
	a.CreatedAt = time.UnixMilli(createdMs)
	return &a, nil
}

func (s *AuthSessionStore) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM auth_sessions WHERE expires_at < ?`, now.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
9-2) AuthZ 실구현 (SQLite 세션 기반)
internal/authz/errors.go
코드 복사
Go
package authz

import "errors"

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrExpired      = errors.New("token expired")
)
internal/authz/authorizer_sqlite.go
코드 복사
Go
package authz

import (
	"context"
	"time"

	"devorch/internal/storage/sqlite"
)

type SessionLookup interface {
	FindByBearer(ctx context.Context, bearer string) (*sqlite.AuthSession, error)
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}

type SQLiteAuthorizer struct {
	Sessions SessionLookup
	Clock    func() time.Time
}

func NewSQLiteAuthorizer(s SessionLookup) *SQLiteAuthorizer {
	return &SQLiteAuthorizer{
		Sessions: s,
		Clock:    time.Now,
	}
}

func (a *SQLiteAuthorizer) Authenticate(ctx context.Context, bearer string) (*Principal, error) {
	if bearer == "" || a.Sessions == nil {
		return nil, ErrUnauthorized
	}

	now := a.Clock()
	// opportunistic cleanup (cheap)
	_, _ = a.Sessions.DeleteExpired(ctx, now)

	sess, err := a.Sessions.FindByBearer(ctx, bearer)
	if err != nil || sess == nil {
		return nil, ErrUnauthorized
	}
	if sess.ExpiresAt.Before(now) {
		return nil, ErrExpired
	}

	return &Principal{
		UserID: sess.UserID,
		Email:  sess.Email,
	}, nil
}

func (a *SQLiteAuthorizer) CanAccessWorkspace(ctx context.Context, p *Principal, workspace string) (bool, error) {
	// MVP 정책:
	// - bearer 토큰이 특정 workspace로 발급된 세션이면 접근 허용
	// - 실제 role/tenancy 기반은 이후 단계에서 확장
	// 여기서는 Authenticate 단계에서 workspace를 알 수 없으므로,
	// Stream/API에서 workspace별로 "workspace 바인딩 토큰"을 쓰는 구조를 권장.
	// 다만 현재 미들웨어 구조에서는 bearer만 주어지므로,
	// CanAccessWorkspace는 "true"로 두고, 라우트에서 workspace 바인딩을 강제하려면
	// 별도의 헤더(X-Workspace-Token) 분리 권장.
	//
	// 9단계에서는 간단히 "세션이 존재하면 허용"으로 둡니다.
	if p == nil {
		return false, ErrUnauthorized
	}
	if workspace == "" {
		return false, nil
	}
	return true, nil
}
✅ 중요: “OAuth-only로 제품 전체를 잠그는 것”은 위 Authorizer로 달성됩니다.
workspace별 강제 바인딩은 10단계에서 X-Workspace-Token(workspace-scoped token)로 더 강하게 만들면 됩니다.
9-3) 멀티 OS 실행 래퍼 (sh / powershell / cmd)
internal/signal/exec_runner.go
코드 복사
Go
package signal

import (
	"context"
	"os/exec"
	"runtime"
	"time"
)

type ExecSpec struct {
	Command string
	Dir     string
	Env     []string
	Timeout time.Duration
}

type ExecResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	TimedOut bool
}

func RunCommand(ctx context.Context, spec ExecSpec) (ExecResult, error) {
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// powershell 우선 (cmd 특수문자 문제 적음)
		cmd = exec.CommandContext(cctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", spec.Command)
	default:
		cmd = exec.CommandContext(cctx, "sh", "-lc", spec.Command)
	}

	if spec.Dir != "" {
		cmd.Dir = spec.Dir
	}
	if len(spec.Env) > 0 {
		cmd.Env = append(cmd.Environ(), spec.Env...)
	}

	stdout, err := cmd.Output()
	res := ExecResult{Stdout: stdout}

	if err == nil {
		res.ExitCode = 0
		return res, nil
	}

	// 에러 처리: stderr 추출
	if ee, ok := err.(*exec.ExitError); ok {
		res.Stderr = ee.Stderr
		res.ExitCode = ee.ExitCode()
	} else {
		res.ExitCode = 1
	}

	if cctx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
	}

	return res, err
}
internal/signal/test_runner_os.go (기존 test_runner.go를 이걸로 교체)
코드 복사
Go
package signal

import (
	"context"
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

	Runner  string
	Command string
	Dir     string
	Env     []string
	Timeout time.Duration
}

func (r *TestRunner) Run(ctx context.Context, req RunTestsRequest) (TestRun, error) {
	start := time.Now()

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

	res, err := RunCommand(ctx, ExecSpec{
		Command: req.Command,
		Dir:     req.Dir,
		Env:     req.Env,
		Timeout: req.Timeout,
	})

	end := time.Now()
	run.EndedAt = end
	run.DurationMs = end.Sub(start).Milliseconds()
	run.ExitCode = res.ExitCode
	run.Success = (err == nil && res.ExitCode == 0 && !res.TimedOut)
	run.Error = errString(err)
	run.StdoutSum = summarize(string(res.Stdout), 2000)
	run.StderrSum = summarize(string(res.Stderr), 2000)

	if r.Store != nil {
		_, _ = r.Store.InsertTestRun(ctx, run)
	}

	return run, err
}
9-4) Tool 결과 “구조화” (메타 인터페이스 + DB 확장)
internal/tool/resultmeta.go
코드 복사
Go
package tool

// Tool들이 “구조화된 결과 메타”를 선택적으로 제공하기 위한 인터페이스
// 예: exec tool => exit_code, changed_files, bytes_written
type ResultMeta interface {
	Meta() map[string]any
}
internal/storage/sqlite/migrations/0008_toolruns_meta.sql
코드 복사
Sql
ALTER TABLE tool_runs ADD COLUMN exit_code INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tool_runs ADD COLUMN meta_json TEXT;
SQLite는 ALTER TABLE ... ADD COLUMN을 허용합니다. 이미 실행된 경우 실패할 수 있으니, 실제 마이그레이션 러너에서 “이미 컬럼 존재하면 스킵” 처리하는 게 좋습니다(10단계에서 정교화).
internal/storage/sqlite/toolruns.go (수정본)
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
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
	    success, error, input_summary, output_summary, output_bytes, tags,
	    exit_code, meta_json
	  ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		r.ID, r.Workspace, r.SessionID, r.TaskID,
		r.ToolName, r.StartedAt.UnixMilli(), r.EndedAt.UnixMilli(), r.DurationMs,
		boolToInt(r.Success), r.Error, r.InputSummary, r.OutputSummary, r.OutputBytes, r.Tags,
		r.ExitCode, r.MetaJSON,
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

func (s *ToolRunStore) RecentRuns(ctx context.Context, workspace, sessionID, taskID string, limit int) ([]signal.ToolRun, error) {
	rows, err := s.DB.QueryContext(ctx, `
	  SELECT id, workspace, session_id, task_id, tool_name,
	         started_at, ended_at, duration_ms, success, error,
	         input_summary, output_summary, output_bytes, tags,
	         exit_code, meta_json
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
			&r.ExitCode, &r.MetaJSON,
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

// helper
func jsonString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
internal/signal/types.go (9단계 수정: 메타 필드 추가)
코드 복사
Go
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
	Tags        string

	ExitCode int
	MetaJSON string
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
internal/signal/tool_observer.go (9단계 수정: meta/exit_code 저장)
코드 복사
Go
package signal

import (
	"context"
	"encoding/json"
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
	Input     string
	Time      time.Time
}

type ToolEnd struct {
	Start    ToolStart
	Success  bool
	Error    error
	Output   []byte
	Time     time.Time
	ExitCode int
	Meta     map[string]any
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

	metaJSON := ""
	if end.Meta != nil {
		if b, err := json.Marshal(end.Meta); err == nil {
			metaJSON = string(b)
		}
	}

	tr := ToolRun{
		ID:            id.NewULID(),
		Workspace:     end.Start.Workspace,
		SessionID:     end.Start.SessionID,
		TaskID:        end.Start.TaskID,
		ToolName:      end.Start.ToolName,
		StartedAt:     end.Start.Time,
		EndedAt:       end.Time,
		DurationMs:    end.Time.Sub(end.Start.Time).Milliseconds(),
		Success:       end.Success,
		Error:         errString(end.Error),
		InputSummary:  inSum,
		OutputSummary: outSum,
		OutputBytes:   int64(len(end.Output)),
		Tags:          "",
		ExitCode:      end.ExitCode,
		MetaJSON:      metaJSON,
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
internal/tool/registry.go (9단계 수정: ResultMeta 추출)
코드 복사
Go
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

	// output bytes for summary
	var outBytes []byte
	if out != nil {
		if b, mErr := json.Marshal(out); mErr == nil {
			outBytes = b
		}
	}

	meta := map[string]any{}
	exitCode := 0

	// 구조화된 메타
	if rm, ok := out.(ResultMeta); ok {
		for k, v := range rm.Meta() {
			meta[k] = v
		}
	}

	// 표준 키(있으면)
	if v, ok := meta["exit_code"].(float64); ok { // json decode path
		exitCode = int(v)
	}
	if v, ok := meta["exit_code"].(int); ok {
		exitCode = v
	}

	if r.Observer != nil {
		r.Observer.OnToolEnd(ctx, signal.ToolEnd{
			Start:    ts,
			Success: err == nil,
			Error:   err,
			Output:  outBytes,
			Time:    time.Now(),
			ExitCode: exitCode,
			Meta:    meta,
		})
	}

	return out, err
}
9-5) Router 학습: 모델별 “실제 품질 점수” 축적 + 라우팅 반영
internal/storage/sqlite/migrations/0009_model_stats.sql
코드 복사
Sql
CREATE TABLE IF NOT EXISTS model_stats (
  id TEXT PRIMARY KEY,
  workspace TEXT NOT NULL,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,

  samples INTEGER NOT NULL DEFAULT 0,
  avg_score REAL NOT NULL DEFAULT 0.0,
  avg_latency_ms REAL NOT NULL DEFAULT 0.0,
  fail_rate REAL NOT NULL DEFAULT 0.0,

  updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_model_stats_ws_model
ON model_stats(workspace, provider, model);
internal/storage/sqlite/model_stats.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"devorch/internal/id"
)

type ModelStats struct {
	ID        string
	Workspace string
	Provider  string
	Model     string

	Samples      int
	AvgScore     float64
	AvgLatencyMs float64
	FailRate     float64

	UpdatedAt time.Time
}

type ModelStatsStore struct {
	DB *sql.DB
}

func (s *ModelStatsStore) Upsert(ctx context.Context, ms ModelStats) error {
	// SQLite upsert
	_, err := s.DB.ExecContext(ctx, `
	  INSERT INTO model_stats (
	    id, workspace, provider, model,
	    samples, avg_score, avg_latency_ms, fail_rate, updated_at
	  ) VALUES (?,?,?,?,?,?,?,?,?)
	  ON CONFLICT(workspace, provider, model)
	  DO UPDATE SET
	    samples=excluded.samples,
	    avg_score=excluded.avg_score,
	    avg_latency_ms=excluded.avg_latency_ms,
	    fail_rate=excluded.fail_rate,
	    updated_at=excluded.updated_at
	`, ms.ID, ms.Workspace, ms.Provider, ms.Model,
		ms.Samples, ms.AvgScore, ms.AvgLatencyMs, ms.FailRate, ms.UpdatedAt.UnixMilli(),
	)
	return err
}

func (s *ModelStatsStore) Get(ctx context.Context, workspace, provider, model string) (*ModelStats, error) {
	row := s.DB.QueryRowContext(ctx, `
	  SELECT id, workspace, provider, model,
	         samples, avg_score, avg_latency_ms, fail_rate, updated_at
	  FROM model_stats
	  WHERE workspace=? AND provider=? AND model=?
	  LIMIT 1
	`, workspace, provider, model)

	var ms ModelStats
	var updatedMs int64
	if err := row.Scan(&ms.ID, &ms.Workspace, &ms.Provider, &ms.Model,
		&ms.Samples, &ms.AvgScore, &ms.AvgLatencyMs, &ms.FailRate, &updatedMs); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	ms.UpdatedAt = time.UnixMilli(updatedMs)
	return &ms, nil
}

func NewModelStats(workspace, provider, model string) ModelStats {
	return ModelStats{
		ID:        id.NewULID(),
		Workspace: workspace,
		Provider:  provider,
		Model:     model,
		UpdatedAt: time.Now(),
	}
}
internal/router/learning.go
코드 복사
Go
package router

import (
	"context"
	"math"
	"time"

	"devorch/internal/storage/sqlite"
)

type ModelStatsReader interface {
	Get(ctx context.Context, workspace, provider, model string) (*sqlite.ModelStats, error)
}

type LearningScorer struct {
	Stats ModelStatsReader
}

// 라우팅 점수에 “실제 운영 품질”을 넣는다.
// baseScore(기존 latency/cost/affinity) + learnedBonus
// learnedBonus = f(avg_score, fail_rate, avg_latency_ms)
func (l *LearningScorer) LearnedBonus(ctx context.Context, workspace, provider, model string) float64 {
	if l == nil || l.Stats == nil {
		return 0
	}
	ms, err := l.Stats.Get(ctx, workspace, provider, model)
	if err != nil || ms == nil || ms.Samples < 5 {
		return 0 // 샘플 적으면 영향 최소화
	}

	// avg_score(0~1) → [-0.10, +0.20]
	scoreBonus := (ms.AvgScore - 0.5) * 0.4

	// fail_rate(0~1) → [-0.25, 0]
	failPenalty := -0.25 * clamp01(ms.FailRate)

	// latency penalty: 0~30s는 완만, 그 이상은 가파르게
	lat := ms.AvgLatencyMs
	latPenalty := 0.0
	if lat > 3000 {
		latPenalty = -0.05 * math.Log1p((lat-3000)/3000)
	}

	return scoreBonus + failPenalty + latPenalty
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// Recorder가 학습값 업데이트할 때 쓰는 헬퍼
type ModelStatsWriter interface {
	Get(ctx context.Context, workspace, provider, model string) (*sqlite.ModelStats, error)
	Upsert(ctx context.Context, ms sqlite.ModelStats) error