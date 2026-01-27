좋습니다. 30단계 풀코드는 “멀티OS 환경 fingerprint + hwTier 계산 + OkAON 자동 기록”을 완결합니다.

> ✅ 30단계 결과



실행 시점(daemon 부팅)에서 OS/ARCH/CPU/RAM/GPU(가능하면) 수집

EnvFingerprint(sha256) 생성

HwTier(low/mid/high/ultra) 산정

background task enqueue 시 OkAON(ok_aon_task_runs)에 os/arch/hw_tier/env_fingerprint 자동 저장

task 완료 시에도 동일 fingerprint 유지



---

A) 30단계 변경 파일 목록

신규

platform/detect/profile.go

platform/detect/cpu_mem.go

platform/detect/gpu_darwin.go

platform/detect/gpu_windows.go

platform/detect/gpu_linux.go

platform/detect/env_fingerprint.go


SQL 신규

internal/storage/sqlite/migrations/0005_env_fingerprint.sql


수정

internal/okAON/sqlite/task_runs.go (env_fingerprint 필드/쿼리 반영)

internal/background/manager.go (deps에 EnvFingerprint 추가, InsertQueued/MarkFinished 기록)

internal/app/app.go (platform.detect.Profile()로 deps 채움)



---

1) SQL 마이그레이션: env_fingerprint 컬럼 추가

1-1) internal/storage/sqlite/migrations/0005_env_fingerprint.sql (신규)

-- 0005_env_fingerprint.sql
BEGIN;

ALTER TABLE ok_aon_task_runs
ADD COLUMN env_fingerprint TEXT;

CREATE INDEX IF NOT EXISTS idx_ok_aon_task_runs_env_fp
  ON ok_aon_task_runs(env_fingerprint);

COMMIT;


---

2) 플랫폼 프로파일: OS/ARCH/CPU/RAM/GPU + HwTier + Fingerprint

2-1) platform/detect/profile.go (신규)

package detect

import (
	"runtime"
)

type HWProfile struct {
	OS   string
	Arch string

	CPUCores int
	MemBytes uint64

	GPUName string // best-effort
	GPUMemBytes uint64 // best-effort

	HwTier string // low/mid/high/ultra
	EnvFingerprint string
}

func Profile() HWProfile {
	p := HWProfile{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	c, m := cpuAndMem()
	p.CPUCores = c
	p.MemBytes = m

	p.GPUName, p.GPUMemBytes = gpuInfo()

	p.HwTier = classifyTier(p)
	p.EnvFingerprint = makeEnvFingerprint(p)

	return p
}

func classifyTier(p HWProfile) string {
	// 매우 단순하지만 “로컬에서 반복 측정되는 기준”으로 일관성 있게 동작하도록 설계
	// - CPU 코어 수 + 메모리 용량 중심
	// - GPU가 잡히면 ultra로 승급 가능 (best-effort)

	memGB := float64(p.MemBytes) / (1024 * 1024 * 1024)
	cores := p.CPUCores

	// GPU가 명확히 있고 메모리가 높으면 ultra
	if p.GPUName != "" && memGB >= 32 && cores >= 12 {
		return "ultra"
	}

	switch {
	case memGB < 8 || cores <= 4:
		return "low"
	case memGB < 16 || cores <= 6:
		return "mid"
	case memGB < 32 || cores <= 10:
		return "high"
	default:
		return "ultra"
	}
}

2-2) platform/detect/cpu_mem.go (신규)

package detect

import (
	"os"
	"runtime"
)

// cpuAndMem: 표준 라이브러리 기반 best-effort
// - CPU 코어: runtime.NumCPU()
// - 메모리: OS별로 더 정밀하게 하고 싶으면 후속 단계에서 확장 가능
//   (30단계는 “일관성 있는 fingerprint + tier”가 핵심이라서, 실패 시 0 허용)
func cpuAndMem() (cores int, memBytes uint64) {
	cores = runtime.NumCPU()

	// Linux: /proc/meminfo
	if runtime.GOOS == "linux" {
		if b, ok := meminfoLinux(); ok {
			memBytes = b
			return
		}
	}

	// macOS: sysctl (간단 구현은 gpu_darwin.go에서 같이)
	if runtime.GOOS == "darwin" {
		if b, ok := meminfoDarwin(); ok {
			memBytes = b
			return
		}
	}

	// Windows: wmic (fallback)
	if runtime.GOOS == "windows" {
		if b, ok := meminfoWindows(); ok {
			memBytes = b
			return
		}
	}

	// 최후 fallback: env로 주입 가능(후속 단계). 여기선 0 허용.
	_ = os.Getenv("DEVORCH_MEM_BYTES")
	return
}

2-3) platform/detect/env_fingerprint.go (신규)

package detect

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func makeEnvFingerprint(p HWProfile) string {
	// “환경”을 대표하는 값만 포함 (프로젝트/세션/프롬프트와 분리)
	// - tier, OS/arch, cores, mem, gpuName 정도를 포함
	s := fmt.Sprintf("os=%s|arch=%s|cores=%d|mem=%d|gpu=%s|gpumem=%d|tier=%s",
		p.OS, p.Arch, p.CPUCores, p.MemBytes, p.GPUName, p.GPUMemBytes, p.HwTier,
	)
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}


---

3) GPU best-effort (OS별 build tags)

3-1) platform/detect/gpu_darwin.go (신규)

//go:build darwin

package detect

import (
	"bytes"
	"os/exec"
	"strings"
)

// macOS GPU: system_profiler best-effort (느릴 수 있으나 30단계는 초기 1회만 사용)
// 더 가볍게 하려면 후속 단계에서 IOKit로 교체
func gpuInfo() (name string, memBytes uint64) {
	out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
	if err != nil {
		return "", 0
	}
	s := string(out)

	// 매우 단순 파서: "Chipset Model:" 또는 "Model:" 라인
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Chipset Model:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Chipset Model:")), 0
		}
		if strings.HasPrefix(line, "Model:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Model:")), 0
		}
	}
	return "", 0
}

// macOS meminfo: sysctl hw.memsize
func meminfoDarwin() (uint64, bool) {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, false
	}
	out = bytes.TrimSpace(out)
	var v uint64
	_, err = fmt.Sscanf(string(out), "%d", &v)
	if err != nil {
		return 0, false
	}
	return v, true
}

import "fmt"

3-2) platform/detect/gpu_windows.go (신규)

//go:build windows

package detect

import (
	"bytes"
	"os/exec"
	"strings"
)

// Windows GPU: wmic best-effort
func gpuInfo() (name string, memBytes uint64) {
	// Name, AdapterRAM을 얻고 싶지만 환경에 따라 wmic 미존재 가능
	out, err := exec.Command("wmic", "path", "win32_VideoController", "get", "Name,AdapterRAM", "/format:list").Output()
	if err != nil {
		return "", 0
	}
	s := string(out)

	var gpuName string
	var ram uint64

	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Name=") && gpuName == "" {
			gpuName = strings.TrimPrefix(line, "Name=")
		}
		if strings.HasPrefix(line, "AdapterRAM=") && ram == 0 {
			var v uint64
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "AdapterRAM="), "%d", &v)
			ram = v
		}
		if gpuName != "" && ram != 0 {
			break
		}
	}

	return gpuName, ram
}

func meminfoWindows() (uint64, bool) {
	// TotalPhysicalMemory
	out, err := exec.Command("wmic", "computersystem", "get", "TotalPhysicalMemory", "/value").Output()
	if err != nil {
		return 0, false
	}
	out = bytes.TrimSpace(out)
	s := string(out)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TotalPhysicalMemory=") {
			var v uint64
			_, err := fmt.Sscanf(strings.TrimPrefix(line, "TotalPhysicalMemory="), "%d", &v)
			return v, err == nil
		}
	}
	return 0, false
}

import "fmt"

3-3) platform/detect/gpu_linux.go (신규)

//go:build linux

package detect

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// Linux GPU: lspci best-effort (없을 수 있음)
func gpuInfo() (name string, memBytes uint64) {
	out, err := exec.Command("sh", "-lc", "command -v lspci >/dev/null 2>&1 && lspci | grep -i 'vga\\|3d\\|2d' | head -n 1 || true").Output()
	if err != nil {
		return "", 0
	}
	s := strings.TrimSpace(string(out))
	return s, 0
}

func meminfoLinux() (uint64, bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		// MemTotal:       16330856 kB
		if strings.HasPrefix(line, "MemTotal:") {
			var kb uint64
			_, err := fmt.Sscanf(line, "MemTotal:%d kB", &kb)
			if err != nil {
				// 일부 포맷 변형 대비
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					_, _ = fmt.Sscanf(fields[1], "%d", &kb)
				}
			}
			if kb == 0 {
				return 0, false
			}
			return kb * 1024, true
		}
	}
	_ = bytes.NewBuffer(nil)
	return 0, false
}

import "fmt"


---

4) OkAON TaskRun: env_fingerprint 반영

4-1) internal/okAON/sqlite/task_runs.go (수정본)

> 아래는 29단계 파일에서 추가/변경된 부분까지 포함한 “완성본” 입니다.



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

	OS      sql.NullString
	Arch    sql.NullString
	HwTier  sql.NullString
	EnvFingerprint sql.NullString
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
  provider, model, status, os, arch, hw_tier, env_fingerprint
) VALUES (
  ?, ?, ?, ?, ?, ?,
  ?, ?, ?, ?, ?, ?, ?
)`,
		tr.ID, tr.CreatedAtUnix,
		tr.Category, tr.SubagentType, tr.Description, tr.PromptHash,
		tr.Provider, tr.Model, tr.Status,
		tr.OS, tr.Arch, tr.HwTier, tr.EnvFingerprint,
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
    reward=?,
    env_fingerprint=COALESCE(env_fingerprint, ?)
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
		tr.EnvFingerprint,
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
  os, arch, hw_tier, env_fingerprint
FROM ok_aon_task_runs
WHERE id=?`, id)

	var tr TaskRun
	err := row.Scan(
		&tr.ID, &tr.CreatedAtUnix, &tr.FinishedAtUnix,
		&tr.Category, &tr.SubagentType, &tr.Description, &tr.PromptHash,
		&tr.Provider, &tr.Model, &tr.Status, &tr.ErrorMessage, &tr.OutputText,
		&tr.LatencyMs, &tr.TokensIn, &tr.TokensOut,
		&tr.QualityScore, &tr.Reward,
		&tr.OS, &tr.Arch, &tr.HwTier, &tr.EnvFingerprint,
	)
	return tr, err
}


---

5) Background Manager: deps에 EnvFingerprint 추가 + OkAON 자동 기록

5-1) internal/background/manager.go (30단계 수정본)

> 29단계 manager.go에 EnvFingerprint 추가/반영한 완성본입니다.
(파일 전체를 아래로 교체하세요.)



package background

import (
	"context"
	"crypto/sha256"
	"database/sql"
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

	OS     string
	Arch   string
	HwTier string
	EnvFingerprint string
}

type Manager struct {
	providers *provider.Registry
	ok        *oksql.TaskRunStore
	learner   learning.Learner
	exec      ExecuteFunc

	store *Store
	pool  *WorkerPool

	cancelFn map[string]context.CancelFunc

	os, arch, hwTier, envFP string
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
		envFP:  d.EnvFingerprint,
	}

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

	// ✅ OkAON queued 저장 (30단계: env_fp까지 자동 기록)
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
			EnvFingerprint: nullStr(m.envFP),
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
	if cf, ok := m.cancelFn[id]; ok && cf != nil {
		cf()
	}
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

	p, ok := m.providers.Get(t.Provider)
	if !ok {
		return m.finish(id, start, "", 0, 0, errors.New("provider_not_registered"))
	}

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

	reward := 0.0
	if success {
		reward = 1.0
	}

	_ = m.store.Update(id, func(t *Task) error {
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

	// ✅ OkAON finish 기록 (30단계: env_fp 유지 기록)
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
			EnvFingerprint: nullStr(m.envFP),
		}
		if tt != nil && tt.QualityScore != nil {
			tr.QualityScore = sqlNullFloat64(*tt.QualityScore)
		}
		_ = m.ok.MarkFinished(context.Background(), tr)
	}

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


---

6) App: platform.detect.Profile()을 background deps에 주입

6-1) internal/app/app.go (30단계 수정본)

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
	"devorch/platform/detect"
)

type App struct {
	Config *config.Store

	Providers *provider.Registry
	Router    *router.Router

	OkTaskRuns *oksql.TaskRunStore

	Learner     learning.Learner
	Background  *background.Manager
	Delegate    *delegate.Executor
}

func NewApp(reg *provider.Registry, okDB oksql.TaskRunStore, learner learning.Learner) *App {
	cfg, err := config.LoadMerged(config.LoadOptions{})
	if err != nil {
		cfg = config.DefaultConfig()
	}
	cs := config.NewStore(cfg)

	a := &App{
		Config:     cs,
		Providers:  reg,
		OkTaskRuns: &okDB,
		Learner:    learner,
	}

	// Router는 기존 단계 구현이 있으면 그대로 주입
	// a.Router = router.NewRouter(...)

	// ✅ 30단계: 멀티OS 프로파일 수집 + fingerprint + tier
	prof := detect.Profile()

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

		OS:     prof.OS,
		Arch:   prof.Arch,
		HwTier: prof.HwTier,
		EnvFingerprint: prof.EnvFingerprint,
	})

	dr := delegate.NewDelegateRouter(delegate.RouterDeps{
		Models: nil, // 28단계 modelresolver 연결부가 있으면 거기 객체를 주입
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


---

7) 30단계 동작 확인 체크

1. daemon 실행 후, background enqueue 한 번


2. sqlite에서 기록 확인:



SELECT id, provider, model, os, arch, hw_tier, env_fingerprint
FROM ok_aon_task_runs
ORDER BY created_at_unix DESC
LIMIT 5;

os/arch/hw_tier/env_fingerprint 값이 채워져 있으면 ✅



---

다음 31단계 예고(바로 연결됨)

30단계로 “환경 기반 최적화의 축”이 생겼습니다.
31단계에서는 bench/quality_eval.go를 붙여서 quality_score를 실측으로 만들고, reward를 success(1/0)에서 quality/latency/cost 반영 실수값으로 업그레이드합니다.

원하면 “네 31단계 풀코드 작성하세요”라고만 보내주세요.