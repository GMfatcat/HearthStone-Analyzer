<script setup>
import { computed, onMounted, reactive } from "vue";
import {
  didHistoryAdvance,
  extractMetaDeckRows,
  formatExecutionDuration,
  formatExecutionStatus,
  formatJobError,
  formatMetaPayloadPreview,
  formatMetaStatus,
  formatTimestamp,
  mapJob,
  summarizeJobHistoryError,
  summarizeMetaSnapshot,
  summarizeLatestRun
} from "./jobsViewModel.js";
import {
  findPreferredReportID,
  hydrateReportResult,
  hasCompareGuidance,
  normalizeCompareGuidance,
  summarizeGuidanceSupport,
  summarizeReportCompare,
  summarizeReportCompareCandidate,
  summarizeStoredReport
} from "./reportsViewModel.js";

const localeMessages = {
  en: {
    language: "Language", english: "English", chinese: "中文",
    heroTitle: "Deck Parse & Analyze Workbench",
    heroSummary: "Paste a Hearthstone deck code to inspect the parsed list, legality issues, archetype, and early rule-based structural signals.",
    deckInput: "Deck Input", runParser: "Run Parser or Analyzer", deckCode: "Deck code", deckPlaceholder: "Paste a deck code here...",
    parsing: "Parsing...", parseDeck: "Parse Deck", analyzing: "Analyzing...", analyzeDeck: "Analyze Deck",
    comparing: "Comparing...", compareMeta: "Compare Meta", generating: "Generating...", generateReport: "Generate Report",
    parseError: "Parse error", analyzeError: "Analyze error", reportError: "Report error",
    idleHint: "Parse checks structure and legality. Analyze adds rule-based archetype and feature signals.",
    deckOutput: "Deck Output", parseAndAnalyzeResults: "Parse & Analyze Results", parse: "Parse", analyze: "Analyze", compare: "Compare", report: "Report",
    settings: "Settings", jobs: "Jobs", meta: "Meta", refresh: "Refresh", loading: "Loading...",
    saved: "Saved {key}", savedJob: "Saved {key}", loadedLatestMetaSnapshot: "Loaded latest meta snapshot", loadedSnapshot: "Loaded snapshot {id}",
    refreshedHistory: "Refreshed history for {key}", startedJobWaiting: "Started {key}, waiting for execution update...", completedRefresh: "Completed refresh for {key}", timedOutRefresh: "Started {key}. History refresh timed out, but the latest state was reloaded.",
    deckParsedSuccess: "Deck parsed successfully", deckAnalyzedSuccess: "Deck analyzed successfully", reportGeneratedSuccess: "AI report generated successfully", comparedSuccess: "Deck compared against latest meta snapshot",
    failedLoadSettings: "Failed to load settings", failedSaveSetting: "Failed to save setting", failedLoadJobs: "Failed to load jobs", failedLoadMetaSnapshot: "Failed to load meta snapshot", failedLoadMetaHistory: "Failed to load meta history", metaUnavailable: "Meta snapshot not available yet", failedLoadReports: "Failed to load reports", failedLoadReportDetail: "Failed to load report detail", failedLoadMetaSnapshotDetail: "Failed to load meta snapshot detail", failedSaveJob: "Failed to save job", failedRunJob: "Failed to run job",
    unknown: "Unknown", noLegalityIssues: "No legality issues", valid: "Valid", invalid: "Invalid", confidence: "Confidence", confidenceUnavailable: "Confidence unavailable", unspecifiedSource: "Unspecified source",
    noSavedReportsYet: "No saved reports yet", noMetaSnapshotYet: "No Meta Snapshot Yet", noMetaSnapshotBody: "Core deck parsing and analysis still work without meta data. Run sync_meta after configuring a meta source or fixture to populate this area.",
    llmConfiguration: "LLM Configuration", value: "Value", enterSecretValue: "Enter secret value", enterValue: "Enter value", save: "Save", saving: "Saving...",
    enabled: "Enabled", disabled: "Disabled", next: "Next", cronExpression: "Cron expression", saveJobAction: "Save Job", running: "Running...", runNow: "Run Now", refreshHistory: "Refresh History", schedule: "Schedule", lastRun: "Last run", never: "Never", nextRun: "Next run", recentHistory: "Recent History", finished: "Finished", stillRunning: "Still running", duration: "Duration", recordsAffected: "Records affected", noExecutionHistory: "No execution history yet.",
    latestSnapshotOverview: "Latest Snapshot Overview", recentSnapshots: "Recent Snapshots", selectedSnapshot: "Selected Snapshot",
    recentReports: "Recent Reports", savedCompareContext: "Saved Compare Context", structuredGuidance: "Structured Guidance", itemsSuffix: "items"
  },
  zh: {
    language: "語言", english: "English", chinese: "中文",
    heroTitle: "牌組解析與分析工作台",
    heroSummary: "貼上 Hearthstone deck code，檢視解析後牌組、合法性、牌型判讀與早期規則式分析訊號。",
    deckInput: "牌組輸入", runParser: "執行解析或分析", deckCode: "牌組代碼", deckPlaceholder: "在這裡貼上 deck code...",
    parsing: "解析中...", parseDeck: "解析牌組", analyzing: "分析中...", analyzeDeck: "分析牌組",
    comparing: "比對中...", compareMeta: "比對 Meta", generating: "生成中...", generateReport: "生成報告",
    parseError: "解析錯誤", analyzeError: "分析錯誤", reportError: "報告錯誤",
    idleHint: "Parse 會檢查牌組結構與合法性，Analyze 會補上牌型與規則式訊號。",
    deckOutput: "牌組輸出", parseAndAnalyzeResults: "解析與分析結果", parse: "解析", analyze: "分析", compare: "比對", report: "報告",
    settings: "設定", jobs: "排程", meta: "Meta", refresh: "重新整理", loading: "載入中...",
    saved: "已儲存 {key}", savedJob: "已儲存 {key}", loadedLatestMetaSnapshot: "已載入最新 meta 快照", loadedSnapshot: "已載入快照 {id}",
    refreshedHistory: "已重新整理 {key} 的歷史紀錄", startedJobWaiting: "已啟動 {key}，等待執行狀態更新...", completedRefresh: "已完成 {key} 的狀態更新", timedOutRefresh: "已啟動 {key}，但歷史刷新逾時，已重新載入最新狀態。",
    deckParsedSuccess: "牌組解析成功", deckAnalyzedSuccess: "牌組分析成功", reportGeneratedSuccess: "AI 報告生成成功", comparedSuccess: "已與最新 meta 快照完成比對",
    failedLoadSettings: "載入設定失敗", failedSaveSetting: "儲存設定失敗", failedLoadJobs: "載入排程失敗", failedLoadMetaSnapshot: "載入 meta 快照失敗", failedLoadMetaHistory: "載入 meta 歷史失敗", metaUnavailable: "目前還沒有 meta 快照", failedLoadReports: "載入報告失敗", failedLoadReportDetail: "載入報告詳情失敗", failedLoadMetaSnapshotDetail: "載入快照詳情失敗", failedSaveJob: "儲存排程失敗", failedRunJob: "執行排程失敗",
    unknown: "未知", noLegalityIssues: "沒有合法性問題", valid: "合法", invalid: "不合法", confidence: "信心度", confidenceUnavailable: "沒有信心度", unspecifiedSource: "未指定來源",
    noSavedReportsYet: "目前沒有已保存的報告", noMetaSnapshotYet: "目前沒有 Meta 快照", noMetaSnapshotBody: "即使沒有 meta 資料，核心牌組解析與分析仍可使用。設定好 meta source 或 fixture 後執行 sync_meta，這裡就會出現資料。",
    llmConfiguration: "LLM 設定", value: "值", enterSecretValue: "輸入密鑰", enterValue: "輸入值", save: "儲存", saving: "儲存中...",
    enabled: "啟用", disabled: "停用", next: "下次", cronExpression: "Cron 表達式", saveJobAction: "儲存排程", running: "執行中...", runNow: "立即執行", refreshHistory: "重新整理歷史", schedule: "排程", lastRun: "上次執行", never: "從未", nextRun: "下次執行", recentHistory: "最近紀錄", finished: "完成時間", stillRunning: "仍在執行", duration: "耗時", recordsAffected: "影響筆數", noExecutionHistory: "目前沒有執行紀錄。",
    latestSnapshotOverview: "最新快照總覽", recentSnapshots: "最近快照", selectedSnapshot: "已選擇快照",
    recentReports: "最近報告", savedCompareContext: "已保存的比對上下文", structuredGuidance: "結構化建議", itemsSuffix: "筆"
  }
};

const initialLocale = typeof window !== "undefined" ? window.localStorage.getItem("ui.locale") || "en" : "en";
const state = reactive({ locale: initialLocale, loadingSettings: true, loadingJobs: true, loadingMeta: true, loadingReports: true, loadingReportDetail: false, savingKey: "", savingJobKey: "", runningJobKey: "", runningAction: "", parseError: "", analyzeError: "", reportError: "", settingsError: "", jobsError: "", metaError: "", deckSuccess: "", settingsSuccess: "", jobsSuccess: "", metaSuccess: "", deckCode: "", parseResult: null, analysisResult: null, compareResult: null, reportResult: null, reportsHistory: [], selectedReport: null, activeDeckTab: "parse", settings: [], jobs: [], metaSnapshot: null, metaHistory: [], selectedMetaSnapshot: null });
const hasDeckOutput = computed(() => state.parseResult || state.analysisResult || state.compareResult || state.reportResult);
const compareGuidance = computed(() => normalizeCompareGuidance(state.compareResult));
const t = (key, vars = {}) => (localeMessages[state.locale]?.[key] || localeMessages.en[key] || key).replace(/\{(\w+)\}/g, (_, name) => vars[name] ?? "");
const setLocale = (locale) => { state.locale = locale; if (typeof window !== "undefined") { window.localStorage.setItem("ui.locale", locale); } };
const formatIssues = (issues) => issues?.length ? issues.join(" | ") : t("noLegalityIssues");
const isSelectedMetaSnapshot = (snapshot) => state.selectedMetaSnapshot?.id === snapshot.id;
const isSelectedReport = (item) => state.selectedReport?.id === item.id;
const selectedMetaDeckRows = () => extractMetaDeckRows(state.selectedMetaSnapshot?.raw_payload);
const formatFeatureValue = (value, fixed = null) => typeof value !== "number" || Number.isNaN(value) ? t("unknown") : value.toLocaleString(undefined, { minimumFractionDigits: fixed ?? (Number.isInteger(value) ? 0 : 2), maximumFractionDigits: fixed ?? (Number.isInteger(value) ? 0 : 2) });
const formatGuidanceConfidence = (value) => typeof value !== "number" ? t("confidenceUnavailable") : `${t("confidence")} ${(value * 100).toFixed(0)}%`;
const formatGuidanceSource = (value) => !value ? t("unspecifiedSource") : value.split("_").filter(Boolean).map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(" ");
const featureRows = (features) => !features ? [] : [[state.locale === "zh" ? "平均費用" : "Average cost", formatFeatureValue(features.avg_cost, 1)], [state.locale === "zh" ? "手下" : "Minions", formatFeatureValue(features.minion_count)], [state.locale === "zh" ? "法術" : "Spells", formatFeatureValue(features.spell_count)], [state.locale === "zh" ? "武器" : "Weapons", formatFeatureValue(features.weapon_count)], [state.locale === "zh" ? "前期曲線" : "Early curve", formatFeatureValue(features.early_curve_count)], [state.locale === "zh" ? "高費牌" : "Top-heavy", formatFeatureValue(features.top_heavy_count)], [state.locale === "zh" ? "抽牌" : "Draw", formatFeatureValue(features.draw_count)], [state.locale === "zh" ? "直傷" : "Burn", formatFeatureValue(features.burn_count)], [state.locale === "zh" ? "AOE" : "Aoe", formatFeatureValue(features.aoe_count)], [state.locale === "zh" ? "治療" : "Heal", formatFeatureValue(features.heal_count)], [state.locale === "zh" ? "發現" : "Discover", formatFeatureValue(features.discover_count)], [state.locale === "zh" ? "單體解牌" : "Single removal", formatFeatureValue(features.single_removal_count)], [state.locale === "zh" ? "嘲諷" : "Taunt", formatFeatureValue(features.taunt_count)], [state.locale === "zh" ? "鋪場" : "Token", formatFeatureValue(features.token_count)], [state.locale === "zh" ? "死聲" : "Deathrattle", formatFeatureValue(features.deathrattle_count)], [state.locale === "zh" ? "戰吼" : "Battlecry", formatFeatureValue(features.battlecry_count)], [state.locale === "zh" ? "費用作弊" : "Mana cheat", formatFeatureValue(features.mana_cheat_count)], [state.locale === "zh" ? "Combo 零件" : "Combo piece", formatFeatureValue(features.combo_piece_count)]];
const translateArchetype = (value) => state.locale !== "zh" ? value : ({ Aggro: "快攻", Midrange: "中速", Control: "控制" }[value] || value);
const translateStatus = (value) => state.locale !== "zh" ? value : ({ underbuilt: "不足", balanced: "平衡", overbuilt: "過量", conflict: "衝突" }[value] || value);
const translateAnalysisText = (value) => {
  if (state.locale !== "zh" || typeof value !== "string") { return value; }
  const map = {
    "Fast early curve": "前期曲線很快",
    "Consistent board development": "場面展開穩定",
    "Strong late-game profile": "後期能力強",
    "Well-spread mana curve": "法力曲線分布均衡",
    "Balanced card mix": "牌組構成整體平衡",
    "May run out of resources": "資源續航可能不足",
    "Slow early turns": "前期節奏偏慢",
    "Top-heavy curve can create clunky early turns": "高費牌偏多，容易讓前期節奏卡手",
    "Limited reactive spell package": "反應型法術配置偏少",
    "No obvious structural weakness identified yet": "目前沒有明顯的結構性弱點",
    "A dense low-curve package makes the aggressive read more reliable.": "低費密度很高，讓快攻判讀更可靠。",
    "A high early-game share reinforces fast pressure as the primary plan.": "前期卡位比例高，強化了快速施壓的主計畫。",
    "A heavy late-game concentration strongly supports a control profile.": "後期卡位占比高，強烈支持控制牌型判讀。",
    "The deck commits a large share of slots to slower payoff turns.": "牌組投入大量格位在偏慢的後期收益回合。",
    "The curve is spread across multiple turns, which fits a midrange shell.": "曲線分布跨越多個回合，符合中速牌型。",
    "Both early and late buckets are present, so the list does not lean hard into one extreme.": "前後期區段都有配置，代表這副牌沒有明顯偏向單一極端。",
    "Multiple functional card signals give the structural read more support.": "多種功能型訊號讓這次結構判讀更有支撐。",
    "Confidence is limited because card text signals are still sparse and mostly curve-driven.": "信心度有限，因為目前卡牌文字訊號仍偏少，主要還是依曲線判讀。",
    "Confidence is reduced because the submitted deck is not a clean, legal 30-card list.": "這副牌不是乾淨合法的 30 張牌組，因此信心度下降。",
    "Confidence is driven mostly by general curve shape rather than one overwhelming archetype signal.": "目前信心度主要來自整體曲線形狀，而非單一非常強烈的牌型訊號。"
  };
  return map[value] || value;
};
async function readResponsePayload(response) { const contentType = response.headers.get("content-type") || ""; return contentType.includes("application/json") ? response.json() : response.text(); }
async function loadSettings() { state.loadingSettings = true; try { const response = await fetch("/api/settings"); if (!response.ok) { throw new Error(t("failedLoadSettings")); } const payload = await response.json(); state.settings = payload.map((item) => ({ ...item, draftValue: item.value ?? "" })); state.settingsError = ""; } catch (error) { state.settingsError = error.message; } finally { state.loadingSettings = false; } }
async function saveSetting(setting) { state.savingKey = setting.key; state.settingsError = ""; state.settingsSuccess = ""; try { const response = await fetch(`/api/settings/${setting.key}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ value: setting.draftValue }) }); const payload = await readResponsePayload(response); if (!response.ok) { throw new Error(payload.message || payload || t("failedSaveSetting")); } setting.value = payload.value; setting.draftValue = payload.value; state.settingsSuccess = t("saved", { key: setting.key }); } catch (error) { state.settingsError = error.message; } finally { state.savingKey = ""; } }
async function loadJobs() { state.loadingJobs = true; state.jobsError = ""; try { const response = await fetch("/api/jobs"); if (!response.ok) { throw new Error(t("failedLoadJobs")); } const payload = await response.json(); const jobs = payload.map(mapJob); await Promise.all(jobs.map(async (job) => { job.history = await loadJobHistory(job.key); })); state.jobs = jobs; } catch (error) { state.jobsError = formatJobError(error.message, state.locale); } finally { state.loadingJobs = false; } }
async function loadMetaSnapshot() { state.loadingMeta = true; state.metaError = ""; state.metaSuccess = ""; try { const [latestResponse, historyResponse] = await Promise.all([fetch("/api/meta/latest?format=standard"), fetch("/api/meta?format=standard&limit=6")]); const latestPayload = await readResponsePayload(latestResponse); const historyPayload = await readResponsePayload(historyResponse); if (latestResponse.status === 404) { state.metaSnapshot = null; state.metaHistory = []; state.selectedMetaSnapshot = null; state.metaError = t("metaUnavailable"); return; } if (!latestResponse.ok) { throw new Error(latestPayload.message || latestPayload || t("failedLoadMetaSnapshot")); } if (!historyResponse.ok) { throw new Error(historyPayload.message || historyPayload || t("failedLoadMetaHistory")); } state.metaSnapshot = latestPayload; state.metaHistory = historyPayload; state.selectedMetaSnapshot = null; if (state.metaHistory[0]?.id) { await loadMetaSnapshotDetail(state.metaHistory[0].id, { silent: true }); } state.metaSuccess = t("loadedLatestMetaSnapshot"); } catch (error) { state.metaError = error.message; } finally { state.loadingMeta = false; } }
async function loadReports(options = {}) { state.loadingReports = true; state.reportError = ""; try { const response = await fetch("/api/reports?limit=5"); const payload = await readResponsePayload(response); if (!response.ok) { throw new Error(payload.message || payload || t("failedLoadReports")); } state.reportsHistory = payload; if (options.skipDetailLoad) { return; } const preferredID = findPreferredReportID(payload, options.selectedID || state.selectedReport?.id); if (preferredID) { await loadReportDetail(preferredID, { silent: true }); } else if (!state.reportResult) { state.selectedReport = null; } } catch (error) { state.reportError = error.message; } finally { state.loadingReports = false; } }
async function loadReportDetail(id, options = {}) { state.loadingReportDetail = true; if (!options.silent) { state.reportError = ""; } try { const response = await fetch(`/api/reports/${id}`); const payload = await readResponsePayload(response); if (!response.ok) { throw new Error(payload.message || payload || t("failedLoadReportDetail")); } state.selectedReport = payload; state.reportResult = hydrateReportResult(payload); state.analysisResult = payload.result?.analysis || state.analysisResult; state.compareResult = payload.compare || null; state.activeDeckTab = "report"; } catch (error) { state.reportError = error.message; } finally { state.loadingReportDetail = false; } }
async function loadMetaSnapshotDetail(id, options = {}) { state.metaError = ""; if (!options.silent) { state.metaSuccess = ""; } try { const response = await fetch(`/api/meta/${id}`); const payload = await readResponsePayload(response); if (!response.ok) { throw new Error(payload.message || payload || t("failedLoadMetaSnapshotDetail")); } state.selectedMetaSnapshot = payload; if (!options.silent) { state.metaSuccess = t("loadedSnapshot", { id }); } } catch (error) { state.metaError = error.message; } }
async function loadJobHistory(jobKey) { const response = await fetch(`/api/jobs/${jobKey}/history?limit=5`); const payload = await readResponsePayload(response); if (!response.ok) { throw new Error(payload.message || payload || `Failed to load history for ${jobKey}`); } return payload; }
async function refreshJobHistory(job, options = {}) { state.jobsError = ""; try { job.history = await loadJobHistory(job.key); if (!options.silent) { state.jobsSuccess = t("refreshedHistory", { key: job.key }); } } catch (error) { state.jobsError = formatJobError(error.message, state.locale); } }
async function saveJob(job) { state.savingJobKey = job.key; state.jobsError = ""; state.jobsSuccess = ""; try { const response = await fetch(`/api/jobs/${job.key}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ cron_expr: job.draftCronExpr, enabled: job.draftEnabled }) }); const payload = await readResponsePayload(response); if (!response.ok) { throw new Error(payload.message || payload || t("failedSaveJob")); } Object.assign(job, mapJob(payload), { history: job.history }); state.jobsSuccess = t("savedJob", { key: job.key }); await refreshJobHistory(job, { silent: true }); } catch (error) { state.jobsError = formatJobError(error.message, state.locale); } finally { state.savingJobKey = ""; } }
async function runJob(job) { state.runningJobKey = job.key; state.jobsError = ""; state.jobsSuccess = ""; const previousHistory = [...job.history]; try { const response = await fetch(`/api/jobs/${job.key}/run`, { method: "POST" }); const payload = await readResponsePayload(response); if (!response.ok) { throw new Error(payload.message || payload || t("failedRunJob")); } state.jobsSuccess = t("startedJobWaiting", { key: job.key }); await waitForJobHistoryUpdate(job, previousHistory); } catch (error) { state.jobsError = formatJobError(error.message, state.locale); } finally { state.runningJobKey = ""; } }
async function waitForJobHistoryUpdate(job, previousHistory) { const maxAttempts = 6; for (let attempt = 0; attempt < maxAttempts; attempt += 1) { await new Promise((resolve) => window.setTimeout(resolve, 1000)); const nextHistory = await loadJobHistory(job.key); job.history = nextHistory; if (didHistoryAdvance(nextHistory, previousHistory)) { const detailResponse = await fetch(`/api/jobs/${job.key}`); const detailPayload = await readResponsePayload(detailResponse); if (!detailResponse.ok) { throw new Error(detailPayload.message || detailPayload || `Failed to refresh ${job.key}`); } Object.assign(job, mapJob(detailPayload), { history: nextHistory }); state.jobsSuccess = t("completedRefresh", { key: job.key }); return; } } await loadJobs(); state.jobsSuccess = t("timedOutRefresh", { key: job.key }); }
async function parseDeck() { await runDeckAction("parse", "/api/decks/parse", (payload) => { state.parseResult = payload; state.analysisResult = null; state.compareResult = null; state.reportResult = null; state.activeDeckTab = "parse"; state.deckSuccess = t("deckParsedSuccess"); }); }
async function analyzeDeck() { await runDeckAction("analyze", "/api/decks/analyze", (payload) => { state.analysisResult = payload; state.activeDeckTab = "analyze"; state.deckSuccess = t("deckAnalyzedSuccess"); }); }
async function generateReport() { await runDeckAction("report", "/api/reports/generate", (payload) => { state.reportResult = payload; state.analysisResult = payload.analysis; state.compareResult = payload.compare || null; state.selectedReport = payload.report_id ? { id: payload.report_id, created_at: payload.generated_at, report_type: "ai_deck_report" } : null; state.activeDeckTab = "report"; state.deckSuccess = t("reportGeneratedSuccess"); }); if (state.reportResult?.report_id) { await loadReports({ selectedID: state.reportResult.report_id }); } else { await loadReports({ skipDetailLoad: true }); } }
async function compareDeck() { await runDeckAction("compare", "/api/decks/compare", (payload) => { state.compareResult = payload; state.activeDeckTab = "compare"; state.deckSuccess = t("comparedSuccess"); }); }
async function runDeckAction(action, url, onSuccess) { state.runningAction = action; state.parseError = ""; state.analyzeError = ""; state.reportError = ""; state.deckSuccess = ""; try { const response = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ deck_code: state.deckCode, language: state.locale }) }); const payload = await readResponsePayload(response); if (!response.ok) { throw new Error(payload.message || payload || "Request failed"); } onSuccess(payload); } catch (error) { if (action === "parse") { state.parseError = error.message; } else if (action === "report") { state.reportError = error.message; } else { state.analyzeError = error.message; } } finally { state.runningAction = ""; } }
onMounted(() => { loadSettings(); loadJobs(); loadMetaSnapshot(); loadReports(); });
</script>

<template>
  <main class="shell">
    <section class="hero">
      <div class="hero-top">
        <div></div>
        <div class="language-toggle"><span class="muted">{{ t("language") }}</span><button class="ghost-button locale-button" type="button" :class="{ active: state.locale === 'en' }" @click="setLocale('en')">{{ t("english") }}</button><button class="ghost-button locale-button" type="button" :class="{ active: state.locale === 'zh' }" @click="setLocale('zh')">{{ t("chinese") }}</button></div>
      </div>
      <p class="eyebrow">HearthStone Analyzer</p>
      <h1>{{ t("heroTitle") }}</h1>
      <p class="summary">{{ t("heroSummary") }}</p>
    </section>

    <section class="panel">
      <header class="panel-header"><div><p class="eyebrow">{{ t("deckInput") }}</p><h2>{{ t("runParser") }}</h2></div></header>
      <label class="field"><span>{{ t("deckCode") }}</span><textarea v-model="state.deckCode" rows="5" :placeholder="t('deckPlaceholder')" /></label>
      <div class="actions split-actions">
        <button class="ghost-button" type="button" :disabled="state.runningAction !== ''" @click="parseDeck">{{ state.runningAction === "parse" ? t("parsing") : t("parseDeck") }}</button>
        <button class="primary-button" type="button" :disabled="state.runningAction !== ''" @click="analyzeDeck">{{ state.runningAction === "analyze" ? t("analyzing") : t("analyzeDeck") }}</button>
        <button class="ghost-button" type="button" :disabled="state.runningAction !== ''" @click="compareDeck">{{ state.runningAction === "compare" ? t("comparing") : t("compareMeta") }}</button>
        <button class="primary-button" type="button" :disabled="state.runningAction !== ''" @click="generateReport">{{ state.runningAction === "report" ? t("generating") : t("generateReport") }}</button>
      </div>
    </section>

    <section class="panel status-panel">
      <p v-if="state.parseError" class="status error">{{ t("parseError") }}: {{ state.parseError }}</p>
      <p v-if="state.analyzeError" class="status error">{{ t("analyzeError") }}: {{ state.analyzeError }}</p>
      <p v-if="state.reportError" class="status error">{{ t("reportError") }}: {{ state.reportError }}</p>
      <p v-if="state.deckSuccess" class="status success">{{ state.deckSuccess }}</p>
      <p v-if="!state.parseError && !state.analyzeError && !state.reportError && !state.deckSuccess" class="status muted">{{ t("idleHint") }}</p>
    </section>
    <section v-if="hasDeckOutput" class="panel">
      <header class="panel-header"><div><p class="eyebrow">{{ t("deckOutput") }}</p><h2>{{ t("parseAndAnalyzeResults") }}</h2></div></header>
      <div class="tab-row"><button v-if="state.parseResult" class="tab-button" :class="{ active: state.activeDeckTab === 'parse' }" type="button" @click="state.activeDeckTab = 'parse'">{{ t("parse") }}</button><button v-if="state.analysisResult" class="tab-button" :class="{ active: state.activeDeckTab === 'analyze' }" type="button" @click="state.activeDeckTab = 'analyze'">{{ t("analyze") }}</button><button v-if="state.compareResult" class="tab-button" :class="{ active: state.activeDeckTab === 'compare' }" type="button" @click="state.activeDeckTab = 'compare'">{{ t("compare") }}</button><button v-if="state.reportResult" class="tab-button" :class="{ active: state.activeDeckTab === 'report' }" type="button" @click="state.activeDeckTab = 'report'">{{ t("report") }}</button></div>

      <div v-if="state.activeDeckTab === 'parse' && state.parseResult" class="stack">
        <div class="meta-row"><span class="pill">{{ state.parseResult.class }}</span><span class="pill">{{ state.parseResult.format }}</span><span class="pill">{{ state.parseResult.total_count }}</span><span class="pill" :class="state.parseResult.legality.valid ? 'pill-success' : 'pill-error'">{{ state.parseResult.legality.valid ? t("valid") : t("invalid") }}</span></div>
        <p class="muted">{{ formatIssues(state.parseResult.legality.issues) }}</p>
        <p class="hash">Deck hash: {{ state.parseResult.deck_hash }}</p>
        <ul class="list"><li v-for="card in state.parseResult.cards" :key="card.card_id">{{ card.count }}x {{ card.name }} <span class="muted">({{ card.cost }} / {{ card.card_type }})</span></li></ul>
      </div>

      <div v-if="state.activeDeckTab === 'analyze' && state.analysisResult" class="stack">
        <div class="meta-row"><span class="pill">{{ translateArchetype(state.analysisResult.archetype) }}</span><span class="pill">{{ t("confidence") }} {{ ((state.analysisResult.confidence || 0) * 100).toFixed(1) }}%</span></div>
        <div class="grid two">
          <article class="card-block"><h3>Signals</h3><dl class="stats-list"><template v-for="[label, value] in featureRows(state.analysisResult.features)" :key="label"><dt>{{ label }}</dt><dd>{{ value }}</dd></template></dl></article>
          <article class="card-block"><h3>Mana Curve</h3><ul class="list"><li v-for="(count, cost) in state.analysisResult.features.mana_curve" :key="cost">{{ cost }} <span class="muted">:</span> {{ count }}</li></ul></article>
          <article v-if="state.analysisResult.confidence_reasons?.length" class="card-block"><h3>Confidence Reasons</h3><ul class="list"><li v-for="item in state.analysisResult.confidence_reasons" :key="item">{{ translateAnalysisText(item) }}</li></ul></article>
          <article v-if="state.analysisResult.structural_tag_details?.length || state.analysisResult.structural_tags?.length" class="card-block"><h3>Structural Read</h3><ul class="list" v-if="state.analysisResult.structural_tag_details?.length"><li v-for="item in state.analysisResult.structural_tag_details" :key="item.tag"><strong>{{ translateAnalysisText(item.title) }}</strong><div class="muted">{{ translateAnalysisText(item.explanation) }}</div><code>{{ item.tag }}</code></li></ul><ul class="list" v-else><li v-for="item in state.analysisResult.structural_tags" :key="item"><code>{{ item }}</code></li></ul></article>
          <article v-if="state.analysisResult.package_details?.length" class="card-block"><h3>Package Read</h3><ul class="list"><li v-for="item in state.analysisResult.package_details" :key="item.package"><strong>{{ translateAnalysisText(item.label) }} | {{ translateStatus(item.status) }}</strong><div class="muted">{{ item.slots }} slots <template v-if="item.target_min != null || item.target_max != null">| target {{ item.target_min ?? 0 }}-{{ item.target_max ?? 0 }}</template></div><div>{{ translateAnalysisText(item.explanation) }}</div></li></ul></article>
          <article v-if="state.analysisResult.functional_role_summary?.length" class="card-block"><h3>Functional Roles</h3><ul class="list"><li v-for="item in state.analysisResult.functional_role_summary" :key="item.role"><strong>{{ translateAnalysisText(item.label) }} | {{ item.count }}</strong><div class="muted">{{ translateAnalysisText(item.explanation) }}</div></li></ul></article>
          <article class="card-block"><h3>Strengths</h3><ul class="list"><li v-for="item in state.analysisResult.strengths" :key="item">{{ translateAnalysisText(item) }}</li></ul></article>
          <article class="card-block"><h3>Weaknesses</h3><ul class="list"><li v-for="item in state.analysisResult.weaknesses" :key="item">{{ translateAnalysisText(item) }}</li></ul></article>
          <article class="card-block"><h3>Suggested Adds</h3><ul class="list"><li v-for="item in state.analysisResult.suggested_adds || []" :key="item">{{ translateAnalysisText(item) }}</li></ul></article>
          <article class="card-block"><h3>Suggested Cuts</h3><ul class="list"><li v-for="item in state.analysisResult.suggested_cuts || []" :key="item">{{ translateAnalysisText(item) }}</li></ul></article>
        </div>
      </div>

      <div v-if="state.activeDeckTab === 'compare' && state.compareResult" class="stack">
        <div class="meta-row"><span class="pill">{{ state.compareResult.snapshot_id }}</span><span class="pill">{{ state.compareResult.format }}</span><span class="pill">{{ state.compareResult.patch_version }}</span></div>
        <article v-if="hasCompareGuidance(state.compareResult)" class="card-block"><h3>Merged Guidance</h3><div class="grid three"><div><strong>Summary</strong><ul class="list"><li v-for="item in compareGuidance?.summary || []" :key="item.key"><div>{{ item.message }}</div><div class="muted">{{ formatGuidanceSource(item.source) }} | {{ formatGuidanceConfidence(item.confidence) }}</div><div class="muted">{{ summarizeGuidanceSupport(item) }}</div></li></ul></div><div><strong>Adds</strong><ul class="list"><li v-for="item in compareGuidance?.adds || []" :key="item.key"><div>{{ item.message }}</div><div class="muted">{{ formatGuidanceSource(item.source) }} | {{ formatGuidanceConfidence(item.confidence) }}</div><div class="muted">{{ summarizeGuidanceSupport(item) }}</div></li></ul></div><div><strong>Cuts</strong><ul class="list"><li v-for="item in compareGuidance?.cuts || []" :key="item.key"><div>{{ item.message }}</div><div class="muted">{{ formatGuidanceSource(item.source) }} | {{ formatGuidanceConfidence(item.confidence) }}</div><div class="muted">{{ summarizeGuidanceSupport(item) }}</div></li></ul></div></div></article>
        <article v-if="state.compareResult.candidates?.length" class="card-block"><h3>Top Candidates</h3><div class="meta-deck-table-wrap"><table class="meta-deck-table"><thead><tr><th>Deck</th><th>Class</th><th>Archetype</th><th>Similarity</th><th>Playrate</th><th>Winrate</th><th>Tier</th></tr></thead><tbody><tr v-for="candidate in state.compareResult.candidates" :key="candidate.deck_id"><td>{{ candidate.name }}</td><td>{{ candidate.class }}</td><td>{{ candidate.archetype || t("unknown") }}</td><td>{{ (candidate.similarity * 100).toFixed(1) }}%</td><td>{{ candidate.playrate == null ? t("unknown") : `${candidate.playrate.toFixed(1)}%` }}</td><td>{{ candidate.winrate == null ? t("unknown") : `${candidate.winrate.toFixed(1)}%` }}</td><td>{{ candidate.tier || t("unknown") }}</td></tr></tbody></table></div></article>
      </div>

      <div v-if="state.activeDeckTab === 'report' && state.reportResult" class="stack">
        <div class="meta-row"><span class="pill">{{ state.reportResult.model }}</span><span class="pill">{{ summarizeReportCompare(state.compareResult, undefined, state.locale) }}</span></div>
        <article class="card-block"><h3>Report</h3><pre class="payload-preview">{{ state.reportResult.report }}</pre></article>
        <article v-if="state.reportResult.structured" class="card-block"><h3>Structured Summary</h3><div class="grid two"><div><strong>Deck Identity</strong><ul class="list"><li v-for="item in state.reportResult.structured.deck_identity || []" :key="item">{{ item }}</li></ul></div><div><strong>Doing Well</strong><ul class="list"><li v-for="item in state.reportResult.structured.what_the_deck_is_doing_well || []" :key="item">{{ item }}</li></ul></div><div><strong>Main Risks</strong><ul class="list"><li v-for="item in state.reportResult.structured.main_risks || []" :key="item">{{ item }}</li></ul></div><div><strong>Next Adjustments</strong><ul class="list"><li v-for="item in state.reportResult.structured.practical_next_adjustments || []" :key="item">{{ item }}</li></ul></div></div></article>
        <article class="card-block"><header class="panel-header compact"><div><h3>{{ t("recentReports") }}</h3></div><span class="muted">{{ state.loadingReports || state.loadingReportDetail ? t("loading") : `${state.reportsHistory.length} ${t("itemsSuffix")}` }}</span></header><ul class="list"><li v-for="item in state.reportsHistory" :key="item.id"><button class="meta-history-button" :class="{ active: isSelectedReport(item) }" type="button" @click="loadReportDetail(item.id)"><strong>{{ item.id }}</strong><span class="muted">{{ summarizeStoredReport(item, undefined, state.locale) }}</span></button></li><li v-if="!state.reportsHistory.length" class="muted">{{ t("noSavedReportsYet") }}</li></ul></article>
      </div>
    </section>
    <section class="panel">
      <header class="panel-header"><div><p class="eyebrow">{{ t("meta") }}</p><h2>{{ t("latestSnapshotOverview") }}</h2></div><button class="ghost-button" type="button" @click="loadMetaSnapshot">{{ state.loadingMeta ? t("loading") : t("refresh") }}</button></header>
      <p v-if="state.metaError" class="status error">{{ state.metaError }}</p>
      <p v-if="state.metaSuccess" class="status success">{{ state.metaSuccess }}</p>
      <div v-if="state.metaHistory.length" class="grid two">
        <article class="card-block"><h3>{{ t("recentSnapshots") }}</h3><ul class="list"><li v-for="snapshot in state.metaHistory" :key="snapshot.id"><button class="meta-history-button" :class="{ active: isSelectedMetaSnapshot(snapshot) }" type="button" @click="loadMetaSnapshotDetail(snapshot.id)"><strong>{{ snapshot.id }}</strong><span class="muted">{{ summarizeMetaSnapshot(snapshot, undefined, state.locale) }}</span></button></li></ul></article>
        <article v-if="state.selectedMetaSnapshot" class="card-block"><h3>{{ t("selectedSnapshot") }}</h3><p class="muted">{{ summarizeMetaSnapshot(state.selectedMetaSnapshot, undefined, state.locale) }}</p><div class="meta-deck-table-wrap" v-if="selectedMetaDeckRows().length"><table class="meta-deck-table"><thead><tr><th>Deck</th><th>Class</th><th>Tier</th><th>Playrate</th><th>Winrate</th><th>Sample</th></tr></thead><tbody><tr v-for="deck in selectedMetaDeckRows()" :key="deck.id"><td>{{ deck.name }}</td><td>{{ deck.className }}</td><td>{{ deck.tier }}</td><td>{{ deck.playrateLabel }}</td><td>{{ deck.winrateLabel }}</td><td>{{ deck.sampleSizeLabel }}</td></tr></tbody></table></div><pre class="payload-preview">{{ formatMetaPayloadPreview(state.selectedMetaSnapshot.raw_payload) }}</pre></article>
      </div>
      <article v-else class="card-block"><h3>{{ t("noMetaSnapshotYet") }}</h3><p class="muted">{{ t("noMetaSnapshotBody") }}</p></article>
    </section>

    <section class="panel">
      <header class="panel-header"><div><p class="eyebrow">{{ t("jobs") }}</p><h2>{{ t("jobs") }}</h2></div><button class="ghost-button" type="button" @click="loadJobs">{{ state.loadingJobs ? t("loading") : t("refresh") }}</button></header>
      <p v-if="state.jobsError" class="status error">{{ state.jobsError }}</p>
      <p v-if="state.jobsSuccess" class="status success">{{ state.jobsSuccess }}</p>
      <div class="stack"><article v-for="job in state.jobs" :key="job.key" class="card-block"><header class="panel-header compact"><div><h3>{{ job.key }}</h3><p class="muted">{{ summarizeLatestRun(job, state.locale) }}</p></div><div class="meta-row"><span class="pill" :class="job.enabled ? 'pill-success' : 'pill-error'">{{ job.enabled ? t("enabled") : t("disabled") }}</span><span class="pill">{{ t("next") }} {{ formatTimestamp(job.next_run_at) }}</span></div></header><div class="grid two"><label class="field"><span>{{ t("cronExpression") }}</span><input v-model="job.draftCronExpr" type="text" placeholder="*/15 * * * *" /></label><label class="field"><span>{{ t("enabled") }}</span><input v-model="job.draftEnabled" type="checkbox" /></label></div><div class="actions split-actions"><button class="primary-button" type="button" :disabled="state.savingJobKey === job.key || state.runningJobKey === job.key" @click="saveJob(job)">{{ state.savingJobKey === job.key ? t("saving") : t("saveJobAction") }}</button><button class="ghost-button" type="button" :disabled="state.runningJobKey === job.key || state.savingJobKey === job.key" @click="runJob(job)">{{ state.runningJobKey === job.key ? t("running") : t("runNow") }}</button><button class="ghost-button" type="button" @click="refreshJobHistory(job)">{{ t("refreshHistory") }}</button></div><ul class="list"><li v-for="item in job.history" :key="`${item.id}-${item.started_at}`"><strong :class="item.status === 'success' ? 'success-text' : 'error-text'">{{ formatExecutionStatus(item.status, state.locale) }}</strong><span class="muted">{{ formatTimestamp(item.started_at, t("unknown")) }}</span><span class="muted">{{ t("finished") }}: {{ formatTimestamp(item.finished_at, t("stillRunning")) }}</span><span class="muted">{{ t("duration") }}: {{ formatExecutionDuration(item.started_at, item.finished_at, state.locale) }}</span><span v-if="item.records_affected != null" class="muted">{{ t("recordsAffected") }}: {{ item.records_affected }}</span><span v-if="item.error_message" class="error-text" :title="item.error_message">{{ summarizeJobHistoryError(item.error_message, state.locale) }}</span></li><li v-if="!job.history.length" class="muted">{{ t("noExecutionHistory") }}</li></ul></article></div>
    </section>

    <section class="panel">
      <header class="panel-header"><div><p class="eyebrow">{{ t("settings") }}</p><h2>{{ t("llmConfiguration") }}</h2></div><button class="ghost-button" type="button" @click="loadSettings">{{ state.loadingSettings ? t("loading") : t("refresh") }}</button></header>
      <p v-if="state.settingsError" class="status error">{{ state.settingsError }}</p>
      <p v-if="state.settingsSuccess" class="status success">{{ state.settingsSuccess }}</p>
      <div class="stack"><article v-for="setting in state.settings" :key="setting.key" class="card-block"><h3>{{ setting.key }}</h3><p class="muted">{{ setting.description }}</p><label class="field"><span>{{ t("value") }}</span><input v-model="setting.draftValue" :type="setting.sensitive ? 'password' : 'text'" :placeholder="setting.sensitive ? t('enterSecretValue') : t('enterValue')" /></label><div class="actions"><button class="primary-button" type="button" :disabled="state.savingKey === setting.key" @click="saveSetting(setting)">{{ state.savingKey === setting.key ? t("saving") : t("save") }}</button></div></article></div>
    </section>
  </main>
</template>

<style scoped>
.shell { min-height: 100vh; padding: 2rem 1rem 3rem; color: #f4ecde; background: linear-gradient(145deg, #14110f, #241b15 48%, #0e1823); font-family: Georgia, "Times New Roman", serif; }
.hero, .panel { width: min(100%, 1080px); margin: 0 auto 1rem; padding: 1.25rem; border: 1px solid rgba(255,255,255,0.12); border-radius: 18px; background: rgba(14,18,27,0.58); }
.hero-top, .panel-header, .meta-row, .language-toggle, .actions, .split-actions, .tab-row { display: flex; gap: 0.75rem; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; }
.eyebrow { margin: 0 0 0.5rem; color: #f2a442; letter-spacing: 0.18em; text-transform: uppercase; font-size: 0.8rem; }
.summary, .muted { color: #d9cfbe; line-height: 1.6; }
.field { display: grid; gap: 0.4rem; }
.field input, .field textarea { width: 100%; padding: 0.85rem 1rem; border-radius: 14px; border: 1px solid rgba(255,255,255,0.12); background: rgba(7,10,16,0.6); color: #fff7e7; font: inherit; }
.primary-button, .ghost-button, .tab-button, .meta-history-button { border-radius: 999px; padding: 0.7rem 1rem; font: inherit; cursor: pointer; }
.primary-button { border: none; color: #1f1711; background: linear-gradient(135deg, #f2a442, #ffd38b); }
.ghost-button, .tab-button { color: #f4ecde; background: transparent; border: 1px solid rgba(255,255,255,0.14); }
.tab-button.active, .locale-button.active { border-color: rgba(242,164,66,0.55); color: #ffd38b; }
.status { margin: 0; }
.error, .error-text { color: #ffb2ad; }
.success, .success-text { color: #b5efb8; }
.stack, .grid, .card-block { display: grid; gap: 1rem; }
.grid.two { grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); }
.grid.three { grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); }
.list { list-style: none; padding: 0; margin: 0; display: grid; gap: 0.55rem; }
.card-block { padding: 1rem; border-radius: 14px; background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.08); }
.stats-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.45rem 1rem; margin: 0; }
.stats-list dd { margin: 0; text-align: right; }
.pill { padding: 0.35rem 0.7rem; border-radius: 999px; background: rgba(255,255,255,0.08); color: #f2d39d; }
.pill-success { color: #b5efb8; }
.pill-error { color: #ffb2ad; }
.hash { margin: 0; color: #f2d39d; word-break: break-all; }
.meta-history-button { width: 100%; text-align: left; border: 1px solid rgba(255,255,255,0.12); background: rgba(7,10,16,0.55); color: #fff7e7; }
.payload-preview { margin: 0; padding: 0.9rem 1rem; border-radius: 14px; background: rgba(7,10,16,0.6); border: 1px solid rgba(255,255,255,0.08); color: #d9cfbe; white-space: pre-wrap; word-break: break-word; font-family: "Courier New", monospace; }
.meta-deck-table-wrap { overflow-x: auto; }
.meta-deck-table { width: 100%; border-collapse: collapse; color: #fff7e7; }
.meta-deck-table th, .meta-deck-table td { padding: 0.6rem 0.45rem; border-bottom: 1px solid rgba(255,255,255,0.08); text-align: left; }
@media (max-width: 640px) { .shell { padding: 1rem 0.75rem 2rem; } .stats-list { grid-template-columns: 1fr; } .stats-list dd { text-align: left; } .primary-button, .ghost-button, .tab-button { width: 100%; } }
</style>
