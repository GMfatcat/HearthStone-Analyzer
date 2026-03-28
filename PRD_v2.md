# HearthStone Analyzer MVP PRD v2

## 1. Product Goal

Build a Hearthstone deck analysis system centered on Go that can:

- sync card data on a schedule
- optionally sync meta snapshots from external sources
- parse user-provided deck codes
- analyze deck structure with deterministic rules
- compare a deck against popular/meta decks when meta data is available
- generate an AI-assisted natural language report through an OpenAI-compatible interface

This version reflects the confirmed project direction and current validated source strategy:

- single Go application
- SQLite instead of PostgreSQL
- no Redis
- Vue frontend compiled to static assets and served by Go
- single-container deployment target
- Windows local development with optional Dev Container support
- HearthstoneJSON remains the canonical card source
- remote meta ingestion remains adapter-based and should prefer validated site profiles over speculative scraping assumptions

## 2. MVP Scope

### In Scope

- card sync from HearthstoneJSON as the primary source
- official Hearthstone card data as a supplementary source when needed
- deck code parsing
- rule-based deck analysis
- optional meta sync through pluggable adapters
- deck comparison against stored meta decks and meta deck card lists
- AI report generation through an internal provider interface
- settings page in the UI
- built-in scheduler with user-controlled cron jobs
- job execution history and status display
- single-container Docker deployment

### Out of Scope

- Redis-based caching
- PostgreSQL support
- Ollama or local LLM runtime inside the app container
- real-time match simulation
- full matchup engine
- ML training pipeline
- multi-node deployment

## 3. Target Runtime Model

### Development

- primary workflow is local Windows development
- Go may be installed locally
- Dev Container is supported for environment consistency
- local development does not require full multi-container orchestration

### Deployment

- target is a single PC
- the application should be deployable as a single Docker container
- the app process includes API, scheduler, sync jobs, and static file serving
- persistent data must survive container restarts through mounted storage

## 4. High-Level Architecture

The system runs as one Go application with the following internal modules:

- `cards`: source adapters, normalization, persistence
- `decks`: deck code parsing and normalized deck model
- `analysis`: feature extraction, heuristics, and structural comparison helpers
- `meta`: pluggable meta source adapters, remote source profiles, snapshot persistence, and meta deck mapping
- `report`: rule-based and AI-enhanced report generation
- `llm`: internal provider interface for OpenAI-compatible endpoints
- `scheduler`: in-process cron scheduler with runtime reload
- `settings`: persistent application settings and secret handling
- `httpapi`: REST API and static asset serving
- `web`: Vue frontend build output

## 5. Core Technical Decisions

### Backend

- language: Go
- HTTP framework: Chi-compatible standard routing style
- database: SQLite
- migrations: startup-applied migrations
- logging: structured logging with `slog` or equivalent

### Frontend

- framework: Vue
- build output: static assets
- serving model: Go serves compiled frontend assets directly

### AI Integration

- the app exposes an internal provider abstraction
- provider implementations must support OpenAI-compatible APIs
- the app is not coupled to one vendor as long as the endpoint is compatible
- no bundled local model runtime

### Infra

- no Redis
- no PostgreSQL
- no Docker Compose as a required local development model
- deployment optimized for `docker run`

## 6. Data Sources

### Cards

Primary source:

- HearthstoneJSON

Supplementary source:

- official Hearthstone card library and set/version information when needed for enrichment or validation

Source priority rule:

- HearthstoneJSON is the canonical source for the MVP
- supplementary official data is optional and should not block the base sync pipeline

### Meta

- meta ingestion must be adapter-based
- remote meta sites must be treated as source profiles, not assumed to share one stable schema
- validated source behavior should drive profile design
- current validated profile path is `Vicious Syndicate`
- `HSReplay` remains a candidate source, but public machine-readable access and long-term scraping stability are still uncertain
- meta sync must be optional and failure-tolerant
- if meta sync is unavailable, core deck analysis must still work

## 7. User-Facing Functional Requirements

### Deck Analysis

The user can:

- submit a deck code
- view parsed deck contents
- see mana curve and structural features
- view archetype classification
- read strengths, weaknesses, and suggested changes
- compare against meta decks when snapshots exist

### Meta Overview

The user can:

- view the latest meta snapshot summary
- browse recent snapshot history
- inspect selected snapshot details
- inspect meta deck summaries extracted from stored snapshot payloads

### Compare

The user can:

- compare an input deck against stored meta decks
- see ranked candidate matches
- see card overlap and missing-card differences
- read suggested adds and cuts derived from the closest meta decks

### Settings UI

The user can edit settings in the UI and changes take effect immediately.

The settings page must cover at least:

- OpenAI-compatible base URL
- API key
- model name
- cards sync job schedule
- meta sync job schedule
- job enable/disable state
- data source toggles where applicable

### Job Control UI

The user can manage only a fixed set of built-in jobs, not arbitrary user-defined jobs.

Initial job set:

- `sync_cards`
- `sync_meta`
- `rebuild_features`

For each job, the UI must support:

- cron expression editing
- enable/disable toggle
- manual run
- latest status display
- execution history

## 8. Scheduler Requirements

The scheduler runs inside the same Go application process as the API.

Rules:

- updating a cron schedule reloads that job immediately
- enabling or disabling a job takes effect immediately
- manual execution does not alter the next scheduled run time
- only known built-in jobs can be scheduled
- scheduler state is persisted in SQLite

## 9. Settings and Secret Handling

Application settings are persisted in SQLite.

Encryption policy:

- only sensitive settings must be encrypted
- the first sensitive setting to support is the LLM API key
- non-sensitive settings may remain plain text in SQLite

Tradeoff:

- this is intended for a single-machine deployment, so simple encryption is acceptable for MVP
- secret handling should be abstracted so stronger protection can be added later

## 10. Reliability and Fallback Behavior

### Meta Sync Failure

If meta sync fails:

- cards sync must still work
- deck parsing must still work
- deck analysis must still work
- compare/meta features may be degraded or unavailable
- the UI must show the failure state clearly

### Meta Compare Fallback

If a meta deck does not provide a deckstring:

- the system should still attempt compare if the source profile exposes deck card lines
- if card lines can be mapped to local cards, they should be persisted into `deck_cards`
- compare should prefer persisted `deck_cards` over reparsing raw snapshot payloads

### Data Persistence

Persistent data must include:

- SQLite database file
- application settings
- job definitions
- job execution history
- synced card and meta snapshots
- persisted meta decks and, when resolvable, their deck cards

## 11. Data Model

Required persisted entities include:

- cards
- card locales
- decks
- deck cards
- meta snapshots
- meta decks
- analysis reports
- app settings
- scheduled jobs
- job execution logs

## 12. API and UI Implications

The backend should expose APIs for:

- card queries
- deck parsing
- deck analysis
- deck comparison
- latest meta snapshot
- meta snapshot list/detail
- settings read/update
- job list/read/update
- manual job execution
- job history

The frontend should provide:

- deck analysis workbench
- compare results in the workbench, with a path to expand later
- meta overview/history/detail views
- settings page
- job management page

## 13. Non-Functional Requirements

- rule-based deck analysis should complete in about 1 second or less per request
- compare should remain responsive when using persisted meta deck data
- AI report generation may be slower and should remain optional
- the app must be maintainable as a single-developer project
- modules should remain decoupled enough that data sources and AI providers can be swapped
- logs and job history must be sufficient for debugging sync failures

## 14. Updated Risks

### High Risk

- `HSReplay` availability, legality, and scraping stability are still uncertain
- card-name normalization from remote meta deck pages may be incomplete
- UI-controlled scheduler increases MVP scope beyond a simple analysis tool

### Medium Risk

- card tagging and archetype classification accuracy may require iteration
- storing secrets in SQLite requires careful but lightweight encryption handling
- source-specific HTML layouts may change over time

### Mitigations

- keep meta source adapter-based and optional
- validate real source structure before baking in source-specific assumptions
- preserve raw snapshots when possible
- expose job history and error messages in UI
- clearly distinguish stored data from inferred analysis

## 15. Suggested Implementation Order From Here

1. strengthen remote card-name normalization so more meta deck lines resolve into persisted `deck_cards`
2. improve compare beyond card-overlap similarity
3. add another explicit validated meta source profile
4. continue rule-based analysis heuristics and confidence
5. add AI provider interface and report generation
6. finish deployment hardening and docs

## 16. Definition of MVP Complete

The MVP is considered complete when all of the following are true:

- card data can be synced and persisted locally
- deck codes can be parsed correctly
- rule-based analysis can be produced consistently
- settings can be edited in the UI and applied immediately
- built-in jobs can be scheduled, enabled, disabled, and run manually
- job history is visible in the UI
- meta sync remains optional and does not block core deck analysis
- at least one real remote meta source profile is validated and operational
- meta decks can be compared against user decks using persisted data
- the app can run as a single Docker container on one machine
- Vue frontend assets are served by the Go application
- AI report generation works through an OpenAI-compatible endpoint
