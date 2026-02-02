좋습니다. Step 19 = “승자(Experiment Winner)를 ‘영구 정책(Versioned Policy)’으로 승격/롤백하고, router/modelresolver가 항상 그 정책을 우선 적용” 입니다.

Step17/18이 “실험을 만들고 자동으로 굴리는 것”이라면

Step19는 “승자 결과를 운영 정책으로 고정(승격) → 라우팅/모델 선택에 즉시 반영 → 언제든 롤백”입니다.


아래는 Step19 풀코드(Go + SQL) 입니다. (누락 없이)


---

Step 19 소스트리 추가/변경

internal/
├─ policy/
│  ├─ types.go                  # policy 타입/스펙
│  ├─ store.go                  # DB 저장소
│  ├─ resolver.go               # effective policy 계산
│  ├─ promotion.go              # experiment winner → policy 승격
│  ├─ rollback.go               # 정책 롤백/승격/채널
│  └─ bootstrap.go              # app start wiring helper
│
├─ router/
│  ├─ policy_overlay.go         # (추가) router에 정책 오버레이 적용
│  └─ router.go                 # (수정) ResolvePolicyHook 호출
│
├─ modelresolver/
│  ├─ policy_overlay.go         # (추가) modelresolver에 정책 오버레이 적용
│  └─ resolve.go                # (수정) ResolvePolicyHook 호출
│
├─ server/routes/
│  └─ policy.go                 # 정책 조회/승격/롤백 API
│
├─ diagnostics/checks/
│  └─ policy_registry.go        # doctor 체크(정책/채널/무결성)
│
└─ storage/sqlite/migrations/
   └─ 0009_policy_registry.sql


---

1) DB 마이그레이션 (SQL)

internal/storage/sqlite/migrations/0009_policy_registry.sql

-- Step 19: Versioned Policy Registry + Promotion/Rollback Channels

CREATE TABLE IF NOT EXISTS policy_versions (
  id TEXT PRIMARY KEY,
  scope TEXT NOT NULL,                -- "global" | "workspace:<id>" | "project:<id>"
  domain TEXT NOT NULL,               -- "router" | "modelresolver" | "prompt"
  name TEXT NOT NULL,                 -- e.g. "default-routing", "ultrabrain-models", "system-prompt"

  version INTEGER NOT NULL,           -- monotonic per (scope,domain,name)
  status TEXT NOT NULL,               -- "draft" | "active" | "archived" | "rolled_back"
  channel TEXT NOT NULL,              -- "stable" | "beta" | "nightly"

  spec_json TEXT NOT NULL,            -- policy content
  source TEXT NOT NULL,               -- "manual" | "experiment:<id>" | "system"
  created_by TEXT,                    -- optional user id
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  activated_at DATETIME,
  archived_at DATETIME,

  UNIQUE(scope, domain, name, version)
);

CREATE INDEX IF NOT EXISTS idx_policy_active
  ON policy_versions(scope, domain, name, status, channel, version);

CREATE TABLE IF NOT EXISTS policy_events (
  id TEXT PRIMARY KEY,
  scope TEXT NOT NULL,
  domain TEXT NOT NULL,
  name TEXT NOT NULL,

  action TEXT NOT NULL,               -- "create" | "activate" | "archive" | "rollback" | "promote_from_experiment"
  from_version INTEGER,
  to_version INTEGER,
  reason TEXT NOT NULL,
  actor TEXT,                          -- user/daemon
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_policy_events_scope_time
  ON policy_events(scope, created_at);

-- Simple pointer table for "current" active version per channel (fast lookup).
CREATE TABLE IF NOT EXISTS policy_pointers (
  scope TEXT NOT NULL,
  domain TEXT NOT NULL,
  name TEXT NOT NULL,
  channel TEXT NOT NULL,
  active_version INTEGER NOT NULL,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(scope, domain, name, channel)
);

CREATE INDEX IF NOT EXISTS idx_policy_pointers
  ON policy_pointers(scope, domain, name, channel);

-- Optional: bind experiment → policy promotion record
CREATE TABLE IF NOT EXISTS policy_promotions (
  id TEXT PRIMARY KEY,
  experiment_id TEXT NOT NULL,
  scope TEXT NOT NULL,
  domain TEXT NOT NULL,
  name TEXT NOT NULL,
  promoted_version INTEGER NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_policy_promotions_exp
  ON policy_promotions(experiment_id);


---

2) 정책 타입/스펙

internal/policy/types.go

package policy

import "encoding/json"

type Domain string

const (
	DomainRouter       Domain = "router"
	DomainModelResolver Domain = "modelresolver"
	DomainPrompt       Domain = "prompt"
)

type Channel string

const (
	ChannelStable  Channel = "stable"
	ChannelBeta    Channel = "beta"
	ChannelNightly Channel = "nightly"
)

type Status string

const (
	StatusDraft      Status = "draft"
	StatusActive     Status = "active"
	StatusArchived   Status = "archived"
	StatusRolledBack Status = "rolled_back"
)

type Version struct {
	ID     string
	Scope  string
	Domain Domain
	Name   string

	Version int
	Status  Status
	Channel Channel

	SpecJSON string
	Source   string
	CreatedBy string
}

type Pointer struct {
	Scope         string
	Domain        Domain
	Name          string
	Channel       Channel
	ActiveVersion int
}

type Event struct {
	ID     string
	Scope  string
	Domain Domain
	Name   string

	Action      string
	FromVersion int
	ToVersion   int
	Reason      string
	Actor       string
}

// ---- Policy specs (JSON) ----

type RouterPolicySpec struct {
	// If set, router can override provider/model choice for some categories/agents.
	// key examples: "category:ultrabrain", "agent:oracle", "default"
	Overrides map[string]ProviderModel `json:"overrides"`

	// Optional knobs:
	MaxRetries int     `json:"max_retries"`
	HedgeAfterMs int   `json:"hedge_after_ms"`
	CostWeight float64 `json:"cost_weight"`
	LatencyWeight float64 `json:"latency_weight"`
	QualityWeight float64 `json:"quality_weight"`
}

type ModelResolverPolicySpec struct {
	// Override model resolution by agent/category name.
	AgentModels    map[string]string `json:"agent_models"`
	CategoryModels map[string]string `json:"category_models"`

	// Provider preference overrides for fallback chain (optional)
	ProviderPriority map[string][]string `json:"provider_priority"`
}

type PromptPolicySpec struct {
	SystemPrompt string `json:"system_prompt"`
	ExtraRules   string `json:"extra_rules"`
}

type ProviderModel struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func MustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}


---

3) Policy Store (DB)

internal/policy/store.go

package policy

import (
	"context"
	"database/sql"

	"devorch/internal/id"
	"devorch/internal/storage"
)

type Store struct {
	Storage storage.Storage
}

func NewStore(s storage.Storage) *Store { return &Store{Storage: s} }

func (st *Store) NextVersion(ctx context.Context, scope string, domain Domain, name string) (int, error) {
	row := st.Storage.QueryRow(ctx, `
SELECT COALESCE(MAX(version), 0) + 1
FROM policy_versions
WHERE scope=? AND domain=? AND name=?
`, scope, string(domain), name)
	var v int
	if err := row.Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

func (st *Store) CreateDraft(ctx context.Context, v Version, reason, actor string) (Version, error) {
	if v.ID == "" {
		v.ID = id.NewULID()
	}
	if v.Channel == "" {
		v.Channel = ChannelStable
	}
	if v.Status == "" {
		v.Status = StatusDraft
	}
	if v.Source == "" {
		v.Source = "manual"
	}
	if v.Version <= 0 {
		nv, err := st.NextVersion(ctx, v.Scope, v.Domain, v.Name)
		if err != nil {
			return Version{}, err
		}
		v.Version = nv
	}

	_, err := st.Storage.Exec(ctx, `
INSERT INTO policy_versions
(id, scope, domain, name, version, status, channel, spec_json, source, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, v.ID, v.Scope, string(v.Domain), v.Name, v.Version, string(v.Status), string(v.Channel), v.SpecJSON, v.Source, nullIfEmpty(v.CreatedBy))
	if err != nil {
		return Version{}, err
	}

	_ = st.insertEvent(ctx, Event{
		ID: id.NewULID(), Scope: v.Scope, Domain: v.Domain, Name: v.Name,
		Action: "create", FromVersion: 0, ToVersion: v.Version, Reason: reason, Actor: actor,
	})
	return v, nil
}

func (st *Store) Activate(ctx context.Context, scope string, domain Domain, name string, channel Channel, version int, reason, actor string) error {
	// mark previous active archived (same scope/domain/name/channel)
	// we keep versions immutable; pointer table is the "current"
	_, _ = st.Storage.Exec(ctx, `
UPDATE policy_versions
SET status='archived', archived_at=CURRENT_TIMESTAMP
WHERE scope=? AND domain=? AND name=? AND channel=? AND status='active'
`, scope, string(domain), name, string(channel))

	_, err := st.Storage.Exec(ctx, `
UPDATE policy_versions
SET status='active', activated_at=CURRENT_TIMESTAMP
WHERE scope=? AND domain=? AND name=? AND channel=? AND version=?
`, scope, string(domain), name, string(channel), version)
	if err != nil {
		return err
	}

	_, err = st.Storage.Exec(ctx, `
INSERT INTO policy_pointers (scope, domain, name, channel, active_version)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(scope,domain,name,channel) DO UPDATE SET
active_version=excluded.active_version,
updated_at=CURRENT_TIMESTAMP
`, scope, string(domain), name, string(channel), version)
	if err != nil {
		return err
	}

	return st.insertEvent(ctx, Event{
		ID: id.NewULID(), Scope: scope, Domain: domain, Name: name,
		Action: "activate", FromVersion: 0, ToVersion: version, Reason: reason, Actor: actor,
	})
}

func (st *Store) GetActive(ctx context.Context, scope string, domain Domain, name string, channel Channel) (Version, bool, error) {
	row := st.Storage.QueryRow(ctx, `
SELECT pv.id, pv.scope, pv.domain, pv.name, pv.version, pv.status, pv.channel, pv.spec_json, pv.source, COALESCE(pv.created_by,'')
FROM policy_pointers pp
JOIN policy_versions pv
  ON pv.scope=pp.scope AND pv.domain=pp.domain AND pv.name=pp.name AND pv.channel=pp.channel AND pv.version=pp.active_version
WHERE pp.scope=? AND pp.domain=? AND pp.name=? AND pp.channel=?
`, scope, string(domain), name, string(channel))

	var v Version
	var dom, stt, ch string
	err := row.Scan(&v.ID, &v.Scope, &dom, &v.Name, &v.Version, &stt, &ch, &v.SpecJSON, &v.Source, &v.CreatedBy)
	if err == sql.ErrNoRows {
		return Version{}, false, nil
	}
	if err != nil {
		return Version{}, false, err
	}
	v.Domain = Domain(dom)
	v.Status = Status(stt)
	v.Channel = Channel(ch)
	return v, true, nil
}

func (st *Store) Rollback(ctx context.Context, scope string, domain Domain, name string, channel Channel, reason, actor string) error {
	// find current active
	cur, ok, err := st.GetActive(ctx, scope, domain, name, channel)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	// find previous version (largest < current)
	row := st.Storage.QueryRow(ctx, `
SELECT version
FROM policy_versions
WHERE scope=? AND domain=? AND name=? AND channel=? AND version < ?
ORDER BY version DESC
LIMIT 1
`, scope, string(domain), name, string(channel), cur.Version)

	var prev int
	if err := row.Scan(&prev); err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return err
	}

	// mark current rolled back
	_, _ = st.Storage.Exec(ctx, `
UPDATE policy_versions
SET status='rolled_back', archived_at=CURRENT_TIMESTAMP
WHERE scope=? AND domain=? AND name=? AND channel=? AND version=?
`, scope, string(domain), name, string(channel), cur.Version)

	// activate previous
	if err := st.Activate(ctx, scope, domain, name, channel, prev, "rollback->"+itoa(cur.Version)+": "+reason, actor); err != nil {
		return err
	}

	return st.insertEvent(ctx, Event{
		ID: id.NewULID(), Scope: scope, Domain: domain, Name: name,
		Action: "rollback", FromVersion: cur.Version, ToVersion: prev, Reason: reason, Actor: actor,
	})
}

func (st *Store) insertEvent(ctx context.Context, e Event) error {
	_, err := st.Storage.Exec(ctx, `
INSERT INTO policy_events (id, scope, domain, name, action, from_version, to_version, reason, actor)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`, e.ID, e.Scope, string(e.Domain), e.Name, e.Action, nullIfZero(e.FromVersion), nullIfZero(e.ToVersion), e.Reason, nullIfEmpty(e.Actor))
	return err
}

// helpers
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func nullIfZero(i int) any {
	if i == 0 {
		return nil
	}
	return i
}
func itoa(n int) string {
	if n == 0 { return "0" }
	sign := ""
	if n < 0 { sign = "-"; n = -n }
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + (n % 10))
		n /= 10
	}
	return sign + string(b[i:])
}


---

4) Effective Policy Resolver (scope 우선순위)

internal/policy/resolver.go

package policy

import (
	"context"
	"encoding/json"
)

type Resolver struct {
	Store *Store
}

func NewResolver(st *Store) *Resolver { return &Resolver{Store: st} }

// Scope precedence: project > workspace > global
func (r *Resolver) ResolveActive(ctx context.Context, scopeProject string, scopeWorkspace string, domain Domain, name string, channel Channel) (Version, bool, error) {
	if scopeProject != "" {
		if v, ok, err := r.Store.GetActive(ctx, scopeProject, domain, name, channel); err != nil {
			return Version{}, false, err
		} else if ok {
			return v, true, nil
		}
	}
	if scopeWorkspace != "" {
		if v, ok, err := r.Store.GetActive(ctx, scopeWorkspace, domain, name, channel); err != nil {
			return Version{}, false, err
		} else if ok {
			return v, true, nil
		}
	}
	return r.Store.GetActive(ctx, "global", domain, name, channel)
}

func ParseRouterSpec(v Version) (RouterPolicySpec, error) {
	var s RouterPolicySpec
	err := json.Unmarshal([]byte(v.SpecJSON), &s)
	return s, err
}

func ParseModelResolverSpec(v Version) (ModelResolverPolicySpec, error) {
	var s ModelResolverPolicySpec
	err := json.Unmarshal([]byte(v.SpecJSON), &s)
	return s, err
}

func ParsePromptSpec(v Version) (PromptPolicySpec, error) {
	var s PromptPolicySpec
	err := json.Unmarshal([]byte(v.SpecJSON), &s)
	return s, err
}


---

5) Experiment Winner → Policy 승격

> Step17/18의 experiments/variants/winner_key 구조를 사용한다고 가정합니다.
Winner variant의 payload_json을 정책 spec_json으로 변환해 “새 policy version(draft)” 생성 후 “activate” 합니다.



internal/policy/promotion.go

package policy

import (
	"context"
	"database/sql"
	"encoding/json"

	"devorch/internal/id"
	"devorch/internal/storage"
)

type PromotionService struct {
	Store   *Store
	Storage storage.Storage
}

func NewPromotionService(st *Store, s storage.Storage) *PromotionService {
	return &PromotionService{Store: st, Storage: s}
}

type ExperimentWinner struct {
	ExperimentID string
	Kind string
	WinnerKey string
	WinnerPayloadJSON string
}

func (ps *PromotionService) LoadExperimentWinner(ctx context.Context, experimentID string) (ExperimentWinner, bool, error) {
	// experiments table must have winner_key
	row := ps.Storage.QueryRow(ctx, `
SELECT e.id, e.kind, COALESCE(e.winner_key,''), COALESCE(v.payload_json,'')
FROM experiments e
LEFT JOIN experiment_variants v
  ON v.experiment_id = e.id AND v.key = e.winner_key
WHERE e.id = ?
`, experimentID)

	var ew ExperimentWinner
	err := row.Scan(&ew.ExperimentID, &ew.Kind, &ew.WinnerKey, &ew.WinnerPayloadJSON)
	if err == sql.ErrNoRows {
		return ExperimentWinner{}, false, nil
	}
	if err != nil {
		return ExperimentWinner{}, false, err
	}
	if ew.WinnerKey == "" || ew.WinnerPayloadJSON == "" {
		return ExperimentWinner{}, false, nil
	}
	return ew, true, nil
}

// Promote winner -> policy version and activate.
// domain/name mapping rule:
// - kind="prompt" => domain="prompt", name="system-prompt"
// - kind="routing" => domain="router", name="default-routing"
// - kind="policy" => domain="modelresolver", name="default-models"
func (ps *PromotionService) PromoteWinnerToPolicy(ctx context.Context, experimentID string, scope string, channel Channel, reason, actor string) (Version, error) {
	ew, ok, err := ps.LoadExperimentWinner(ctx, experimentID)
	if err != nil {
		return Version{}, err
	}
	if !ok {
		return Version{}, errf("no winner payload for experiment")
	}

	var domain Domain
	var name string

	switch ew.Kind {
	case "prompt":
		domain = DomainPrompt
		name = "system-prompt"
	case "routing":
		domain = DomainRouter
		name = "default-routing"
	default:
		domain = DomainModelResolver
		name = "default-models"
	}

	specJSON, err := normalizeWinnerPayloadToPolicySpec(domain, ew.WinnerPayloadJSON)
	if err != nil {
		return Version{}, err
	}

	v := Version{
		ID:       id.NewULID(),
		Scope:    scope,
		Domain:   domain,
		Name:     name,
		Channel:  channel,
		Status:   StatusDraft,
		SpecJSON: specJSON,
		Source:   "experiment:" + experimentID,
	}
	created, err := ps.Store.CreateDraft(ctx, v, "promote_from_experiment: "+reason, actor)
	if err != nil {
		return Version{}, err
	}

	if err := ps.Store.Activate(ctx, scope, domain, name, channel, created.Version, "promoted winner: "+reason, actor); err != nil {
		return Version{}, err
	}

	_, _ = ps.Storage.Exec(ctx, `
INSERT INTO policy_promotions (id, experiment_id, scope, domain, name, promoted_version)
VALUES (?, ?, ?, ?, ?, ?)
`, id.NewULID(), experimentID, scope, string(domain), name, created.Version)

	_ = ps.Store.insertEvent(ctx, Event{
		ID: id.NewULID(), Scope: scope, Domain: domain, Name: name,
		Action: "promote_from_experiment", FromVersion: 0, ToVersion: created.Version,
		Reason: reason, Actor: actor,
	})

	return created, nil
}

// Winner payload_json format (from experiment variants) can be:
// - router policy: {"overrides":{...}, ...}
// - modelresolver policy: {"agent_models":{...}, ...}
// - prompt policy: {"system_prompt":"...", "extra_rules":"..."}
// If payload is a "generic" envelope, unwrap gracefully.
func normalizeWinnerPayloadToPolicySpec(domain Domain, payload string) (string, error) {
	// try as map first
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return payload, nil // already a string json; let later validation fail if needed
	}

	// unwrap known envelope fields
	if v, ok := m["policy"]; ok {
		b, _ := json.Marshal(v)
		return string(b), nil
	}
	if v, ok := m["spec"]; ok {
		b, _ := json.Marshal(v)
		return string(b), nil
	}

	// otherwise keep as-is
	b, _ := json.Marshal(m)
	return string(b), nil
}

// tiny error helper
type simpleErr string
func (e simpleErr) Error() string { return string(e) }
func errf(s string) error { return simpleErr(s) }


---

6) 롤백/채널 이동(승격)

internal/policy/rollback.go

package policy

import "context"

// Promote channel: copy active from one channel to another as new version in target channel.
func PromoteChannel(ctx context.Context, st *Store, scope string, domain Domain, name string, from Channel, to Channel, reason, actor string) error {
	cur, ok, err := st.GetActive(ctx, scope, domain, name, from)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	// create new draft in target channel with same spec
	newV := Version{
		Scope:    scope,
		Domain:   domain,
		Name:     name,
		Channel:  to,
		Status:   StatusDraft,
		SpecJSON: cur.SpecJSON,
		Source:   "manual",
	}
	created, err := st.CreateDraft(ctx, newV, "promote_channel "+string(from)+"->"+string(to)+": "+reason, actor)
	if err != nil {
		return err
	}

	return st.Activate(ctx, scope, domain, name, to, created.Version, "channel promotion: "+reason, actor)
}


---

7) Router에 정책 오버레이 적용

internal/router/policy_overlay.go

package router

import (
	"context"

	"devorch/internal/policy"
)

type PolicyHook struct {
	Resolver *policy.Resolver
	Store    *policy.Store
	Channel  policy.Channel
}

type PolicyContext struct {
	ScopeProject   string
	ScopeWorkspace string
}

func (h *PolicyHook) ApplyRouterPolicy(ctx context.Context, pc PolicyContext, key string) (provider string, model string, ok bool) {
	if h == nil || h.Resolver == nil {
		return "", "", false
	}
	v, found, err := h.Resolver.ResolveActive(ctx, pc.ScopeProject, pc.ScopeWorkspace, policy.DomainRouter, "default-routing", h.Channel)
	if err != nil || !found {
		return "", "", false
	}
	spec, err := policy.ParseRouterSpec(v)
	if err != nil {
		return "", "", false
	}
	if spec.Overrides == nil {
		return "", "", false
	}
	pm, ok2 := spec.Overrides[key]
	if !ok2 || pm.Provider == "" || pm.Model == "" {
		return "", "", false
	}
	return pm.Provider, pm.Model, true
}

internal/router/router.go (수정: 정책 적용 지점만 추가)

package router

import (
	"context"

	"devorch/internal/provider"
)

type Router struct {
	// 기존 필드들...
	Registry *provider.Registry

	Policy *PolicyHook // Step19 추가
}

type RouteInput struct {
	Category string
	Agent    string

	ScopeProject   string
	ScopeWorkspace string

	// existing fields: latency/cost/quality context...
}

type RouteResult struct {
	Provider string
	Model    string
}

func (r *Router) Route(ctx context.Context, in RouteInput) (RouteResult, error) {
	// Step19: policy override priority
	if r.Policy != nil {
		if p, m, ok := r.Policy.ApplyRouterPolicy(ctx, PolicyContext{ScopeProject: in.ScopeProject, ScopeWorkspace: in.ScopeWorkspace}, "category:"+in.Category); ok {
			return RouteResult{Provider: p, Model: m}, nil
		}
		if p, m, ok := r.Policy.ApplyRouterPolicy(ctx, PolicyContext{ScopeProject: in.ScopeProject, ScopeWorkspace: in.ScopeWorkspace}, "agent:"+in.Agent); ok {
			return RouteResult{Provider: p, Model: m}, nil
		}
		if p, m, ok := r.Policy.ApplyRouterPolicy(ctx, PolicyContext{ScopeProject: in.ScopeProject, ScopeWorkspace: in.ScopeWorkspace}, "default"); ok {
			return RouteResult{Provider: p, Model: m}, nil
		}
	}

	// ... 기존 라우팅 스코어링/폴백 로직
	// (여기서는 Step19 영향 최소화를 위해 기존 구현 유지)
	return RouteResult{Provider: "openai", Model: "gpt-5.2"}, nil
}


---

8) ModelResolver에 정책 오버레이 적용

internal/modelresolver/policy_overlay.go

package modelresolver

import (
	"context"

	"devorch/internal/policy"
)

type PolicyHook struct {
	Resolver *policy.Resolver
	Channel  policy.Channel
}

type PolicyContext struct {
	ScopeProject   string
	ScopeWorkspace string
}

func (h *PolicyHook) OverrideModel(ctx context.Context, pc PolicyContext, agent string, category string) (string, bool) {
	if h == nil || h.Resolver == nil {
		return "", false
	}
	v, found, err := h.Resolver.ResolveActive(ctx, pc.ScopeProject, pc.ScopeWorkspace, policy.DomainModelResolver, "default-models", h.Channel)
	if err != nil || !found {
		return "", false
	}
	spec, err := policy.ParseModelResolverSpec(v)
	if err != nil {
		return "", false
	}

	if agent != "" && spec.AgentModels != nil {
		if m, ok := spec.AgentModels[agent]; ok && m != "" {
			return m, true
		}
	}
	if category != "" && spec.CategoryModels != nil {
		if m, ok := spec.CategoryModels[category]; ok && m != "" {
			return m, true
		}
	}
	return "", false
}

internal/modelresolver/resolve.go (수정: 최우선 오버라이드)

package modelresolver

import "context"

type Resolver struct {
	// 기존 resolver 필드...
	Policy *PolicyHook // Step19 추가
}

type ResolveInput struct {
	Agent    string
	Category string

	ScopeProject   string
	ScopeWorkspace string
}

func (r *Resolver) Resolve(ctx context.Context, in ResolveInput) (string, error) {
	// Step19: policy override highest priority
	if r.Policy != nil {
		if m, ok := r.Policy.OverrideModel(ctx, PolicyContext{ScopeProject: in.ScopeProject, ScopeWorkspace: in.ScopeWorkspace}, in.Agent, in.Category); ok {
			return m, nil
		}
	}

	// ... 기존 3단계(override→fallback chain→default) 로직
	return "openai/gpt-5.2", nil
}


---

9) Policy API (조회/승격/롤백)

internal/server/routes/policy.go

package routes

import (
	"net/http"

	"devorch/internal/policy"
)

type PolicyRoutes struct {
	Store *policy.Store
	Resolver *policy.Resolver
	Promoter *policy.PromotionService
}

func NewPolicyRoutes(st *policy.Store, r *policy.Resolver, p *policy.PromotionService) *PolicyRoutes {
	return &PolicyRoutes{Store: st, Resolver: r, Promoter: p}
}

func (pr *PolicyRoutes) Register(mux Mux) {
	mux.GET("/policy/active", pr.getActive)
	mux.POST("/policy/activate", pr.activate)
	mux.POST("/policy/rollback", pr.rollback)
	mux.POST("/policy/promote-from-experiment", pr.promoteFromExperiment)
	mux.POST("/policy/promote-channel", pr.promoteChannel)
}

func (pr *PolicyRoutes) getActive(w http.ResponseWriter, req *http.Request) {
	scope := req.URL.Query().Get("scope")
	if scope == "" { scope = "global" }
	domain := policy.Domain(req.URL.Query().Get("domain"))
	name := req.URL.Query().Get("name")
	channel := policy.Channel(req.URL.Query().Get("channel"))
	if channel == "" { channel = policy.ChannelStable }

	v, ok, err := pr.Store.GetActive(req.Context(), scope, domain, name, channel)
	if err != nil {
		writeErr(w, 500, err); return
	}
	if !ok {
		writeJSON(w, 200, map[string]any{"ok": true, "found": false}); return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "found": true, "policy": v})
}

func (pr *PolicyRoutes) activate(w http.ResponseWriter, req *http.Request) {
	var in struct {
		Scope string `json:"scope"`
		Domain string `json:"domain"`
		Name string `json:"name"`
		Channel string `json:"channel"`
		Version int `json:"version"`
		Reason string `json:"reason"`
		Actor string `json:"actor"`
	}
	if err := decodeJSON(req, &in); err != nil {
		writeErr(w, 400, err); return
	}
	if in.Scope == "" { in.Scope = "global" }
	if in.Channel == "" { in.Channel = string(policy.ChannelStable) }
	if in.Reason == "" { in.Reason = "manual activate" }
	if err := pr.Store.Activate(req.Context(), in.Scope, policy.Domain(in.Domain), in.Name, policy.Channel(in.Channel), in.Version, in.Reason, in.Actor); err != nil {
		writeErr(w, 500, err); return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (pr *PolicyRoutes) rollback(w http.ResponseWriter, req *http.Request) {
	var in struct {
		Scope string `json:"scope"`
		Domain string `json:"domain"`
		Name string `json:"name"`
		Channel string `json:"channel"`
		Reason string `json:"reason"`
		Actor string `json:"actor"`
	}
	if err := decodeJSON(req, &in); err != nil {
		writeErr(w, 400, err); return
	}
	if in.Scope == "" { in.Scope = "global" }
	if in.Channel == "" { in.Channel = string(policy.ChannelStable) }
	if in.Reason == "" { in.Reason = "manual rollback" }

	if err := pr.Store.Rollback(req.Context(), in.Scope, policy.Domain(in.Domain), in.Name, policy.Channel(in.Channel), in.Reason, in.Actor); err != nil {
		writeErr(w, 500, err); return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (pr *PolicyRoutes) promoteFromExperiment(w http.ResponseWriter, req *http.Request) {
	var in struct {
		ExperimentID string `json:"experiment_id"`
		Scope string `json:"scope"`
		Channel string `json:"channel"`
		Reason string `json:"reason"`
		Actor string `json:"actor"`
	}
	if err := decodeJSON(req, &in); err != nil {
		writeErr(w, 400, err); return
	}
	if in.Scope == "" { in.Scope = "global" }
	ch := policy.Channel(in.Channel)
	if ch == "" { ch = policy.ChannelStable }
	if in.Reason == "" { in.Reason = "promote winner" }

	v, err := pr.Promoter.PromoteWinnerToPolicy(req.Context(), in.ExperimentID, in.Scope, ch, in.Reason, in.Actor)
	if err != nil {
		writeErr(w, 500, err); return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "created_version": v.Version})
}

func (pr *PolicyRoutes) promoteChannel(w http.ResponseWriter, req *http.Request) {
	var in struct {
		Scope string `json:"scope"`
		Domain string `json:"domain"`
		Name string `json:"name"`
		From string `json:"from"`
		To string `json:"to"`
		Reason string `json:"reason"`
		Actor string `json:"actor"`
	}
	if err := decodeJSON(req, &in); err != nil {
		writeErr(w, 400, err); return
	}
	if in.Scope == "" { in.Scope = "global" }
	if in.Reason == "" { in.Reason = "channel promotion" }

	if err := policy.PromoteChannel(req.Context(), pr.Store, in.Scope, policy.Domain(in.Domain), in.Name, policy.Channel(in.From), policy.Channel(in.To), in.Reason, in.Actor); err != nil {
		writeErr(w, 500, err); return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}


---

10) Doctor 체크(정책 레지스트리)

internal/diagnostics/checks/policy_registry.go

package checks

import (
	"context"

	"devorch/internal/diagnostics"
	"devorch/internal/storage"
)

type PolicyRegistryCheck struct {
	Storage storage.Storage
}

func (c PolicyRegistryCheck) Name() string { return "policy_registry" }

func (c PolicyRegistryCheck) Run(ctx context.Context) diagnostics.Result {
	row := c.Storage.QueryRow(ctx, `
SELECT COUNT(*) FROM sqlite_master
WHERE type='table' AND name IN ('policy_versions','policy_pointers','policy_events')
`)
	var n int
	_ = row.Scan(&n)
	if n < 3 {
		return diagnostics.Fail("policy tables missing (run migrations)")
	}
	return diagnostics.Pass("policy registry ok")
}


---

11) 부트스트랩 wiring

internal/policy/bootstrap.go

package policy

import (
	"devorch/internal/router"
	"devorch/internal/modelresolver"
)

type Wiring struct {
	Store    *Store
	Resolver *Resolver
	Channel  Channel
}

func AttachToRouter(w Wiring, r *router.Router) {
	if r == nil { return }
	r.Policy = &router.PolicyHook{
		Resolver: w.Resolver,
		Store:    w.Store,
		Channel:  w.Channel,
	}
}

func AttachToModelResolver(w Wiring, mr *modelresolver.Resolver) {
	if mr == nil { return }
	mr.Policy = &modelresolver.PolicyHook{
		Resolver: w.Resolver,
		Channel:  w.Channel,
	}
}


---

Step19까지 “운영 루프” 완성

Step17/18: 실험 자동 운영 + 드리프트 감지

Step19: 승자 → 운영 정책(버전/채널) 반영 + 롤백 가능

Router/ModelResolver는 항상 정책을 최우선 적용 → “쓰면 쓸수록 자동으로 최적 설정으로 굳어짐”



---

원하시면 Step20는 보통 이렇게 갑니다:

Step20: 정책 검증/서명/배포(오프라인 번들 포함) + 정책 충돌 검사 + “정책 PR(리뷰)” 워크플로우 (엔터프라이즈급)


바로 20단계로 진행할까요?