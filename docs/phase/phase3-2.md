좋습니다. 이어서 “실제로 돌아가는 3단계(daemon 코어 + 기본 API + 로컬/원격 LLM 최소 오케스트레이션 + SQLite 저장)” 쪽을 먼저 **풀코드(파일별)**로 계속 생성하겠습니다.
(전체 트리 전부를 한 번에 다 뽑으면 너무 커져서, 실행 가능한 단위로 끊어서 제공합니다. 이번 묶음은 devorchd 데몬이 실제로 실행되고 /health, /v1/chat, /v1/models, /v1/bench 엔드포인트까지 동작하는 수준입니다.)
아래 파일들을 그대로 생성하세요.
1) go.mod / go.sum
go.mod
코드 복사
Go
module devorch

go 1.22
외부 라이브러리 없이 표준 라이브러리만으로 먼저 굴립니다(고성능/확장용은 다음 묶음에서 점진 도입).
2) 엔트리포인트
cmd/devorchd/main.go
코드 복사
Go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"devorch/internal/app"
	"devorch/internal/config"
	"devorch/internal/global"
	"devorch/internal/log"
)

func main() {
	var (
		addr     = flag.String("addr", "127.0.0.1:8844", "listen address")
		logLevel = flag.String("log-level", "INFO", "DEBUG|INFO|WARN|ERROR")
	)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// signals
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	paths := global.MustPaths()
	logger := log.New(log.Options{
		Level:   *logLevel,
		AppName: "devorchd",
		LogDir:  paths.LogDir,
	})

	cfg, err := config.Load(config.LoadInput{
		Paths:  paths,
		Logger: logger,
	})
	if err != nil {
		logger.Error("config load failed", "err", err.Error())
		os.Exit(1)
	}

	a, err := app.New(app.Options{
		Addr:   *addr,
		Paths:  paths,
		Logger: logger,
		Config: cfg,
	})
	if err != nil {
		logger.Error("app init failed", "err", err.Error())
		os.Exit(1)
	}

	logger.Info("starting devorchd", "version", global.Version, "addr", *addr)

	if err := a.Start(ctx); err != nil {
		logger.Error("start failed", "err", err.Error())
		os.Exit(1)
	}

	<-ctx.Done()

	shutdownCtx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	logger.Info("stopping devorchd")
	if err := a.Stop(shutdownCtx); err != nil {
		logger.Error("stop failed", "err", err.Error())
	}

	fmt.Println("bye")
}
3) global / paths
internal/global/version.go
코드 복사
Go
package global

// 빌드시 -ldflags "-X devorch/internal/global.Version=1.0.0 -X devorch/internal/global.Commit=... -X devorch/internal/global.BuildDate=..."
var (
	Version   = "0.0.0-dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)
internal/global/constants.go
코드 복사
Go
package global

const (
	AppName = "devorch"
)
internal/global/paths.go
코드 복사
Go
package global

import (
	"os"
	"path/filepath"
	"runtime"
)

type Paths struct {
	HomeDir  string
	ConfigDir string
	CacheDir  string
	DataDir   string
	StateDir  string
	LogDir    string
}

func MustPaths() Paths {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	// XDG-ish defaults
	config := getenvDefault("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	cache := getenvDefault("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	data := getenvDefault("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	state := getenvDefault("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))

	// Windows: prefer AppData paths (simple version)
	if runtime.GOOS == "windows" {
		if v := os.Getenv("APPDATA"); v != "" {
			config = v
		}
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			cache = filepath.Join(v, "Cache")
			data = v
			state = v
		}
	}

	p := Paths{
		HomeDir:   home,
		ConfigDir: filepath.Join(config, AppName),
		CacheDir:  filepath.Join(cache, AppName),
		DataDir:   filepath.Join(data, AppName),
		StateDir:  filepath.Join(state, AppName),
		LogDir:    filepath.Join(data, AppName, "log"),
	}

	mkdirAll(p.ConfigDir)
	mkdirAll(p.CacheDir)
	mkdirAll(p.DataDir)
	mkdirAll(p.StateDir)
	mkdirAll(p.LogDir)

	return p
}

func getenvDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func mkdirAll(p string) {
	_ = os.MkdirAll(p, 0o755)
}
4) log (간단 고성능 콘솔+파일 로테이션 “초기판”)
internal/log/log.go
코드 복사
Go
package log

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

type Options struct {
	Level   string
	AppName string
	LogDir  string
}

type Logger struct {
	min   Level
	mu    sync.Mutex
	file  *os.File
	logDir string
	app    string
}

func New(opts Options) *Logger {
	l := &Logger{
		min:    parseLevel(opts.Level),
		logDir: opts.LogDir,
		app:    opts.AppName,
	}
	l.rotateLocked()
	return l
}

func (l *Logger) Debug(msg string, kv ...any) { l.write(DEBUG, msg, kv...) }
func (l *Logger) Info(msg string, kv ...any)  { l.write(INFO, msg, kv...) }
func (l *Logger) Warn(msg string, kv ...any)  { l.write(WARN, msg, kv...) }
func (l *Logger) Error(msg string, kv ...any) { l.write(ERROR, msg, kv...) }

func (l *Logger) write(level Level, msg string, kv ...any) {
	if level < l.min {
		return
	}
	now := time.Now().Format(time.RFC3339Nano)
	levelStr := levelString(level)
	line := fmt.Sprintf("%s [%s] %s", now, levelStr, msg)

	if len(kv) > 0 {
		line += " |"
		for i := 0; i < len(kv); i += 2 {
			if i+1 >= len(kv) {
				line += fmt.Sprintf(" %v=?", kv[i])
				break
			}
			line += fmt.Sprintf(" %v=%v", kv[i], kv[i+1])
		}
	}

	// stdout
	fmt.Println(line)

	// file
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		l.rotateLocked()
	}
	// naive daily rotate
	if l.file != nil {
		_, _ = l.file.WriteString(line + "\n")
	}
}

func (l *Logger) rotateLocked() {
	_ = os.MkdirAll(l.logDir, 0o755)
	name := fmt.Sprintf("%s-%s.log", l.app, time.Now().Format("2006-01-02"))
	path := filepath.Join(l.logDir, name)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		l.file = nil
		return
	}
	l.file = f
}

func parseLevel(s string) Level {
	switch s {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}

func levelString(lv Level) string {
	switch lv {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	default:
		return "ERROR"
	}
}
5) config (JSONC “초기판”: 주석 제거 + 머지)
JSONC 완전체(Zod급 검증)는 다음 묶음에서 확장하고, 지금은 실행 가능한 최소 로더를 먼저 둡니다.
internal/config/types.go
코드 복사
Go
package config

type Config struct {
	Server ServerConfig `json:"server"`
	Local  LocalConfig  `json:"local"`
	Remote RemoteConfig `json:"remote"`
}

type ServerConfig struct {
	AllowCORS bool `json:"allowCORS"`
}

type LocalConfig struct {
	OllamaBaseURL string `json:"ollamaBaseURL"` // ex) http://127.0.0.1:11434
}

type RemoteConfig struct {
	// OpenAI-compatible endpoint (optional)
	OpenAICompatBaseURL string `json:"openaiCompatBaseURL"` // ex) https://api.openai.com/v1
	OpenAICompatToken   string `json:"openaiCompatToken"`   // env override recommended
}
internal/config/load.go
코드 복사
Go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"devorch/internal/global"
	"devorch/internal/log"
)

type LoadInput struct {
	Paths  global.Paths
	Logger *log.Logger
}

func Load(in LoadInput) (Config, error) {
	// defaults
	cfg := Config{
		Server: ServerConfig{AllowCORS: true},
		Local:  LocalConfig{OllamaBaseURL: "http://127.0.0.1:11434"},
	}

	// order: global -> project(.devorch/devorch.jsonc) -> env overrides
	globalPath := filepath.Join(in.Paths.ConfigDir, "devorch.jsonc")
	cfg = merge(cfg, readIfExists(globalPath, in.Logger))

	// project override (current working dir)
	if wd, err := os.Getwd(); err == nil {
		projectPath := filepath.Join(wd, ".devorch", "devorch.jsonc")
		cfg = merge(cfg, readIfExists(projectPath, in.Logger))
	}

	// env overrides (token is sensitive)
	if v := os.Getenv("DEVORCH_OPENAI_COMPAT_BASEURL"); v != "" {
		cfg.Remote.OpenAICompatBaseURL = v
	}
	if v := os.Getenv("DEVORCH_OPENAI_COMPAT_TOKEN"); v != "" {
		cfg.Remote.OpenAICompatToken = v
	}
	if v := os.Getenv("DEVORCH_OLLAMA_BASEURL"); v != "" {
		cfg.Local.OllamaBaseURL = v
	}

	return cfg, nil
}

func readIfExists(path string, logger *log.Logger) Config {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}
	}
	// JSONC strip: remove //... and /*...*/ (simple)
	s := string(b)
	s = stripLineComments(s)
	s = stripBlockComments(s)

	var cfg Config
	if err := json.Unmarshal([]byte(s), &cfg); err != nil {
		logger.Warn("config parse failed", "path", path, "err", err.Error())
		return Config{}
	}
	logger.Info("config loaded", "path", path)
	return cfg
}

func stripLineComments(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "//"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

func stripBlockComments(s string) string {
	for {
		start := strings.Index(s, "/*")
		if start < 0 {
			return s
		}
		end := strings.Index(s[start+2:], "*/")
		if end < 0 {
			return s[:start]
		}
		end = start + 2 + end
		s = s[:start] + s[end+2:]
	}
}

func merge(base, overlay Config) Config {
	// only overwrite non-zero / non-empty
	if overlay.Server.AllowCORS {
		base.Server.AllowCORS = overlay.Server.AllowCORS
	}
	if overlay.Local.OllamaBaseURL != "" {
		base.Local.OllamaBaseURL = overlay.Local.OllamaBaseURL
	}
	if overlay.Remote.OpenAICompatBaseURL != "" {
		base.Remote.OpenAICompatBaseURL = overlay.Remote.OpenAICompatBaseURL
	}
	if overlay.Remote.OpenAICompatToken != "" {
		base.Remote.OpenAICompatToken = overlay.Remote.OpenAICompatToken
	}
	return base
}
6) storage (SQLite “초기판”: 파일 DB + migrations)
internal/storage/sqlite/sqlite.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	SQL *sql.DB
}

type OpenInput struct {
	DataDir string
}

func Open(ctx context.Context, in OpenInput) (*DB, error) {
	if err := os.MkdirAll(in.DataDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(in.DataDir, "devorch.db")
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // sqlite WAL: keep simple for now
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{SQL: db}, nil
}

func (d *DB) Close() error {
	if d == nil || d.SQL == nil {
		return nil
	}
	return d.SQL.Close()
}
internal/storage/sqlite/migrate.go
코드 복사
Go
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations(
  version TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
);`); err != nil {
		return err
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}

	type mig struct {
		name string
		sql  string
	}
	var list []mig
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		b, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		list = append(list, mig{name: e.Name(), sql: string(b)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })

	applied := map[string]bool{}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return err
		}
		applied[v] = true
	}

	for _, m := range list {
		if applied[m.name] {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", m.name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES(?, datetime('now'))`, m.name); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
internal/storage/sqlite/migrations/0001_init.sql
코드 복사
Sql
CREATE TABLE IF NOT EXISTS sessions(
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS messages(
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY(session_id) REFERENCES sessions(id)
);

CREATE INDEX IF NOT EXISTS idx_messages_session_id_created_at
ON messages(session_id, created_at);
internal/storage/sqlite/migrations/0002_bench.sql
코드 복사
Sql
CREATE TABLE IF NOT EXISTS bench_runs(
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  os TEXT NOT NULL,
  arch TEXT NOT NULL,
  model TEXT NOT NULL,
  provider TEXT NOT NULL,
  task TEXT NOT NULL,
  latency_ms INTEGER NOT NULL,
  ok INTEGER NOT NULL,
  score REAL NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_bench_runs_created_at
ON bench_runs(created_at);

CREATE INDEX IF NOT EXISTS idx_bench_runs_model_provider
ON bench_runs(model, provider);
modernc.org/sqlite 드라이버를 쓰므로 CGO 없이도 동작합니다. (go.mod에는 자동으로 go.sum이 생깁니다)
7) id (ULID 간단판)
internal/id/ulid.go
코드 복사
Go
package id

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// “간단 ULID 대체”: time prefix + random 10 bytes
func New() string {
	t := time.Now().UnixMilli()
	r := make([]byte, 10)
	_, _ = rand.Read(r)
	return hex.EncodeToString([]byte{
		byte(t >> 40), byte(t >> 32), byte(t >> 24), byte(t >> 16), byte(t >> 8), byte(t),
	}) + hex.EncodeToString(r)
}
8) provider (로컬 Ollama + 원격 OpenAI-compatible 최소 구현)
internal/provider/types.go
코드 복사
Go
package provider

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type ChatResponse struct {
	Text       string `json:"text"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	LatencyMs  int64  `json:"latencyMs"`
	RequestID  string `json:"requestId"`
}
internal/provider/provider.go
코드 복사
Go
package provider

import "context"

type Provider interface {
	Name() string
	ListModels(ctx context.Context) ([]string, error)
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
internal/provider/registry.go
코드 복사
Go
package provider

import (
	"context"
	"errors"
)

type Registry struct {
	list []Provider
}

func NewRegistry(providers ...Provider) *Registry {
	var p []Provider
	for _, x := range providers {
		if x != nil {
			p = append(p, x)
		}
	}
	return &Registry{list: p}
}

func (r *Registry) Providers() []Provider {
	return append([]Provider(nil), r.list...)
}

func (r *Registry) Get(name string) (Provider, bool) {
	for _, p := range r.list {
		if p.Name() == name {
			return p, true
		}
	}
	return nil, false
}

func (r *Registry) ListAllModels(ctx context.Context) (map[string][]string, error) {
	out := map[string][]string{}
	var anyOK bool
	for _, p := range r.list {
		m, err := p.ListModels(ctx)
		if err != nil {
			continue
		}
		out[p.Name()] = m
		anyOK = true
	}
	if !anyOK {
		return out, errors.New("no provider available")
	}
	return out, nil
}
internal/provider/ollama/ollama.go
코드 복사
Go
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"devorch/internal/id"
	"devorch/internal/provider"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Name() string { return "ollama" }

func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/api/tags", nil)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, errors.New("ollama tags failed")
	}
	var parsed struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(parsed.Models))
	for _, m := range parsed.Models {
		out = append(out, m.Name)
	}
	return out, nil
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	// Convert to Ollama chat format
	type ollamaMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	var msgs []ollamaMsg
	for _, m := range req.Messages {
		msgs = append(msgs, ollamaMsg{Role: m.Role, Content: m.Content})
	}
	payload := map[string]any{
		"model":    req.Model,
		"messages": msgs,
		"stream":   false, // 초기판은 non-stream
	}

	b, _ := json.Marshal(payload)
	start := time.Now()

	hreq, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/chat", bytes.NewReader(b))
	hreq.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(hreq)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return provider.ChatResponse{}, errors.New("ollama chat failed: " + string(body))
	}

	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return provider.ChatResponse{}, err
	}

	return provider.ChatResponse{
		Text:      parsed.Message.Content,
		Provider:  c.Name(),
		Model:     req.Model,
		LatencyMs: time.Since(start).Milliseconds(),
		RequestID: id.New(),
	}, nil
}
internal/provider/openai_compat/openai_compat.go
코드 복사
Go
package openai_compat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"devorch/internal/id"
	"devorch/internal/provider"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Name() string { return "openai_compat" }

func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	if c.BaseURL == "" || c.Token == "" {
		return nil, errors.New("openai_compat not configured")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+c.Token)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, errors.New("models failed")
	}
	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		out = append(out, m.ID)
	}
	return out, nil
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if c.BaseURL == "" || c.Token == "" {
		return provider.ChatResponse{}, errors.New("openai_compat not configured")
	}

	start := time.Now()
	payload := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   false, // 초기판 non-stream
	}
	b, _ := json.Marshal(payload)

	hreq, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(b))
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+c.Token)

	res, err := c.HTTP.Do(hreq)
	if err != nil {
		return provider.ChatResponse{}, err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return provider.ChatResponse{}, errors.New("chat failed: " + string(body))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return provider.ChatResponse{}, err
	}
	if len(parsed.Choices) == 0 {
		return provider.ChatResponse{}, errors.New("no choices")
	}

	return provider.ChatResponse{
		Text:      parsed.Choices[0].Message.Content,
		Provider:  c.Name(),
		Model:     req.Model,
		LatencyMs: time.Since(start).Milliseconds(),
		RequestID: id.New(),
	}, nil
}
9) router (멀티 LLM “초기 자동 선택”: 로컬 우선 → 원격 폴백)
internal/router/router.go
코드 복사
Go
package router

import (
	"context"
	"errors"
	"time"

	"devorch/internal/provider"
)

type Router struct {
	Providers *provider.Registry
}

func New(reg *provider.Registry) *Router {
	return &Router{Providers: reg}
}

type Routed struct {
	Chosen provider.Provider
	Reason string
}

func (r *Router) ChooseForChat(ctx context.Context, req provider.ChatRequest) (Routed, error) {
	// 초기 규칙:
	// 1) 모델명이 "ollama/" prefix면 ollama 강제
	// 2) 그 외에는 ollama 가능하면 ollama (로컬), 실패하면