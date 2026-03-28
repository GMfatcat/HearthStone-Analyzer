package decks_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"

	"hearthstone-analyzer/internal/cards"
	"hearthstone-analyzer/internal/decks"
	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

func TestParserParsesDeckCodeIntoStructuredDeck(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openDecksTestDB(t)
	cardsRepo := sqliteStore.NewCardsRepository(db)

	if err := cardsRepo.UpsertMany(ctx, []cards.Card{
		testCard("CORE_001", 7, "WARRIOR", 1, "Guard"),
		testCard("CORE_002", 8, "WARRIOR", 2, "Strike"),
		testCard("CORE_003", 9, "NEUTRAL", 3, "Ogre"),
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	lookup := sqliteStore.NewCardLookupRepository(db)
	parser := decks.NewParser(lookup)
	code := mustEncodeDeckstring(t, 2, 7, map[int]int{
		7: 1,
		8: 2,
		9: 2,
	})

	got, err := parser.Parse(ctx, code)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if got.Class != "WARRIOR" {
		t.Fatalf("expected class %q, got %q", "WARRIOR", got.Class)
	}

	if got.TotalCount != 5 {
		t.Fatalf("expected total count 5, got %d", got.TotalCount)
	}

	if got.Cards[0].CardID != "CORE_001" || got.Cards[1].Count != 2 {
		t.Fatalf("unexpected parsed cards: %+v", got.Cards)
	}

	if got.Legality.Valid {
		t.Fatal("expected 5-card test deck to be marked illegal")
	}

	if got.DeckHash == "" {
		t.Fatal("expected deck hash to be populated")
	}
}

func TestParserReturnsErrorWhenCardDBFIDMissing(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openDecksTestDB(t)
	cardsRepo := sqliteStore.NewCardsRepository(db)

	if err := cardsRepo.UpsertMany(ctx, []cards.Card{
		testCard("HERO_001", 7, "WARRIOR", 0, "Garrosh"),
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	parser := decks.NewParser(sqliteStore.NewCardLookupRepository(db))

	code := mustEncodeDeckstring(t, 2, 7, map[int]int{
		999999: 2,
	})

	_, err := parser.Parse(ctx, code)
	if err == nil {
		t.Fatal("expected error when card DBFID cannot be resolved")
	}

	parseErr, ok := decks.AsParseError(err)
	if !ok {
		t.Fatalf("expected typed parse error, got %T", err)
	}

	if parseErr.Code != decks.ErrCodeCardNotFound {
		t.Fatalf("expected error code %q, got %q", decks.ErrCodeCardNotFound, parseErr.Code)
	}
}

func TestParserLegalityFlagsClassMismatchAndStandardIssues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openDecksTestDB(t)
	cardsRepo := sqliteStore.NewCardsRepository(db)

	rogueOnly := testCard("ROGUE_001", 101, "ROGUE", 2, "Backstabber")
	rogueOnly.StandardLegal = false

	if err := cardsRepo.UpsertMany(ctx, []cards.Card{
		testCard("HERO_001", 7, "WARRIOR", 0, "Garrosh"),
		rogueOnly,
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	parser := decks.NewParser(sqliteStore.NewCardLookupRepository(db))
	code := mustEncodeDeckstring(t, 2, 7, map[int]int{
		101: 30,
	})

	got, err := parser.Parse(ctx, code)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if got.Legality.Valid {
		t.Fatal("expected legality to be invalid")
	}

	assertIssueContains(t, got.Legality.Issues, "class mismatch")
	assertIssueContains(t, got.Legality.Issues, "not legal in standard")
	assertIssueContains(t, got.Legality.Issues, "more than 2 copies")
}

func TestParserLegalityFlagsLegendaryDuplicates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openDecksTestDB(t)
	cardsRepo := sqliteStore.NewCardsRepository(db)

	legendary := testCard("LEGEND_001", 201, "WARRIOR", 6, "Grommash")
	legendary.Rarity = "LEGENDARY"

	if err := cardsRepo.UpsertMany(ctx, []cards.Card{
		testCard("HERO_001", 7, "WARRIOR", 0, "Garrosh"),
		legendary,
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	parser := decks.NewParser(sqliteStore.NewCardLookupRepository(db))
	code := mustEncodeDeckstring(t, 2, 7, map[int]int{
		201: 2,
	})

	got, err := parser.Parse(ctx, code)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	assertIssueContains(t, got.Legality.Issues, "legendary")
}

func TestParserClassifiesInvalidDeckCode(t *testing.T) {
	t.Parallel()

	parser := decks.NewParser(nil)

	_, err := parser.Parse(context.Background(), "%%%")
	if err == nil {
		t.Fatal("expected error for invalid deck code")
	}

	parseErr, ok := decks.AsParseError(err)
	if !ok {
		t.Fatalf("expected typed parse error, got %T", err)
	}

	if parseErr.Code != decks.ErrCodeInvalidDeckCode {
		t.Fatalf("expected error code %q, got %q", decks.ErrCodeInvalidDeckCode, parseErr.Code)
	}
}

func openDecksTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "decks.db")
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

func testCard(id string, dbfID int, class string, cost int, name string) cards.Card {
	return cards.Card{
		ID:            id,
		DBFID:         dbfID,
		Class:         class,
		CardType:      "MINION",
		Set:           "CORE",
		Rarity:        "FREE",
		Cost:          cost,
		Text:          name + " text",
		Collectible:   true,
		StandardLegal: true,
		WildLegal:     true,
		Locales: []cards.LocaleText{
			{Locale: "enUS", Name: name, Text: name + " text"},
		},
	}
}

func mustEncodeDeckstring(t *testing.T, format int, heroDBFID int, cardCounts map[int]int) string {
	t.Helper()

	buf := &bytes.Buffer{}
	buf.WriteByte(0)
	writeVarint(buf, int64(format))
	writeVarint(buf, 1)
	writeVarint(buf, int64(heroDBFID))

	singles := make([]int, 0)
	doubles := make([]int, 0)
	multiples := make([]struct {
		dbfID int
		count int
	}, 0)

	for dbfID, count := range cardCounts {
		switch count {
		case 1:
			singles = append(singles, dbfID)
		case 2:
			doubles = append(doubles, dbfID)
		default:
			multiples = append(multiples, struct {
				dbfID int
				count int
			}{dbfID: dbfID, count: count})
		}
	}

	sortInts(singles)
	sortInts(doubles)
	sortMulti(multiples)

	writeVarint(buf, int64(len(singles)))
	for _, id := range singles {
		writeVarint(buf, int64(id))
	}

	writeVarint(buf, int64(len(doubles)))
	for _, id := range doubles {
		writeVarint(buf, int64(id))
	}

	writeVarint(buf, int64(len(multiples)))
	for _, item := range multiples {
		writeVarint(buf, int64(item.dbfID))
		writeVarint(buf, int64(item.count))
	}

	return base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func writeVarint(buf *bytes.Buffer, value int64) {
	tmp := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(tmp, uint64(value))
	buf.Write(tmp[:n])
}

func sortInts(values []int) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

func sortMulti(values []struct {
	dbfID int
	count int
}) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j].dbfID < values[i].dbfID {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

func assertIssueContains(t *testing.T, issues []string, needle string) {
	t.Helper()

	for _, issue := range issues {
		if strings.Contains(strings.ToLower(issue), strings.ToLower(needle)) {
			return
		}
	}

	t.Fatalf("expected issues %+v to contain %q", issues, needle)
}
