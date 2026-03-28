import test from "node:test";
import assert from "node:assert/strict";

import {
  didHistoryAdvance,
  extractMetaDeckRows,
  formatExecutionStatus,
  formatExecutionDuration,
  formatJobError,
  formatMetaPayloadPreview,
  formatMetaStatus,
  summarizeMetaSnapshot,
  formatTimestamp,
  mapJob,
  summarizeLatestRun
} from "./jobsViewModel.js";

test("mapJob creates editable draft fields", () => {
  const job = mapJob({
    key: "sync_cards",
    cron_expr: "*/15 * * * *",
    enabled: true,
    next_run_at: "2026-03-25T10:15:00Z",
    last_run_at: null
  });

  assert.equal(job.key, "sync_cards");
  assert.equal(job.draftCronExpr, "*/15 * * * *");
  assert.equal(job.draftEnabled, true);
  assert.deepEqual(job.history, []);
});

test("summarizeLatestRun prefers latest history item", () => {
  const summary = summarizeLatestRun({
    history: [
      { status: "success", started_at: "2026-03-25T10:15:00Z" }
    ]
  });

  assert.match(summary, /Success/i);
  assert.match(summary, /2026/);
});

test("summarizeLatestRun falls back when no history exists", () => {
  assert.equal(summarizeLatestRun({ history: [] }), "No runs yet");
});

test("formatExecutionStatus renders readable labels", () => {
  assert.equal(formatExecutionStatus("success"), "Success");
  assert.equal(formatExecutionStatus("failed"), "Failed");
  assert.equal(formatExecutionStatus("queued"), "Queued");
});

test("formatExecutionStatus supports zh locale", () => {
  assert.equal(formatExecutionStatus("success", "zh"), "成功");
  assert.equal(formatExecutionStatus("failed", "zh"), "失敗");
});

test("formatTimestamp returns placeholder for empty values", () => {
  assert.equal(formatTimestamp(""), "Not scheduled");
});

test("didHistoryAdvance detects a newly added execution", () => {
  assert.equal(
    didHistoryAdvance(
      [{ id: 2, started_at: "2026-03-25T10:15:00Z" }],
      [{ id: 1, started_at: "2026-03-25T10:00:00Z" }]
    ),
    true
  );
});

test("didHistoryAdvance detects unchanged execution history", () => {
  assert.equal(
    didHistoryAdvance(
      [{ id: 1, started_at: "2026-03-25T10:00:00Z" }],
      [{ id: 1, started_at: "2026-03-25T10:00:00Z" }]
    ),
    false
  );
});

test("formatJobError returns friendly duplicate-run message", () => {
  assert.equal(
    formatJobError("job already running"),
    "This job is already running. Wait for the current execution to finish and try again."
  );
});

test("formatJobError falls back to original message", () => {
  assert.equal(formatJobError("source timeout"), "source timeout");
});

test("formatExecutionDuration renders seconds for short runs", () => {
  assert.equal(
    formatExecutionDuration("2026-03-25T10:00:00Z", "2026-03-25T10:00:05Z"),
    "5s"
  );
});

test("formatExecutionDuration returns placeholder when timestamps are missing", () => {
  assert.equal(formatExecutionDuration("2026-03-25T10:00:00Z", ""), "In progress");
});

test("formatMetaStatus reports missing snapshot clearly", () => {
  assert.equal(
    formatMetaStatus(null, "Meta snapshot not available yet"),
    "Meta snapshot not available yet"
  );
});

test("formatMetaStatus summarizes snapshot details", () => {
  assert.match(
    formatMetaStatus({ source: "fixture", patch_version: "32.1.0", format: "standard" }, ""),
    /fixture/i
  );
});

test("summarizeMetaSnapshot includes source patch and fetched time", () => {
  const summary = summarizeMetaSnapshot({
    source: "remote",
    patch_version: "32.4.0",
    format: "standard",
    fetched_at: "2026-03-26T11:30:00Z"
  });

  assert.match(summary, /remote/i);
  assert.match(summary, /32.4.0/);
  assert.match(summary, /2026/);
});

test("formatMetaPayloadPreview truncates long payloads", () => {
  const preview = formatMetaPayloadPreview("x".repeat(260));

  assert.equal(preview.length, 243);
  assert.match(preview, /\.\.\.$/);
});

test("formatMetaPayloadPreview uses placeholder for empty payload", () => {
  assert.equal(formatMetaPayloadPreview(""), "No raw payload available");
});

test("extractMetaDeckRows returns sorted deck rows from payload", () => {
  const rows = extractMetaDeckRows(`{
    "decks": [
      { "name": "Control Warrior", "class": "WARRIOR", "playrate": 0.08, "winrate": 0.54, "tier": "T2" },
      { "name": "Cycle Rogue", "class": "ROGUE", "playrate": 0.12, "winrate": 0.51, "tier": "T1" }
    ]
  }`);

  assert.equal(rows.length, 2);
  assert.equal(rows[0].name, "Cycle Rogue");
  assert.equal(rows[0].playrateLabel, "12.0%");
  assert.equal(rows[0].winrateLabel, "51.0%");
});

test("extractMetaDeckRows falls back to sample size sorting when rates are missing", () => {
  const rows = extractMetaDeckRows(`{
    "decks": [
      { "name": "Deck A", "sample_size": 800 },
      { "name": "Deck B", "sample_size": 1200 }
    ]
  }`);

  assert.equal(rows[0].name, "Deck B");
  assert.equal(rows[0].sampleSizeLabel, "1,200");
});

test("extractMetaDeckRows returns empty list for invalid payload", () => {
  assert.deepEqual(extractMetaDeckRows("not json"), []);
});

