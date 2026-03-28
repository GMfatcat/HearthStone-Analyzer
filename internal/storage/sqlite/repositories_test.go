package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"hearthstone-analyzer/internal/cards"
)

func TestSettingsRepositoryUpsertAndList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repo := NewSettingsRepository(db)

	if err := repo.Upsert(ctx, Setting{
		Key:         "llm.api_key",
		Value:       "secret-token",
		IsEncrypted: true,
	}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if err := repo.Upsert(ctx, Setting{
		Key:         "llm.base_url",
		Value:       "https://example.test/v1",
		IsEncrypted: false,
	}); err != nil {
		t.Fatalf("second Upsert() error = %v", err)
	}

	setting, err := repo.GetByKey(ctx, "llm.api_key")
	if err != nil {
		t.Fatalf("GetByKey() error = %v", err)
	}

	if setting.Value != "secret-token" || !setting.IsEncrypted {
		t.Fatalf("unexpected setting returned: %+v", setting)
	}

	settings, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(settings) != 2 {
		t.Fatalf("expected 2 settings, got %d", len(settings))
	}

	if settings[0].Key != "llm.api_key" || settings[1].Key != "llm.base_url" {
		t.Fatalf("expected settings sorted by key, got %+v", settings)
	}
}

func TestScheduledJobsRepositoryUpsertAndList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repo := NewScheduledJobsRepository(db)

	now := time.Now().UTC().Truncate(time.Second)
	next := now.Add(30 * time.Minute)
	if err := repo.Upsert(ctx, ScheduledJob{
		Key:       "sync_cards",
		CronExpr:  "0 0 * * *",
		Enabled:   true,
		LastRunAt: &now,
		NextRunAt: &next,
	}); err != nil {
		t.Fatalf("Upsert(sync_cards) error = %v", err)
	}

	if err := repo.Upsert(ctx, ScheduledJob{
		Key:      "sync_meta",
		CronExpr: "0 */12 * * *",
		Enabled:  false,
	}); err != nil {
		t.Fatalf("Upsert(sync_meta) error = %v", err)
	}

	jobs, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	if jobs[0].Key != "sync_cards" || jobs[1].Key != "sync_meta" {
		t.Fatalf("expected jobs sorted by key, got %+v", jobs)
	}

	if jobs[0].LastRunAt == nil || !jobs[0].LastRunAt.Equal(now) {
		t.Fatalf("expected last_run_at to round-trip, got %+v", jobs[0].LastRunAt)
	}
}

func TestJobExecutionLogsRepositoryCreateAndListByJob(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	jobsRepo := NewScheduledJobsRepository(db)
	logsRepo := NewJobExecutionLogsRepository(db)

	if err := jobsRepo.Upsert(ctx, ScheduledJob{
		Key:      "sync_cards",
		CronExpr: "0 0 * * *",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("Upsert(job) error = %v", err)
	}

	earlier := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	later := earlier.Add(30 * time.Minute)
	finished := later.Add(2 * time.Minute)
	affected := int64(42)

	if err := logsRepo.Create(ctx, JobExecutionLog{
		JobKey:          "sync_cards",
		Status:          "success",
		StartedAt:       earlier,
		FinishedAt:      &finished,
		RecordsAffected: &affected,
	}); err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}

	if err := logsRepo.Create(ctx, JobExecutionLog{
		JobKey:       "sync_cards",
		Status:       "failed",
		StartedAt:    later,
		ErrorMessage: strPtr("source timeout"),
		FinishedAt:   nil,
	}); err != nil {
		t.Fatalf("Create(second) error = %v", err)
	}

	logs, err := logsRepo.ListByJobKey(ctx, "sync_cards", 10)
	if err != nil {
		t.Fatalf("ListByJobKey() error = %v", err)
	}

	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}

	if logs[0].Status != "failed" || logs[1].Status != "success" {
		t.Fatalf("expected newest-first ordering, got %+v", logs)
	}

	if logs[1].RecordsAffected == nil || *logs[1].RecordsAffected != affected {
		t.Fatalf("expected records affected to round-trip, got %+v", logs[1].RecordsAffected)
	}
}

func TestMetaSnapshotsRepositoryCreateAndGetLatest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repo := NewMetaSnapshotsRepository(db)

	earlier := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)
	later := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)

	if err := repo.Create(ctx, MetaSnapshot{
		ID:           "snapshot-1",
		Source:       "stub",
		PatchVersion: "32.0.0",
		Format:       "standard",
		FetchedAt:    earlier,
		RawPayload:   `{"items":1}`,
	}); err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}

	if err := repo.Create(ctx, MetaSnapshot{
		ID:           "snapshot-2",
		Source:       "stub",
		PatchVersion: "32.0.1",
		Format:       "standard",
		FetchedAt:    later,
		RawPayload:   `{"items":2}`,
	}); err != nil {
		t.Fatalf("Create(second) error = %v", err)
	}

	latest, err := repo.GetLatestByFormat(ctx, "standard")
	if err != nil {
		t.Fatalf("GetLatestByFormat() error = %v", err)
	}

	if latest.ID != "snapshot-2" || latest.PatchVersion != "32.0.1" {
		t.Fatalf("unexpected latest snapshot: %+v", latest)
	}
}

func TestMetaSnapshotsRepositoryListByFormatAndGetByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	repo := NewMetaSnapshotsRepository(db)

	firstTime := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)
	secondTime := time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)
	thirdTime := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)

	for _, snapshot := range []MetaSnapshot{
		{
			ID:           "snapshot-1",
			Source:       "stub",
			PatchVersion: "32.0.0",
			Format:       "standard",
			FetchedAt:    firstTime,
			RawPayload:   `{"items":1}`,
		},
		{
			ID:           "snapshot-2",
			Source:       "stub",
			PatchVersion: "32.0.1",
			Format:       "wild",
			FetchedAt:    secondTime,
			RawPayload:   `{"items":2}`,
		},
		{
			ID:           "snapshot-3",
			Source:       "remote",
			PatchVersion: "32.0.2",
			Format:       "standard",
			FetchedAt:    thirdTime,
			RawPayload:   `{"items":3}`,
		},
	} {
		if err := repo.Create(ctx, snapshot); err != nil {
			t.Fatalf("Create(%q) error = %v", snapshot.ID, err)
		}
	}

	list, err := repo.ListByFormat(ctx, "standard", 10)
	if err != nil {
		t.Fatalf("ListByFormat() error = %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 standard snapshots, got %d", len(list))
	}

	if list[0].ID != "snapshot-3" || list[1].ID != "snapshot-1" {
		t.Fatalf("expected newest-first standard snapshots, got %+v", list)
	}

	found, err := repo.GetByID(ctx, "snapshot-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if found.ID != "snapshot-1" || found.RawPayload != `{"items":1}` {
		t.Fatalf("unexpected snapshot detail: %+v", found)
	}
}

func TestDecksRepositoryUpsertMetaDeckAndMetaDecksReplaceSnapshotDecks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	decksRepo := NewDecksRepository(db)
	metaDecksRepo := NewMetaDecksRepository(db)
	snapshotsRepo := NewMetaSnapshotsRepository(db)

	if err := snapshotsRepo.Create(ctx, MetaSnapshot{
		ID:           "snapshot-1",
		Source:       "remote",
		PatchVersion: "32.4.0",
		Format:       "standard",
		FetchedAt:    time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC),
		RawPayload:   `{"decks":[]}`,
	}); err != nil {
		t.Fatalf("Create(snapshot) error = %v", err)
	}

	deckID, err := decksRepo.UpsertMetaDeck(ctx, Deck{
		ID:          "deck-1",
		Source:      "remote",
		ExternalRef: strPtr("ext-1"),
		Name:        strPtr("Cycle Rogue"),
		Class:       "ROGUE",
		Format:      "standard",
		Archetype:   strPtr("Combo"),
	})
	if err != nil {
		t.Fatalf("UpsertMetaDeck() error = %v", err)
	}

	if deckID != "deck-1" {
		t.Fatalf("expected deck id to round-trip, got %q", deckID)
	}

	if err := metaDecksRepo.ReplaceSnapshotDecks(ctx, "snapshot-1", []MetaDeck{
		{
			SnapshotID: "snapshot-1",
			DeckID:     deckID,
			Winrate:    floatPtr(54.2),
			Playrate:   floatPtr(12.7),
			SampleSize: metaIntPtr(2450),
			Tier:       strPtr("T1"),
		},
	}); err != nil {
		t.Fatalf("ReplaceSnapshotDecks(first) error = %v", err)
	}

	if err := metaDecksRepo.ReplaceSnapshotDecks(ctx, "snapshot-1", []MetaDeck{
		{
			SnapshotID: "snapshot-1",
			DeckID:     deckID,
			Winrate:    floatPtr(55.1),
			Playrate:   floatPtr(11.9),
			SampleSize: metaIntPtr(2600),
			Tier:       strPtr("T1"),
		},
	}); err != nil {
		t.Fatalf("ReplaceSnapshotDecks(second) error = %v", err)
	}

	items, err := metaDecksRepo.ListBySnapshotID(ctx, "snapshot-1")
	if err != nil {
		t.Fatalf("ListBySnapshotID() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 meta deck row, got %d", len(items))
	}

	if items[0].Winrate == nil || *items[0].Winrate != 55.1 {
		t.Fatalf("expected replaced winrate, got %+v", items[0])
	}
}

func TestDeckCardsRepositoryReplaceAndListByDeckID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	decksRepo := NewDecksRepository(db)
	deckCardsRepo := NewDeckCardsRepository(db)
	cardsRepo := NewCardsRepository(db)

	if err := cardsRepo.UpsertMany(ctx, []cards.Card{
		{
			ID:            "CARD_001",
			DBFID:         1,
			Class:         "MAGE",
			CardType:      "SPELL",
			Set:           "CORE",
			Rarity:        "FREE",
			Cost:          1,
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales:       []cards.LocaleText{{Locale: "enUS", Name: "Arcane Shot"}},
		},
		{
			ID:            "CARD_002",
			DBFID:         2,
			Class:         "MAGE",
			CardType:      "MINION",
			Set:           "CORE",
			Rarity:        "FREE",
			Cost:          2,
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales:       []cards.LocaleText{{Locale: "enUS", Name: "Mana Wyrm"}},
		},
	}); err != nil {
		t.Fatalf("UpsertMany(cards) error = %v", err)
	}

	if _, err := decksRepo.UpsertMetaDeck(ctx, Deck{
		ID:     "deck-1",
		Source: "remote",
		Name:   strPtr("Tempo Mage"),
		Class:  "MAGE",
		Format: "standard",
	}); err != nil {
		t.Fatalf("UpsertMetaDeck() error = %v", err)
	}

	if err := deckCardsRepo.ReplaceDeckCards(ctx, "deck-1", []DeckCardRecord{
		{DeckID: "deck-1", CardID: "CARD_001", CardCount: 2},
		{DeckID: "deck-1", CardID: "CARD_002", CardCount: 1},
	}); err != nil {
		t.Fatalf("ReplaceDeckCards(first) error = %v", err)
	}

	if err := deckCardsRepo.ReplaceDeckCards(ctx, "deck-1", []DeckCardRecord{
		{DeckID: "deck-1", CardID: "CARD_001", CardCount: 2},
	}); err != nil {
		t.Fatalf("ReplaceDeckCards(second) error = %v", err)
	}

	items, err := deckCardsRepo.ListByDeckID(ctx, "deck-1")
	if err != nil {
		t.Fatalf("ListByDeckID() error = %v", err)
	}

	if len(items) != 1 || items[0].CardID != "CARD_001" || items[0].CardCount != 2 {
		t.Fatalf("unexpected persisted deck cards: %+v", items)
	}
}

func TestAnalysisReportsRepositoryCreateAndGetByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	decksRepo := NewDecksRepository(db)
	reportsRepo := NewAnalysisReportsRepository(db)
	snapshotsRepo := NewMetaSnapshotsRepository(db)

	if err := snapshotsRepo.Create(ctx, MetaSnapshot{
		ID:           "snapshot-1",
		Source:       "remote",
		PatchVersion: "32.7.0",
		Format:       "standard",
		FetchedAt:    time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC),
		RawPayload:   `{"decks":[]}`,
	}); err != nil {
		t.Fatalf("Create(snapshot) error = %v", err)
	}

	if _, err := decksRepo.UpsertMetaDeck(ctx, Deck{
		ID:       "deck-report-1",
		Source:   "user_report",
		Class:    "MAGE",
		Format:   "standard",
		DeckCode: strPtr("AAEAAA=="),
		DeckHash: strPtr("hash-123"),
	}); err != nil {
		t.Fatalf("UpsertMetaDeck() error = %v", err)
	}

	record := AnalysisReport{
		ID:               "report-1",
		DeckID:           "deck-report-1",
		BasedOnSnapshotID: strPtr("snapshot-1"),
		ReportType:       "ai_deck_report",
		InputHash:        "hash-123",
		ReportJSON:       `{"report":"body"}`,
		ReportText:       strPtr("body"),
	}
	if err := reportsRepo.Create(ctx, record); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := reportsRepo.GetByID(ctx, "report-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.DeckID != "deck-report-1" || got.ReportType != "ai_deck_report" || got.ReportText == nil || *got.ReportText != "body" {
		t.Fatalf("unexpected analysis report row: %+v", got)
	}
}

func TestAnalysisReportsRepositoryListRecent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	decksRepo := NewDecksRepository(db)
	reportsRepo := NewAnalysisReportsRepository(db)

	if _, err := decksRepo.UpsertMetaDeck(ctx, Deck{
		ID:     "deck-report-1",
		Source: "user_report",
		Class:  "MAGE",
		Format: "standard",
	}); err != nil {
		t.Fatalf("UpsertMetaDeck() error = %v", err)
	}

	for _, record := range []AnalysisReport{
		{
			ID:         "report-1",
			DeckID:     "deck-report-1",
			ReportType: "ai_deck_report",
			InputHash:  "hash-1",
			ReportJSON: `{"report":"one"}`,
		},
		{
			ID:         "report-2",
			DeckID:     "deck-report-1",
			ReportType: "ai_deck_report",
			InputHash:  "hash-2",
			ReportJSON: `{"report":"two"}`,
		},
	} {
		if err := reportsRepo.Create(ctx, record); err != nil {
			t.Fatalf("Create(%q) error = %v", record.ID, err)
		}
	}

	items, err := reportsRepo.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}

	if len(items) != 2 || items[0].ID != "report-2" || items[1].ID != "report-1" {
		t.Fatalf("expected newest-first reports, got %+v", items)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
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

	return db
}

func strPtr(v string) *string {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}

func metaIntPtr(v int) *int {
	return &v
}
