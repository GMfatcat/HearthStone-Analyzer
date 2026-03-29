# v0.1.0 - First Public Baseline

## English

### Features

- Parse Hearthstone deck codes into structured deck lists
- Run deterministic deck analysis with archetype reads, confidence reasons, structural tags, package analysis, and suggested adds/cuts
- Compare decks against stored meta candidates with merged guidance
- Generate AI reports through an OpenAI-compatible LLM endpoint such as Ollama
- Save and replay report history
- Use the web UI in English or Traditional Chinese
- Deploy with a simple single-container setup backed by SQLite

### Limitations

- Some analysis or report content may still remain partly English depending on source content
- Remote meta normalization still has a few edge cases
- Frontend automated coverage is still fairly light
- Scheduler observability and retention are still basic

## 繁體中文

### 功能

- 可將 Hearthstone deck code 解析為結構化牌組清單
- 提供 deterministic 牌組分析，包含 archetype 判讀、confidence reasons、structural tags、package analysis 與 suggested adds/cuts
- 可與已儲存的 meta 候選牌組比對，並提供 merged guidance
- 可透過 OpenAI-compatible LLM endpoint 生成 AI 報告，例如 Ollama
- 可儲存並重播歷史報告
- Web UI 支援英文與繁體中文切換
- 採單容器加 SQLite 的簡單部署模式

### 限制

- 部分分析或報告內容仍可能因來源不同而保留英文
- Remote meta normalization 仍有少數邊角情況
- Frontend 自動化測試覆蓋率仍偏少
- Scheduler 的 observability 與 retention 仍較基礎
