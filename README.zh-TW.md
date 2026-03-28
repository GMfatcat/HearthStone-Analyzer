# HearthStone Analyzer

`HearthStone Analyzer` 是一個以單容器部署為目標的 Hearthstone 牌組分析工具。
它可以解析 deck code、做規則式分析、比對已儲存的 meta 牌組，並透過 OpenAI-compatible API 生成 AI 報告，例如本機 Ollama。

## 功能介紹

目前已可使用的核心功能：

- 從 HearthstoneJSON 同步卡牌到本地 SQLite
- 解析 Hearthstone deck code 與合法性檢查
- 規則式牌組分析：
  - archetype 判讀
  - confidence 與 confidence reasons
  - structural tags 與說明
  - package analysis
  - suggested adds / cuts
- 與已儲存的 meta 牌組比對：
  - 候選牌組排序
  - similarity breakdown
  - shared / missing cards diff
  - merged guidance，包含 source / support / confidence
- 透過 OpenAI-compatible chat API 生成 AI 牌組報告
- 儲存報告並可從歷史記錄重新開啟
- 內建 Settings / Jobs UI
- UI 可一鍵切換英文 / 繁中

## 架構

目前專案維持簡單部署：

- 單一 Go 應用程式
- Vue 前端編譯後嵌入 Go binary
- SQLite 單機儲存
- in-process scheduler
- 單容器部署

## Deck Code 來源

你可以從這些網站取得 Hearthstone deck code：

- [Hearthstone Top Decks](https://www.hearthstonetopdecks.com/)
- [Vicious Syndicate](https://www.vicioussyndicate.com/)
- [HSReplay](https://hsreplay.net/)

通常頁面上會有 `Copy Deck Code` 按鈕。

測試用 deck code：

```text
AAIB8eEEAA-zAY0Qt2ziygLP0QPboASFoQSC5ASL7AWi-gXHpAbd5QaKsQeEAZ4BAA
```

## 重要文件

- [README.md](D:\HearthStone\README.md)
- [CURRENT_PROGRESS.md](D:\HearthStone\CURRENT_PROGRESS.md)
- [DEPLOYMENT.md](D:\HearthStone\DEPLOYMENT.md)
- [BACKUP_RESTORE.md](D:\HearthStone\BACKUP_RESTORE.md)
- [IMPLEMENTATION_PLAN.md](D:\HearthStone\IMPLEMENTATION_PLAN.md)
- [PRD_v2.md](D:\HearthStone\PRD_v2.md)

## 本地開發

### Backend

需求：

- Go 1.21+

常用指令：

```bash
go test ./...
go run ./cmd/api
go run ./cmd/sync_cards
```

### Frontend

```bash
cd web
npm install
npm test
npm run build
```

Windows PowerShell：

```powershell
$env:PATH='C:\Program Files\nodejs;' + $env:PATH
& 'C:\Program Files\nodejs\npm.cmd' test
& 'C:\Program Files\nodejs\npm.cmd' run build
```
## 部署方式

完整部署說明請看 [DEPLOYMENT.md](D:\HearthStone\DEPLOYMENT.md)。

### Docker Build

```bash
docker build -t hearthstone-analyzer:dev .
```

### 建議的持久化啟動方式

```bash
docker run -d \
  --name hearthstone-analyzer \
  -p 8080:8080 \
  -e APP_SETTINGS_KEY=replace-with-32-char-secret \
  -v /absolute/host/path:/data \
  hearthstone-analyzer:dev
```

### APP_SETTINGS_KEY 注意事項

- 必須是原始 32 字元字串
- 不要使用 `openssl rand -hex 32` 產生的 64 字元 hex

範例：

```text
m7Kp2Qx9Lr4Vz8Nc1Tw6By3Hs5Df0GaJ
```

## Windows Docker + 本機 Ollama

已實際驗證可用。

### 啟動容器

```powershell
cd D:\HearthStone
docker build -t hearthstone-analyzer:dev .

docker run -d `
  --name hearthstone-analyzer `
  -p 8080:8080 `
  -e APP_SETTINGS_KEY=m7Kp2Qx9Lr4Vz8Nc1Tw6By3Hs5Df0GaJ `
  -v D:\HearthStone\data:/data `
  hearthstone-analyzer:dev
```

### UI 內設定 Ollama

- `llm.base_url = http://host.docker.internal:11434/v1`
- `llm.api_key = ollama`
- `llm.model = <你的本機模型名稱>`

例如：`qwen3.5:2b`

### Ollama 快速驗證

```powershell
Invoke-RestMethod http://localhost:11434/v1/models
```

然後在 UI：

1. 跑 `sync_cards`
2. 貼 deck code
3. 按 `Parse`
4. 按 `Analyze`
5. 按 `Generate Report`

## 首次啟動 Smoke Test

1. `GET /healthz` 回 `ok`
2. UI 可正常打開
3. settings 可存檔
4. `sync_cards` 成功
5. parse / analyze 成功
6. compare 成功
7. report 成功
8. Recent Reports 可重播
9. 中英切換後 refresh 仍保留語系

## 最近驗證狀態

已確認通過：

- `go test ./...`
- `web/npm test`
- `web/npm run build`
- Windows Docker 本地部署
- 本機 Ollama 報告生成

## 已知限制

- `Analyze` 與 `Report` 仍有部分內容可能殘留英文
- remote meta card-name normalization 還有邊角 case
- frontend 自動測試覆蓋率仍偏少
- scheduler logging / retention 仍較基礎
