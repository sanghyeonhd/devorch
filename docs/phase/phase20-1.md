좋습니다. **“계속 개발”**로 이해하고, 이번 응답에서는 **빌드/실행이 되는 최소 코어(daemon + CLI + 경로/설정/로깅/SQLite 스토리지/헬스 API)**를 “풀코드”로 먼저 깔아두겠습니다.
(다음 턴부터 OAuth/Provider/Ollama/OpenRouter/Router/Scheduler/Session/Tool/IDE/VSCode 확장 순으로 계속 이어가면 됩니다.)

아래는 파일 경로별 전체 코드입니다.


---

0) go.mod

go.mod

module devorch

go 1.22

require (
	github.com/google/uuid v1.6.0
	github.com/mattn/go-sqlite3 v1.14.22
	golang.org/x/sys v0.22.0
)

go.sum

> go mod tidy로 생성하세요. (여기서 강제로 다 쓰면 오히려 충돌 납니다.)




---

1) cmd 엔트리포인트

cmd/devorchd/main.go

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"devorch/internal/app"
	"devorch/internal/config"
	"devorch/internal/global"
	"devorch/internal/log"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	build := global.BuildInfoFromEnv()
	paths := global.DetectPaths()
	logger := log.New(log.Config{Level: "info", Service: "devorchd", Build: build})

	cfg, cfgSource, err := config.Load(config.LoadInput{
		Paths:  paths,
		Env:    os.Environ(),
		Logger: logger,
	})
	if err != nil {
		logger.Error("config load failed", "err", err)
		os.Exit(1)
	}
	logger.Info("config loaded", "source", cfgSource)

	a, err := app.New(app.Input{
		Build:  build,
		Paths:  paths,
		Logger: logger,
		Config: cfg,
	})
	if err != nil {
		logger.Error("app init failed", "err", err)
		os.Exit(1)
	}

	if err := a.Start(ctx); err != nil {
		logger.Error("app start failed", "err", err)
		os.Exit(1)
	}
	logger.Info("devorch daemon started", "addr", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout())
	defer shutdownCancel()

	if err := a.Stop(shutdownCtx); err != nil {
		logger.Error("app stop failed", "err", err)
		os.Exit(1)
	}
	logger.Info("devorch daemon stopped")
}

cmd/devorch/main.go

package main

import (
	"fmt"
	"os"

	"devorch/internal/cli"
	"devorch/internal/global"
	"devorch/internal/log"
)

func main() {
	build := global.BuildInfoFromEnv()
	paths := global.DetectPaths()
	logger := log.New(log.Config{Level: "info", Service: "devorch", Build: build})

	code := cli.Run(cli.Input{
		Args:   os.Args[1:],
		Build:  build,
		Paths:  paths,
		Logger: logger,
		Env:    os.Environ(),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if code != 0 {
		fmt.Fprintln(os.Stderr, "exit code:", code)
	}
	os.Exit(code)
}


---

2) internal/global (버전/빌드/경로)

internal/global/buildinfo.go

package global

import "os"

type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

func BuildInfoFromEnv() BuildInfo {
	// 빌드 시 -ldflags 로 넣거나, 환경변수로도 주입 가능하게.
	v := os.Getenv("DEVORCH_VERSION")
	if v == "" {
		v = "0.0.0-dev"
	}
	return BuildInfo{
		Version:   v,
		Commit:    getenvDefault("DEVORCH_COMMIT", "unknown"),
		BuildDate: getenvDefault("DEVORCH_BUILD_DATE", "unknown"),
	}
}

func getenvDefault(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

internal/global/version.go

package global

func UserAgent(build BuildInfo) string {
	return "devorch/" + build.Version
}

internal/global/constants.go

package global

const (
	AppName          = "devorch"
	DefaultHTTPHost  = "127.0.0.1"
	DefaultHTTPPort  = 4096
	DefaultLogDir    = "log"
	DefaultDataDir   = "data"
	DefaultStateDir  = "state"
	DefaultConfigDir = "config"
	DefaultCacheDir  = "cache"
)

internal/global/paths.go

package global

import (
	"os"
	"path/filepath"
	"runtime"
)

type Paths struct {
	Home   string
	Config string
	Data   string
	State  string
	Cache  string
	Log    string
}

func DetectPaths() Paths {
	home, _ := os.UserHomeDir()
	// 기본은 XDG 스타일을 따르되, Windows는 AppData 계열로 맵핑
	cfgRoot := xdgOrDefault("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	dataRoot := xdgOrDefault("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	stateRoot := xdgOrDefault("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	cacheRoot := xdgOrDefault("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	// Windows special-case
	if runtime.GOOS == "windows" {
		roaming := os.Getenv("APPDATA")
		local := os.Getenv("LOCALAPPDATA")
		if roaming != "" {
			cfgRoot = roaming
			dataRoot = roaming
			stateRoot = roaming
		}
		if local != "" {
			cacheRoot = local
		}
	}

	return Paths{
		Home:   home,
		Config: filepath.Join(cfgRoot, AppName),
		Data:   filepath.Join(dataRoot, AppName),
		State:  filepath.Join(stateRoot, AppName),
		Cache:  filepath.Join(cacheRoot, AppName),
		Log:    filepath.Join(dataRoot, AppName, "log"),
	}
}

func xdgOrDefault(envKey, def string) string {
	v := os.Getenv(envKey)
	if v == "" {
		return def
	}
	return v
}


---

3) internal/log (간단 구조화 로거)

internal/log/log.go

package log

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"devorch/internal/global"
)

type Config struct {
	Level   string
	Service string
	Build   global.BuildInfo
}

type Logger struct {
	mu      sync.Mutex
	level   string
	service string
	build   global.BuildInfo
}

func New(cfg Config) *Logger {
	lvl := cfg.Level
	if lvl == "" {
		lvl = "info"
	}
	return &Logger{
		level:   lvl,
		service: cfg.Service,
		build:   cfg.Build,
	}
}

func (l *Logger) Info(msg string, kv ...any)  { l.emit("info", msg, kv...) }
func (l *Logger) Warn(msg string, kv ...any)  { l.emit("warn", msg, kv...) }
func (l *Logger) Error(msg string, kv ...any) { l.emit("error", msg, kv...) }

func (l *Logger) emit(level, msg string, kv ...any) {
	// 매우 단순한 레벨 필터(필요하면 확장)
	if l.level == "error" && level != "error" {
		return
	}
	if l.level == "warn" && level == "info" {
		return
	}

	fields := map[string]any{
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
		"level":   level,
		"service": l.service,
		"msg":     msg,
		"version": l.build.Version,
		"commit":  l.build.Commit,
	}
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			continue
		}
		fields[k] = kv[i+1]
	}

	b, _ := json.Marshal(fields)

	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintln(os.Stdout, string(b))
}


---

4) internal/config (로컬 설정 로딩: 기본값 + 파일/환경변수)

> 지금은 “최소 런타임”을 위해:

1. 기본값 → 2) ~/.config/devorch/devorch.json 있으면 덮어쓰기 → 3) env DEVORCH_* 일부 오버라이드
JSONC/머지/스키마/원격 config는 다음 배치에서 확장합니다.





internal/config/config.go

package config

import "time"

type Config struct {
	Server ServerConfig `json:"server"`
	Storage StorageConfig `json:"storage"`
}

type ServerConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	ShutdownTimeoutMs int `json:"shutdown_timeout_ms"`
}

func (s ServerConfig) ShutdownTimeout() time.Duration {
	if s.ShutdownTimeoutMs <= 0 {
		return 5 * time.Second
	}
	return time.Duration(s.ShutdownTimeoutMs) * time.Millisecond
}

type StorageConfig struct {
	Driver string `json:"driver"` // "sqlite"
	SQLite SQLiteConfig `json:"sqlite"`
}

type SQLiteConfig struct {
	Path string `json:"path"`
}

internal/config/defaults.go

package config

import "devorch/internal/global"

func Defaults(paths global.Paths) Config {
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 4096,
			ShutdownTimeoutMs: 5000,
		},
		Storage: StorageConfig{
			Driver: "sqlite",
			SQLite: SQLiteConfig{
				Path: paths.Data + "/devorch.sqlite",
			},
		},
	}
}

internal/config/load.go

package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"devorch/internal/global"
	"devorch/internal/log"
)

type LoadInput struct {
	Paths  global.Paths
	Env    []string
	Logger *log.Logger
}

func Load(in LoadInput) (Config, string, error) {
	cfg := Defaults(in.Paths)
	source := "defaults"

	// config file: ~/.config/devorch/devorch.json
	configFile := filepath.Join(in.Paths.Config, "devorch.json")
	if b, err := os.ReadFile(configFile); err == nil {
		var fileCfg Config
		if err := json.Unmarshal(b, &fileCfg); err != nil {
			return Config{}, "", err
		}
		cfg = merge(cfg, fileCfg)
		source = "file:" + configFile
	} else {
		if !errors.Is(err, os.ErrNotExist) && in.Logger != nil {
			in.Logger.Warn("config read failed", "path", configFile, "err", err)
		}
	}

	// env overrides (최소)
	// DEVORCH_HOST, DEVORCH_PORT, DEVORCH_SQLITE_PATH
	envMap := envToMap(in.Env)
	if v := envMap["DEVORCH_HOST"]; v != "" {
		cfg.Server.Host = v
		source += "+env"
	}
	if v := envMap["DEVORCH_PORT"]; v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p <= 0 {
			return Config{}, "", errors.New("invalid DEVORCH_PORT")
		}
		cfg.Server.Port = p
		source += "+env"
	}
	if v := envMap["DEVORCH_SQLITE_PATH"]; v != "" {
		cfg.Storage.SQLite.Path = v
		source += "+env"
	}

	return cfg, source, nil
}

func envToMap(env []string) map[string]string {
	m := map[string]string{}
	for _, e := range env {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				m[e[:i]] = e[i+1:]
				break
			}
		}
	}
	return m
}

// merge: non-zero fields from b override a
func merge(a, b Config) Config {
	if b.Server.Host != "" {
		a.Server.Host = b.Server.Host
	}
	if b.Server.Port != 0 {
		a.Server.Port = b.Server.Port
	}
	if b.Server.ShutdownTimeoutMs != 0 {
		a.Server.ShutdownTimeoutMs = b.Server.ShutdownTimeoutMs
	}
	if b.Storage.Driver != "" {
		a.Storage.Driver = b.Storage.Driver
	}
	if b.Storage.SQLite.Path != "" {
		a.Storage.SQLite.Path = b.Storage.SQLite.Path
	}
	return a
}


---

5) internal/storage/sqlite (마이그레이션 + 연결)

internal/storage/storage.go

package storage

import "context"

type Storage interface {
	Open(ctx context.Context) error
	Close() error
	Health(ctx context.Context) error
}

internal/storage/sqlite/sqlite.go

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	path string
	db   *sql.DB
}

func New(path string) *DB {
	return &DB{path: path}
}

func (d *DB) Open(ctx context.Context) error {
	if d.path == "" {
		return errors.New("sqlite path required")
	}
	if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", d.path+"?_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return err
	}
	d.db = db
	if err := d.Ping(ctx); err != nil {
		_ = db.Close()
		return err
	}
	return nil
}

func (d *DB) Close() error {
	if d.db == nil {
		return nil
	}
	return d.db.Close()
}

func (d *DB) Ping(ctx context.Context) error {
	if d.db == nil {
		return errors.New("db not open")
	}
	return d.db.PingContext(ctx)
}

func (d *DB) Health(ctx context.Context) error {
	return d.Ping(ctx)
}

func (d *DB) Raw() *sql.DB {
	return d.db
}

internal/storage/sqlite/migrate.go

package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
)

//go:embed migrations/0001_init.sql
var mig0001 string

//go:embed migrations/0002_usage.sql
var mig0002 string

type Migration struct {
	ID   int
	Name string
	SQL  string
}

func Migrations() []Migration {
	return []Migration{
		{ID: 1, Name: "init", SQL: mig0001},
		{ID: 2, Name: "usage", SQL: mig0002},
	}
}

func EnsureMigrations(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("db nil")
	}

	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations(
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at TEXT NOT NULL
);`); err != nil {
		return err
	}

	applied := map[int]bool{}
	rows, err := db.QueryContext(ctx, `SELECT id FROM schema_migrations`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		applied[id] = true
	}

	for _, m := range Migrations() {
		if applied[m.ID] {
			continue
		}
		if err := applyOne(ctx, db, m); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmts := splitSQL(m.SQL)
	for _, s := range stmts {
		if strings.TrimSpace(s) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("migration %d(%s) failed: %w", m.ID, m.Name, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(id, name, applied_at) VALUES(?, ?, datetime('now'))`,
		m.ID, m.Name,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func splitSQL(sqlText string) []string {
	// 매우 단순 분리. (추후 개선 가능)
	parts := strings.Split(sqlText, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p+";")
	}
	return out
}

internal/storage/sqlite/migrations/0001_init.sql

PRAGMA journal_mode=WAL;

CREATE TABLE IF NOT EXISTS kv (
  k TEXT PRIMARY KEY,
  v BLOB NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_log (
  id TEXT PRIMARY KEY,
  ts TEXT NOT NULL,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  target TEXT NOT NULL,
  meta_json TEXT NOT NULL
);

internal/storage/sqlite/migrations/0002_usage.sql

CREATE TABLE IF NOT EXISTS usage_ledger (
  id TEXT PRIMARY KEY,
  ts TEXT NOT NULL,
  workspace_id TEXT NOT NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  input_tokens INTEGER NOT NULL,
  output_tokens INTEGER NOT NULL,
  cost_usd REAL NOT NULL,
  meta_json TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_ledger_ts ON usage_ledger(ts);
CREATE INDEX IF NOT EXISTS idx_usage_ledger_workspace ON usage_ledger(workspace_id);


---

6) internal/server (HTTP 서버 + health 라우트)

> 외부 프레임워크 없이 표준 net/http로 시작합니다. (나중에 Hono 같은 구조를 Go식으로 확장 가능)



internal/server/server.go

package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"devorch/internal/config"
	"devorch/internal/log"
)

type Server struct {
	cfg    config.ServerConfig
	logger *log.Logger

	httpServer *http.Server
	ln         net.Listener
	mux        *http.ServeMux
}

type Input struct {
	Config config.ServerConfig
	Logger *log.Logger
}

func New(in Input) *Server {
	mux := http.NewServeMux()
	return &Server{
		cfg:    in.Config,
		logger: in.Logger,
		mux:    mux,
	}
}

func (s *Server) Mux() *http.ServeMux { return s.mux }

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.ln = ln

	s.httpServer = &http.Server{
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			if s.logger != nil {
				s.logger.Error("http serve error", "err", err)
			}
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

internal/server/routes/health.go

package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"devorch/internal/global"
	"devorch/internal/storage"
)

type HealthHandler struct {
	Build   global.BuildInfo
	Storage storage.Storage
}

func (h HealthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handle)
}

func (h HealthHandler) handle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
	defer cancel()

	dbErr := ""
	if h.Storage != nil {
		if err := h.Storage.Health(ctx); err != nil {
			dbErr = err.Error()
		}
	}

	resp := map[string]any{
		"ok":        dbErr == "",
		"version":   h.Build.Version,
		"commit":    h.Build.Commit,
		"buildDate": h.Build.BuildDate,
		"storage": map[string]any{
			"ok":  dbErr == "",
			"err": dbErr,
		},
		"ts": time.Now().UTC().Format(time.RFC3339Nano),
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}


---

7) internal/app (앱 조립/라이프사이클)

internal/app/app.go

package app

import (
	"context"
	"errors"

	"devorch/internal/config"
	"devorch/internal/global"
	"devorch/internal/log"
	"devorch/internal/server"
	"devorch/internal/server/routes"
	"devorch/internal/storage"
	"devorch/internal/storage/sqlite"
)

type App struct {
	build  global.BuildInfo
	paths  global.Paths
	logger *log.Logger
	cfg    config.Config

	store storage.Storage
	srv   *server.Server
}

type Input struct {
	Build  global.BuildInfo
	Paths  global.Paths
	Logger *log.Logger
	Config config.Config
}

func New(in Input) (*App, error) {
	if in.Logger == nil {
		return nil, errors.New("logger required")
	}

	var st storage.Storage
	switch in.Config.Storage.Driver {
	case "sqlite", "":
		st = sqlite.New(in.Config.Storage.SQLite.Path)
	default:
		return nil, errors.New("unsupported storage driver")
	}

	srv := server.New(server.Input{
		Config: in.Config.Server,
		Logger: in.Logger,
	})

	return &App{
		build:  in.Build,
		paths:  in.Paths,
		logger: in.Logger,
		cfg:    in.Config,
		store:  st,
		srv:    srv,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	// storage open + migrations
	if err := a.store.Open(ctx); err != nil {
		return err
	}

	// apply migrations if sqlite
	if s, ok := a.store.(*sqlite.DB); ok {
		if err := sqlite.EnsureMigrations(ctx, s.Raw()); err != nil {
			return err
		}
	}

	// routes
	routes.HealthHandler{Build: a.build, Storage: a.store}.Register(a.srv.Mux())

	// start server
	if err := a.srv.Start(); err != nil {
		return err
	}
	return nil
}

func (a *App) Stop(ctx context.Context) error {
	if a.srv != nil {
		_ = a.srv.Stop(ctx)
	}
	if a.store != nil {
		_ = a.store.Close()
	}
	return nil
}


---

8) internal/cli (최소 커맨드: version, paths, ping)

internal/cli/cli.go

package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"devorch/internal/global"
	"devorch/internal/log"
)

type Input struct {
	Args   []string
	Build  global.BuildInfo
	Paths  global.Paths
	Logger *log.Logger
	Env    []string
	Stdout io.Writer
	Stderr io.Writer
}

func Run(in Input) int {
	if in.Stdout == nil {
		in.Stdout = os.Stdout
	}
	if in.Stderr == nil {
		in.Stderr = os.Stderr
	}

	if len(in.Args) == 0 {
		printHelp(in.Stdout)
		return 0
	}

	switch in.Args[0] {
	case "version":
		fmt.Fprintf(in.Stdout, "devorch %s (%s)\n", in.Build.Version, in.Build.Commit)
		return 0
	case "paths":
		fmt.Fprintf(in.Stdout, "home:   %s\n", in.Paths.Home)
		fmt.Fprintf(in.Stdout, "config: %s\n", in.Paths.Config)
		fmt.Fprintf(in.Stdout, "data:   %s\n", in.Paths.Data)
		fmt.Fprintf(in.Stdout, "state:  %s\n", in.Paths.State)
		fmt.Fprintf(in.Stdout, "cache:  %s\n", in.Paths.Cache)
		fmt.Fprintf(in.Stdout, "log:    %s\n", in.Paths.Log)
		return 0
	case "ping":
		// devorch ping http://127.0.0.1:4096/health
		url := "http://127.0.0.1:4096/health"
		if len(in.Args) >= 2 {
			url = in.Args[1]
		}
		return ping(in.Stdout, in.Stderr, url)
	default:
		fmt.Fprintln(in.Stderr, "unknown command:", in.Args[0])
		printHelp(in.Stderr)
		return 2
	}
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  devorch version")
	fmt.Fprintln(w, "  devorch paths")
	fmt.Fprintln(w, "  devorch ping [url]")
}

func ping(out, errw io.Writer, url string) int {
	client := &http.Client{Timeout: 2 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "devorch-cli")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(errw, "ping failed:", err)
		return 1
	}
	defer resp.Body.Close()

	fmt.Fprintln(out, "status:", resp.Status)
	b, _ := io.ReadAll(resp.Body)
	fmt.Fprintln(out, strings.TrimSpace(string(b)))
	return 0
}


---

9) 실행 방법(확인)

# daemon 실행
go run ./cmd/devorchd

# 다른 터미널에서
go run ./cmd/devorch version
go run ./cmd/devorch paths
go run ./cmd/devorch ping


---

다음 개발 배치(바로 이어서)

원하신 트리 기준으로 “다음에 바로 채울” 핵심은 아래 순서가 가장 안전합니다.

1. config JSONC + merge 우선순위 + schema 검증 (OpenCode 방식)


2. auth/OAuth(PKCE + device flow) + token store (OS별 keychain/credman/libsecret)


3. provider 인터페이스 + openrouter/ollama 최소 호출 + streaming


4. modelresolver(override→fallback→default) + router(score/affinity)


5. session 파이프라인(메시지/요약/리트라이/컴팩션) + tool registry



원하시면 다음 응답은 (1) config 전체 + JSONC 파서 + sources/merge/validate를 풀코드로 이어서 드릴게요.