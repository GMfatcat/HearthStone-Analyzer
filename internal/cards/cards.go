package cards

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Card struct {
	ID             string
	DBFID          int
	Class          string
	CardType       string
	Set            string
	Rarity         string
	Cost           int
	Attack         *int
	Health         *int
	Text           string
	Metadata       CardMetadata
	FunctionalTags []string
	Collectible    bool
	StandardLegal  bool
	WildLegal      bool
	Locales        []LocaleText
}

type CardMetadata struct {
	Mechanics      []string `json:"mechanics,omitempty"`
	ReferencedTags []string `json:"referenced_tags,omitempty"`
	Tribes         []string `json:"tribes,omitempty"`
	SpellSchool    string   `json:"spell_school,omitempty"`
}

type LocaleText struct {
	Locale string
	Name   string
	Text   string
}

type Source interface {
	FetchCards(ctx context.Context) (FetchResult, error)
}

type SyncSummary struct {
	CardsUpserted int
}

type FetchResult struct {
	Source     string
	FetchedAt  time.Time
	RawPayload string
	Cards      []Card
}

type SyncService struct {
	repo   Repository
	source Source
}

type ListFilter struct {
	Class string
	Set   string
	Cost  *int
}

type Summary struct {
	ID             string
	Class          string
	CardType       string
	Cost           int
	Name           string
	Text           string
	Metadata       CardMetadata
	FunctionalTags []string
}

type Repository interface {
	UpsertMany(ctx context.Context, cards []Card) error
	RecordSyncRun(ctx context.Context, run SyncRun) error
}

type SyncRun struct {
	Source     string
	FetchedAt  time.Time
	CardCount  int
	RawPayload string
}

type HearthstoneJSONSource struct {
	url    string
	locale string
	client *http.Client
}

type hearthstoneJSONCard struct {
	ID             string   `json:"id"`
	DBFID          int      `json:"dbfId"`
	Name           string   `json:"name"`
	CardClass      string   `json:"cardClass"`
	Type           string   `json:"type"`
	Set            string   `json:"set"`
	Rarity         string   `json:"rarity"`
	Cost           int      `json:"cost"`
	Attack         *int     `json:"attack"`
	Health         *int     `json:"health"`
	Text           string   `json:"text"`
	Mechanics      []string `json:"mechanics"`
	ReferencedTags []string `json:"referencedTags"`
	Race           string   `json:"race"`
	SpellSchool    string   `json:"spellSchool"`
	Collectible    bool     `json:"collectible"`
}

func NewSyncService(repo Repository, source Source) *SyncService {
	return &SyncService{
		repo:   repo,
		source: source,
	}
}

func NewHearthstoneJSONSource(url, locale string, client *http.Client) *HearthstoneJSONSource {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	return &HearthstoneJSONSource{
		url:    url,
		locale: locale,
		client: client,
	}
}

func (s *SyncService) Sync(ctx context.Context) (SyncSummary, error) {
	result, err := s.source.FetchCards(ctx)
	if err != nil {
		return SyncSummary{}, fmt.Errorf("fetch cards: %w", err)
	}

	for i := range result.Cards {
		if len(result.Cards[i].FunctionalTags) == 0 {
			result.Cards[i].FunctionalTags = InferFunctionalTags(result.Cards[i].Text, result.Cards[i].Metadata)
		}
	}

	if err := s.repo.UpsertMany(ctx, result.Cards); err != nil {
		return SyncSummary{}, fmt.Errorf("persist cards: %w", err)
	}

	fetchedAt := result.FetchedAt
	if fetchedAt.IsZero() {
		fetchedAt = time.Now().UTC()
	}

	if err := s.repo.RecordSyncRun(ctx, SyncRun{
		Source:     result.Source,
		FetchedAt:  fetchedAt,
		CardCount:  len(result.Cards),
		RawPayload: result.RawPayload,
	}); err != nil {
		return SyncSummary{}, fmt.Errorf("record cards sync run: %w", err)
	}

	return SyncSummary{CardsUpserted: len(result.Cards)}, nil
}

func (s *HearthstoneJSONSource) FetchCards(ctx context.Context) (FetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return FetchResult{}, fmt.Errorf("build request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return FetchResult{}, fmt.Errorf("fetch HearthstoneJSON cards: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return FetchResult{}, fmt.Errorf("fetch HearthstoneJSON cards: unexpected status %d", resp.StatusCode)
	}

	var payload []hearthstoneJSONCard
	rawPayload, err := io.ReadAll(resp.Body)
	if err != nil {
		return FetchResult{}, fmt.Errorf("read HearthstoneJSON cards body: %w", err)
	}

	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return FetchResult{}, fmt.Errorf("decode HearthstoneJSON cards: %w", err)
	}

	cards := make([]Card, 0, len(payload))
	for _, item := range payload {
		metadata := normalizeCardMetadata(CardMetadata{
			Mechanics:      item.Mechanics,
			ReferencedTags: item.ReferencedTags,
			Tribes:         collectCardTribes(item.Race),
			SpellSchool:    item.SpellSchool,
		})
		cards = append(cards, Card{
			ID:             item.ID,
			DBFID:          item.DBFID,
			Class:          item.CardClass,
			CardType:       item.Type,
			Set:            item.Set,
			Rarity:         item.Rarity,
			Cost:           item.Cost,
			Attack:         item.Attack,
			Health:         item.Health,
			Text:           item.Text,
			Metadata:       metadata,
			FunctionalTags: InferFunctionalTags(item.Text, metadata),
			Collectible:    item.Collectible,
			StandardLegal:  true,
			WildLegal:      true,
			Locales: []LocaleText{
				{
					Locale: s.locale,
					Name:   item.Name,
					Text:   item.Text,
				},
			},
		})
	}

	return FetchResult{
		Source:     "hearthstonejson",
		FetchedAt:  time.Now().UTC(),
		RawPayload: string(rawPayload),
		Cards:      cards,
	}, nil
}

func InferFunctionalTags(text string, metadata CardMetadata) []string {
	normalized := normalizeFunctionalText(text)

	out := make([]string, 0, 8)
	appendIf := func(tag string, ok bool) {
		if !ok {
			return
		}
		for _, existing := range out {
			if existing == tag {
				return
			}
		}
		out = append(out, tag)
	}

	metadataSignals := functionalSignalsFromMetadata(metadata)
	appendIf("discover", metadataSignals["discover"])
	appendIf("taunt", metadataSignals["taunt"])
	appendIf("deathrattle", metadataSignals["deathrattle"])
	appendIf("battlecry", metadataSignals["battlecry"])
	appendIf("combo", metadataSignals["combo"])

	if normalized == "" {
		if len(out) == 0 {
			return nil
		}
		return out
	}

	appendIf("draw", containsFunctionalPattern(normalized, "draw", "add a copy", "get a copy"))
	appendIf("discover", containsFunctionalPattern(normalized, "discover"))
	appendIf("heal", containsFunctionalPattern(normalized, "restore", "heal"))
	appendIf("aoe", containsFunctionalPattern(normalized, "all enemy", "all minions", "all characters", "enemy board"))
	appendIf("single_removal", containsSingleRemovalPattern(normalized))
	appendIf("burn", containsFunctionalPattern(normalized, "deal") && containsFunctionalPattern(normalized, "damage"))
	appendIf("taunt", containsFunctionalPattern(normalized, "taunt"))
	appendIf("deathrattle", containsFunctionalPattern(normalized, "deathrattle"))
	appendIf("battlecry", containsFunctionalPattern(normalized, "battlecry"))
	appendIf("token", containsFunctionalPattern(normalized, "summon"))
	appendIf("mana_cheat", containsManaCheatPattern(normalized))
	appendIf("combo", containsFunctionalPattern(normalized, "if you played another card this turn", "combo:", "combo "))
	return out
}

func functionalSignalsFromMetadata(metadata CardMetadata) map[string]bool {
	signals := map[string]bool{}
	for _, value := range append(append([]string{}, metadata.Mechanics...), metadata.ReferencedTags...) {
		switch strings.ToUpper(strings.TrimSpace(value)) {
		case "DISCOVER":
			signals["discover"] = true
		case "TAUNT":
			signals["taunt"] = true
		case "DEATHRATTLE":
			signals["deathrattle"] = true
		case "BATTLECRY":
			signals["battlecry"] = true
		case "COMBO":
			signals["combo"] = true
		}
	}
	return signals
}

func normalizeCardMetadata(metadata CardMetadata) CardMetadata {
	metadata.Mechanics = normalizeMetadataList(metadata.Mechanics)
	metadata.ReferencedTags = normalizeMetadataList(metadata.ReferencedTags)
	metadata.Tribes = normalizeMetadataList(metadata.Tribes)
	metadata.SpellSchool = strings.ToUpper(strings.TrimSpace(metadata.SpellSchool))
	return metadata
}

func normalizeMetadataList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		normalized := strings.ToUpper(strings.TrimSpace(item))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func collectCardTribes(race string) []string {
	race = strings.TrimSpace(race)
	if race == "" {
		return nil
	}
	return []string{race}
}

func normalizeFunctionalText(text string) string {
	replacer := strings.NewReplacer(
		".", " ",
		",", " ",
		"!", " ",
		"?", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
		"\n", " ",
		"\r", " ",
		"'", " ",
	)
	return strings.Join(strings.Fields(replacer.Replace(strings.ToLower(text))), " ")
}

func containsFunctionalPattern(text string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(text, normalizeFunctionalText(pattern)) {
			return true
		}
	}
	return false
}

func containsSingleRemovalPattern(text string) bool {
	if containsFunctionalPattern(text, "destroy an enemy minion", "destroy a minion", "destroy target minion", "return an enemy minion", "return a minion") {
		return true
	}
	return containsFunctionalPattern(text, "deal") && containsFunctionalPattern(text, "to a minion", "to enemy minion", "to an enemy minion")
}

func containsManaCheatPattern(text string) bool {
	return containsFunctionalPattern(text,
		"costs less",
		"costs",
		"reduce the cost",
		"reduce cost",
		"your next minion costs",
		"costs health instead",
		"mana cheat",
	) && containsFunctionalPattern(text, "cost", "costs", "reduce")
}
