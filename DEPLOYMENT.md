# Deployment Guide

## Runtime Model

The app is designed to run as a single process inside one container:

- Go API server
- static frontend asset serving
- in-process scheduler
- built-in jobs
- SQLite persistence
- automatic SQLite schema migration on startup

There is no Redis, PostgreSQL, or multi-container dependency in the intended MVP deployment.

## Container Defaults

The production image now uses these defaults:

- listen address: `:8080`
- data directory: `/data`
- SQLite path: `/data/hearthstone.db` through `APP_DATA_DIR=/data`

The image declares `/data` as a Docker volume so database state can survive container restarts.

## Required Deployment Inputs

At minimum, set:

- `APP_SETTINGS_KEY`

Why:

- the app will run without overriding it, but the built-in fallback key is only suitable for local development
- production-like deployment should always provide a unique 32-byte key

Important clarification:

- the app expects a raw 32-character key string
- do not pass a 64-character hex string produced by `openssl rand -hex 32`
- example acceptable value:
  - `m7Kp2Qx9Lr4Vz8Nc1Tw6By3Hs5Df0GaJ`

Recommended additional inputs:

- `APP_ADDR`
- `APP_DATA_DIR` or `APP_DB_PATH`
- `APP_CARDS_SOURCE_URL` only if you want to override the default HearthstoneJSON endpoint
- `APP_CARDS_LOCALE` if you want a locale other than `enUS`

Optional meta-source inputs:

- `APP_META_FILE`
- `APP_META_FIXTURE`
- `APP_META_REMOTE_URL`
- `APP_META_REMOTE_TOKEN`
- `APP_META_REMOTE_HEADER_NAME`
- `APP_META_REMOTE_HEADER_VALUE`
- `APP_META_REMOTE_PROFILE`

## Upgrade Safety

Before replacing an existing container image with a newer build:

1. stop the running container
2. back up `/data/hearthstone.db`
3. start the new image with the same `/data` mount and the same `APP_SETTINGS_KEY`

Why:

- the app applies schema migrations automatically during startup
- recent releases now persist richer card metadata in addition to functional tags
- keeping a pre-upgrade backup gives you a fast rollback point if anything unexpected happens

## Recommended Docker Run

If you prefer repo-level shortcuts during local operations, the Makefile now includes:

- `make docker-build`
- `make docker-run`
- `make smoke-health`
- `make smoke-api`

Example with a named volume:

```bash
docker volume create hearthstone-data

docker run -d \
  --name hearthstone-analyzer \
  -p 8080:8080 \
  -e APP_SETTINGS_KEY=replace-with-32-byte-secret \
  -v hearthstone-data:/data \
  hearthstone-analyzer:dev
```

Example with a host bind mount:

```bash
docker run -d \
  --name hearthstone-analyzer \
  -p 8080:8080 \
  -e APP_SETTINGS_KEY=replace-with-32-byte-secret \
  -v /absolute/host/path/hearthstone-data:/data \
  hearthstone-analyzer:dev
```

## Windows Docker + Local Ollama

Validated local settings for Windows Docker Desktop plus host-installed Ollama:

- expose the app on `http://localhost:8080`
- keep Ollama running on the Windows host
- configure the UI to use:
  - `llm.base_url = http://host.docker.internal:11434/v1`
  - `llm.api_key = ollama`
  - `llm.model = <local-model-name>`

Why `host.docker.internal`:

- inside the container, `localhost` points to the container itself
- `host.docker.internal` is the standard Docker Desktop bridge back to the Windows host

PowerShell note for this machine:

```powershell
$env:PATH='C:\Program Files\nodejs;' + $env:PATH
```

## First-Start Checklist

After the container starts:

1. confirm `GET /healthz` returns `ok`
2. open the UI and verify settings page loads
3. set `llm.base_url`, `llm.api_key`, and `llm.model` in the UI
4. run `sync_cards` manually from the Jobs panel
5. verify cards are available and deck parse/analyze works
6. if meta is configured, run `sync_meta` manually and verify snapshot history appears
7. generate one AI report and confirm:
   - report text renders
   - structured summary appears when schema-compliant output is returned
   - report history can reload the saved detail
   - local Ollama requests complete successfully if you are using a host model
8. run one compare request and confirm:
   - merged recommendations render
   - structured compare guidance shows source, confidence, and support text
9. optionally switch the UI language and confirm the choice persists after refresh

## Smoke Test Endpoints

Useful checks after deployment:

- `GET /healthz`
- `GET /api/settings`
- `GET /api/jobs`
- `GET /api/meta/latest?format=standard`
- `GET /api/reports`
- `POST /api/decks/compare`

Expected behavior:

- `GET /healthz` should always return `200 ok`
- `GET /api/meta/latest?format=standard` may return `404` before any meta sync has run
- `GET /api/reports` may return an empty list on a fresh deployment

Repo shortcut:

```bash
make smoke-api
```

Current `make smoke-api` checks:

- `/healthz`
- `/api/jobs`
- `/api/reports`
- `/api/settings`
- `/api/meta/latest?format=standard` with `404` tolerated on fresh installs

Manual compare smoke payload example:

```bash
curl -fsS \
  -H "Content-Type: application/json" \
  -d '{"deck_code":"<deck-code-here>","limit":3}' \
  http://localhost:8080/api/decks/compare
```

## Operational Notes

- The scheduler runs inside the same app process, so stopping the container also stops scheduled jobs
- Persistent state lives primarily in SQLite under `/data`
- Startup automatically runs any pending SQLite migrations before the API begins serving traffic
- Frontend assets are embedded into the Go binary at build time, so rebuilding the image is required after frontend changes
- the runtime image now includes `ca-certificates`, which is required for HTTPS card sync from HearthstoneJSON
- If you change frontend code locally, run `npm run build` before the final Go test/build pass
- startup logs now include:
  - `addr`
  - `db_path`
  - `data_dir`
  - `cards_locale`
  - `meta_source_mode`
  - whether the app is still using the default `APP_SETTINGS_KEY`
  - scheduler job summaries with `key`, `enabled`, `cron_expr`, `next_run_at`, and `last_run_at`

## Current Limitations

- there is not yet a dedicated retention policy for job logs
- structured logs are still basic
- backup and restore are documented, but still manual/operator-driven rather than automated

For concrete backup and restore steps, see [BACKUP_RESTORE.md](D:\HearthStone\BACKUP_RESTORE.md).
