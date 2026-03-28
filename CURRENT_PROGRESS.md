# CURRENT_PROGRESS

## Project Snapshot

Project: HearthStone Analyzer  
Primary spec: [PRD_v2.md](D:\HearthStone\PRD_v2.md)  
Task breakdown: [IMPLEMENTATION_PLAN.md](D:\HearthStone\IMPLEMENTATION_PLAN.md)

Current confirmed architecture:

- single Go application
- SQLite only
- no Redis
- Vue frontend compiled to static assets and served by Go
- single-container deployment target
- Windows local development is supported

## Current Overall Status

The project is now beyond the original parser/analyzer prototype and has multiple usable end-to-end product slices.

Current working vertical slices:

- cards sync and local card storage via HearthstoneJSON
- deck parsing
- rule-based deck analysis
- analysis confidence scoring and confidence-reason explanations
- settings management with encrypted API key storage
- built-in scheduler/job control with frontend UI
- persisted UI locale switch for English / Traditional Chinese
- deterministic first-pass Chinese localization for major Analyze UI text without requiring LLM translation
- meta snapshot storage, list/detail APIs, and latest snapshot read path
- remote meta source support with configurable headers/auth
- validated `Vicious Syndicate` remote profile crawl path:
  - latest report discovery
  - deck-library link discovery
  - deck variant page discovery
  - card line extraction
- validated `Hearthstone Top Decks` remote profile crawl path:
  - tier section discovery
  - deck anchor discovery
  - class inference
  - card line extraction
  - sideboard exclusion
- meta deck persistence into:
  - `decks`
  - `meta_decks`
  - `deck_cards` when card names can be resolved locally
- compare API and frontend UI with:
  - candidate ranking
  - similarity score
  - similarity breakdown:
    - overlap
    - curve
    - card type
  - qualitative match summary
  - shared / missing card diff
  - suggested adds / cuts
- compare fallback for meta decks that have no deckstring but do have persisted `deck_cards`
- AI report generation vertical slice with:
  - OpenAI-compatible chat completion client
  - prompt assembly from analysis + optional compare context
  - structured report shaping with JSON-first output parsing and markdown fallback
  - report generation API
  - persisted report storage
  - recent report list API
  - report detail API
  - frontend report tab and recent-report history/detail replay
  - Windows Docker + local Ollama validation through the OpenAI-compatible `/v1` path
  - report output language now follows UI locale through prompt-level language steering
  - prompt guardrails for unavailable compare context and no-fabrication wording
  - structured summary replay in API and UI when provider output is schema-compliant
  - structured payload sanitize/replay on generated, persisted, and frontend hydrate paths
- analysis-native deck tuning output with:
  - suggested adds
  - suggested cuts
  - structural tags exposed in API and UI

The biggest unfinished product areas are now:

- richer rule-based analysis heuristics beyond the current first-pass structural tagging layer
- stronger card-name normalization for the remaining remote meta deck edge cases
- compare-aware recommendation/report synthesis that combines analysis-native and meta-context signals
- deeper AI report content-quality guardrails beyond the current grounded/schema-enforced pass
- fuller Chinese localization coverage for every analysis recommendation string

## External Source Validation

As of 2026-03-27, current source validation is:

### Cards

- HearthstoneJSON is confirmed usable and remains the canonical MVP source
- current live endpoint still works for collectible cards JSON

### Meta

- `HSReplay` remains a candidate, but stable public machine-readable access is still uncertain
- `Vicious Syndicate` is now the first validated real remote profile:
  - latest report page can be discovered
  - report pages link to deck-library entries
  - deck-library pages link to concrete deck variant pages
  - deck variant pages expose enough metadata and card lines for structured ingestion
- `Hearthstone Top Decks` is now the second validated real remote profile:
  - standard meta page can be fetched and parsed
  - tier sections and deck anchors can be extracted
  - deck card lines can be ingested into the existing snapshot schema
  - end-to-end `sync_meta` persistence into `meta_decks` and `deck_cards` has been manually validated
- `Tempo Storm` is not a viable current source and should not be prioritized

## Milestone Status

### Milestone 0: Project Foundation

Status: Done

Completed:

- Go module and project skeleton
- API entrypoint at `cmd/api`
- static frontend assets served by Go
- health endpoint: `GET /healthz`
- Dockerfile for single-container deployment
- Dev Container baseline
- Vue + Vite frontend baseline

Key files:

- [go.mod](D:\HearthStone\go.mod)
- [cmd/api/main.go](D:\HearthStone\cmd\api\main.go)
- [Dockerfile](D:\HearthStone\Dockerfile)
- [.devcontainer/devcontainer.json](D:\HearthStone\.devcontainer\devcontainer.json)

### Milestone 1: SQLite and Core Persistence

Status: Mostly done for current scope

Completed:

- SQLite open + migrate
- idempotent migrations
- schema bootstrapped on startup
- repositories implemented for:
  - settings
  - scheduled jobs
  - job execution logs
  - cards
  - card lookup by DBF ID
  - meta snapshots
  - decks
  - meta decks
  - deck cards
- persisted meta deck card storage now actively used by remote meta sync when names can be resolved

Current schema includes:

- `app_settings`
- `cards`
- `card_locales`
- `decks`
- `deck_cards`
- `meta_snapshots`
- `meta_decks`
- `analysis_reports`
- `scheduled_jobs`
- `job_execution_logs`
- `card_sync_runs`
- `schema_migrations`

Still missing:

- more formal migration file layout

Key files:

- [internal/storage/sqlite/sqlite.go](D:\HearthStone\internal\storage\sqlite\sqlite.go)
- [internal/storage/sqlite/repositories.go](D:\HearthStone\internal\storage\sqlite\repositories.go)
- [internal/storage/sqlite/cards_repository.go](D:\HearthStone\internal\storage\sqlite\cards_repository.go)
- [internal/storage/sqlite/card_lookup_repository.go](D:\HearthStone\internal\storage\sqlite\card_lookup_repository.go)

### Milestone 2: Settings and Secret Storage

Status: Done for first usable version

Completed:

- allowed settings catalog
- AES-GCM encryption for sensitive settings
- encrypted storage for `llm.api_key`
- settings service
- settings API:
  - `GET /api/settings`
  - `GET /api/settings/{key}`
  - `PUT /api/settings/{key}`
- settings UI

Current supported settings:

- `llm.api_key`
- `llm.base_url`
- `llm.model`

Still missing:

- richer validation rules
- settings audit history
- more explicit runtime reload behavior for future settings categories

Key files:

- [internal/settings/service.go](D:\HearthStone\internal\settings\service.go)
- [internal/httpapi/router.go](D:\HearthStone\internal\httpapi\router.go)
- [web/src/App.vue](D:\HearthStone\web\src\App.vue)

### Milestone 3: Cards Sync

Status: Done for first usable version

Completed:

- HearthstoneJSON adapter
- card sync service
- card upsert into `cards` and `card_locales`
- sync metadata recorded in `card_sync_runs`
- manual sync command:
  - `go run ./cmd/sync_cards`
- cards query API:
  - `GET /api/cards`
  - `GET /api/cards/{id}`
- cards filters:
  - `class`
  - `set`
  - `cost`
- localized-name lookup helper for mapping external meta card lines back to local cards

Still missing:

- official card source adapter
- richer locale handling
- dedicated cards page in the frontend

Key files:

- [internal/cards/cards.go](D:\HearthStone\internal\cards\cards.go)
- [cmd/sync_cards/main.go](D:\HearthStone\cmd\sync_cards\main.go)
- [internal/cardquery/service.go](D:\HearthStone\internal\cardquery\service.go)

### Milestone 4: Deck Parser

Status: Done for first usable version

Completed:

- deckstring decoding
- hero / format / card count parsing
- DBF ID resolution through SQLite
- stable deck hash
- legality checks:
  - total count
  - class mismatch
  - standard legality
  - duplicate copy rules
  - legendary duplicate rules
- typed parser errors
- parse API:
  - `POST /api/decks/parse`

Current typed parser errors:

- `invalid_deck_code`
- `hero_not_found`
- `card_not_found`
- `lookup_failed`

Still missing:

- more refined legality rules by format/class specifics
- parser persistence of parsed user decks into DB

Key files:

- [internal/decks/parser.go](D:\HearthStone\internal\decks\parser.go)
- [internal/decks/errors.go](D:\HearthStone\internal\decks\errors.go)

### Milestone 5: Rule-Based Analysis Engine

Status: In progress, already usable and substantially deeper than the original first pass

Completed:

- feature extraction from parsed decks
- archetype classification:
  - `Aggro`
  - `Midrange`
  - `Control`
- strengths / weaknesses generation
- parse + analyze orchestration service
- analyze API:
  - `POST /api/decks/analyze`
- frontend parse/analyze workbench
- confidence scoring for archetype classification
- confidence-reason explanation output in API and UI
- structural tag inference for deck-shape diagnosis
- analysis-native `suggested_adds` / `suggested_cuts` output in API and UI
- readable structural tag explanations in API and UI
- package-aware analysis output:
  - `package_details`
  - underbuilt / balanced / overbuilt / conflict states
  - package target ranges and explanations
- package-driven recommendation synthesis for analysis-native adds/cuts
- normalized functional-role summary in API and UI
- persisted `cards.functional_tags` storage and parser hydrate path

Current feature signals include:

- `avg_cost`
- `mana_curve`
- `minion_count`
- `spell_count`
- `weapon_count`
- `early_curve_count`
- `top_heavy_count`
- `early_game_score`
- `mid_game_score`
- `late_game_score`
- `curve_balance_score`
- `draw_count`
- `discover_count`
- `single_removal_count`
- `aoe_count`
- `heal_count`
- `burn_count`
- `taunt_count`
- `token_count`
- `deathrattle_count`
- `battlecry_count`
- `mana_cheat_count`
- `combo_piece_count`

Current structural tags include:

- `thin_early_board`
- `low_refill`
- `light_card_draw`
- `thin_early_curve`
- `light_reactive_package`
- `light_late_game_payoff`
- `reactive_spell_saturation`
- `heavy_top_end`
- `clunky_curve`
- `aggro_top_end_conflict`
- `control_early_slot_overload`

Still missing:

- stronger archetype heuristics
- richer card metadata normalization beyond the current persisted text-derived functional tags
- finer-grained package taxonomy beyond the current broad buckets
- structured compare/report recommendation objects built on top of current merged guidance

Key files:

- [internal/analysis/service.go](D:\HearthStone\internal\analysis\service.go)
- [internal/deckanalysis/service.go](D:\HearthStone\internal\deckanalysis\service.go)
- [web/src/App.vue](D:\HearthStone\web\src\App.vue)

### Milestone 6: Scheduler and Job Control

Status: Done for first usable version

Completed:

- in-process scheduler engine
- scheduler bootstrap from DB state
- immediate reload on job changes
- duplicate-run prevention across manual and scheduled execution
- scheduled job persistence usage
- built-in jobs:
  - `sync_cards`
  - `sync_meta`
  - `rebuild_features`
- job management API:
  - `GET /api/jobs`
  - `GET /api/jobs/{key}`
  - `PUT /api/jobs/{key}`
  - `POST /api/jobs/{key}/run`
  - `GET /api/jobs/{key}/history`
- job management frontend UI
- `sync_cards` wired as a real managed built-in job
- `sync_meta` wired as a real managed built-in job with profile-based source selection

Still missing:

- richer scheduler observability/logging polish
- retry policy
- history retention policy
- broader frontend test coverage beyond helper-level tests
- more complete localization coverage for AI-generated / backend-provided English content

Key files:

- [internal/jobs/service.go](D:\HearthStone\internal\jobs\service.go)
- [internal/jobs/scheduler.go](D:\HearthStone\internal\jobs\scheduler.go)
- [internal/jobs/execution_gate.go](D:\HearthStone\internal\jobs\execution_gate.go)
- [internal/httpapi/router.go](D:\HearthStone\internal\httpapi\router.go)
- [web/src/App.vue](D:\HearthStone\web\src\App.vue)

### Milestone 7: Meta Adapter Framework

Status: In progress, substantially advanced and now validated against two real remote profiles

Completed:

- meta source interface in code
- meta snapshot repository
- meta sync service with adapter-based source contract
- fallback unavailable source
- fixture meta source for local/test success-path validation
- file-based experimental meta adapter via `APP_META_FILE`
- generic remote meta source via `APP_META_REMOTE_URL`
- remote auth/header configuration:
  - `APP_META_REMOTE_TOKEN`
  - `APP_META_REMOTE_HEADER_NAME`
  - `APP_META_REMOTE_HEADER_VALUE`
- remote profile selection:
  - `APP_META_REMOTE_PROFILE`
- current validated production-style profile:
  - `vicioussyndicate`
- second validated production-style profile:
  - `hearthstonetopdecks`
- schema normalization for multiple remote payload shapes and aliases
- `Vicious Syndicate` crawl path validation and implementation:
  - latest report discovery
  - deck-library discovery
  - deck variant discovery
  - deck card line extraction
- `Hearthstone Top Decks` crawl path validation and implementation:
  - tier section discovery
  - deck anchor discovery
  - class inference
  - card line extraction
  - sideboard exclusion
- `sync_meta` runner wired into app/scheduler with source selection:
  - file source if `APP_META_FILE` is set
  - profile-aware remote source if `APP_META_REMOTE_URL` is set
  - fixture source if `APP_META_FIXTURE` is set
  - fallback unavailable source otherwise
- latest meta snapshot query service
- meta snapshot APIs:
  - `GET /api/meta/latest`
  - `GET /api/meta`
  - `GET /api/meta/{id}`
- frontend meta overview panel with snapshot history and selected snapshot detail
- meta deck persistence / remapping into:
  - `decks`
  - `meta_decks`
  - `deck_cards` when remote card names resolve to local cards

Still missing:

- broader remote source coverage beyond the current two validated profiles
- stronger card-name normalization for the remaining external edge cases
- richer meta overview UI beyond snapshot/deck summary
- explicit source-specific profiles for additional sites

Key files:

- [internal/meta/service.go](D:\HearthStone\internal\meta\service.go)
- [internal/meta/query.go](D:\HearthStone\internal\meta\query.go)
- [internal/meta/remote_source.go](D:\HearthStone\internal\meta\remote_source.go)
- [internal/meta/vicioussyndicate_source.go](D:\HearthStone\internal\meta\vicioussyndicate_source.go)
- [internal/meta/fixture_source.go](D:\HearthStone\internal\meta\fixture_source.go)
- [internal/meta/file_source.go](D:\HearthStone\internal\meta\file_source.go)
- [internal/meta/fallback_source.go](D:\HearthStone\internal\meta\fallback_source.go)
- [internal/httpapi/router.go](D:\HearthStone\internal\httpapi\router.go)
- [web/src/App.vue](D:\HearthStone\web\src\App.vue)

### Milestone 8: Deck Comparison

Status: In progress, already usable

Completed:

- compare service
- similarity scoring based on card overlap
- compare API:
  - `POST /api/decks/compare`
- compare UI in the main workbench
- compare response includes:
  - ranked candidates
  - similarity
  - similarity breakdown
  - qualitative summary
  - shared cards
  - missing cards
  - suggested adds
  - suggested cuts
- compare can use:
  - persisted meta deck deckstrings
  - persisted `deck_cards`
  - raw snapshot card-line fallback when needed
- compare sorting now uses explicit tie-break rules:
  - similarity
  - tier
  - playrate
  - winrate
  - deck id
- compare-aware merged recommendation output:
  - `merged_summary`
  - `merged_suggested_adds`
  - `merged_suggested_cuts`
- merge logic between:
  - analysis package state
  - closest meta candidate evidence
- weighting across the top 2-3 meta candidates when they agree
- conservative compare guidance when top candidates disagree

Still missing:

- structured compare-aware recommendation objects with explicit source/support/confidence fields
- dedicated compare page instead of workbench-only presentation

Key files:

- [internal/compare/service.go](D:\HearthStone\internal\compare\service.go)
- [internal/httpapi/router.go](D:\HearthStone\internal\httpapi\router.go)
- [web/src/App.vue](D:\HearthStone\web\src\App.vue)

### Milestone 9: AI Report Layer

Status: In progress, substantially more complete and usable end-to-end

Completed:

- internal report service
- OpenAI-compatible chat completion client
- prompt assembly from:
  - analysis output
  - optional compare output
- report generation API:
  - `POST /api/reports/generate`
- report persistence into `analysis_reports`
- recent report list API:
  - `GET /api/reports`
- report detail API:
  - `GET /api/reports/{id}`
- frontend report tab
- frontend recent-report history/detail replay
- saved compare-context replay in generated and persisted report flows
- structured report payload in generated and persisted report flows
- one-click English / Traditional Chinese UI toggle with saved preference
- prompt guardrails for:
  - unavailable compare context
  - no invented meta deck / snapshot / tier / winrate claims
- structured payload shape validation with:
  - per-section item limits
  - per-item length limits
  - duplicate item removal
  - minimum section/detail thresholds
- structured payload groundedness validation for:
  - snapshot claims
  - tier claims
  - winrate claims
  - playrate claims
  - unsupported percentage claims
- safe plain-text fallback when provider structured JSON is invalid
- persisted report detail replay now re-sanitizes stored structured payload
- frontend report hydrate path now re-sanitizes stored structured payload
- frontend helper tests for report history/detail view model
- merged compare-aware recommendation context included in report prompt assembly

Still missing:

- provider abstraction expansion beyond the current OpenAI-compatible implementation
- broader near-duplicate / low-information content guardrails
- deeper end-to-end report integration coverage across generate -> persist -> replay
- structured report consumption of deterministic compare-aware recommendation objects once they exist

### Milestone 10: Packaging and Hardening

Status: Partially done

Completed:

- Docker image builds successfully
- frontend assets compile successfully
- app runs as single process
- container data directory convention set to `/data`
- Docker volume declaration added for persisted app data
- startup logging now includes:
  - `addr`
  - `db_path`
  - `data_dir`
  - `cards_locale`
  - `meta_source_mode`
  - default-settings-key warning state
  - scheduler job summaries
- repo-level Makefile targets now cover:
  - backend test/build/run
  - frontend install/test/build
  - combined verification
  - Docker build/run
  - health smoke check
  - basic API smoke check
- deployment guide documented with:
  - runtime model
  - environment variables
  - volume conventions
  - first-start smoke checklist
- backup/restore guide documented with:
  - bind-mount backup flow
  - named-volume backup flow
  - restore procedure
  - settings-key constraint after restore

Still missing:

- operational logging polish
- automated backup strategy / scheduling
- more deployment automation beyond the now-documented Windows local Docker path

## Current User-Facing Features

### APIs

Available now:

- `GET /healthz`
- `GET /api/settings`
- `GET /api/settings/{key}`
- `PUT /api/settings/{key}`
- `GET /api/jobs`
- `GET /api/jobs/{key}`
- `PUT /api/jobs/{key}`
- `POST /api/jobs/{key}/run`
- `GET /api/jobs/{key}/history`
- `GET /api/meta/latest`
- `GET /api/meta`
- `GET /api/meta/{id}`
- `GET /api/cards`
- `GET /api/cards/{id}`
- `POST /api/decks/parse`
- `POST /api/decks/analyze`
- `POST /api/decks/compare`
- `POST /api/reports/generate`
- `GET /api/reports`
- `GET /api/reports/{id}`

### Frontend

Available now:

- deck code input
- parse action
- analyze action
- compare action
- parse/analyze/compare result tabs
- parse summary shown inside analyze tab
- analysis structural tags display in analyze tab
- analysis structural tag explanations display in analyze tab
- analysis package-read display in analyze tab
- analysis functional-role summary display in analyze tab
- compare candidate list with:
  - similarity
  - similarity breakdown
  - qualitative summary
  - card diffs
  - suggested adds/cuts
- compare merged recommendation panel with:
  - merged summary
  - merged adds
  - merged cuts
- AI report generation action
- AI report result tab
- recent AI report history list
- persisted AI report detail replay
- saved compare-context replay inside report detail
- structured summary rendering inside report detail when available
- settings editor
- meta overview panel
- snapshot history display
- selected snapshot detail view
- selected snapshot meta deck summary table
- job management panel
- job history display
- manual job run action
- separated parse/analyze/settings/meta/jobs error messaging

Main frontend file:

- [web/src/App.vue](D:\HearthStone\web\src\App.vue)

## Commands That Work Right Now

### Backend

```bash
go test ./...
go run ./cmd/api
go run ./cmd/sync_cards
make test
make build
make run
```

### Frontend

```bash
cd web
npm install
npm test
npm run build
make frontend-install
make frontend-test
make frontend-build
make verify
```

Important Windows note:

- if `npm` or `node` is not found in PowerShell, prepend:
  - `$env:PATH='C:\Program Files\nodejs;' + $env:PATH`
- confirmed absolute paths:
  - `C:\Program Files\nodejs\node.exe`
  - `C:\Program Files\nodejs\npm.cmd`
- if `go test ./...` fails with `web\embed.go: pattern dist/*: no matching files found`, rebuild frontend first with `npm run build`

### Docker

```bash
docker build -t hearthstone-analyzer:dev .
docker run --rm -p 8080:8080 hearthstone-analyzer:dev
make docker-build
make docker-run
make smoke-health
make smoke-api
```

## Validation Status

Last known good status:

- `go test ./...` passes
- frontend helper tests pass via Node test runner
- `npm test` passes
- `npm run build` passes
- `docker build -t hearthstone-analyzer:milestone10 .` passes

## Environment Notes

Useful local note:

- Node/npm path details are recorded in [NODE_NPM_PATH.md](D:\HearthStone\OLD-PRD\NODE_NPM_PATH.md)

Meta source selection notes:

- `APP_META_FILE` enables the file-based experimental meta adapter
- `APP_META_FIXTURE` enables the fixture meta source
- `APP_META_REMOTE_URL` enables the remote meta source path
- `APP_META_REMOTE_PROFILE=vicioussyndicate` enables the current validated site-specific profile
- `APP_META_REMOTE_PROFILE=hearthstonetopdecks` enables the second validated site-specific profile
- if no supported source is configured, `sync_meta` fails with a controlled unavailable-source error

## Known Gaps / Risks

- persisted card functional tags now exist, but they are still derived mostly from normalized text heuristics rather than richer source metadata
- scheduler works, but retry/retention/operational logging are still light
- `HSReplay` access pattern and long-term legality/stability remain uncertain
- remote meta card-name normalization is still imperfect for some external edge-case lines
- AI reports are now usable end-to-end, but content-quality guardrails still need more depth beyond the current schema/groundedness pass
- Go embed depends on current `web/dist` asset filenames, so frontend rebuild order matters before final Go verification
- no dedicated cards page yet
- frontend automated coverage is still minimal

## Recommended Next Steps

Best next platform step:

1. extend card sync/schema so card metadata stores richer role inputs beyond text-derived functional tags

Best next analysis step:

2. refine package taxonomy into more Hearthstone-specific packages and sub-packages

Best next compare/report step:

3. turn merged compare guidance into a structured deterministic recommendation object with source/support/confidence fields, then let report generation consume that object directly

Best next report-hardening step:

4. continue report anti-fabrication hardening with near-duplicate, low-information, and end-to-end replay coverage

## Important Context For New Sessions

- old original PRD was moved to [OLD-PRD/PRD.txt](D:\HearthStone\OLD-PRD\PRD.txt)
- current source of truth for product direction is [PRD_v2.md](D:\HearthStone\PRD_v2.md)
- ADR files reflect the agreed architecture:
  - [ADR-001-architecture-baseline.md](D:\HearthStone\OLD-PRD\ADR-001-architecture-baseline.md)
  - [ADR-002-frontend-delivery.md](D:\HearthStone\OLD-PRD\ADR-002-frontend-delivery.md)
  - [ADR-003-settings-and-secrets.md](D:\HearthStone\OLD-PRD\ADR-003-settings-and-secrets.md)
  - [ADR-004-scheduler-model.md](D:\HearthStone\OLD-PRD\ADR-004-scheduler-model.md)
  - [ADR-005-data-sources-and-fallbacks.md](D:\HearthStone\OLD-PRD\ADR-005-data-sources-and-fallbacks.md)

## Session Handoff Summary

If starting a new session, the shortest useful recap is:

- Milestone 0 to 4 are usable
- Milestone 5 is partially implemented and already exposes `POST /api/decks/analyze`
- Milestone 5 now also includes confidence scoring and confidence-reason explanations
- Milestone 5 now also includes:
  - structural tags
  - structural tag explanations
  - package details
  - functional role summary
  - analysis-native suggested adds/cuts
  - analyze API/UI exposure for those fields
- Milestone 5 now also persists card-level functional tags and lets parser/analyzer consume them
- Milestone 6 scheduler/job control is implemented end-to-end with UI
- Milestone 7 now has:
  - remote source support
  - list/detail/latest meta APIs
  - validated `Vicious Syndicate` profile
  - validated `Hearthstone Top Decks` profile
  - meta deck persistence into `decks`, `meta_decks`, and `deck_cards` when names resolve
- Milestone 8 now has:
  - `POST /api/decks/compare`
  - compare UI
  - similarity breakdown
  - qualitative summary
  - diff and suggested adds/cuts
  - persisted `deck_cards` fallback for meta decks without deckstring
  - compare-aware merged summary/add/cut guidance
  - weighting across top compare candidates when they agree
  - conservative fallback when top compare candidates disagree
- Milestone 9 now has:
  - `POST /api/reports/generate`
  - `GET /api/reports`
  - `GET /api/reports/{id}`
  - OpenAI-compatible provider path
  - persisted AI reports
  - report tab + recent history/detail replay in UI
  - compare-context replay for generated and persisted reports
  - merged compare-aware guidance in prompt context
  - JSON-first structured report payload with markdown fallback
  - schema/groundedness guardrails on generated, persisted replay, and frontend hydrate paths
- biggest unfinished areas are richer card metadata normalization, finer-grained package taxonomy, structured compare-aware recommendation objects, and further report content-quality guardrails
