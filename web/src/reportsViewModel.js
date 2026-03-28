import { formatTimestamp } from "./jobsViewModel.js";

const MAX_STRUCTURED_SECTION_ITEMS = 5;
const MAX_STRUCTURED_ITEM_LENGTH = 240;
const MAX_GUIDANCE_ITEMS = 3;

export function summarizeStoredReport(report, fallback = "Unknown report", locale = "en") {
  if (!report) {
    return fallback;
  }

  return `${formatTimestamp(report.created_at, locale === "zh" ? "未知時間" : "Unknown time")} | ${report.report_type || (locale === "zh" ? "未知類型" : "unknown type")}`;
}

export function findPreferredReportID(reports, selectedID = "") {
  const items = Array.isArray(reports) ? reports : [];
  if (selectedID && items.some((item) => item?.id === selectedID)) {
    return selectedID;
  }

  return items[0]?.id || "";
}

export function hydrateReportResult(detail) {
  if (!detail?.result) {
    return null;
  }

  const structured = sanitizeStructuredReport(detail.result.structured);
  return {
    ...detail.result,
    report_id: detail.result.report_id || detail.id,
    structured
  };
}

export function sanitizeStructuredReport(structured) {
  if (!structured || typeof structured !== "object") {
    return null;
  }

  const normalized = {
    deck_identity: normalizeStructuredItems(structured.deck_identity),
    what_the_deck_is_doing_well: normalizeStructuredItems(structured.what_the_deck_is_doing_well),
    main_risks: normalizeStructuredItems(structured.main_risks),
    practical_next_adjustments: normalizeStructuredItems(structured.practical_next_adjustments)
  };

  const sections = Object.values(normalized);
  const totalItems = sections.reduce((sum, items) => sum + items.length, 0);
  const populatedSections = sections.filter((items) => items.length > 0).length;
  if (totalItems === 0 || totalItems < 3 || populatedSections < 2) {
    return null;
  }

  for (const items of sections) {
    if (items.length > MAX_STRUCTURED_SECTION_ITEMS) {
      return null;
    }
    if (items.some((item) => item.length > MAX_STRUCTURED_ITEM_LENGTH)) {
      return null;
    }
  }

  return normalized;
}

function normalizeStructuredItems(value) {
  const items = Array.isArray(value) ? value : typeof value === "string" ? [value] : [];
  const out = [];
  const seen = new Set();

  for (const item of items) {
    if (typeof item !== "string") {
      continue;
    }

    const normalized = item.trim();
    if (!normalized || seen.has(normalized)) {
      continue;
    }

    seen.add(normalized);
    out.push(normalized);
  }

  return out;
}

export function summarizeReportCompare(compare, fallback = "No saved compare context", locale = "en") {
  if (!compare) {
    return fallback;
  }

  return locale === "zh"
    ? `${compare.snapshot_id || "未知快照"} | ${compare.format || "未知模式"} | 版本 ${compare.patch_version || "未知"}`
    : `${compare.snapshot_id || "unknown snapshot"} | ${compare.format || "unknown format"} | patch ${compare.patch_version || "unknown"}`;
}

export function normalizeCompareGuidance(compare) {
  if (!compare || typeof compare !== "object") {
    return null;
  }

  const mergedGuidance = compare.merged_guidance && typeof compare.merged_guidance === "object"
    ? compare.merged_guidance
    : {};

  const normalized = {
    summary: normalizeGuidanceItems(mergedGuidance.summary),
    adds: normalizeGuidanceItems(mergedGuidance.adds),
    cuts: normalizeGuidanceItems(mergedGuidance.cuts)
  };

  if (!normalized.summary.length && Array.isArray(compare.merged_summary)) {
    normalized.summary = compare.merged_summary
      .map((message, index) => normalizeFallbackGuidanceItem("summary", message, index))
      .filter(Boolean);
  }
  if (!normalized.adds.length && Array.isArray(compare.merged_suggested_adds)) {
    normalized.adds = compare.merged_suggested_adds
      .map((message, index) => normalizeFallbackGuidanceItem("add", message, index))
      .filter(Boolean);
  }
  if (!normalized.cuts.length && Array.isArray(compare.merged_suggested_cuts)) {
    normalized.cuts = compare.merged_suggested_cuts
      .map((message, index) => normalizeFallbackGuidanceItem("cut", message, index))
      .filter(Boolean);
  }

  if (!normalized.summary.length && !normalized.adds.length && !normalized.cuts.length) {
    return null;
  }

  return normalized;
}

export function hasCompareGuidance(compare) {
  const guidance = normalizeCompareGuidance(compare);
  return Boolean(guidance && (guidance.summary.length || guidance.adds.length || guidance.cuts.length));
}

export function summarizeGuidanceSupport(item, fallback = "No support detail", locale = "en") {
  if (!item?.support?.length) {
    return fallback;
  }

  const parts = item.support
    .map((support) => support?.evidence?.trim())
    .filter(Boolean);
  return parts[0] || fallback;
}

export function summarizeReportCompareCandidate(candidate, fallback = "Unknown candidate", locale = "en") {
  if (!candidate) {
    return fallback;
  }

  const similarity = typeof candidate.similarity === "number"
    ? `${(candidate.similarity * 100).toFixed(1)}%`
    : locale === "zh" ? "相似度未知" : "unknown similarity";
  const tier = candidate.tier || (locale === "zh" ? "未知 Tier" : "unknown tier");
  return locale === "zh"
    ? `${candidate.name || fallback} | 相似度 ${similarity} | ${tier}`
    : `${candidate.name || fallback} | similarity ${similarity} | ${tier}`;
}

function normalizeGuidanceItems(items) {
  if (!Array.isArray(items)) {
    return [];
  }

  const out = [];
  const seen = new Set();
  for (const item of items) {
    const normalized = normalizeGuidanceItem(item);
    if (!normalized) {
      continue;
    }

    const key = `${normalized.kind}|${normalized.message}`;
    if (seen.has(key)) {
      continue;
    }

    seen.add(key);
    out.push(normalized);
    if (out.length === MAX_GUIDANCE_ITEMS) {
      break;
    }
  }
  return out;
}

function normalizeGuidanceItem(item) {
  if (!item || typeof item !== "object") {
    return null;
  }

  const message = typeof item.message === "string" ? item.message.trim() : "";
  if (!message) {
    return null;
  }

  return {
    key: typeof item.key === "string" && item.key.trim() ? item.key.trim() : message,
    kind: typeof item.kind === "string" && item.kind.trim() ? item.kind.trim() : "summary",
    package: typeof item.package === "string" ? item.package.trim() : "",
    source: typeof item.source === "string" && item.source.trim() ? item.source.trim() : "legacy_string",
    message,
    confidence: typeof item.confidence === "number" ? item.confidence : null,
    support: normalizeGuidanceSupport(item.support)
  };
}

function normalizeGuidanceSupport(items) {
  if (!Array.isArray(items)) {
    return [];
  }

  const out = [];
  for (const item of items) {
    if (!item || typeof item !== "object") {
      continue;
    }

    const evidence = typeof item.evidence === "string" ? item.evidence.trim() : "";
    if (!evidence) {
      continue;
    }

    out.push({
      source: typeof item.source === "string" ? item.source.trim() : "",
      candidate_deck_id: typeof item.candidate_deck_id === "string" ? item.candidate_deck_id.trim() : "",
      candidate_name: typeof item.candidate_name === "string" ? item.candidate_name.trim() : "",
      candidate_rank: typeof item.candidate_rank === "number" ? item.candidate_rank : null,
      weight: typeof item.weight === "number" ? item.weight : null,
      evidence
    });
  }
  return out;
}

function normalizeFallbackGuidanceItem(kind, message, index) {
  if (typeof message !== "string" || !message.trim()) {
    return null;
  }

  return {
    key: `legacy-${kind}-${index}`,
    kind,
    package: "",
    source: "legacy_string",
    message: message.trim(),
    confidence: null,
    support: []
  };
}
