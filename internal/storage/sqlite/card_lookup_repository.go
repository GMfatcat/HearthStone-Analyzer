package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"hearthstone-analyzer/internal/cards"
)

type CardLookupRepository struct {
	db *sql.DB
}

func NewCardLookupRepository(db *sql.DB) *CardLookupRepository {
	return &CardLookupRepository{db: db}
}

func (r *CardLookupRepository) GetByDBFIDs(ctx context.Context, dbfIDs []int) (map[int]cards.Card, error) {
	if len(dbfIDs) == 0 {
		return map[int]cards.Card{}, nil
	}

	placeholders := make([]string, len(dbfIDs))
	args := make([]any, 0, len(dbfIDs))
	for i, dbfID := range dbfIDs {
		placeholders[i] = "?"
		args = append(args, dbfID)
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT id, dbf_id, class, card_type, set_name, rarity, cost, attack, health, text,
       metadata, functional_tags, collectible, standard_legal, wild_legal
FROM cards
WHERE dbf_id IN (`+strings.Join(placeholders, ",")+`)
`, args...)
	if err != nil {
		return nil, fmt.Errorf("query cards by dbf ids: %w", err)
	}
	defer rows.Close()

	out := make(map[int]cards.Card, len(dbfIDs))
	for rows.Next() {
		card, err := scanCardRow(rows)
		if err != nil {
			return nil, err
		}
		card.Locales, err = r.loadLocales(ctx, card.ID)
		if err != nil {
			return nil, err
		}
		out[card.DBFID] = card
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cards by dbf ids: %w", err)
	}

	return out, nil
}

func (r *CardLookupRepository) loadLocales(ctx context.Context, cardID string) ([]cards.LocaleText, error) {
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
