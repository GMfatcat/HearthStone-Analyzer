# HearthStone Analyzer

HearthStone Analyzer is a single-container deck analysis app for Hearthstone.
It parses deck codes, runs rule-based analysis, compares your list against stored meta decks, and can generate AI reports through an OpenAI-compatible endpoint such as local Ollama.

## What It Does

Current usable features:

- sync collectible cards from HearthstoneJSON into local SQLite
- parse Hearthstone deck codes into card lists with legality checks
- run rule-based deck analysis with:
  - archetype classification
  - confidence scoring and confidence reasons
  - structural tag explanations
  - package analysis
  - suggested adds and cuts
- compare your deck against persisted meta decks with:
  - ranked candidates
  - similarity breakdown
  - shared / missing card diff
  - merged guidance with source, support, and confidence
- generate AI deck reports through an OpenAI-compatible chat API
- store and replay past reports
- manage app settings in the UI
- run built-in jobs from the UI:
  - `sync_cards`
  - `sync_meta`
  - `rebuild_features`
- switch the UI between English and Traditional Chinese

## Architecture

The project is intentionally simple to deploy:

- single Go application
- Vue frontend embedded into the Go binary
- SQLite only
- in-process scheduler
- single-container deployment target

There is no Redis, PostgreSQL, or multi-service orchestration requirement for the current product shape.

## Main Screens

The web UI currently includes:

- deck input and parse/analyze/compare/report actions
- analysis view with structural and package reads
- compare view with candidate decks and merged guidance
- report view with saved history replay
- meta snapshot overview
- scheduler/jobs control
- settings page for LLM configuration

## Where To Get Deck Codes

You can paste standard Hearthstone deck codes from places like:

- [Hearthstone Top Decks](https://www.hearthstonetopdecks.com/)
- [Vicious Syndicate](https://www.vicioussyndicate.com/)
- [HSReplay](https://hsreplay.net/)

Look for buttons such as `Copy Deck Code` on those sites.

You can also use this sample deck code for quick testing:

```text
AAIB8eEEAA-zAY0Qt2ziygLP0QPboASFoQSC5ASL7AWi-gXHpAbd5QaKsQeEAZ4BAA
```

## Project Docs

Primary docs:

- [PRD_v2.md](D:\HearthStone\PRD_v2.md)
- [IMPLEMENTATION_PLAN.md](D:\HearthStone\IMPLEMENTATION_PLAN.md)
- [CURRENT_PROGRESS.md](D:\HearthStone\CURRENT_PROGRESS.md)
- [DEPLOYMENT.md](D:\HearthStone\DEPLOYMENT.md)
- [BACKUP_RESTORE.md](D:\HearthStone\BACKUP_RESTORE.md)

## Local Development

### Backend

Requirements:

- Go 1.21+

Commands:

```bash
go test ./...
go run ./cmd/api
go run ./cmd/sync_cards
```

Repo shortcuts:

```bash
make test
make build
make run
```

Defaults:

- HTTP address: `:8080`
- SQLite path: `data/hearthstone.db`

Useful environment variables:

- `APP_ADDR`
- `APP_DB_PATH`
- `APP_DATA_DIR`
- `APP_SETTINGS_KEY`
- `APP_CARDS_SOURCE_URL`
- `APP_CARDS_LOCALE`
- `APP_META_FILE`
- `APP_META_FIXTURE`
- `APP_META_REMOTE_URL`
- `APP_META_REMOTE_TOKEN`
- `APP_META_REMOTE_HEADER_NAME`
- `APP_META_REMOTE_HEADER_VALUE`
- `APP_META_REMOTE_PROFILE`

### Frontend

The frontend is built with Vue + Vite and outputs to `web/dist/`.

Commands:

```bash
cd web
npm install
npm test
npm run build
```

Repo shortcuts:

```bash
make frontend-install
make frontend-test
make frontend-build
make verify
```

Windows PowerShell note for this machine:

```powershell
$env:PATH='C:\Program Files\nodejs;' + $env:PATH
& 'C:\Program Files\nodejs\npm.cmd' test
& 'C:\Program Files\nodejs\npm.cmd' run build
```

### Important Build Order

The Go app embeds files from `web/dist` via [web/embed.go](D:\HearthStone\web\embed.go).

After frontend changes, use this final verification order:

```bash
cd web
npm test
npm run build
cd ..
go test ./...
```

If `go test ./...` fails with an embed error like `web\embed.go: pattern dist/*: no matching files found`, rebuild the frontend first.

## API Surface

Current endpoints:

- `GET /healthz`
- `GET /api/settings`
- `GET /api/settings/{key}`
- `PUT /api/settings/{key}`
- `GET /api/cards`
- `GET /api/cards/{id}`
- `POST /api/decks/parse`
- `POST /api/decks/analyze`
- `POST /api/decks/compare`
- `POST /api/reports/generate`
- `GET /api/reports`
- `GET /api/reports/{id}`
- `GET /api/jobs`
- `GET /api/jobs/{key}`
- `PUT /api/jobs/{key}`
- `POST /api/jobs/{key}/run`
- `GET /api/jobs/{key}/history`
- `GET /api/meta/latest`
- `GET /api/meta`
- `GET /api/meta/{id}`

## Deployment

For the full deployment guide, see [DEPLOYMENT.md](D:\HearthStone\DEPLOYMENT.md).

### Docker Build

```bash
docker build -t hearthstone-analyzer:dev .
```

### Basic Docker Run

```bash
docker run --rm -p 8080:8080 hearthstone-analyzer:dev
```

### Recommended Persistent Run

Named volume:

```bash
docker volume create hearthstone-data

docker run -d \
  --name hearthstone-analyzer \
  -p 8080:8080 \
  -e APP_SETTINGS_KEY=replace-with-32-char-secret \
  -v hearthstone-data:/data \
  hearthstone-analyzer:dev
```

Bind mount:

```bash
docker run -d \
  --name hearthstone-analyzer \
  -p 8080:8080 \
  -e APP_SETTINGS_KEY=replace-with-32-char-secret \
  -v /absolute/host/path:/data \
  hearthstone-analyzer:dev
```

Important:

- `APP_SETTINGS_KEY` must be a raw 32-character string
- do not use a 64-character hex string from `openssl rand -hex 32`

Example valid key:

```text
m7Kp2Qx9Lr4Vz8Nc1Tw6By3Hs5Df0GaJ
```

## Windows Docker + Local Ollama

This path has been validated locally.

### Start the Container

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

### Configure Ollama in the UI

Open `http://localhost:8080` and set:

- `llm.base_url = http://host.docker.internal:11434/v1`
- `llm.api_key = ollama`
- `llm.model = <your-local-model>`

Example local model:

- `qwen3.5:2b`

Why `host.docker.internal`:

- inside Docker, `localhost` points at the container
- `host.docker.internal` points back to the Windows host running Ollama

### Validate Ollama

Host-side quick check:

```powershell
Invoke-RestMethod http://localhost:11434/v1/models
```

Then in the app:

1. run `sync_cards`
2. paste a deck code
3. click `Parse`
4. click `Analyze`
5. click `Generate Report`

## First-Start Smoke Test

Recommended checklist after startup:

1. `GET /healthz` returns `ok`
2. UI loads
3. settings can be saved
4. `sync_cards` succeeds
5. parse works
6. analyze works
7. compare works if meta is available
8. report generation works
9. recent report replay works
10. UI language switch persists after refresh

## Current Notes

- the runtime image includes `ca-certificates`, which is required for HTTPS card sync
- report generation timeout has been sized to better tolerate slower local Ollama inference
- `Analyze` is partially localized without calling an external translation service
- report language now follows the UI language for LLM-generated output

## Backup and Restore

See [BACKUP_RESTORE.md](D:\HearthStone\BACKUP_RESTORE.md).

At minimum, back up your SQLite file before upgrades:

- `/data/hearthstone.db` inside the container
- or your host-mounted `data` directory if using a bind mount

## Validation Status

Last confirmed verification:

- `go test ./...`
- `web`:
  - `npm test`
  - `npm run build`
- Windows Docker local deployment
- local Ollama report generation

## Known Limitations

- some analyze/report wording is still partly English depending on content source
- remote meta card-name normalization still has edge cases
- frontend automated coverage is still fairly light
- scheduler logging/retention is still basic

## Dev Container

The repository includes a Dev Container with:

- Go
- Node.js
- common build tooling

Use it if you want a consistent local environment for backend and frontend work.
