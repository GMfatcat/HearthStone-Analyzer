package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenAndMigrateCreatesCoreTables(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "hearthstone.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	for _, tableName := range []string{
		"schema_migrations",
		"app_settings",
		"cards",
		"card_locales",
		"decks",
		"deck_cards",
		"meta_snapshots",
		"meta_decks",
		"analysis_reports",
		"card_sync_runs",
		"scheduled_jobs",
		"job_execution_logs",
	} {
		if !tableExists(t, db, tableName) {
			t.Fatalf("expected table %q to exist after migration", tableName)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "hearthstone.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
}

func tableExists(t *testing.T, db *sql.DB, tableName string) bool {
	t.Helper()

	var name string
	err := db.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, tableName).Scan(&name)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		t.Fatalf("tableExists query error = %v", err)
	}

	return name == tableName
}
