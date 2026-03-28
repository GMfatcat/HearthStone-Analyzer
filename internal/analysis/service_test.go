package analysis_test

import (
	"strings"
	"testing"

	"hearthstone-analyzer/internal/analysis"
	"hearthstone-analyzer/internal/cards"
	"hearthstone-analyzer/internal/decks"
)

func TestAnalyzerExtractsBasicFeaturesAndClassifiesAggro(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "HUNTER",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "C1", Name: "One Drop", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "C2", Name: "Two Drop", Count: 2, Cost: 2, Class: "HUNTER", CardType: "MINION"},
			{CardID: "C3", Name: "Burn", Count: 2, Cost: 1, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "C4", Name: "Curve", Count: 2, Cost: 2, Class: "NEUTRAL", CardType: "MINION"},
			{CardID: "C5", Name: "Push", Count: 2, Cost: 3, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "C6", Name: "Topper", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
			{CardID: "C7", Name: "Filler7", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "C8", Name: "Filler8", Count: 2, Cost: 2, Class: "HUNTER", CardType: "MINION"},
			{CardID: "C9", Name: "Filler9", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "C10", Name: "Filler10", Count: 2, Cost: 1, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "C11", Name: "Filler11", Count: 2, Cost: 2, Class: "HUNTER", CardType: "MINION"},
			{CardID: "C12", Name: "Filler12", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "C13", Name: "Filler13", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "C14", Name: "Filler14", Count: 2, Cost: 2, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "C15", Name: "Filler15", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)

	if got.Archetype != "Aggro" {
		t.Fatalf("expected archetype %q, got %q", "Aggro", got.Archetype)
	}

	if got.Features.AvgCost <= 0 || got.Features.AvgCost >= 3 {
		t.Fatalf("expected low avg cost, got %f", got.Features.AvgCost)
	}

	if got.Features.MinionCount <= got.Features.SpellCount {
		t.Fatalf("expected more minions than spells, got minions=%d spells=%d", got.Features.MinionCount, got.Features.SpellCount)
	}

	if got.Features.ManaCurve[1] == 0 {
		t.Fatalf("expected mana curve to include one-cost cards, got %+v", got.Features.ManaCurve)
	}

	if len(got.Strengths) == 0 || len(got.Weaknesses) == 0 {
		t.Fatalf("expected strengths and weaknesses, got %+v", got)
	}

	if got.Confidence < 0.7 {
		t.Fatalf("expected aggro deck confidence to be reasonably high, got %f", got.Confidence)
	}

	if len(got.ConfidenceReasons) == 0 {
		t.Fatalf("expected confidence reasons for aggro deck, got %+v", got)
	}
}

func TestAnalyzerClassifiesControlForTopHeavyDeck(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "PRIEST",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "C1", Name: "Removal", Count: 2, Cost: 4, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "C2", Name: "Big Drop", Count: 2, Cost: 8, Class: "PRIEST", CardType: "MINION"},
			{CardID: "C3", Name: "Board Clear", Count: 2, Cost: 6, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "C4", Name: "Value", Count: 2, Cost: 7, Class: "PRIEST", CardType: "MINION"},
			{CardID: "C5", Name: "Late", Count: 2, Cost: 9, Class: "PRIEST", CardType: "MINION"},
			{CardID: "C6", Name: "Stall", Count: 2, Cost: 5, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "C7", Name: "Filler7", Count: 2, Cost: 6, Class: "PRIEST", CardType: "MINION"},
			{CardID: "C8", Name: "Filler8", Count: 2, Cost: 7, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "C9", Name: "Filler9", Count: 2, Cost: 8, Class: "PRIEST", CardType: "MINION"},
			{CardID: "C10", Name: "Filler10", Count: 2, Cost: 9, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "C11", Name: "Filler11", Count: 2, Cost: 5, Class: "PRIEST", CardType: "MINION"},
			{CardID: "C12", Name: "Filler12", Count: 2, Cost: 6, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "C13", Name: "Filler13", Count: 2, Cost: 7, Class: "PRIEST", CardType: "MINION"},
			{CardID: "C14", Name: "Filler14", Count: 2, Cost: 8, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "C15", Name: "Filler15", Count: 2, Cost: 9, Class: "PRIEST", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)

	if got.Archetype != "Control" {
		t.Fatalf("expected archetype %q, got %q", "Control", got.Archetype)
	}
}

func TestAnalyzerComputesPhaseScoresAndCurveSignals(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "MAGE",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "C1", Name: "One", Count: 2, Cost: 1, Class: "MAGE", CardType: "SPELL"},
			{CardID: "C2", Name: "Two", Count: 2, Cost: 2, Class: "MAGE", CardType: "MINION"},
			{CardID: "C3", Name: "Three", Count: 2, Cost: 3, Class: "MAGE", CardType: "MINION"},
			{CardID: "C4", Name: "Four", Count: 2, Cost: 4, Class: "MAGE", CardType: "SPELL"},
			{CardID: "C5", Name: "Five", Count: 2, Cost: 5, Class: "MAGE", CardType: "MINION"},
			{CardID: "C6", Name: "Six", Count: 2, Cost: 6, Class: "MAGE", CardType: "SPELL"},
			{CardID: "C7", Name: "Seven", Count: 2, Cost: 7, Class: "MAGE", CardType: "MINION"},
			{CardID: "C8", Name: "Eight", Count: 2, Cost: 8, Class: "MAGE", CardType: "SPELL"},
			{CardID: "C9", Name: "Nine", Count: 2, Cost: 1, Class: "MAGE", CardType: "MINION"},
			{CardID: "C10", Name: "Ten", Count: 2, Cost: 2, Class: "MAGE", CardType: "SPELL"},
			{CardID: "C11", Name: "Eleven", Count: 2, Cost: 6, Class: "MAGE", CardType: "MINION"},
			{CardID: "C12", Name: "Twelve", Count: 2, Cost: 7, Class: "MAGE", CardType: "SPELL"},
			{CardID: "C13", Name: "Thirteen", Count: 2, Cost: 8, Class: "MAGE", CardType: "MINION"},
			{CardID: "C14", Name: "Fourteen", Count: 2, Cost: 1, Class: "MAGE", CardType: "SPELL"},
			{CardID: "C15", Name: "Fifteen", Count: 2, Cost: 2, Class: "MAGE", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)

	if got.Features.EarlyGameScore <= 0 {
		t.Fatalf("expected positive early game score, got %f", got.Features.EarlyGameScore)
	}

	if got.Features.LateGameScore <= 0 {
		t.Fatalf("expected positive late game score, got %f", got.Features.LateGameScore)
	}

	if got.Features.EarlyCurveCount == 0 {
		t.Fatalf("expected early curve count to be populated, got %+v", got.Features)
	}

	if got.Features.TopHeavyCount == 0 {
		t.Fatalf("expected top heavy count to be populated, got %+v", got.Features)
	}

	if got.Features.CurveBalanceScore <= 0 {
		t.Fatalf("expected curve balance score to be positive, got %f", got.Features.CurveBalanceScore)
	}
}

func TestAnalyzerSurfacesTopHeavyWeakness(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "WARLOCK",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "C1", Name: "Big 1", Count: 2, Cost: 8, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "C2", Name: "Big 2", Count: 2, Cost: 9, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "C3", Name: "Big 3", Count: 2, Cost: 10, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "C4", Name: "Big 4", Count: 2, Cost: 7, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "C5", Name: "Big 5", Count: 2, Cost: 8, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "C6", Name: "Big 6", Count: 2, Cost: 9, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "C7", Name: "Big 7", Count: 2, Cost: 10, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "C8", Name: "Big 8", Count: 2, Cost: 7, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "C9", Name: "Big 9", Count: 2, Cost: 8, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "C10", Name: "Big 10", Count: 2, Cost: 9, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "C11", Name: "Big 11", Count: 2, Cost: 10, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "C12", Name: "Big 12", Count: 2, Cost: 7, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "C13", Name: "Small 1", Count: 2, Cost: 1, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "C14", Name: "Small 2", Count: 2, Cost: 2, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "C15", Name: "Small 3", Count: 2, Cost: 3, Class: "WARLOCK", CardType: "SPELL"},
		},
	}

	got := analyzer.Analyze(input)

	found := false
	for _, weakness := range got.Weaknesses {
		if weakness == "Top-heavy curve can create clunky early turns" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected top-heavy weakness, got %+v", got.Weaknesses)
	}
}

func TestAnalyzerComputesFunctionalSignalsFromCardText(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "MAGE",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "D1", Name: "Arcane Intellect", Count: 2, Cost: 3, Class: "MAGE", CardType: "SPELL", Text: "Draw 2 cards."},
			{CardID: "D2", Name: "Fireball", Count: 2, Cost: 4, Class: "MAGE", CardType: "SPELL", Text: "Deal 6 damage."},
			{CardID: "D3", Name: "Blizzard", Count: 2, Cost: 6, Class: "MAGE", CardType: "SPELL", Text: "Deal 2 damage to all enemy minions."},
			{CardID: "D4", Name: "Light Heal", Count: 2, Cost: 2, Class: "MAGE", CardType: "SPELL", Text: "Restore 4 Health."},
			{CardID: "D5", Name: "Discovery Orb", Count: 2, Cost: 2, Class: "MAGE", CardType: "SPELL", Text: "Discover a spell."},
			{CardID: "D6", Name: "Single Shot", Count: 2, Cost: 3, Class: "MAGE", CardType: "SPELL", Text: "Destroy an enemy minion."},
			{CardID: "D7", Name: "Filler1", Count: 2, Cost: 1, Class: "MAGE", CardType: "MINION", Text: ""},
			{CardID: "D8", Name: "Filler2", Count: 2, Cost: 1, Class: "MAGE", CardType: "MINION", Text: ""},
			{CardID: "D9", Name: "Filler3", Count: 2, Cost: 2, Class: "MAGE", CardType: "MINION", Text: ""},
			{CardID: "D10", Name: "Filler4", Count: 2, Cost: 2, Class: "MAGE", CardType: "MINION", Text: ""},
			{CardID: "D11", Name: "Filler5", Count: 2, Cost: 3, Class: "MAGE", CardType: "MINION", Text: ""},
			{CardID: "D12", Name: "Filler6", Count: 2, Cost: 3, Class: "MAGE", CardType: "MINION", Text: ""},
			{CardID: "D13", Name: "Filler7", Count: 2, Cost: 4, Class: "MAGE", CardType: "MINION", Text: ""},
			{CardID: "D14", Name: "Filler8", Count: 2, Cost: 4, Class: "MAGE", CardType: "MINION", Text: ""},
			{CardID: "D15", Name: "Filler9", Count: 2, Cost: 5, Class: "MAGE", CardType: "MINION", Text: ""},
		},
	}

	got := analyzer.Analyze(input)

	if got.Features.DrawCount != 2 {
		t.Fatalf("expected draw count 2, got %d", got.Features.DrawCount)
	}

	if got.Features.BurnCount != 4 {
		t.Fatalf("expected burn count 4, got %d", got.Features.BurnCount)
	}

	if got.Features.AoeCount != 2 {
		t.Fatalf("expected aoe count 2, got %d", got.Features.AoeCount)
	}

	if got.Features.HealCount != 2 {
		t.Fatalf("expected heal count 2, got %d", got.Features.HealCount)
	}

	if got.Features.DiscoverCount != 2 {
		t.Fatalf("expected discover count 2, got %d", got.Features.DiscoverCount)
	}

	if got.Features.SingleRemovalCount != 2 {
		t.Fatalf("expected single removal count 2, got %d", got.Features.SingleRemovalCount)
	}
}

func TestAnalyzerComputesExtendedFunctionalSignals(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "PALADIN",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "E1", Name: "Shielded Guard", Count: 2, Cost: 3, Class: "PALADIN", CardType: "MINION", Text: "Taunt. Battlecry: Summon a 1/1 Silver Hand Recruit."},
			{CardID: "E2", Name: "Haunted Relic", Count: 2, Cost: 2, Class: "PALADIN", CardType: "MINION", Text: "Deathrattle: Summon two 1/1 Spirits."},
			{CardID: "E3", Name: "Combo Spark", Count: 2, Cost: 1, Class: "PALADIN", CardType: "SPELL", Text: "If you played another card this turn, deal 3 damage."},
			{CardID: "E4", Name: "Discount Aura", Count: 2, Cost: 4, Class: "PALADIN", CardType: "SPELL", Text: "Your next minion costs (2) less."},
			{CardID: "E5", Name: "Filler1", Count: 2, Cost: 1, Class: "PALADIN", CardType: "MINION", Text: ""},
			{CardID: "E6", Name: "Filler2", Count: 2, Cost: 2, Class: "PALADIN", CardType: "MINION", Text: ""},
			{CardID: "E7", Name: "Filler3", Count: 2, Cost: 3, Class: "PALADIN", CardType: "MINION", Text: ""},
			{CardID: "E8", Name: "Filler4", Count: 2, Cost: 4, Class: "PALADIN", CardType: "MINION", Text: ""},
			{CardID: "E9", Name: "Filler5", Count: 2, Cost: 5, Class: "PALADIN", CardType: "MINION", Text: ""},
			{CardID: "E10", Name: "Filler6", Count: 2, Cost: 1, Class: "PALADIN", CardType: "SPELL", Text: ""},
			{CardID: "E11", Name: "Filler7", Count: 2, Cost: 2, Class: "PALADIN", CardType: "SPELL", Text: ""},
			{CardID: "E12", Name: "Filler8", Count: 2, Cost: 3, Class: "PALADIN", CardType: "SPELL", Text: ""},
			{CardID: "E13", Name: "Filler9", Count: 2, Cost: 4, Class: "PALADIN", CardType: "MINION", Text: ""},
			{CardID: "E14", Name: "Filler10", Count: 2, Cost: 5, Class: "PALADIN", CardType: "MINION", Text: ""},
			{CardID: "E15", Name: "Filler11", Count: 2, Cost: 6, Class: "PALADIN", CardType: "MINION", Text: ""},
		},
	}

	got := analyzer.Analyze(input)

	if got.Features.TauntCount != 2 {
		t.Fatalf("expected taunt count 2, got %d", got.Features.TauntCount)
	}

	if got.Features.TokenCount != 4 {
		t.Fatalf("expected token count 4, got %d", got.Features.TokenCount)
	}

	if got.Features.DeathrattleCount != 2 {
		t.Fatalf("expected deathrattle count 2, got %d", got.Features.DeathrattleCount)
	}

	if got.Features.BattlecryCount != 2 {
		t.Fatalf("expected battlecry count 2, got %d", got.Features.BattlecryCount)
	}

	if got.Features.ManaCheatCount != 2 {
		t.Fatalf("expected mana cheat count 2, got %d", got.Features.ManaCheatCount)
	}

	if got.Features.ComboPieceCount != 2 {
		t.Fatalf("expected combo piece count 2, got %d", got.Features.ComboPieceCount)
	}
}

func TestAnalyzerAssignsLowerConfidenceToAmbiguousMidrangeDeck(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "DRUID",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "M1", Name: "One", Count: 2, Cost: 1, Class: "DRUID", CardType: "MINION"},
			{CardID: "M2", Name: "Two", Count: 2, Cost: 2, Class: "DRUID", CardType: "SPELL"},
			{CardID: "M3", Name: "Three", Count: 2, Cost: 3, Class: "DRUID", CardType: "MINION"},
			{CardID: "M4", Name: "Four", Count: 2, Cost: 4, Class: "DRUID", CardType: "SPELL"},
			{CardID: "M5", Name: "Five", Count: 2, Cost: 5, Class: "DRUID", CardType: "MINION"},
			{CardID: "M6", Name: "Six", Count: 2, Cost: 6, Class: "DRUID", CardType: "SPELL"},
			{CardID: "M7", Name: "Seven", Count: 2, Cost: 2, Class: "DRUID", CardType: "MINION"},
			{CardID: "M8", Name: "Eight", Count: 2, Cost: 3, Class: "DRUID", CardType: "SPELL"},
			{CardID: "M9", Name: "Nine", Count: 2, Cost: 4, Class: "DRUID", CardType: "MINION"},
			{CardID: "M10", Name: "Ten", Count: 2, Cost: 5, Class: "DRUID", CardType: "SPELL"},
			{CardID: "M11", Name: "Eleven", Count: 2, Cost: 3, Class: "DRUID", CardType: "MINION"},
			{CardID: "M12", Name: "Twelve", Count: 2, Cost: 4, Class: "DRUID", CardType: "SPELL"},
			{CardID: "M13", Name: "Thirteen", Count: 2, Cost: 5, Class: "DRUID", CardType: "MINION"},
			{CardID: "M14", Name: "Fourteen", Count: 2, Cost: 4, Class: "DRUID", CardType: "SPELL"},
			{CardID: "M15", Name: "Fifteen", Count: 2, Cost: 3, Class: "DRUID", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)

	if got.Archetype != "Midrange" {
		t.Fatalf("expected ambiguous deck to stay midrange, got %q", got.Archetype)
	}

	if got.Confidence >= 0.7 {
		t.Fatalf("expected ambiguous midrange confidence to stay lower, got %f", got.Confidence)
	}

	if len(got.ConfidenceReasons) == 0 {
		t.Fatalf("expected confidence reasons for ambiguous deck, got %+v", got)
	}
}

func TestAnalyzerConfidenceReasonsExplainArchetypeSignals(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "HUNTER",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "R1", Name: "One", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "R2", Name: "Two", Count: 2, Cost: 2, Class: "HUNTER", CardType: "MINION"},
			{CardID: "R3", Name: "Three", Count: 2, Cost: 3, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "R4", Name: "Four", Count: 2, Cost: 1, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "R5", Name: "Five", Count: 2, Cost: 2, Class: "HUNTER", CardType: "MINION"},
			{CardID: "R6", Name: "Six", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "R7", Name: "Seven", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "R8", Name: "Eight", Count: 2, Cost: 2, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "R9", Name: "Nine", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "R10", Name: "Ten", Count: 2, Cost: 1, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "R11", Name: "Eleven", Count: 2, Cost: 2, Class: "HUNTER", CardType: "MINION"},
			{CardID: "R12", Name: "Twelve", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "R13", Name: "Thirteen", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
			{CardID: "R14", Name: "Fourteen", Count: 2, Cost: 4, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "R15", Name: "Fifteen", Count: 2, Cost: 5, Class: "HUNTER", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	joined := strings.Join(got.ConfidenceReasons, " ")
	if !strings.Contains(joined, "early") && !strings.Contains(joined, "low-curve") {
		t.Fatalf("expected confidence reasons to mention aggro-style evidence, got %+v", got.ConfidenceReasons)
	}
}

func TestAnalyzerSuggestsAddsForLowResourceAggroDeck(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "HUNTER",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "A1", Name: "One", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "A2", Name: "Two", Count: 2, Cost: 2, Class: "HUNTER", CardType: "MINION"},
			{CardID: "A3", Name: "Three", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "A4", Name: "Four", Count: 2, Cost: 1, Class: "HUNTER", CardType: "SPELL", Text: "Deal 2 damage."},
			{CardID: "A5", Name: "Five", Count: 2, Cost: 2, Class: "HUNTER", CardType: "SPELL", Text: "Deal 3 damage."},
			{CardID: "A6", Name: "Six", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "A7", Name: "Seven", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "A8", Name: "Eight", Count: 2, Cost: 2, Class: "HUNTER", CardType: "MINION"},
			{CardID: "A9", Name: "Nine", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "A10", Name: "Ten", Count: 2, Cost: 1, Class: "HUNTER", CardType: "SPELL", Text: "Deal 2 damage."},
			{CardID: "A11", Name: "Eleven", Count: 2, Cost: 2, Class: "HUNTER", CardType: "MINION"},
			{CardID: "A12", Name: "Twelve", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "A13", Name: "Thirteen", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
			{CardID: "A14", Name: "Fourteen", Count: 2, Cost: 4, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "A15", Name: "Fifteen", Count: 2, Cost: 5, Class: "HUNTER", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	joined := strings.Join(got.SuggestedAdds, " ")
	if !strings.Contains(strings.ToLower(joined), "draw") {
		t.Fatalf("expected low-resource aggro deck to suggest card draw, got %+v", got.SuggestedAdds)
	}
}

func TestAnalyzerSuggestsCutsForTopHeavyDeck(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "WARLOCK",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "T1", Name: "Big 1", Count: 2, Cost: 8, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "T2", Name: "Big 2", Count: 2, Cost: 9, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "T3", Name: "Big 3", Count: 2, Cost: 10, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "T4", Name: "Big 4", Count: 2, Cost: 7, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "T5", Name: "Big 5", Count: 2, Cost: 8, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "T6", Name: "Big 6", Count: 2, Cost: 9, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "T7", Name: "Big 7", Count: 2, Cost: 10, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "T8", Name: "Big 8", Count: 2, Cost: 7, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "T9", Name: "Big 9", Count: 2, Cost: 8, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "T10", Name: "Big 10", Count: 2, Cost: 9, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "T11", Name: "Big 11", Count: 2, Cost: 10, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "T12", Name: "Big 12", Count: 2, Cost: 7, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "T13", Name: "Small 1", Count: 2, Cost: 1, Class: "WARLOCK", CardType: "SPELL"},
			{CardID: "T14", Name: "Small 2", Count: 2, Cost: 2, Class: "WARLOCK", CardType: "MINION"},
			{CardID: "T15", Name: "Small 3", Count: 2, Cost: 3, Class: "WARLOCK", CardType: "SPELL"},
		},
	}

	got := analyzer.Analyze(input)
	joined := strings.Join(got.SuggestedCuts, " ")
	if !strings.Contains(strings.ToLower(joined), "high-cost") && !strings.Contains(strings.ToLower(joined), "top end") {
		t.Fatalf("expected top-heavy deck to suggest trimming expensive cards, got %+v", got.SuggestedCuts)
	}
}

func TestAnalyzerSuggestsEarlyMinionAddsForSpellHeavyMidrangeDeck(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "MAGE",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "S1", Name: "Burn 1", Count: 2, Cost: 1, Class: "MAGE", CardType: "SPELL", Text: "Deal 2 damage."},
			{CardID: "S2", Name: "Burn 2", Count: 2, Cost: 2, Class: "MAGE", CardType: "SPELL", Text: "Deal 3 damage."},
			{CardID: "S3", Name: "Draw", Count: 2, Cost: 3, Class: "MAGE", CardType: "SPELL", Text: "Draw 2 cards."},
			{CardID: "S4", Name: "Removal", Count: 2, Cost: 4, Class: "MAGE", CardType: "SPELL", Text: "Destroy an enemy minion."},
			{CardID: "S5", Name: "AoE", Count: 2, Cost: 5, Class: "MAGE", CardType: "SPELL", Text: "Deal 2 damage to all enemy minions."},
			{CardID: "S6", Name: "Spell 6", Count: 2, Cost: 6, Class: "MAGE", CardType: "SPELL"},
			{CardID: "S7", Name: "Spell 7", Count: 2, Cost: 7, Class: "MAGE", CardType: "SPELL"},
			{CardID: "S8", Name: "Spell 8", Count: 2, Cost: 4, Class: "MAGE", CardType: "SPELL"},
			{CardID: "S9", Name: "Minion 1", Count: 2, Cost: 4, Class: "MAGE", CardType: "MINION"},
			{CardID: "S10", Name: "Minion 2", Count: 2, Cost: 5, Class: "MAGE", CardType: "MINION"},
			{CardID: "S11", Name: "Minion 3", Count: 2, Cost: 6, Class: "MAGE", CardType: "MINION"},
			{CardID: "S12", Name: "Minion 4", Count: 2, Cost: 5, Class: "MAGE", CardType: "MINION"},
			{CardID: "S13", Name: "Minion 5", Count: 2, Cost: 6, Class: "MAGE", CardType: "MINION"},
			{CardID: "S14", Name: "Minion 6", Count: 2, Cost: 7, Class: "MAGE", CardType: "MINION"},
			{CardID: "S15", Name: "Minion 7", Count: 2, Cost: 8, Class: "MAGE", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	joined := strings.ToLower(strings.Join(got.SuggestedAdds, " "))
	if !strings.Contains(joined, "one- to three-cost") && !strings.Contains(joined, "early") {
		t.Fatalf("expected spell-heavy deck to get early-board add advice, got %+v", got.SuggestedAdds)
	}
}

func TestAnalyzerSuggestsReactiveSpellCutsWhenTheyCrowdOutThreats(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "PRIEST",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "R1", Name: "Spell 1", Count: 2, Cost: 1, Class: "PRIEST", CardType: "SPELL", Text: "Destroy an enemy minion."},
			{CardID: "R2", Name: "Spell 2", Count: 2, Cost: 2, Class: "PRIEST", CardType: "SPELL", Text: "Deal 2 damage to all enemy minions."},
			{CardID: "R3", Name: "Spell 3", Count: 2, Cost: 3, Class: "PRIEST", CardType: "SPELL", Text: "Discover a spell."},
			{CardID: "R4", Name: "Spell 4", Count: 2, Cost: 4, Class: "PRIEST", CardType: "SPELL", Text: "Restore 6 Health."},
			{CardID: "R5", Name: "Spell 5", Count: 2, Cost: 5, Class: "PRIEST", CardType: "SPELL", Text: "Draw 2 cards."},
			{CardID: "R6", Name: "Spell 6", Count: 2, Cost: 6, Class: "PRIEST", CardType: "SPELL", Text: "Deal 3 damage to all minions."},
			{CardID: "R7", Name: "Spell 7", Count: 2, Cost: 7, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "R8", Name: "Spell 8", Count: 2, Cost: 8, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "R9", Name: "Spell 9", Count: 2, Cost: 4, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "R10", Name: "Minion 1", Count: 2, Cost: 4, Class: "PRIEST", CardType: "MINION"},
			{CardID: "R11", Name: "Minion 2", Count: 2, Cost: 5, Class: "PRIEST", CardType: "MINION"},
			{CardID: "R12", Name: "Minion 3", Count: 2, Cost: 6, Class: "PRIEST", CardType: "MINION"},
			{CardID: "R13", Name: "Minion 4", Count: 2, Cost: 7, Class: "PRIEST", CardType: "MINION"},
			{CardID: "R14", Name: "Minion 5", Count: 2, Cost: 8, Class: "PRIEST", CardType: "MINION"},
			{CardID: "R15", Name: "Minion 6", Count: 2, Cost: 9, Class: "PRIEST", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	joined := strings.ToLower(strings.Join(got.SuggestedCuts, " "))
	if !strings.Contains(joined, "reactive spells") {
		t.Fatalf("expected spell-heavy shell to get reactive spell cut advice, got %+v", got.SuggestedCuts)
	}
}

func TestAnalyzerSuggestsOneAndTwoCostMinionsWhenEarlyBoardIsTooThin(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "PALADIN",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "E1", Name: "Spell 1", Count: 2, Cost: 1, Class: "PALADIN", CardType: "SPELL"},
			{CardID: "E2", Name: "Spell 2", Count: 2, Cost: 2, Class: "PALADIN", CardType: "SPELL"},
			{CardID: "E3", Name: "Spell 3", Count: 2, Cost: 3, Class: "PALADIN", CardType: "SPELL"},
			{CardID: "E4", Name: "Minion 4", Count: 2, Cost: 4, Class: "PALADIN", CardType: "MINION"},
			{CardID: "E5", Name: "Minion 5", Count: 2, Cost: 4, Class: "PALADIN", CardType: "MINION"},
			{CardID: "E6", Name: "Minion 6", Count: 2, Cost: 5, Class: "PALADIN", CardType: "MINION"},
			{CardID: "E7", Name: "Minion 7", Count: 2, Cost: 5, Class: "PALADIN", CardType: "MINION"},
			{CardID: "E8", Name: "Minion 8", Count: 2, Cost: 6, Class: "PALADIN", CardType: "MINION"},
			{CardID: "E9", Name: "Minion 9", Count: 2, Cost: 6, Class: "PALADIN", CardType: "MINION"},
			{CardID: "E10", Name: "Minion 10", Count: 2, Cost: 7, Class: "PALADIN", CardType: "MINION"},
			{CardID: "E11", Name: "Minion 11", Count: 2, Cost: 7, Class: "PALADIN", CardType: "MINION"},
			{CardID: "E12", Name: "Minion 12", Count: 2, Cost: 8, Class: "PALADIN", CardType: "MINION"},
			{CardID: "E13", Name: "Minion 13", Count: 2, Cost: 8, Class: "PALADIN", CardType: "MINION"},
			{CardID: "E14", Name: "Minion 14", Count: 2, Cost: 9, Class: "PALADIN", CardType: "MINION"},
			{CardID: "E15", Name: "Minion 15", Count: 2, Cost: 10, Class: "PALADIN", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	joined := strings.ToLower(strings.Join(got.SuggestedAdds, " "))
	if !strings.Contains(joined, "1-2") && !strings.Contains(joined, "one- to two-cost") {
		t.Fatalf("expected thin early board deck to suggest cheaper minions, got %+v", got.SuggestedAdds)
	}
}

func TestAnalyzerSuggestsTrimmingSevenPlusTopEndWhenPayoffDensityIsTooHigh(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "DRUID",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "P1", Name: "Ramp 1", Count: 2, Cost: 1, Class: "DRUID", CardType: "SPELL"},
			{CardID: "P2", Name: "Ramp 2", Count: 2, Cost: 2, Class: "DRUID", CardType: "SPELL"},
			{CardID: "P3", Name: "Setup", Count: 2, Cost: 3, Class: "DRUID", CardType: "SPELL"},
			{CardID: "P4", Name: "Payoff 1", Count: 2, Cost: 7, Class: "DRUID", CardType: "MINION"},
			{CardID: "P5", Name: "Payoff 2", Count: 2, Cost: 7, Class: "DRUID", CardType: "MINION"},
			{CardID: "P6", Name: "Payoff 3", Count: 2, Cost: 8, Class: "DRUID", CardType: "MINION"},
			{CardID: "P7", Name: "Payoff 4", Count: 2, Cost: 8, Class: "DRUID", CardType: "MINION"},
			{CardID: "P8", Name: "Payoff 5", Count: 2, Cost: 9, Class: "DRUID", CardType: "MINION"},
			{CardID: "P9", Name: "Payoff 6", Count: 2, Cost: 9, Class: "DRUID", CardType: "MINION"},
			{CardID: "P10", Name: "Payoff 7", Count: 2, Cost: 10, Class: "DRUID", CardType: "MINION"},
			{CardID: "P11", Name: "Payoff 8", Count: 2, Cost: 10, Class: "DRUID", CardType: "MINION"},
			{CardID: "P12", Name: "Payoff 9", Count: 2, Cost: 7, Class: "DRUID", CardType: "SPELL"},
			{CardID: "P13", Name: "Payoff 10", Count: 2, Cost: 8, Class: "DRUID", CardType: "SPELL"},
			{CardID: "P14", Name: "Payoff 11", Count: 2, Cost: 9, Class: "DRUID", CardType: "SPELL"},
			{CardID: "P15", Name: "Payoff 12", Count: 2, Cost: 10, Class: "DRUID", CardType: "SPELL"},
		},
	}

	got := analyzer.Analyze(input)
	joined := strings.ToLower(strings.Join(got.SuggestedCuts, " "))
	if !strings.Contains(joined, "7+") && !strings.Contains(joined, "seven") {
		t.Fatalf("expected overloaded top end to suggest trimming 7+ cost cards, got %+v", got.SuggestedCuts)
	}
}

func TestAnalyzerSurfacesStructuralTagsForThinEarlyBoardAndHeavyTopEnd(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "DRUID",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "G1", Name: "Ramp 1", Count: 2, Cost: 1, Class: "DRUID", CardType: "SPELL"},
			{CardID: "G2", Name: "Ramp 2", Count: 2, Cost: 2, Class: "DRUID", CardType: "SPELL"},
			{CardID: "G3", Name: "Setup", Count: 2, Cost: 3, Class: "DRUID", CardType: "SPELL"},
			{CardID: "G4", Name: "Big 1", Count: 2, Cost: 7, Class: "DRUID", CardType: "MINION"},
			{CardID: "G5", Name: "Big 2", Count: 2, Cost: 7, Class: "DRUID", CardType: "MINION"},
			{CardID: "G6", Name: "Big 3", Count: 2, Cost: 8, Class: "DRUID", CardType: "MINION"},
			{CardID: "G7", Name: "Big 4", Count: 2, Cost: 8, Class: "DRUID", CardType: "MINION"},
			{CardID: "G8", Name: "Big 5", Count: 2, Cost: 9, Class: "DRUID", CardType: "MINION"},
			{CardID: "G9", Name: "Big 6", Count: 2, Cost: 9, Class: "DRUID", CardType: "MINION"},
			{CardID: "G10", Name: "Big 7", Count: 2, Cost: 10, Class: "DRUID", CardType: "MINION"},
			{CardID: "G11", Name: "Big 8", Count: 2, Cost: 10, Class: "DRUID", CardType: "MINION"},
			{CardID: "G12", Name: "Big 9", Count: 2, Cost: 7, Class: "DRUID", CardType: "SPELL"},
			{CardID: "G13", Name: "Big 10", Count: 2, Cost: 8, Class: "DRUID", CardType: "SPELL"},
			{CardID: "G14", Name: "Big 11", Count: 2, Cost: 9, Class: "DRUID", CardType: "SPELL"},
			{CardID: "G15", Name: "Big 12", Count: 2, Cost: 10, Class: "DRUID", CardType: "SPELL"},
		},
	}

	got := analyzer.Analyze(input)
	joined := strings.ToLower(strings.Join(got.StructuralTags, " "))
	if !strings.Contains(joined, "thin_early_board") {
		t.Fatalf("expected thin early board tag, got %+v", got.StructuralTags)
	}
	if !strings.Contains(joined, "heavy_top_end") {
		t.Fatalf("expected heavy top end tag, got %+v", got.StructuralTags)
	}
}

func TestAnalyzerSurfacesReactiveSpellSaturationTag(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "PRIEST",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "H1", Name: "Spell 1", Count: 2, Cost: 1, Class: "PRIEST", CardType: "SPELL", Text: "Destroy an enemy minion."},
			{CardID: "H2", Name: "Spell 2", Count: 2, Cost: 2, Class: "PRIEST", CardType: "SPELL", Text: "Deal 2 damage to all enemy minions."},
			{CardID: "H3", Name: "Spell 3", Count: 2, Cost: 3, Class: "PRIEST", CardType: "SPELL", Text: "Discover a spell."},
			{CardID: "H4", Name: "Spell 4", Count: 2, Cost: 4, Class: "PRIEST", CardType: "SPELL", Text: "Restore 6 Health."},
			{CardID: "H5", Name: "Spell 5", Count: 2, Cost: 5, Class: "PRIEST", CardType: "SPELL", Text: "Draw 2 cards."},
			{CardID: "H6", Name: "Spell 6", Count: 2, Cost: 6, Class: "PRIEST", CardType: "SPELL", Text: "Deal 3 damage to all minions."},
			{CardID: "H7", Name: "Spell 7", Count: 2, Cost: 7, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "H8", Name: "Spell 8", Count: 2, Cost: 8, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "H9", Name: "Spell 9", Count: 2, Cost: 4, Class: "PRIEST", CardType: "SPELL"},
			{CardID: "H10", Name: "Minion 1", Count: 2, Cost: 4, Class: "PRIEST", CardType: "MINION"},
			{CardID: "H11", Name: "Minion 2", Count: 2, Cost: 5, Class: "PRIEST", CardType: "MINION"},
			{CardID: "H12", Name: "Minion 3", Count: 2, Cost: 6, Class: "PRIEST", CardType: "MINION"},
			{CardID: "H13", Name: "Minion 4", Count: 2, Cost: 7, Class: "PRIEST", CardType: "MINION"},
			{CardID: "H14", Name: "Minion 5", Count: 2, Cost: 8, Class: "PRIEST", CardType: "MINION"},
			{CardID: "H15", Name: "Minion 6", Count: 2, Cost: 9, Class: "PRIEST", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	joined := strings.ToLower(strings.Join(got.StructuralTags, " "))
	if !strings.Contains(joined, "reactive_spell_saturation") {
		t.Fatalf("expected reactive spell saturation tag, got %+v", got.StructuralTags)
	}
}

func TestAnalyzerProvidesReadableStructuralTagExplanations(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "DRUID",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "X1", Name: "Ramp 1", Count: 2, Cost: 1, Class: "DRUID", CardType: "SPELL"},
			{CardID: "X2", Name: "Ramp 2", Count: 2, Cost: 2, Class: "DRUID", CardType: "SPELL"},
			{CardID: "X3", Name: "Setup", Count: 2, Cost: 3, Class: "DRUID", CardType: "SPELL"},
			{CardID: "X4", Name: "Big 1", Count: 2, Cost: 7, Class: "DRUID", CardType: "MINION"},
			{CardID: "X5", Name: "Big 2", Count: 2, Cost: 7, Class: "DRUID", CardType: "MINION"},
			{CardID: "X6", Name: "Big 3", Count: 2, Cost: 8, Class: "DRUID", CardType: "MINION"},
			{CardID: "X7", Name: "Big 4", Count: 2, Cost: 8, Class: "DRUID", CardType: "MINION"},
			{CardID: "X8", Name: "Big 5", Count: 2, Cost: 9, Class: "DRUID", CardType: "MINION"},
			{CardID: "X9", Name: "Big 6", Count: 2, Cost: 9, Class: "DRUID", CardType: "MINION"},
			{CardID: "X10", Name: "Big 7", Count: 2, Cost: 10, Class: "DRUID", CardType: "MINION"},
			{CardID: "X11", Name: "Big 8", Count: 2, Cost: 10, Class: "DRUID", CardType: "MINION"},
			{CardID: "X12", Name: "Big 9", Count: 2, Cost: 7, Class: "DRUID", CardType: "SPELL"},
			{CardID: "X13", Name: "Big 10", Count: 2, Cost: 8, Class: "DRUID", CardType: "SPELL"},
			{CardID: "X14", Name: "Big 11", Count: 2, Cost: 9, Class: "DRUID", CardType: "SPELL"},
			{CardID: "X15", Name: "Big 12", Count: 2, Cost: 10, Class: "DRUID", CardType: "SPELL"},
		},
	}

	got := analyzer.Analyze(input)

	if len(got.StructuralTagDetails) == 0 {
		t.Fatalf("expected structural tag details, got %+v", got)
	}

	foundThinBoard := false
	for _, item := range got.StructuralTagDetails {
		if item.Tag != "thin_early_board" {
			continue
		}
		foundThinBoard = true
		if item.Title == "" || item.Explanation == "" {
			t.Fatalf("expected structural tag detail to include readable copy, got %+v", item)
		}
		if !strings.Contains(strings.ToLower(item.Explanation), "1-2") && !strings.Contains(strings.ToLower(item.Explanation), "early") {
			t.Fatalf("expected explanation to mention early-board issue, got %+v", item)
		}
	}

	if !foundThinBoard {
		t.Fatalf("expected thin_early_board detail, got %+v", got.StructuralTagDetails)
	}
}

func TestAnalyzerUsesSlotLevelAddAndCutGuidance(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "HUNTER",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "L1", Name: "Arc Runner", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "L2", Name: "Quick Shot", Count: 2, Cost: 2, Class: "HUNTER", CardType: "SPELL", Text: "Deal 3 damage."},
			{CardID: "L3", Name: "Pressure", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "L4", Name: "Late Beast", Count: 2, Cost: 7, Class: "HUNTER", CardType: "MINION"},
			{CardID: "L5", Name: "Huge Beast", Count: 2, Cost: 8, Class: "HUNTER", CardType: "MINION"},
			{CardID: "L6", Name: "Colossus", Count: 2, Cost: 9, Class: "HUNTER", CardType: "MINION"},
			{CardID: "L7", Name: "Refill Missing 1", Count: 2, Cost: 4, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "L8", Name: "Refill Missing 2", Count: 2, Cost: 4, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "L9", Name: "Mid 1", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
			{CardID: "L10", Name: "Mid 2", Count: 2, Cost: 5, Class: "HUNTER", CardType: "MINION"},
			{CardID: "L11", Name: "Mid 3", Count: 2, Cost: 5, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "L12", Name: "Mid 4", Count: 2, Cost: 6, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "L13", Name: "Mid 5", Count: 2, Cost: 6, Class: "HUNTER", CardType: "MINION"},
			{CardID: "L14", Name: "Mid 6", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "L15", Name: "Mid 7", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	adds := strings.ToLower(strings.Join(got.SuggestedAdds, " "))
	cuts := strings.ToLower(strings.Join(got.SuggestedCuts, " "))

	if !strings.Contains(adds, "slots") && !strings.Contains(adds, "copies") {
		t.Fatalf("expected add guidance to mention concrete slot pressure, got %+v", got.SuggestedAdds)
	}

	if !strings.Contains(cuts, "late beast") && !strings.Contains(cuts, "huge beast") && !strings.Contains(cuts, "colossus") {
		t.Fatalf("expected cut guidance to mention concrete top-end cards, got %+v", got.SuggestedCuts)
	}
}

func TestAnalyzerRecognizesBroaderRemovalAndManaCheatPatterns(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "ROGUE",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "B1", Name: "Swipe Away", Count: 2, Cost: 2, Class: "ROGUE", CardType: "SPELL", Text: "Return an enemy minion to its owner's hand."},
			{CardID: "B2", Name: "Sticky Discount", Count: 2, Cost: 3, Class: "ROGUE", CardType: "SPELL", Text: "Reduce the Cost of cards in your hand by (1)."},
			{CardID: "B3", Name: "Planner", Count: 2, Cost: 2, Class: "ROGUE", CardType: "SPELL", Text: "Draw a card."},
			{CardID: "B4", Name: "Filler 1", Count: 2, Cost: 1, Class: "ROGUE", CardType: "MINION"},
			{CardID: "B5", Name: "Filler 2", Count: 2, Cost: 1, Class: "ROGUE", CardType: "MINION"},
			{CardID: "B6", Name: "Filler 3", Count: 2, Cost: 2, Class: "ROGUE", CardType: "MINION"},
			{CardID: "B7", Name: "Filler 4", Count: 2, Cost: 2, Class: "ROGUE", CardType: "MINION"},
			{CardID: "B8", Name: "Filler 5", Count: 2, Cost: 3, Class: "ROGUE", CardType: "MINION"},
			{CardID: "B9", Name: "Filler 6", Count: 2, Cost: 3, Class: "ROGUE", CardType: "MINION"},
			{CardID: "B10", Name: "Filler 7", Count: 2, Cost: 4, Class: "ROGUE", CardType: "MINION"},
			{CardID: "B11", Name: "Filler 8", Count: 2, Cost: 4, Class: "ROGUE", CardType: "MINION"},
			{CardID: "B12", Name: "Filler 9", Count: 2, Cost: 5, Class: "ROGUE", CardType: "MINION"},
			{CardID: "B13", Name: "Filler 10", Count: 2, Cost: 5, Class: "ROGUE", CardType: "MINION"},
			{CardID: "B14", Name: "Filler 11", Count: 2, Cost: 6, Class: "ROGUE", CardType: "MINION"},
			{CardID: "B15", Name: "Filler 12", Count: 2, Cost: 6, Class: "ROGUE", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)

	if got.Features.SingleRemovalCount != 2 {
		t.Fatalf("expected bounce-style removal to count as single removal, got %d", got.Features.SingleRemovalCount)
	}

	if got.Features.ManaCheatCount != 2 {
		t.Fatalf("expected cost reduction phrasing to count as mana cheat, got %d", got.Features.ManaCheatCount)
	}
}

func TestAnalyzerProducesNormalizedFunctionalRoleSummary(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "MAGE",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "FR1", Name: "Arcane Intellect", Count: 2, Cost: 3, Class: "MAGE", CardType: "SPELL", Text: "Draw 2 cards."},
			{CardID: "FR2", Name: "Discovery Orb", Count: 2, Cost: 2, Class: "MAGE", CardType: "SPELL", Text: "Discover a spell."},
			{CardID: "FR3", Name: "Blizzard", Count: 2, Cost: 6, Class: "MAGE", CardType: "SPELL", Text: "Deal 2 damage to all enemy minions."},
			{CardID: "FR4", Name: "Single Shot", Count: 2, Cost: 3, Class: "MAGE", CardType: "SPELL", Text: "Return an enemy minion to its owner's hand."},
			{CardID: "FR5", Name: "Discount Aura", Count: 2, Cost: 4, Class: "MAGE", CardType: "SPELL", Text: "Your next minion costs (2) less."},
			{CardID: "FR6", Name: "Combo Spark", Count: 2, Cost: 1, Class: "MAGE", CardType: "SPELL", Text: "If you played another card this turn, deal 3 damage."},
			{CardID: "FR7", Name: "Filler1", Count: 2, Cost: 1, Class: "MAGE", CardType: "MINION"},
			{CardID: "FR8", Name: "Filler2", Count: 2, Cost: 1, Class: "MAGE", CardType: "MINION"},
			{CardID: "FR9", Name: "Filler3", Count: 2, Cost: 2, Class: "MAGE", CardType: "MINION"},
			{CardID: "FR10", Name: "Filler4", Count: 2, Cost: 2, Class: "MAGE", CardType: "MINION"},
			{CardID: "FR11", Name: "Filler5", Count: 2, Cost: 3, Class: "MAGE", CardType: "MINION"},
			{CardID: "FR12", Name: "Filler6", Count: 2, Cost: 3, Class: "MAGE", CardType: "MINION"},
			{CardID: "FR13", Name: "Filler7", Count: 2, Cost: 4, Class: "MAGE", CardType: "MINION"},
			{CardID: "FR14", Name: "Filler8", Count: 2, Cost: 4, Class: "MAGE", CardType: "MINION"},
			{CardID: "FR15", Name: "Filler9", Count: 2, Cost: 5, Class: "MAGE", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	if len(got.FunctionalRoleSummary) == 0 {
		t.Fatalf("expected functional role summary, got %+v", got)
	}

	foundDraw := false
	foundRemoval := false
	for _, item := range got.FunctionalRoleSummary {
		if item.Role == "refill" {
			foundDraw = true
			if item.Count < 4 {
				t.Fatalf("expected refill summary to count draw+discover slots, got %+v", item)
			}
		}
		if item.Role == "reactive" {
			foundRemoval = true
			if item.Count < 4 {
				t.Fatalf("expected reactive summary to capture bounce and aoe, got %+v", item)
			}
		}
	}

	if !foundDraw || !foundRemoval {
		t.Fatalf("expected refill and reactive roles in summary, got %+v", got.FunctionalRoleSummary)
	}
}

func TestAnalyzerUsesPersistedFunctionalTagsWhenCardTextIsMissing(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "MAGE",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "TG1", Name: "Tagged Draw", Count: 2, Cost: 3, Class: "MAGE", CardType: "SPELL", FunctionalTags: []string{"draw"}},
			{CardID: "TG2", Name: "Tagged Discover", Count: 2, Cost: 2, Class: "MAGE", CardType: "SPELL", FunctionalTags: []string{"discover"}},
			{CardID: "TG3", Name: "Tagged AoE", Count: 2, Cost: 6, Class: "MAGE", CardType: "SPELL", FunctionalTags: []string{"aoe"}},
			{CardID: "TG4", Name: "Tagged Removal", Count: 2, Cost: 3, Class: "MAGE", CardType: "SPELL", FunctionalTags: []string{"single_removal"}},
			{CardID: "TG5", Name: "Filler1", Count: 2, Cost: 1, Class: "MAGE", CardType: "MINION"},
			{CardID: "TG6", Name: "Filler2", Count: 2, Cost: 1, Class: "MAGE", CardType: "MINION"},
			{CardID: "TG7", Name: "Filler3", Count: 2, Cost: 2, Class: "MAGE", CardType: "MINION"},
			{CardID: "TG8", Name: "Filler4", Count: 2, Cost: 2, Class: "MAGE", CardType: "MINION"},
			{CardID: "TG9", Name: "Filler5", Count: 2, Cost: 3, Class: "MAGE", CardType: "MINION"},
			{CardID: "TG10", Name: "Filler6", Count: 2, Cost: 3, Class: "MAGE", CardType: "MINION"},
			{CardID: "TG11", Name: "Filler7", Count: 2, Cost: 4, Class: "MAGE", CardType: "MINION"},
			{CardID: "TG12", Name: "Filler8", Count: 2, Cost: 4, Class: "MAGE", CardType: "MINION"},
			{CardID: "TG13", Name: "Filler9", Count: 2, Cost: 5, Class: "MAGE", CardType: "MINION"},
			{CardID: "TG14", Name: "Filler10", Count: 2, Cost: 5, Class: "MAGE", CardType: "MINION"},
			{CardID: "TG15", Name: "Filler11", Count: 2, Cost: 6, Class: "MAGE", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	if got.Features.DrawCount != 2 || got.Features.DiscoverCount != 2 {
		t.Fatalf("expected persisted tags to drive refill features, got %+v", got.Features)
	}
	if got.Features.AoeCount != 2 || got.Features.SingleRemovalCount != 2 {
		t.Fatalf("expected persisted tags to drive reactive features, got %+v", got.Features)
	}
}

func TestAnalyzerSurfacesPackageDetailsForUnderbuiltAndOverbuiltPackages(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "HUNTER",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "PK1", Name: "Arc Runner", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "PK2", Name: "Quick Shot", Count: 2, Cost: 2, Class: "HUNTER", CardType: "SPELL", Text: "Deal 3 damage."},
			{CardID: "PK3", Name: "Pressure", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "PK4", Name: "Late Beast", Count: 2, Cost: 7, Class: "HUNTER", CardType: "MINION"},
			{CardID: "PK5", Name: "Huge Beast", Count: 2, Cost: 8, Class: "HUNTER", CardType: "MINION"},
			{CardID: "PK6", Name: "Colossus", Count: 2, Cost: 9, Class: "HUNTER", CardType: "MINION"},
			{CardID: "PK7", Name: "Blank Utility 1", Count: 2, Cost: 4, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "PK8", Name: "Blank Utility 2", Count: 2, Cost: 4, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "PK9", Name: "Mid 1", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
			{CardID: "PK10", Name: "Mid 2", Count: 2, Cost: 5, Class: "HUNTER", CardType: "MINION"},
			{CardID: "PK11", Name: "Mid 3", Count: 2, Cost: 5, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "PK12", Name: "Mid 4", Count: 2, Cost: 6, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "PK13", Name: "Mid 5", Count: 2, Cost: 6, Class: "HUNTER", CardType: "MINION"},
			{CardID: "PK14", Name: "Mid 6", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "PK15", Name: "Mid 7", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	if len(got.PackageDetails) == 0 {
		t.Fatalf("expected package details, got %+v", got)
	}

	var refill analysis.PackageDetail
	var payoff analysis.PackageDetail
	for _, item := range got.PackageDetails {
		switch item.Package {
		case "refill_package":
			refill = item
		case "late_payoff_package":
			payoff = item
		}
	}

	if refill.Package == "" || refill.Status != "underbuilt" {
		t.Fatalf("expected refill package to be underbuilt, got %+v", refill)
	}

	if payoff.Package == "" || payoff.Status != "overbuilt" {
		t.Fatalf("expected late payoff package to be overbuilt, got %+v", payoff)
	}

	if payoff.Slots < 6 {
		t.Fatalf("expected late payoff package to count concrete slots, got %+v", payoff)
	}
}

func TestAnalyzerAddsHearthstoneSpecificSubPackages(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "ROGUE",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "SP1", Name: "Opener", Count: 2, Cost: 1, Class: "ROGUE", CardType: "MINION"},
			{CardID: "SP2", Name: "Bridge", Count: 2, Cost: 2, Class: "ROGUE", CardType: "MINION"},
			{CardID: "SP3", Name: "Draw", Count: 2, Cost: 2, Class: "ROGUE", CardType: "SPELL", Text: "Draw 2 cards."},
			{CardID: "SP4", Name: "Discover", Count: 2, Cost: 2, Class: "ROGUE", CardType: "SPELL", Text: "Discover a card."},
			{CardID: "SP5", Name: "Removal", Count: 2, Cost: 3, Class: "ROGUE", CardType: "SPELL", Text: "Destroy an enemy minion."},
			{CardID: "SP6", Name: "Clear", Count: 2, Cost: 5, Class: "ROGUE", CardType: "SPELL", Text: "Deal 2 damage to all enemy minions."},
			{CardID: "SP7", Name: "Combo", Count: 2, Cost: 1, Class: "ROGUE", CardType: "SPELL", Metadata: cards.CardMetadata{Mechanics: []string{"COMBO"}}},
			{CardID: "SP8", Name: "Discount", Count: 2, Cost: 4, Class: "ROGUE", CardType: "SPELL", Text: "Your next minion costs (2) less."},
			{CardID: "SP9", Name: "Filler1", Count: 2, Cost: 3, Class: "ROGUE", CardType: "MINION"},
			{CardID: "SP10", Name: "Filler2", Count: 2, Cost: 3, Class: "ROGUE", CardType: "MINION"},
			{CardID: "SP11", Name: "Filler3", Count: 2, Cost: 4, Class: "ROGUE", CardType: "MINION"},
			{CardID: "SP12", Name: "Filler4", Count: 2, Cost: 4, Class: "ROGUE", CardType: "MINION"},
			{CardID: "SP13", Name: "Filler5", Count: 2, Cost: 5, Class: "ROGUE", CardType: "MINION"},
			{CardID: "SP14", Name: "Filler6", Count: 2, Cost: 6, Class: "ROGUE", CardType: "MINION"},
			{CardID: "SP15", Name: "Filler7", Count: 2, Cost: 7, Class: "ROGUE", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	foundSubpackage := false
	for _, item := range got.PackageDetails {
		if item.Parent == "reactive_package" && (item.Package == "spot_removal_suite" || item.Package == "board_clear_suite") {
			foundSubpackage = true
			break
		}
	}

	if !foundSubpackage {
		t.Fatalf("expected hearthstone-specific reactive subpackages, got %+v", got.PackageDetails)
	}
}

func TestAnalyzerSurfacesPackageConflictForAggroTopEndTension(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "HUNTER",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "CF1", Name: "One", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "CF2", Name: "Two", Count: 2, Cost: 2, Class: "HUNTER", CardType: "MINION"},
			{CardID: "CF3", Name: "Three", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "CF4", Name: "Burn 1", Count: 2, Cost: 1, Class: "HUNTER", CardType: "SPELL", Text: "Deal 2 damage."},
			{CardID: "CF5", Name: "Burn 2", Count: 2, Cost: 2, Class: "HUNTER", CardType: "SPELL", Text: "Deal 3 damage."},
			{CardID: "CF6", Name: "Refill", Count: 2, Cost: 3, Class: "HUNTER", CardType: "SPELL", Text: "Draw 2 cards."},
			{CardID: "CF7", Name: "Big 1", Count: 2, Cost: 7, Class: "HUNTER", CardType: "MINION"},
			{CardID: "CF8", Name: "Big 2", Count: 2, Cost: 8, Class: "HUNTER", CardType: "MINION"},
			{CardID: "CF9", Name: "Big 3", Count: 2, Cost: 9, Class: "HUNTER", CardType: "MINION"},
			{CardID: "CF10", Name: "Big 4", Count: 2, Cost: 7, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "CF11", Name: "Big 5", Count: 2, Cost: 8, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "CF12", Name: "Mid 1", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
			{CardID: "CF13", Name: "Mid 2", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
			{CardID: "CF14", Name: "Mid 3", Count: 2, Cost: 5, Class: "HUNTER", CardType: "MINION"},
			{CardID: "CF15", Name: "Mid 4", Count: 2, Cost: 6, Class: "HUNTER", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	foundConflict := false
	for _, item := range got.PackageDetails {
		if item.Status == "conflict" && strings.Contains(strings.ToLower(item.Explanation), "aggro") {
			foundConflict = true
			break
		}
	}

	if !foundConflict {
		t.Fatalf("expected package conflict describing aggro/top-end tension, got %+v", got.PackageDetails)
	}
}

func TestAnalyzerSynthesizesSuggestionsFromPackageDetails(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "HUNTER",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "SG1", Name: "Arc Runner", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "SG2", Name: "Quick Shot", Count: 2, Cost: 2, Class: "HUNTER", CardType: "SPELL", Text: "Deal 3 damage."},
			{CardID: "SG3", Name: "Pressure", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "SG4", Name: "Late Beast", Count: 2, Cost: 7, Class: "HUNTER", CardType: "MINION"},
			{CardID: "SG5", Name: "Huge Beast", Count: 2, Cost: 8, Class: "HUNTER", CardType: "MINION"},
			{CardID: "SG6", Name: "Colossus", Count: 2, Cost: 9, Class: "HUNTER", CardType: "MINION"},
			{CardID: "SG7", Name: "Blank Utility 1", Count: 2, Cost: 4, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "SG8", Name: "Blank Utility 2", Count: 2, Cost: 4, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "SG9", Name: "Mid 1", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
			{CardID: "SG10", Name: "Mid 2", Count: 2, Cost: 5, Class: "HUNTER", CardType: "MINION"},
			{CardID: "SG11", Name: "Mid 3", Count: 2, Cost: 5, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "SG12", Name: "Mid 4", Count: 2, Cost: 6, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "SG13", Name: "Mid 5", Count: 2, Cost: 6, Class: "HUNTER", CardType: "MINION"},
			{CardID: "SG14", Name: "Mid 6", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "SG15", Name: "Mid 7", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	adds := strings.ToLower(strings.Join(got.SuggestedAdds, " "))
	cuts := strings.ToLower(strings.Join(got.SuggestedCuts, " "))

	if !strings.Contains(adds, "refill package") {
		t.Fatalf("expected adds to mention underbuilt refill package, got %+v", got.SuggestedAdds)
	}

	if !strings.Contains(cuts, "late payoff package") {
		t.Fatalf("expected cuts to mention overbuilt late payoff package, got %+v", got.SuggestedCuts)
	}
}

func TestAnalyzerSynthesizesConflictCutGuidanceFromPackageTension(t *testing.T) {
	t.Parallel()

	analyzer := analysis.NewAnalyzer()
	input := decks.ParseResult{
		Class:      "HUNTER",
		Format:     2,
		TotalCount: 30,
		Legality:   decks.Legality{Valid: true},
		Cards: []decks.DeckCard{
			{CardID: "TC1", Name: "One", Count: 2, Cost: 1, Class: "HUNTER", CardType: "MINION"},
			{CardID: "TC2", Name: "Two", Count: 2, Cost: 2, Class: "HUNTER", CardType: "MINION"},
			{CardID: "TC3", Name: "Three", Count: 2, Cost: 3, Class: "HUNTER", CardType: "MINION"},
			{CardID: "TC4", Name: "Burn 1", Count: 2, Cost: 1, Class: "HUNTER", CardType: "SPELL", Text: "Deal 2 damage."},
			{CardID: "TC5", Name: "Burn 2", Count: 2, Cost: 2, Class: "HUNTER", CardType: "SPELL", Text: "Deal 3 damage."},
			{CardID: "TC6", Name: "Refill", Count: 2, Cost: 3, Class: "HUNTER", CardType: "SPELL", Text: "Draw 2 cards."},
			{CardID: "TC7", Name: "Big 1", Count: 2, Cost: 7, Class: "HUNTER", CardType: "MINION"},
			{CardID: "TC8", Name: "Big 2", Count: 2, Cost: 8, Class: "HUNTER", CardType: "MINION"},
			{CardID: "TC9", Name: "Big 3", Count: 2, Cost: 9, Class: "HUNTER", CardType: "MINION"},
			{CardID: "TC10", Name: "Big 4", Count: 2, Cost: 7, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "TC11", Name: "Big 5", Count: 2, Cost: 8, Class: "HUNTER", CardType: "SPELL"},
			{CardID: "TC12", Name: "Mid 1", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
			{CardID: "TC13", Name: "Mid 2", Count: 2, Cost: 4, Class: "HUNTER", CardType: "MINION"},
			{CardID: "TC14", Name: "Mid 3", Count: 2, Cost: 5, Class: "HUNTER", CardType: "MINION"},
			{CardID: "TC15", Name: "Mid 4", Count: 2, Cost: 6, Class: "HUNTER", CardType: "MINION"},
		},
	}

	got := analyzer.Analyze(input)
	cuts := strings.ToLower(strings.Join(got.SuggestedCuts, " "))
	if !strings.Contains(cuts, "pressure") && !strings.Contains(cuts, "tension") {
		t.Fatalf("expected cuts to mention package tension, got %+v", got.SuggestedCuts)
	}
}
