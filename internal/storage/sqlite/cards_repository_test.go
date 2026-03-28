package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"hearthstone-analyzer/internal/cards"
)

func TestCardsRepositoryListFiltersByClass(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCardsRepoTestDB(t)
	repo := NewCardsRepository(db)

	if err := repo.UpsertMany(ctx, []cards.Card{
		{
			ID:            "CORE_EX1_391",
			DBFID:         391,
			Class:         "WARRIOR",
			CardType:      "SPELL",
			Set:           "CORE",
			Rarity:        "FREE",
			Cost:          2,
			Text:          "Deal 2 damage.",
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales: []cards.LocaleText{
				{Locale: "enUS", Name: "Slam", Text: "Deal 2 damage."},
			},
		},
		{
			ID:            "CORE_CS2_029",
			DBFID:         29,
			Class:         "MAGE",
			CardType:      "SPELL",
			Set:           "CORE",
			Rarity:        "FREE",
			Cost:          4,
			Text:          "Deal 6 damage.",
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales: []cards.LocaleText{
				{Locale: "enUS", Name: "Fireball", Text: "Deal 6 damage."},
			},
		},
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	got, err := repo.List(ctx, CardsListFilter{Class: "WARRIOR"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 card, got %d", len(got))
	}

	if got[0].ID != "CORE_EX1_391" || got[0].Locales[0].Name != "Slam" {
		t.Fatalf("unexpected card returned: %+v", got[0])
	}

	if len(got[0].FunctionalTags) != 0 {
		t.Fatalf("expected no functional tags for plain fixture card, got %+v", got[0].FunctionalTags)
	}
}

func TestCardsRepositoryListFiltersBySetAndCost(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCardsRepoTestDB(t)
	repo := NewCardsRepository(db)

	if err := repo.UpsertMany(ctx, []cards.Card{
		{
			ID:            "CORE_EX1_391",
			DBFID:         391,
			Class:         "WARRIOR",
			CardType:      "SPELL",
			Set:           "CORE",
			Rarity:        "FREE",
			Cost:          2,
			Text:          "Deal 2 damage.",
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales: []cards.LocaleText{
				{Locale: "enUS", Name: "Slam", Text: "Deal 2 damage."},
			},
		},
		{
			ID:            "CORE_CS2_029",
			DBFID:         29,
			Class:         "MAGE",
			CardType:      "SPELL",
			Set:           "CORE",
			Rarity:        "FREE",
			Cost:          4,
			Text:          "Deal 6 damage.",
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales: []cards.LocaleText{
				{Locale: "enUS", Name: "Fireball", Text: "Deal 6 damage."},
			},
		},
		{
			ID:            "WORKSHOP_001",
			DBFID:         1001,
			Class:         "MAGE",
			CardType:      "SPELL",
			Set:           "WORKSHOP",
			Rarity:        "COMMON",
			Cost:          2,
			Text:          "New spell.",
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales: []cards.LocaleText{
				{Locale: "enUS", Name: "Workshop Bolt", Text: "New spell."},
			},
		},
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	got, err := repo.List(ctx, CardsListFilter{Set: "CORE", Cost: intPtr(2)})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(got) != 1 || got[0].ID != "CORE_EX1_391" {
		t.Fatalf("unexpected filtered cards: %+v", got)
	}
}

func TestCardsRepositoryGetByID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCardsRepoTestDB(t)
	repo := NewCardsRepository(db)

	if err := repo.UpsertMany(ctx, []cards.Card{
		{
			ID:            "CORE_CS2_029",
			DBFID:         29,
			Class:         "MAGE",
			CardType:      "SPELL",
			Set:           "CORE",
			Rarity:        "FREE",
			Cost:          4,
			Text:          "Deal 6 damage.",
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales: []cards.LocaleText{
				{Locale: "enUS", Name: "Fireball", Text: "Deal 6 damage."},
			},
		},
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	card, err := repo.GetByID(ctx, "CORE_CS2_029")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if card.ID != "CORE_CS2_029" || len(card.Locales) != 1 || card.Locales[0].Name != "Fireball" {
		t.Fatalf("unexpected card returned: %+v", card)
	}
}

func TestCardsRepositoryGetByLocalizedNamesNormalizesVariants(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCardsRepoTestDB(t)
	repo := NewCardsRepository(db)

	if err := repo.UpsertMany(ctx, []cards.Card{
		{
			ID:            "WORKSHOP_123",
			DBFID:         123,
			Class:         "PRIEST",
			CardType:      "SPELL",
			Set:           "WORKSHOP",
			Rarity:        "RARE",
			Cost:          3,
			Text:          "Draw cards.",
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales: []cards.LocaleText{
				{Locale: "enUS", Name: "Pop-Up Book", Text: "Draw cards."},
			},
		},
		{
			ID:            "CORE_456",
			DBFID:         456,
			Class:         "ROGUE",
			CardType:      "MINION",
			Set:           "CORE",
			Rarity:        "LEGENDARY",
			Cost:          4,
			Text:          "Battlecry.",
			Collectible:   true,
			StandardLegal: true,
			WildLegal:     true,
			Locales: []cards.LocaleText{
				{Locale: "enUS", Name: "Maiev Shadowsong", Text: "Battlecry."},
			},
		},
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	got, err := repo.GetByLocalizedNames(ctx, []string{
		"Pop Up Book",
		"Maiev Shadowsong (Core)",
	})
	if err != nil {
		t.Fatalf("GetByLocalizedNames() error = %v", err)
	}

	if got["Pop Up Book"].ID != "WORKSHOP_123" {
		t.Fatalf("expected punctuation-normalized match, got %+v", got["Pop Up Book"])
	}

	if got["Maiev Shadowsong (Core)"].ID != "CORE_456" {
		t.Fatalf("expected bracket-normalized match, got %+v", got["Maiev Shadowsong (Core)"])
	}
}

func TestCardsRepositoryPersistsFunctionalTags(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCardsRepoTestDB(t)
	repo := NewCardsRepository(db)

	if err := repo.UpsertMany(ctx, []cards.Card{
		{
			ID:       "CORE_EX1_391",
			DBFID:    391,
			Class:    "WARRIOR",
			CardType: "SPELL",
			Set:      "CORE",
			Rarity:   "FREE",
			Cost:     2,
			Text:     "Deal 2 damage to a minion. If it survives, draw a card.",
			Metadata: cards.CardMetadata{
				Mechanics:      []string{"DISCOVER"},
				ReferencedTags: []string{"BATTLECRY"},
				SpellSchool:    "ARCANE",
			},
			FunctionalTags: []string{"draw", "single_removal", "burn"},
			Collectible:    true,
			StandardLegal:  true,
			WildLegal:      true,
			Locales: []cards.LocaleText{
				{Locale: "enUS", Name: "Slam", Text: "Deal 2 damage to a minion. If it survives, draw a card."},
			},
		},
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	card, err := repo.GetByID(ctx, "CORE_EX1_391")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if len(card.FunctionalTags) != 3 {
		t.Fatalf("expected functional tags to round-trip, got %+v", card.FunctionalTags)
	}

	if card.Metadata.SpellSchool != "ARCANE" || len(card.Metadata.Mechanics) != 1 {
		t.Fatalf("expected metadata to round-trip, got %+v", card.Metadata)
	}
}

func openCardsRepoTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "cards_repo.db")
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

func intPtr(v int) *int {
	return &v
}
