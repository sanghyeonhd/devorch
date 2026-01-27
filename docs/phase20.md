좋습니다. Step 20 = “정책(Policy) 검증/서명/배포(오프라인 번들)/충돌검사/승인(Policy PR)” 입니다.
Step19가 “승격/롤백”이라면, Step20은 운영·배포·감사·오프라인 반입까지 완성하는 단계입니다.
아래는 Step20 풀코드(Go + SQL) 입니다. (Step20에서 추가/수정되는 파일 전부)
Step 20 소스트리 추가/변경
코드 복사

internal/
├─ policy/
│  ├─ validate.go                 # schema/규칙 검증 + 충돌 검사
│  ├─ signing/
│  │  ├─ keys.go                  # ed25519 키 로딩/관리
│  │  ├─ sign.go                  # 서명 생성/검증
│  │  └─ canonical.go             # canonical JSON(서명 안정화)
│  ├─ bundle/
│  │  ├─ bundle.go                # 정책 번들 구조체
│  │  ├─ export.go                # export(.devorch-policy.tgz)
│  │  ├─ import.go                # import + verify + apply
│  │  └─ manifest.go              # 번들 매니페스트
│  ├─ approval/
│  │  ├─ types.go                 # PR/승인 타입
│  │  ├─ store.go                 # DB 저장소
│  │  └─ workflow.go              # propose → approve → apply
│  └─ apply.go                    # "apply 정책" 공용 함수
│
├─ server/routes/
│  └─ policy_pr.go                # 정책 PR API(제안/승인/적용/번들)
│
├─ cli/
│  ├─ policy.go                   # CLI: policy export/import/verify/pr
│  └─ doctor.go                   # (수정) policy 체크 추가
│
├─ diagnostics/checks/
│  └─ policy_signing.go           # doctor: 서명/키/정책 무결성
│
└─ storage/sqlite/migrations/
   └─ 0010_policy_signing_bundle_pr.sql
1) DB 마이그레이션 (SQL)
internal/storage/sqlite/migrations/0010_policy_signing_bundle_pr.sql
코드 복사
Sql
-- Step 20: policy signing + bundle + approval workflow (policy PR)

CREATE TABLE IF NOT EXISTS policy_keys (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,          -- "org-root", "ci-signing"
  algorithm TEXT NOT NULL,            -- "ed25519"
  public_key_b64 TEXT NOT NULL,
  is_trusted INTEGER NOT NULL DEFAULT 1,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  rotated_at DATETIME
);

-- Each policy version may have 0..n signatures
CREATE TABLE IF NOT EXISTS policy_signatures (
  id TEXT PRIMARY KEY,
  policy_id TEXT NOT NULL,            -- policy_versions.id
  key_id TEXT NOT NULL,               -- policy_keys.id
  signature_b64 TEXT NOT NULL,
  canonical_hash TEXT NOT NULL,        -- sha256 of canonical spec for quick compare
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(policy_id) REFERENCES policy_versions(id),
  FOREIGN KEY(key_id) REFERENCES policy_keys(id)
);

CREATE INDEX IF NOT EXISTS idx_policy_signatures_policy
  ON policy_signatures(policy_id);

-- Policy bundle imports (offline)
CREATE TABLE IF NOT EXISTS policy_bundles (
  id TEXT PRIMARY KEY,
  filename TEXT NOT NULL,
  manifest_json TEXT NOT NULL,
  bundle_hash TEXT NOT NULL,           -- sha256 of tgz bytes
  verified INTEGER NOT NULL DEFAULT 0,
  imported_by TEXT,
  imported_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Policy PR workflow
CREATE TABLE IF NOT EXISTS policy_prs (
  id TEXT PRIMARY KEY,
  scope TEXT NOT NULL,
  domain TEXT NOT NULL,
  name TEXT NOT NULL,
  channel TEXT NOT NULL,
  proposed_spec_json TEXT NOT NULL,
  base_version INTEGER,                -- optional: the version you based changes on
  status TEXT NOT NULL,                -- "open" | "approved" | "rejected" | "applied"
  created_by TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  applied_version INTEGER,
  reason TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_policy_prs_scope_status
  ON policy_prs(scope, domain, name, channel, status);

CREATE TABLE IF NOT EXISTS policy_pr_reviews (
  id TEXT PRIMARY KEY,
  pr_id TEXT NOT NULL,
  reviewer TEXT NOT NULL,
  decision TEXT NOT NULL,              -- "approve" | "reject"
  comment TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(pr_id) REFERENCES policy_prs(id)
);

CREATE INDEX IF NOT EXISTS idx_policy_pr_reviews_pr
  ON policy_pr_reviews(pr_id);
2) 정책 검증 + 충돌 검사
internal/policy/validate.go
코드 복사
Go
package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

// ValidateSpec: schema-like validation (lightweight, no external deps).
func (v *Validator) ValidateSpec(domain Domain, specJSON string) error {
	switch domain {
	case DomainRouter:
		var s RouterPolicySpec
		if err := json.Unmarshal([]byte(specJSON), &s); err != nil {
			return fmt.Errorf("router policy json invalid: %w", err)
		}
		// sanity
		if s.MaxRetries < 0 || s.MaxRetries > 10 {
			return fmt.Errorf("router.max_retries out of range")
		}
		if s.HedgeAfterMs < 0 || s.HedgeAfterMs > 60000 {
			return fmt.Errorf("router.hedge_after_ms out of range")
		}
		if s.Overrides != nil {
			for k, pm := range s.Overrides {
				if !strings.HasPrefix(k, "category:") && !strings.HasPrefix(k, "agent:") && k != "default" {
					return fmt.Errorf("router.overrides key invalid: %s", k)
				}
				if pm.Provider == "" || pm.Model == "" {
					return fmt.Errorf("router.overrides[%s] provider/model required", k)
				}
			}
		}
		return nil

	case DomainModelResolver:
		var s ModelResolverPolicySpec
		if err := json.Unmarshal([]byte(specJSON), &s); err != nil {
			return fmt.Errorf("modelresolver policy json invalid: %w", err)
		}
		// ensure strings not empty
		for k, m := range s.AgentModels {
			if k == "" || m == "" {
				return fmt.Errorf("agent_models invalid empty key/value")
			}
		}
		for k, m := range s.CategoryModels {
			if k == "" || m == "" {
				return fmt.Errorf("category_models invalid empty key/value")
			}
		}
		return nil

	case DomainPrompt:
		var s PromptPolicySpec
		if err := json.Unmarshal([]byte(specJSON), &s); err != nil {
			return fmt.Errorf("prompt policy json invalid: %w", err)
		}
		if strings.TrimSpace(s.SystemPrompt) == "" {
			return fmt.Errorf("system_prompt required")
		}
		return nil

	default:
		return fmt.Errorf("unknown policy domain: %s", domain)
	}
}

// ConflictCheck: detect dangerous conflicts vs current active (optional).
// - Example: Router policy sets a model/provider not allowed by security policy, or references disabled provider.
// Here we implement minimal "structural conflict" check: empty spec, or if change is identical (no-op).
func (v *Validator) ConflictCheck(current *Version, proposedSpec string) (noop bool, err error) {
	if strings.TrimSpace(proposedSpec) == "" {
		return false, fmt.Errorf("proposed spec empty")
	}
	if current != nil && strings.TrimSpace(current.SpecJSON) == strings.TrimSpace(proposedSpec) {
		return true, nil
	}
	return false, nil
}

func ValidateBeforeApply(ctx context.Context, val *Validator, domain Domain, spec string) error {
	if val == nil {
		val = NewValidator()
	}
	return val.ValidateSpec(domain, spec)
}
3) 서명(Signing): canonical JSON + ed25519
internal/policy/signing/canonical.go
코드 복사
Go
package signing

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// CanonicalJSON: stable ordering for maps so signature is deterministic.
func CanonicalJSON(raw string) ([]byte, string, error) {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		// if it's not JSON, sign raw bytes as-is
		b := []byte(raw)
		return b, sha256Hex(b), nil
	}
	b := canonicalEncode(v)
	return b, sha256Hex(b), nil
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// canonicalEncode recursively sorts map keys.
func canonicalEncode(v any) []byte {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.WriteByte(':')
			buf.Write(canonicalEncode(t[k]))
		}
		buf.WriteByte('}')
		return buf.Bytes()

	case []any:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.Write(canonicalEncode(t[i]))
		}
		buf.WriteByte(']')
		return buf.Bytes()

	default:
		b, _ := json.Marshal(t)
		return b
	}
}
internal/policy/signing/keys.go
코드 복사
Go
package signing

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"

	"devorch/internal/storage"
)

type Key struct {
	ID        string
	Name      string
	Algorithm string // "ed25519"
	PublicB64 string
	Trusted   bool
}

type KeyStore struct {
	Storage storage.Storage
}

func NewKeyStore(s storage.Storage) *KeyStore { return &KeyStore{Storage: s} }

func (ks *KeyStore) ListTrusted(ctx context.Context) ([]Key, error) {
	rows, err := ks.Storage.Query(ctx, `
SELECT id, name, algorithm, public_key_b64, is_trusted
FROM policy_keys
WHERE is_trusted = 1
ORDER BY created_at DESC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Key
	for rows.Next() {
		var k Key
		var trusted int
		if err := rows.Scan(&k.ID, &k.Name, &k.Algorithm, &k.PublicB64, &trusted); err != nil {
			return nil, err
		}
		k.Trusted = trusted == 1
		out = append(out, k)
	}
	return out, nil
}

func (k Key) PublicBytes() ([]byte, error) {
	if k.Algorithm != "ed25519" {
		return nil, fmt.Errorf("unsupported algorithm: %s", k.Algorithm)
	}
	b, err := base64.StdEncoding.DecodeString(k.PublicB64)
	if err != nil {
		return nil, fmt.Errorf("public key b64 decode: %w", err)
	}
	return b, nil
}

func (ks *KeyStore) GetByID(ctx context.Context, id string) (Key, bool, error) {
	row := ks.Storage.QueryRow(ctx, `
SELECT id, name, algorithm, public_key_b64, is_trusted
FROM policy_keys WHERE id = ?
`, id)

	var k Key
	var trusted int
	err := row.Scan(&k.ID, &k.Name, &k.Algorithm, &k.PublicB64, &trusted)
	if err == sql.ErrNoRows {
		return Key{}, false, nil
	}
	if err != nil {
		return Key{}, false, err
	}
	k.Trusted = trusted == 1
	return k, true, nil
}
internal/policy/signing/sign.go
코드 복사
Go
package signing

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"

	"devorch/internal/id"
	"devorch/internal/storage"
)

type Signer struct {
	// PrivateKeyB64: stored outside DB (file/env/secret store)
	PrivateKeyB64 string
	KeyID         string // policy_keys.id that corresponds to the pubkey
	Storage       storage.Storage
}

func (s *Signer) SignPolicySpec(ctx context.Context, policyID string, specJSON string) (sigB64 string, hash string, err error) {
	if s.PrivateKeyB64 == "" || s.KeyID == "" {
		return "", "", fmt.Errorf("signer not configured")
	}
	canon, h, err := CanonicalJSON(specJSON)
	if err != nil {
		return "", "", err
	}

	priv, err := base64.StdEncoding.DecodeString(s.PrivateKeyB64)
	if err != nil {
		return "", "", fmt.Errorf("private key decode: %w", err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		return "", "", fmt.Errorf("invalid ed25519 private key size")
	}
	signature := ed25519.Sign(ed25519.PrivateKey(priv), canon)
	sigB64 = base64.StdEncoding.EncodeToString(signature)

	_, err = s.Storage.Exec(ctx, `
INSERT INTO policy_signatures (id, policy_id, key_id, signature_b64, canonical_hash)
VALUES (?, ?, ?, ?, ?)
`, id.NewULID(), policyID, s.KeyID, sigB64, h)
	if err != nil {
		return "", "", err
	}
	return sigB64, h, nil
}

type Verifier struct {
	Keys *KeyStore
}

func (v *Verifier) VerifyAnyTrusted(ctx context.Context, specJSON string, signatures []SignatureRecord) (bool, string, error) {
	trusted, err := v.Keys.ListTrusted(ctx)
	if err != nil {
		return false, "", err
	}
	canon, hash, err := CanonicalJSON(specJSON)
	if err != nil {
		return false, "", err
	}

	// build pubkey map
	pub := map[string]ed25519.PublicKey{}
	for _, k := range trusted {
		b, err := k.PublicBytes()
		if err != nil {
			continue
		}
		if len(b) != ed25519.PublicKeySize {
			continue
		}
		pub[k.ID] = ed25519.PublicKey(b)
	}

	for _, srec := range signatures {
		pk, ok := pub[srec.KeyID]
		if !ok {
			continue
		}
		sig, err := base64.StdEncoding.DecodeString(srec.SignatureB64)
		if err != nil || len(sig) != ed25519.SignatureSize {
			continue
		}
		if ed25519.Verify(pk, canon, sig) {
			return true, hash, nil
		}
	}
	return false, hash, nil
}

type SignatureRecord struct {
	KeyID        string
	SignatureB64 string
	Hash         string
}
4) 정책 “적용(apply)” 공용 함수
internal/policy/apply.go
코드 복사
Go
package policy

import (
	"context"
	"fmt"

	"devorch/internal/policy/signing"
)

type ApplyService struct {
	Store     *Store
	Validator *Validator
	Verifier  *signing.Verifier

	// require signatures in certain environments
	RequireSignature bool
}

func NewApplyService(st *Store, val *Validator, ver *signing.Verifier) *ApplyService {
	return &ApplyService{Store: st, Validator: val, Verifier: ver}
}

// ApplySpec creates a new draft version, optionally signs (caller), and activates it.
func (a *ApplyService) ApplySpec(ctx context.Context, v Version, reason, actor string, sigs []signing.SignatureRecord) (Version, error) {
	if err := ValidateBeforeApply(ctx, a.Validator, v.Domain, v.SpecJSON); err != nil {
		return Version{}, err
	}

	created, err := a.Store.CreateDraft(ctx, v, "apply: "+reason, actor)
	if err != nil {
		return Version{}, err
	}

	if a.RequireSignature {
		ok, _, err := a.Verifier.VerifyAnyTrusted(ctx, created.SpecJSON, sigs)
		if err != nil {
			return Version{}, err
		}
		if !ok {
			return Version{}, fmt.Errorf("signature required but not valid")
		}
	}

	if err := a.Store.Activate(ctx, created.Scope, created.Domain, created.Name, created.Channel, created.Version, reason, actor); err != nil {
		return Version{}, err
	}
	return created, nil
}
5) 정책 번들(Bundle) export/import (오프라인 반입)
번들은 tgz(tar+gzip) 하나로 내보내고, 안에는:
manifest.json
policies/{scope}/{domain}/{name}/{channel}/{version}.json
signatures/{policyID}.json (policy_signatures dump)
internal/policy/bundle/bundle.go
코드 복사
Go
package bundle

type Manifest struct {
	BundleVersion int    `json:"bundle_version"`
	CreatedAt     string `json:"created_at"`
	CreatedBy     string `json:"created_by"`

	Scope   string `json:"scope"`
	Channel string `json:"channel"`

	Policies []PolicyEntry `json:"policies"`
}

type PolicyEntry struct {
	PolicyID string `json:"policy_id"`
	Scope    string `json:"scope"`
	Domain   string `json:"domain"`
	Name     string `json:"name"`
	Channel  string `json:"channel"`
	Version  int    `json:"version"`
	Hash     string `json:"canonical_hash"`
	Source   string `json:"source"`
}

type SignatureDump struct {
	PolicyID string          `json:"policy_id"`
	Signatures []SignatureRow `json:"signatures"`
}

type SignatureRow struct {
	KeyID        string `json:"key_id"`
	SignatureB64 string `json:"signature_b64"`
	CanonicalHash string `json:"canonical_hash"`
	CreatedAt    string `json:"created_at"`
}
internal/policy/bundle/manifest.go
코드 복사
Go
package bundle

import (
	"encoding/json"
	"time"
)

func NewManifest(scope, channel, createdBy string) Manifest {
	return Manifest{
		BundleVersion: 1,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		CreatedBy:     createdBy,
		Scope:         scope,
		Channel:       channel,
	}
}

func MustJSON(v any) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}
internal/policy/bundle/export.go
코드 복사
Go
package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"time"

	"devorch/internal/policy"
	"devorch/internal/policy/signing"
	"devorch/internal/storage"
)

type Exporter struct {
	Storage storage.Storage
}

func NewExporter(s storage.Storage) *Exporter { return &Exporter{Storage: s} }

type ExportOptions struct {
	Scope   string
	Channel policy.Channel
	CreatedBy string

	// If true, export only active pointers; else export all versions for that scope+channel
	ActiveOnly bool
}

func (e *Exporter) ExportTGZ(ctx context.Context, opt ExportOptions) (tgz []byte, bundleHash string, man Manifest, err error) {
	if opt.Scope == "" {
		opt.Scope = "global"
	}
	if opt.Channel == "" {
		opt.Channel = policy.ChannelStable
	}
	man = NewManifest(opt.Scope, string(opt.Channel), opt.CreatedBy)

	// gather policies
	entries, sigDump, err := e.collect(ctx, opt)
	if err != nil {
		return nil, "", Manifest{}, err
	}
	man.Policies = entries

	// build tar.gz
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)

	// manifest
	if err := writeTarFile(tw, "manifest.json", MustJSON(man), time.Now()); err != nil {
		return nil, "", Manifest{}, err
	}

	// policies
	for _, pe := range entries {
		policyPath := path.Join("policies", pe.Scope, pe.Domain, pe.Name, pe.Channel, fmt.Sprintf("%d.json", pe.Version))
		body := []byte(pe.Source) // temporary placeholder; overwrite below
		_ = body
		// load spec json for this policy id
		spec, src, err := e.loadPolicySpecByID(ctx, pe.PolicyID)
		if err != nil {
			return nil, "", Manifest{}, err
		}
		_ = src
		body = []byte(spec)
		if err := writeTarFile(tw, policyPath, body, time.Now()); err != nil {
			return nil, "", Manifest{}, err
		}
	}

	// signatures dump
	for pid, dump := range sigDump {
		p := path.Join("signatures", pid+".json")
		b, _ := json.MarshalIndent(dump, "", "  ")
		if err := writeTarFile(tw, p, b, time.Now()); err != nil {
			return nil, "", Manifest{}, err
		}
	}

	_ = tw.Close()
	_ = gz.Close()

	sum := sha256.Sum256(raw.Bytes())
	return raw.Bytes(), hex.EncodeToString(sum[:]), man, nil
}

func (e *Exporter) collect(ctx context.Context, opt ExportOptions) ([]PolicyEntry, map[string]SignatureDump, error) {
	var rows *storage.Rows
	var err error

	if opt.ActiveOnly {
		rows, err = e.Storage.Query(ctx, `
SELECT pv.id, pv.scope, pv.domain, pv.name, pv.channel, pv.version, COALESCE(ps.canonical_hash,''), pv.source
FROM policy_pointers pp
JOIN policy_versions pv ON pv.scope=pp.scope AND pv.domain=pp.domain AND pv.name=pp.name AND pv.channel=pp.channel AND pv.version=pp.active_version
LEFT JOIN policy_signatures ps ON ps.policy_id = pv.id
WHERE pv.scope=? AND pv.channel=?
`, opt.Scope, string(opt.Channel))
	} else {
		rows, err = e.Storage.Query(ctx, `
SELECT pv.id, pv.scope, pv.domain, pv.name, pv.channel, pv.version, COALESCE(ps.canonical_hash,''), pv.source
FROM policy_versions pv
LEFT JOIN policy_signatures ps ON ps.policy_id = pv.id
WHERE pv.scope=? AND pv.channel=?
ORDER BY pv.domain, pv.name, pv.version
`, opt.Scope, string(opt.Channel))
	}
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var entries []PolicyEntry
	sigDump := map[string]SignatureDump{}

	for rows.Next() {
		var pe PolicyEntry
		if err := rows.Scan(&pe.PolicyID, &pe.Scope, &pe.Domain, &pe.Name, &pe.Channel, &pe.Version, &pe.Hash, &pe.Source); err != nil {
			return nil, nil, err
		}
		entries = append(entries, pe)
		// signature dump for this policy id
		dump, err := e.loadSignatureDump(ctx, pe.PolicyID)
		if err != nil {
			return nil, nil, err
		}
		sigDump[pe.PolicyID] = dump
	}
	return entries, sigDump, nil
}

func (e *Exporter) loadPolicySpecByID(ctx context.Context, policyID string) (spec string, source string, err error) {
	row := e.Storage.QueryRow(ctx, `SELECT spec_json, source FROM policy_versions WHERE id=?`, policyID)
	if err := row.Scan(&spec, &source); err != nil {
		return "", "", err
	}
	return spec, source, nil
}

func (e *Exporter) loadSignatureDump(ctx context.Context, policyID string) (SignatureDump, error) {
	rows, err := e.Storage.Query(ctx, `
SELECT key_id, signature_b64, canonical_hash, created_at
FROM policy_signatures
WHERE policy_id=?
ORDER BY created_at ASC
`, policyID)
	if err != nil {
		return SignatureDump{}, err
	}
	defer rows.Close()

	d := SignatureDump{PolicyID: policyID}
	for rows.Next() {
		var r SignatureRow
		if err := rows.Scan(&r.KeyID, &r.SignatureB64, &r.CanonicalHash, &r.CreatedAt); err != nil {
			return SignatureDump{}, err
		}
		d.Signatures = append(d.Signatures, r)
	}
	return d, nil
}

func writeTarFile(tw *tar.Writer, name string, data []byte, mod time.Time) error {
	h := &tar.Header{
		Name:    name,
		Mode:    0o644,
		Size:    int64(len(data)),
		ModTime: mod,
	}
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	_, err := io.Copy(tw, bytes.NewReader(data))
	return err
}

// keep unused import reference safe for future extension
var _ = signing.CanonicalJSON
internal/policy/bundle/import.go
코드 복사
Go
package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"devorch/internal/id"
	"devorch/internal/policy"
	"devorch/internal/policy/signing"
	"devorch/internal/storage"
)

type Importer struct {
	Storage storage.Storage
	Validator *policy.Validator
	Verifier *signing.Verifier
}

func NewImporter(s storage.Storage, val *policy.Validator, ver *signing.Verifier) *Importer {
	return &Importer{Storage: s, Validator: val, Verifier: ver}
}

type ImportOptions struct {
	Apply bool              // if true: activate imported policies (only those in manifest)
	RequireSignature bool   // must verify at least one trusted signature (recommended in offline)
	Actor string
}

func (im *Importer) ImportTGZ(ctx context.Context, filename string, tgz []byte, opt ImportOptions) (bundleID string, verified bool, err error) {
	sum := sha256.Sum256(tgz)
	hash := hex.EncodeToString(sum[:])

	// parse tgz
	man, files, sigs, err := untar(tgz)
	if err != nil {
		return "", false, err
	}

	// store bundle record
	bundleID = id.NewULID()
	_, err = im.Storage.Exec(ctx, `
INSERT INTO policy_bundles (id, filename, manifest_json, bundle_hash, verified, imported_by)
VALUES (?, ?, ?, ?, 0, ?)
`, bundleID, filename, string(MustJSON(man)), hash, nullIfEmpty(opt.Actor))
	if err != nil {
		return "", false, err
	}

	// verify each policy spec vs signature dumps + trusted keys
	for _, pe := range man.Policies {
		specKey := policyFileKey(pe)
		spec, ok := files[specKey]
		if !ok {
			return bundleID, false, fmt.Errorf("missing policy file: %s", specKey)
		}
		if err := im.Validator.ValidateSpec(policy.Domain(pe.Domain), string(spec)); err != nil {
			return bundleID, false, fmt.Errorf("policy spec invalid (%s): %w", specKey, err)
		}

		// signature validation (if required)
		if opt.RequireSignature {
			dump, ok := sigs[pe.PolicyID]
			if !ok {
				return bundleID, false, fmt.Errorf("missing signature dump for policy_id=%s", pe.PolicyID)
			}
			var recs []signing.SignatureRecord
			for _, s := range dump.Signatures {
				recs = append(recs, signing.SignatureRecord{
					KeyID: s.KeyID, SignatureB64: s.SignatureB64, Hash: s.CanonicalHash,
				})
			}
			okv, chash, err := im.Verifier.VerifyAnyTrusted(ctx, string(spec), recs)
			if err != nil {
				return bundleID, false, err
			}
			if !okv {
				return bundleID, false, fmt.Errorf("signature verification failed for policy_id=%s (hash=%s)", pe.PolicyID, chash)
			}
		}
	}

	// mark verified
	_, _ = im.Storage.Exec(ctx, `UPDATE policy_bundles SET verified=1 WHERE id=?`, bundleID)
	verified = true

	// optionally apply: insert as new local versions and activate per manifest
	if opt.Apply {
		for _, pe := range man.Policies {
			specKey := policyFileKey(pe)
			spec := string(files[specKey])
			// create draft locally with source "bundle:<hash>"
			v := policy.Version{
				Scope: pe.Scope, Domain: policy.Domain(pe.Domain), Name: pe.Name,
				Channel: policy.Channel(pe.Channel), Status: policy.StatusDraft,
				SpecJSON: spec, Source: "bundle:" + hash,
			}
			// use policy.Store minimally here to avoid circular imports: do DB inserts directly
			created, err := createPolicyDraftDirect(ctx, im.Storage, v)
			if err != nil {
				return bundleID, true, err
			}
			if err := activatePolicyDirect(ctx, im.Storage, created.Scope, created.Domain, created.Name, created.Channel, created.Version); err != nil {
				return bundleID, true, err
			}
		}
	}
	return bundleID, verified, nil
}

func policyFileKey(pe PolicyEntry) string {
	return "policies/" + pe.Scope + "/" + pe.Domain + "/" + pe.Name + "/" + pe.Channel + "/" + itoa(pe.Version) + ".json"
}

func untar(tgz []byte) (Manifest, map[string][]byte, map[string]SignatureDump, error) {
	gr, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		return Manifest{}, nil, nil, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	files := map[string][]byte{}
	sigs := map[string]SignatureDump{}
	var man Manifest

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Manifest{}, nil, nil, err
		}
		if h.Size <= 0 {
			continue
		}
		b := make([]byte, h.Size)
		if _, err := io.ReadFull(tr, b); err != nil {
			return Manifest{}, nil, nil, err
		}
		name := strings.TrimPrefix(h.Name, "./")
		files[name] = b

		if name == "manifest.json" {
			_ = json.Unmarshal(b, &man)
		}
		if strings.HasPrefix(name, "signatures/") && strings.HasSuffix(name, ".json") {
			var d SignatureDump
			_ = json.Unmarshal(b, &d)
			if d.PolicyID != "" {
				sigs[d.PolicyID] = d
			}
		}
	}
	if man.BundleVersion == 0 {
		return Manifest{}, nil, nil, fmt.Errorf("manifest missing/invalid")
	}
	return man, files, sigs, nil
}

// ---- direct db helpers (keep importer standalone) ----

func createPolicyDraftDirect(ctx context.Context, s storage.Storage, v policy.Version) (policy.Version, error) {
	// compute next version
	row := s.QueryRow(ctx, `
SELECT COALESCE(MAX(version), 0) + 1
FROM policy_versions
WHERE scope=? AND domain=? AND name=?
`, v.Scope, string(v.Domain), v.Name)
	var next int
	if err := row.Scan(&next); err != nil {
		return policy.Version{}, err
	}
	v.Version = next
	v.ID = id.NewULID()
	if v.Channel == "" {
		v.Channel = policy.ChannelStable
	}
	if v.Status == "" {
		v.Status = policy.StatusDraft
	}
	if v.Source == "" {
		v.Source = "bundle"
	}

	_, err := s.Exec(ctx, `
INSERT INTO policy_versions
(id, scope, domain, name, version, status, channel, spec_json, source)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`, v.ID, v.Scope, string(v.Domain), v.Name, v.Version, string(v.Status), string(v.Channel), v.SpecJSON, v.Source)
	return v, err
}

func activatePolicyDirect(ctx context.Context, s storage.Storage, scope string, domain policy.Domain, name string, channel policy.Channel, version int) error {
	_, _ = s.Exec(ctx, `
UPDATE policy_versions SET status='archived', archived_at=CURRENT_TIMESTAMP
WHERE scope=? AND domain=? AND name=? AND channel=? AND status='active'
`, scope, string(domain), name, string(channel))

	_, err := s.Exec(ctx, `
UPDATE policy_versions SET status='active', activated_at=CURRENT_TIMESTAMP
WHERE scope=? AND domain=? AND name=? AND channel=? AND version=?
`, scope, string(domain), name, string(channel), version)
	if err != nil {
		return err
	}

	_, err = s.Exec(ctx, `
INSERT INTO policy_pointers (scope, domain, name, channel, active_version)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(scope,domain,name,channel) DO UPDATE SET
active_version=excluded.active_version,
updated_at=CURRENT_TIMESTAMP
`, scope, string(domain), name, string(channel), version)
	return err
}

func nullIfEmpty(s string) any {
	if s == "" { return nil }
	return s
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
6) 정책 PR(승인 워크플로우)
internal/policy/approval/types.go
코드 복사
Go
package approval

import "devorch/internal/policy"

type PRStatus string

const (
	PROpen     PRStatus = "open"
	PRApproved PRStatus = "approved"
	PRRejected PRStatus = "rejected"
	PRApplied  PRStatus = "applied"
)

type PolicyPR struct {
	ID      string
	Scope   string
	Domain  policy.Domain
	Name    string
	Channel policy.Channel

	ProposedSpecJSON string
	BaseVersion      int
	Status           PRStatus
	CreatedBy        string
	Reason           string
	AppliedVersion   int
	CreatedAt        string
}

type Review struct {
	ID       string
	PRID     string
	Reviewer string
	Decision string // approve|reject
	Comment  string
	CreatedAt string
}
internal/policy/approval/store.go
코드 복사
Go
package approval

import (
	"context"
	"database/sql"

	"devorch/internal/id"
	"devorch/internal/policy"
	"devorch/internal/storage"
)

type Store struct {
	Storage storage.Storage
}

func NewStore(s storage.Storage) *Store { return &Store{Storage: s} }

func (st *Store) CreatePR(ctx context.Context, pr PolicyPR) (PolicyPR, error) {
	if pr.ID == "" { pr.ID = id.NewULID() }
	if pr.Status == "" { pr.Status = PROpen }

	_, err := st.Storage.Exec(ctx, `
INSERT INTO policy_prs
(id, scope, domain, name, channel, proposed_spec_json, base_version, status, created_by, reason)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, pr.ID, pr.Scope, string(pr.Domain), pr.Name, string(pr.Channel), pr.ProposedSpecJSON, nullIfZero(pr.BaseVersion), string(pr.Status), nullIfEmpty(pr.CreatedBy), pr.Reason)
	if err != nil { return PolicyPR{}, err }
	return pr, nil
}

func (st *Store) GetPR(ctx context.Context, id string) (PolicyPR, bool, error) {
	row := st.Storage.QueryRow(ctx, `
SELECT id, scope, domain, name, channel, proposed_spec_json, COALESCE(base_version,0),
       status, COALESCE(created_by,''), COALESCE(applied_version,0), reason, created_at
FROM policy_prs WHERE id=?
`, id)

	var pr PolicyPR
	var dom, ch, stt string
	err := row.Scan(&pr.ID, &pr.Scope, &dom, &pr.Name, &ch, &pr.ProposedSpecJSON, &pr.BaseVersion,
		&stt, &pr.CreatedBy, &pr.AppliedVersion, &pr.Reason, &pr.CreatedAt)
	if err == sql.ErrNoRows { return PolicyPR{}, false, nil }
	if err != nil { return PolicyPR{}, false, err }
	pr.Domain = policy.Domain(dom)
	pr.Channel = policy.Channel(ch)
	pr.Status = PRStatus(stt)
	return pr, true, nil
}

func (st *Store) AddReview(ctx context.Context, prID, reviewer, decision, comment string) (Review, error) {
	r := Review{ID: id.NewULID(), PRID: prID, Reviewer: reviewer, Decision: decision, Comment: comment}
	_, err := st.Storage.Exec(ctx, `
INSERT INTO policy_pr_reviews (id, pr_id, reviewer, decision, comment)
VALUES (?, ?, ?, ?, ?)
`, r.ID, r.PRID, r.Reviewer, r.Decision, nullIfEmpty(r.Comment))
	return r, err
}

func (st *Store) UpdateStatus(ctx context.Context, prID string, status PRStatus) error {
	_, err := st.Storage.Exec(ctx, `UPDATE policy_prs SET status=? WHERE id=?`, string(status), prID)
	return err
}

func (st *Store) MarkApplied(ctx context.Context, prID string, version int) error {
	_, err := st.Storage.Exec(ctx, `UPDATE policy_prs SET status='applied', applied_version=? WHERE id=?`, version, prID)
	return err
}

func nullIfEmpty(s string) any {
	if s == "" { return nil }
	return s
}
func nullIfZero(i int) any {
	if i == 0 { return nil }
	return i
}
internal/policy/approval/workflow.go
코드 복사
Go
package approval

import (
	"context"
	"fmt"

	"devorch/internal/policy"
)

type Workflow struct {
	PRs   *Store
	Apply *policy.ApplyService
	Val   *policy.Validator
	PolicyStore *policy.Store
}

func NewWorkflow(prs *Store, ap *policy.ApplyService, val *policy.Validator, ps *policy.Store) *Workflow {
	return &Workflow{PRs: prs, Apply: ap, Val: val, PolicyStore: ps}
}

// Rule: 1 approve => approved (simple). (엔터프라이즈에서는 N-of-M, role 기반으로 확장)
func (wf *Workflow) Approve(ctx context.Context, prID, reviewer, comment string) error {
	pr, ok, err := wf.PRs.GetPR(ctx, prID)
	if err != nil { return err }
	if !ok { return fmt.Errorf("pr not found") }
	if pr.Status != PROpen { return fmt.Errorf("pr not open") }

	if _, err := wf.PRs.AddReview(ctx, prID, reviewer, "approve", comment); err != nil {
		return err
	}
	return wf.PRs.UpdateStatus(ctx, prID, PRApproved)
}

func (wf *Workflow) Reject(ctx context.Context, prID, reviewer, comment string) error {
	pr, ok, err := wf.PRs.GetPR(ctx, prID)
	if err != nil { return err }
	if !ok { return fmt.Errorf("pr not found") }
	if pr.Status != PROpen { return fmt.Errorf("pr not open") }

	if _, err := wf.PRs.AddReview(ctx, prID, reviewer, "reject", comment); err != nil {
		return err
	}
	return wf.PRs.UpdateStatus(ctx, prID, PRRejected)
}

func (wf *Workflow) ApplyApproved(ctx context.Context, prID, actor string) (int, error) {
	pr, ok, err := wf.PRs.GetPR(ctx, prID)
	if err != nil { return 0, err }
	if !ok { return 0, fmt.Errorf("pr not found") }
	if pr.Status != PRApproved { return 0, fmt.Errorf("pr not approved") }

	// current active for conflict/no-op check
	cur, found, err := wf.PolicyStore.GetActive(ctx, pr.Scope, pr.Domain, pr.Name, pr.Channel)
	if err != nil { return 0, err }
	var curPtr *policy.Version
	if found { curPtr = &cur }

	noop, err := wf.Val.ConflictCheck(curPtr, pr.ProposedSpecJSON)
	if err != nil { return 0, err }
	if noop {
		// still mark applied but version unchanged (0)
		_ = wf.PRs.MarkApplied(ctx, prID, 0)
		return 0, nil
	}

	created, err := wf.Apply.ApplySpec(ctx, policy.Version{
		Scope: pr.Scope, Domain: pr.Domain, Name: pr.Name, Channel: pr.Channel,
		Status: policy.StatusDraft, SpecJSON: pr.ProposedSpecJSON, Source: "manual",
	}, "policy PR apply: "+pr.Reason, actor, nil)
	if err != nil { return 0, err }

	if err := wf.PRs.MarkApplied(ctx, prID, created.Version); err != nil {
		return created.Version, err
	}
	return created.Version, nil
}
7) Policy PR + Bundle API Routes
internal/server/routes/policy_pr.go
코드 복사
Go
package routes

import (
	"net/http"

	"devorch/internal/policy"
	"devorch/internal/policy/approval"
	"devorch/internal/policy/bundle"
)

type PolicyPRRoutes struct {
	PRWorkflow *approval.Workflow
	PRStore    *approval.Store
	Exporter   *bundle.Exporter
	Importer   *bundle.Importer
	PolicyStore *policy.Store
}

func NewPolicyPRRoutes(wf *approval.Workflow, prs *approval.Store, ex *bundle.Exporter, im *bundle.Importer, ps *policy.Store) *PolicyPRRoutes {
	return &PolicyPRRoutes{PRWorkflow: wf, PRStore: prs, Exporter: ex, Importer: im, PolicyStore: ps}
}

func (r *PolicyPRRoutes) Register(mux Mux) {
	mux.POST("/policy/pr/create", r.createPR)
	mux.POST("/policy/pr/approve", r.approvePR)
	mux.POST("/policy/pr/reject", r.rejectPR)
	mux.POST("/policy/pr/apply", r.applyPR)

	mux.POST("/policy/bundle/export", r.exportBundle)
	mux.POST("/policy/bundle/import", r.importBundle)
}

func (r *PolicyPRRoutes) createPR(w http.ResponseWriter, req *http.Request) {
	var in struct {
		Scope string `json:"scope"`
		Domain string `json:"domain"`
		Name string `json:"name"`
		Channel string `json:"channel"`
		BaseVersion int `json:"base_version"`
		ProposedSpecJSON string `json:"proposed_spec_json"`
		Reason string `json:"reason"`
		CreatedBy string `json:"created_by"`
	}
	if err := decodeJSON(req, &in); err != nil { writeErr(w, 400, err); return }
	if in.Scope == "" { in.Scope = "global" }
	if in.Channel == "" { in.Channel = string(policy.ChannelStable) }
	if in.Reason == "" { in.Reason = "policy change" }

	pr, err := r.PRStore.CreatePR(req.Context(), approval.PolicyPR{
		Scope: in.Scope,
		Domain: policy.Domain(in.Domain),
		Name: in.Name,
		Channel: policy.Channel(in.Channel),
		BaseVersion: in.BaseVersion,
		ProposedSpecJSON: in.ProposedSpecJSON,
		Status: approval.PROpen,
		CreatedBy: in.CreatedBy,
		Reason: in.Reason,
	})
	if err != nil { writeErr(w, 500, err); return }
	writeJSON(w, 200, map[string]any{"ok": true, "pr_id": pr.ID})
}

func (r *PolicyPRRoutes) approvePR(w http.ResponseWriter, req *http.Request) {
	var in struct {
		PRID string `json:"pr_id"`
		Reviewer string `json:"reviewer"`
		Comment string `json:"comment"`
	}
	if err := decodeJSON(req, &in); err != nil { writeErr(w, 400, err); return }
	if err := r.PRWorkflow.Approve(req.Context(), in.PRID, in.Reviewer, in.Comment); err != nil {
		writeErr(w, 500, err); return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (r *PolicyPRRoutes) rejectPR(w http.ResponseWriter, req *http.Request) {
	var in struct {
		PRID string `json:"pr_id"`
		Reviewer string `json:"reviewer"`
		Comment string `json:"comment"`
	}
	if err := decodeJSON(req, &in); err != nil { writeErr(w, 400, err); return }
	if err := r.PRWorkflow.Reject(req.Context(), in.PRID, in.Reviewer, in.Comment); err != nil {
		writeErr(w, 500, err); return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (r *PolicyPRRoutes) applyPR(w http.ResponseWriter, req *http.Request) {
	var in struct {
		PRID string `json:"pr_id"`
		Actor string `json:"actor"`
	}
	if err := decodeJSON(req, &in); err != nil { writeErr(w, 400, err); return }
	v, err := r.PRWorkflow.ApplyApproved(req.Context(), in.PRID, in.Actor)
	if err != nil { writeErr(w, 500, err); return }
	writeJSON(w, 200, map[string]any{"ok": true, "applied_version": v})
}

func (r *PolicyPRRoutes) exportBundle(w http.ResponseWriter, req *http.Request) {
	var in struct {
		Scope string `json:"scope"`
		Channel string `json:"channel"`
		CreatedBy string `json:"created_by"`
		ActiveOnly bool `json:"active_only"`
	}
	if err := decodeJSON(req, &in); err != nil { writeErr(w, 400, err); return }
	if in.Scope == "" { in.Scope = "global" }
	ch := policy.Channel(in.Channel)
	if ch == "" { ch = policy.ChannelStable }

	tgz, hash, man, err := r.Exporter.ExportTGZ(req.Context(), bundle.ExportOptions{
		Scope: in.Scope, Channel: ch, CreatedBy: in.CreatedBy, ActiveOnly: in.ActiveOnly,
	})
	if err != nil { writeErr(w, 500, err); return }

	// return as base64 to keep API simple
	writeJSON(w, 200, map[string]any{
		"ok": true,
		"bundle_hash": hash,
		"manifest": man,
		"tgz_b64": b64(tgz),
	})
}

func (r *PolicyPRRoutes) importBundle(w http.ResponseWriter, req *http.Request) {
	var in struct {
		Filename string `json:"filename"`
		TGZB64 string `json:"tgz_b64"`
		Apply bool `json:"apply"`
		RequireSignature bool `json:"require_signature"`
		Actor string `json:"actor"`
	}
	if err := decodeJSON(req, &in); err != nil { writeErr(w, 400, err); return }
	tgz, err := b64decode(in.TGZB64)
	if err != nil { writeErr(w, 400, err); return }

	id, verified, err := r.Importer.ImportTGZ(req.Context(), in.Filename, tgz, bundle.ImportOptions{
		Apply: in.Apply,
		RequireSignature: in.RequireSignature,
		Actor: in.Actor,
	})
	if err != nil { writeErr(w, 500, err); return }
	writeJSON(w, 200, map[string]any{"ok": true, "bundle_id": id, "verified": verified})
}
b64, b64decode는 기존 routes 유틸에 없으면 아래 헬퍼를 routes/_util.go 등에 추가하세요.
(필요 시) internal/server/routes/_b64.go
코드 복사
Go
package routes

import "encoding/base64"

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
func b64decode(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }
8) CLI: policy export/import/verify/pr
internal/cli/policy.go
코드 복사
Go
package cli

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"devorch/internal/policy"
	"devorch/internal/policy/approval"
	"devorch/internal/policy/bundle"
	"devorch/internal/policy/signing"
)

type PolicyCmd struct {
	Store *policy.Store
	Apply *policy.ApplyService
	Promoter *policy.PromotionService

	PRs *approval.Store
	WF  *approval.Workflow

	Exporter *bundle.Exporter
	Importer *bundle.Importer

	Signer *signing.Signer
}

func (c *PolicyCmd) Register(root *Root) {
	root.Add("policy", "policy management", func(ctx context.Context, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: devorch policy <export|import|sign|pr-create|pr-approve|pr-apply>")
		}
		switch args[0] {
		case "export":
			// devorch policy export --scope global --channel stable --out bundle.tgz
			scope := flagVal(args, "--scope", "global")
			ch := policy.Channel(flagVal(args, "--channel", "stable"))
			out := flagVal(args, "--out", "policy-bundle.tgz")
			activeOnly := hasFlag(args, "--active-only")

			tgz, hash, man, err := c.Exporter.ExportTGZ(ctx, bundle.ExportOptions{
				Scope: scope, Channel: ch, CreatedBy: os.Getenv("USER"), ActiveOnly: activeOnly,
			})
			if err != nil { return err }
			if err := os.WriteFile(out, tgz, 0o644); err != nil { return err }
			fmt.Printf("exported %s (hash=%s) policies=%d\n", out, hash, len(man.Policies))
			return nil

		case "import":
			// devorch policy import --file bundle.tgz --apply --require-signature
			file := flagVal(args, "--file", "")
			if file == "" { return fmt.Errorf("--file required") }
			apply := hasFlag(args, "--apply")
			reqSig := hasFlag(args, "--require-signature")
			b, err := os.ReadFile(file)
			if err != nil { return err }
			id, verified, err := c.Importer.ImportTGZ(ctx, file, b, bundle.ImportOptions{
				Apply: apply, RequireSignature: reqSig, Actor: os.Getenv("USER"),
			})
			if err != nil { return err }
			fmt.Printf("imported bundle id=%s verified=%v\n", id, verified)
			return nil

		case "sign":
			// devorch policy sign --policy-id <id>
			pid := flagVal(args, "--policy-id", "")
			if pid == "" { return fmt.Errorf("--policy-id required") }
			// load spec
			spec, err := loadPolicySpec(ctx, c.Store, pid)
			if err != nil { return err }
			sig, hash, err := c.Signer.SignPolicySpec(ctx, pid, spec)
			if err != nil { return err }
			fmt.Printf("signed policy=%s hash=%s sig(b64)=%s\n", pid, hash, sig)
			return nil

		case "pr-create":
			// devorch policy pr-create --scope global --domain router --name default-routing --channel stable --spec-file x.json --reason "..."
			scope := flagVal(args, "--scope", "global")
			dom := policy.Domain(flagVal(args, "--domain", "router"))
			name := flagVal(args, "--name", "")
			ch := policy.Channel(flagVal(args, "--channel", "stable"))
			sf := flagVal(args, "--spec-file", "")
			reason := flagVal(args, "--reason", "policy change")
			if name == "" || sf == "" { return fmt.Errorf("--name and --spec-file required") }
			spec, err := os.ReadFile(sf)
			if err != nil { return err }

			pr, err := c.PRs.CreatePR(ctx, approval.PolicyPR{
				Scope: scope, Domain: dom, Name: name, Channel: ch,
				ProposedSpecJSON: string(spec), Status: approval.PROpen,
				CreatedBy: os.Getenv("USER"), Reason: reason,
			})
			if err != nil { return err }
			fmt.Printf("created pr=%s\n", pr.ID)
			return nil

		case "pr-approve":
			prid := flagVal(args, "--pr-id", "")
			if prid == "" { return fmt.Errorf("--pr-id required") }
			if err := c.WF.Approve(ctx, prid, os.Getenv("USER"), ""); err != nil { return err }
			fmt.Printf("approved pr=%s\n", prid)
			return nil

		case "pr-apply":
			prid := flagVal(args, "--pr-id", "")
			if prid == "" { return fmt.Errorf("--pr-id required") }
			v, err := c.WF.ApplyApproved(ctx, prid, os.Getenv("USER"))
			if err != nil { return err }
			fmt.Printf("applied pr=%s version=%d\n", prid, v)
			return nil

		case "export-b64":
			// helper for airgap transfer
			scope := flagVal(args, "--scope", "global")
			ch := policy.Channel(flagVal(args, "--channel", "stable"))
			tgz, hash, _, err := c.Exporter.ExportTGZ(ctx, bundle.ExportOptions{Scope: scope, Channel: ch, CreatedBy: os.Getenv("USER"), ActiveOnly: true})
			if err != nil { return err }
			fmt.Printf("hash=%s\n%s\n", hash, base64.StdEncoding.EncodeToString(tgz))
			return nil

		default:
			return fmt.Errorf("unknown policy subcommand: %s", args[0])
		}
	})
}

// minimal arg parsing helpers
func flagVal(args []string, k string, def string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == k {
			return args[i+1]
		}
	}
	return def
}
func hasFlag(args []string, k string) bool {
	for _, a := range args {
		if a == k { return true }
	}
	return false
}

func loadPolicySpec(ctx context.Context, st *policy.Store, policyID string) (string, error) {
	// direct query via store.Storage is not exposed; simplest: add method, but keep local helper:
	// NOTE: you can add st.Get