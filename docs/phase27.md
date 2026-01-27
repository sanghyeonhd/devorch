아래는 27단계(“ModelResolver 실제 연결 + category→requirements→fallback 체인 + 사용자 override”) 풀코드입니다.
26단계의 defaultCandidates()가 “대충 후보 생성”이었는데, 27단계에서 oh-my-opencode식 3단계 모델 해석(override → fallback chain → default) 을 실제로 구현하고 서버 라우트(/v1/chat, /v1/chat/stream)가 ModelResolver 결과를 그대로 쓰도록 연결합니다.

> ✅ 27단계 결과



.devorch/devorch.jsonc + ~/.config/devorch/devorch.jsonc 로부터 categories/agents/providers 오버라이드 읽음 (JSONC 허용)

모델 해석:

1. 사용자 오버라이드(category 또는 agent)


2. fallback chain (providers 우선순위 + provider별 모델 네임스페이스)


3. system default



Provider registry에서 실제 사용 가능한 provider만 후보로 생성

서버 라우트에서 후보 생성은 ModelResolver.ResolveCandidates()로 통일



---

A) 27단계 신규/수정 파일 목록

신규

internal/modelresolver/types.go

internal/modelresolver/requirements.go

internal/modelresolver/fallback.go

internal/modelresolver/resolve.go

internal/modelresolver/policy.go

internal/config/types.go

internal/config/jsonc.go

internal/config/load.go

internal/config/store.go


수정

internal/app/app.go (ConfigStore/ModelResolver 연결)

internal/server/routes/chat.go (defaultCandidates 교체)

internal/server/routes/chat_stream.go (defaultCandidates 교체)



---

1) Config (JSONC + 머지 + Store)

1-1) internal/config/types.go (신규)

package config

// DevOrchConfig는 27단계에서 “모델 선택”에 필요한 최소 필드만 포함합니다.
// (추후: routing, quotas, oauth, performance 등 확장)
type DevOrchConfig struct {
	SystemDefaultModel string `json:"systemDefaultModel,omitempty"`

	Providers ProvidersConfig `json:"providers,omitempty"`

	// category별 모델/프로바이더 우선순위 오버라이드
	Categories map[string]CategoryConfig `json:"categories,omitempty"`

	// agent별 모델 오버라이드 (27단계는 category 중심으로 사용)
	Agents map[string]AgentConfig `json:"agents,omitempty"`
}

type ProvidersConfig struct {
	// 전역 provider 우선순위(없으면 내장 기본값)
	Priority []string `json:"priority,omitempty"`

	// provider별 접두사/별칭 (예: openai -> openai/, anthropic -> anthropic/)
	// 없으면 provider name을 그대로 prefix로 사용.
	Namespaces map[string]string `json:"namespaces,omitempty"`
}

type CategoryConfig struct {
	Model     string   `json:"model,omitempty"`      // 사용자 강제 지정(1단계 override)
	Providers []string `json:"providers,omitempty"`  // category별 우선순위 fallback chain
}

type AgentConfig struct {
	Model     string   `json:"model,omitempty"`
	Providers []string `json:"providers,omitempty"`
}

1-2) internal/config/jsonc.go (신규)

package config

import (
	"bytes"
	"errors"
)

// 매우 단순한 JSONC 제거기입니다.
// - // 라인 주석 제거
// - /* */ 블록 주석 제거
// (문자열 내부 주석은 완벽 처리하지 않음: 27단계 목표는 “구동 가능한 최소”)
func StripJSONC(in []byte) ([]byte, error) {
	if len(in) == 0 {
		return in, nil
	}

	// 1) 블록 주석 제거
	out := make([]byte, 0, len(in))
	i := 0
	for i < len(in) {
		// /* ... */
		if i+1 < len(in) && in[i] == '/' && in[i+1] == '*' {
			i += 2
			for i+1 < len(in) && !(in[i] == '*' && in[i+1] == '/') {
				i++
			}
			if i+1 >= len(in) {
				return nil, errors.New("unterminated block comment")
			}
			i += 2
			continue
		}
		out = append(out, in[i])
		i++
	}

	// 2) 라인 주석 제거 (// ...)
	in2 := out
	out = make([]byte, 0, len(in2))
	for {
		idx := bytes.Index(in2, []byte("//"))
		if idx < 0 {
			out = append(out, in2...)
			break
		}
		// idx 이전까지 copy
		out = append(out, in2[:idx]...)

		// 줄 끝까지 skip
		in2 = in2[idx+2:]
		nl := bytes.IndexByte(in2, '\n')
		if nl < 0 {
			// 파일 끝
			break
		}
		in2 = in2[nl:] // '\n' 포함 유지
	}

	return out, nil
}

1-3) internal/config/load.go (신규)

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type LoadOptions struct {
	// 프로젝트 루트(없으면 현재 디렉토리 기준 .devorch 탐색)
	ProjectRoot string
	// 사용자 홈 기반 전역 config 경로 override (테스트용)
	GlobalConfigPath string
}

// 27단계: 다음 순서로 머지(뒤가 우선)
// 1) defaults
// 2) global (~/.config/devorch/devorch.jsonc)
// 3) project (.devorch/devorch.jsonc)
func LoadMerged(opts LoadOptions) (DevOrchConfig, error) {
	cfg := DefaultConfig()

	// global
	gpath := opts.GlobalConfigPath
	if gpath == "" {
		home, _ := os.UserHomeDir()
		gpath = filepath.Join(home, ".config", "devorch", "devorch.jsonc")
	}
	if c2, ok, err := loadOne(gpath); err != nil {
		return DevOrchConfig{}, err
	} else if ok {
		cfg = Merge(cfg, c2)
	}

	// project
	proot := opts.ProjectRoot
	if proot == "" {
		proot, _ = os.Getwd()
	}
	ppath := filepath.Join(proot, ".devorch", "devorch.jsonc")
	if c3, ok, err := loadOne(ppath); err != nil {
		return DevOrchConfig{}, err
	} else if ok {
		cfg = Merge(cfg, c3)
	}

	return cfg, nil
}

func loadOne(path string) (DevOrchConfig, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DevOrchConfig{}, false, nil
		}
		return DevOrchConfig{}, false, err
	}
	b2, err := StripJSONC(b)
	if err != nil {
		return DevOrchConfig{}, false, err
	}

	var cfg DevOrchConfig
	if err := json.Unmarshal(b2, &cfg); err != nil {
		return DevOrchConfig{}, false, err
	}
	return cfg, true, nil
}

func DefaultConfig() DevOrchConfig {
	return DevOrchConfig{
		SystemDefaultModel: "claude-sonnet",
		Providers: ProvidersConfig{
			Priority: []string{"anthropic", "openai", "google", "opencode", "copilot", "openrouter", "ollama"},
			Namespaces: map[string]string{
				"anthropic": "anthropic/",
				"openai":    "openai/",
				"google":    "google/",
				"opencode":  "opencode/",
				"copilot":   "github-copilot/",
				"openrouter":"openrouter/",
				"ollama":    "ollama/",
			},
		},
		Categories: map[string]CategoryConfig{
			"quick":               {Model: "claude-haiku", Providers: []string{"anthropic", "opencode", "openai", "google"}},
			"ultrabrain":          {Model: "gpt-4o-mini",  Providers: []string{"openai", "anthropic", "google"}},
			"visual-engineering":  {Model: "gemini-3-flash", Providers: []string{"google", "openai", "anthropic"}},
			"unspecified-high":    {Model: "claude-opus", Providers: []string{"anthropic", "openai"}},
			"unspecified-low":     {Model: "claude-sonnet", Providers: []string{"anthropic", "openai"}},
		},
		Agents: map[string]AgentConfig{},
	}
}

// Merge: b가 a를 덮어씀(뒤가 우선)
func Merge(a, b DevOrchConfig) DevOrchConfig {
	out := a

	if b.SystemDefaultModel != "" {
		out.SystemDefaultModel = b.SystemDefaultModel
	}
	// providers
	if len(b.Providers.Priority) > 0 {
		out.Providers.Priority = append([]string{}, b.Providers.Priority...)
	}
	if b.Providers.Namespaces != nil {
		if out.Providers.Namespaces == nil {
			out.Providers.Namespaces = map[string]string{}
		}
		for k, v := range b.Providers.Namespaces {
			if v != "" {
				out.Providers.Namespaces[k] = v
			}
		}
	}

	// categories
	if b.Categories != nil {
		if out.Categories == nil {
			out.Categories = map[string]CategoryConfig{}
		}
		for k, v := range b.Categories {
			prev := out.Categories[k]
			if v.Model != "" {
				prev.Model = v.Model
			}
			if len(v.Providers) > 0 {
				prev.Providers = append([]string{}, v.Providers...)
			}
			out.Categories[k] = prev
		}
	}

	// agents
	if b.Agents != nil {
		if out.Agents == nil {
			out.Agents = map[string]AgentConfig{}
		}
		for k, v := range b.Agents {
			prev := out.Agents[k]
			if v.Model != "" {
				prev.Model = v.Model
			}
			if len(v.Providers) > 0 {
				prev.Providers = append([]string{}, v.Providers...)
			}
			out.Agents[k] = prev
		}
	}

	return out
}

1-4) internal/config/store.go (신규)

package config

import (
	"sync"
)

type Store struct {
	mu  sync.RWMutex
	cfg DevOrchConfig
}

func NewStore(initial DevOrchConfig) *Store {
	return &Store{cfg: initial}
}

func (s *Store) Get() DevOrchConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Store) Set(cfg DevOrchConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}


---

2) ModelResolver (override → fallback → default)

2-1) internal/modelresolver/types.go (신규)

package modelresolver

type Requirement struct {
	// base model name without provider prefix (ex: "claude-sonnet", "gpt-4o-mini")
	BaseModel string

	// provider fallback chain (ex: ["anthropic","openai","google"])
	Providers []string
}

type Resolution struct {
	Provider string
	Model    string // resolved model string with namespace (ex: "anthropic/claude-sonnet")
	Source   string // "override" | "fallback" | "default"
}

2-2) internal/modelresolver/requirements.go (신규)

package modelresolver

import "devorch/internal/config"

func RequirementForCategory(cfg config.DevOrchConfig, category string) Requirement {
	// category 없으면 unspecified-low로 다운그레이드
	cc, ok := cfg.Categories[category]
	if !ok {
		if cc2, ok2 := cfg.Categories["unspecified-low"]; ok2 {
			cc = cc2
		} else {
			cc = config.CategoryConfig{Model: cfg.SystemDefaultModel, Providers: cfg.Providers.Priority}
		}
	}

	base := cc.Model
	if base == "" {
		base = cfg.SystemDefaultModel
	}

	prov := cc.Providers
	if len(prov) == 0 {
		prov = cfg.Providers.Priority
	}

	return Requirement{
		BaseModel: base,
		Providers: append([]string{}, prov...),
	}
}

// 27단계에서는 agent도 제공(필요 시 category 대신 사용 가능)
func RequirementForAgent(cfg config.DevOrchConfig, agent string) Requirement {
	ac, ok := cfg.Agents[agent]
	if !ok {
		return Requirement{BaseModel: cfg.SystemDefaultModel, Providers: cfg.Providers.Priority}
	}

	base := ac.Model
	if base == "" {
		base = cfg.SystemDefaultModel
	}
	prov := ac.Providers
	if len(prov) == 0 {
		prov = cfg.Providers.Priority
	}

	return Requirement{
		BaseModel: base,
		Providers: append([]string{}, prov...),
	}
}

2-3) internal/modelresolver/fallback.go (신규)

package modelresolver

import "devorch/internal/config"

// provider namespace를 적용해 최종 모델 문자열 생성
func NamespacedModel(cfg config.DevOrchConfig, provider, baseModel string) string {
	ns := provider
	if cfg.Providers.Namespaces != nil {
		if v, ok := cfg.Providers.Namespaces[provider]; ok && v != "" {
			ns = v
		} else {
			// 기본: provider + "/"
			ns = provider + "/"
		}
	} else {
		ns = provider + "/"
	}
	return ns + baseModel
}

2-4) internal/modelresolver/policy.go (신규)

package modelresolver

import (
	"strings"
)

// 27단계 최소 정책:
// - provider/model 문자열이 이미 "xxx/yyy" 형태라면 그대로 사용 가능.
// - baseModel이 이미 "provider/model" 형태면 provider만 추출해서 사용 가능.
func ParseMaybeNamespacedModel(s string) (provider string, model string, ok bool) {
	if s == "" {
		return "", "", false
	}
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], s, true
}

2-5) internal/modelresolver/resolve.go (신규)

package modelresolver

import (
	"context"
	"errors"

	"devorch/internal/config"
	"devorch/internal/provider"
	"devorch/internal/router"
)

type Resolver struct {
	Config   *config.Store
	Registry *provider.Registry
}

func NewResolver(cs *config.Store, reg *provider.Registry) *Resolver {
	return &Resolver{Config: cs, Registry: reg}
}

// ResolveCandidates: category 기반으로 “실제 사용 가능한 provider”만 후보로 만듭니다.
// 1) override (category.model이 "openai/gpt-..."처럼 namespaced면 그대로)
// 2) fallback chain (providers 순회)
// 3) default (systemDefaultModel + global providers)
func (r *Resolver) ResolveCandidates(ctx context.Context, category string) ([]router.Candidate, error) {
	cfg := r.Config.Get()

	// 1) category requirement
	req := RequirementForCategory(cfg, category)

	// 1-A) baseModel이 namespaced면 provider 고정 후보로 1개 생성
	if p, full, ok := ParseMaybeNamespacedModel(req.BaseModel); ok {
		if r.providerAvailable(p) {
			return []router.Candidate{{Provider: p, Model: full}}, nil
		}
		// namespaced인데 provider가 없으면 fallback로 진행
	}

	// 2) fallback chain 후보 생성
	out := make([]router.Candidate, 0, len(req.Providers))
	for _, p := range req.Providers {
		if !r.providerAvailable(p) {
			continue
		}
		out = append(out, router.Candidate{
			Provider: p,
			Model:    NamespacedModel(cfg, p, req.BaseModel),
		})
	}

	if len(out) > 0 {
		return out, nil
	}

	// 3) default
	if cfg.SystemDefaultModel == "" {
		return nil, errors.New("no_system_default_model")
	}
	for _, p := range cfg.Providers.Priority {
		if !r.providerAvailable(p) {
			continue
		}
		out = append(out, router.Candidate{
			Provider: p,
			Model:    NamespacedModel(cfg, p, cfg.SystemDefaultModel),
		})
	}
	if len(out) == 0 {
		return nil, errors.New("no_provider_available")
	}
	return out, nil
}

func (r *Resolver) providerAvailable(name string) bool {
	if r.Registry == nil {
		return false
	}
	_, ok := r.Registry.Get(name)
	return ok
}


---

3) App에 ConfigStore + ModelResolver 주입

3-1) internal/app/app.go (수정, “최소 완성형” 제공)

> 26단계에서 app.NewApp(ok, reg) 형태였다고 가정하고, 27단계에 Config/Resolver를 붙입니다.



package app

import (
	"context"

	"devorch/internal/config"
	"devorch/internal/exec"
	"devorch/internal/modelresolver"
	"devorch/internal/okAON/sqlite"
	"devorch/internal/provider"
)

type App struct {
	Ok        sqlite.Store
	Providers *provider.Registry

	Config *config.Store
	Models *modelresolver.Resolver

	Runner *exec.Runner
}

func NewApp(ok sqlite.Store, reg *provider.Registry) *App {
	// 27단계: config 로드(실패 시 defaults로 구동)
	cfg, err := config.LoadMerged(config.LoadOptions{})
	if err != nil {
		cfg = config.DefaultConfig()
	}

	cs := config.NewStore(cfg)
	res := modelresolver.NewResolver(cs, reg)

	a := &App{
		Ok:        ok,
		Providers: reg,
		Config:    cs,
		Models:    res,
	}

	a.Runner = exec.NewRunner(exec.RunnerDeps{
		Ok:        ok,
		Providers: reg,
		Router:    nil, // 26단계/이전 단계 router 사용 중이면 주입
	})

	return a
}

func (a *App) Start(ctx context.Context) error {
	// TODO: config watcher, migrations, telemetry 시작 등
	return nil
}

func (a *App) Stop() {
	// TODO: 자원 정리
}


---

4) Server Route에서 ModelResolver 사용하도록 교체

4-1) internal/server/routes/chat.go (수정: defaultCandidates 교체)

아래 함수만 교체하면 됩니다. (나머지 핸들러 로직은 26단계 그대로)

// 기존 defaultCandidates(...) 함수 교체용
func defaultCandidates(a *app.App, category string) []router.Candidate {
	if a.Models == nil {
		// fallback: 기존 provider list 기반
		names := a.Providers.List()
		if len(names) == 0 {
			return nil
		}
		model := defaultModelByCategory(category)
		out := make([]router.Candidate, 0, len(names))
		for _, p := range names {
			out = append(out, router.Candidate{Provider: p, Model: p + "/" + model})
		}
		return out
	}

	cands, err := a.Models.ResolveCandidates(context.Background(), category)
	if err != nil {
		return nil
	}
	return cands
}

> ⚠️ 위 교체를 위해 chat.go 상단 import에 context가 필요합니다:



import (
  "context"
  ...
)

4-2) internal/server/routes/chat_stream.go (수정: defaultCandidates 그대로 재사용)

chat_stream.go는 26단계에서 defaultCandidates(a, req.Category)를 호출하므로, chat.go의 defaultCandidates가 교체되면 자동 반영됩니다.
따라서 추가 수정 없음(단, 패키지 내 동일 함수 공유 전제).


---

5) Provider Registry 최소 구현 확인 (필수)

> 27단계는 Registry.Get(name) / Registry.List()를 씁니다.
만약 Registry가 아직 없다면 아래 파일을 추가하세요.



5-1) internal/provider/registry.go (없으면 추가 / 있으면 스킵)

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

6) .devorch/devorch.jsonc 예시 (27단계 해석 동작 확인용)

> 파일 자체는 코드가 아니지만, 27단계가 제대로 동작하는지 확인하려면 예시가 필요합니다.



{
  "systemDefaultModel": "claude-sonnet",

  "providers": {
    "priority": ["openai", "anthropic", "google", "ollama"],
    "namespaces": {
      "openai": "openai/",
      "anthropic": "anthropic/",
      "google": "google/",
      "ollama": "ollama/"
    }
  },

  "categories": {
    "quick": {
      "model": "claude-haiku",
      "providers": ["anthropic", "openai", "ollama"]
    },
    "ultrabrain": {
      "model": "openai/gpt-4o-mini" // namespaced override -> provider 고정 1개 후보
    }
  }
}


---

27단계 검증 포인트(정상 기준)

1. .devorch/devorch.jsonc에 "ultrabrain": {"model": "openai/gpt-4o-mini"} 넣고
POST /v1/chat에 "category":"ultrabrain" 요청하면
→ 후보군이 openai 하나로 고정되어 실행되는지


2. "quick": {"model":"claude-haiku","providers":["anthropic","openai","ollama"]}일 때
→ Registry에 등록된 provider만 후보로 남는지


3. category가 없는 "category":"unknown"일 때
→ unspecified-low 또는 systemDefaultModel로 fallback 되는지




---

다음 28단계(추천): “Agent overrides + delegate_task / subagent_type까지 동일 해석”

27단계는 category 중심이므로, 다음 단계에서

subagent_type="oracle" 같은 호출도 동일한 3단계 해석 적용

hook/command에서 자동으로 category/agent를 태깅 까지 연결하면 “oh-my-opencode 수준”으로 완성됩니다.


원하면 28단계 풀코드 바로 이어서 작성할게요.