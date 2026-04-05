package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateCreatesTables(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	err = RunMigrations(db, "../../migrations")
	if err != nil {
		t.Fatal(err)
	}

	tables := []string{"incidents", "rca_reports", "evidence_items", "recommendations", "feedback"}
	for _, tbl := range tables {
		var count int
		err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("table %s not found", tbl)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	// Run migrations twice — should not error
	if err := RunMigrations(db, "../../migrations"); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if err := RunMigrations(db, "../../migrations"); err != nil {
		t.Fatalf("second migration: %v", err)
	}

	// Verify tables still exist
	var count int
	err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='incidents'").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Error("incidents table missing after idempotent migration")
	}
}

func TestMigrateCreatesIndexes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	if err := RunMigrations(db, "../../migrations"); err != nil {
		t.Fatal(err)
	}

	indexes := []string{
		"idx_incidents_service",
		"idx_incidents_status",
		"idx_incidents_severity",
		"idx_reports_incident",
		"idx_evidence_report",
		"idx_feedback_report",
	}
	for _, idx := range indexes {
		var count int
		err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&count)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("index %s not found", idx)
		}
	}
}
