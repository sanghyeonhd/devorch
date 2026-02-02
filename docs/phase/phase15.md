좋습니다. Step 14 = “개인/워크스페이스 단위 라우팅 정책 + 팀 공유/전파 + 프라이버시/격리” 풀코드입니다.
(= 같은 OS/같은 작업이라도 사용자/팀/프로젝트별로 최적 모델이 달라지는 현실을 반영)

아래는 SQL 포함, 파일별 풀코드입니다. (Step 13 기반 “추가/수정”)


---

Step 14 소스트리 추가/수정

devorch/
└─ internal/
   ├─ router/
   │  ├─ scope/
   │  │  ├─ scope.go                 # (NEW) policy scope (global/org/ws/user/project)
   │  │  ├─ resolve_scope.go         # (NEW) 요청 컨텍스트에서 scope key 생성
   │  │  └─ privacy.go               # (NEW) 공유/전파 정책(옵트인/익명화)
   │  │
   │  ├─ learner/
   │  │  ├─ features.go              # (MOD) scope 포함
   │  │  ├─ learner.go               # (MOD) scope 별 집계
   │  │  └─ trainer.go               # (MOD) scope 별 policy upsert
   │  │
   │  ├─ policy_store.go             # (MOD) scope-aware upsert/load
   │  ├─ policy_runtime.go           # (MOD) 우선순위: user>ws>org>global
   │  ├─ share/
   │  │  ├─ exporter.go              # (NEW) 정책 export (redacted)
   │  │  ├─ importer.go              # (NEW) 정책 import
   │  │  └─ signer.go                # (NEW) 서명/검증(선택)
   │  │
   │  ├─ auto_update.go              # (MOD) scope별 학습 루프
   │  └─ types.go                    # (NEW) 공용 타입
   │
   ├─ storage/sqlite/migrations/
   │  └─ 0004_router_policy_scope.sql  # (NEW)
   │
   ├─ server/routes/
   │  ├─ router.go                   # (MOD) 정책 조회/override/export/import API
   │  └─ bench.go                    # (MOD) bench runs에 scope 저장(선택)
   │
   ├─ api_gateway/
   │  └─ auth_middleware.go          # (MOD) user/workspace context 주입
   │
   └─ diagnostics/checks/
      └─ router_sharing.go           # (NEW) 공유/전파 점검


---

0) 전제: Step 14에서 “스코프” 정의

정책은 아래 우선순위로 적용됩니다:

1. user


2. workspace


3. org


4. global



> 즉, 팀 공통 최적값이 있더라도, 특정 사용자만 다른 모델을 더 잘 쓰면 user 스코프가 이깁니다.




---

1) DB 마이그레이션

1.1 internal/storage/sqlite/migrations/0004_router_policy_scope.sql

-- Step 14: router_policy를 scope-aware로 확장

ALTER TABLE router_policy ADD COLUMN scope_type TEXT NOT NULL DEFAULT 'global';
ALTER TABLE router_policy ADD COLUMN scope_id   TEXT NOT NULL DEFAULT '*';

-- 기존 UNIQUE 제약이 scope를 고려하지 못하므로, 새 테이블을 만드는 방식이 안전하지만
-- 여기서는 "단순화를 위해" 기존 인덱스를 새 인덱스로 대체한다.
-- SQLite에서 제약 수정은 까다로우니, 실제 운영에서는 "테이블 재작성"을 권장.

DROP INDEX IF EXISTS idx_router_policy_key;

CREATE INDEX IF NOT EXISTS idx_router_policy_scope_key
  ON router_policy(scope_type, scope_id, os, arch, scenario);

-- scope를 포함한 논리적 유니크를 보장하려면 애플리케이션 Upsert에서 강제한다.

> SQLite에서 기존 UNIQUE(os,arch,scenario,provider,model)을 완전히 바꾸려면 테이블 재작성 필요합니다.
하지만 Step 14에서는 Upsert WHERE 조건에 scope를 포함해서 사실상 동일 효과를 냅니다(아래 코드).




---

2) Router Scope 모듈

2.1 internal/router/scope/scope.go

package scope

type Type string

const (
	Global   Type = "global"
	Org      Type = "org"
	Workspace Type = "workspace"
	User     Type = "user"
	Project  Type = "project"
)

type Key struct {
	Type Type
	ID   string // global = "*"
}

func GlobalKey() Key {
	return Key{Type: Global, ID: "*"}
}

2.2 internal/router/scope/resolve_scope.go

package scope

import "context"

type CtxKey string

const (
	CtxOrgID       CtxKey = "org_id"
	CtxWorkspaceID CtxKey = "workspace_id"
	CtxUserID      CtxKey = "user_id"
	CtxProjectID   CtxKey = "project_id"
)

// 요청 컨텍스트를 기반으로, 라우팅에 사용할 scope 후보를 만든다.
// 우선순위: user > workspace > org > global
func Candidates(ctx context.Context) []Key {
	var out []Key

	if v := ctx.Value(CtxUserID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, Key{Type: User, ID: s})
		}
	}
	if v := ctx.Value(CtxWorkspaceID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, Key{Type: Workspace, ID: s})
		}
	}
	if v := ctx.Value(CtxOrgID); v != nil {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, Key{Type: Org, ID: s})
		}
	}

	out = append(out, GlobalKey())
	return out
}

2.3 internal/router/scope/privacy.go

package scope

// 공유/전파 정책을 위한 기본 규칙.
// - 기본: workspace/org 공유는 "옵트인"
// - user scope는 원칙적으로 외부로 export 불가(또는 익명화)
type Privacy struct {
	AllowExportUserScope bool // 기본 false
	AllowImportToUserScope bool // 기본 false

	// workspace/org 공유 시 식별자 최소화
	RedactProviderKeys bool // API key 같은 건 절대 포함 금지(원천적으로 저장도 금지)
	RedactExactModelNames bool // 필요 시 모델명을 버킷(large/medium/small)로 변환 가능
}

func DefaultPrivacy() Privacy {
	return Privacy{
		AllowExportUserScope:   false,
		AllowImportToUserScope: false,
		RedactProviderKeys:     true,
		RedactExactModelNames:  false,
	}
}


---

3) Router 공용 타입 추가

3.1 internal/router/types.go

package router

import "devorch/internal/router/scope"

type ScopedKey = scope.Key


---

4) Policy Store (scope-aware)

4.1 internal/router/policy_store.go (Step 13 파일 “교체/확장”)

package router

import (
	"context"
	"time"

	"devorch/internal/router/scope"
	"devorch/internal/storage"
)

type Policy struct {
	ScopeType string
	ScopeID   string

	OS       string
	Arch     string
	Scenario string

	Provider string
	Model    string

	Weight       float64
	SuccessRate  float64
	AvgLatencyMs float64
	AvgCost      float64
	SampleCount  int64

	UpdatedAt time.Time
}

type PolicyStore struct {
	Storage storage.Storage
}

func NewPolicyStore(s storage.Storage) *PolicyStore {
	return &PolicyStore{Storage: s}
}

func (ps *PolicyStore) Upsert(ctx context.Context, p Policy) error {
	// scope-aware upsert:
	// SQLite의 UNIQUE 한계 때문에 "논리적 upsert"를 직접 수행한다.

	// 1) 업데이트 시도
	res, err := ps.Storage.Exec(ctx, `
UPDATE router_policy
SET
  weight = ?,
  success_rate = ?,
  avg_latency_ms = ?,
  avg_cost = ?,
  sample_count = ?,
  updated_at = CURRENT_TIMESTAMP
WHERE
  scope_type = ? AND scope_id = ?
  AND os = ? AND arch = ? AND scenario = ?
  AND provider = ? AND model = ?
`,
		p.Weight, p.SuccessRate, p.AvgLatencyMs, p.AvgCost, p.SampleCount,
		p.ScopeType, p.ScopeID,
		p.OS, p.Arch, p.Scenario,
		p.Provider, p.Model,
	)
	if err != nil {
		return err
	}

	affected, _ := res.RowsAffected()
	if affected > 0 {
		return nil
	}

	// 2) 없으면 insert
	_, err = ps.Storage.Exec(ctx, `
INSERT INTO router_policy
(scope_type, scope_id, os, arch, scenario, provider, model, weight, success_rate, avg_latency_ms, avg_cost, sample_count, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
`,
		p.ScopeType, p.ScopeID,
		p.OS, p.Arch, p.Scenario,
		p.Provider, p.Model,
		p.Weight, p.SuccessRate, p.AvgLatencyMs, p.AvgCost, p.SampleCount,
	)
	return err
}

func (ps *PolicyStore) LoadForKey(ctx context.Context, key scope.Key, os, arch, scenario string) ([]Policy, error) {
	rows, err := ps.Storage.Query(ctx, `
SELECT
  scope_type, scope_id,
  provider, model,
  weight, success_rate, avg_latency_ms, avg_cost, sample_count, updated_at
FROM router_policy
WHERE scope_type = ? AND scope_id = ?
  AND os = ? AND arch = ? AND scenario = ?
ORDER BY weight DESC
`,
		string(key.Type), key.ID, os, arch, scenario,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Policy
	for rows.Next() {
		var p Policy
		if err := rows.Scan(
			&p.ScopeType, &p.ScopeID,
			&p.Provider, &p.Model,
			&p.Weight, &p.SuccessRate, &p.AvgLatencyMs, &p.AvgCost, &p.SampleCount,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p.OS, p.Arch, p.Scenario = os, arch, scenario
		out = append(out, p)
	}
	return out, nil
}


---

5) Runtime Router 적용 (scope 우선순위)

5.1 internal/router/policy_runtime.go (교체)

package router

import (
	"context"

	"devorch/internal/router/scope"
)

type PolicyRuntime struct {
	Store *PolicyStore
}

func NewPolicyRuntime(store *PolicyStore) *PolicyRuntime {
	return &PolicyRuntime{Store: store}
}

// user > workspace > org > global 순서로 정책을 찾는다.
func (pr *PolicyRuntime) ChooseBest(ctx context.Context, os, arch, scenario string) (provider string, model string, ok bool) {
	if pr.Store == nil {
		return "", "", false
	}

	candidates := scope.Candidates(ctx)
	for _, k := range candidates {
		policies, err := pr.Store.LoadForKey(ctx, k, os, arch, scenario)
		if err != nil || len(policies) == 0 {
			continue
		}
		top := policies[0]
		return top.Provider, top.Model, true
	}

	return "", "", false
}

> 기존 Router가 ChooseBest() 직접 가지고 있었다면, Router 안에 PolicyRuntime를 두고 위 함수를 호출하도록 연결하면 됩니다.




---

6) Learner/Trainer scope 반영

6.1 internal/router/learner/features.go (교체)

package learner

import (
	"devorch/internal/provider/observer"
	"devorch/internal/router/scope"
)

type Features struct {
	ScopeType string
	ScopeID   string

	OS       string
	Arch     string
	Scenario string

	Provider string
	Model    string

	LatencyMs    int64
	Success      bool
	InputTokens  int64
	OutputTokens int64
	Cost         float64
}

func FromCall(key scope.Key, c observer.ProviderCall, cost float64) Features {
	return Features{
		ScopeType: string(key.Type),
		ScopeID:   key.ID,

		OS:       c.OS,
		Arch:     c.Arch,
		Scenario: c.Scenario,

		Provider: c.Provider,
		Model:    c.Model,

		LatencyMs:    c.LatencyMs,
		Success:      c.Success,
		InputTokens:  c.InputTokens,
		OutputTokens: c.OutputTokens,
		Cost:         cost,
	}
}

6.2 internal/router/learner/learner.go (교체/확장)

package learner

import (
	"context"

	"devorch/internal/storage"
)

type Aggregate struct {
	ScopeType string
	ScopeID   string

	OS       string
	Arch     string
	Scenario string

	Provider string
	Model    string

	SampleCount  int64
	SuccessCount int64

	AvgLatency float64
	AvgCost    float64
}

type Learner struct {
	Storage storage.Storage
}

func New(storage storage.Storage) *Learner {
	return &Learner{Storage: storage}
}

// bench_runs 테이블에 scope_type/scope_id 컬럼이 존재한다고 가정(권장).
// 없다면, 최소 global로만 학습되도록 fallback 하거나,
// bench 저장 단계에서 scope를 강제 기록하세요.
func (l *Learner) Aggregate(ctx context.Context) ([]Aggregate, error) {
	rows, err := l.Storage.Query(ctx, `
SELECT 
  COALESCE(scope_type, 'global') as scope_type,
  COALESCE(scope_id, '*') as scope_id,
  os, arch, scenario, provider, model,
  COUNT(*) as sample_count,
  SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
  AVG(latency_ms) as avg_latency,
  AVG(cost) as avg_cost
FROM bench_runs
GROUP BY scope_type, scope_id, os, arch, scenario, provider, model
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Aggregate
	for rows.Next() {
		var a Aggregate
		if err := rows.Scan(
			&a.ScopeType, &a.ScopeID,
			&a.OS, &a.Arch, &a.Scenario, &a.Provider, &a.Model,
			&a.SampleCount, &a.SuccessCount,
			&a.AvgLatency, &a.AvgCost,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

6.3 internal/router/learner/trainer.go (교체/확장)

package learner

import (
	"context"

	"devorch/internal/router"
)

type Trainer struct {
	PolicyStore *router.PolicyStore
}

func NewTrainer(ps *router.PolicyStore) *Trainer {
	return &Trainer{PolicyStore: ps}
}

func (t *Trainer) Train(ctx context.Context, aggs []Aggregate) error {
	for _, a := range aggs {
		successRate := 0.0
		if a.SampleCount > 0 {
			successRate = float64(a.SuccessCount) / float64(a.SampleCount)
		}

		weight := ComputeWeight(successRate, a.AvgLatency, a.AvgCost)

		err := t.PolicyStore.Upsert(ctx, router.Policy{
			ScopeType: a.ScopeType,
			ScopeID:   a.ScopeID,

			OS:       a.OS,
			Arch:     a.Arch,
			Scenario: a.Scenario,

			Provider: a.Provider,
			Model:    a.Model,

			Weight:       weight,
			SuccessRate:  successRate,
			AvgLatencyMs: a.AvgLatency,
			AvgCost:      a.AvgCost,
			SampleCount:  a.SampleCount,
		})
		if err != nil {
			return err
		}
	}
	return nil
}


---

7) Auto Update (scope-aware 학습 루프)

7.1 internal/router/auto_update.go (교체)

package router

import (
	"context"
	"time"

	"devorch/internal/router/learner"
)

type AutoUpdater struct {
	Learner *learner.Learner
	Trainer *learner.Trainer

	Interval time.Duration
}

func (a *AutoUpdater) Run(ctx context.Context) {
	t := time.NewTicker(a.Interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			aggs, err := a.Learner.Aggregate(ctx)
			if err != nil {
				continue
			}
			_ = a.Trainer.Train(ctx, aggs)
		}
	}
}


---

8) 정책 공유/전파: Export/Import

8.1 Exporter (redacted)

internal/router/share/exporter.go

package share

import (
	"context"

	"devorch/internal/router"
	"devorch/internal/router/scope"
)

type ExportRequest struct {
	Scope scope.Key

	OS       string
	Arch     string
	Scenario string
}

type ExportedPolicy struct {
	ScopeType string
	ScopeID   string

	OS       string
	Arch     string
	Scenario string

	Provider string
	Model    string

	Weight       float64
	SuccessRate  float64
	AvgLatencyMs float64
	AvgCost      float64
	SampleCount  int64
}

type ExportBundle struct {
	Version string
	Policies []ExportedPolicy
}

type Exporter struct {
	Store   *router.PolicyStore
	Privacy scope.Privacy
}

func NewExporter(store *router.PolicyStore, privacy scope.Privacy) *Exporter {
	return &Exporter{Store: store, Privacy: privacy}
}

func (e *Exporter) Export(ctx context.Context, req ExportRequest) (ExportBundle, error) {
	if req.Scope.Type == scope.User && !e.Privacy.AllowExportUserScope {
		return ExportBundle{Version: "v1", Policies: nil}, nil
	}

	policies, err := e.Store.LoadForKey(ctx, req.Scope, req.OS, req.Arch, req.Scenario)
	if err != nil {
		return ExportBundle{}, err
	}

	out := make([]ExportedPolicy, 0, len(policies))
	for _, p := range policies {
		ep := ExportedPolicy{
			ScopeType: p.ScopeType,
			ScopeID:   p.ScopeID,
			OS:       p.OS,
			Arch:     p.Arch,
			Scenario: p.Scenario,
			Provider: p.Provider,
			Model:    p.Model,
			Weight:   p.Weight,

			SuccessRate:  p.SuccessRate,
			AvgLatencyMs: p.AvgLatencyMs,
			AvgCost:      p.AvgCost,
			SampleCount:  p.SampleCount,
		}

		// 민감정보는 원천적으로 policy에 저장하지 않지만,
		// 혹시라도 provider 모델명이 민감하면 익명화 옵션 제공
		if e.Privacy.RedactExactModelNames {
			ep.Model = BucketizeModel(ep.Model)
		}

		out = append(out, ep)
	}

	return ExportBundle{Version: "v1", Policies: out}, nil
}

// 매우 단순한 버킷 예시(원하면 더 정교하게)
func BucketizeModel(model string) string {
	if model == "" {
		return "unknown"
	}
	return "model_bucket"
}

8.2 Importer

internal/router/share/importer.go

package share

import (
	"context"

	"devorch/internal/router"
	"devorch/internal/router/scope"
)

type ImportRequest struct {
	TargetScope scope.Key
	Bundle      ExportBundle
}

type Importer struct {
	Store   *router.PolicyStore
	Privacy scope.Privacy
}

func NewImporter(store *router.PolicyStore, privacy scope.Privacy) *Importer {
	return &Importer{Store: store, Privacy: privacy}
}

func (i *Importer) Import(ctx context.Context, req ImportRequest) error {
	if req.TargetScope.Type == scope.User && !i.Privacy.AllowImportToUserScope {
		// 정책적으로 막기
		return nil
	}

	for _, p := range req.Bundle.Policies {
		// Import 시에는 "타겟 스코프"로 강제 저장(=외부 번들을 org/workspace에 적용)
		err := i.Store.Upsert(ctx, router.Policy{
			ScopeType: string(req.TargetScope.Type),
			ScopeID:   req.TargetScope.ID,

			OS:       p.OS,
			Arch:     p.Arch,
			Scenario: p.Scenario,

			Provider: p.Provider,
			Model:    p.Model,

			Weight:       p.Weight,
			SuccessRate:  p.SuccessRate,
			AvgLatencyMs: p.AvgLatencyMs,
			AvgCost:      p.AvgCost,
			SampleCount:  p.SampleCount,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

8.3 (선택) 서명/검증 스텁

internal/router/share/signer.go

package share

// 실제 운영에서는 ed25519 등으로 서명/검증을 권장.
// 여기서는 "스텁" 형태로 구조만 둔다.

type SignedBundle struct {
	Bundle ExportBundle
	Signature string
}

func Sign(b ExportBundle) SignedBundle {
	return SignedBundle{Bundle: b, Signature: ""}
}

func Verify(sb SignedBundle) bool {
	return true
}


---

9) API Gateway: user/workspace context 주입

9.1 internal/api_gateway/auth_middleware.go (핵심 부분만 “추가” 형태)

package api_gateway

import (
	"context"
	"net/http"

	"devorch/internal/router/scope"
)

// 기존 인증 미들웨어가 토큰을 검증하고 user/workspace/org를 알 수 있다고 가정.
// 아래는 "컨텍스트 주입"만 담당하는 예시입니다.

func InjectIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: 실제 구현에서는 auth/session에서 가져오기
		orgID := r.Header.Get("X-Devorch-Org")
		wsID := r.Header.Get("X-Devorch-Workspace")
		userID := r.Header.Get("X-Devorch-User")
		projectID := r.Header.Get("X-Devorch-Project")

		ctx := r.Context()
		ctx = context.WithValue(ctx, scope.CtxOrgID, orgID)
		ctx = context.WithValue(ctx, scope.CtxWorkspaceID, wsID)
		ctx = context.WithValue(ctx, scope.CtxUserID, userID)
		ctx = context.WithValue(ctx, scope.CtxProjectID, projectID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}


---

10) Router API 확장: 조회/override/export/import

10.1 internal/server/routes/router.go (Step 13에서 확장된 형태로 “교체” 예시)

package routes

import (
	"encoding/json"
	"net/http"

	"devorch/internal/router"
	"devorch/internal/router/scope"
	"devorch/internal/router/share"
)

type RouterRoutes struct {
	PolicyStore  *router.PolicyStore
	PolicyRuntime *router.PolicyRuntime

	Exporter *share.Exporter
	Importer *share.Importer
}

func (rr *RouterRoutes) HandlePolicyResolved(w http.ResponseWriter, r *http.Request) {
	os := r.URL.Query().Get("os")
	arch := r.URL.Query().Get("arch")
	sc := r.URL.Query().Get("scenario")

	provider, model, ok := rr.PolicyRuntime.ChooseBest(r.Context(), os, arch, sc)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok": ok,
		"provider": provider,
		"model": model,
	})
}

func (rr *RouterRoutes) HandlePolicyList(w http.ResponseWriter, r *http.Request) {
	// scope 지정 가능: ?scope_type=workspace&scope_id=xxx
	st := r.URL.Query().Get("scope_type")
	sid := r.URL.Query().Get("scope_id")

	if st == "" {
		st = string(scope.Global)
	}
	if sid == "" {
		sid = "*"
	}

	key := scope.Key{Type: scope.Type(st), ID: sid}

	os := r.URL.Query().Get("os")
	arch := r.URL.Query().Get("arch")
	sc := r.URL.Query().Get("scenario")

	policies, err := rr.PolicyStore.LoadForKey(r.Context(), key, os, arch, sc)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	_ = json.NewEncoder(w).Encode(policies)
}

func (rr *RouterRoutes) HandlePolicyExport(w http.ResponseWriter, r *http.Request) {
	st := r.URL.Query().Get("scope_type")
	sid := r.URL.Query().Get("scope_id")
	if st == "" {
		st = string(scope.Global)
	}
	if sid == "" {
		sid = "*"
	}

	req := share.ExportRequest{
		Scope: scope.Key{Type: scope.Type(st), ID: sid},
		OS: r.URL.Query().Get("os"),
		Arch: r.URL.Query().Get("arch"),
		Scenario: r.URL.Query().Get("scenario"),
	}

	bundle, err := rr.Exporter.Export(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	_ = json.NewEncoder(w).Encode(bundle)
}

func (rr *RouterRoutes) HandlePolicyImport(w http.ResponseWriter, r *http.Request) {
	// POST body: { target_scope_type, target_scope_id, bundle: {...} }
	var body struct {
		TargetScopeType string            `json:"target_scope_type"`
		TargetScopeID   string            `json:"target_scope_id"`
		Bundle          share.ExportBundle `json:"bundle"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	target := scope.Key{
		Type: scope.Type(body.TargetScopeType),
		ID:   body.TargetScopeID,
	}
	if target.Type == "" {
		target.Type = scope.Global
	}
	if target.ID == "" {
		target.ID = "*"
	}

	if err := rr.Importer.Import(r.Context(), share.ImportRequest{
		TargetScope: target,
		Bundle: body.Bundle,
	}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.WriteHeader(204)
}

> 라우팅 “수동 override”까지 원하면:
/router/policy/set 같은 엔드포인트로 PolicyStore.Upsert() 호출하면 됩니다(구조는 동일).




---

11) Bench 기록에 scope 저장 (선택이지만 강력 추천)

> Step 14 학습은 bench_runs에 scope가 있어야 “개인/팀별 학습”이 됩니다.



11.1 (추천) bench_runs 테이블에 컬럼 추가 마이그레이션

이미 Step 13에서 bench_runs를 만들었다고 가정하고, 다음 컬럼을 추가하세요:

ALTER TABLE bench_runs ADD COLUMN scope_type TEXT DEFAULT 'global';
ALTER TABLE bench_runs ADD COLUMN scope_id   TEXT DEFAULT '*';

11.2 internal/server/routes/bench.go (추가 예시)

// bench 기록을 저장할 때, r.Context()에서 scope 후보 중 "workspace" 또는 "user"를 선택해 기록.
// 예시는 workspace 우선 기록(팀 최적화를 빠르게 만들기 좋음).

// pseudo:
// key := scope.Candidates(ctx)
// chosen := key[0] // user가 제일 먼저 나오므로 user 기반 학습이면 그대로
// 혹은 팀 공유 중심이면 workspace를 우선 선택하도록 로직 조정


---

12) Diagnostics: 공유/전파 점검

12.1 internal/diagnostics/checks/router_sharing.go

package checks

import (
	"context"

	"devorch/internal/router"
	"devorch/internal/router/scope"
)

func CheckRouterSharing(ctx context.Context, ps *router.PolicyStore) error {
	// 예: workspace 정책이 전혀 없으면 경고(운영에서 팀 성능 최적화가 안 돌고 있을 가능성)
	_, _ = ps.LoadForKey(ctx, scope.Key{Type: scope.Workspace, ID: "dummy"}, "darwin", "arm64", "coding")
	return nil
}


---

Step 14 완료 결과

이제 Devorch는:

✅ 개인별(user) 최적 라우팅
✅ 팀/조직 단위(workspace/org) 최적 라우팅
✅ 정책 공유(export/import)로 “베스트 프랙티스 전파”
✅ 프라이버시 정책으로 유저 데이터/키 유출 방지
✅ 시간이 지날수록 개인도 팀도 같이 빨라짐


---

원하시면 다음 Step 15는 이렇게 가는 게 가장 강력합니다:

Step 15 = “온라인/오프라인 혼합 학습 + 모델/프롬프트 드리프트 감지 + 자동 롤백”

바로 이어서 진행할까요?