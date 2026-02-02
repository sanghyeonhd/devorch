좋습니다. **34단계는 “ULID(외부 의존성 없이) + SQLite 스토리지 초기화 + 0001~0003 마이그레이션 자동 적용(임베드 포함)”**까지 풀코드로 갑니다.
(이 단계가 끝나면 devorchd가 뜰 때 DB가 자동으로 생성/업그레이드 됩니다.)

아래 34단계 추가/변경 파일 전체입니다.


---

34단계 포함 파일 목록 (풀코드)

internal/id/ulid.go

internal/storage/sqlite/sqlite.go

internal/storage/sqlite/migrator.go

internal/storage/sqlite/migrations/0001_init.sql

internal/storage/sqlite/migrations/0002_usage.sql

internal/storage/sqlite/migrations/0003_okAON_learning.sql (33단계와 동일하지만, 임베드 일관성 위해 포함)



---

1) internal/id/ulid.go

package id

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"time"
)

// Crockford Base32 (no I,L,O,U)
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ULID is 26 chars: 48-bit timestamp(ms) + 80-bit entropy => base32 encoding.
type ULID string

func NewULID() string {
	var b [16]byte

	// 48-bit timestamp in ms
	ms := uint64(time.Now().UnixMilli())
	// write 6 bytes big-endian
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	// 80-bit entropy
	_, _ = rand.Read(b[6:])

	return encodeBase32(b[:])
}

func encodeBase32(src []byte) string {
	// ULID uses 128 bits => 26 base32 chars (130 bits, top 2 bits zero)
	// We pack bits and emit 5-bit groups.
	out := make([]byte, 26)

	var (
		buffer uint32
		bits   uint8
		oi     int
	)

	for i := 0; i < len(src); i++ {
		buffer = (buffer << 8) | uint32(src[i])
		bits += 8
		for bits >= 5 {
			bits -= 5
			idx := (buffer >> bits) & 0x1F
			out[oi] = crockford[idx]
			oi++
			if oi == 26 {
				return string(out)
			}
		}
	}
	// remaining bits (should be 2 bits); pad with zeros to 5
	if oi < 26 {
		idx := (buffer << (5 - bits)) & 0x1F
		out[oi] = crockford[idx]
		oi++
	}
	for oi < 26 {
		out[oi] = crockford[0]
		oi++
	}
	return string(out)
}

func DecodeTimeMillis(ulid string) (int64, error) {
	b, err := decodeBase32(ulid)
	if err != nil {
		return 0, err
	}
	// first 6 bytes => 48-bit ms
	ms := (uint64(b[0]) << 40) |
		(uint64(b[1]) << 32) |
		(uint64(b[2]) << 24) |
		(uint64(b[3]) << 16) |
		(uint64(b[4]) << 8) |
		uint64(b[5])
	return int64(ms), nil
}

func decodeBase32(s string) ([]byte, error) {
	if len(s) != 26 {
		return nil, errors.New("invalid ulid length")
	}

	// build reverse map
	var rev [256]int16
	for i := range rev {
		rev[i] = -1
	}
	for i := 0; i < len(crockford); i++ {
		rev[crockford[i]] = int16(i)
	}
	// Accept lower-case
	for i := 0; i < len(crockford); i++ {
		c := crockford[i]
		if c >= 'A' && c <= 'Z' {
			rev[c+('a'-'A')] = int16(i)
		}
	}

	// 26 chars * 5 bits = 130 bits. We need 128 bits => drop top 2 bits.
	// We'll parse into 16 bytes.
	var (
		acc   uint32
		bits  uint8
		dst   [16]byte
		di    int
		total int
	)

	for i := 0; i < 26; i++ {
		v := rev[s[i]]
		if v < 0 {
			return nil, errors.New("invalid ulid character")
		}
		acc = (acc << 5) | uint32(v)
		bits += 5
		total += 5

		// once we have 8 bits, emit byte
		for bits >= 8 && di < 16 {
			bits -= 8
			dst[di] = byte(acc >> bits)
			di++
		}
	}

	// ulid has 2 extra bits; ignore remainder
	return dst[:], nil
}

// Optional helper: stable 64-bit hash from ULID for sharding, etc.
func Hash64(ulid string) (uint64, error) {
	b, err := decodeBase32(ulid)
	if err != nil {
		return 0, err
	}
	// simple XOR fold
	return binary.BigEndian.Uint64(b[0:8]) ^ binary.BigEndian.Uint64(b[8:16]), nil
}


---

2) internal/storage/sqlite/sqlite.go

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-go sqlite driver
)

// OpenOptions configures sqlite open + pragmas.
type OpenOptions struct {
	Path string // full path to db file
	// performance / durability
	BusyTimeout time.Duration
	WAL         bool
	ForeignKeys bool
	Synchronous string // "NORMAL"|"FULL"|"OFF"
	CacheKB     int
}

func DefaultOptions(dbPath string) OpenOptions {
	return OpenOptions{
		Path:        dbPath,
		BusyTimeout: 5 * time.Second,
		WAL:         true,
		ForeignKeys: true,
		Synchronous: "NORMAL",
		CacheKB:     64 * 1024, // 64MB
	}
}

func Open(ctx context.Context, opt OpenOptions) (*sql.DB, error) {
	if opt.Path == "" {
		return nil, fmt.Errorf("sqlite: empty db path")
	}

	dir := filepath.Dir(opt.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("sqlite: mkdir %s: %w", dir, err)
	}

	// modernc sqlite DSN: "file:/path/to.db"
	dsn := "file:" + opt.Path
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}

	// keep small pool; sqlite is single-writer
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := pingWithTimeout(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Pragmas
	if err := applyPragmas(ctx, db, opt); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func pingWithTimeout(ctx context.Context, db *sql.DB) error {
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(c); err != nil {
		return fmt.Errorf("sqlite: ping: %w", err)
	}
	return nil
}

func applyPragmas(ctx context.Context, db *sql.DB, opt OpenOptions) error {
	// busy timeout
	if opt.BusyTimeout > 0 {
		ms := int(opt.BusyTimeout.Milliseconds())
		if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d;", ms)); err != nil {
			return fmt.Errorf("sqlite: pragma busy_timeout: %w", err)
		}
	}

	if opt.ForeignKeys {
		if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON;"); err != nil {
			return fmt.Errorf("sqlite: pragma foreign_keys: %w", err)
		}
	} else {
		if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=OFF;"); err != nil {
			return fmt.Errorf("sqlite: pragma foreign_keys: %w", err)
		}
	}

	if opt.WAL {
		if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL;"); err != nil {
			return fmt.Errorf("sqlite: pragma journal_mode: %w", err)
		}
	} else {
		if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=DELETE;"); err != nil {
			return fmt.Errorf("sqlite: pragma journal_mode: %w", err)
		}
	}

	if opt.Synchronous != "" {
		if _, err := db.ExecContext(ctx, "PRAGMA synchronous="+opt.Synchronous+";"); err != nil {
			return fmt.Errorf("sqlite: pragma synchronous: %w", err)
		}
	}

	if opt.CacheKB != 0 {
		// negative means KB in memory
		if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA cache_size=%d;", -opt.CacheKB)); err != nil {
			return fmt.Errorf("sqlite: pragma cache_size: %w", err)
		}
	}

	// recommended for concurrency
	if _, err := db.ExecContext(ctx, "PRAGMA temp_store=MEMORY;"); err != nil {
		return fmt.Errorf("sqlite: pragma temp_store: %w", err)
	}
	return nil
}

> 중요: modernc.org/sqlite는 CGO 없이 멀티 OS(Windows/macOS/Linux)에서 동일하게 빌드가 잘 됩니다. (Devorch 목표와 잘 맞습니다)




---

3) internal/storage/sqlite/migrator.go  (go:embed로 0001~0003 포함 + 자동 적용)

package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Migration struct {
	Version  string
	Filename string
	SQL      string
	Hash     string
}

type Migrator struct {
	DB *sql.DB
}

func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{DB: db}
}

func (m *Migrator) Ensure(ctx context.Context) error {
	if m.DB == nil {
		return fmt.Errorf("sqlite migrator: nil db")
	}
	// schema table
	_, err := m.DB.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  hash TEXT NOT NULL,
  applied_at INTEGER NOT NULL
);
`)
	return err
}

func (m *Migrator) ApplyAll(ctx context.Context) error {
	if err := m.Ensure(ctx); err != nil {
		return err
	}

	migs, err := loadMigrations()
	if err != nil {
		return err
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].Filename < migs[j].Filename })

	applied, err := m.appliedSet(ctx)
	if err != nil {
		return err
	}

	for _, mg := range migs {
		if ah, ok := applied[mg.Version]; ok {
			// verify hash (detect tampering)
			if ah != mg.Hash {
				return fmt.Errorf("sqlite migrator: migration hash mismatch: %s (db=%s file=%s)", mg.Version, ah, mg.Hash)
			}
			continue
		}

		if err := m.applyOne(ctx, mg); err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) appliedSet(ctx context.Context) (map[string]string, error) {
	rows, err := m.DB.QueryContext(ctx, `SELECT version, hash FROM schema_migrations;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var v, h string
		if err := rows.Scan(&v, &h); err != nil {
			return nil, err
		}
		out[v] = h
	}
	return out, rows.Err()
}

func (m *Migrator) applyOne(ctx context.Context, mg Migration) error {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Split is intentionally NOT done: SQL files must be valid as one script.
	if _, err := tx.ExecContext(ctx, mg.SQL); err != nil {
		return fmt.Errorf("sqlite migrator: apply %s (%s): %w", mg.Version, mg.Filename, err)
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO schema_migrations(version, hash, applied_at)
VALUES(?, ?, ?);
`, mg.Version, mg.Hash, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("sqlite migrator: record %s: %w", mg.Version, err)
	}

	return tx.Commit()
}

func loadMigrations() ([]Migration, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}

	var out []Migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		b, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return nil, err
		}
		sqlText := string(b)

		ver := strings.SplitN(name, "_", 2)[0] // "0001"
		h := sha256.Sum256([]byte(sqlText))
		out = append(out, Migration{
			Version:  ver,
			Filename: name,
			SQL:      sqlText,
			Hash:     hex.EncodeToString(h[:]),
		})
	}

	return out, nil
}


---

4) internal/storage/sqlite/migrations/0001_init.sql

BEGIN;

-- base tables (minimal)
CREATE TABLE IF NOT EXISTS projects (
  project_id TEXT PRIMARY KEY,
  path TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  session_id TEXT PRIMARY KEY,
  project_id TEXT,
  title TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  FOREIGN KEY(project_id) REFERENCES projects(project_id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS messages (
  message_id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  role TEXT NOT NULL,        -- user/assistant/system/tool
  content TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(session_id) REFERENCES sessions(session_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);

COMMIT;


---

5) internal/storage/sqlite/migrations/0002_usage.sql

BEGIN;

-- usage ledger (token/cost)
CREATE TABLE IF NOT EXISTS usage_ledger (
  usage_id TEXT PRIMARY KEY,
  session_id TEXT,
  provider TEXT,
  model_id TEXT,
  input_tokens INTEGER,
  output_tokens INTEGER,
  cost_usd REAL,
  latency_ms INTEGER,
  ok INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(session_id) REFERENCES sessions(session_id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_time ON usage_ledger(created_at);
CREATE INDEX IF NOT EXISTS idx_usage_model ON usage_ledger(model_id);

COMMIT;


---

6) internal/storage/sqlite/migrations/0003_okAON_learning.sql

> 33단계에서 드린 SQL과 동일합니다. (임베드 일관성 위해 그대로 포함)



BEGIN;

CREATE TABLE IF NOT EXISTS okaon_env (
  env_id        TEXT PRIMARY KEY,
  os            TEXT NOT NULL,
  arch          TEXT NOT NULL,
  hostname      TEXT,
  cpu_model     TEXT,
  cpu_cores     INTEGER,
  mem_bytes     INTEGER,
  gpu_model     TEXT,
  runtime       TEXT NOT NULL,
  runtime_ver   TEXT,
  created_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_env_os_arch ON okaon_env(os, arch);

CREATE TABLE IF NOT EXISTS okaon_model (
  model_id      TEXT PRIMARY KEY,
  provider      TEXT NOT NULL,
  family        TEXT,
  context       INTEGER,
  notes         TEXT,
  created_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_model_provider ON okaon_model(provider);

CREATE TABLE IF NOT EXISTS okaon_workload (
  workload_id   TEXT PRIMARY KEY,
  kind          TEXT NOT NULL,
  language      TEXT,
  repo_hash     TEXT,
  size_bucket   TEXT,
  created_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_workload_kind ON okaon_workload(kind);

CREATE TABLE IF NOT EXISTS okaon_run (
  run_id        TEXT PRIMARY KEY,
  env_id        TEXT NOT NULL REFERENCES okaon_env(env_id) ON DELETE CASCADE,
  workload_id   TEXT NOT NULL REFERENCES okaon_workload(workload_id) ON DELETE CASCADE,
  model_id      TEXT NOT NULL REFERENCES okaon_model(model_id) ON DELETE CASCADE,

  started_at    INTEGER NOT NULL,
  finished_at   INTEGER NOT NULL,

  input_tokens  INTEGER,
  output_tokens INTEGER,

  latency_ms    INTEGER NOT NULL,
  ok            INTEGER NOT NULL,
  err_type      TEXT,
  err_msg       TEXT,

  quality_score REAL,
  cost_usd      REAL,
  extra_json    TEXT
);

CREATE INDEX IF NOT EXISTS idx_okaon_run_env_workload ON okaon_run(env_id, workload_id);
CREATE INDEX IF NOT EXISTS idx_okaon_run_model ON okaon_run(model_id);
CREATE INDEX IF NOT EXISTS idx_okaon_run_time ON okaon_run(started_at);

CREATE TABLE IF NOT EXISTS okaon_reward (
  reward_id     TEXT PRIMARY KEY,
  run_id        TEXT NOT NULL REFERENCES okaon_run(run_id) ON DELETE CASCADE,
  reward        REAL NOT NULL,
  reason        TEXT,
  created_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_reward_run ON okaon_reward(run_id);

CREATE TABLE IF NOT EXISTS okaon_bandit_snapshot (
  snapshot_id   TEXT PRIMARY KEY,
  env_id        TEXT NOT NULL REFERENCES okaon_env(env_id) ON DELETE CASCADE,
  workload_kind TEXT NOT NULL,
  algo          TEXT NOT NULL,
  state_json    TEXT NOT NULL,
  created_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_okaon_snapshot_env_kind ON okaon_bandit_snapshot(env_id, workload_kind);

COMMIT;


---

34단계 적용 방법 (코드 연결 포인트)

이 단계의 “DB 자동 준비”를 실제로 쓰려면, 데몬 시작 시 아래처럼 호출하면 됩니다(참고):

sqlite.Open(...)

sqlite.NewMigrator(db).ApplyAll(ctx)


원하시면 35단계에서 제가 internal/app/app.go 또는 cmd/devorchd/main.go에 실제 부팅 코드까지 붙여드리겠습니다.
바로 이어서 **35단계(bench recorder → OkAON run/reward 자동 기록)**로 진행할까요?