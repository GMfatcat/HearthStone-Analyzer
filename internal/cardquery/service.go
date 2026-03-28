package cardquery

import (
	"context"

	"hearthstone-analyzer/internal/cards"
	"hearthstone-analyzer/internal/storage/sqlite"
)

type Service struct {
	repo *sqlite.CardsRepository
}

func NewService(repo *sqlite.CardsRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, filter cards.ListFilter) ([]cards.Summary, error) {
	items, err := s.repo.List(ctx, sqlite.CardsListFilter{
		Class: filter.Class,
		Set:   filter.Set,
		Cost:  filter.Cost,
	})
	if err != nil {
		return nil, err
	}

	out := make([]cards.Summary, 0, len(items))
	for _, item := range items {
		out = append(out, toSummary(item))
	}

	return out, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (cards.Summary, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return cards.Summary{}, err
	}

	return toSummary(item), nil
}

func toSummary(card cards.Card) cards.Summary {
	name := ""
	text := card.Text
	if len(card.Locales) > 0 {
		name = card.Locales[0].Name
		if card.Locales[0].Text != "" {
			text = card.Locales[0].Text
		}
	}

	return cards.Summary{
		ID:             card.ID,
		Class:          card.Class,
		CardType:       card.CardType,
		Cost:           card.Cost,
		Name:           name,
		Text:           text,
		Metadata:       card.Metadata,
		FunctionalTags: append([]string(nil), card.FunctionalTags...),
	}
}
