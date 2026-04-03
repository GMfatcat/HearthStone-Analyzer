package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Setting struct {
	Key         string
	Value       string
	IsEncrypted bool
	UpdatedAt   time.Time
}

type ScheduledJob struct {
	Key       string
	CronExpr  string
	Enabled   bool
	LastRunAt *time.Time
	NextRunAt *time.Time
	UpdatedAt time.Time
}

type JobExecutionLog struct {
	ID              int64
	JobKey          string
	Status          string
	StartedAt       time.Time
	FinishedAt      *time.Time
	RecordsAffected *int64
	ErrorMessage    *string
}

type MetaSnapshot struct {
	ID           string
	Source       string
	PatchVersion string
	Format       string
	RankBracket  *string
	Region       *string
	FetchedAt    time.Time
	RawPayload   string
	CreatedAt    time.Time
}

type Deck struct {
	ID          string
	Source      string
	ExternalRef *string
	Name        *string
	Class       string
	Format      string
	DeckCode    *string
	Archetype   *string
	DeckHash    *string
	CreatedAt   time.Time
}

type MetaDeck struct {
	SnapshotID string
	DeckID     string
	Winrate    *float64
	Playrate   *float64
	SampleSize *int
	Tier       *string
}

type AnalysisReport struct {
	ID                string
	DeckID            string
	BasedOnSnapshotID *string
	ReportType        string
	InputHash         string
	ReportJSON        string
	ReportText        *string
	CreatedAt         time.Time
}

type DeckCardRecord struct {
	DeckID    string
	CardID    string
	CardCount int
}

type ComparableMetaDeck struct {
	DeckID     string
	Name       *string
	Class      string
	Format     string
	DeckCode   *string
	Archetype  *string
	Winrate    *float64
	Playrate   *float64
	SampleSize *int
	Tier       *string
}

type SettingsRepository struct {
	db *sql.DB
}

type ScheduledJobsRepository struct {
	db *sql.DB
}

type JobExecutionLogsRepository struct {
	db *sql.DB
}

type MetaSnapshotsRepository struct {
	db *sql.DB
}

type DecksRepository struct {
	db *sql.DB
}

type MetaDecksRepository struct {
	db *sql.DB
}

type DeckCardsRepository struct {
	db *sql.DB
}

type AnalysisReportsRepository struct {
	db *sql.DB
}

type Repositories struct {
	Cards            *CardsRepository
	Settings         *SettingsRepository
	ScheduledJobs    *ScheduledJobsRepository
	JobExecutionLogs *JobExecutionLogsRepository
	MetaSnapshots    *MetaSnapshotsRepository
	Decks            *DecksRepository
	MetaDecks        *MetaDecksRepository
	DeckCards        *DeckCardsRepository
	AnalysisReports  *AnalysisReportsRepository
}

func NewRepositories(db *sql.DB) *Repositories {
	return &Repositories{
		Cards:            NewCardsRepository(db),
		Settings:         NewSettingsRepository(db),
		ScheduledJobs:    NewScheduledJobsRepository(db),
		JobExecutionLogs: NewJobExecutionLogsRepository(db),
		MetaSnapshots:    NewMetaSnapshotsRepository(db),
		Decks:            NewDecksRepository(db),
		MetaDecks:        NewMetaDecksRepository(db),
		DeckCards:        NewDeckCardsRepository(db),
		AnalysisReports:  NewAnalysisReportsRepository(db),
	}
}

func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

func NewScheduledJobsRepository(db *sql.DB) *ScheduledJobsRepository {
	return &ScheduledJobsRepository{db: db}
}

func NewJobExecutionLogsRepository(db *sql.DB) *JobExecutionLogsRepository {
	return &JobExecutionLogsRepository{db: db}
}

func NewMetaSnapshotsRepository(db *sql.DB) *MetaSnapshotsRepository {
	return &MetaSnapshotsRepository{db: db}
}

func NewDecksRepository(db *sql.DB) *DecksRepository {
	return &DecksRepository{db: db}
}

func NewMetaDecksRepository(db *sql.DB) *MetaDecksRepository {
	return &MetaDecksRepository{db: db}
}

func NewDeckCardsRepository(db *sql.DB) *DeckCardsRepository {
	return &DeckCardsRepository{db: db}
}

func NewAnalysisReportsRepository(db *sql.DB) *AnalysisReportsRepository {
	return &AnalysisReportsRepository{db: db}
}

func (r *SettingsRepository) Upsert(ctx context.Context, setting Setting) error {
	const stmt = `
INSERT INTO app_settings (key, value, is_encrypted, updated_at)
VALUES (?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(key) DO UPDATE SET
    value = excluded.value,
    is_encrypted = excluded.is_encrypted,
    updated_at = CURRENT_TIMESTAMP;
`
	if _, err := r.db.ExecContext(ctx, stmt, setting.Key, setting.Value, setting.IsEncrypted); err != nil {
		return fmt.Errorf("upsert setting %q: %w", setting.Key, err)
	}

	return nil
}

func (r *SettingsRepository) GetByKey(ctx context.Context, key string) (Setting, error) {
	const stmt = `
SELECT key, value, is_encrypted, updated_at
FROM app_settings
WHERE key = ?;
`
	var setting Setting
	if err := r.db.QueryRowContext(ctx, stmt, key).Scan(
		&setting.Key,
		&setting.Value,
		&setting.IsEncrypted,
		&setting.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return Setting{}, fmt.Errorf("get setting %q: %w", key, err)
		}

		return Setting{}, fmt.Errorf("get setting %q: %w", key, err)
	}

	return setting, nil
}

func (r *SettingsRepository) List(ctx context.Context) ([]Setting, error) {
	const stmt = `
SELECT key, value, is_encrypted, updated_at
FROM app_settings
ORDER BY key ASC;
`
	rows, err := r.db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer rows.Close()

	var settings []Setting
	for rows.Next() {
		var setting Setting
		if err := rows.Scan(
			&setting.Key,
			&setting.Value,
			&setting.IsEncrypted,
			&setting.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}

		settings = append(settings, setting)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings: %w", err)
	}

	return settings, nil
}

func (r *ScheduledJobsRepository) Upsert(ctx context.Context, job ScheduledJob) error {
	const stmt = `
INSERT INTO scheduled_jobs (key, cron_expr, enabled, last_run_at, next_run_at, updated_at)
VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(key) DO UPDATE SET
    cron_expr = excluded.cron_expr,
    enabled = excluded.enabled,
    last_run_at = excluded.last_run_at,
    next_run_at = excluded.next_run_at,
    updated_at = CURRENT_TIMESTAMP;
`
	if _, err := r.db.ExecContext(
		ctx,
		stmt,
		job.Key,
		job.CronExpr,
		job.Enabled,
		job.LastRunAt,
		job.NextRunAt,
	); err != nil {
		return fmt.Errorf("upsert scheduled job %q: %w", job.Key, err)
	}

	return nil
}

func (r *ScheduledJobsRepository) GetByKey(ctx context.Context, key string) (ScheduledJob, error) {
	const stmt = `
SELECT key, cron_expr, enabled, last_run_at, next_run_at, updated_at
FROM scheduled_jobs
WHERE key = ?;
`
	var job ScheduledJob
	if err := r.db.QueryRowContext(ctx, stmt, key).Scan(
		&job.Key,
		&job.CronExpr,
		&job.Enabled,
		&job.LastRunAt,
		&job.NextRunAt,
		&job.UpdatedAt,
	); err != nil {
		return ScheduledJob{}, fmt.Errorf("get scheduled job %q: %w", key, err)
	}

	return job, nil
}

func (r *ScheduledJobsRepository) List(ctx context.Context) ([]ScheduledJob, error) {
	const stmt = `
SELECT key, cron_expr, enabled, last_run_at, next_run_at, updated_at
FROM scheduled_jobs
ORDER BY key ASC;
`
	rows, err := r.db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("list scheduled jobs: %w", err)
	}
	defer rows.Close()

	var jobs []ScheduledJob
	for rows.Next() {
		var job ScheduledJob
		if err := rows.Scan(
			&job.Key,
			&job.CronExpr,
			&job.Enabled,
			&job.LastRunAt,
			&job.NextRunAt,
			&job.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan scheduled job: %w", err)
		}

		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scheduled jobs: %w", err)
	}

	return jobs, nil
}

func (r *JobExecutionLogsRepository) Create(ctx context.Context, log JobExecutionLog) error {
	const stmt = `
INSERT INTO job_execution_logs (
    job_key,
    status,
    started_at,
    finished_at,
    records_affected,
    error_message
)
VALUES (?, ?, ?, ?, ?, ?);
`
	result, err := r.db.ExecContext(
		ctx,
		stmt,
		log.JobKey,
		log.Status,
		log.StartedAt,
		log.FinishedAt,
		log.RecordsAffected,
		log.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("create job execution log for %q: %w", log.JobKey, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("read job execution log insert id: %w", err)
	}

	log.ID = id
	return nil
}

func (r *JobExecutionLogsRepository) ListByJobKey(ctx context.Context, jobKey string, limit int) ([]JobExecutionLog, error) {
	if limit <= 0 {
		limit = 50
	}

	const stmt = `
SELECT id, job_key, status, started_at, finished_at, records_affected, error_message
FROM job_execution_logs
WHERE job_key = ?
ORDER BY started_at DESC, id DESC
LIMIT ?;
`
	rows, err := r.db.QueryContext(ctx, stmt, jobKey, limit)
	if err != nil {
		return nil, fmt.Errorf("list job execution logs for %q: %w", jobKey, err)
	}
	defer rows.Close()

	var logs []JobExecutionLog
	for rows.Next() {
		var log JobExecutionLog
		if err := rows.Scan(
			&log.ID,
			&log.JobKey,
			&log.Status,
			&log.StartedAt,
			&log.FinishedAt,
			&log.RecordsAffected,
			&log.ErrorMessage,
		); err != nil {
			return nil, fmt.Errorf("scan job execution log: %w", err)
		}

		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate job execution logs: %w", err)
	}

	return logs, nil
}

func (r *MetaSnapshotsRepository) Create(ctx context.Context, snapshot MetaSnapshot) error {
	const stmt = `
INSERT INTO meta_snapshots (
    id,
    source,
    patch_version,
    format,
    rank_bracket,
    region,
    fetched_at,
    raw_payload
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    source = excluded.source,
    patch_version = excluded.patch_version,
    format = excluded.format,
    rank_bracket = excluded.rank_bracket,
    region = excluded.region,
    fetched_at = excluded.fetched_at,
    raw_payload = excluded.raw_payload;
`
	if _, err := r.db.ExecContext(
		ctx,
		stmt,
		snapshot.ID,
		snapshot.Source,
		snapshot.PatchVersion,
		snapshot.Format,
		snapshot.RankBracket,
		snapshot.Region,
		snapshot.FetchedAt,
		snapshot.RawPayload,
	); err != nil {
		return fmt.Errorf("create meta snapshot %q: %w", snapshot.ID, err)
	}

	return nil
}

func (r *MetaSnapshotsRepository) GetLatestByFormat(ctx context.Context, format string) (MetaSnapshot, error) {
	const stmt = `
SELECT id, source, patch_version, format, rank_bracket, region, fetched_at, raw_payload, created_at
FROM meta_snapshots
WHERE format = ?
ORDER BY fetched_at DESC, created_at DESC
LIMIT 1;
`
	var snapshot MetaSnapshot
	if err := r.db.QueryRowContext(ctx, stmt, format).Scan(
		&snapshot.ID,
		&snapshot.Source,
		&snapshot.PatchVersion,
		&snapshot.Format,
		&snapshot.RankBracket,
		&snapshot.Region,
		&snapshot.FetchedAt,
		&snapshot.RawPayload,
		&snapshot.CreatedAt,
	); err != nil {
		return MetaSnapshot{}, fmt.Errorf("get latest meta snapshot for format %q: %w", format, err)
	}

	return snapshot, nil
}

func (r *MetaSnapshotsRepository) ListByFormat(ctx context.Context, format string, limit int) ([]MetaSnapshot, error) {
	if limit <= 0 {
		limit = 20
	}

	const stmt = `
SELECT id, source, patch_version, format, rank_bracket, region, fetched_at, raw_payload, created_at
FROM meta_snapshots
WHERE format = ?
ORDER BY fetched_at DESC, created_at DESC
LIMIT ?;
`
	rows, err := r.db.QueryContext(ctx, stmt, format, limit)
	if err != nil {
		return nil, fmt.Errorf("list meta snapshots for format %q: %w", format, err)
	}
	defer rows.Close()

	var snapshots []MetaSnapshot
	for rows.Next() {
		var snapshot MetaSnapshot
		if err := rows.Scan(
			&snapshot.ID,
			&snapshot.Source,
			&snapshot.PatchVersion,
			&snapshot.Format,
			&snapshot.RankBracket,
			&snapshot.Region,
			&snapshot.FetchedAt,
			&snapshot.RawPayload,
			&snapshot.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan meta snapshot: %w", err)
		}

		snapshots = append(snapshots, snapshot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate meta snapshots: %w", err)
	}

	return snapshots, nil
}

func (r *MetaSnapshotsRepository) GetByID(ctx context.Context, id string) (MetaSnapshot, error) {
	const stmt = `
SELECT id, source, patch_version, format, rank_bracket, region, fetched_at, raw_payload, created_at
FROM meta_snapshots
WHERE id = ?;
`
	var snapshot MetaSnapshot
	if err := r.db.QueryRowContext(ctx, stmt, id).Scan(
		&snapshot.ID,
		&snapshot.Source,
		&snapshot.PatchVersion,
		&snapshot.Format,
		&snapshot.RankBracket,
		&snapshot.Region,
		&snapshot.FetchedAt,
		&snapshot.RawPayload,
		&snapshot.CreatedAt,
	); err != nil {
		return MetaSnapshot{}, fmt.Errorf("get meta snapshot %q: %w", id, err)
	}

	return snapshot, nil
}

func (r *DecksRepository) UpsertMetaDeck(ctx context.Context, deck Deck) (string, error) {
	return r.upsertDeck(ctx, deck)
}

func (r *DecksRepository) UpsertReportDeck(ctx context.Context, deck Deck) (string, error) {
	return r.upsertDeck(ctx, deck)
}

func (r *DecksRepository) upsertDeck(ctx context.Context, deck Deck) (string, error) {
	const stmt = `
INSERT INTO decks (id, source, external_ref, name, class, format, deck_code, archetype, deck_hash)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    source = excluded.source,
    external_ref = excluded.external_ref,
    name = excluded.name,
    class = excluded.class,
    format = excluded.format,
    deck_code = excluded.deck_code,
    archetype = excluded.archetype,
    deck_hash = excluded.deck_hash;
`
	if _, err := r.db.ExecContext(
		ctx,
		stmt,
		deck.ID,
		deck.Source,
		deck.ExternalRef,
		deck.Name,
		deck.Class,
		deck.Format,
		deck.DeckCode,
		deck.Archetype,
		deck.DeckHash,
	); err != nil {
		return "", fmt.Errorf("upsert deck %q: %w", deck.ID, err)
	}

	return deck.ID, nil
}

func (r *MetaDecksRepository) ReplaceSnapshotDecks(ctx context.Context, snapshotID string, items []MetaDeck) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace meta decks tx: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM meta_decks WHERE snapshot_id = ?`, snapshotID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete meta decks for snapshot %q: %w", snapshotID, err)
	}

	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO meta_decks (snapshot_id, deck_id, winrate, playrate, sample_size, tier)
VALUES (?, ?, ?, ?, ?, ?)
`, item.SnapshotID, item.DeckID, item.Winrate, item.Playrate, item.SampleSize, item.Tier); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert meta deck for snapshot %q deck %q: %w", item.SnapshotID, item.DeckID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace meta decks tx: %w", err)
	}

	return nil
}

func (r *MetaDecksRepository) ListBySnapshotID(ctx context.Context, snapshotID string) ([]MetaDeck, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT snapshot_id, deck_id, winrate, playrate, sample_size, tier
FROM meta_decks
WHERE snapshot_id = ?
ORDER BY playrate DESC, winrate DESC, deck_id ASC
`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("list meta decks for snapshot %q: %w", snapshotID, err)
	}
	defer rows.Close()

	var items []MetaDeck
	for rows.Next() {
		var item MetaDeck
		if err := rows.Scan(&item.SnapshotID, &item.DeckID, &item.Winrate, &item.Playrate, &item.SampleSize, &item.Tier); err != nil {
			return nil, fmt.Errorf("scan meta deck: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate meta decks: %w", err)
	}

	return items, nil
}

func (r *MetaDecksRepository) ListComparableBySnapshotID(ctx context.Context, snapshotID string) ([]ComparableMetaDeck, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT d.id, d.name, d.class, d.format, d.deck_code, d.archetype, md.winrate, md.playrate, md.sample_size, md.tier
FROM meta_decks md
JOIN decks d ON d.id = md.deck_id
WHERE md.snapshot_id = ? AND d.deck_code IS NOT NULL AND d.deck_code != ''
ORDER BY md.playrate DESC, md.winrate DESC, d.id ASC
`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("list comparable meta decks for snapshot %q: %w", snapshotID, err)
	}
	defer rows.Close()

	var items []ComparableMetaDeck
	for rows.Next() {
		var item ComparableMetaDeck
		if err := rows.Scan(&item.DeckID, &item.Name, &item.Class, &item.Format, &item.DeckCode, &item.Archetype, &item.Winrate, &item.Playrate, &item.SampleSize, &item.Tier); err != nil {
			return nil, fmt.Errorf("scan comparable meta deck: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comparable meta decks: %w", err)
	}
	return items, nil
}

func (r *DeckCardsRepository) ReplaceDeckCards(ctx context.Context, deckID string, items []DeckCardRecord) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace deck cards tx: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM deck_cards WHERE deck_id = ?`, deckID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete deck cards for deck %q: %w", deckID, err)
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO deck_cards (deck_id, card_id, card_count)
VALUES (?, ?, ?)
`, item.DeckID, item.CardID, item.CardCount); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert deck card for deck %q card %q: %w", item.DeckID, item.CardID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace deck cards tx: %w", err)
	}
	return nil
}

func (r *DeckCardsRepository) ListByDeckID(ctx context.Context, deckID string) ([]DeckCardRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT deck_id, card_id, card_count
FROM deck_cards
WHERE deck_id = ?
ORDER BY card_id ASC
`, deckID)
	if err != nil {
		return nil, fmt.Errorf("list deck cards for deck %q: %w", deckID, err)
	}
	defer rows.Close()

	var items []DeckCardRecord
	for rows.Next() {
		var item DeckCardRecord
		if err := rows.Scan(&item.DeckID, &item.CardID, &item.CardCount); err != nil {
			return nil, fmt.Errorf("scan deck card: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deck cards: %w", err)
	}
	return items, nil
}

func (r *AnalysisReportsRepository) Create(ctx context.Context, report AnalysisReport) error {
	const stmt = `
INSERT INTO analysis_reports (
    id, deck_id, based_on_snapshot_id, report_type, input_hash, report_json, report_text
)
VALUES (?, ?, ?, ?, ?, ?, ?)
`
	if _, err := r.db.ExecContext(ctx, stmt, report.ID, report.DeckID, report.BasedOnSnapshotID, report.ReportType, report.InputHash, report.ReportJSON, report.ReportText); err != nil {
		return fmt.Errorf("create analysis report %q: %w", report.ID, err)
	}
	return nil
}

func (r *AnalysisReportsRepository) GetByID(ctx context.Context, id string) (AnalysisReport, error) {
	const stmt = `
SELECT id, deck_id, based_on_snapshot_id, report_type, input_hash, report_json, report_text, created_at
FROM analysis_reports
WHERE id = ?
`
	var report AnalysisReport
	if err := r.db.QueryRowContext(ctx, stmt, id).Scan(
		&report.ID,
		&report.DeckID,
		&report.BasedOnSnapshotID,
		&report.ReportType,
		&report.InputHash,
		&report.ReportJSON,
		&report.ReportText,
		&report.CreatedAt,
	); err != nil {
		return AnalysisReport{}, fmt.Errorf("get analysis report %q: %w", id, err)
	}
	return report, nil
}

func (r *AnalysisReportsRepository) ListRecent(ctx context.Context, limit int) ([]AnalysisReport, error) {
	if limit <= 0 {
		limit = 20
	}

	const stmt = `
SELECT id, deck_id, based_on_snapshot_id, report_type, input_hash, report_json, report_text, created_at
FROM analysis_reports
ORDER BY created_at DESC, id DESC
LIMIT ?
`
	rows, err := r.db.QueryContext(ctx, stmt, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent analysis reports: %w", err)
	}
	defer rows.Close()

	out := make([]AnalysisReport, 0)
	for rows.Next() {
		var report AnalysisReport
		if err := rows.Scan(
			&report.ID,
			&report.DeckID,
			&report.BasedOnSnapshotID,
			&report.ReportType,
			&report.InputHash,
			&report.ReportJSON,
			&report.ReportText,
			&report.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan analysis report: %w", err)
		}
		out = append(out, report)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate analysis reports: %w", err)
	}

	return out, nil
}
