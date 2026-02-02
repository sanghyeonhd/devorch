아래는 **1단계 “완성본”**을 파일별 풀 코드 + SQL(migrations) 포함해서 누락 없이 드립니다.
목표: **devorch / devorchd 빌드 + SQLite 마이그레이션 + provider health/models + router+OkAON 기록(최소)**까지 동작.

> ✅ 의도적으로 “외부 의존성 최소화” (CLI도 표준 flag 기반)
✅ SQLite는 CGO 없이 동작하는 modernc.org/sqlite 사용
✅ OpenAI / OpenRouter는 OpenAI-compatible 클라이언트로 구현
✅ OkAON(로컬 실제 벤치/품질 추정) 테이블에 실행 결과 저장




---

0) 1단계 최소 트리 (이번에 제공하는 범위)

devorch/
├─ go.mod
├─ go.sum                       # (생성됨)
├─ cmd/
│  ├─ devorch/main.go
│  └─ devorchd/main.go
├─ internal/
│  ├─ app/app.go
│  ├─ app/wire.go
│  ├─ config/config.go
│  ├─ config/load.go
│  ├─ global/paths.go
│  ├─ log/log.go
│  ├─ provider/provider.go
│  ├─ provider/registry.go
│  ├─ provider/errors.go
│  ├─ provider/openai_compat/client.go
│  ├─ provider/openai/openai.go
│  ├─ provider/openrouter/openrouter.go
│  ├─ router/types.go
│  ├─ router/weights.go
│  ├─ router/bandit.go
│  ├─ router/enrich_okaon.go
│  ├─ router/router.go
│  ├─ router/policy.go
│  ├─ storage/sqlite/sqlite.go
│  ├─ storage/sqlite/migrations/0001_init.sql
│  ├─ storage/sqlite/migrations/0002_usage.sql
│  ├─ storage/sqlite/migrations/0003_okaon.sql
│  ├─ okaon/types.go
│  ├─ okaon/store.go
│  ├─ session/llm_call.go
│  └─ diagnostics/doctor.go
└─ README.md                    # (선택)


---

1) go.mod

module devorch

go 1.22

require (
	modernc.org/sqlite v1.32.0
	github.com/oklog/ulid/v2 v2.1.0
)


---

2) cmd/devorch/main.go (CLI)

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"devorch/internal/app"
	"devorch/internal/config"
	"devorch/internal/diagnostics"
	"devorch/internal/log"
	"devorch/internal/provider"
	"devorch/internal/provider/openai"
	"devorch/internal/provider/openrouter"
	"devorch/internal/router"
)

func usage() {
	fmt.Println(`devorch - local-first multi-LLM orchestrator (Step1)

Usage:
  devorch doctor
  devorch providers
  devorch models --provider openai|openrouter
  devorch chat --provider openai|openrouter --model <model> --prompt "hi"

Env:
  DEVORCH_DB_PATH=./devorch.db (default)
  OPENAI_API_KEY=...
  OPENROUTER_API_KEY=...
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

	// Step1: wire minimal deps
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
		if err := diagnostics.RunDoctor(ctx, cfg, deps); err != nil {
			log.Errorf("doctor failed: %v", err)
			os.Exit(1)
		}
		fmt.Println("OK")
	case "providers":
		for _, p := range deps.ProviderRegistry.List() {
			fmt.Printf("- %s (%s)\n", p.Name, p.Kind)
		}
	case "models":
		fs := flag.NewFlagSet("models", flag.ExitOnError)
		provName := fs.String("provider", "", "openai|openrouter")
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
	case "chat":
		fs := flag.NewFlagSet("chat", flag.ExitOnError)
		provName := fs.String("provider", "", "openai|openrouter")
		model := fs.String("model", "", "model id")
		prompt := fs.String("prompt", "", "prompt text")
		_ = fs.Parse(os.Args[2:])
		if *provName == "" || *model == "" || *prompt == "" {
			fmt.Println("missing --provider/--model/--prompt")
			os.Exit(2)
		}

		// ensure provider exists; if not, attempt to register quickly (handy)
		if _, ok := deps.ProviderRegistry.Get(*provName); !ok {
			if *provName == "openai" && cfg.OpenAIKey != "" {
				deps.ProviderRegistry.Register(openai.New(cfg.OpenAIKey))
			}
			if *provName == "openrouter" && cfg.OpenRouterKey != "" {
				deps.ProviderRegistry.Register(openrouter.New(cfg.OpenRouterKey))
			}
		}

		p, ok := deps.ProviderRegistry.Get(*provName)
		if !ok {
			fmt.Printf("provider not found: %s\n", *provName)
			os.Exit(2)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		req := provider.ChatRequest{
			Model: *model,
			Messages: []provider.ChatMessage{
				{Role: "user", Content: *prompt},
			},
			Temperature: 0.2,
		}

		// Step1: route+okaon (minimal)
		wl := router.Workload{
			WorkspaceID: "local",
			UserID:      "local",
			ProjectID:   "adhoc",
			AgentType:   "cli",
			Category:    "quick",
			TaskType:    "chat",
		}

		// candidates: only chosen provider/model (step1 minimal) + enrich okaon anyway
		cands := []router.Candidate{
			{Provider: p.Name(), Model: *model, Endpoint: p.Endpoint()},
		}

		resp, err := deps.SessionCaller.CallAndRecord(ctx, deps.ProviderRegistry, deps.Router, deps.OkAONStore, wl, cands, req)
		if err != nil {
			log.Errorf("chat failed: %v", err)
			os.Exit(1)
		}
		fmt.Println(resp.Text())
	default:
		usage()
		os.Exit(2)
	}
}


---

3) cmd/devorchd/main.go (Daemon 최소)

package main

import (
	"fmt"
	"net/http"
	"os"

	"devorch/internal/app"
	"devorch/internal/config"
	"devorch/internal/log"
)

func main() {
	log.Init()

	cfg, err := config.Load()
	if err != nil {
		log.Errorf("config load failed: %v", err)
		os.Exit(1)
	}

	_, err = app.WireMinimal(cfg)
	if err != nil {
		log.Errorf("wire failed: %v", err)
		os.Exit(1)
	}

	// Step1 daemon: just health endpoint (later expand to SSE/gRPC)
	addr := cfg.DaemonAddr
	if addr == "" {
		addr = "127.0.0.1:8787"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	log.Infof("devorchd listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}


---

4) internal/log/log.go

package log

import (
	"fmt"
	"log"
	"os"
	"time"
)

var (
	std = log.New(os.Stdout, "", 0)
)

func Init() {
	std.SetFlags(0)
}

func ts() string {
	return time.Now().Format(time.RFC3339)
}

func Infof(format string, args ...any) {
	std.Printf("%s [INFO] %s", ts(), fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	std.Printf("%s [ERROR] %s", ts(), fmt.Sprintf(format, args...))
}


---

5) internal/global/paths.go

package global

import (
	"os"
	"path/filepath"
)

func DefaultDBPath() string {
	if v := os.Getenv("DEVORCH_DB_PATH"); v != "" {
		return v
	}
	// Step1: local file next to 실행 위치 (later: XDG/OS-specific)
	return filepath.Join(".", "devorch.db")
}


---

6) internal/config/config.go

package config

type Config struct {
	DBPath     string
	DaemonAddr string

	OpenAIKey     string
	OpenRouterKey string
}


---

7) internal/config/load.go

package config

import (
	"os"

	"devorch/internal/global"
)

func Load() (Config, error) {
	return Config{
		DBPath:        global.DefaultDBPath(),
		DaemonAddr:    os.Getenv("DEVORCH_DAEMON_ADDR"),
		OpenAIKey:     os.Getenv("OPENAI_API_KEY"),
		OpenRouterKey: os.Getenv("OPENROUTER_API_KEY"),
	}, nil
}


---

8) internal/storage/sqlite/sqlite.go (마이그레이션 포함)

package sqlite

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type DB struct {
	SQL *sql.DB
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	// basic pragmas (safe defaults)
	_, _ = db.Exec(`PRAGMA journal_mode=WAL;`)
	_, _ = db.Exec(`PRAGMA synchronous=NORMAL;`)
	return &DB{SQL: db}, nil
}

func runMigrations(db *sql.DB) error {
	dir, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	entries, err := fs.ReadDir(dir, ".")
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".sql") {
			files = append(files, name)
		}
	}
	sort.Strings(files)

	// Step1: idempotent migrations: each file uses IF NOT EXISTS / safe inserts.
	for _, f := range files {
		b, err := fs.ReadFile(dir, f)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(b)); err != nil {
			return fmt.Errorf("migration %s failed: %w", f, err)
		}
	}
	return nil
}


---

9) migrations SQL (3개 전부 제공)

internal/storage/sqlite/migrations/0001_init.sql

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- migration bookkeeping (simple)
INSERT OR IGNORE INTO meta(key, value) VALUES ('schema_version', '1');

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  title TEXT
);

CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_session_created
ON messages(session_id, created_at);

internal/storage/sqlite/migrations/0002_usage.sql

CREATE TABLE IF NOT EXISTS usage_ledger (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,

  workspace_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  project_id TEXT NOT NULL,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,

  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,

  cost_micro_usd INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_usage_ledger_time
ON usage_ledger(created_at);

internal/storage/sqlite/migrations/0003_okaon.sql  ✅ (OkAON “실제 로컬 벤치/품질” 핵심)

-- OkAON: Outcome-KPI-Aware Orchestration Network (local learning store)

CREATE TABLE IF NOT EXISTS okaon_runs (
  run_id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,

  policy_key TEXT NOT NULL,

  workspace_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  project_id TEXT NOT NULL,

  agent_type TEXT NOT NULL,
  category TEXT NOT NULL,
  task_type TEXT NOT NULL,

  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  endpoint TEXT NOT NULL,

  latency_ms INTEGER NOT NULL,
  ok INTEGER NOT NULL, -- 0/1

  -- quality estimation: 0~1
  quality REAL NOT NULL,

  -- minimal token/cost fields
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  cost_micro_usd INTEGER NOT NULL DEFAULT 0,

  -- trace references (optional)
  trace_id TEXT,
  error_code TEXT,
  error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_okaon_policy_time
ON okaon_runs(policy_key, created_at);

CREATE INDEX IF NOT EXISTS idx_okaon_provider_model_time
ON okaon_runs(provider, model, created_at);


---

10) internal/provider/provider.go

package provider

import "context"

type Model struct {
	ID string
}

type ChatMessage struct {
	Role    string // system|user|assistant|tool
	Content string
}

type ChatRequest struct {
	Model       string
	Messages    []ChatMessage
	Temperature float64
	MaxTokens   int
}

type ChatResponse struct {
	RawText          string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CostMicroUSD     int64
}

func (r ChatResponse) Text() string { return r.RawText }

type Provider interface {
	Name() string
	Kind() string        // remote|local
	Endpoint() string    // base url or local runtime id
	Health(ctx context.Context) error
	ListModels(ctx context.Context) ([]Model, error)
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}


---

11) internal/provider/errors.go

package provider

import "fmt"

type Error struct {
	Code    string
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func Err(code, msg string) error {
	return Error{Code: code, Message: msg}
}


---

12) internal/provider/registry.go

package provider

import "sync"

type ProviderInfo struct {
	Name string
	Kind string
}

type Registry struct {
	mu   sync.RWMutex
	m    map[string]Provider
	list []ProviderInfo
}

func NewRegistry() *Registry {
	return &Registry{
		m: make(map[string]Provider),
	}
}

func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[p.Name()] = p
	r.rebuildList()
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
	out := make([]ProviderInfo, len(r.list))
	copy(out, r.list)
	return out
}

func (r *Registry) rebuildList() {
	r.list = r.list[:0]
	for name, p := range r.m {
		r.list = append(r.list, ProviderInfo{Name: name, Kind: p.Kind()})
	}
}


---

13) internal/provider/openai_compat/client.go  (OpenAI-compatible 공통 클라이언트)

package openai_compat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"devorch/internal/provider"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
	Headers map[string]string // optional extra headers
}

func New(baseURL, apiKey string, headers map[string]string) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
		},
		Headers: headers,
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var buf *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		// best-effort read
		var b bytes.Buffer
		_, _ = b.ReadFrom(resp.Body)
		return provider.Err("http_error", fmt.Sprintf("status=%d body=%s", resp.StatusCode, b.String()))
	}

	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (c *Client) ListModels(ctx context.Context) ([]provider.Model, error) {
	var mr modelsResponse
	if err := c.do(ctx, http.MethodGet, "/models", nil, &mr); err != nil {
		return nil, err
	}
	out := make([]provider.Model, 0, len(mr.Data))
	for _, d := range mr.Data {
		out = append(out, provider.Model{ID: d.ID})
	}
	return out, nil
}

// Chat Completions compatible (widely supported)
type chatReq struct {
	Model       string             `json:"model"`
	Messages    []provider.ChatMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	var cr chatResp
	payload := chatReq{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
	if err := c.do(ctx, http.MethodPost, "/chat/completions", payload, &cr); err != nil {
		return provider.ChatResponse{}, err
	}
	txt := ""
	if len(cr.Choices) > 0 {
		txt = cr.Choices[0].Message.Content
	}
	return provider.ChatResponse{
		RawText:          txt,
		PromptTokens:     cr.Usage.PromptTokens,
		CompletionTokens: cr.Usage.CompletionTokens,
		TotalTokens:      cr.Usage.TotalTokens,
		// Step1: cost unknown unless pricing table -> 0
		CostMicroUSD: 0,
	}, nil
}

func (c *Client) Health(ctx context.Context) error {
	// simplest: list models with short timeout
	_, err := c.ListModels(ctx)
	return err
}


---

14) internal/provider/openai/openai.go

package openai

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

type OpenAI struct {
	c *openai_compat.Client
}

func New(apiKey string) *OpenAI {
	return &OpenAI{
		c: openai_compat.New("https://api.openai.com/v1", apiKey, nil),
	}
}

func (p *OpenAI) Name() string     { return "openai" }
func (p *OpenAI) Kind() string     { return "remote" }
func (p *OpenAI) Endpoint() string { return "https://api.openai.com/v1" }

func (p *OpenAI) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *OpenAI) ListModels(ctx context.Context) ([]provider.Model, error) {
	return p.c.ListModels(ctx)
}

func (p *OpenAI) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}


---

15) internal/provider/openrouter/openrouter.go

package openrouter

import (
	"context"

	"devorch/internal/provider"
	"devorch/internal/provider/openai_compat"
)

type OpenRouter struct {
	c *openai_compat.Client
}

func New(apiKey string) *OpenRouter {
	// OpenRouter is OpenAI-compatible, but requires extra headers in many cases
	headers := map[string]string{
		"HTTP-Referer": "http://localhost",
		"X-Title":      "devorch",
	}
	return &OpenRouter{
		c: openai_compat.New("https://openrouter.ai/api/v1", apiKey, headers),
	}
}

func (p *OpenRouter) Name() string     { return "openrouter" }
func (p *OpenRouter) Kind() string     { return "remote" }
func (p *OpenRouter) Endpoint() string { return "https://openrouter.ai/api/v1" }

func (p *OpenRouter) Health(ctx context.Context) error {
	return p.c.Health(ctx)
}

func (p *OpenRouter) ListModels(ctx context.Context) ([]provider.Model, error) {
	return p.c.ListModels(ctx)
}

func (p *OpenRouter) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	return p.c.Chat(ctx, req)
}


---

16) internal/okaon/types.go

package okaon

type Run struct {
	RunID     string
	CreatedAt string

	PolicyKey string

	WorkspaceID string
	UserID      string
	ProjectID   string

	AgentType string
	Category  string
	TaskType  string

	Provider  string
	Model     string
	Endpoint  string

	LatencyMS int
	OK        int     // 0/1
	Quality   float64 // 0~1

	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	CostMicroUSD     int64

	TraceID      string
	ErrorCode    string
	ErrorMessage string
}


---

17) internal/okaon/store.go

package okaon

import (
	"context"
	"database/sql"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) InsertRun(ctx context.Context, r Run) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO okaon_runs(
  run_id, created_at,
  policy_key,
  workspace_id, user_id, project_id,
  agent_type, category, task_type,
  provider, model, endpoint,
  latency_ms, ok, quality,
  prompt_tokens, completion_tokens, total_tokens, cost_micro_usd,
  trace_id, error_code, error_message
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.RunID, r.CreatedAt,
		r.PolicyKey,
		r.WorkspaceID, r.UserID, r.ProjectID,
		r.AgentType, r.Category, r.TaskType,
		r.Provider, r.Model, r.Endpoint,
		r.LatencyMS, r.OK, r.Quality,
		r.PromptTokens, r.CompletionTokens, r.TotalTokens, r.CostMicroUSD,
		nullable(r.TraceID), nullable(r.ErrorCode), nullable(r.ErrorMessage),
	)
	return err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Aggregate recent quality/latency for (policy_key, provider, model).
type Aggregate struct {
	Count     int
	AvgQual   float64
	AvgLatMS  float64
	SuccessRt float64
}

func (s *Store) AggregateRecent(ctx context.Context, policyKey, providerName, model string, limit int) (Aggregate, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT ok, quality, latency_ms
FROM okaon_runs
WHERE policy_key = ? AND provider = ? AND model = ?
ORDER BY created_at DESC
LIMIT ?`, policyKey, providerName, model, limit)
	if err != nil {
		return Aggregate{}, err
	}
	defer rows.Close()

	var (
		n       int
		sumQ    float64
		sumL    float64
		sumOK   float64
	)
	for rows.Next() {
		var ok int
		var q float64
		var lat int
		if err := rows.Scan(&ok, &q, &lat); err != nil {
			return Aggregate{}, err
		}
		n++
		sumQ += q
		sumL += float64(lat)
		sumOK += float64(ok)
	}
	if n == 0 {
		return Aggregate{Count: 0, AvgQual: 0.0, AvgLatMS: 0.0, SuccessRt: 0.0}, nil
	}
	return Aggregate{
		Count:     n,
		AvgQual:   sumQ / float64(n),
		AvgLatMS:  sumL / float64(n),
		SuccessRt: sumOK / float64(n),
	}, nil
}


---

18) internal/router/types.go

package router

type Workload struct {
	WorkspaceID string
	UserID      string
	ProjectID   string

	AgentType string
	Category  string
	TaskType  string
}

type Candidate struct {
	Provider string
	Model    string
	Endpoint string

	EstLatencyMS    int
	EstCostMicroUSD int64
	EstQuality      float64 // 0~1
}

type Decision struct {
	Provider string
	Model    string
	Endpoint string

	Score float64
	Why   string
}


---

19) internal/router/weights.go

package router

type Weights struct {
	Quality float64
	Latency float64
	Cost    float64
	Success float64
}

func DefaultWeights() Weights {
	return Weights{
		Quality: 0.55,
		Latency: 0.25,
		Cost:    0.10,
		Success: 0.10,
	}
}


---

20) internal/router/bandit.go (Epsilon-greedy 최소)

package router

import (
	"math/rand"
	"time"
)

type Bandit interface {
	Choose(exploitIndex int, n int) int
}

type EpsilonGreedy struct {
	Epsilon float64
	rng     *rand.Rand
}

func NewEpsilonGreedy() *EpsilonGreedy {
	return &EpsilonGreedy{
		Epsilon: 0.10,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (b *EpsilonGreedy) Choose(exploitIndex int, n int) int {
	if n <= 0 {
		return -1
	}
	if n == 1 {
		return 0
	}
	if b.rng.Float64() < b.Epsilon {
		return b.rng.Intn(n)
	}
	return exploitIndex
}


---

21) internal/router/policy.go

package router

import "fmt"

func BuildPolicyKey(w Workload) string {
	return fmt.Sprintf("ws=%s|proj=%s|agent=%s|cat=%s|task=%s",
		w.WorkspaceID, w.ProjectID, w.AgentType, w.Category, w.TaskType)
}


---

22) internal/router/enrich_okaon.go  ✅ (OkAON 결과로 후보 점수 보정)

package router

import (
	"context"

	"devorch/internal/okaon"
)

type OkAONEnricher struct {
	Store *okaon.Store
}

func (e *OkAONEnricher) Enrich(ctx context.Context, policyKey string, c Candidate) (Candidate, string) {
	if e == nil || e.Store == nil {
		return c, "okaon:disabled"
	}
	agg, err := e.Store.AggregateRecent(ctx, policyKey, c.Provider, c.Model, 80)
	if err != nil || agg.Count == 0 {
		return c, "okaon:none"
	}

	// Step1 heuristic: replace estimates with real observed aggregates
	if agg.AvgLatMS > 0 {
		c.EstLatencyMS = int(agg.AvgLatMS + 0.5)
	}
	c.EstQuality = clamp01(agg.AvgQual)
	// success gets applied in scoring separately; cost stays 0 in step1

	why := "okaon:applied"
	return c, why
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

23) internal/router/router.go

package router

import (
	"context"
	"fmt"
	"math"
)

type Router struct {
	w        Weights
	bandit   Bandit
	enricher *OkAONEnricher
}

func New(w Weights, bandit Bandit, enricher *OkAONEnricher) *Router {
	return &Router{w: w, bandit: bandit, enricher: enricher}
}

func (r *Router) Select(ctx context.Context, wl Workload, candidates []Candidate) (Decision, error) {
	if len(candidates) == 0 {
		return Decision{}, fmt.Errorf("no candidates")
	}

	policyKey := BuildPolicyKey(wl)

	// Enrich using OkAON (real local history)
	enriched := make([]Candidate, 0, len(candidates))
	enrichWhy := make([]string, 0, len(candidates))
	for _, c := range candidates {
		ec, why := r.enricher.Enrich(ctx, policyKey, c)
		enriched = append(enriched, ec)
		enrichWhy = append(enrichWhy, why)
	}

	// Score (higher is better). Normalize latency cheaply: 1/(1+lat)
	bestIdx := 0
	bestScore := -math.MaxFloat64

	scores := make([]float64, len(enriched))
	for i, c := range enriched {
		qual := c.EstQuality
		lat := c.EstLatencyMS
		latScore := 1.0
		if lat > 0 {
			latScore = 1.0 / (1.0 + float64(lat)/200.0)
		}
		costScore := 1.0 // Step1 unknown => neutral

		score := r.w.Quality*qual + r.w.Latency*latScore + r.w.Cost*costScore
		scores[i] = score
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	chosen := bestIdx
	if r.bandit != nil {
		chosen = r.bandit.Choose(bestIdx, len(enriched))
		if chosen < 0 {
			chosen = bestIdx
		}
	}

	c := enriched[chosen]
	why := fmt.Sprintf("score=%.4f (enrich=%s)", scores[chosen], enrichWhy[chosen])

	return Decision{
		Provider: c.Provider,
		Model:    c.Model,
		Endpoint: c.Endpoint,
		Score:    scores[chosen],
		Why:      why,
	}, nil
}


---

24) internal/session/llm_call.go  ✅ (Router 결정 + Provider 호출 + OkAON 기록)

package session

import (
	"context"
	"time"

	"github.com/oklog/ulid/v2"

	"devorch/internal/okaon"
	"devorch/internal/provider"
	"devorch/internal/router"
)

type Caller struct{}

func NewCaller() *Caller { return &Caller{} }

// Step1: "quality" 자동 산정은 매우 보수적으로(성공=1이면 0.6 고정)
// 다음 단계에서: 테스트/린트/사용자 피드백으로 quality 계산 고도화
func (c *Caller) CallAndRecord(
	ctx context.Context,
	reg *provider.Registry,
	rt *router.Router,
	okStore *okaon.Store,
	wl router.Workload,
	candidates []router.Candidate,
	req provider.ChatRequest,
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
		Quality:   estimateQuality(callErr == nil),

		PromptTokens:     resp.PromptTokens,
		CompletionTokens: resp.CompletionTokens,
		TotalTokens:      resp.TotalTokens,
		CostMicroUSD:     resp.CostMicroUSD,
	}

	if callErr != nil {
		run.ErrorCode = "provider_call_failed"
		run.ErrorMessage = callErr.Error()
	}

	// best-effort insert (do not mask provider errors)
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
	// Step1: placeholder. Step5에서 자동평가/테스트 결과로 교체
	return 0.60
}


---

25) internal/diagnostics/doctor.go

package diagnostics

import (
	"context"
	"fmt"
	"time"

	"devorch/internal/app"
	"devorch/internal/config"
	"devorch/internal/log"
)

func RunDoctor(ctx context.Context, cfg config.Config, deps *app.Deps) error {
	log.Infof("doctor: db=%s", cfg.DBPath)

	// provider health checks
	for _, pinfo := range deps.ProviderRegistry.List() {
		p, _ := deps.ProviderRegistry.Get(pinfo.Name)
		cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := p.Health(cctx)
		cancel()
		if err != nil {
			return fmt.Errorf("provider %s health failed: %w", pinfo.Name, err)
		}
		log.Infof("doctor: provider %s health ok", pinfo.Name)
	}
	return nil
}


---

26) internal/app/app.go

package app

import (
	"devorch/internal/okaon"
	"devorch/internal/provider"
	"devorch/internal/router"
	"devorch/internal/session"
	"devorch/internal/storage/sqlite"
)

type Deps struct {
	DB               *sqlite.DB
	ProviderRegistry *provider.Registry
	OkAONStore       *okaon.Store
	Router           *router.Router
	SessionCaller    *session.Caller
}


---

27) internal/app/wire.go  ✅ (전체 연결 핵심)

package app

import (
	"devorch/internal/config"
	"devorch/internal/okaon"
	"devorch/internal/provider"
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

	enricher := &router.OkAONEnricher{Store: okStore}
	rt := router.New(router.DefaultWeights(), router.NewEpsilonGreedy(), enricher)

	return &Deps{
		DB:               db,
		ProviderRegistry: reg,
		OkAONStore:       okStore,
		Router:           rt,
		SessionCaller:    session.NewCaller(),
	}, nil
}


---

28) README.md (선택이지만 바로 테스트용으로 추천)

# devorch (Step1)

## Setup
```bash
export OPENAI_API_KEY="..."
export OPENROUTER_API_KEY="..."
export DEVORCH_DB_PATH="./devorch.db"

Build

go mod tidy
go build ./...

Doctor

./devorch doctor

Providers

./devorch providers

Models

./devorch models --provider openrouter

Chat (records into okaon_runs)

./devorch chat --provider openrouter --model openai/gpt-4o-mini --prompt "hello"
# sqlite3 devorch.db "select provider, model, latency_ms, ok, quality from okaon_runs order by created_at desc limit 5;"

---

# 29) 실행 확인 체크리스트 (1단계 DoD)

1) 빌드
```bash
go mod tidy
go build ./...

2. doctor



./devorch doctor

3. 모델 리스트



./devorch models --provider openrouter

4. chat + OkAON 기록



./devorch chat --provider openrouter --model <모델ID> --prompt "hi"
sqlite3 devorch.db "select provider, model, latency_ms, ok, quality, error_code from okaon_runs order by created_at desc limit 3;"


---

다음 단계(2단계) 예고

이제 Step1이 “기록/학습 기반 라우팅”의 토대를 만들었습니다.
다음 턴부터는 요청하신 핵심인:

멀티 OS(맥 인텔/실리콘, 윈도우, 리눅스) 로컬 LLM 자동 설치/다운로드

런타임 번들(runtime/llm/ollama + models manifest) 사용

성능 벤치(실측) → OkAON 품질/지연/성공률 자동 업데이트

작업 완료 기준 기반 벤치마크(테스트/린트/빌드 결과)


을 2단계 풀코드로 이어갑니다.

원하시면 다음 턴은 바로:

internal/provider/local/*

internal/provider/ollama/*

runtime/llm/ollama/* 번들 매니페스트

internal/diagnostics/checks/ollama.go 스타일 확장


을 “파일별 풀코드”로 드리겠습니다.