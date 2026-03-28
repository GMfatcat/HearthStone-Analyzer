package report

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"hearthstone-analyzer/internal/analysis"
	comparepkg "hearthstone-analyzer/internal/compare"
	"hearthstone-analyzer/internal/decks"
	"hearthstone-analyzer/internal/settings"
)

type AnalysisService interface {
	AnalyzeDeck(ctx context.Context, deckCode string) (analysis.Result, error)
}

type CompareService interface {
	CompareDeck(ctx context.Context, deckCode string, limit int) (comparepkg.Result, error)
}

type SettingsService interface {
	Get(ctx context.Context, key string) (settings.Setting, error)
}

type Provider interface {
	GenerateReport(ctx context.Context, input PromptInput, cfg ProviderConfig) (GeneratedReport, error)
}

type Parser interface {
	Parse(ctx context.Context, deckCode string) (decks.ParseResult, error)
}

type DeckRecord struct {
	Source    string
	Class     string
	Format    string
	DeckCode  string
	Archetype string
	DeckHash  string
}

type DeckStore interface {
	UpsertReportDeck(ctx context.Context, deck DeckRecord) (string, error)
}

type ReportRecord struct {
	ID                string
	DeckID            string
	BasedOnSnapshotID *string
	ReportType        string
	InputHash         string
	ReportJSON        string
	ReportText        *string
}

type ReportStore interface {
	Create(ctx context.Context, record ReportRecord) error
	ListReports(ctx context.Context, limit int) ([]StoredReport, error)
	GetReport(ctx context.Context, id string) (StoredReport, error)
}

type ProviderConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

type PromptInput struct {
	DeckCode string
	Language string
	Analysis analysis.Result
	Compare  *comparepkg.Result
}

type GeneratedReport struct {
	Content    string
	Model      string
	Structured *StructuredReport
}

type StructuredReport struct {
	DeckIdentity             []string `json:"deck_identity,omitempty"`
	WhatTheDeckIsDoingWell   []string `json:"what_the_deck_is_doing_well,omitempty"`
	MainRisks                []string `json:"main_risks,omitempty"`
	PracticalNextAdjustments []string `json:"practical_next_adjustments,omitempty"`
}

type Result struct {
	ReportID    string             `json:"report_id,omitempty"`
	Report      string             `json:"report"`
	Model       string             `json:"model"`
	GeneratedAt time.Time          `json:"generated_at"`
	Analysis    analysis.Result    `json:"analysis"`
	Compare     *comparepkg.Result `json:"compare,omitempty"`
	Structured  *StructuredReport  `json:"structured,omitempty"`
}

type ReportDetail struct {
	ID                string    `json:"id"`
	DeckID            string    `json:"deck_id"`
	BasedOnSnapshotID *string   `json:"based_on_snapshot_id,omitempty"`
	ReportType        string    `json:"report_type"`
	CreatedAt         time.Time `json:"created_at"`
	Result            Result    `json:"result"`
}

type Service struct {
	parser   Parser
	analysis AnalysisService
	compare  CompareService
	settings SettingsService
	provider Provider
	decks    DeckStore
	reports  ReportStore
}

func NewService(analysis AnalysisService, compare CompareService, settings SettingsService, provider Provider) *Service {
	return &Service{
		analysis: analysis,
		compare:  compare,
		settings: settings,
		provider: provider,
	}
}

func NewServiceWithPersistence(parser Parser, analysis AnalysisService, compare CompareService, settings SettingsService, provider Provider, decks DeckStore, reports ReportStore) *Service {
	return &Service{
		parser:   parser,
		analysis: analysis,
		compare:  compare,
		settings: settings,
		provider: provider,
		decks:    decks,
		reports:  reports,
	}
}

func (s *Service) GenerateDeckReport(ctx context.Context, deckCode string, language string) (Result, error) {
	if strings.TrimSpace(deckCode) == "" {
		return Result{}, fmt.Errorf("deck_code is required")
	}
	if s.analysis == nil || s.settings == nil || s.provider == nil {
		return Result{}, fmt.Errorf("report service is not fully configured")
	}

	cfg, err := s.loadProviderConfig(ctx)
	if err != nil {
		return Result{}, err
	}

	analysisResult, err := s.analysis.AnalyzeDeck(ctx, deckCode)
	if err != nil {
		return Result{}, err
	}

	var compareResult *comparepkg.Result
	if s.compare != nil {
		if compared, err := s.compare.CompareDeck(ctx, deckCode, 3); err == nil {
			compareResult = &compared
		}
	}

	generated, err := s.provider.GenerateReport(ctx, PromptInput{
		DeckCode: deckCode,
		Language: normalizeLanguage(language),
		Analysis: analysisResult,
		Compare:  compareResult,
	}, cfg)
	if err != nil {
		return Result{}, err
	}

	generatedAt := time.Now().UTC()
	result := Result{
		Report:      generated.Content,
		Model:       firstNonEmpty(generated.Model, cfg.Model),
		GeneratedAt: generatedAt,
		Analysis:    analysisResult,
		Compare:     compareResult,
		Structured:  generated.Structured,
	}

	if persistedID, err := s.persistGeneratedReport(ctx, deckCode, result); err == nil {
		result.ReportID = persistedID
	}

	return result, nil
}

func normalizeLanguage(language string) string {
	lowered := strings.ToLower(strings.TrimSpace(language))
	switch lowered {
	case "zh", "zh-tw", "zh_tw", "traditional_chinese":
		return "zh"
	default:
		return "en"
	}
}

func (s *Service) ListReports(ctx context.Context, limit int) ([]StoredReport, error) {
	if s.reports == nil {
		return nil, fmt.Errorf("report store is not configured")
	}
	return s.reports.ListReports(ctx, limit)
}

func (s *Service) GetReport(ctx context.Context, id string) (ReportDetail, error) {
	if s.reports == nil {
		return ReportDetail{}, fmt.Errorf("report store is not configured")
	}
	if strings.TrimSpace(id) == "" {
		return ReportDetail{}, fmt.Errorf("report id is required")
	}

	stored, err := s.reports.GetReport(ctx, id)
	if err != nil {
		return ReportDetail{}, err
	}

	var result Result
	if err := json.Unmarshal([]byte(stored.ReportJSON), &result); err != nil {
		return ReportDetail{}, fmt.Errorf("unmarshal stored report %q: %w", id, err)
	}
	normalizeStoredReportResult(&result)
	result.ReportID = stored.ID

	return ReportDetail{
		ID:                stored.ID,
		DeckID:            stored.DeckID,
		BasedOnSnapshotID: stored.BasedOnSnapshotID,
		ReportType:        stored.ReportType,
		CreatedAt:         stored.CreatedAt,
		Result:            result,
	}, nil
}

func normalizeStoredReportResult(result *Result) {
	if result == nil || result.Structured == nil {
		return
	}

	rawStructured, err := json.Marshal(result.Structured)
	if err != nil {
		result.Structured = nil
		return
	}

	normalized, err := parseStructuredReport(string(rawStructured))
	if err != nil {
		result.Structured = nil
		return
	}

	result.Structured = &normalized
}

func (s *Service) loadProviderConfig(ctx context.Context) (ProviderConfig, error) {
	baseURL, err := s.requiredSetting(ctx, settings.KeyLLMBaseURL)
	if err != nil {
		return ProviderConfig{}, err
	}
	apiKey, err := s.requiredSetting(ctx, settings.KeyLLMAPIKey)
	if err != nil {
		return ProviderConfig{}, err
	}
	model, err := s.requiredSetting(ctx, settings.KeyLLMModel)
	if err != nil {
		return ProviderConfig{}, err
	}

	return ProviderConfig{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	}, nil
}

func (s *Service) requiredSetting(ctx context.Context, key string) (string, error) {
	setting, err := s.settings.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("load setting %q: %w", key, err)
	}
	value := strings.TrimSpace(setting.Value)
	if value == "" {
		return "", fmt.Errorf("setting %q cannot be empty", key)
	}
	return value, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *Service) persistGeneratedReport(ctx context.Context, deckCode string, result Result) (string, error) {
	if s.parser == nil || s.decks == nil || s.reports == nil {
		return "", nil
	}

	parsed, err := s.parser.Parse(ctx, deckCode)
	if err != nil {
		return "", err
	}

	deckID, err := s.decks.UpsertReportDeck(ctx, DeckRecord{
		Source:    "user_report",
		Class:     parsed.Class,
		Format:    toFormatName(parsed.Format),
		DeckCode:  deckCode,
		Archetype: result.Analysis.Archetype,
		DeckHash:  parsed.DeckHash,
	})
	if err != nil {
		return "", err
	}

	rawJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal report result: %w", err)
	}

	reportID := buildReportID(parsed.DeckHash, result.Model, result.GeneratedAt)
	var snapshotID *string
	if result.Compare != nil && strings.TrimSpace(result.Compare.SnapshotID) != "" {
		snapshotID = &result.Compare.SnapshotID
	}
	reportText := result.Report
	if err := s.reports.Create(ctx, ReportRecord{
		ID:                reportID,
		DeckID:            deckID,
		BasedOnSnapshotID: snapshotID,
		ReportType:        "ai_deck_report",
		InputHash:         parsed.DeckHash,
		ReportJSON:        string(rawJSON),
		ReportText:        &reportText,
	}); err != nil {
		return "", err
	}

	return reportID, nil
}

func buildReportID(deckHash, model string, generatedAt time.Time) string {
	sum := sha1.Sum([]byte(deckHash + "|" + model + "|" + generatedAt.UTC().Format(time.RFC3339Nano)))
	return fmt.Sprintf("report_%x", sum[:8])
}

func toFormatName(format int) string {
	if format == 2 {
		return "standard"
	}
	return "wild"
}
