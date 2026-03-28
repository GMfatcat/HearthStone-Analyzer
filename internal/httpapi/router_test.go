package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"hearthstone-analyzer/internal/analysis"
	"hearthstone-analyzer/internal/cards"
	comparepkg "hearthstone-analyzer/internal/compare"
	"hearthstone-analyzer/internal/decks"
	"hearthstone-analyzer/internal/jobs"
	"hearthstone-analyzer/internal/meta"
	reportpkg "hearthstone-analyzer/internal/report"
	"hearthstone-analyzer/internal/settings"
)

func TestRouterHealthz(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := strings.TrimSpace(rec.Body.String()); got != "ok" {
		t.Fatalf("expected body %q, got %q", "ok", got)
	}
}

func TestRouterServesFrontendIndex(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected html content type, got %q", contentType)
	}

	if body := rec.Body.String(); !strings.Contains(body, "<div id=\"app\"></div>") {
		t.Fatalf("expected frontend app shell in body, got %q", body)
	}
}

func TestRouterServesFrontendAssets(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "javascript") {
		t.Fatalf("expected javascript content type, got %q", contentType)
	}

	if body := rec.Body.String(); !strings.Contains(body, "boot") {
		t.Fatalf("expected asset body to contain marker, got %q", body)
	}
}

func TestRouterListsSettings(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Settings: &stubSettingsService{
			listResult: []settings.Setting{
				{Key: settings.KeyLLMAPIKey, Value: "secret", Sensitive: true, Description: "key"},
				{Key: settings.KeyLLMBaseURL, Value: "https://api.example.test/v1", Sensitive: false, Description: "url"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got []settingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 settings, got %d", len(got))
	}

	if got[0].Key != settings.KeyLLMAPIKey || got[1].Key != settings.KeyLLMBaseURL {
		t.Fatalf("unexpected settings response: %+v", got)
	}
}

func TestRouterUpdatesSetting(t *testing.T) {
	t.Parallel()

	stub := &stubSettingsService{
		getResult: settings.Setting{
			Key:         settings.KeyLLMModel,
			Value:       "gpt-4.1-mini",
			Sensitive:   false,
			Description: "model",
		},
	}
	handler := NewRouter(testFrontendFS(t), Dependencies{Settings: stub})

	body := bytes.NewBufferString(`{"value":"gpt-4.1-mini"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/settings/llm.model", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if stub.lastUpsert.Key != settings.KeyLLMModel || stub.lastUpsert.Value != "gpt-4.1-mini" {
		t.Fatalf("unexpected upsert input: %+v", stub.lastUpsert)
	}
}

func TestRouterRejectsInvalidSettingPayload(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Settings: &stubSettingsService{},
	})

	req := httptest.NewRequest(http.MethodPut, "/api/settings/llm.model", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestRouterListsCards(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Cards: &stubCardsService{
			listResult: []cards.Summary{
				{
					ID:       "CORE_EX1_391",
					Class:    "WARRIOR",
					CardType: "SPELL",
					Cost:     2,
					Name:     "Slam",
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/cards?class=WARRIOR", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got []cardsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(got) != 1 || got[0].ID != "CORE_EX1_391" {
		t.Fatalf("unexpected cards response: %+v", got)
	}
}

func TestRouterListsCardsWithSetAndCostFilters(t *testing.T) {
	t.Parallel()

	stub := &stubCardsService{
		listResult: []cards.Summary{
			{
				ID:       "CORE_EX1_391",
				Class:    "WARRIOR",
				CardType: "SPELL",
				Cost:     2,
				Name:     "Slam",
			},
		},
	}
	handler := NewRouter(testFrontendFS(t), Dependencies{Cards: stub})

	req := httptest.NewRequest(http.MethodGet, "/api/cards?set=CORE&cost=2", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if stub.lastFilter.Set != "CORE" || stub.lastFilter.Cost == nil || *stub.lastFilter.Cost != 2 {
		t.Fatalf("unexpected filter passed to service: %+v", stub.lastFilter)
	}
}

func TestRouterGetsCardByID(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Cards: &stubCardsService{
			getResult: cards.Summary{
				ID:       "CORE_CS2_029",
				Class:    "MAGE",
				CardType: "SPELL",
				Cost:     4,
				Name:     "Fireball",
				Text:     "Deal 6 damage.",
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/cards/CORE_CS2_029", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got cardsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ID != "CORE_CS2_029" || got.Name != "Fireball" {
		t.Fatalf("unexpected card response: %+v", got)
	}
}

func TestRouterReturnsCardNotFound(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Cards: &stubCardsService{
			getErr: errors.New("card not found"),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/cards/UNKNOWN", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestRouterParsesDeckCode(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Decks: &stubDecksService{
			parseResult: decks.ParseResult{
				Class:      "WARRIOR",
				Format:     2,
				TotalCount: 30,
				DeckHash:   "abc123",
				Legality: decks.Legality{
					Valid: true,
				},
				Cards: []decks.DeckCard{
					{CardID: "CORE_001", Name: "Guard", Count: 2, Cost: 1, Class: "WARRIOR", CardType: "MINION"},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/decks/parse", bytes.NewBufferString(`{"deck_code":"AAEAAA=="}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got parseDeckResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Class != "WARRIOR" || got.TotalCount != 30 || got.DeckHash != "abc123" {
		t.Fatalf("unexpected parse response: %+v", got)
	}
}

func TestRouterRejectsInvalidParsePayload(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Decks: &stubDecksService{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/decks/parse", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestRouterReturnsStructuredParseError(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Decks: &stubDecksService{
			parseErr: decks.NewParseError(decks.ErrCodeInvalidDeckCode, "deck code is not valid base64"),
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/decks/parse", bytes.NewBufferString(`{"deck_code":"bad"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var got apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Code != string(decks.ErrCodeInvalidDeckCode) {
		t.Fatalf("expected error code %q, got %q", decks.ErrCodeInvalidDeckCode, got.Code)
	}

	if got.Message == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestRouterAnalyzesDeckCode(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Analysis: &stubAnalysisService{
			result: analysis.Result{
				Archetype:         "Aggro",
				Confidence:        0.84,
				ConfidenceReasons: []string{"Strong early curve concentration supports an aggro read."},
				Features: analysis.Features{
					AvgCost:     2.1,
					ManaCurve:   map[int]int{1: 8, 2: 10},
					MinionCount: 20,
					SpellCount:  10,
				},
				Strengths:      []string{"Fast early curve"},
				Weaknesses:     []string{"May run out of resources"},
				StructuralTags: []string{"thin_early_board", "low_refill"},
				StructuralTagDetails: []analysis.StructuralTagDetail{
					{
						Tag:         "thin_early_board",
						Title:       "Thin early board",
						Explanation: "Only four 1-2 cost minion slots means the deck struggles to contest board before turn three.",
					},
				},
				PackageDetails: []analysis.PackageDetail{
					{
						Package:     "early_board_package",
						Label:       "Early board package",
						Status:      "underbuilt",
						Slots:       4,
						Explanation: "The deck is short on proactive early board slots.",
					},
				},
				FunctionalRoleSummary: []analysis.FunctionalRoleSummaryItem{
					{
						Role:        "refill",
						Label:       "Refill",
						Count:       4,
						Explanation: "Draw and discover effects give the deck ways to reload resources.",
					},
				},
				SuggestedAdds: []string{"Add 2-4 more draw or discover effects so the deck does not stall after the opening push."},
				SuggestedCuts: []string{"Trim a couple of higher-cost reactive cards to keep the opening turns cleaner."},
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/decks/analyze", bytes.NewBufferString(`{"deck_code":"AAEAAA=="}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got analyzeDeckResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Archetype != "Aggro" || got.Features.MinionCount != 20 {
		t.Fatalf("unexpected analyze response: %+v", got)
	}

	if got.Confidence != 0.84 {
		t.Fatalf("expected confidence in analyze response, got %+v", got)
	}

	if len(got.ConfidenceReasons) != 1 {
		t.Fatalf("expected confidence reasons in analyze response, got %+v", got)
	}

	if len(got.SuggestedAdds) != 1 || len(got.SuggestedCuts) != 1 {
		t.Fatalf("expected suggested adds/cuts in analyze response, got %+v", got)
	}

	if len(got.StructuralTags) != 2 || got.StructuralTags[0] != "thin_early_board" {
		t.Fatalf("expected structural tags in analyze response, got %+v", got)
	}

	if len(got.StructuralTagDetails) != 1 || got.StructuralTagDetails[0].Tag != "thin_early_board" {
		t.Fatalf("expected structural tag details in analyze response, got %+v", got.StructuralTagDetails)
	}

	if len(got.PackageDetails) != 1 || got.PackageDetails[0].Package != "early_board_package" {
		t.Fatalf("expected package details in analyze response, got %+v", got.PackageDetails)
	}

	if len(got.FunctionalRoleSummary) != 1 || got.FunctionalRoleSummary[0].Role != "refill" {
		t.Fatalf("expected functional role summary in analyze response, got %+v", got.FunctionalRoleSummary)
	}
}

func TestRouterComparesDeckCode(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Compare: &stubCompareService{
			result: comparepkg.Result{
				SnapshotID:          "snapshot-1",
				PatchVersion:        "32.4.0",
				Format:              "standard",
				MergedSummary:       []string{"Cycle Rogue supports the refill package concern."},
				MergedSuggestedAdds: []string{"Refill package is light compared to Cycle Rogue, so start by testing its draw-oriented adds."},
				MergedSuggestedCuts: []string{"Late payoff package is heavier than Cycle Rogue, so trim one top end slot first."},
				Candidates: []comparepkg.Candidate{
					{
						DeckID:     "meta-deck-1",
						Name:       "Cycle Rogue",
						Class:      "ROGUE",
						Archetype:  "Combo",
						Similarity: 0.82,
						Breakdown: comparepkg.SimilarityBreakdown{
							Total:    0.82,
							Overlap:  0.78,
							Curve:    0.91,
							CardType: 0.88,
						},
						Summary: []string{"Cycle Rogue has a heavier late-game curve than your deck."},
						SharedCards: []comparepkg.CardDiff{
							{CardID: "CARD_001", Name: "Alpha", InputCount: 2, MetaCount: 2},
						},
						SuggestedAdds: []string{"Add 2x Beta to align with Cycle Rogue."},
					},
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/decks/compare", bytes.NewBufferString(`{"deck_code":"AAEAAA==","limit":3}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got compareDeckResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.SnapshotID != "snapshot-1" || len(got.Candidates) != 1 || got.Candidates[0].DeckID != "meta-deck-1" {
		t.Fatalf("unexpected compare response: %+v", got)
	}

	if len(got.Candidates[0].SharedCards) != 1 || got.Candidates[0].SharedCards[0].CardID != "CARD_001" {
		t.Fatalf("expected compare diff details in response, got %+v", got.Candidates[0].SharedCards)
	}

	if len(got.Candidates[0].SuggestedAdds) != 1 {
		t.Fatalf("expected suggested adds in response, got %+v", got.Candidates[0].SuggestedAdds)
	}

	if len(got.MergedSuggestedAdds) != 1 || len(got.MergedSuggestedCuts) != 1 || len(got.MergedSummary) != 1 {
		t.Fatalf("expected merged compare guidance in response, got %+v", got)
	}

	if len(got.Candidates[0].Summary) != 1 || !strings.Contains(got.Candidates[0].Summary[0], "heavier") {
		t.Fatalf("expected summary in response, got %+v", got.Candidates[0].Summary)
	}

	if got.Candidates[0].Breakdown.Total != 0.82 || got.Candidates[0].Breakdown.Curve != 0.91 {
		t.Fatalf("expected breakdown in response, got %+v", got.Candidates[0].Breakdown)
	}
}

func TestRouterGeneratesAIReport(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Reports: &stubReportsService{
			result: reportResultStub(),
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/reports/generate", bytes.NewBufferString(`{"deck_code":"AAEAAA=="}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got reportGenerateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ReportID != "report-1" || got.Report == "" || got.Model != "gpt-test" || got.Analysis.Archetype != "Aggro" {
		t.Fatalf("unexpected report response: %+v", got)
	}

	if got.Structured == nil || got.Structured.DeckIdentity[0] != "Aggro deck with a low curve" {
		t.Fatalf("expected structured payload in report response, got %+v", got.Structured)
	}

	if got.Compare == nil || got.Compare.SnapshotID != "snapshot-1" || len(got.Compare.Candidates) != 1 {
		t.Fatalf("expected compare payload in report response, got %+v", got.Compare)
	}
}

func TestRouterGeneratesAIReportWithPlainTextFallbackWhenStructuredPayloadIsUnavailable(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Reports: &stubReportsService{
			result: reportpkg.Result{
				ReportID:    "report-fallback",
				Report:      "Plain fallback report",
				Model:       "gpt-test",
				GeneratedAt: time.Date(2026, 3, 27, 2, 0, 0, 0, time.UTC),
				Analysis: analysis.Result{
					Archetype:  "Midrange",
					Confidence: 0.61,
				},
				Structured: nil,
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/reports/generate", bytes.NewBufferString(`{"deck_code":"AAEAAA=="}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got reportGenerateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ReportID != "report-fallback" || got.Report != "Plain fallback report" {
		t.Fatalf("unexpected fallback report response: %+v", got)
	}

	if got.Structured != nil {
		t.Fatalf("expected structured payload to stay nil for fallback response, got %+v", got.Structured)
	}
}

func TestRouterListsReports(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Reports: &stubReportsService{
			listResult: []reportpkg.StoredReport{
				{ID: "report-2", DeckID: "deck-2", ReportType: "ai_deck_report"},
				{ID: "report-1", DeckID: "deck-1", ReportType: "ai_deck_report"},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/reports?limit=2", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got []reportListItemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(got) != 2 || got[0].ID != "report-2" {
		t.Fatalf("unexpected reports list response: %+v", got)
	}
}

func TestRouterGetsReportDetail(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Reports: &stubReportsService{
			detailResult: reportDetailStub(),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/reports/report-1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got reportDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ID != "report-1" || got.Result.ReportID != "report-1" {
		t.Fatalf("unexpected report detail response: %+v", got)
	}

	if got.Result.Report != "AI report body" || got.Result.Analysis.Archetype != "Aggro" {
		t.Fatalf("expected persisted report result in detail response, got %+v", got.Result)
	}
}

func TestRouterGetsReportDetailWithPlainTextFallback(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Reports: &stubReportsService{
			detailResult: reportpkg.ReportDetail{
				ID:         "report-fallback",
				DeckID:     "deck-1",
				ReportType: "ai_deck_report",
				CreatedAt:  time.Date(2026, 3, 27, 3, 0, 0, 0, time.UTC),
				Result: reportpkg.Result{
					ReportID:    "report-fallback",
					Report:      "Plain fallback detail",
					Model:       "gpt-test",
					GeneratedAt: time.Date(2026, 3, 27, 2, 59, 0, 0, time.UTC),
					Analysis: analysis.Result{
						Archetype:  "Midrange",
						Confidence: 0.61,
					},
					Structured: nil,
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/reports/report-fallback", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got reportDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.Result.Report != "Plain fallback detail" {
		t.Fatalf("expected fallback report detail body, got %+v", got.Result)
	}

	if got.Result.Structured != nil {
		t.Fatalf("expected structured payload to be nil for fallback detail, got %+v", got.Result.Structured)
	}
}

func TestRouterListsJobs(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Jobs: &stubJobsService{
			listResult: []jobs.Job{
				{Key: jobs.KeySyncCards, CronExpr: "0 */6 * * *", Enabled: true},
				{Key: jobs.KeySyncMeta, CronExpr: "0 */12 * * *", Enabled: false},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got []jobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(got) != 2 || got[0].Key != jobs.KeySyncCards {
		t.Fatalf("unexpected jobs response: %+v", got)
	}
}

func TestRouterUpdatesJob(t *testing.T) {
	t.Parallel()

	stub := &stubJobsService{
		updateResult: jobs.Job{Key: jobs.KeySyncCards, CronExpr: "*/15 * * * *", Enabled: false},
	}
	handler := NewRouter(testFrontendFS(t), Dependencies{Jobs: stub})

	req := httptest.NewRequest(http.MethodPut, "/api/jobs/sync_cards", bytes.NewBufferString(`{"cron_expr":"*/15 * * * *","enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if stub.lastUpdate.Key != jobs.KeySyncCards || stub.lastUpdate.CronExpr != "*/15 * * * *" || stub.lastUpdate.Enabled {
		t.Fatalf("unexpected update payload: %+v", stub.lastUpdate)
	}
}

func TestRouterRunsJobManually(t *testing.T) {
	t.Parallel()

	stub := &stubJobsService{}
	handler := NewRouter(testFrontendFS(t), Dependencies{Jobs: stub})

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/sync_cards/run", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}

	if stub.lastRunKey != jobs.KeySyncCards {
		t.Fatalf("expected manual run for %q, got %q", jobs.KeySyncCards, stub.lastRunKey)
	}
}

func TestRouterListsJobHistory(t *testing.T) {
	t.Parallel()

	startedAt := strings.TrimSpace("2026-03-25T10:00:00Z")
	started, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		t.Fatalf("time.Parse() error = %v", err)
	}

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Jobs: &stubJobsService{
			historyResult: []jobs.Execution{
				{JobKey: jobs.KeySyncCards, Status: jobs.StatusSuccess, StartedAt: started},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/jobs/sync_cards/history?limit=5", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got []jobExecutionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(got) != 1 || got[0].JobKey != jobs.KeySyncCards || got[0].Status != jobs.StatusSuccess {
		t.Fatalf("unexpected history response: %+v", got)
	}
}

func TestRouterGetsLatestMetaSnapshot(t *testing.T) {
	t.Parallel()

	fetchedAt := time.Date(2026, 3, 26, 8, 0, 0, 0, time.UTC)
	handler := NewRouter(testFrontendFS(t), Dependencies{
		Meta: &stubMetaService{
			latestResult: meta.Snapshot{
				ID:           "meta_123",
				Source:       "stub",
				PatchVersion: "32.0.1",
				Format:       "standard",
				FetchedAt:    fetchedAt,
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/meta/latest?format=standard", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got metaSnapshotResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ID != "meta_123" || got.Format != "standard" || got.Source != "stub" {
		t.Fatalf("unexpected meta snapshot response: %+v", got)
	}
}

func TestRouterReturnsMetaSnapshotNotFound(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Meta: &stubMetaService{
			latestErr: errors.New("meta snapshot not found"),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/meta/latest?format=standard", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestRouterListsMetaSnapshots(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Meta: &stubMetaService{
			listResult: []meta.Snapshot{
				{
					ID:           "meta_2",
					Source:       "remote",
					PatchVersion: "32.0.2",
					Format:       "standard",
					FetchedAt:    time.Date(2026, 3, 26, 8, 0, 0, 0, time.UTC),
				},
				{
					ID:           "meta_1",
					Source:       "fixture",
					PatchVersion: "32.0.1",
					Format:       "standard",
					FetchedAt:    time.Date(2026, 3, 25, 8, 0, 0, 0, time.UTC),
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/meta?format=standard&limit=2", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got []metaSnapshotResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(got) != 2 || got[0].ID != "meta_2" || got[1].ID != "meta_1" {
		t.Fatalf("unexpected meta snapshot list response: %+v", got)
	}
}

func TestRouterGetsMetaSnapshotByID(t *testing.T) {
	t.Parallel()

	handler := NewRouter(testFrontendFS(t), Dependencies{
		Meta: &stubMetaService{
			detailResult: meta.SnapshotDetail{
				Snapshot: meta.Snapshot{
					ID:           "meta_detail",
					Source:       "remote",
					PatchVersion: "32.4.0",
					Format:       "standard",
					FetchedAt:    time.Date(2026, 3, 26, 11, 30, 0, 0, time.UTC),
				},
				RawPayload: `{"decks":[{"name":"Cycle Rogue"}]}`,
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/meta/meta_detail", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got metaSnapshotDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got.ID != "meta_detail" || got.RawPayload == "" {
		t.Fatalf("unexpected meta snapshot detail response: %+v", got)
	}
}

func testFrontendFS(t *testing.T) fs.FS {
	t.Helper()

	return fstest.MapFS{
		"dist/index.html": &fstest.MapFile{
			Data: []byte("<!doctype html><html><body><div id=\"app\"></div></body></html>"),
		},
		"dist/assets/app.js": &fstest.MapFile{
			Data: []byte("console.log('boot');"),
		},
	}
}

type stubSettingsService struct {
	listResult []settings.Setting
	getResult  settings.Setting
	listErr    error
	getErr     error
	upsertErr  error
	lastUpsert settings.Input
}

func (s *stubSettingsService) List(ctx context.Context) ([]settings.Setting, error) {
	return s.listResult, s.listErr
}

func (s *stubSettingsService) Get(ctx context.Context, key string) (settings.Setting, error) {
	return s.getResult, s.getErr
}

func (s *stubSettingsService) Upsert(ctx context.Context, input settings.Input) error {
	s.lastUpsert = input
	return s.upsertErr
}

type stubCardsService struct {
	listResult []cards.Summary
	getResult  cards.Summary
	listErr    error
	getErr     error
	lastFilter cards.ListFilter
}

func (s *stubCardsService) List(ctx context.Context, filter cards.ListFilter) ([]cards.Summary, error) {
	s.lastFilter = filter
	return s.listResult, s.listErr
}

func (s *stubCardsService) GetByID(ctx context.Context, id string) (cards.Summary, error) {
	return s.getResult, s.getErr
}

type stubDecksService struct {
	parseResult decks.ParseResult
	parseErr    error
}

func (s *stubDecksService) Parse(ctx context.Context, deckCode string) (decks.ParseResult, error) {
	return s.parseResult, s.parseErr
}

type stubAnalysisService struct {
	result analysis.Result
	err    error
}

func (s *stubAnalysisService) AnalyzeDeck(ctx context.Context, deckCode string) (analysis.Result, error) {
	return s.result, s.err
}

type stubCompareService struct {
	result comparepkg.Result
	err    error
}

func (s *stubCompareService) CompareDeck(ctx context.Context, deckCode string, limit int) (comparepkg.Result, error) {
	return s.result, s.err
}

type stubReportsService struct {
	result       reportpkg.Result
	listResult   []reportpkg.StoredReport
	detailResult reportpkg.ReportDetail
	err          error
}

func (s *stubReportsService) GenerateDeckReport(ctx context.Context, deckCode string, language string) (reportpkg.Result, error) {
	return s.result, s.err
}

func (s *stubReportsService) ListReports(ctx context.Context, limit int) ([]reportpkg.StoredReport, error) {
	return s.listResult, s.err
}

func (s *stubReportsService) GetReport(ctx context.Context, id string) (reportpkg.ReportDetail, error) {
	return s.detailResult, s.err
}

type stubJobsService struct {
	listResult    []jobs.Job
	getResult     jobs.Job
	updateResult  jobs.Job
	historyResult []jobs.Execution
	listErr       error
	getErr        error
	updateErr     error
	runErr        error
	historyErr    error
	lastUpdate    jobs.UpdateInput
	lastRunKey    string
}

func (s *stubJobsService) List(ctx context.Context) ([]jobs.Job, error) {
	return s.listResult, s.listErr
}

func (s *stubJobsService) Get(ctx context.Context, key string) (jobs.Job, error) {
	return s.getResult, s.getErr
}

func (s *stubJobsService) Update(ctx context.Context, input jobs.UpdateInput) (jobs.Job, error) {
	s.lastUpdate = input
	return s.updateResult, s.updateErr
}

func (s *stubJobsService) RunNow(ctx context.Context, key string) error {
	s.lastRunKey = key
	return s.runErr
}

func (s *stubJobsService) History(ctx context.Context, key string, limit int) ([]jobs.Execution, error) {
	return s.historyResult, s.historyErr
}

type stubMetaService struct {
	latestResult meta.Snapshot
	listResult   []meta.Snapshot
	detailResult meta.SnapshotDetail
	latestErr    error
	listErr      error
	detailErr    error
	lastFormat   string
	lastLimit    int
	lastID       string
}

func (s *stubMetaService) GetLatestSnapshot(ctx context.Context, format string) (meta.Snapshot, error) {
	s.lastFormat = format
	return s.latestResult, s.latestErr
}

func (s *stubMetaService) ListSnapshots(ctx context.Context, format string, limit int) ([]meta.Snapshot, error) {
	s.lastFormat = format
	s.lastLimit = limit
	return s.listResult, s.listErr
}

func (s *stubMetaService) GetSnapshotByID(ctx context.Context, id string) (meta.SnapshotDetail, error) {
	s.lastID = id
	return s.detailResult, s.detailErr
}

func reportResultStub() reportpkg.Result {
	return reportpkg.Result{
		ReportID:    "report-1",
		Report:      "AI report body",
		Model:       "gpt-test",
		GeneratedAt: time.Date(2026, 3, 26, 20, 0, 0, 0, time.UTC),
		Analysis: analysis.Result{
			Archetype:         "Aggro",
			Confidence:        0.84,
			ConfidenceReasons: []string{"Strong early curve concentration supports an aggro read."},
		},
		Structured: &reportpkg.StructuredReport{
			DeckIdentity:             []string{"Aggro deck with a low curve"},
			WhatTheDeckIsDoingWell:   []string{"Strong early tempo"},
			MainRisks:                []string{"Can run out of resources"},
			PracticalNextAdjustments: []string{"Consider one more draw card"},
		},
		Compare: &comparepkg.Result{
			SnapshotID:   "snapshot-1",
			PatchVersion: "32.4.0",
			Format:       "standard",
			Candidates: []comparepkg.Candidate{
				{
					DeckID:     "meta-deck-1",
					Name:       "Cycle Rogue",
					Class:      "ROGUE",
					Similarity: 0.82,
					Tier:       stringPtr("T1"),
					Summary:    []string{"Cycle Rogue has a heavier late-game curve than your deck."},
				},
			},
		},
	}
}

func reportDetailStub() reportpkg.ReportDetail {
	return reportpkg.ReportDetail{
		ID:                "report-1",
		DeckID:            "deck-1",
		BasedOnSnapshotID: stringPtr("snapshot-1"),
		ReportType:        "ai_deck_report",
		CreatedAt:         time.Date(2026, 3, 26, 20, 1, 0, 0, time.UTC),
		Result:            reportResultStub(),
	}
}

func stringPtr(v string) *string {
	return &v
}
