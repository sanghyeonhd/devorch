// Package sqlite provides SQLite database initialization and boot utilities.
// Phase 34/35: OpenAndMigrate for daemon startup
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// OpenAndMigrate opens the SQLite database and applies all pending migrations.
func OpenAndMigrate(ctx context.Context, dbPath string) (*sql.DB, error) {
	db, err := Open(dbPath)
	if err != nil {
		return nil, err
	}

	// Migrations are already applied in Open()
	// Verify database is accessible
	if err := pingDB(ctx, db.SQL); err != nil {
		_ = db.SQL.Close()
		return nil, fmt.Errorf("sqlite: ping after migrate: %w", err)
	}

	return db.SQL, nil
}

// OpenWithOptions opens SQLite with custom options.
func OpenWithOptions(ctx context.Context, dbPath string, opts DBOptions) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(opts.MaxOpenConns)
	db.SetMaxIdleConns(opts.MaxIdleConns)
	db.SetConnMaxLifetime(opts.ConnMaxLifetime)

	// Apply pragmas
	if err := applyPragmas(ctx, db, opts); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: migrations: %w", err)
	}

	return db, nil
}

// DBOptions configures SQLite database options.
type DBOptions struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	BusyTimeout     time.Duration
	WAL             bool
	ForeignKeys     bool
	Synchronous     string // "OFF", "NORMAL", "FULL"
	CacheKB         int
}

// DefaultDBOptions returns sensible defaults for SQLite.
func DefaultDBOptions() DBOptions {
	return DBOptions{
		MaxOpenConns:    8,
		MaxIdleConns:    8,
		ConnMaxLifetime: 30 * time.Minute,
		BusyTimeout:     5 * time.Second,
		WAL:             true,
		ForeignKeys:     true,
		Synchronous:     "NORMAL",
		CacheKB:         64 * 1024, // 64MB
	}
}

func applyPragmas(ctx context.Context, db *sql.DB, opts DBOptions) error {
	pragmas := []string{}

	if opts.BusyTimeout > 0 {
		ms := int(opts.BusyTimeout.Milliseconds())
		pragmas = append(pragmas, fmt.Sprintf("PRAGMA busy_timeout=%d;", ms))
	}

	if opts.ForeignKeys {
		pragmas = append(pragmas, "PRAGMA foreign_keys=ON;")
	} else {
		pragmas = append(pragmas, "PRAGMA foreign_keys=OFF;")
	}

	if opts.WAL {
		pragmas = append(pragmas, "PRAGMA journal_mode=WAL;")
	} else {
		pragmas = append(pragmas, "PRAGMA journal_mode=DELETE;")
	}

	if opts.Synchronous != "" {
		pragmas = append(pragmas, "PRAGMA synchronous="+opts.Synchronous+";")
	}

	if opts.CacheKB > 0 {
		// Negative means KB
		pragmas = append(pragmas, fmt.Sprintf("PRAGMA cache_size=%d;", -opts.CacheKB))
	}

	pragmas = append(pragmas, "PRAGMA temp_store=MEMORY;")

	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("sqlite: pragma %s: %w", pragma, err)
		}
	}

	return nil
}

func pingDB(ctx context.Context, db *sql.DB) error {
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return db.PingContext(c)
}

// CloseDB closes the database connection gracefully.
func CloseDB(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- db.Close()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
