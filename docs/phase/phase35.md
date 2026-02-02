좋습니다. **35단계는 “devorchd 부팅 시 SQLite DB 경로 결정 → Open → 마이그레이션(0001~0003) 자동 적용 → 정상 종료 시 정리”**까지 풀코드로 완성합니다.
(= 이제 데몬이 뜨면 DB가 항상 최신 스키마로 준비됩니다)

아래는 35단계 추가/변경 파일 전체입니다.


---

35단계 포함 파일 목록 (풀코드)

cmd/devorchd/main.go

internal/app/app.go

internal/app/lifecycle.go

internal/global/paths.go

internal/storage/sqlite/boot.go


> ⚠️ 주의: 이미 같은 파일이 있다면 이 코드로 덮어쓰기 기준입니다. (최소 의존/컴파일 가능 형태)




---

1) cmd/devorchd/main.go

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"devorch/internal/app"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// OS signal handling
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	application, err := app.NewApp()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "devorchd: init error:", err)
		os.Exit(1)
	}

	// start
	if err := application.Start(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "devorchd: start error:", err)
		_ = application.Stop(context.Background())
		os.Exit(1)
	}

	select {
	case sig := <-sigCh:
		_, _ = fmt.Fprintln(os.Stderr, "devorchd: received signal:", sig)
	case <-ctx.Done():
		_, _ = fmt.Fprintln(os.Stderr, "devorchd: context cancelled")
	}

	// stop with timeout
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()

	if err := application.Stop(stopCtx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "devorchd: stop error:", err)
		os.Exit(1)
	}
}


---

2) internal/app/app.go

package app

import (
	"context"
	"database/sql"
	"fmt"

	"devorch/internal/global"
	storesqlite "devorch/internal/storage/sqlite"
)

type App struct {
	db     *sql.DB
	dbPath string
}

func NewApp() (*App, error) {
	paths, err := global.ResolvePaths("devorch")
	if err != nil {
		return nil, err
	}

	// Allow override via env
	dbPath := global.EnvOr("DEVORCH_DB_PATH", paths.DataFile("devorch.db"))

	return &App{
		dbPath: dbPath,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	if a.db != nil {
		return nil
	}

	db, err := storesqlite.OpenAndMigrate(ctx, a.dbPath)
	if err != nil {
		return fmt.Errorf("app: db open+migrate: %w", err)
	}

	a.db = db
	return nil
}

func (a *App) Stop(ctx context.Context) error {
	if a.db == nil {
		return nil
	}
	// sqlite close is fast, but respect ctx anyway
	done := make(chan error, 1)
	go func() {
		done <- a.db.Close()
	}()

	select {
	case err := <-done:
		a.db = nil
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *App) DB() *sql.DB {
	return a.db
}

func (a *App) DBPath() string {
	return a.dbPath
}


---

3) internal/app/lifecycle.go

package app

// (Stage 35) lifecycle.go는 지금은 최소 형태로 유지합니다.
// 이후 단계에서 health/cleanup/background manager 등을 여기로 붙입니다.
//
// 현재는 App.Start/Stop이 라이프사이클의 단일 진입점입니다.


---

4) internal/global/paths.go

package global

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Paths struct {
	AppName string

	Home   string
	Config string
	Cache  string
	Data   string
	State  string
	Log    string
}

func EnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ResolvePaths(appName string) (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("paths: home: %w", err)
	}

	// XDG overrides (Linux and also respected elsewhere)
	xdgConfig := EnvOr("XDG_CONFIG_HOME", "")
	xdgCache := EnvOr("XDG_CACHE_HOME", "")
	xdgData := EnvOr("XDG_DATA_HOME", "")
	xdgState := EnvOr("XDG_STATE_HOME", "")

	var cfgBase, cacheBase, dataBase, stateBase string

	switch runtime.GOOS {
	case "darwin":
		// macOS convention
		cfgBase = filepath.Join(home, "Library", "Application Support")
		cacheBase = filepath.Join(home, "Library", "Caches")
		dataBase = cfgBase
		stateBase = cfgBase
	case "windows":
		// Roaming profile
		appData := EnvOr("APPDATA", filepath.Join(home, "AppData", "Roaming"))
		localAppData := EnvOr("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
		cfgBase = appData
		cacheBase = localAppData
		dataBase = appData
		stateBase = localAppData
	default:
		// linux/others
		cfgBase = filepath.Join(home, ".config")
		cacheBase = filepath.Join(home, ".cache")
		dataBase = filepath.Join(home, ".local", "share")
		stateBase = filepath.Join(home, ".local", "state")
	}

	if xdgConfig != "" {
		cfgBase = xdgConfig
	}
	if xdgCache != "" {
		cacheBase = xdgCache
	}
	if xdgData != "" {
		dataBase = xdgData
	}
	if xdgState != "" {
		stateBase = xdgState
	}

	p := Paths{
		AppName: appName,
		Home:    home,
		Config:  filepath.Join(cfgBase, appName),
		Cache:   filepath.Join(cacheBase, appName),
		Data:    filepath.Join(dataBase, appName),
		State:   filepath.Join(stateBase, appName),
	}

	// Logs: prefer Data/log
	p.Log = filepath.Join(p.Data, "log")

	// ensure dirs exist (best-effort)
	dirs := []string{p.Config, p.Cache, p.Data, p.State, p.Log}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return Paths{}, fmt.Errorf("paths: mkdir %s: %w", d, err)
		}
	}

	return p, nil
}

func (p Paths) ConfigFile(name string) string { return filepath.Join(p.Config, name) }
func (p Paths) CacheFile(name string) string  { return filepath.Join(p.Cache, name) }
func (p Paths) DataFile(name string) string   { return filepath.Join(p.Data, name) }
func (p Paths) StateFile(name string) string  { return filepath.Join(p.State, name) }
func (p Paths) LogFile(name string) string    { return filepath.Join(p.Log, name) }


---

5) internal/storage/sqlite/boot.go

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// OpenAndMigrate opens sqlite db + applies embedded migrations (0001~0003).
func OpenAndMigrate(ctx context.Context, dbPath string) (*sql.DB, error) {
	openOpt := DefaultOptions(dbPath)
	openOpt.BusyTimeout = 5 * time.Second
	openOpt.WAL = true
	openOpt.ForeignKeys = true
	openOpt.Synchronous = "NORMAL"
	openOpt.CacheKB = 64 * 1024

	db, err := Open(ctx, openOpt)
	if err != nil {
		return nil, err
	}

	m := NewMigrator(db)
	if err := m.ApplyAll(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: migrate: %w", err)
	}

	return db, nil
}


---

35단계 결과

이제 devorchd를 실행하면:

OS별 적절한 기본 경로에 devorch.db 생성

schema_migrations 생성

0001_init.sql → 0002_usage.sql → 0003_okAON_learning.sql 자동 적용

다음 실행부터는 해시 검증 후 스킵 (변조 감지)



---

다음(36단계)은 제가 권장하던 흐름대로 **OkAON CRUD(최소 insert/query)**부터 붙이면 바로 “실측 이력 저장”이 돌아갑니다.

원하시면 36단계로:

internal/okAON/sqlite/insert.go

internal/okAON/sqlite/query.go

internal/okAON/store.go

internal/okAON/models.go 까지 풀코드로 이어가겠습니다.