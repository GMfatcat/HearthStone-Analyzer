package compare

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	analysispkg "hearthstone-analyzer/internal/analysis"
	"hearthstone-analyzer/internal/cards"
	"hearthstone-analyzer/internal/decks"
	sqliteStore "hearthstone-analyzer/internal/storage/sqlite"
)

type Parser interface {
	Parse(ctx context.Context, deckCode string) (decks.ParseResult, error)
}

type Result struct {
	SnapshotID          string
	PatchVersion        string
	Format              string
	MergedSummary       []string
	MergedSuggestedAdds []string
	MergedSuggestedCuts []string
	MergedGuidance      StructuredGuidance
	Candidates          []Candidate
}

type StructuredGuidance struct {
	Summary []Recommendation
	Adds    []Recommendation
	Cuts    []Recommendation
}

type Recommendation struct {
	Key        string
	Kind       string
	Package    string
	Source     string
	Message    string
	Confidence float64
	Support    []RecommendationSupport
}

type RecommendationSupport struct {
	Source          string
	CandidateDeckID string
	CandidateName   string
	CandidateRank   int
	Weight          float64
	Evidence        string
}

type Candidate struct {
	DeckID           string
	Name             string
	Class            string
	Archetype        string
	Similarity       float64
	Breakdown        SimilarityBreakdown
	Summary          []string
	Winrate          *float64
	Playrate         *float64
	SampleSize       *int
	Tier             *string
	SharedCards      []CardDiff
	MissingFromInput []CardDiff
	MissingFromMeta  []CardDiff
	SuggestedAdds    []string
	SuggestedCuts    []string
	metaCards        []decks.DeckCard
}

type CardDiff struct {
	CardID     string
	Name       string
	InputCount int
	MetaCount  int
}

type SimilarityBreakdown struct {
	Total    float64
	Overlap  float64
	Curve    float64
	CardType float64
}

type Service struct {
	parser        Parser
	snapshotsRepo *sqliteStore.MetaSnapshotsRepository
	metaDecksRepo *sqliteStore.MetaDecksRepository
	cardsRepo     *sqliteStore.CardsRepository
	deckCardsRepo *sqliteStore.DeckCardsRepository
}

func NewService(parser Parser, snapshotsRepo *sqliteStore.MetaSnapshotsRepository, metaDecksRepo *sqliteStore.MetaDecksRepository, cardsRepo *sqliteStore.CardsRepository, deckCardsRepo *sqliteStore.DeckCardsRepository) *Service {
	return &Service{parser: parser, snapshotsRepo: snapshotsRepo, metaDecksRepo: metaDecksRepo, cardsRepo: cardsRepo, deckCardsRepo: deckCardsRepo}
}

func (s *Service) CompareDeck(ctx context.Context, deckCode string, limit int) (Result, error) {
	if limit <= 0 {
		limit = 5
	}

	parsed, err := s.parser.Parse(ctx, deckCode)
	if err != nil {
		return Result{}, err
	}

	format := toFormatName(parsed.Format)
	snapshot, err := s.snapshotsRepo.GetLatestByFormat(ctx, format)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Result{}, fmt.Errorf("meta snapshot not found: %w", err)
		}
		return Result{}, err
	}

	items, err := s.metaDecksRepo.ListComparableBySnapshotID(ctx, snapshot.ID)
	if err != nil {
		return Result{}, err
	}

	targetCounts := countsFromDeck(parsed.Cards)
	candidates := make([]Candidate, 0, len(items))
	for _, item := range items {
		if item.DeckCode == nil || *item.DeckCode == "" {
			continue
		}

		other, err := s.parser.Parse(ctx, *item.DeckCode)
		if err != nil {
			continue
		}

		name := item.DeckID
		if item.Name != nil && *item.Name != "" {
			name = *item.Name
		}
		archetype := ""
		if item.Archetype != nil {
			archetype = *item.Archetype
		}

		sharedCards, missingFromInput, missingFromMeta := buildCardDiffs(parsed.Cards, other.Cards)
		breakdown := computeSimilarityBreakdown(parsed.Cards, other.Cards)
		candidates = append(candidates, Candidate{
			DeckID:           item.DeckID,
			Name:             name,
			Class:            item.Class,
			Archetype:        archetype,
			Similarity:       breakdown.Total,
			Breakdown:        breakdown,
			Summary:          buildComparisonSummary(parsed.Cards, other.Cards, name),
			Winrate:          item.Winrate,
			Playrate:         item.Playrate,
			SampleSize:       item.SampleSize,
			Tier:             item.Tier,
			SharedCards:      sharedCards,
			MissingFromInput: missingFromInput,
			MissingFromMeta:  missingFromMeta,
			SuggestedAdds:    buildSuggestions("Add", missingFromInput, name),
			SuggestedCuts:    buildSuggestions("Cut", missingFromMeta, name),
			metaCards:        other.Cards,
		})
	}

	if len(candidates) == 0 && s.cardsRepo != nil && s.deckCardsRepo != nil {
		persistedCandidates, err := s.compareUsingPersistedDeckCards(ctx, snapshot, parsed.Cards, targetCounts)
		if err == nil && len(persistedCandidates) > 0 {
			candidates = append(candidates, persistedCandidates...)
		}
	}

	if len(candidates) == 0 && s.cardsRepo != nil {
		fallbackCandidates, err := s.compareUsingSnapshotCardLines(ctx, snapshot, parsed.Cards, targetCounts)
		if err == nil {
			candidates = append(candidates, fallbackCandidates...)
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return compareCandidateRank(candidates[i], candidates[j])
	})

	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	mergedSummary, mergedAdds, mergedCuts, mergedGuidance := synthesizeMergedGuidance(parsed, candidates)

	return Result{
		SnapshotID:          snapshot.ID,
		PatchVersion:        snapshot.PatchVersion,
		Format:              snapshot.Format,
		MergedSummary:       mergedSummary,
		MergedSuggestedAdds: mergedAdds,
		MergedSuggestedCuts: mergedCuts,
		MergedGuidance:      mergedGuidance,
		Candidates:          candidates,
	}, nil
}

func (s *Service) compareUsingPersistedDeckCards(ctx context.Context, snapshot sqliteStore.MetaSnapshot, inputCards []decks.DeckCard, targetCounts map[string]int) ([]Candidate, error) {
	items, err := s.metaDecksRepo.ListBySnapshotID(ctx, snapshot.ID)
	if err != nil {
		return nil, err
	}

	candidates := make([]Candidate, 0, len(items))
	for _, item := range items {
		deckCards, err := s.deckCardsRepo.ListByDeckID(ctx, item.DeckID)
		if err != nil || len(deckCards) == 0 {
			continue
		}

		metaCards := make([]decks.DeckCard, 0, len(deckCards))
		for _, deckCard := range deckCards {
			card, err := s.cardsRepo.GetByID(ctx, deckCard.CardID)
			if err != nil {
				continue
			}
			metaCards = append(metaCards, decks.DeckCard{
				CardID:   deckCard.CardID,
				Name:     localizedCardName(card),
				Count:    deckCard.CardCount,
				Cost:     card.Cost,
				Class:    card.Class,
				CardType: card.CardType,
			})
		}
		if len(metaCards) == 0 {
			continue
		}

		metaCounts := countsFromDeck(metaCards)
		sharedCards, missingFromInput, missingFromMeta := buildCardDiffsFromCounts(targetCounts, metaCounts, metaCards)
		breakdown := computeSimilarityBreakdown(inputCards, metaCards)
		candidates = append(candidates, Candidate{
			DeckID:           item.DeckID,
			Name:             item.DeckID,
			Similarity:       breakdown.Total,
			Breakdown:        breakdown,
			Summary:          buildComparisonSummary(inputCards, metaCards, item.DeckID),
			Winrate:          item.Winrate,
			Playrate:         item.Playrate,
			SampleSize:       item.SampleSize,
			Tier:             item.Tier,
			SharedCards:      sharedCards,
			MissingFromInput: missingFromInput,
			MissingFromMeta:  missingFromMeta,
			SuggestedAdds:    buildSuggestions("Add", missingFromInput, item.DeckID),
			SuggestedCuts:    buildSuggestions("Cut", missingFromMeta, item.DeckID),
			metaCards:        metaCards,
		})
	}
	return candidates, nil
}

type fallbackSnapshotPayload struct {
	Decks []fallbackSnapshotDeck `json:"decks"`
}

type fallbackSnapshotDeck struct {
	ExternalRef string   `json:"external_ref"`
	Name        string   `json:"name"`
	Class       string   `json:"class"`
	Format      string   `json:"format"`
	Cards       []string `json:"cards"`
}

func (s *Service) compareUsingSnapshotCardLines(ctx context.Context, snapshot sqliteStore.MetaSnapshot, inputCards []decks.DeckCard, targetCounts map[string]int) ([]Candidate, error) {
	var payload fallbackSnapshotPayload
	if err := json.Unmarshal([]byte(snapshot.RawPayload), &payload); err != nil {
		return nil, err
	}

	allNames := make([]string, 0)
	for _, deck := range payload.Decks {
		for _, line := range deck.Cards {
			_, name := parseSnapshotCardLine(line)
			if name != "" {
				allNames = append(allNames, name)
			}
		}
	}
	nameLookup, err := s.cardsRepo.GetByLocalizedNames(ctx, allNames)
	if err != nil {
		return nil, err
	}

	candidates := make([]Candidate, 0, len(payload.Decks))
	for _, deck := range payload.Decks {
		metaCards := make([]decks.DeckCard, 0, len(deck.Cards))
		for _, line := range deck.Cards {
			count, name := parseSnapshotCardLine(line)
			card, ok := nameLookup[name]
			if !ok {
				continue
			}
			metaCards = append(metaCards, decks.DeckCard{
				CardID:   card.ID,
				Name:     localizedCardName(card),
				Count:    count,
				Cost:     card.Cost,
				Class:    card.Class,
				CardType: card.CardType,
			})
		}
		if len(metaCards) == 0 {
			continue
		}

		metaCounts := countsFromDeck(metaCards)
		sharedCards, missingFromInput, missingFromMeta := buildCardDiffsFromCounts(targetCounts, metaCounts, metaCards)
		breakdown := computeSimilarityBreakdown(inputCards, metaCards)
		candidates = append(candidates, Candidate{
			DeckID:           deck.ExternalRef,
			Name:             deck.Name,
			Class:            deck.Class,
			Similarity:       breakdown.Total,
			Breakdown:        breakdown,
			Summary:          buildComparisonSummary(inputCards, metaCards, deck.Name),
			SharedCards:      sharedCards,
			MissingFromInput: missingFromInput,
			MissingFromMeta:  missingFromMeta,
			SuggestedAdds:    buildSuggestions("Add", missingFromInput, deck.Name),
			SuggestedCuts:    buildSuggestions("Cut", missingFromMeta, deck.Name),
			metaCards:        metaCards,
		})
	}
	return candidates, nil
}

func countsFromDeck(cards []decks.DeckCard) map[string]int {
	out := make(map[string]int, len(cards))
	for _, card := range cards {
		out[card.CardID] += card.Count
	}
	return out
}

func synthesizeMergedGuidance(parsed decks.ParseResult, candidates []Candidate) ([]string, []string, []string, StructuredGuidance) {
	if len(candidates) == 0 {
		return nil, nil, nil, StructuredGuidance{}
	}

	analyzer := analysispkg.NewAnalyzer()
	inputAnalysis := analyzer.Analyze(parsed)
	inputPackages := indexAnalysisPackages(inputAnalysis.PackageDetails)
	considered := 0
	guidance := StructuredGuidance{
		Summary: make([]Recommendation, 0, 3),
		Adds:    make([]Recommendation, 0, 3),
		Cuts:    make([]Recommendation, 0, 3),
	}
	signals := make(map[string]*packageSupport)
	hasSplitSignal := false
	hasConsensus := false

	for idx, candidate := range candidates {
		if len(candidate.metaCards) == 0 || considered == 3 {
			continue
		}
		considered++
		rank := idx + 1
		weight := candidateAgreementWeight(rank)
		metaAnalysis := analyzer.Analyze(buildCompareParseResult(parsed.Class, parsed.Format, candidate.metaCards))
		metaPackages := indexAnalysisPackages(metaAnalysis.PackageDetails)

		for _, pkg := range []string{"refill_package", "early_board_package", "reactive_package", "late_payoff_package"} {
			inputDetail, ok := inputPackages[pkg]
			if !ok {
				continue
			}
			metaDetail, hasMeta := metaPackages[pkg]
			if !hasMeta {
				continue
			}

			addSignal := inputDetail.Status == "underbuilt" && metaDetail.Slots >= inputDetail.TargetMin
			cutSignal := inputDetail.Status == "overbuilt" && metaDetail.Slots < inputDetail.Slots
			if addSignal && cutSignal {
				continue
			}
			signal := ensurePackageSupport(signals, pkg)
			if addSignal {
				signal.addWeight += weight
				signal.addSupport = append(signal.addSupport, RecommendationSupport{
					Source:          "meta_candidate",
					CandidateDeckID: candidate.DeckID,
					CandidateName:   candidate.Name,
					CandidateRank:   rank,
					Weight:          weight,
					Evidence:        fmt.Sprintf("%s carries %d slots in %s versus %d in the input list.", metaDetail.Label, metaDetail.Slots, candidate.Name, inputDetail.Slots),
				})
			}
			if cutSignal {
				signal.cutWeight += weight
				signal.cutSupport = append(signal.cutSupport, RecommendationSupport{
					Source:          "meta_candidate",
					CandidateDeckID: candidate.DeckID,
					CandidateName:   candidate.Name,
					CandidateRank:   rank,
					Weight:          weight,
					Evidence:        fmt.Sprintf("%s trims %s down to %d slots versus %d in the input list.", candidate.Name, strings.ToLower(inputDetail.Label), metaDetail.Slots, inputDetail.Slots),
				})
			}
			if addSignal != cutSignal && signal.addWeight > 0 && signal.cutWeight > 0 {
				hasSplitSignal = true
			}
		}
	}

	for _, pkg := range []string{"refill_package", "early_board_package", "reactive_package", "late_payoff_package"} {
		inputDetail, ok := inputPackages[pkg]
		if !ok {
			continue
		}
		signal := signals[pkg]
		if signal == nil {
			continue
		}
		if signal.addWeight >= 1.5 {
			lead := primarySupport(signal.addSupport)
			guidance.Summary = append(guidance.Summary, Recommendation{
				Key:        "summary_" + pkg,
				Kind:       "summary",
				Package:    pkg,
				Source:     "multi_candidate_consensus",
				Message:    fmt.Sprintf("Multiple close meta decks agree that your %s is behind the current shape.", strings.ToLower(inputDetail.Label)),
				Confidence: clampConfidence(0.55 + signal.addWeight*0.2),
				Support:    appendPackageSupport(inputDetail, signal.addSupport, "analysis_package_gap"),
			})
			guidance.Adds = append(guidance.Adds, Recommendation{
				Key:        "add_" + pkg,
				Kind:       "add",
				Package:    pkg,
				Source:     "multi_candidate_consensus",
				Message:    fmt.Sprintf("Multiple close meta decks, led by %s, support adding into your %s first.", lead.CandidateName, strings.ToLower(inputDetail.Label)),
				Confidence: clampConfidence(0.6 + signal.addWeight*0.18),
				Support:    appendPackageSupport(inputDetail, signal.addSupport, "analysis_package_gap"),
			})
			hasConsensus = true
			continue
		}
		if signal.cutWeight >= 1.5 {
			lead := primarySupport(signal.cutSupport)
			guidance.Summary = append(guidance.Summary, Recommendation{
				Key:        "summary_" + pkg,
				Kind:       "summary",
				Package:    pkg,
				Source:     "multi_candidate_consensus",
				Message:    fmt.Sprintf("Multiple close meta decks agree that your %s is heavier than the current norm.", strings.ToLower(inputDetail.Label)),
				Confidence: clampConfidence(0.55 + signal.cutWeight*0.2),
				Support:    appendPackageSupport(inputDetail, signal.cutSupport, "analysis_package_gap"),
			})
			guidance.Cuts = append(guidance.Cuts, Recommendation{
				Key:        "cut_" + pkg,
				Kind:       "cut",
				Package:    pkg,
				Source:     "multi_candidate_consensus",
				Message:    fmt.Sprintf("Multiple close meta decks, led by %s, support trimming your %s before smaller flex-slot swaps.", lead.CandidateName, strings.ToLower(inputDetail.Label)),
				Confidence: clampConfidence(0.6 + signal.cutWeight*0.18),
				Support:    appendPackageSupport(inputDetail, signal.cutSupport, "analysis_package_gap"),
			})
			hasConsensus = true
		}
	}

	best := candidates[0]
	if conflict, ok := inputPackages["early_pressure_vs_late_payoff"]; ok && conflict.Status == "conflict" {
		guidance.Cuts = append(guidance.Cuts, Recommendation{
			Key:        "cut_early_pressure_vs_late_payoff",
			Kind:       "cut",
			Package:    conflict.Package,
			Source:     "analysis_package_conflict",
			Message:    fmt.Sprintf("%s shows the cleaner pressure profile here. Resolve the %s tension by trimming one top end slot before copying any extra meta tech.", best.Name, strings.ToLower(conflict.Label)),
			Confidence: 0.72,
			Support: []RecommendationSupport{
				{
					Source:   "analysis_package_conflict",
					Evidence: conflict.Explanation,
				},
				{
					Source:          "meta_candidate",
					CandidateDeckID: best.DeckID,
					CandidateName:   best.Name,
					CandidateRank:   1,
					Weight:          1,
					Evidence:        "Closest candidate keeps a cleaner pressure-first top end profile.",
				},
			},
		})
	}

	if len(guidance.Summary) == 0 && (hasSplitSignal || (considered > 1 && !hasConsensus)) {
		guidance.Summary = append(guidance.Summary, Recommendation{
			Key:        "summary_split_signal",
			Kind:       "summary",
			Source:     "conflict_caution",
			Message:    "Top compare candidates are split on the best correction, so keep package changes conservative and test one direction at a time.",
			Confidence: 0.48,
			Support: []RecommendationSupport{
				{Source: "top_candidate_divergence", Evidence: "The leading compare candidates disagree on which package should move first."},
			},
		})
	}
	if len(guidance.Summary) == 0 && len(best.Summary) > 0 {
		guidance.Summary = append(guidance.Summary, Recommendation{
			Key:        "summary_closest_candidate",
			Kind:       "summary",
			Source:     "closest_candidate_alignment",
			Message:    fmt.Sprintf("Closest compare context comes from %s: %s", best.Name, best.Summary[0]),
			Confidence: 0.58,
			Support: []RecommendationSupport{
				{Source: "meta_candidate", CandidateDeckID: best.DeckID, CandidateName: best.Name, CandidateRank: 1, Weight: 1, Evidence: best.Summary[0]},
			},
		})
	}
	if len(guidance.Adds) == 0 && len(best.SuggestedAdds) > 0 {
		guidance.Adds = append(guidance.Adds, Recommendation{
			Key:        "add_closest_candidate",
			Kind:       "add",
			Source:     "closest_candidate_alignment",
			Message:    fmt.Sprintf("Closest meta reference is %s, so use its package shape as the first add path: %s", best.Name, best.SuggestedAdds[0]),
			Confidence: 0.56,
			Support: []RecommendationSupport{
				{Source: "meta_candidate", CandidateDeckID: best.DeckID, CandidateName: best.Name, CandidateRank: 1, Weight: 1, Evidence: best.SuggestedAdds[0]},
			},
		})
	}
	if len(guidance.Cuts) == 0 && len(best.SuggestedCuts) > 0 {
		guidance.Cuts = append(guidance.Cuts, Recommendation{
			Key:        "cut_closest_candidate",
			Kind:       "cut",
			Source:     "closest_candidate_alignment",
			Message:    fmt.Sprintf("Closest meta reference is %s, so use its package shape as the first cut path: %s", best.Name, best.SuggestedCuts[0]),
			Confidence: 0.56,
			Support: []RecommendationSupport{
				{Source: "meta_candidate", CandidateDeckID: best.DeckID, CandidateName: best.Name, CandidateRank: 1, Weight: 1, Evidence: best.SuggestedCuts[0]},
			},
		})
	}

	return guidanceMessages(guidance.Summary), guidanceMessages(guidance.Adds), guidanceMessages(guidance.Cuts), trimGuidance(guidance, 3)
}

type packageSupport struct {
	addWeight  float64
	cutWeight  float64
	addSupport []RecommendationSupport
	cutSupport []RecommendationSupport
}

func ensurePackageSupport(signals map[string]*packageSupport, pkg string) *packageSupport {
	if existing, ok := signals[pkg]; ok {
		return existing
	}
	signals[pkg] = &packageSupport{}
	return signals[pkg]
}

func candidateAgreementWeight(rank int) float64 {
	switch rank {
	case 1:
		return 1.0
	case 2:
		return 0.7
	default:
		return 0.45
	}
}

func primarySupport(items []RecommendationSupport) RecommendationSupport {
	if len(items) == 0 {
		return RecommendationSupport{}
	}
	best := items[0]
	for _, item := range items[1:] {
		if item.Weight > best.Weight {
			best = item
		}
	}
	return best
}

func appendPackageSupport(detail analysispkg.PackageDetail, items []RecommendationSupport, source string) []RecommendationSupport {
	out := make([]RecommendationSupport, 0, len(items)+1)
	out = append(out, RecommendationSupport{
		Source:   source,
		Evidence: fmt.Sprintf("%s is currently %s at %d slots against a %d-%d target.", detail.Label, detail.Status, detail.Slots, detail.TargetMin, detail.TargetMax),
	})
	out = append(out, items...)
	return out
}

func guidanceMessages(items []Recommendation) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Message)
	}
	return limitStrings(out, 3)
}

func trimGuidance(guidance StructuredGuidance, limit int) StructuredGuidance {
	guidance.Summary = limitRecommendations(guidance.Summary, limit)
	guidance.Adds = limitRecommendations(guidance.Adds, limit)
	guidance.Cuts = limitRecommendations(guidance.Cuts, limit)
	return guidance
}

func limitRecommendations(items []Recommendation, limit int) []Recommendation {
	if len(items) == 0 {
		return nil
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

func clampConfidence(value float64) float64 {
	if value < 0.1 {
		return 0.1
	}
	if value > 0.95 {
		return 0.95
	}
	return value
}

func buildCompareParseResult(class string, format int, cards []decks.DeckCard) decks.ParseResult {
	total := 0
	for _, card := range cards {
		total += card.Count
	}
	return decks.ParseResult{
		Class:      class,
		Format:     format,
		TotalCount: total,
		Legality:   decks.Legality{Valid: total == 30},
		Cards:      cards,
	}
}

func indexAnalysisPackages(items []analysispkg.PackageDetail) map[string]analysispkg.PackageDetail {
	index := make(map[string]analysispkg.PackageDetail, len(items))
	for _, item := range items {
		index[item.Package] = item
	}
	return index
}

func mergedAddSuggestion(input analysispkg.PackageDetail, meta analysispkg.PackageDetail, candidate Candidate) string {
	if len(candidate.SuggestedAdds) > 0 {
		return fmt.Sprintf("%s is underbuilt in your list (%d slots vs %d in %s). Use that package gap to prioritize adds like: %s", input.Label, input.Slots, meta.Slots, candidate.Name, candidate.SuggestedAdds[0])
	}
	return fmt.Sprintf("%s is underbuilt in your list (%d slots vs %d in %s). Shift your next adds toward that package first.", input.Label, input.Slots, meta.Slots, candidate.Name)
}

func mergedCutSuggestion(input analysispkg.PackageDetail, meta analysispkg.PackageDetail, candidate Candidate) string {
	if len(candidate.SuggestedCuts) > 0 {
		return fmt.Sprintf("%s is heavier than %s wants (%d slots vs %d). Start with cuts like: %s", input.Label, candidate.Name, input.Slots, meta.Slots, candidate.SuggestedCuts[0])
	}
	return fmt.Sprintf("%s is heavier than %s wants (%d slots vs %d). Start trimming that package before making smaller flex-slot swaps.", input.Label, candidate.Name, input.Slots, meta.Slots)
}

func limitStrings(items []string, limit int) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
		if len(out) == limit {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func similarityScore(left, right map[string]int) float64 {
	union := 0
	intersection := 0
	seen := make(map[string]struct{}, len(left)+len(right))
	for id, count := range left {
		seen[id] = struct{}{}
		other := right[id]
		if count < other {
			intersection += count
		} else {
			intersection += other
		}
		if count > other {
			union += count
		} else {
			union += other
		}
	}
	for id, count := range right {
		if _, ok := seen[id]; ok {
			continue
		}
		union += count
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func scoreDeckSimilarity(left, right []decks.DeckCard) float64 {
	return computeSimilarityBreakdown(left, right).Total
}

func scoreDeckSimilarityFromCounts(leftCounts map[string]int, leftCards, rightCards []decks.DeckCard) float64 {
	return computeSimilarityBreakdownFromCounts(leftCounts, leftCards, rightCards).Total
}

func computeSimilarityBreakdown(left, right []decks.DeckCard) SimilarityBreakdown {
	return computeSimilarityBreakdownFromCounts(countsFromDeck(left), left, right)
}

func computeSimilarityBreakdownFromCounts(leftCounts map[string]int, leftCards, rightCards []decks.DeckCard) SimilarityBreakdown {
	rightCounts := countsFromDeck(rightCards)

	overlap := similarityScore(leftCounts, rightCounts)
	curve := bucketedDistributionSimilarity(costBuckets(leftCards), costBuckets(rightCards))
	cardTypes := bucketedDistributionSimilarity(cardTypeBuckets(leftCards), cardTypeBuckets(rightCards))
	total := overlap*0.7 + curve*0.2 + cardTypes*0.1

	return SimilarityBreakdown{
		Total:    total,
		Overlap:  overlap,
		Curve:    curve,
		CardType: cardTypes,
	}
}

func costBuckets(cards []decks.DeckCard) map[string]int {
	buckets := map[string]int{
		"0-1": 0,
		"2-3": 0,
		"4-5": 0,
		"6+":  0,
	}
	for _, card := range cards {
		key := "6+"
		switch {
		case card.Cost <= 1:
			key = "0-1"
		case card.Cost <= 3:
			key = "2-3"
		case card.Cost <= 5:
			key = "4-5"
		}
		buckets[key] += card.Count
	}
	return buckets
}

func cardTypeBuckets(cards []decks.DeckCard) map[string]int {
	buckets := map[string]int{}
	for _, card := range cards {
		key := strings.ToUpper(strings.TrimSpace(card.CardType))
		if key == "" {
			key = "UNKNOWN"
		}
		buckets[key] += card.Count
	}
	return buckets
}

func bucketedDistributionSimilarity(left, right map[string]int) float64 {
	totalLeft := 0
	totalRight := 0
	for _, count := range left {
		totalLeft += count
	}
	for _, count := range right {
		totalRight += count
	}
	if totalLeft == 0 && totalRight == 0 {
		return 1
	}
	if totalLeft == 0 || totalRight == 0 {
		return 0
	}

	unionKeys := make(map[string]struct{}, len(left)+len(right))
	for key := range left {
		unionKeys[key] = struct{}{}
	}
	for key := range right {
		unionKeys[key] = struct{}{}
	}

	diff := 0.0
	for key := range unionKeys {
		leftShare := float64(left[key]) / float64(totalLeft)
		rightShare := float64(right[key]) / float64(totalRight)
		if leftShare > rightShare {
			diff += leftShare - rightShare
		} else {
			diff += rightShare - leftShare
		}
	}

	score := 1 - diff/2
	if score < 0 {
		return 0
	}
	return score
}

func buildComparisonSummary(inputCards, metaCards []decks.DeckCard, deckName string) []string {
	summary := make([]string, 0, 3)

	if curveSummary := buildCurveSummary(inputCards, metaCards, deckName); curveSummary != "" {
		summary = append(summary, curveSummary)
	}
	if typeSummary := buildCardTypeSummary(inputCards, metaCards, deckName); typeSummary != "" {
		summary = append(summary, typeSummary)
	}
	if overlapSummary := buildOverlapSummary(inputCards, metaCards, deckName); overlapSummary != "" {
		summary = append(summary, overlapSummary)
	}

	return summary
}

func buildCurveSummary(inputCards, metaCards []decks.DeckCard, deckName string) string {
	inputEarly, inputLate := earlyLateCounts(inputCards)
	metaEarly, metaLate := earlyLateCounts(metaCards)

	inputLateShare := ratio(inputLate, inputEarly+inputLate)
	metaLateShare := ratio(metaLate, metaEarly+metaLate)
	if metaLateShare-inputLateShare >= 0.2 {
		return fmt.Sprintf("%s has a heavier late-game curve than your deck.", deckName)
	}
	if inputLateShare-metaLateShare >= 0.2 {
		return fmt.Sprintf("%s is lower-curve and pressures earlier than your deck.", deckName)
	}
	return ""
}

func buildCardTypeSummary(inputCards, metaCards []decks.DeckCard, deckName string) string {
	inputMinions, inputSpells := typeCounts(inputCards)
	metaMinions, metaSpells := typeCounts(metaCards)

	inputSpellShare := ratio(inputSpells, inputMinions+inputSpells)
	metaSpellShare := ratio(metaSpells, metaMinions+metaSpells)
	if metaSpellShare-inputSpellShare >= 0.2 {
		return fmt.Sprintf("%s leans more on spells and reactive turns than your list.", deckName)
	}
	if inputSpellShare-metaSpellShare >= 0.2 {
		return fmt.Sprintf("%s is more minion-dense than your list.", deckName)
	}
	return ""
}

func buildOverlapSummary(inputCards, metaCards []decks.DeckCard, deckName string) string {
	score := similarityScore(countsFromDeck(inputCards), countsFromDeck(metaCards))
	switch {
	case score >= 0.8:
		return fmt.Sprintf("Your core game plan is already very close to %s.", deckName)
	case score >= 0.5:
		return fmt.Sprintf("Your list overlaps with %s, but still has several flex slots in different places.", deckName)
	default:
		return fmt.Sprintf("%s looks like a more distinct build rather than a near match.", deckName)
	}
}

func earlyLateCounts(cards []decks.DeckCard) (early int, late int) {
	for _, card := range cards {
		if card.Cost <= 3 {
			early += card.Count
			continue
		}
		late += card.Count
	}
	return early, late
}

func typeCounts(cards []decks.DeckCard) (minions int, spells int) {
	for _, card := range cards {
		switch strings.ToUpper(strings.TrimSpace(card.CardType)) {
		case "MINION":
			minions += card.Count
		case "SPELL":
			spells += card.Count
		}
	}
	return minions, spells
}

func ratio(part int, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total)
}

func compareCandidateRank(left, right Candidate) bool {
	if left.Similarity != right.Similarity {
		return left.Similarity > right.Similarity
	}

	leftTier := tierRank(left.Tier)
	rightTier := tierRank(right.Tier)
	if leftTier != rightTier {
		return leftTier < rightTier
	}

	leftPlayrate := floatValueOr(left.Playrate, -1)
	rightPlayrate := floatValueOr(right.Playrate, -1)
	if leftPlayrate != rightPlayrate {
		return leftPlayrate > rightPlayrate
	}

	leftWinrate := floatValueOr(left.Winrate, -1)
	rightWinrate := floatValueOr(right.Winrate, -1)
	if leftWinrate != rightWinrate {
		return leftWinrate > rightWinrate
	}

	return left.DeckID < right.DeckID
}

func tierRank(tier *string) int {
	if tier == nil {
		return 99
	}
	switch strings.ToUpper(strings.TrimSpace(*tier)) {
	case "T1":
		return 1
	case "T2":
		return 2
	case "T3":
		return 3
	case "T4":
		return 4
	default:
		return 99
	}
}

func floatValueOr(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func toFormatName(format int) string {
	if format == 2 {
		return "standard"
	}
	return "wild"
}

func buildCardDiffs(inputCards, metaCards []decks.DeckCard) ([]CardDiff, []CardDiff, []CardDiff) {
	inputByID := make(map[string]decks.DeckCard, len(inputCards))
	metaByID := make(map[string]decks.DeckCard, len(metaCards))
	for _, card := range inputCards {
		inputByID[card.CardID] = card
	}
	for _, card := range metaCards {
		metaByID[card.CardID] = card
	}

	shared := make([]CardDiff, 0)
	metaOnly := make([]CardDiff, 0)
	inputOnly := make([]CardDiff, 0)
	for id, inputCard := range inputByID {
		if metaCard, ok := metaByID[id]; ok {
			shared = append(shared, CardDiff{
				CardID:     id,
				Name:       preferredCardName(inputCard, metaCard),
				InputCount: inputCard.Count,
				MetaCount:  metaCard.Count,
			})
			continue
		}

		inputOnly = append(inputOnly, CardDiff{
			CardID:     id,
			Name:       inputCard.Name,
			InputCount: inputCard.Count,
			MetaCount:  0,
		})
	}
	for id, metaCard := range metaByID {
		if _, ok := inputByID[id]; ok {
			continue
		}
		metaOnly = append(metaOnly, CardDiff{
			CardID:     id,
			Name:       metaCard.Name,
			InputCount: 0,
			MetaCount:  metaCard.Count,
		})
	}

	sortCardDiffs(shared)
	sortCardDiffs(metaOnly)
	sortCardDiffs(inputOnly)
	return shared, metaOnly, inputOnly
}

func preferredCardName(inputCard, metaCard decks.DeckCard) string {
	if inputCard.Name != "" {
		return inputCard.Name
	}
	return metaCard.Name
}

func sortCardDiffs(items []CardDiff) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].CardID < items[j].CardID
		}
		return items[i].Name < items[j].Name
	})
}

func buildSuggestions(verb string, items []CardDiff, deckName string) []string {
	if len(items) == 0 {
		return nil
	}

	limited := items
	if len(limited) > 3 {
		limited = limited[:3]
	}

	out := make([]string, 0, len(limited))
	for _, item := range limited {
		count := item.MetaCount
		if verb == "Cut" {
			count = item.InputCount
		}
		out = append(out, fmt.Sprintf("%s %dx %s to align with %s.", verb, count, item.Name, deckName))
	}
	return out
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

func localizedCardName(card cards.Card) string {
	if len(card.Locales) > 0 && card.Locales[0].Name != "" {
		return card.Locales[0].Name
	}
	return card.ID
}

func buildCardDiffsFromCounts(inputCounts, metaCounts map[string]int, metaCards []decks.DeckCard) ([]CardDiff, []CardDiff, []CardDiff) {
	metaByID := make(map[string]decks.DeckCard, len(metaCards))
	for _, card := range metaCards {
		metaByID[card.CardID] = card
	}
	shared := make([]CardDiff, 0)
	metaOnly := make([]CardDiff, 0)
	inputOnly := make([]CardDiff, 0)
	for id, inputCount := range inputCounts {
		if metaCount, ok := metaCounts[id]; ok {
			name := id
			if card, ok := metaByID[id]; ok && card.Name != "" {
				name = card.Name
			}
			shared = append(shared, CardDiff{CardID: id, Name: name, InputCount: inputCount, MetaCount: metaCount})
		} else {
			inputOnly = append(inputOnly, CardDiff{CardID: id, Name: id, InputCount: inputCount})
		}
	}
	for id, metaCount := range metaCounts {
		if _, ok := inputCounts[id]; ok {
			continue
		}
		name := id
		if card, ok := metaByID[id]; ok && card.Name != "" {
			name = card.Name
		}
		metaOnly = append(metaOnly, CardDiff{CardID: id, Name: name, MetaCount: metaCount})
	}
	sortCardDiffs(shared)
	sortCardDiffs(metaOnly)
	sortCardDiffs(inputOnly)
	return shared, metaOnly, inputOnly
}
