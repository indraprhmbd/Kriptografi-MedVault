package database

import (
	"path/filepath"
	"testing"
	"time"
)

func TestOpenEnablesForeignKeysAndWAL(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "pragma.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys must be 1, got %d", fk)
	}

	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode must be wal, got %q", mode)
	}
}

func TestOpenCreatesAllTables(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "schema.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	expected := []string{
		"users", "webauthn_credentials", "auth_challenges", "web_sessions",
		"app_sessions", "patients", "records", "audit_blocks",
	}

	for _, table := range expected {
		var name string
		err := db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("table %s is missing: %v", table, err)
		}
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "twice.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	first.Close()

	second, err := Open(path)
	if err != nil {
		t.Fatalf("second Open must succeed: %v", err)
	}
	defer second.Close()

	var count int
	if err := second.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'",
	).Scan(&count); err != nil {
		t.Fatalf("count users table: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one users table, got %d", count)
	}
}

func TestForeignKeysAreEnforced(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "fk.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(
		`INSERT INTO records (patient_id, doctor_id, diagnosis_super_enc, prescription_super_enc,
		 record_state_hash, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`,
		999, 999, "x", "y", "z", now, now,
	)
	if err == nil {
		t.Fatal("insert with a non existent patient must be rejected by the foreign key")
	}
}

func TestUniqueConstraints(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "unique.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	insert := `INSERT INTO patients (medical_record_number, nik_enc, full_name_enc, address_enc, birth_date, created_at)
	           VALUES (?,?,?,?,?,?)`

	if _, err := db.Exec(insert, "RM-2026-0001", "a", "b", "c", "1990-01-01", now); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := db.Exec(insert, "RM-2026-0001", "a", "b", "c", "1990-01-01", now); err == nil {
		t.Fatal("duplicate medical record number must be rejected")
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestStatsReportsPool(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "stats.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	stats := Stats(db)
	if stats["max_open"] != 1 {
		t.Fatalf("expected max_open 1, got %d", stats["max_open"])
	}
}
