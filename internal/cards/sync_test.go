package cards_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"hearthstone-analyzer/internal/cards"
	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

func TestHearthstoneJSONSourceFetchCards(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cards.collectible.json" {
			http.NotFound(w, r)
			return
		}

		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":             "CORE_EX1_391",
				"dbfId":          391,
				"name":           "Slam",
				"cardClass":      "WARRIOR",
				"type":           "SPELL",
				"set":            "CORE",
				"rarity":         "FREE",
				"cost":           2,
				"text":           "Deal 2 damage to a minion. If it survives, draw a card.",
				"mechanics":      []string{"DISCOVER"},
				"referencedTags": []string{"BATTLECRY"},
				"spellSchool":    "ARCANE",
				"collectible":    true,
			},
		})
	}))
	defer server.Close()

	source := cards.NewHearthstoneJSONSource(server.URL+"/cards.collectible.json", "enUS", server.Client())
	result, err := source.FetchCards(context.Background())
	if err != nil {
		t.Fatalf("FetchCards() error = %v", err)
	}

	if len(result.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(result.Cards))
	}

	card := result.Cards[0]
	if card.ID != "CORE_EX1_391" {
		t.Fatalf("expected card ID %q, got %q", "CORE_EX1_391", card.ID)
	}

	if len(card.FunctionalTags) == 0 {
		t.Fatalf("expected source to derive functional tags, got %+v", card)
	}

	if len(card.Locales) != 1 || card.Locales[0].Locale != "enUS" {
		t.Fatalf("expected one enUS locale, got %+v", card.Locales)
	}

	if card.Metadata.SpellSchool != "ARCANE" || len(card.Metadata.Mechanics) == 0 {
		t.Fatalf("expected metadata to be populated, got %+v", card.Metadata)
	}

	if result.Source != "hearthstonejson" || result.RawPayload == "" {
		t.Fatalf("expected source metadata and raw payload, got %+v", result)
	}
}

func TestSyncServicePersistsCardsAndLocales(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCardsTestDB(t)
	repo := sqliteStore.NewCardsRepository(db)
	service := cards.NewSyncService(repo, stubSource{
		result: cards.FetchResult{
			Source:     "hearthstonejson",
			FetchedAt:  time.Now().UTC(),
			RawPayload: `[{"id":"CORE_EX1_391"}]`,
			Cards: []cards.Card{
				{
					ID:       "CORE_EX1_391",
					DBFID:    391,
					Class:    "WARRIOR",
					CardType: "SPELL",
					Set:      "CORE",
					Rarity:   "FREE",
					Cost:     2,
					Text:     "Deal 2 damage.",
					Metadata: cards.CardMetadata{
						Mechanics:   []string{"DISCOVER"},
						SpellSchool: "ARCANE",
					},
					Collectible:   true,
					StandardLegal: true,
					WildLegal:     true,
					Locales: []cards.LocaleText{
						{Locale: "enUS", Name: "Slam", Text: "Deal 2 damage."},
					},
				},
			},
		},
	})

	summary, err := service.Sync(ctx)
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if summary.CardsUpserted != 1 {
		t.Fatalf("expected 1 upserted card, got %d", summary.CardsUpserted)
	}

	if count := countRows(t, db, "cards"); count != 1 {
		t.Fatalf("expected 1 card row, got %d", count)
	}

	if count := countRows(t, db, "card_locales"); count != 1 {
		t.Fatalf("expected 1 card locale row, got %d", count)
	}

	if count := countRows(t, db, "card_sync_runs"); count != 1 {
		t.Fatalf("expected 1 card sync run row, got %d", count)
	}

	var source, rawPayload string
	if err := db.QueryRowContext(ctx, "SELECT source, raw_payload FROM card_sync_runs LIMIT 1").Scan(&source, &rawPayload); err != nil {
		t.Fatalf("QueryRowContext(card_sync_runs) error = %v", err)
	}

	if source != "hearthstonejson" || rawPayload == "" {
		t.Fatalf("expected sync metadata to be stored, got source=%q raw=%q", source, rawPayload)
	}

	var functionalTags string
	if err := db.QueryRowContext(ctx, "SELECT functional_tags FROM cards WHERE id = ?", "CORE_EX1_391").Scan(&functionalTags); err != nil {
		t.Fatalf("QueryRowContext(functional_tags) error = %v", err)
	}
	if functionalTags == "" || functionalTags == "[]" {
		t.Fatalf("expected functional tags to be persisted, got %q", functionalTags)
	}

	var metadata string
	if err := db.QueryRowContext(ctx, "SELECT metadata FROM cards WHERE id = ?", "CORE_EX1_391").Scan(&metadata); err != nil {
		t.Fatalf("QueryRowContext(metadata) error = %v", err)
	}
	if metadata == "" || metadata == "{}" {
		t.Fatalf("expected card metadata to be persisted, got %q", metadata)
	}
}

func openCardsTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "cards.db")
	db, err := sqliteStore.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := sqliteStore.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return db
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
	if err != nil {
		t.Fatalf("countRows(%s) error = %v", table, err)
	}

	return count
}

type stubSource struct {
	result cards.FetchResult
	err    error
}

func (s stubSource) FetchCards(ctx context.Context) (cards.FetchResult, error) {
	return s.result, s.err
}
