package compare

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hearthstone-analyzer/internal/cards"
	"hearthstone-analyzer/internal/decks"
	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

func TestServiceCompareDeckRanksClosestMetaDecks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCompareTestDB(t)
	repos := sqliteStore.NewRepositories(db)

	if err := repos.Cards.UpsertMany(ctx, []cards.Card{
		testCompareCard("HERO_001", 1, "WARRIOR", 0, "Garrosh"),
		testCompareCard("CARD_002", 2, "WARRIOR", 1, "One"),
		testCompareCard("CARD_003", 3, "WARRIOR", 2, "Two"),
		testCompareCard("CARD_004", 4, "WARRIOR", 3, "Three"),
		testCompareCard("CARD_005", 5, "WARRIOR", 4, "Four"),
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	if err := repos.MetaSnapshots.Create(ctx, sqliteStore.MetaSnapshot{
		ID:           "snapshot-1",
		Source:       "remote",
		PatchVersion: "32.4.0",
		Format:       "standard",
		FetchedAt:    time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
		RawPayload:   `{"decks":[]}`,
	}); err != nil {
		t.Fatalf("Create(snapshot) error = %v", err)
	}

	closeDeckCode := mustEncodeCompareDeckstring(t, 2, 1, map[int]int{2: 2, 3: 2, 4: 2})
	farDeckCode := mustEncodeCompareDeckstring(t, 2, 1, map[int]int{4: 2, 5: 2})
	for _, deck := range []sqliteStore.Deck{
		{
			ID:        "meta-deck-1",
			Source:    "remote",
			Name:      strPtrCompare("Close Deck"),
			Class:     "WARRIOR",
			Format:    "standard",
			DeckCode:  &closeDeckCode,
			Archetype: strPtrCompare("Midrange"),
		},
		{
			ID:        "meta-deck-2",
			Source:    "remote",
			Name:      strPtrCompare("Far Deck"),
			Class:     "WARRIOR",
			Format:    "standard",
			DeckCode:  &farDeckCode,
			Archetype: strPtrCompare("Control"),
		},
	} {
		if _, err := repos.Decks.UpsertMetaDeck(ctx, deck); err != nil {
			t.Fatalf("UpsertMetaDeck(%q) error = %v", deck.ID, err)
		}
	}

	if err := repos.MetaDecks.ReplaceSnapshotDecks(ctx, "snapshot-1", []sqliteStore.MetaDeck{
		{SnapshotID: "snapshot-1", DeckID: "meta-deck-1", Playrate: floatPtrCompare(12), Winrate: floatPtrCompare(51), Tier: strPtrCompare("T1")},
		{SnapshotID: "snapshot-1", DeckID: "meta-deck-2", Playrate: floatPtrCompare(7), Winrate: floatPtrCompare(49), Tier: strPtrCompare("T3")},
	}); err != nil {
		t.Fatalf("ReplaceSnapshotDecks() error = %v", err)
	}

	parser := decks.NewParser(sqliteStore.NewCardLookupRepository(db))
	service := NewService(parser, repos.MetaSnapshots, repos.MetaDecks, repos.Cards, repos.DeckCards)

	result, err := service.CompareDeck(ctx, mustEncodeCompareDeckstring(t, 2, 1, map[int]int{2: 2, 3: 2}), 2)
	if err != nil {
		t.Fatalf("CompareDeck() error = %v", err)
	}

	if result.SnapshotID != "snapshot-1" || len(result.Candidates) != 2 {
		t.Fatalf("unexpected compare result: %+v", result)
	}

	if result.Candidates[0].DeckID != "meta-deck-1" {
		t.Fatalf("expected closest deck first, got %+v", result.Candidates)
	}

	if result.Candidates[0].Similarity <= result.Candidates[1].Similarity {
		t.Fatalf("expected descending similarity, got %+v", result.Candidates)
	}

	if len(result.Candidates[0].SharedCards) != 2 {
		t.Fatalf("expected shared cards to be included, got %+v", result.Candidates[0].SharedCards)
	}

	if len(result.Candidates[0].MissingFromInput) != 1 || result.Candidates[0].MissingFromInput[0].CardID != "CARD_004" {
		t.Fatalf("expected meta-only diff card, got %+v", result.Candidates[0].MissingFromInput)
	}

	if len(result.Candidates[0].MissingFromMeta) != 0 {
		t.Fatalf("expected no input-only cards for closest deck, got %+v", result.Candidates[0].MissingFromMeta)
	}

	if len(result.Candidates[0].SuggestedAdds) != 1 {
		t.Fatalf("expected suggested adds to be populated, got %+v", result.Candidates[0].SuggestedAdds)
	}

	if result.Candidates[0].SuggestedCuts != nil && len(result.Candidates[0].SuggestedCuts) != 0 {
		t.Fatalf("expected no suggested cuts for closest deck, got %+v", result.Candidates[0].SuggestedCuts)
	}

	if len(result.MergedSuggestedAdds) == 0 {
		t.Fatalf("expected merged add guidance, got %+v", result)
	}

	if len(result.MergedGuidance.Adds) == 0 || result.MergedGuidance.Adds[0].Source == "" || result.MergedGuidance.Adds[0].Confidence <= 0 {
		t.Fatalf("expected structured merged guidance with source/confidence, got %+v", result.MergedGuidance)
	}

	if len(result.MergedGuidance.Adds[0].Support) == 0 {
		t.Fatalf("expected structured merged guidance support, got %+v", result.MergedGuidance.Adds[0])
	}

	joinedAdds := strings.ToLower(strings.Join(result.MergedSuggestedAdds, " "))
	if !strings.Contains(joinedAdds, "package") || !strings.Contains(joinedAdds, "close deck") {
		t.Fatalf("expected merged adds to connect package analysis with top compare candidate, got %+v", result.MergedSuggestedAdds)
	}

	if len(result.MergedSummary) == 0 {
		t.Fatalf("expected merged summary, got %+v", result)
	}
}

func TestServiceCompareDeckCanUseSnapshotCardLinesWithoutDeckCode(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCompareTestDB(t)
	repos := sqliteStore.NewRepositories(db)

	if err := repos.Cards.UpsertMany(ctx, []cards.Card{
		testCompareCard("HERO_001", 1, "WARRIOR", 0, "Garrosh"),
		testCompareCard("CARD_002", 2, "DEMONHUNTER", 1, "Illidari Studies"),
		testCompareCard("CARD_003", 3, "DEMONHUNTER", 2, "Broxigar"),
		testCompareCard("CARD_004", 4, "DEMONHUNTER", 3, "Chaos Strike"),
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	if err := repos.MetaSnapshots.Create(ctx, sqliteStore.MetaSnapshot{
		ID:           "snapshot-vs",
		Source:       "vicioussyndicate",
		PatchVersion: "vS Data Reaper Report #344",
		Format:       "standard",
		FetchedAt:    time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
		RawPayload: `{
  "report_title": "vS Data Reaper Report #344",
  "decks": [
    {
      "external_ref": "https://example.test/decks/blob-broxigar-demon-hunter-2/",
      "name": "Blob Broxigar Demon Hunter",
      "class": "DEMONHUNTER",
      "format": "standard",
      "cards": ["2x Illidari Studies", "2x Broxigar", "2x Chaos Strike"]
    }
  ]
}`,
	}); err != nil {
		t.Fatalf("Create(snapshot) error = %v", err)
	}

	parser := decks.NewParser(sqliteStore.NewCardLookupRepository(db))
	service := NewService(parser, repos.MetaSnapshots, repos.MetaDecks, repos.Cards, repos.DeckCards)

	result, err := service.CompareDeck(ctx, mustEncodeCompareDeckstring(t, 2, 1, map[int]int{2: 2, 3: 2}), 3)
	if err != nil {
		t.Fatalf("CompareDeck() error = %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 fallback compare candidate, got %+v", result.Candidates)
	}

	if result.Candidates[0].Name != "Blob Broxigar Demon Hunter" {
		t.Fatalf("expected fallback candidate name, got %+v", result.Candidates[0])
	}

	if len(result.Candidates[0].MissingFromInput) != 1 || result.Candidates[0].MissingFromInput[0].CardID != "CARD_004" {
		t.Fatalf("expected fallback diff from snapshot card lines, got %+v", result.Candidates[0].MissingFromInput)
	}
}

func TestServiceCompareDeckNormalizesSnapshotCardLineNames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCompareTestDB(t)
	repos := sqliteStore.NewRepositories(db)

	if err := repos.Cards.UpsertMany(ctx, []cards.Card{
		testCompareCard("HERO_001", 1, "PRIEST", 0, "Anduin"),
		testCompareCard("CARD_010", 10, "PRIEST", 3, "Pop-Up Book"),
		testCompareCard("CARD_011", 11, "PRIEST", 4, "Maiev Shadowsong"),
		testCompareCard("CARD_012", 12, "PRIEST", 2, "Creation Protocol"),
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	if err := repos.MetaSnapshots.Create(ctx, sqliteStore.MetaSnapshot{
		ID:           "snapshot-normalized",
		Source:       "remote",
		PatchVersion: "32.6.0",
		Format:       "standard",
		FetchedAt:    time.Date(2026, 3, 26, 18, 0, 0, 0, time.UTC),
		RawPayload: `{
  "decks": [
    {
      "external_ref": "https://example.test/decks/value-priest/",
      "name": "Value Priest",
      "class": "PRIEST",
      "format": "standard",
      "cards": ["2x Pop Up Book", "1x Maiev Shadowsong (Core)", "2x Creation Protocol"]
    }
  ]
}`,
	}); err != nil {
		t.Fatalf("Create(snapshot) error = %v", err)
	}

	parser := decks.NewParser(sqliteStore.NewCardLookupRepository(db))
	service := NewService(parser, repos.MetaSnapshots, repos.MetaDecks, repos.Cards, repos.DeckCards)

	result, err := service.CompareDeck(ctx, mustEncodeCompareDeckstring(t, 2, 1, map[int]int{10: 2, 11: 1}), 3)
	if err != nil {
		t.Fatalf("CompareDeck() error = %v", err)
	}

	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 normalized fallback compare candidate, got %+v", result.Candidates)
	}

	if len(result.Candidates[0].MissingFromInput) != 1 || result.Candidates[0].MissingFromInput[0].CardID != "CARD_012" {
		t.Fatalf("expected normalized fallback diff, got %+v", result.Candidates[0].MissingFromInput)
	}
}

func TestScoreDeckSimilarityPrefersCloserStructureWhenOverlapTies(t *testing.T) {
	t.Parallel()

	input := []decks.DeckCard{
		{CardID: "CARD_001", Count: 2, Cost: 1, CardType: "MINION"},
		{CardID: "CARD_002", Count: 2, Cost: 2, CardType: "SPELL"},
		{CardID: "CARD_003", Count: 2, Cost: 5, CardType: "MINION"},
		{CardID: "CARD_004", Count: 2, Cost: 6, CardType: "SPELL"},
	}

	curveClose := []decks.DeckCard{
		{CardID: "CARD_001", Count: 2, Cost: 1, CardType: "MINION"},
		{CardID: "CARD_002", Count: 2, Cost: 2, CardType: "SPELL"},
		{CardID: "CARD_005", Count: 2, Cost: 5, CardType: "MINION"},
		{CardID: "CARD_006", Count: 2, Cost: 6, CardType: "SPELL"},
	}

	curveFar := []decks.DeckCard{
		{CardID: "CARD_001", Count: 2, Cost: 1, CardType: "MINION"},
		{CardID: "CARD_002", Count: 2, Cost: 2, CardType: "SPELL"},
		{CardID: "CARD_007", Count: 2, Cost: 1, CardType: "MINION"},
		{CardID: "CARD_008", Count: 2, Cost: 1, CardType: "MINION"},
	}

	closeScore := scoreDeckSimilarity(input, curveClose)
	farScore := scoreDeckSimilarity(input, curveFar)

	if closeScore <= farScore {
		t.Fatalf("expected structurally closer deck to score higher, close=%f far=%f", closeScore, farScore)
	}
}

func TestBuildComparisonSummaryDescribesStructuralDifferences(t *testing.T) {
	t.Parallel()

	input := []decks.DeckCard{
		{CardID: "CARD_001", Count: 2, Cost: 1, CardType: "MINION"},
		{CardID: "CARD_002", Count: 2, Cost: 2, CardType: "MINION"},
		{CardID: "CARD_003", Count: 2, Cost: 3, CardType: "SPELL"},
		{CardID: "CARD_004", Count: 2, Cost: 4, CardType: "SPELL"},
	}

	metaCards := []decks.DeckCard{
		{CardID: "CARD_001", Count: 2, Cost: 1, CardType: "MINION"},
		{CardID: "CARD_002", Count: 2, Cost: 2, CardType: "MINION"},
		{CardID: "CARD_005", Count: 2, Cost: 6, CardType: "SPELL"},
		{CardID: "CARD_006", Count: 2, Cost: 7, CardType: "SPELL"},
	}

	summary := buildComparisonSummary(input, metaCards, "Ramp Hybrid")

	if len(summary) == 0 {
		t.Fatal("expected non-empty comparison summary")
	}

	joined := strings.Join(summary, " ")
	if !strings.Contains(joined, "Ramp Hybrid") {
		t.Fatalf("expected summary to mention candidate deck name, got %v", summary)
	}
	if !strings.Contains(joined, "heavier") {
		t.Fatalf("expected summary to describe heavier curve, got %v", summary)
	}
}

func TestComputeSimilarityBreakdownReturnsWeightedComponents(t *testing.T) {
	t.Parallel()

	input := []decks.DeckCard{
		{CardID: "CARD_001", Count: 2, Cost: 1, CardType: "MINION"},
		{CardID: "CARD_002", Count: 2, Cost: 2, CardType: "SPELL"},
		{CardID: "CARD_003", Count: 2, Cost: 6, CardType: "SPELL"},
	}
	metaCards := []decks.DeckCard{
		{CardID: "CARD_001", Count: 2, Cost: 1, CardType: "MINION"},
		{CardID: "CARD_004", Count: 2, Cost: 2, CardType: "SPELL"},
		{CardID: "CARD_005", Count: 2, Cost: 6, CardType: "SPELL"},
	}

	breakdown := computeSimilarityBreakdown(input, metaCards)

	if breakdown.Overlap <= 0 || breakdown.Curve <= 0 || breakdown.CardType <= 0 {
		t.Fatalf("expected positive breakdown components, got %+v", breakdown)
	}

	expected := breakdown.Overlap*0.7 + breakdown.Curve*0.2 + breakdown.CardType*0.1
	if breakdown.Total != expected {
		t.Fatalf("expected weighted total, got %+v expected=%f", breakdown, expected)
	}
}

func TestServiceCompareDeckBreaksSimilarityTiesByTierAndPlayrate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCompareTestDB(t)
	repos := sqliteStore.NewRepositories(db)

	if err := repos.Cards.UpsertMany(ctx, []cards.Card{
		testCompareCard("HERO_001", 1, "WARRIOR", 0, "Garrosh"),
		testCompareCard("CARD_002", 2, "WARRIOR", 1, "One"),
		testCompareCard("CARD_003", 3, "WARRIOR", 2, "Two"),
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	if err := repos.MetaSnapshots.Create(ctx, sqliteStore.MetaSnapshot{
		ID:           "snapshot-tie",
		Source:       "remote",
		PatchVersion: "32.7.0",
		Format:       "standard",
		FetchedAt:    time.Date(2026, 3, 26, 19, 0, 0, 0, time.UTC),
		RawPayload:   `{"decks":[]}`,
	}); err != nil {
		t.Fatalf("Create(snapshot) error = %v", err)
	}

	sameDeckCode := mustEncodeCompareDeckstring(t, 2, 1, map[int]int{2: 2, 3: 2})
	for _, deck := range []sqliteStore.Deck{
		{
			ID:       "a-low",
			Source:   "remote",
			Name:     strPtrCompare("Lower Priority Deck"),
			Class:    "WARRIOR",
			Format:   "standard",
			DeckCode: &sameDeckCode,
		},
		{
			ID:       "z-high",
			Source:   "remote",
			Name:     strPtrCompare("Higher Priority Deck"),
			Class:    "WARRIOR",
			Format:   "standard",
			DeckCode: &sameDeckCode,
		},
	} {
		if _, err := repos.Decks.UpsertMetaDeck(ctx, deck); err != nil {
			t.Fatalf("UpsertMetaDeck(%q) error = %v", deck.ID, err)
		}
	}

	if err := repos.MetaDecks.ReplaceSnapshotDecks(ctx, "snapshot-tie", []sqliteStore.MetaDeck{
		{SnapshotID: "snapshot-tie", DeckID: "a-low", Playrate: floatPtrCompare(15), Winrate: floatPtrCompare(56), Tier: strPtrCompare("T2")},
		{SnapshotID: "snapshot-tie", DeckID: "z-high", Playrate: floatPtrCompare(12), Winrate: floatPtrCompare(51), Tier: strPtrCompare("T1")},
	}); err != nil {
		t.Fatalf("ReplaceSnapshotDecks() error = %v", err)
	}

	parser := decks.NewParser(sqliteStore.NewCardLookupRepository(db))
	service := NewService(parser, repos.MetaSnapshots, repos.MetaDecks, repos.Cards, repos.DeckCards)

	result, err := service.CompareDeck(ctx, sameDeckCode, 2)
	if err != nil {
		t.Fatalf("CompareDeck() error = %v", err)
	}

	if len(result.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %+v", result.Candidates)
	}

	if result.Candidates[0].Similarity != result.Candidates[1].Similarity {
		t.Fatalf("expected identical similarity for tie-break test, got %+v", result.Candidates)
	}

	if result.Candidates[0].DeckID != "z-high" {
		t.Fatalf("expected T1 deck to win similarity tie, got %+v", result.Candidates)
	}
}

func TestServiceCompareDeckSynthesizesMergedCutGuidanceFromPackageTension(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCompareTestDB(t)
	repos := sqliteStore.NewRepositories(db)

	if err := repos.Cards.UpsertMany(ctx, []cards.Card{
		testCompareCard("HERO_001", 1, "HUNTER", 0, "Rexxar"),
		testCompareCard("CARD_101", 101, "HUNTER", 1, "One"),
		testCompareCard("CARD_102", 102, "HUNTER", 2, "Two"),
		testCompareCard("CARD_103", 103, "HUNTER", 3, "Refill"),
		testCompareCard("CARD_104", 104, "HUNTER", 4, "Mid"),
		testCompareCard("CARD_105", 105, "HUNTER", 7, "Big One"),
		testCompareCard("CARD_106", 106, "HUNTER", 8, "Big Two"),
		testCompareCard("CARD_107", 107, "HUNTER", 9, "Big Three"),
		testCompareCard("CARD_108", 108, "HUNTER", 5, "Closer"),
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	if err := repos.MetaSnapshots.Create(ctx, sqliteStore.MetaSnapshot{
		ID:           "snapshot-merge",
		Source:       "remote",
		PatchVersion: "32.4.0",
		Format:       "standard",
		FetchedAt:    time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
		RawPayload:   `{"decks":[]}`,
	}); err != nil {
		t.Fatalf("Create(snapshot) error = %v", err)
	}

	metaDeckCode := mustEncodeCompareDeckstring(t, 2, 1, map[int]int{101: 2, 102: 2, 103: 2, 104: 2, 108: 2})
	if _, err := repos.Decks.UpsertMetaDeck(ctx, sqliteStore.Deck{
		ID:       "meta-deck-merge",
		Source:   "remote",
		Name:     strPtrCompare("Pressure Hunter"),
		Class:    "HUNTER",
		Format:   "standard",
		DeckCode: &metaDeckCode,
	}); err != nil {
		t.Fatalf("UpsertMetaDeck() error = %v", err)
	}

	if err := repos.MetaDecks.ReplaceSnapshotDecks(ctx, "snapshot-merge", []sqliteStore.MetaDeck{
		{SnapshotID: "snapshot-merge", DeckID: "meta-deck-merge", Tier: strPtrCompare("T1")},
	}); err != nil {
		t.Fatalf("ReplaceSnapshotDecks() error = %v", err)
	}

	parser := decks.NewParser(sqliteStore.NewCardLookupRepository(db))
	service := NewService(parser, repos.MetaSnapshots, repos.MetaDecks, repos.Cards, repos.DeckCards)

	inputDeckCode := mustEncodeCompareDeckstring(t, 2, 1, map[int]int{101: 2, 102: 2, 105: 2, 106: 2, 107: 2, 104: 2})
	result, err := service.CompareDeck(ctx, inputDeckCode, 1)
	if err != nil {
		t.Fatalf("CompareDeck() error = %v", err)
	}

	joinedCuts := strings.ToLower(strings.Join(result.MergedSuggestedCuts, " "))
	if !strings.Contains(joinedCuts, "pressure hunter") || !strings.Contains(joinedCuts, "top end") {
		t.Fatalf("expected merged cuts to mention candidate and package conflict, got %+v", result.MergedSuggestedCuts)
	}
}

func TestServiceCompareDeckWeightsAgreementAcrossTopCandidates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCompareTestDB(t)
	repos := sqliteStore.NewRepositories(db)

	if err := repos.Cards.UpsertMany(ctx, []cards.Card{
		testCompareCard("HERO_001", 1, "WARRIOR", 0, "Garrosh"),
		testCompareCard("CARD_201", 201, "WARRIOR", 1, "One"),
		testCompareCard("CARD_202", 202, "WARRIOR", 2, "Two"),
		testCompareCard("CARD_203", 203, "WARRIOR", 3, "Draw"),
		testCompareCard("CARD_204", 204, "WARRIOR", 4, "Mid"),
		testCompareCard("CARD_205", 205, "WARRIOR", 5, "Bridge"),
		testCompareCard("CARD_206", 206, "WARRIOR", 6, "Late"),
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	if err := repos.MetaSnapshots.Create(ctx, sqliteStore.MetaSnapshot{
		ID:           "snapshot-agree",
		Source:       "remote",
		PatchVersion: "32.4.0",
		Format:       "standard",
		FetchedAt:    time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
		RawPayload:   `{"decks":[]}`,
	}); err != nil {
		t.Fatalf("Create(snapshot) error = %v", err)
	}

	deckA := mustEncodeCompareDeckstring(t, 2, 1, map[int]int{201: 2, 202: 2, 203: 2, 204: 2, 205: 2})
	deckB := mustEncodeCompareDeckstring(t, 2, 1, map[int]int{201: 2, 202: 2, 203: 2, 204: 2, 206: 2})
	for _, deck := range []sqliteStore.Deck{
		{ID: "agree-1", Source: "remote", Name: strPtrCompare("Refill Warrior"), Class: "WARRIOR", Format: "standard", DeckCode: &deckA},
		{ID: "agree-2", Source: "remote", Name: strPtrCompare("Value Warrior"), Class: "WARRIOR", Format: "standard", DeckCode: &deckB},
	} {
		if _, err := repos.Decks.UpsertMetaDeck(ctx, deck); err != nil {
			t.Fatalf("UpsertMetaDeck(%q) error = %v", deck.ID, err)
		}
	}

	if err := repos.MetaDecks.ReplaceSnapshotDecks(ctx, "snapshot-agree", []sqliteStore.MetaDeck{
		{SnapshotID: "snapshot-agree", DeckID: "agree-1", Tier: strPtrCompare("T1"), Playrate: floatPtrCompare(12)},
		{SnapshotID: "snapshot-agree", DeckID: "agree-2", Tier: strPtrCompare("T1"), Playrate: floatPtrCompare(11)},
	}); err != nil {
		t.Fatalf("ReplaceSnapshotDecks() error = %v", err)
	}

	parser := decks.NewParser(sqliteStore.NewCardLookupRepository(db))
	service := NewService(parser, repos.MetaSnapshots, repos.MetaDecks, repos.Cards, repos.DeckCards)

	inputDeckCode := mustEncodeCompareDeckstring(t, 2, 1, map[int]int{201: 2, 202: 2, 204: 2, 205: 2})
	result, err := service.CompareDeck(ctx, inputDeckCode, 2)
	if err != nil {
		t.Fatalf("CompareDeck() error = %v", err)
	}

	joinedAdds := strings.ToLower(strings.Join(result.MergedSuggestedAdds, " "))
	if !strings.Contains(joinedAdds, "multiple close meta decks") && !strings.Contains(joinedAdds, "both") {
		t.Fatalf("expected merged adds to reflect candidate agreement, got %+v", result.MergedSuggestedAdds)
	}

	if len(result.MergedGuidance.Adds) == 0 || result.MergedGuidance.Adds[0].Source != "multi_candidate_consensus" {
		t.Fatalf("expected agreement guidance to become structured consensus output, got %+v", result.MergedGuidance)
	}
}

func TestServiceCompareDeckStaysConservativeWhenTopCandidatesDisagree(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openCompareTestDB(t)
	repos := sqliteStore.NewRepositories(db)

	if err := repos.Cards.UpsertMany(ctx, []cards.Card{
		testCompareCard("HERO_001", 1, "PRIEST", 0, "Anduin"),
		testCompareCard("CARD_301", 301, "PRIEST", 1, "One"),
		testCompareCard("CARD_302", 302, "PRIEST", 2, "Two"),
		testCompareCard("CARD_303", 303, "PRIEST", 3, "Draw"),
		testCompareCard("CARD_304", 304, "PRIEST", 4, "Removal"),
		testCompareCard("CARD_305", 305, "PRIEST", 7, "Late One"),
		testCompareCard("CARD_306", 306, "PRIEST", 8, "Late Two"),
	}); err != nil {
		t.Fatalf("UpsertMany() error = %v", err)
	}

	if err := repos.MetaSnapshots.Create(ctx, sqliteStore.MetaSnapshot{
		ID:           "snapshot-disagree",
		Source:       "remote",
		PatchVersion: "32.4.0",
		Format:       "standard",
		FetchedAt:    time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC),
		RawPayload:   `{"decks":[]}`,
	}); err != nil {
		t.Fatalf("Create(snapshot) error = %v", err)
	}

	deckA := mustEncodeCompareDeckstring(t, 2, 1, map[int]int{301: 2, 302: 2, 303: 2, 304: 2})
	deckB := mustEncodeCompareDeckstring(t, 2, 1, map[int]int{301: 2, 302: 2, 305: 2, 306: 2})
	for _, deck := range []sqliteStore.Deck{
		{ID: "disagree-1", Source: "remote", Name: strPtrCompare("Cycle Priest"), Class: "PRIEST", Format: "standard", DeckCode: &deckA},
		{ID: "disagree-2", Source: "remote", Name: strPtrCompare("Greedy Priest"), Class: "PRIEST", Format: "standard", DeckCode: &deckB},
	} {
		if _, err := repos.Decks.UpsertMetaDeck(ctx, deck); err != nil {
			t.Fatalf("UpsertMetaDeck(%q) error = %v", deck.ID, err)
		}
	}

	if err := repos.MetaDecks.ReplaceSnapshotDecks(ctx, "snapshot-disagree", []sqliteStore.MetaDeck{
		{SnapshotID: "snapshot-disagree", DeckID: "disagree-1", Tier: strPtrCompare("T1"), Playrate: floatPtrCompare(10)},
		{SnapshotID: "snapshot-disagree", DeckID: "disagree-2", Tier: strPtrCompare("T1"), Playrate: floatPtrCompare(9.5)},
	}); err != nil {
		t.Fatalf("ReplaceSnapshotDecks() error = %v", err)
	}

	parser := decks.NewParser(sqliteStore.NewCardLookupRepository(db))
	service := NewService(parser, repos.MetaSnapshots, repos.MetaDecks, repos.Cards, repos.DeckCards)

	inputDeckCode := mustEncodeCompareDeckstring(t, 2, 1, map[int]int{301: 2, 302: 2, 304: 2, 305: 2})
	result, err := service.CompareDeck(ctx, inputDeckCode, 2)
	if err != nil {
		t.Fatalf("CompareDeck() error = %v", err)
	}

	joinedSummary := strings.ToLower(strings.Join(result.MergedSummary, " "))
	if !strings.Contains(joinedSummary, "split") && !strings.Contains(joinedSummary, "mixed") {
		t.Fatalf("expected merged summary to stay conservative when candidates disagree, got %+v", result.MergedSummary)
	}

	if len(result.MergedGuidance.Summary) == 0 || result.MergedGuidance.Summary[0].Source != "conflict_caution" {
		t.Fatalf("expected disagreement guidance to become structured caution output, got %+v", result.MergedGuidance)
	}
}

func openCompareTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "compare.db")
	db, err := sqliteStore.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := sqliteStore.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	return db
}

func testCompareCard(id string, dbfID int, class string, cost int, name string) cards.Card {
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
		Locales:       []cards.LocaleText{{Locale: "enUS", Name: name, Text: name + " text"}},
	}
}

func mustEncodeCompareDeckstring(t *testing.T, format int, heroDBFID int, cardCounts map[int]int) string {
	t.Helper()

	buf := &bytes.Buffer{}
	buf.WriteByte(0)
	writeCompareVarint(buf, int64(format))
	writeCompareVarint(buf, 1)
	writeCompareVarint(buf, int64(heroDBFID))

	singles := make([]int, 0)
	doubles := make([]int, 0)
	for dbfID, count := range cardCounts {
		if count == 1 {
			singles = append(singles, dbfID)
		}
		if count == 2 {
			doubles = append(doubles, dbfID)
		}
	}

	writeCompareVarint(buf, int64(len(singles)))
	for _, id := range singles {
		writeCompareVarint(buf, int64(id))
	}

	writeCompareVarint(buf, int64(len(doubles)))
	for _, id := range doubles {
		writeCompareVarint(buf, int64(id))
	}

	writeCompareVarint(buf, 0)
	return base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func writeCompareVarint(buf *bytes.Buffer, value int64) {
	tmp := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(tmp, uint64(value))
	buf.Write(tmp[:n])
}

func strPtrCompare(v string) *string     { return &v }
func floatPtrCompare(v float64) *float64 { return &v }
