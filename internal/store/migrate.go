package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RunMigrations reads and applies all .up.sql migration files from the given
// directory path to the database. It tracks applied migrations in a
// schema_migrations table and is idempotent.
func RunMigrations(db *sql.DB, migrationsPath string) error {
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("store: ensure migrations table: %w", err)
	}

	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("store: read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, filepath.Join(migrationsPath, e.Name()))
		}
	}
	sort.Strings(files)

	for _, file := range files {
		version := filepath.Base(file)
		applied, err := isApplied(db, version)
		if err != nil {
			return fmt.Errorf("store: check migration %s: %w", file, err)
		}
		if applied {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("store: read migration %s: %w", file, err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("store: apply migration %s: %w", file, err)
		}

		if err := recordMigration(db, version); err != nil {
			return fmt.Errorf("store: record migration %s: %w", file, err)
		}
	}
	return nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func isApplied(db *sql.DB, version string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT count(*) FROM schema_migrations WHERE version = ?",
		version,
	).Scan(&count)
	return count > 0, err
}

func recordMigration(db *sql.DB, version string) error {
	_, err := db.Exec(
		"INSERT INTO schema_migrations (version) VALUES (?)",
		version,
	)
	return err
}
