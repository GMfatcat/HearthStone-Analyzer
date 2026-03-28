package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"hearthstone-analyzer/internal/cards"
)

type CardsListFilter struct {
	Class string
	Set   string
	Cost  *int
}

type CardsRepository struct {
	db *sql.DB
}

func NewCardsRepository(db *sql.DB) *CardsRepository {
	return &CardsRepository{db: db}
}

func (r *CardsRepository) UpsertMany(ctx context.Context, cardsIn []cards.Card) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cards upsert tx: %w", err)
	}

	for _, card := range cardsIn {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO cards (
    id, dbf_id, class, card_type, set_name, rarity, cost, attack, health, text,
    metadata, functional_tags, collectible, standard_legal, wild_legal, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
    dbf_id = excluded.dbf_id,
    class = excluded.class,
    card_type = excluded.card_type,
    set_name = excluded.set_name,
    rarity = excluded.rarity,
    cost = excluded.cost,
    attack = excluded.attack,
    health = excluded.health,
    text = excluded.text,
    metadata = excluded.metadata,
    functional_tags = excluded.functional_tags,
    collectible = excluded.collectible,
    standard_legal = excluded.standard_legal,
    wild_legal = excluded.wild_legal,
    updated_at = CURRENT_TIMESTAMP;
`, card.ID, card.DBFID, card.Class, card.CardType, card.Set, card.Rarity, card.Cost, card.Attack, card.Health, card.Text, marshalCardMetadata(card.Metadata), marshalFunctionalTags(card.FunctionalTags), card.Collectible, card.StandardLegal, card.WildLegal); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("upsert card %q: %w", card.ID, err)
		}

		for _, locale := range card.Locales {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO card_locales (card_id, locale, name, text)
VALUES (?, ?, ?, ?)
ON CONFLICT(card_id, locale) DO UPDATE SET
    name = excluded.name,
    text = excluded.text;
`, card.ID, locale.Locale, locale.Name, locale.Text); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("upsert card locale %q/%q: %w", card.ID, locale.Locale, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cards upsert tx: %w", err)
	}

	return nil
}

func (r *CardsRepository) List(ctx context.Context, filter CardsListFilter) ([]cards.Card, error) {
	query := `
SELECT id, dbf_id, class, card_type, set_name, rarity, cost, attack, health, text,
       metadata, functional_tags, collectible, standard_legal, wild_legal
FROM cards
`
	args := make([]any, 0, 3)
	where := make([]string, 0, 3)
	if filter.Class != "" {
		where = append(where, "class = ?")
		args = append(args, filter.Class)
	}
	if filter.Set != "" {
		where = append(where, "set_name = ?")
		args = append(args, filter.Set)
	}
	if filter.Cost != nil {
		where = append(where, "cost = ?")
		args = append(args, *filter.Cost)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY cost ASC, id ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list cards: %w", err)
	}
	defer rows.Close()

	var out []cards.Card
	for rows.Next() {
		card, err := scanCardRow(rows)
		if err != nil {
			return nil, err
		}

		card.Locales, err = r.loadLocales(ctx, card.ID)
		if err != nil {
			return nil, err
		}

		out = append(out, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cards: %w", err)
	}

	return out, nil
}

func (r *CardsRepository) GetByID(ctx context.Context, id string) (cards.Card, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, dbf_id, class, card_type, set_name, rarity, cost, attack, health, text,
       metadata, functional_tags, collectible, standard_legal, wild_legal
FROM cards
WHERE id = ?
`, id)

	card, err := scanCardRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cards.Card{}, fmt.Errorf("card not found")
		}
		return cards.Card{}, err
	}

	card.Locales, err = r.loadLocales(ctx, card.ID)
	if err != nil {
		return cards.Card{}, err
	}

	return card, nil
}

func (r *CardsRepository) GetByLocalizedNames(ctx context.Context, names []string) (map[string]cards.Card, error) {
	if len(names) == 0 {
		return map[string]cards.Card{}, nil
	}

	placeholders := make([]string, 0, len(names))
	args := make([]any, 0, len(names))
	lookup := make(map[string][]string, len(names))
	orderedNames := make([]string, 0, len(names))
	seenOriginals := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, ok := seenOriginals[name]; !ok {
			orderedNames = append(orderedNames, name)
			seenOriginals[name] = struct{}{}
		}
		for _, normalized := range cards.LookupNameVariants(name) {
			key := strings.ToLower(strings.TrimSpace(normalized))
			if key == "" {
				continue
			}
			if _, ok := lookup[key]; !ok {
				placeholders = append(placeholders, "?")
				args = append(args, key)
			}
			lookup[key] = append(lookup[key], name)
		}
	}
	if len(placeholders) == 0 {
		return map[string]cards.Card{}, nil
	}

	query := `
SELECT c.id, c.dbf_id, c.class, c.card_type, c.set_name, c.rarity, c.cost, c.attack, c.health, c.text,
       c.metadata, c.functional_tags, c.collectible, c.standard_legal, c.wild_legal, l.locale, l.name
FROM cards c
JOIN card_locales l ON l.card_id = c.id
WHERE lower(l.name) IN (` + strings.Join(placeholders, ",") + `)
ORDER BY c.id ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("lookup cards by localized names: %w", err)
	}
	defer rows.Close()

	out := make(map[string]cards.Card, len(orderedNames))
	for rows.Next() {
		card, localizedName, err := scanLocalizedCardRow(rows)
		if err != nil {
			return nil, err
		}

		for _, original := range lookup[strings.ToLower(strings.TrimSpace(localizedName))] {
			if _, ok := out[original]; ok {
				continue
			}
			out[original] = card
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate localized card lookup: %w", err)
	}

	if len(out) == len(orderedNames) {
		return out, nil
	}

	fallbackCards, err := r.loadAllLocalizedCards(ctx)
	if err != nil {
		return nil, err
	}

	normalizedIndex := make(map[string]cards.Card, len(fallbackCards))
	for _, item := range fallbackCards {
		for _, key := range cards.LookupNameKeys(item.localizedName) {
			if _, ok := normalizedIndex[key]; ok {
				continue
			}
			normalizedIndex[key] = item.card
		}
	}

	for _, original := range orderedNames {
		if _, ok := out[original]; ok {
			continue
		}
		for _, key := range cards.LookupNameKeys(original) {
			card, ok := normalizedIndex[key]
			if !ok {
				continue
			}
			out[original] = card
			break
		}
	}

	return out, nil
}

func (r *CardsRepository) RecordSyncRun(ctx context.Context, run cards.SyncRun) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO card_sync_runs (source, fetched_at, card_count, raw_payload)
VALUES (?, ?, ?, ?)
`, run.Source, run.FetchedAt, run.CardCount, run.RawPayload)
	if err != nil {
		return fmt.Errorf("record card sync run: %w", err)
	}

	return nil
}

type cardScanner interface {
	Scan(dest ...any) error
}

type localizedCardLookupRow struct {
	card          cards.Card
	localizedName string
}

func scanCardRow(scanner cardScanner) (cards.Card, error) {
	var card cards.Card
	var rawMetadata string
	var rawFunctionalTags string
	err := scanner.Scan(
		&card.ID,
		&card.DBFID,
		&card.Class,
		&card.CardType,
		&card.Set,
		&card.Rarity,
		&card.Cost,
		&card.Attack,
		&card.Health,
		&card.Text,
		&rawMetadata,
		&rawFunctionalTags,
		&card.Collectible,
		&card.StandardLegal,
		&card.WildLegal,
	)
	if err != nil {
		return cards.Card{}, fmt.Errorf("scan card row: %w", err)
	}
	card.Metadata = unmarshalCardMetadata(rawMetadata)
	card.FunctionalTags = unmarshalFunctionalTags(rawFunctionalTags)

	return card, nil
}

func scanLocalizedCardRow(scanner cardScanner) (cards.Card, string, error) {
	var card cards.Card
	var rawMetadata string
	var rawFunctionalTags string
	var locale string
	var localizedName string
	err := scanner.Scan(
		&card.ID,
		&card.DBFID,
		&card.Class,
		&card.CardType,
		&card.Set,
		&card.Rarity,
		&card.Cost,
		&card.Attack,
		&card.Health,
		&card.Text,
		&rawMetadata,
		&rawFunctionalTags,
		&card.Collectible,
		&card.StandardLegal,
		&card.WildLegal,
		&locale,
		&localizedName,
	)
	if err != nil {
		return cards.Card{}, "", fmt.Errorf("scan localized card row: %w", err)
	}

	card.Metadata = unmarshalCardMetadata(rawMetadata)
	card.FunctionalTags = unmarshalFunctionalTags(rawFunctionalTags)
	card.Locales = []cards.LocaleText{{Locale: locale, Name: localizedName, Text: card.Text}}
	return card, localizedName, nil
}

func marshalFunctionalTags(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func marshalCardMetadata(metadata cards.CardMetadata) string {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return "{}"
	}
	if string(raw) == "null" {
		return "{}"
	}
	return string(raw)
}

func unmarshalFunctionalTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}

func unmarshalCardMetadata(raw string) cards.CardMetadata {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cards.CardMetadata{}
	}
	var metadata cards.CardMetadata
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return cards.CardMetadata{}
	}
	return metadata
}

func (r *CardsRepository) loadAllLocalizedCards(ctx context.Context) ([]localizedCardLookupRow, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT c.id, c.dbf_id, c.class, c.card_type, c.set_name, c.rarity, c.cost, c.attack, c.health, c.text,
       c.metadata, c.functional_tags, c.collectible, c.standard_legal, c.wild_legal, l.locale, l.name
FROM cards c
JOIN card_locales l ON l.card_id = c.id
ORDER BY c.id ASC, l.locale ASC
`)
	if err != nil {
		return nil, fmt.Errorf("load all localized cards: %w", err)
	}
	defer rows.Close()

	out := make([]localizedCardLookupRow, 0)
	for rows.Next() {
		card, localizedName, err := scanLocalizedCardRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, localizedCardLookupRow{
			card:          card,
			localizedName: localizedName,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate all localized cards: %w", err)
	}

	return out, nil
}

func (r *CardsRepository) loadLocales(ctx context.Context, cardID string) ([]cards.LocaleText, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT locale, name, text
FROM card_locales
WHERE card_id = ?
ORDER BY locale ASC
`, cardID)
	if err != nil {
		return nil, fmt.Errorf("load locales for %q: %w", cardID, err)
	}
	defer rows.Close()

	var locales []cards.LocaleText
	for rows.Next() {
		var locale cards.LocaleText
		if err := rows.Scan(&locale.Locale, &locale.Name, &locale.Text); err != nil {
			return nil, fmt.Errorf("scan locale row: %w", err)
		}
		locales = append(locales, locale)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate locales for %q: %w", cardID, err)
	}

	return locales, nil
}
