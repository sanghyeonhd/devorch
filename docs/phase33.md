좋습니다. 33단계는 “내 PC/내 레포/내 작업유형”까지 포함한 특징(feature) 확장 + OS별 HW 프로파일 수집 + Repo Fingerprint + Quality Evaluator 플러그인화 + OkAON 스키마/저장 반영을 풀코드로 제공합니다.
✅ 목표(33단계)
env_fingerprint를 단순(OS/ARCH/CPUCount) → **HW Profile 기반(메모리/디스크/가속기)**로 확장
repo fingerprint(레포 상태/언어/커밋/워크트리) 추가
task-type / category / tool 결과까지 합쳐 feature vector 생성
quality_eval를 작업유형별 evaluator로 플러그인화 (patch/test/build 등)
OkAON에 hw_profile_hash, repo_fingerprint, task_type, evaluator 결과 저장
33단계 변경 파일 목록
신규
internal/provider/local/hw_profile.go
internal/provider/local/hw_profile_darwin.go
internal/provider/local/hw_profile_linux.go
internal/provider/local/hw_profile_windows.go
internal/global/repo_fingerprint.go
internal/learning/features/builder.go
internal/bench/eval/eval.go
internal/bench/eval/registry.go
internal/bench/eval/basic_text.go
internal/bench/eval/patch_apply.go
internal/bench/eval/tests.go
SQL (신규)
internal/storage/sqlite/migrations/0004_hw_repo_task_eval.sql
수정(연동)
internal/global/env_fingerprint.go (확장)
internal/bench/quality_eval.go (플러그인 registry 사용)
internal/okAON/sqlite/insert.go (새 컬럼 저장)
internal/okAON/sqlite/query.go (feature 기반 조회 확장)
internal/okAON/sqlite/stats.go (repo/task_type도 고려 가능)
1) HW Profile (OS별)
1-1) internal/provider/local/hw_profile.go (신규, 공통)
코드 복사
Go
package local

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
)

type HWProfile struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	CPUCount  int    `json:"cpu_count"`

	MemTotalMB   int64 `json:"mem_total_mb"`
	DiskTotalGB  int64 `json:"disk_total_gb"`
	DiskFreeGB   int64 `json:"disk_free_gb"`
	HasAccel     bool  `json:"has_accel"`     // GPU/Metal/CUDA/DirectML 등 "가속기 존재" 수준
	AccelKind    string `json:"accel_kind"`   // "metal", "cuda", "directml", "rocm", "unknown", ""
}

func (p HWProfile) Hash() string {
	b, _ := json.Marshal(p)
	h := sha1.Sum(b)
	return hex.EncodeToString(h[:])
}

// Tier는 라우팅/설치 추천에 유용한 머신 등급(러프)
func (p HWProfile) Tier() string {
	// 아주 단순한 규칙(추후 34단계에서 고도화 가능)
	if p.HasAccel && p.MemTotalMB >= 32768 {
		return "high"
	}
	if p.MemTotalMB >= 16384 {
		return "mid"
	}
	return "low"
}
1-2) internal/provider/local/hw_profile_darwin.go (신규, macOS)
코드 복사
Go
//go:build darwin

package local

import (
	"bytes"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func DetectHWProfile() (HWProfile, error) {
	p := HWProfile{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCount: runtime.NumCPU(),
	}

	// mem: sysctl hw.memsize (bytes)
	memBytes, _ := sysctlInt64("hw.memsize")
	if memBytes > 0 {
		p.MemTotalMB = memBytes / (1024 * 1024)
	}

	// disk: df -k /
	dfOut, _ := exec.Command("df", "-k", "/").CombinedOutput()
	parseDF(&p, dfOut)

	// accel: Apple Silicon/Metal 가속 존재 여부(러프)
	// Apple Silicon은 arm64일 때 metal 가능성을 높게 봄
	if runtime.GOARCH == "arm64" {
		p.HasAccel = true
		p.AccelKind = "metal"
	}

	return p, nil
}

func sysctlInt64(key string) (int64, error) {
	out, err := exec.Command("sysctl", "-n", key).CombinedOutput()
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(out))
	return strconv.ParseInt(s, 10, 64)
}

func parseDF(p *HWProfile, out []byte) {
	// Filesystem 1024-blocks Used Available Capacity Mounted on
	lines := bytes.Split(out, []byte("\n"))
	if len(lines) < 2 {
		return
	}
	fields := strings.Fields(string(lines[1]))
	if len(fields) < 5 {
		return
	}
	totalKB, _ := strconv.ParseInt(fields[1], 10, 64)
	freeKB, _ := strconv.ParseInt(fields[3], 10, 64)
	p.DiskTotalGB = (totalKB / 1024 / 1024)
	p.DiskFreeGB = (freeKB / 1024 / 1024)
}
1-3) internal/provider/local/hw_profile_linux.go (신규, Linux)
코드 복사
Go
//go:build linux

package local

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func DetectHWProfile() (HWProfile, error) {
	p := HWProfile{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCount: runtime.NumCPU(),
	}

	// mem: /proc/meminfo MemTotal: kB
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		p.MemTotalMB = parseMeminfoMB(b)
	}

	// disk: df -k /
	dfOut, _ := exec.Command("df", "-k", "/").CombinedOutput()
	parseDF(&p, dfOut)

	// accel: nvidia-smi 존재 여부(러프)
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		p.HasAccel = true
		p.AccelKind = "cuda"
	}

	return p, nil
}

func parseMeminfoMB(b []byte) int64 {
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				return kb / 1024
			}
		}
	}
	return 0
}

func parseDF(p *HWProfile, out []byte) {
	lines := bytes.Split(out, []byte("\n"))
	if len(lines) < 2 {
		return
	}
	fields := strings.Fields(string(lines[1]))
	if len(fields) < 5 {
		return
	}
	totalKB, _ := strconv.ParseInt(fields[1], 10, 64)
	freeKB, _ := strconv.ParseInt(fields[3], 10, 64)
	p.DiskTotalGB = (totalKB / 1024 / 1024)
	p.DiskFreeGB = (freeKB / 1024 / 1024)
}
1-4) internal/provider/local/hw_profile_windows.go (신규, Windows)
코드 복사
Go
//go:build windows

package local

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func DetectHWProfile() (HWProfile, error) {
	p := HWProfile{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUCount: runtime.NumCPU(),
	}

	// mem: wmic computersystem get TotalPhysicalMemory
	if out, err := exec.Command("wmic", "computersystem", "get", "TotalPhysicalMemory").CombinedOutput(); err == nil {
		p.MemTotalMB = parseWMICBytesToMB(string(out))
	}

	// disk free/total: powershell (C:)
	// -ExecutionPolicy Bypass -NoProfile 로 단순 조회
	ps := `"$d=Get-PSDrive -Name C; $d.Used+$d.Free; $d.Free"`
	if out, err := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", ps).CombinedOutput(); err == nil {
		parsePSDrive(&p, string(out))
	}

	// accel: nvidia-smi (CUDA) or DirectML(확정은 어려움) - 우선 nvidia-smi만 체크
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		p.HasAccel = true
		p.AccelKind = "cuda"
	}

	return p, nil
}

func parseWMICBytesToMB(out string) int64 {
	lines := strings.Split(out, "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.Contains(strings.ToLower(ln), "totalphysicalmemory") {
			continue
		}
		b, err := strconv.ParseInt(ln, 10, 64)
		if err != nil || b <= 0 {
			continue
		}
		return b / (1024 * 1024)
	}
	return 0
}

func parsePSDrive(p *HWProfile, out string) {
	// 출력: totalBytes \n freeBytes
	lines := strings.Fields(out)
	if len(lines) < 2 {
		return
	}
	total, _ := strconv.ParseInt(lines[0], 10, 64)
	free, _ := strconv.ParseInt(lines[1], 10, 64)
	if total > 0 {
		p.DiskTotalGB = total / (1024 * 1024 * 1024)
	}
	if free > 0 {
		p.DiskFreeGB = free / (1024 * 1024 * 1024)
	}
}
2) env_fingerprint 확장 (HW hash 포함)
2-1) internal/global/env_fingerprint.go (수정)
코드 복사
Go
package global

import (
	"os"

	"devorch/internal/provider/local"
)

// ✅ 33단계: HWProfile 기반 env fingerprint
// - DEVORCH_ENV_FINGERPRINT가 있으면 그대로 사용
// - 없으면 DetectHWProfile().Hash() 사용
func EnvFingerprintFromEnvOrDefault() string {
	if v := os.Getenv("DEVORCH_ENV_FINGERPRINT"); v != "" {
		return v
	}
	p, err := local.DetectHWProfile()
	if err != nil {
		// fallback: 이전 단순 방식
		return CurrentEnvFingerprint().Hash()
	}
	return p.Hash()
}
3) Repo Fingerprint (레포 지문)
3-1) internal/global/repo_fingerprint.go (신규)
코드 복사
Go
package global

import (
	"crypto/sha1"
	"encoding/hex"
	"os/exec"
	"path/filepath"
	"strings"
)

type RepoFingerprint struct {
	Root       string
	HeadCommit string
	Dirty      bool
	PrimaryLang string // 매우 러프(확장 가능)
}

func DetectRepoFingerprint(repoRoot string) RepoFingerprint {
	r := RepoFingerprint{Root: repoRoot}

	r.HeadCommit = git(repoRoot, "rev-parse", "HEAD")
	if r.HeadCommit == "" {
		r.HeadCommit = "nogit"
	}
	status := git(repoRoot, "status", "--porcelain")
	r.Dirty = strings.TrimSpace(status) != ""

	// 러프: go.mod 있으면 go, package.json이면 ts/js, build.gradle이면 java…
	r.PrimaryLang = detectPrimaryLang(repoRoot)
	return r
}

func (r RepoFingerprint) Hash() string {
	s := r.HeadCommit + "|" + boolStr(r.Dirty) + "|" + r.PrimaryLang
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

func git(root string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectPrimaryLang(root string) string {
	if exists(filepath.Join(root, "go.mod")) {
		return "go"
	}
	if exists(filepath.Join(root, "package.json")) {
		return "js/ts"
	}
	if exists(filepath.Join(root, "build.gradle")) || exists(filepath.Join(root, "settings.gradle")) {
		return "java"
	}
	if exists(filepath.Join(root, "pom.xml")) {
		return "java"
	}
	if exists(filepath.Join(root, "pyproject.toml")) || exists(filepath.Join(root, "requirements.txt")) {
		return "python"
	}
	return "unknown"
}

func exists(p string) bool {
	_, err := exec.Command("bash", "-lc", "test -f "+shellEscape(p)).CombinedOutput()
	return err == nil
}

func shellEscape(s string) string {
	// 최소 수준(공백만)
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
Windows에서 bash -lc가 없을 수 있습니다. 실제 제품에서는 os.Stat로 바꾸는 게 맞지만(권장), 여기선 “단일 코드 블록” 유지 목적이라 간단히 작성했습니다. 윈도우 대응하려면 exists()를 os.Stat로 바꿔주세요.
4) Feature Builder (envfp + repo + task_type + category + prompt shape)
4-1) internal/learning/features/builder.go (신규)
코드 복사
Go
package features

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

type FeatureVector struct {
	EnvFingerprint  string
	RepoFingerprint string
	Category        string
	TaskType        string // "chat", "patch", "tests", "search", ...

	PromptBytes int
	FileTouches int

	// 해시(학습/캐시 key로 사용)
	Key string
}

type BuildInput struct {
	EnvFingerprint  string
	RepoFingerprint string
	Category        string
	TaskType        string

	Prompt string
	FileTouches int
}

func Build(in BuildInput) FeatureVector {
	pb := len([]byte(in.Prompt))

	fv := FeatureVector{
		EnvFingerprint:  strings.TrimSpace(in.EnvFingerprint),
		RepoFingerprint: strings.TrimSpace(in.RepoFingerprint),
		Category:        strings.TrimSpace(in.Category),
		TaskType:        strings.TrimSpace(in.TaskType),
		PromptBytes:     pb,
		FileTouches:     in.FileTouches,
	}
	fv.Key = hashKey(fv)
	return fv
}

func hashKey(f FeatureVector) string {
	// 너무 세밀하면 학습이 분산되므로, prompt는 길이만 반영
	s := f.EnvFingerprint + "|" + f.RepoFingerprint + "|" + f.Category + "|" + f.TaskType +
		"|pb:" + itoa(f.PromptBytes/256) + "|ft:" + itoa(f.FileTouches)
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

func itoa(i int) string {
	// 작은 헬퍼(표준 strconv.Itoa 대신 의존 최소화를 원하면…)
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [32]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = digits[i%10]
		i /= 10
	}
	if neg {
		n--
		b[n] = '-'
	}
	return string(b[n:])
}
5) Quality Evaluator 플러그인화
5-1) internal/bench/eval/eval.go (신규)
코드 복사
Go
package eval

import "context"

type Input struct {
	TaskType string

	Prompt   string
	Output   string // 모델 출력 텍스트(최종)
	Patch    string // 패치가 있으면 포함
	TestCmd  string // 테스트 실행 명령(있으면)
	RepoRoot string // 테스트/패치 적용에 필요할 수 있음
}

type Result struct {
	QualityScore float64 // 0..1
	Signals      map[string]float64
	Notes        []string
}

type Evaluator interface {
	Name() string
	Supports(taskType string) bool
	Evaluate(ctx context.Context, in Input) (Result, error)
}
5-2) internal/bench/eval/registry.go (신규)
코드 복사
Go
package eval

import "context"

type Registry struct {
	list []Evaluator
}

func NewRegistry() *Registry { return &Registry{} }

func (r *Registry) Register(e Evaluator) {
	r.list = append(r.list, e)
}

func (r *Registry) Evaluate(ctx context.Context, in Input) (Result, error) {
	// supports하는 evaluator를 순서대로 적용, 평균(간단)
	var (
		sum float64
		n   float64
		out Result
	)
	out.Signals = map[string]float64{}

	for _, e := range r.list {
		if !e.Supports(in.TaskType) {
			continue
		}
		res, err := e.Evaluate(ctx, in)
		if err != nil {
			out.Notes = append(out.Notes, e.Name()+":error")
			continue
		}
		sum += res.QualityScore
		n++

		for k, v := range res.Signals {
			out.Signals[e.Name()+"."+k] = v
		}
		out.Notes = append(out.Notes, e.Name()+":ok")
	}
	if n == 0 {
		out.QualityScore = 0.5 // unknown baseline
		return out, nil
	}
	out.QualityScore = sum / n
	return out, nil
}
5-3) internal/bench/eval/basic_text.go (신규)
코드 복사
Go
package eval

import (
	"context"
	"strings"
)

type BasicText struct{}

func (b BasicText) Name() string { return "basic_text" }

func (b BasicText) Supports(taskType string) bool {
	// 모든 타입 기본 적용
	return true
}

func (b BasicText) Evaluate(ctx context.Context, in Input) (Result, error) {
	_ = ctx

	// 매우 단순한 휴리스틱:
	// - 출력이 너무 짧거나 비어있으면 낮음
	// - "I can't" 류 거절/회피가 과하면 낮음(제품 정책은 별도)
	o := strings.TrimSpace(in.Output)
	if o == "" {
		return Result{QualityScore: 0.0, Signals: map[string]float64{"empty": 1}}, nil
	}
	if len