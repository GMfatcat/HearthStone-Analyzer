package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	comparepkg "hearthstone-analyzer/internal/compare"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type OpenAICompatibleProvider struct {
	client HTTPClient
}

type openAIChatRequest struct {
	Model    string              `json:"model"`
	Messages []openAIChatMessage `json:"messages"`
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message openAIChatMessage `json:"message"`
	} `json:"choices"`
}

func NewOpenAICompatibleProvider(client HTTPClient) *OpenAICompatibleProvider {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Minute}
	}
	return &OpenAICompatibleProvider{client: client}
}

func (p *OpenAICompatibleProvider) GenerateReport(ctx context.Context, input PromptInput, cfg ProviderConfig) (GeneratedReport, error) {
	requestBody := openAIChatRequest{
		Model: cfg.Model,
		Messages: []openAIChatMessage{
			{
				Role:    "system",
				Content: buildSystemPrompt(input.Language),
			},
			{
				Role:    "user",
				Content: buildPrompt(input),
			},
		},
	}

	rawBody, err := json.Marshal(requestBody)
	if err != nil {
		return GeneratedReport{}, fmt.Errorf("marshal openai request: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(rawBody))
	if err != nil {
		return GeneratedReport{}, fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return GeneratedReport{}, fmt.Errorf("send openai request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return GeneratedReport{}, fmt.Errorf("read openai response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GeneratedReport{}, fmt.Errorf("openai-compatible request failed: status %d", resp.StatusCode)
	}

	var payload openAIChatResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return GeneratedReport{}, fmt.Errorf("decode openai response: %w", err)
	}
	if len(payload.Choices) == 0 || strings.TrimSpace(payload.Choices[0].Message.Content) == "" {
		return GeneratedReport{}, fmt.Errorf("openai-compatible response did not include content")
	}

	content := strings.TrimSpace(payload.Choices[0].Message.Content)
	structured, err := parseStructuredReport(content)
	if err == nil {
		if err := validateStructuredReport(structured, input); err == nil {
			return GeneratedReport{
				Content:    formatStructuredReport(structured),
				Model:      firstNonEmpty(payload.Model, cfg.Model),
				Structured: &structured,
			}, nil
		}
	}

	return GeneratedReport{
		Content: content,
		Model:   firstNonEmpty(payload.Model, cfg.Model),
	}, nil
}

func buildSystemPrompt(language string) string {
	base := "You are a Hearthstone deck analyst. Use only provided facts. Do not invent matchup data, winrates, tiers, snapshots, or cards. If compare data is unavailable, explicitly say that compare context is unavailable."
	if language == "zh" {
		return base + " Write all natural-language content in Traditional Chinese. Keep the JSON schema keys exactly as requested."
	}
	return base + " Write all natural-language content in English. Keep the JSON schema keys exactly as requested."
}

func buildPrompt(input PromptInput) string {
	var b strings.Builder
	b.WriteString("Write a concise Hearthstone deck report.\n")
	if input.Language == "zh" {
		b.WriteString("Return all explanatory text in Traditional Chinese.\n")
	} else {
		b.WriteString("Return all explanatory text in English.\n")
	}
	b.WriteString("Deck code:\n")
	b.WriteString(input.DeckCode)
	b.WriteString("\n\nAnalysis summary:\n")
	b.WriteString(fmt.Sprintf("- Archetype: %s\n", input.Analysis.Archetype))
	b.WriteString(fmt.Sprintf("- Confidence: %.2f\n", input.Analysis.Confidence))
	for _, reason := range input.Analysis.ConfidenceReasons {
		b.WriteString("- Confidence reason: " + reason + "\n")
	}
	for _, strength := range input.Analysis.Strengths {
		b.WriteString("- Strength: " + strength + "\n")
	}
	for _, weakness := range input.Analysis.Weaknesses {
		b.WriteString("- Weakness: " + weakness + "\n")
	}

	if input.Compare != nil && len(input.Compare.Candidates) > 0 {
		b.WriteString("\nClosest meta candidates:\n")
		for _, candidate := range input.Compare.Candidates {
			b.WriteString(fmt.Sprintf("- %s (similarity %.2f", candidate.Name, candidate.Similarity))
			if candidate.Tier != nil {
				b.WriteString(", tier " + *candidate.Tier)
			}
			b.WriteString(")\n")
			for _, summary := range candidate.Summary {
				b.WriteString("  - " + summary + "\n")
			}
		}
		if len(input.Compare.MergedGuidance.Summary) > 0 || len(input.Compare.MergedGuidance.Adds) > 0 || len(input.Compare.MergedGuidance.Cuts) > 0 {
			b.WriteString("\nMerged compare-aware recommendations:\n")
			writeGuidanceSection(&b, "Summary guidance", input.Compare.MergedGuidance.Summary)
			writeGuidanceSection(&b, "Add guidance", input.Compare.MergedGuidance.Adds)
			writeGuidanceSection(&b, "Cut guidance", input.Compare.MergedGuidance.Cuts)
		} else if len(input.Compare.MergedSummary) > 0 || len(input.Compare.MergedSuggestedAdds) > 0 || len(input.Compare.MergedSuggestedCuts) > 0 {
			b.WriteString("\nMerged compare-aware recommendations:\n")
			for _, item := range input.Compare.MergedSummary {
				b.WriteString("- " + item + "\n")
			}
			for _, item := range input.Compare.MergedSuggestedAdds {
				b.WriteString("- Add context: " + item + "\n")
			}
			for _, item := range input.Compare.MergedSuggestedCuts {
				b.WriteString("- Cut context: " + item + "\n")
			}
		}
	} else {
		b.WriteString("\nNo meta comparison data was available. Mention that compare context is unavailable.\n")
		b.WriteString("Do not claim any closest meta deck, snapshot, matchup, winrate, or tier data when compare context is unavailable.\n")
	}

	b.WriteString("\nOutput rules:\n")
	b.WriteString("- Use only facts provided above.\n")
	b.WriteString("- If a fact is not present above, say it is unavailable.\n")
	b.WriteString("- Do not invent matchup data, winrates, tiers, snapshots, or card names.\n")
	b.WriteString("- Keep each section concise and practical.\n")
	b.WriteString("- Return JSON only. Do not wrap it in markdown fences.\n")

	b.WriteString("\nOutput schema:\n")
	b.WriteString("{\n")
	b.WriteString("  \"deck_identity\": [\"...\"],\n")
	b.WriteString("  \"what_the_deck_is_doing_well\": [\"...\"],\n")
	b.WriteString("  \"main_risks\": [\"...\"],\n")
	b.WriteString("  \"practical_next_adjustments\": [\"...\"]\n")
	b.WriteString("}\n")

	b.WriteString("\nOutput sections:\n")
	b.WriteString("1. Deck identity\n2. What the deck is doing well\n3. Main risks\n4. Practical next adjustments\n")
	return b.String()
}

func writeGuidanceSection(b *strings.Builder, title string, items []comparepkg.Recommendation) {
	if len(items) == 0 {
		return
	}
	b.WriteString(title + ":\n")
	for _, item := range items {
		b.WriteString(fmt.Sprintf("- [%s] %s (confidence %.2f)\n", item.Source, item.Message, item.Confidence))
		for _, support := range item.Support {
			if strings.TrimSpace(support.Evidence) == "" {
				continue
			}
			b.WriteString("  - Support: " + support.Evidence + "\n")
		}
	}
}

type structuredReportPayload struct {
	DeckIdentity             any `json:"deck_identity"`
	WhatTheDeckIsDoingWell   any `json:"what_the_deck_is_doing_well"`
	MainRisks                any `json:"main_risks"`
	PracticalNextAdjustments any `json:"practical_next_adjustments"`
}

const maxStructuredSectionItems = 5
const maxStructuredItemLength = 240

func parseStructuredReport(raw string) (StructuredReport, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	var payload structuredReportPayload
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return StructuredReport{}, err
	}

	out := StructuredReport{
		DeckIdentity:             normalizeStructuredField(payload.DeckIdentity),
		WhatTheDeckIsDoingWell:   normalizeStructuredField(payload.WhatTheDeckIsDoingWell),
		MainRisks:                normalizeStructuredField(payload.MainRisks),
		PracticalNextAdjustments: normalizeStructuredField(payload.PracticalNextAdjustments),
	}

	totalItems := len(out.DeckIdentity) + len(out.WhatTheDeckIsDoingWell) + len(out.MainRisks) + len(out.PracticalNextAdjustments)
	if totalItems == 0 {
		return StructuredReport{}, fmt.Errorf("structured report was empty")
	}
	if countNonEmptySections(out) < 2 {
		return StructuredReport{}, fmt.Errorf("structured report did not include enough populated sections")
	}
	if totalItems < 3 {
		return StructuredReport{}, fmt.Errorf("structured report did not include enough detail")
	}
	if err := validateStructuredShape(out); err != nil {
		return StructuredReport{}, err
	}

	return out, nil
}

func validateStructuredShape(report StructuredReport) error {
	for section, items := range map[string][]string{
		"deck_identity":               report.DeckIdentity,
		"what_the_deck_is_doing_well": report.WhatTheDeckIsDoingWell,
		"main_risks":                  report.MainRisks,
		"practical_next_adjustments":  report.PracticalNextAdjustments,
	} {
		if len(items) > maxStructuredSectionItems {
			return fmt.Errorf("%s exceeded item limit", section)
		}
		for _, item := range items {
			if len(item) > maxStructuredItemLength {
				return fmt.Errorf("%s contained overlong item", section)
			}
		}
	}
	return nil
}

func validateStructuredReport(report StructuredReport, input PromptInput) error {
	for section, items := range map[string][]string{
		"deck_identity":               report.DeckIdentity,
		"what_the_deck_is_doing_well": report.WhatTheDeckIsDoingWell,
		"main_risks":                  report.MainRisks,
		"practical_next_adjustments":  report.PracticalNextAdjustments,
	} {
		for _, item := range items {
			if err := validateStructuredItem(section, item, input); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateStructuredItem(section, item string, input PromptInput) error {
	normalized := strings.TrimSpace(item)
	if normalized == "" {
		return fmt.Errorf("%s item was empty", section)
	}
	if len(normalized) > maxStructuredItemLength {
		return fmt.Errorf("%s item exceeded length limit", section)
	}

	lowered := strings.ToLower(normalized)
	for _, forbidden := range []string{"```", "## ", "<script", "</", "{\"", "\"}"} {
		if strings.Contains(lowered, forbidden) {
			return fmt.Errorf("%s item contained markup or serialized payload", section)
		}
	}

	if strings.Contains(lowered, "matchup") || strings.Contains(lowered, "matchups") {
		return fmt.Errorf("%s item referenced unavailable matchup data", section)
	}

	if input.Compare == nil || len(input.Compare.Candidates) == 0 {
		for _, forbidden := range []string{
			"meta deck", "closest meta", "snapshot", "tier ", "winrate", "playrate", "sample size",
		} {
			if strings.Contains(lowered, forbidden) {
				return fmt.Errorf("%s item referenced compare-only data without compare context", section)
			}
		}
	} else {
		if strings.Contains(lowered, "snapshot") && strings.TrimSpace(input.Compare.SnapshotID) == "" {
			return fmt.Errorf("%s item referenced snapshot data without snapshot id", section)
		}
		if strings.Contains(lowered, "tier") && !compareHasAnyTier(input.Compare) {
			return fmt.Errorf("%s item referenced tier data without grounded tier values", section)
		}
		if strings.Contains(lowered, "winrate") && !compareHasAnyWinrate(input.Compare) {
			return fmt.Errorf("%s item referenced winrate data without grounded winrate values", section)
		}
		if strings.Contains(lowered, "playrate") && !compareHasAnyPlayrate(input.Compare) {
			return fmt.Errorf("%s item referenced playrate data without grounded playrate values", section)
		}
	}

	if strings.Contains(lowered, "%") || strings.Contains(lowered, " percent") {
		if !percentagesGroundedInCompare(input.Compare, lowered) {
			return fmt.Errorf("%s item referenced unsupported percentage data", section)
		}
	}

	return nil
}

func percentagesGroundedInCompare(compare *comparepkg.Result, item string) bool {
	if compare == nil {
		return false
	}
	if strings.Contains(item, "similarity") {
		return true
	}
	if strings.Contains(item, "winrate") && compareHasAnyWinrate(compare) {
		return true
	}
	if strings.Contains(item, "playrate") && compareHasAnyPlayrate(compare) {
		return true
	}
	return false
}

func compareHasAnyTier(compare *comparepkg.Result) bool {
	if compare == nil {
		return false
	}
	for _, candidate := range compare.Candidates {
		if candidate.Tier != nil && strings.TrimSpace(*candidate.Tier) != "" {
			return true
		}
	}
	return false
}

func compareHasAnyWinrate(compare *comparepkg.Result) bool {
	if compare == nil {
		return false
	}
	for _, candidate := range compare.Candidates {
		if candidate.Winrate != nil {
			return true
		}
	}
	return false
}

func compareHasAnyPlayrate(compare *comparepkg.Result) bool {
	if compare == nil {
		return false
	}
	for _, candidate := range compare.Candidates {
		if candidate.Playrate != nil {
			return true
		}
	}
	return false
}

func countNonEmptySections(report StructuredReport) int {
	count := 0
	if len(report.DeckIdentity) > 0 {
		count++
	}
	if len(report.WhatTheDeckIsDoingWell) > 0 {
		count++
	}
	if len(report.MainRisks) > 0 {
		count++
	}
	if len(report.PracticalNextAdjustments) > 0 {
		count++
	}
	return count
}

func normalizeStructuredField(value any) []string {
	switch typed := value.(type) {
	case string:
		normalized := strings.TrimSpace(typed)
		if normalized == "" {
			return nil
		}
		return []string{normalized}
	case []any:
		out := make([]string, 0, len(typed))
		seen := make(map[string]struct{}, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				continue
			}
			normalized := strings.TrimSpace(text)
			if normalized == "" {
				continue
			}
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			out = append(out, normalized)
		}
		return out
	default:
		return nil
	}
}

func formatStructuredReport(report StructuredReport) string {
	var b strings.Builder
	writeStructuredSection(&b, "Deck Identity", report.DeckIdentity)
	writeStructuredSection(&b, "What The Deck Is Doing Well", report.WhatTheDeckIsDoingWell)
	writeStructuredSection(&b, "Main Risks", report.MainRisks)
	writeStructuredSection(&b, "Practical Next Adjustments", report.PracticalNextAdjustments)
	return strings.TrimSpace(b.String())
}

func writeStructuredSection(b *strings.Builder, title string, items []string) {
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n")
	if len(items) == 0 {
		b.WriteString("- Unavailable\n\n")
		return
	}
	for _, item := range items {
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
	b.WriteString("\n")
}
