import test from "node:test";
import assert from "node:assert/strict";

import {
  findPreferredReportID,
  hydrateReportResult,
  hasCompareGuidance,
  normalizeCompareGuidance,
  sanitizeStructuredReport,
  summarizeGuidanceSupport,
  summarizeReportCompare,
  summarizeReportCompareCandidate,
  summarizeStoredReport
} from "./reportsViewModel.js";

test("findPreferredReportID keeps the selected report when still present", () => {
  const reports = [{ id: "report-2" }, { id: "report-1" }];

  assert.equal(findPreferredReportID(reports, "report-1"), "report-1");
});

test("findPreferredReportID falls back to the newest report", () => {
  const reports = [{ id: "report-2" }, { id: "report-1" }];

  assert.equal(findPreferredReportID(reports, "missing"), "report-2");
});

test("hydrateReportResult injects the persisted report id for replay", () => {
  const hydrated = hydrateReportResult({
    id: "report-1",
    result: {
      report: "Stored body",
      model: "gpt-test",
      generated_at: "2026-03-27T01:00:00Z",
      analysis: { archetype: "Aggro" }
    }
  });

  assert.equal(hydrated.report_id, "report-1");
  assert.equal(hydrated.report, "Stored body");
});

test("hydrateReportResult keeps plain fallback reports without structured payload", () => {
  const hydrated = hydrateReportResult({
    id: "report-fallback",
    result: {
      report: "Plain fallback body",
      model: "gpt-test",
      generated_at: "2026-03-27T01:00:00Z",
      analysis: { archetype: "Midrange" },
      structured: null
    }
  });

  assert.equal(hydrated.report_id, "report-fallback");
  assert.equal(hydrated.structured, null);
});

test("hydrateReportResult normalizes duplicated structured replay items", () => {
  const hydrated = hydrateReportResult({
    id: "report-1",
    result: {
      report: "Stored body",
      model: "gpt-test",
      generated_at: "2026-03-27T01:00:00Z",
      analysis: { archetype: "Aggro" },
      structured: {
        deck_identity: ["Aggro deck", "Aggro deck"],
        what_the_deck_is_doing_well: ["Fast pressure"],
        main_risks: ["Low refill"],
        practical_next_adjustments: ["Add draw"]
      }
    }
  });

  assert.deepEqual(hydrated.structured.deck_identity, ["Aggro deck"]);
});

test("hydrateReportResult drops invalid structured replay payloads", () => {
  const hydrated = hydrateReportResult({
    id: "report-1",
    result: {
      report: "Stored body",
      model: "gpt-test",
      generated_at: "2026-03-27T01:00:00Z",
      analysis: { archetype: "Aggro" },
      structured: {
        deck_identity: ["A", "B", "C", "D", "E", "F"],
        what_the_deck_is_doing_well: ["Fast pressure"],
        main_risks: ["Low refill"],
        practical_next_adjustments: ["Add draw"]
      }
    }
  });

  assert.equal(hydrated.structured, null);
});

test("sanitizeStructuredReport rejects overlong items", () => {
  const sanitized = sanitizeStructuredReport({
    deck_identity: ["x".repeat(241)],
    what_the_deck_is_doing_well: ["Fast pressure"],
    main_risks: ["Low refill"],
    practical_next_adjustments: ["Add draw"]
  });

  assert.equal(sanitized, null);
});

test("summarizeStoredReport renders time and type", () => {
  const summary = summarizeStoredReport({
    created_at: "2026-03-27T01:00:00Z",
    report_type: "ai_deck_report"
  });

  assert.match(summary, /2026/);
  assert.match(summary, /ai_deck_report/);
});

test("summarizeReportCompare renders snapshot metadata", () => {
  const summary = summarizeReportCompare({
    snapshot_id: "snapshot-1",
    patch_version: "32.4.0",
    format: "standard"
  });

  assert.match(summary, /snapshot-1/);
  assert.match(summary, /32.4.0/);
  assert.match(summary, /standard/);
});

test("summarizeReportCompare renders zh locale copy", () => {
  const summary = summarizeReportCompare({
    snapshot_id: "snapshot-1",
    patch_version: "32.4.0",
    format: "standard"
  }, undefined, "zh");

  assert.match(summary, /版本/);
});

test("summarizeReportCompareCandidate renders similarity and tier", () => {
  const summary = summarizeReportCompareCandidate({
    name: "Cycle Rogue",
    similarity: 0.823,
    tier: "T1"
  });

  assert.match(summary, /Cycle Rogue/);
  assert.match(summary, /82.3%/);
  assert.match(summary, /T1/);
});

test("normalizeCompareGuidance prefers structured merged guidance", () => {
  const normalized = normalizeCompareGuidance({
    merged_guidance: {
      adds: [
        {
          key: "add_refill",
          kind: "add",
          package: "refill_package",
          source: "multi_candidate_consensus",
          message: "Multiple close meta decks support adding refill first.",
          confidence: 0.84,
          support: [
            {
              source: "analysis_package_gap",
              evidence: "Refill package is underbuilt at 0 slots against a 3-6 target."
            }
          ]
        }
      ]
    },
    merged_suggested_adds: ["legacy add string"]
  });

  assert.equal(normalized.adds.length, 1);
  assert.equal(normalized.adds[0].source, "multi_candidate_consensus");
  assert.equal(normalized.adds[0].confidence, 0.84);
  assert.equal(normalized.adds[0].support[0].evidence, "Refill package is underbuilt at 0 slots against a 3-6 target.");
});

test("normalizeCompareGuidance falls back to legacy merged strings", () => {
  const normalized = normalizeCompareGuidance({
    merged_summary: ["Top candidates are split."],
    merged_suggested_adds: ["Add refill first."],
    merged_suggested_cuts: ["Trim the top end."]
  });

  assert.equal(normalized.summary[0].source, "legacy_string");
  assert.equal(normalized.adds[0].message, "Add refill first.");
  assert.equal(normalized.cuts[0].message, "Trim the top end.");
});

test("hasCompareGuidance returns true when merged guidance exists", () => {
  assert.equal(hasCompareGuidance({
    merged_guidance: {
      summary: [{ message: "Structured guidance is present." }]
    }
  }), true);
});

test("summarizeGuidanceSupport returns the first support evidence", () => {
  const summary = summarizeGuidanceSupport({
    support: [
      { evidence: "Primary support line." },
      { evidence: "Secondary support line." }
    ]
  });

  assert.equal(summary, "Primary support line.");
});

