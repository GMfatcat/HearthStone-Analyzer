export function mapJob(item) {
  return {
    ...item,
    draftCronExpr: item.cron_expr ?? "",
    draftEnabled: Boolean(item.enabled),
    history: []
  };
}

export function summarizeLatestRun(job, locale = "en") {
  const latest = job.history?.[0];
  if (!latest) {
    return locale === "zh" ? "尚未執行" : "No runs yet";
  }

  return `${formatExecutionStatus(latest.status, locale)} ${locale === "zh" ? "於" : "at"} ${formatTimestamp(latest.started_at, locale === "zh" ? "未知時間" : "Unknown time")}`;
}

export function formatExecutionStatus(status, locale = "en") {
  if (!status) {
    return locale === "zh" ? "未知" : "Unknown";
  }

  if (locale === "zh") {
    switch (status) {
      case "success":
        return "成功";
      case "failed":
        return "失敗";
      case "running":
        return "執行中";
      default:
        return "未知";
    }
  }

  return status.charAt(0).toUpperCase() + status.slice(1);
}

export function formatTimestamp(value, fallback = "Not scheduled") {
  if (!value) {
    return fallback;
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return fallback;
  }

  return date.toLocaleString();
}

export function didHistoryAdvance(nextHistory, previousHistory) {
  const next = nextHistory?.[0];
  const previous = previousHistory?.[0];

  if (!next) {
    return false;
  }

  if (!previous) {
    return true;
  }

  return next.id !== previous.id || next.started_at !== previous.started_at;
}

export function formatJobError(message, locale = "en") {
  if (!message) {
    return locale === "zh" ? "未知排程錯誤" : "Unknown job error";
  }

  if (message.includes("job already running")) {
    return locale === "zh"
      ? "這個工作已經在執行中，請等目前這次完成後再試。"
      : "This job is already running. Wait for the current execution to finish and try again.";
  }

  return message;
}

export function summarizeJobHistoryError(message, locale = "en") {
  if (!message) {
    return "";
  }

  if (message.includes("job already running")) {
    return formatJobError(message, locale);
  }

  return locale === "zh"
    ? "執行失敗，詳細原因請查看後端日誌。"
    : "Execution failed. Check backend logs for details.";
}

export function formatExecutionDuration(startedAt, finishedAt, locale = "en") {
  if (!startedAt || !finishedAt) {
    return locale === "zh" ? "進行中" : "In progress";
  }

  const started = new Date(startedAt);
  const finished = new Date(finishedAt);
  const diffMs = finished.getTime() - started.getTime();
  if (Number.isNaN(diffMs) || diffMs < 0) {
    return locale === "zh" ? "未知" : "Unknown";
  }

  const totalSeconds = Math.floor(diffMs / 1000);
  if (totalSeconds < 60) {
    return `${totalSeconds}s`;
  }

  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes < 60) {
    return seconds === 0 ? `${minutes}m` : `${minutes}m ${seconds}s`;
  }

  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  return remainingMinutes === 0 ? `${hours}h` : `${hours}h ${remainingMinutes}m`;
}

export function formatMetaStatus(snapshot, fallback = "Meta snapshot not available", locale = "en") {
  if (!snapshot) {
    return fallback;
  }

  const source = snapshot.source || (locale === "zh" ? "未知來源" : "unknown source");
  const patchVersion = snapshot.patch_version || (locale === "zh" ? "未知版本" : "unknown patch");
  const format = snapshot.format || (locale === "zh" ? "未知模式" : "unknown format");
  return locale === "zh"
    ? `${source} 的 ${format} meta 快照，版本 ${patchVersion}`
    : `${source} snapshot for ${format} on patch ${patchVersion}`;
}

export function summarizeMetaSnapshot(snapshot, fallback = "No snapshot selected", locale = "en") {
  if (!snapshot) {
    return fallback;
  }

  return locale === "zh"
    ? `${formatMetaStatus(snapshot, fallback, locale)}，抓取時間 ${formatTimestamp(snapshot.fetched_at, "未知時間")}`
    : `${formatMetaStatus(snapshot, fallback, locale)} fetched ${formatTimestamp(snapshot.fetched_at, "Unknown time")}`;
}

export function formatMetaPayloadPreview(rawPayload, fallback = "No raw payload available") {
  if (!rawPayload) {
    return fallback;
  }

  const compact = rawPayload.trim();
  if (compact.length <= 240) {
    return compact;
  }

  return `${compact.slice(0, 240)}...`;
}

export function extractMetaDeckRows(rawPayload) {
  if (!rawPayload) {
    return [];
  }

  try {
    const payload = JSON.parse(rawPayload);
    const decks = Array.isArray(payload?.decks) ? payload.decks : [];

    return decks
      .map((deck, index) => {
        const playrate = normalizeRate(deck.playrate);
        const winrate = normalizeRate(deck.winrate);
        const sampleSize = normalizeInteger(deck.sample_size ?? deck.sampleSize);

        return {
          id: deck.id || deck.deck_id || deck.name || `deck-${index}`,
          name: deck.name || "Unknown deck",
          className: deck.class || deck.deck_class || "Unknown",
          tier: deck.tier || "Unknown",
          playrate,
          winrate,
          sampleSize,
          playrateLabel: formatRate(playrate),
          winrateLabel: formatRate(winrate),
          sampleSizeLabel: formatCount(sampleSize)
        };
      })
      .sort((left, right) => {
        const rateDelta = compareNullableNumber(right.playrate, left.playrate);
        if (rateDelta !== 0) {
          return rateDelta;
        }

        const winrateDelta = compareNullableNumber(right.winrate, left.winrate);
        if (winrateDelta !== 0) {
          return winrateDelta;
        }

        const sampleDelta = compareNullableNumber(right.sampleSize, left.sampleSize);
        if (sampleDelta !== 0) {
          return sampleDelta;
        }

        return left.name.localeCompare(right.name);
      });
  } catch {
    return [];
  }
}

function normalizeRate(value) {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return null;
  }

  return value <= 1 ? value * 100 : value;
}

function normalizeInteger(value) {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return null;
  }

  return Math.trunc(value);
}

function formatRate(value) {
  if (value == null) {
    return "Unknown";
  }

  return `${value.toFixed(1)}%`;
}

function formatCount(value) {
  if (value == null) {
    return "Unknown";
  }

  return value.toLocaleString();
}

function compareNullableNumber(left, right) {
  if (left == null && right == null) {
    return 0;
  }

  if (left == null) {
    return -1;
  }

  if (right == null) {
    return 1;
  }

  return left - right;
}
