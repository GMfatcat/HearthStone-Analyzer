package meta

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

type FetchResult struct {
	Source       string
	PatchVersion string
	Format       string
	RankBracket  *string
	Region       *string
	FetchedAt    time.Time
	RawPayload   string
}

type Summary struct {
	ID           string
	Source       string
	PatchVersion string
	Format       string
}

type Source interface {
	FetchSnapshot(ctx context.Context) (FetchResult, error)
}

type SyncService struct {
	repo          *sqliteStore.MetaSnapshotsRepository
	decksRepo     *sqliteStore.DecksRepository
	metaDecksRepo *sqliteStore.MetaDecksRepository
	cardsRepo     *sqliteStore.CardsRepository
	deckCardsRepo *sqliteStore.DeckCardsRepository
	source        Source
}

func NewSyncService(repo *sqliteStore.MetaSnapshotsRepository, source Source) *SyncService {
	return &SyncService{
		repo:   repo,
		source: source,
	}
}

func NewSyncServiceWithDeckPersistence(repo *sqliteStore.MetaSnapshotsRepository, decksRepo *sqliteStore.DecksRepository, metaDecksRepo *sqliteStore.MetaDecksRepository, cardsRepo *sqliteStore.CardsRepository, deckCardsRepo *sqliteStore.DeckCardsRepository, source Source) *SyncService {
	return &SyncService{
		repo:          repo,
		decksRepo:     decksRepo,
		metaDecksRepo: metaDecksRepo,
		cardsRepo:     cardsRepo,
		deckCardsRepo: deckCardsRepo,
		source:        source,
	}
}

func (s *SyncService) Sync(ctx context.Context) (Summary, error) {
	result, err := s.source.FetchSnapshot(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("fetch meta snapshot: %w", err)
	}

	fetchedAt := result.FetchedAt
	if fetchedAt.IsZero() {
		fetchedAt = time.Now().UTC()
	}

	id := buildSnapshotID(result.Source, result.Format, fetchedAt, result.RawPayload)
	if err := s.repo.Create(ctx, sqliteStore.MetaSnapshot{
		ID:           id,
		Source:       result.Source,
		PatchVersion: result.PatchVersion,
		Format:       result.Format,
		RankBracket:  result.RankBracket,
		Region:       result.Region,
		FetchedAt:    fetchedAt,
		RawPayload:   result.RawPayload,
	}); err != nil {
		return Summary{}, fmt.Errorf("persist meta snapshot: %w", err)
	}

	if s.decksRepo != nil && s.metaDecksRepo != nil {
		if err := s.persistMetaDeckMappings(ctx, id, result); err != nil {
			return Summary{}, fmt.Errorf("persist meta deck mappings: %w", err)
		}
	}

	return Summary{
		ID:           id,
		Source:       result.Source,
		PatchVersion: result.PatchVersion,
		Format:       result.Format,
	}, nil
}

type rawMetaDeck struct {
	ID          string   `json:"id"`
	DeckID      string   `json:"deck_id"`
	ExternalRef string   `json:"external_ref"`
	Name        string   `json:"name"`
	Class       string   `json:"class"`
	Format      string   `json:"format"`
	Archetype   string   `json:"archetype"`
	DeckCode    string   `json:"deck_code"`
	DeckHash    string   `json:"deck_hash"`
	Winrate     *float64 `json:"winrate"`
	Playrate    *float64 `json:"playrate"`
	SampleSize  *int     `json:"sample_size"`
	Tier        *string  `json:"tier"`
	Cards       []string `json:"cards"`
}

func (s *SyncService) persistMetaDeckMappings(ctx context.Context, snapshotID string, result FetchResult) error {
	decks, err := normalizeMetaDecks(result.RawPayload)
	if err != nil {
		return nil
	}

	nameLookup := map[string]string{}
	if s.cardsRepo != nil {
		names := make([]string, 0)
		for _, deck := range decks {
			for _, line := range deck.Cards {
				_, name := parseSnapshotCardLine(line)
				if name != "" {
					names = append(names, name)
				}
			}
		}
		if len(names) > 0 {
			if cardsByName, err := s.cardsRepo.GetByLocalizedNames(ctx, names); err == nil {
				for name, card := range cardsByName {
					nameLookup[name] = card.ID
				}
			}
		}
	}

	items := make([]sqliteStore.MetaDeck, 0, len(decks))
	for _, deck := range decks {
		deckID := buildMetaDeckID(result.Source, result.Format, deck)
		externalRef := firstNonEmpty(deck.ExternalRef, deck.ID, deck.DeckID)
		name := firstNonEmpty(deck.Name, externalRef)
		className := firstNonEmpty(deck.Class, "UNKNOWN")
		format := firstNonEmpty(deck.Format, result.Format, "standard")

		var extRefPtr, namePtr, archetypePtr, deckCodePtr, deckHashPtr *string
		if externalRef != "" {
			extRefPtr = &externalRef
		}
		if name != "" {
			namePtr = &name
		}
		if deck.Archetype != "" {
			archetypePtr = &deck.Archetype
		}
		if deck.DeckCode != "" {
			deckCodePtr = &deck.DeckCode
		}
		if deck.DeckHash != "" {
			deckHashPtr = &deck.DeckHash
		}

		if _, err := s.decksRepo.UpsertMetaDeck(ctx, sqliteStore.Deck{
			ID:          deckID,
			Source:      result.Source,
			ExternalRef: extRefPtr,
			Name:        namePtr,
			Class:       className,
			Format:      format,
			DeckCode:    deckCodePtr,
			Archetype:   archetypePtr,
			DeckHash:    deckHashPtr,
		}); err != nil {
			return err
		}

		if s.deckCardsRepo != nil && len(deck.Cards) > 0 {
			deckCards := make([]sqliteStore.DeckCardRecord, 0)
			for _, line := range deck.Cards {
				count, name := parseSnapshotCardLine(line)
				cardID, ok := nameLookup[name]
				if !ok || count <= 0 {
					continue
				}
				deckCards = append(deckCards, sqliteStore.DeckCardRecord{
					DeckID:    deckID,
					CardID:    cardID,
					CardCount: count,
				})
			}
			if len(deckCards) > 0 {
				if err := s.deckCardsRepo.ReplaceDeckCards(ctx, deckID, deckCards); err != nil {
					return err
				}
			}
		}

		items = append(items, sqliteStore.MetaDeck{
			SnapshotID: snapshotID,
			DeckID:     deckID,
			Winrate:    normalizeRate(deck.Winrate),
			Playrate:   normalizeRate(deck.Playrate),
			SampleSize: deck.SampleSize,
			Tier:       deck.Tier,
		})
	}

	return s.metaDecksRepo.ReplaceSnapshotDecks(ctx, snapshotID, items)
}

func normalizeMetaDecks(raw string) ([]rawMetaDeck, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}

	for _, key := range []string{"decks", "meta_decks", "archetypes"} {
		block, ok := payload[key]
		if !ok {
			continue
		}

		var rawItems []map[string]any
		if err := json.Unmarshal(block, &rawItems); err != nil {
			continue
		}

		out := make([]rawMetaDeck, 0, len(rawItems))
		for _, item := range rawItems {
			out = append(out, rawMetaDeck{
				ID:          stringValue(item, "id"),
				DeckID:      stringValue(item, "deck_id"),
				ExternalRef: stringValue(item, "external_ref", "externalRef"),
				Name:        stringValue(item, "name", "deck_name", "deckName"),
				Class:       stringValue(item, "class", "cls", "hero_class"),
				Format:      stringValue(item, "format", "game_format"),
				Archetype:   stringValue(item, "archetype", "archetype_name"),
				DeckCode:    stringValue(item, "deck_code", "deckCode", "code"),
				DeckHash:    stringValue(item, "deck_hash", "deckHash"),
				Winrate:     floatValue(item, "winrate", "wr"),
				Playrate:    floatValue(item, "playrate", "pr"),
				SampleSize:  intValue(item, "sample_size", "sampleSize", "games"),
				Tier:        stringPtrValue(item, "tier", "tier_label", "tierLabel"),
				Cards:       stringSliceValue(item, "cards"),
			})
		}

		return out, nil
	}

	return nil, nil
}

func buildSnapshotID(source string, format string, fetchedAt time.Time, rawPayload string) string {
	sum := sha1.Sum([]byte(source + "|" + format + "|" + fetchedAt.UTC().Format(time.RFC3339Nano) + "|" + rawPayload))
	return fmt.Sprintf("meta_%x", sum[:8])
}

func buildMetaDeckID(source string, format string, deck rawMetaDeck) string {
	identity := firstNonEmpty(deck.ExternalRef, deck.ID, deck.DeckID, deck.Name)
	sum := sha1.Sum([]byte(source + "|" + format + "|" + identity + "|" + deck.Class))
	return fmt.Sprintf("deck_%x", sum[:8])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

func normalizeRate(value *float64) *float64 {
	if value == nil {
		return nil
	}

	normalized := *value
	if normalized <= 1 {
		normalized *= 100
	}

	return &normalized
}

func stringValue(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			if s, ok := value.(string); ok {
				return s
			}
		}
	}
	return ""
}

func stringPtrValue(item map[string]any, keys ...string) *string {
	value := stringValue(item, keys...)
	if value == "" {
		return nil
	}
	return &value
}

func floatValue(item map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch typed := value.(type) {
			case float64:
				return &typed
			case int:
				converted := float64(typed)
				return &converted
			}
		}
	}
	return nil
}

func intValue(item map[string]any, keys ...string) *int {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch typed := value.(type) {
			case float64:
				converted := int(typed)
				return &converted
			case int:
				return &typed
			}
		}
	}
	return nil
}

func stringSliceValue(item map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok {
			continue
		}
		rawItems, ok := value.([]any)
		if !ok {
			continue
		}
		out := make([]string, 0, len(rawItems))
		for _, raw := range rawItems {
			if s, ok := raw.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func parseSnapshotCardLine(line string) (int, string) {
	parts := strings.SplitN(strings.TrimSpace(line), "x ", 2)
	if len(parts) != 2 {
		return 0, ""
	}
	count, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, ""
	}
	return count, strings.TrimSpace(parts[1])
}
