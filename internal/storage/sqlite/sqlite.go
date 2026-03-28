package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type migration struct {
	version string
	sql     string
}

var migrations = []migration{
	{
		version: "0001_core_settings_and_jobs",
		sql: `
CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT,
    is_encrypted BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cards (
    id TEXT PRIMARY KEY,
    dbf_id INTEGER,
    class TEXT,
    card_type TEXT,
    set_name TEXT,
    rarity TEXT,
    cost INTEGER,
    attack INTEGER NULL,
    health INTEGER NULL,
    text TEXT,
    collectible BOOLEAN NOT NULL DEFAULT FALSE,
    standard_legal BOOLEAN NOT NULL DEFAULT FALSE,
    wild_legal BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS card_locales (
    card_id TEXT NOT NULL,
    locale TEXT NOT NULL,
    name TEXT NOT NULL,
    text TEXT,
    PRIMARY KEY (card_id, locale),
    FOREIGN KEY (card_id) REFERENCES cards(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS decks (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    external_ref TEXT,
    name TEXT,
    class TEXT NOT NULL,
    format TEXT NOT NULL,
    deck_code TEXT,
    archetype TEXT,
    deck_hash TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS deck_cards (
    deck_id TEXT NOT NULL,
    card_id TEXT NOT NULL,
    card_count INTEGER NOT NULL,
    PRIMARY KEY (deck_id, card_id),
    FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE,
    FOREIGN KEY (card_id) REFERENCES cards(id)
);

CREATE TABLE IF NOT EXISTS meta_snapshots (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    patch_version TEXT,
    format TEXT NOT NULL,
    rank_bracket TEXT,
    region TEXT,
    fetched_at TIMESTAMP NOT NULL,
    raw_payload TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS meta_decks (
    snapshot_id TEXT NOT NULL,
    deck_id TEXT NOT NULL,
    winrate REAL,
    playrate REAL,
    sample_size INTEGER,
    tier TEXT,
    PRIMARY KEY (snapshot_id, deck_id),
    FOREIGN KEY (snapshot_id) REFERENCES meta_snapshots(id) ON DELETE CASCADE,
    FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS analysis_reports (
    id TEXT PRIMARY KEY,
    deck_id TEXT NOT NULL,
    based_on_snapshot_id TEXT,
    report_type TEXT NOT NULL,
    input_hash TEXT NOT NULL,
    report_json TEXT NOT NULL,
    report_text TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (deck_id) REFERENCES decks(id) ON DELETE CASCADE,
    FOREIGN KEY (based_on_snapshot_id) REFERENCES meta_snapshots(id)
);

CREATE TABLE IF NOT EXISTS scheduled_jobs (
    key TEXT PRIMARY KEY,
    cron_expr TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at TIMESTAMP NULL,
    next_run_at TIMESTAMP NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS job_execution_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_key TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL,
    finished_at TIMESTAMP NULL,
    records_affected INTEGER NULL,
    error_message TEXT NULL,
    FOREIGN KEY (job_key) REFERENCES scheduled_jobs(key)
);

CREATE INDEX IF NOT EXISTS idx_job_execution_logs_job_key_started_at
ON job_execution_logs (job_key, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_cards_dbf_id
ON cards (dbf_id);

CREATE INDEX IF NOT EXISTS idx_decks_deck_hash
ON decks (deck_hash);

CREATE INDEX IF NOT EXISTS idx_meta_snapshots_fetched_at
ON meta_snapshots (fetched_at DESC);

CREATE INDEX IF NOT EXISTS idx_analysis_reports_deck_id_created_at
ON analysis_reports (deck_id, created_at DESC);
`,
	},
	{
		version: "0002_card_sync_runs",
		sql: `
CREATE TABLE IF NOT EXISTS card_sync_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    fetched_at TIMESTAMP NOT NULL,
    card_count INTEGER NOT NULL,
    raw_payload TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_card_sync_runs_fetched_at
ON card_sync_runs (fetched_at DESC);
`,
	},
	{
		version: "0003_cards_functional_tags",
		sql: `
ALTER TABLE cards ADD COLUMN functional_tags TEXT NOT NULL DEFAULT '[]';
`,
	},
	{
		version: "0004_cards_metadata",
		sql: `
ALTER TABLE cards ADD COLUMN metadata TEXT NOT NULL DEFAULT '{}';
`,
	},
}

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_busy_timeout=5000", filepath.ToSlash(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return err
	}

	for _, m := range migrations {
		applied, err := migrationApplied(ctx, db, m.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		if err := applyMigration(ctx, db, m); err != nil {
			return err
		}
	}

	return nil
}

func ensureSchemaMigrationsTable(ctx context.Context, db *sql.DB) error {
	const stmt = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	return nil
}

func migrationApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var found string
	err := db.QueryRowContext(ctx, `
		SELECT version
		FROM schema_migrations
		WHERE version = ?
	`, version).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query schema migration %q: %w", version, err)
	}

	return found == version, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", m.version, err)
	}

	statements := splitStatements(m.sql)
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %q: %w", m.version, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_migrations (version)
		VALUES (?)
	`, m.version); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %q: %w", m.version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", m.version, err)
	}

	return nil
}

func splitStatements(sqlBlock string) []string {
	parts := strings.Split(sqlBlock, ";")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}

		out = append(out, stmt)
	}

	return out
}
