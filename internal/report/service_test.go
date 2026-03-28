package report

import (
	"context"
	"errors"
	"strings"
	"testing"

	"hearthstone-analyzer/internal/analysis"
	comparepkg "hearthstone-analyzer/internal/compare"
	"hearthstone-analyzer/internal/decks"
	"hearthstone-analyzer/internal/settings"
)

func TestServiceGenerateDeckReportUsesAnalysisAndCompareContext(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{result: GeneratedReport{
		Content: "report body",
		Model:   "gpt-test",
		Structured: &StructuredReport{
			DeckIdentity:             []string{"Aggro deck"},
			WhatTheDeckIsDoingWell:   []string{"Fast starts"},
			MainRisks:                []string{"Low refill"},
			PracticalNextAdjustments: []string{"Add draw"},
		},
	}}
	service := NewService(
		stubAnalysisService{result: analysis.Result{
			Archetype:         "Aggro",
			Confidence:        0.82,
			ConfidenceReasons: []string{"Strong early curve concentration supports an aggro read."},
			Strengths:         []string{"Fast early curve"},
			Weaknesses:        []string{"May run out of resources"},
		}},
		stubCompareService{result: comparepkg.Result{
			Candidates: []comparepkg.Candidate{
				{Name: "Cycle Rogue", Similarity: 0.77, Summary: []string{"Cycle Rogue has a heavier late-game curve than your deck."}},
			},
		}},
		stubSettingsService{
			values: map[string]string{
				settings.KeyLLMAPIKey:  "secret",
				settings.KeyLLMBaseURL: "https://api.example.test/v1",
				settings.KeyLLMModel:   "gpt-test",
			},
		},
		provider,
	)

	got, err := service.GenerateDeckReport(context.Background(), "AAEAAA==", "en")
	if err != nil {
		t.Fatalf("GenerateDeckReport() error = %v", err)
	}

	if got.Report != "report body" || got.Model != "gpt-test" {
		t.Fatalf("unexpected report result: %+v", got)
	}

	if got.Structured == nil || len(got.Structured.DeckIdentity) != 1 {
		t.Fatalf("expected structured report on service result, got %+v", got.Structured)
	}

	if provider.lastConfig.Model != "gpt-test" {
		t.Fatalf("expected provider config to use settings model, got %+v", provider.lastConfig)
	}

	if !strings.Contains(provider.lastPrompt.Analysis.Archetype, "Aggro") && provider.lastPrompt.Analysis.Archetype != "Aggro" {
		t.Fatalf("expected analysis to be forwarded to provider, got %+v", provider.lastPrompt)
	}

	if provider.lastPrompt.Compare == nil || len(provider.lastPrompt.Compare.Candidates) != 1 {
		t.Fatalf("expected compare context to be forwarded, got %+v", provider.lastPrompt.Compare)
	}
}

func TestServiceGenerateDeckReportPersistsGeneratedReportWhenStorageConfigured(t *testing.T) {
	t.Parallel()

	reports := &stubReportStore{}
	decksStore := &stubDeckStore{}
	service := NewServiceWithPersistence(
		stubParser{result: decks.ParseResult{
			Class:    "MAGE",
			Format:   2,
			DeckHash: "hash-123",
		}},
		stubAnalysisService{result: analysis.Result{Archetype: "Aggro", Confidence: 0.82}},
		stubCompareService{result: comparepkg.Result{SnapshotID: "snapshot-1"}},
		stubSettingsService{
			values: map[string]string{
				settings.KeyLLMAPIKey:  "secret",
				settings.KeyLLMBaseURL: "https://api.example.test/v1",
				settings.KeyLLMModel:   "gpt-test",
			},
		},
		&stubProvider{result: GeneratedReport{Content: "persisted report", Model: "gpt-test"}},
		decksStore,
		reports,
	)

	got, err := service.GenerateDeckReport(context.Background(), "AAEAAA==", "en")
	if err != nil {
		t.Fatalf("GenerateDeckReport() error = %v", err)
	}

	if got.ReportID == "" {
		t.Fatalf("expected persisted report id, got %+v", got)
	}

	if decksStore.lastDeck.DeckHash != "hash-123" || reports.lastRecord.InputHash != "hash-123" {
		t.Fatalf("expected parsed deck hash to be used for persistence, deck=%+v report=%+v", decksStore.lastDeck, reports.lastRecord)
	}

	if reports.lastRecord.BasedOnSnapshotID == nil || *reports.lastRecord.BasedOnSnapshotID != "snapshot-1" {
		t.Fatalf("expected snapshot id to be persisted, got %+v", reports.lastRecord)
	}
}

func TestServiceGenerateDeckReportContinuesWithoutCompareContext(t *testing.T) {
	t.Parallel()

	provider := &stubProvider{result: GeneratedReport{Content: "analysis-only report", Model: "gpt-test"}}
	service := NewService(
		stubAnalysisService{result: analysis.Result{Archetype: "Midrange", Confidence: 0.55}},
		stubCompareService{err: errors.New("meta snapshot not found")},
		stubSettingsService{
			values: map[string]string{
				settings.KeyLLMAPIKey:  "secret",
				settings.KeyLLMBaseURL: "https://api.example.test/v1",
				settings.KeyLLMModel:   "gpt-test",
			},
		},
		provider,
	)

	got, err := service.GenerateDeckReport(context.Background(), "AAEAAA==", "en")
	if err != nil {
		t.Fatalf("GenerateDeckReport() error = %v", err)
	}

	if got.Compare != nil {
		t.Fatalf("expected compare context to be optional, got %+v", got.Compare)
	}
}

func TestServiceGenerateDeckReportRequiresLLMSettings(t *testing.T) {
	t.Parallel()

	service := NewService(
		stubAnalysisService{result: analysis.Result{Archetype: "Aggro"}},
		nil,
		stubSettingsService{values: map[string]string{
			settings.KeyLLMBaseURL: "https://api.example.test/v1",
			settings.KeyLLMModel:   "gpt-test",
		}},
		&stubProvider{},
	)

	if _, err := service.GenerateDeckReport(context.Background(), "AAEAAA==", "en"); err == nil {
		t.Fatal("expected GenerateDeckReport() to fail when api key is missing")
	}
}

func TestServiceGetReportReturnsPersistedDetail(t *testing.T) {
	t.Parallel()

	reportText := "persisted body"
	reports := &stubReportStore{
		getResult: StoredReport{
			ID:                "report-1",
			DeckID:            "deck-1",
			BasedOnSnapshotID: ptrString("snapshot-1"),
			ReportType:        "ai_deck_report",
			ReportJSON:        `{"report":"persisted body","model":"gpt-test","generated_at":"2026-03-27T01:00:00Z","analysis":{"archetype":"Aggro","confidence":0.81},"compare":{"SnapshotID":"snapshot-1","Format":"standard","Candidates":[]}}`,
			ReportText:        &reportText,
		},
	}
	service := NewServiceWithPersistence(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		reports,
	)

	got, err := service.GetReport(context.Background(), "report-1")
	if err != nil {
		t.Fatalf("GetReport() error = %v", err)
	}

	if got.ID != "report-1" || got.Result.ReportID != "report-1" {
		t.Fatalf("expected report id to round-trip into detail result, got %+v", got)
	}

	if got.Result.Report != "persisted body" || got.Result.Model != "gpt-test" {
		t.Fatalf("unexpected report detail payload: %+v", got.Result)
	}

	if got.Result.Compare == nil || got.Result.Compare.SnapshotID != "snapshot-1" {
		t.Fatalf("expected compare context in persisted detail, got %+v", got.Result.Compare)
	}
}

func TestServiceGetReportReturnsPlainTextFallbackWhenStoredReportHasNoStructuredPayload(t *testing.T) {
	t.Parallel()

	reportText := "plain fallback body"
	reports := &stubReportStore{
		getResult: StoredReport{
			ID:         "report-fallback",
			DeckID:     "deck-1",
			ReportType: "ai_deck_report",
			ReportJSON: `{"report":"plain fallback body","model":"gpt-test","generated_at":"2026-03-27T01:00:00Z","analysis":{"archetype":"Midrange","confidence":0.61}}`,
			ReportText: &reportText,
		},
	}
	service := NewServiceWithPersistence(nil, nil, nil, nil, nil, nil, reports)

	got, err := service.GetReport(context.Background(), "report-fallback")
	if err != nil {
		t.Fatalf("GetReport() error = %v", err)
	}

	if got.Result.Report != "plain fallback body" {
		t.Fatalf("expected plain fallback report body, got %+v", got.Result)
	}

	if got.Result.Structured != nil {
		t.Fatalf("expected structured payload to remain nil, got %+v", got.Result.Structured)
	}
}

func TestServiceGetReportSanitizesStoredStructuredPayloadOnReplay(t *testing.T) {
	t.Parallel()

	reportText := "persisted body"
	reports := &stubReportStore{
		getResult: StoredReport{
			ID:         "report-sanitized",
			DeckID:     "deck-1",
			ReportType: "ai_deck_report",
			ReportJSON: `{"report":"persisted body","model":"gpt-test","generated_at":"2026-03-27T01:00:00Z","analysis":{"archetype":"Aggro","confidence":0.81},"structured":{"deck_identity":["Aggro deck","Aggro deck"],"what_the_deck_is_doing_well":["Fast pressure"],"main_risks":["Low refill"],"practical_next_adjustments":["Add draw"]}}`,
			ReportText: &reportText,
		},
	}
	service := NewServiceWithPersistence(nil, nil, nil, nil, nil, nil, reports)

	got, err := service.GetReport(context.Background(), "report-sanitized")
	if err != nil {
		t.Fatalf("GetReport() error = %v", err)
	}

	if got.Result.Structured == nil {
		t.Fatalf("expected structured payload to survive replay, got %+v", got.Result)
	}

	if len(got.Result.Structured.DeckIdentity) != 1 {
		t.Fatalf("expected duplicate structured items to be normalized on replay, got %+v", got.Result.Structured)
	}
}

func TestServiceGetReportDropsInvalidStoredStructuredPayloadOnReplay(t *testing.T) {
	t.Parallel()

	reportText := "persisted body"
	reports := &stubReportStore{
		getResult: StoredReport{
			ID:         "report-invalid-structured",
			DeckID:     "deck-1",
			ReportType: "ai_deck_report",
			ReportJSON: `{"report":"persisted body","model":"gpt-test","generated_at":"2026-03-27T01:00:00Z","analysis":{"archetype":"Aggro","confidence":0.81},"structured":{"deck_identity":["A","B","C","D","E","F"],"what_the_deck_is_doing_well":["Fast pressure"],"main_risks":["Low refill"],"practical_next_adjustments":["Add draw"]}}`,
			ReportText: &reportText,
		},
	}
	service := NewServiceWithPersistence(nil, nil, nil, nil, nil, nil, reports)

	got, err := service.GetReport(context.Background(), "report-invalid-structured")
	if err != nil {
		t.Fatalf("GetReport() error = %v", err)
	}

	if got.Result.Structured != nil {
		t.Fatalf("expected invalid stored structured payload to be dropped on replay, got %+v", got.Result.Structured)
	}

	if got.Result.Report != "persisted body" {
		t.Fatalf("expected plain report body to remain available, got %+v", got.Result)
	}
}

type stubAnalysisService struct {
	result analysis.Result
	err    error
}

func (s stubAnalysisService) AnalyzeDeck(ctx context.Context, deckCode string) (analysis.Result, error) {
	return s.result, s.err
}

type stubParser struct {
	result decks.ParseResult
	err    error
}

func (s stubParser) Parse(ctx context.Context, deckCode string) (decks.ParseResult, error) {
	return s.result, s.err
}

type stubCompareService struct {
	result comparepkg.Result
	err    error
}

func (s stubCompareService) CompareDeck(ctx context.Context, deckCode string, limit int) (comparepkg.Result, error) {
	return s.result, s.err
}

type stubSettingsService struct {
	values map[string]string
}

func (s stubSettingsService) Get(ctx context.Context, key string) (settings.Setting, error) {
	value, ok := s.values[key]
	if !ok {
		return settings.Setting{}, errors.New("setting not found")
	}
	return settings.Setting{Key: key, Value: value}, nil
}

type stubProvider struct {
	result     GeneratedReport
	err        error
	lastPrompt PromptInput
	lastConfig ProviderConfig
}

func (s *stubProvider) GenerateReport(ctx context.Context, input PromptInput, cfg ProviderConfig) (GeneratedReport, error) {
	s.lastPrompt = input
	s.lastConfig = cfg
	return s.result, s.err
}

type stubDeckStore struct {
	lastDeck DeckRecord
}

func (s *stubDeckStore) UpsertReportDeck(ctx context.Context, deck DeckRecord) (string, error) {
	s.lastDeck = deck
	return "deck-report-1", nil
}

type stubReportStore struct {
	lastRecord ReportRecord
	listResult []StoredReport
	getResult  StoredReport
}

func (s *stubReportStore) Create(ctx context.Context, record ReportRecord) error {
	s.lastRecord = record
	return nil
}

func (s *stubReportStore) ListReports(ctx context.Context, limit int) ([]StoredReport, error) {
	return s.listResult, nil
}

func (s *stubReportStore) GetReport(ctx context.Context, id string) (StoredReport, error) {
	return s.getResult, nil
}

func ptrString(v string) *string {
	return &v
}

