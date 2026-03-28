package report

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hearthstone-analyzer/internal/analysis"
	comparepkg "hearthstone-analyzer/internal/compare"
)

func TestOpenAICompatibleProviderSendsChatCompletionRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions path, got %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("expected auth header, got %q", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload["model"] != "gpt-test" {
			t.Fatalf("expected model gpt-test, got %+v", payload)
		}

		messages, ok := payload["messages"].([]any)
		if !ok || len(messages) != 2 {
			t.Fatalf("expected two chat messages, got %+v", payload["messages"])
		}

		systemMessage, ok := messages[0].(map[string]any)
		if !ok {
			t.Fatalf("expected system message object, got %+v", messages[0])
		}

		systemContent, _ := systemMessage["content"].(string)
		if !strings.Contains(systemContent, "Use only provided facts") || !strings.Contains(systemContent, "If compare data is unavailable") {
			t.Fatalf("expected stronger no-fabrication system guardrails, got %q", systemContent)
		}

		_, _ = w.Write([]byte(`{
  "model": "gpt-test",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "{\"deck_identity\":[\"Aggro deck with a low curve\"],\"what_the_deck_is_doing_well\":[\"Strong early tempo\"],\"main_risks\":[\"Can run out of resources\"],\"practical_next_adjustments\":[\"Consider one more draw card\"]}"
      }
    }
  ]
}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(server.Client())
	got, err := provider.GenerateReport(context.Background(), PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{
			Archetype:         "Aggro",
			Confidence:        0.8,
			ConfidenceReasons: []string{"Strong early curve concentration supports an aggro read."},
		},
	}, ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "secret",
		Model:   "gpt-test",
	})
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}

	if got.Model != "gpt-test" {
		t.Fatalf("unexpected provider result: %+v", got)
	}

	if got.Structured == nil || len(got.Structured.DeckIdentity) != 1 {
		t.Fatalf("expected structured report result, got %+v", got.Structured)
	}

	if !strings.Contains(got.Content, "## Deck Identity") || !strings.Contains(got.Content, "## Practical Next Adjustments") {
		t.Fatalf("expected markdown report formatted from structured data, got %q", got.Content)
	}
}

func TestBuildPromptMentionsMissingCompareContext(t *testing.T) {
	t.Parallel()

	prompt := buildPrompt(PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{
			Archetype:         "Midrange",
			Confidence:        0.55,
			ConfidenceReasons: []string{"Confidence is driven mostly by general curve shape rather than one overwhelming archetype signal."},
		},
	})

	if !strings.Contains(prompt, "No meta comparison data was available") {
		t.Fatalf("expected prompt to mention missing compare context, got %q", prompt)
	}

	if !strings.Contains(prompt, "Do not claim any closest meta deck, snapshot, matchup, winrate, or tier data") {
		t.Fatalf("expected prompt to forbid invented compare details, got %q", prompt)
	}
}

func TestBuildPromptShapesOutputIntoGroundedSections(t *testing.T) {
	t.Parallel()

	prompt := buildPrompt(PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{
			Archetype:  "Aggro",
			Confidence: 0.81,
			Strengths:  []string{"Fast early curve"},
			Weaknesses: []string{"Can run out of cards"},
		},
	})

	if !strings.Contains(prompt, "Output rules:") {
		t.Fatalf("expected prompt to include explicit output rules, got %q", prompt)
	}

	if !strings.Contains(prompt, "If a fact is not present above, say it is unavailable") {
		t.Fatalf("expected prompt to require explicit uncertainty, got %q", prompt)
	}

	if !strings.Contains(prompt, "1. Deck identity") || !strings.Contains(prompt, "4. Practical next adjustments") {
		t.Fatalf("expected prompt to keep the sectioned output shape, got %q", prompt)
	}

	if !strings.Contains(prompt, "Return JSON only") || !strings.Contains(prompt, "\"deck_identity\"") {
		t.Fatalf("expected prompt to request structured JSON output, got %q", prompt)
	}
}

func TestBuildPromptIncludesStructuredCompareGuidance(t *testing.T) {
	t.Parallel()

	prompt := buildPrompt(PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{
			Archetype:  "Aggro",
			Confidence: 0.81,
		},
		Compare: &comparepkg.Result{
			Candidates: []comparepkg.Candidate{{Name: "Cycle Rogue", Similarity: 0.77}},
			MergedGuidance: comparepkg.StructuredGuidance{
				Adds: []comparepkg.Recommendation{
					{
						Key:        "add_refill",
						Kind:       "add",
						Package:    "refill_package",
						Source:     "multi_candidate_consensus",
						Message:    "Multiple close meta decks support adding refill first.",
						Confidence: 0.84,
						Support: []comparepkg.RecommendationSupport{
							{Source: "analysis_package_gap", Evidence: "Refill package is underbuilt at 0 slots against a 3-6 target."},
						},
					},
				},
			},
		},
	})

	if !strings.Contains(prompt, "confidence 0.84") || !strings.Contains(prompt, "Support: Refill package is underbuilt") {
		t.Fatalf("expected prompt to include structured guidance support/confidence, got %q", prompt)
	}
}

func TestParseStructuredReportParsesJSONString(t *testing.T) {
	t.Parallel()

	got, err := parseStructuredReport(`{"deck_identity":["Aggro deck"],"what_the_deck_is_doing_well":["Fast pressure"],"main_risks":["Low refill"],"practical_next_adjustments":["Add card draw"]}`)
	if err != nil {
		t.Fatalf("parseStructuredReport() error = %v", err)
	}

	if got.DeckIdentity[0] != "Aggro deck" || got.PracticalNextAdjustments[0] != "Add card draw" {
		t.Fatalf("unexpected structured report: %+v", got)
	}
}

func TestParseStructuredReportFallsBackToSingleStringValues(t *testing.T) {
	t.Parallel()

	got, err := parseStructuredReport(`{"deck_identity":"Aggro deck","what_the_deck_is_doing_well":"Fast pressure","main_risks":"Low refill","practical_next_adjustments":"Add card draw"}`)
	if err != nil {
		t.Fatalf("parseStructuredReport() error = %v", err)
	}

	if len(got.MainRisks) != 1 || got.MainRisks[0] != "Low refill" {
		t.Fatalf("expected string fields to normalize to arrays, got %+v", got)
	}
}

func TestParseStructuredReportRejectsSparseStructuredPayload(t *testing.T) {
	t.Parallel()

	if _, err := parseStructuredReport(`{"deck_identity":["Aggro deck"]}`); err == nil {
		t.Fatal("expected sparse structured payload to be rejected")
	}
}

func TestFormatStructuredReportProducesStableMarkdownSections(t *testing.T) {
	t.Parallel()

	formatted := formatStructuredReport(StructuredReport{
		DeckIdentity:             []string{"Aggro deck"},
		WhatTheDeckIsDoingWell:   []string{"Fast pressure"},
		MainRisks:                []string{"Low refill"},
		PracticalNextAdjustments: []string{"Add card draw"},
	})

	if !strings.Contains(formatted, "## Deck Identity") || !strings.Contains(formatted, "- Aggro deck") {
		t.Fatalf("expected formatted markdown sections, got %q", formatted)
	}
}

func TestOpenAICompatibleProviderFallsBackToRawTextWhenStructuredParseFails(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
  "model": "gpt-test",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "Plain fallback report"
      }
    }
  ]
}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(server.Client())
	got, err := provider.GenerateReport(context.Background(), PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{Archetype: "Aggro", Confidence: 0.8},
	}, ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "secret",
		Model:   "gpt-test",
	})
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}

	if got.Content != "Plain fallback report" || got.Structured != nil {
		t.Fatalf("expected raw fallback content, got %+v", got)
	}
}

func TestOpenAICompatibleProviderFallsBackWhenStructuredPayloadIsTooThin(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
  "model": "gpt-test",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "{\"deck_identity\":[\"Aggro deck\"]}"
      }
    }
  ]
}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(server.Client())
	got, err := provider.GenerateReport(context.Background(), PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{Archetype: "Aggro", Confidence: 0.8},
	}, ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "secret",
		Model:   "gpt-test",
	})
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}

	if got.Structured != nil || !strings.Contains(got.Content, `"deck_identity"`) {
		t.Fatalf("expected thin structured payload to fall back to raw content, got %+v", got)
	}
}

func TestValidateStructuredReportRejectsCompareClaimsWithoutCompareContext(t *testing.T) {
	t.Parallel()

	err := validateStructuredReport(StructuredReport{
		DeckIdentity:             []string{"Aggro deck"},
		WhatTheDeckIsDoingWell:   []string{"Strong early pressure"},
		MainRisks:                []string{"This list has no clear closest meta deck because snapshot data is unavailable."},
		PracticalNextAdjustments: []string{"Tighten the early curve"},
	}, PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{Archetype: "Aggro", Confidence: 0.8},
	})
	if err == nil {
		t.Fatal("expected compare-only claim to be rejected without compare context")
	}
}

func TestValidateStructuredReportRejectsUngroundedPercentages(t *testing.T) {
	t.Parallel()

	err := validateStructuredReport(StructuredReport{
		DeckIdentity:             []string{"Aggro deck"},
		WhatTheDeckIsDoingWell:   []string{"Strong early pressure"},
		MainRisks:                []string{"The deck only has a 60% matchup spread."},
		PracticalNextAdjustments: []string{"Tighten the early curve"},
	}, PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{Archetype: "Aggro", Confidence: 0.8},
	})
	if err == nil {
		t.Fatal("expected unsupported percentage claim to be rejected")
	}
}

func TestValidateStructuredReportAllowsGroundedSimilarityReferences(t *testing.T) {
	t.Parallel()

	err := validateStructuredReport(StructuredReport{
		DeckIdentity:             []string{"Aggro deck"},
		WhatTheDeckIsDoingWell:   []string{"Strong early pressure"},
		MainRisks:                []string{"Similarity to the closest candidate is high, so many core slots already line up."},
		PracticalNextAdjustments: []string{"Use the similarity diff to guide a few final flex-slot changes."},
	}, PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{Archetype: "Aggro", Confidence: 0.8},
		Compare: &comparepkg.Result{
			Candidates: []comparepkg.Candidate{
				{Name: "Cycle Rogue", Similarity: 0.77},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected grounded similarity reference to be allowed, got %v", err)
	}
}

func TestValidateStructuredReportRejectsWinrateClaimsWhenCompareDataLacksWinrate(t *testing.T) {
	t.Parallel()

	err := validateStructuredReport(StructuredReport{
		DeckIdentity:             []string{"Aggro deck"},
		WhatTheDeckIsDoingWell:   []string{"Strong early pressure"},
		MainRisks:                []string{"The closest candidate has a lower winrate in the current field."},
		PracticalNextAdjustments: []string{"Keep the proactive shell"},
	}, PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{Archetype: "Aggro", Confidence: 0.8},
		Compare: &comparepkg.Result{
			SnapshotID: "snapshot-1",
			Candidates: []comparepkg.Candidate{
				{Name: "Cycle Rogue", Similarity: 0.77},
			},
		},
	})
	if err == nil {
		t.Fatal("expected winrate claim without grounded winrate data to be rejected")
	}
}

func TestValidateStructuredReportRejectsSnapshotClaimsWhenCompareDataLacksSnapshotID(t *testing.T) {
	t.Parallel()

	err := validateStructuredReport(StructuredReport{
		DeckIdentity:             []string{"Aggro deck"},
		WhatTheDeckIsDoingWell:   []string{"Strong early pressure"},
		MainRisks:                []string{"The latest snapshot shows this shell getting pushed out."},
		PracticalNextAdjustments: []string{"Keep the proactive shell"},
	}, PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{Archetype: "Aggro", Confidence: 0.8},
		Compare: &comparepkg.Result{
			Candidates: []comparepkg.Candidate{
				{Name: "Cycle Rogue", Similarity: 0.77},
			},
		},
	})
	if err == nil {
		t.Fatal("expected snapshot claim without snapshot id to be rejected")
	}
}

func TestValidateStructuredReportAllowsTierClaimsWhenTierDataExists(t *testing.T) {
	t.Parallel()

	err := validateStructuredReport(StructuredReport{
		DeckIdentity:             []string{"Aggro deck"},
		WhatTheDeckIsDoingWell:   []string{"Strong early pressure"},
		MainRisks:                []string{"The closest candidate sits in Tier 1, so your current shell is already near a proven ladder core."},
		PracticalNextAdjustments: []string{"Use the remaining flex slots for card draw"},
	}, PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{Archetype: "Aggro", Confidence: 0.8},
		Compare: &comparepkg.Result{
			SnapshotID: "snapshot-1",
			Candidates: []comparepkg.Candidate{
				{Name: "Cycle Rogue", Similarity: 0.77, Tier: compareStringPtr("T1")},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected grounded tier reference to be allowed, got %v", err)
	}
}

func TestParseStructuredReportDeduplicatesRepeatedItems(t *testing.T) {
	t.Parallel()

	got, err := parseStructuredReport(`{"deck_identity":["Aggro deck","Aggro deck"],"what_the_deck_is_doing_well":["Fast pressure","Fast pressure"],"main_risks":["Low refill"],"practical_next_adjustments":["Add draw"]}`)
	if err != nil {
		t.Fatalf("parseStructuredReport() error = %v", err)
	}

	if len(got.DeckIdentity) != 1 || len(got.WhatTheDeckIsDoingWell) != 1 {
		t.Fatalf("expected duplicate items to be deduplicated, got %+v", got)
	}
}

func TestParseStructuredReportRejectsSectionWithTooManyItems(t *testing.T) {
	t.Parallel()

	if _, err := parseStructuredReport(`{"deck_identity":["A","B","C","D","E","F"],"what_the_deck_is_doing_well":["Fast pressure"],"main_risks":["Low refill"],"practical_next_adjustments":["Add draw"]}`); err == nil {
		t.Fatal("expected oversized section to be rejected")
	}
}

func TestParseStructuredReportRejectsOverlongItems(t *testing.T) {
	t.Parallel()

	longItem := strings.Repeat("x", 241)
	payload := `{"deck_identity":["` + longItem + `"],"what_the_deck_is_doing_well":["Fast pressure"],"main_risks":["Low refill"],"practical_next_adjustments":["Add draw"]}`
	if _, err := parseStructuredReport(payload); err == nil {
		t.Fatal("expected overlong item to be rejected")
	}
}

func TestOpenAICompatibleProviderFallsBackWhenStructuredPayloadContainsUngroundedCompareClaims(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
  "model": "gpt-test",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "{\"deck_identity\":[\"Aggro deck\"],\"what_the_deck_is_doing_well\":[\"Fast pressure\"],\"main_risks\":[\"This deck is Tier 1 in the latest snapshot.\"],\"practical_next_adjustments\":[\"Add one more draw card\"]}"
      }
    }
  ]
}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(server.Client())
	got, err := provider.GenerateReport(context.Background(), PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{Archetype: "Aggro", Confidence: 0.8},
	}, ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "secret",
		Model:   "gpt-test",
	})
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}

	if got.Structured != nil {
		t.Fatalf("expected invalid structured payload to fall back, got %+v", got.Structured)
	}

	if !strings.Contains(got.Content, "Tier 1 in the latest snapshot") {
		t.Fatalf("expected raw fallback content to be preserved, got %q", got.Content)
	}
}

func TestOpenAICompatibleProviderFallsBackWhenStructuredPayloadHasTooManyItems(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
  "model": "gpt-test",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "{\"deck_identity\":[\"Aggro deck\",\"Tempo shell\",\"Low-curve opener\",\"Board-focused plan\",\"Pressure-first list\",\"Burst finish\"],\"what_the_deck_is_doing_well\":[\"Fast pressure\"],\"main_risks\":[\"Low refill\"],\"practical_next_adjustments\":[\"Add draw\"]}"
      }
    }
  ]
}`))
	}))
	defer server.Close()

	provider := NewOpenAICompatibleProvider(server.Client())
	got, err := provider.GenerateReport(context.Background(), PromptInput{
		DeckCode: "AAEAAA==",
		Analysis: analysis.Result{Archetype: "Aggro", Confidence: 0.8},
	}, ProviderConfig{
		BaseURL: server.URL,
		APIKey:  "secret",
		Model:   "gpt-test",
	})
	if err != nil {
		t.Fatalf("GenerateReport() error = %v", err)
	}

	if got.Structured != nil {
		t.Fatalf("expected oversized structured payload to fall back, got %+v", got.Structured)
	}

	if !strings.Contains(got.Content, `"Burst finish"`) {
		t.Fatalf("expected raw fallback content to be preserved, got %q", got.Content)
	}
}

func compareStringPtr(v string) *string {
	return &v
}
