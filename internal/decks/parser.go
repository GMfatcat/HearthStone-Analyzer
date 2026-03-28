package decks

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"hearthstone-analyzer/internal/cards"
)

type CardLookup interface {
	GetByDBFIDs(ctx context.Context, dbfIDs []int) (map[int]cards.Card, error)
}

type Parser struct {
	lookup CardLookup
}

type DeckCard struct {
	CardID         string             `json:"card_id"`
	Name           string             `json:"name"`
	Text           string             `json:"text"`
	Count          int                `json:"count"`
	Cost           int                `json:"cost"`
	Class          string             `json:"class"`
	CardType       string             `json:"card_type"`
	Metadata       cards.CardMetadata `json:"metadata,omitempty"`
	FunctionalTags []string           `json:"functional_tags,omitempty"`
}

type Legality struct {
	Valid  bool     `json:"valid"`
	Issues []string `json:"issues,omitempty"`
}

type ParseResult struct {
	Class      string     `json:"class"`
	Format     int        `json:"format"`
	TotalCount int        `json:"total_count"`
	DeckHash   string     `json:"deck_hash"`
	Legality   Legality   `json:"legality"`
	Cards      []DeckCard `json:"cards"`
}

func NewParser(lookup CardLookup) *Parser {
	return &Parser{lookup: lookup}
}

func (p *Parser) Parse(ctx context.Context, deckCode string) (ParseResult, error) {
	payload, err := base64.RawURLEncoding.DecodeString(deckCode)
	if err != nil {
		return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, "deck code is not valid base64url")
	}
	if len(payload) == 0 {
		return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, "deck code payload is empty")
	}

	index := 0
	if payload[index] != 0 {
		return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, "deck code prefix is unsupported")
	}
	index++

	format, err := readVarint(payload, &index)
	if err != nil {
		return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, "deck code format segment is invalid")
	}

	heroCount, err := readVarint(payload, &index)
	if err != nil {
		return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, "deck code hero segment is invalid")
	}
	if heroCount != 1 {
		return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, fmt.Sprintf("deck code must contain exactly 1 hero, got %d", heroCount))
	}

	heroDBFID, err := readVarint(payload, &index)
	if err != nil {
		return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, "deck code hero id is invalid")
	}

	cardCounts := make(map[int]int)
	for _, defaultCount := range []int{1, 2} {
		groupCount, err := readVarint(payload, &index)
		if err != nil {
			return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, "deck code card group segment is invalid")
		}
		for i := 0; i < groupCount; i++ {
			dbfID, err := readVarint(payload, &index)
			if err != nil {
				return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, "deck code card id is invalid")
			}
			cardCounts[dbfID] = defaultCount
		}
	}

	nCount, err := readVarint(payload, &index)
	if err != nil {
		return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, "deck code variable-count segment is invalid")
	}
	for i := 0; i < nCount; i++ {
		dbfID, err := readVarint(payload, &index)
		if err != nil {
			return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, "deck code variable-count card id is invalid")
		}
		count, err := readVarint(payload, &index)
		if err != nil {
			return ParseResult{}, NewParseError(ErrCodeInvalidDeckCode, "deck code variable-count card count is invalid")
		}
		cardCounts[dbfID] = count
	}

	dbfIDs := make([]int, 0, len(cardCounts)+1)
	dbfIDs = append(dbfIDs, heroDBFID)
	for dbfID := range cardCounts {
		dbfIDs = append(dbfIDs, dbfID)
	}

	lookup, err := p.lookup.GetByDBFIDs(ctx, dbfIDs)
	if err != nil {
		return ParseResult{}, NewParseError(ErrCodeLookupFailed, "failed to resolve deck cards from storage")
	}

	hero, ok := lookup[heroDBFID]
	if !ok {
		return ParseResult{}, NewParseError(ErrCodeHeroNotFound, fmt.Sprintf("hero dbf id %d was not found", heroDBFID))
	}

	orderedIDs := make([]int, 0, len(cardCounts))
	for dbfID := range cardCounts {
		orderedIDs = append(orderedIDs, dbfID)
	}
	sort.Ints(orderedIDs)

	deckCards := make([]DeckCard, 0, len(orderedIDs))
	totalCount := 0
	for _, dbfID := range orderedIDs {
		card, ok := lookup[dbfID]
		if !ok {
			return ParseResult{}, NewParseError(ErrCodeCardNotFound, fmt.Sprintf("card dbf id %d was not found", dbfID))
		}

		count := cardCounts[dbfID]
		totalCount += count
		deckCards = append(deckCards, DeckCard{
			CardID:         card.ID,
			Name:           firstName(card),
			Text:           firstText(card),
			Count:          count,
			Cost:           card.Cost,
			Class:          card.Class,
			CardType:       card.CardType,
			Metadata:       card.Metadata,
			FunctionalTags: append([]string(nil), card.FunctionalTags...),
		})
	}

	return ParseResult{
		Class:      hero.Class,
		Format:     format,
		TotalCount: totalCount,
		DeckHash:   stableHash(format, hero.Class, deckCards),
		Legality:   evaluateLegality(hero.Class, format, cardCounts, lookup, totalCount),
		Cards:      deckCards,
	}, nil
}

func readVarint(payload []byte, index *int) (int, error) {
	value, n := binary.Uvarint(payload[*index:])
	if n <= 0 {
		return 0, fmt.Errorf("invalid varint")
	}
	*index += n
	return int(value), nil
}

func firstName(card cards.Card) string {
	if len(card.Locales) > 0 {
		return card.Locales[0].Name
	}
	return card.ID
}

func firstText(card cards.Card) string {
	if len(card.Locales) > 0 && card.Locales[0].Text != "" {
		return card.Locales[0].Text
	}
	return card.Text
}

func stableHash(format int, class string, cards []DeckCard) string {
	parts := make([]string, 0, len(cards)+2)
	parts = append(parts, fmt.Sprintf("format=%d", format))
	parts = append(parts, "class="+class)
	for _, card := range cards {
		parts = append(parts, fmt.Sprintf("%s:%d", card.CardID, card.Count))
	}

	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func evaluateLegality(heroClass string, format int, cardCounts map[int]int, lookup map[int]cards.Card, totalCount int) Legality {
	issues := make([]string, 0, 4)

	if totalCount == 30 {
		// continue evaluating other rules even for a legal-sized deck
	} else {
		issues = append(issues, fmt.Sprintf("expected 30 cards, got %d", totalCount))
	}

	for dbfID, count := range cardCounts {
		card := lookup[dbfID]

		if card.Class != "" && card.Class != "NEUTRAL" && card.Class != heroClass {
			issues = append(issues, fmt.Sprintf("class mismatch: %s cannot be used in %s deck", card.ID, heroClass))
		}

		if format == 2 && !card.StandardLegal {
			issues = append(issues, fmt.Sprintf("card %s is not legal in standard", card.ID))
		}

		if strings.EqualFold(card.Rarity, "LEGENDARY") {
			if count > 1 {
				issues = append(issues, fmt.Sprintf("legendary card %s has %d copies", card.ID, count))
			}
			continue
		}

		if count > 2 {
			issues = append(issues, fmt.Sprintf("card %s has more than 2 copies", card.ID))
		}
	}

	return Legality{
		Valid:  len(issues) == 0,
		Issues: issues,
	}
}
