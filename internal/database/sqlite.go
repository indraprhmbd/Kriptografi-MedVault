// Package database opens the pure Go SQLite database and applies the schema.
//
// The driver is modernc.org/sqlite, so the binary builds with CGO_ENABLED=0.
// Connection level PRAGMAs are expressed as DSN parameters because that form is
// applied by the driver before the first statement runs, which is required for
// foreign_keys and journal_mode to take effect on every pooled connection.
package database

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations.sql
var schemaSQL string

// Open creates the database file if needed, applies the schema, and returns a
// pool tuned for a single writer on a resource constrained host.
func Open(path string) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path must not be empty")
	}

	dsn := buildDSN(path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	// A single writer avoids SQLITE_BUSY churn. MedVault traffic is one doctor
	// at a time during the demo, so this costs nothing and removes a whole
	// class of lock contention.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite %s: %w", path, err)
	}

	if err := applySchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// buildDSN converts a plain filesystem path into a SQLite URI with the PRAGMA
// set the application relies on.
func buildDSN(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	// Windows paths need forward slashes inside a file URI.
	uriPath := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}

	params := url.Values{}
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Add("_pragma", "busy_timeout(5000)")

	return "file:" + uriPath + "?" + params.Encode()
}

func applySchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	// Verify the critical pragmas actually took effect. A silent failure here
	// would let orphaned rows and torn writes go unnoticed.
	var fk int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		return fmt.Errorf("read foreign_keys pragma: %w", err)
	}
	if fk != 1 {
		return errors.New("sqlite foreign_keys pragma is not enabled")
	}

	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		return fmt.Errorf("read journal_mode pragma: %w", err)
	}
	if !strings.EqualFold(mode, "wal") {
		return fmt.Errorf("sqlite journal_mode is %q, expected wal", mode)
	}

	return nil
}

// Stats reports current connection pool counters for the telemetry payload.
func Stats(db *sql.DB) map[string]int {
	s := db.Stats()
	return map[string]int{
		"open_connections": s.OpenConnections,
		"in_use":           s.InUse,
		"idle":             s.Idle,
		"max_open":         s.MaxOpenConnections,
	}
}
