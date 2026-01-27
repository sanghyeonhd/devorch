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
