# HearthStone Analyzer MVP Implementation Plan

This document breaks [PRD_v2.md](D:\HearthStone\PRD_v2.md) into implementation tasks for a single-developer workflow.

## 1. Guiding Principles

- implement vertical slices that can be tested early
- keep deployment compatible with a single Docker container
- avoid premature support for optional integrations
- treat meta sync as non-blocking for the core product
- preserve clear module boundaries even in a single-process app
- prefer validated source profiles over speculative scraping assumptions
- keep remote meta ingestion schema-flexible, but only after confirming real source behavior

## 2. Milestones

## Milestone 0: Project Foundation

Goal:

- establish repository layout, app skeleton, and development workflow

Status:

- done

## Milestone 1: SQLite and Core Persistence

Goal:

- establish local persistence and startup-safe migrations

Status:

- mostly done for current scope

Completed:

- schema and migration runner
- repositories for cards, settings, jobs, meta snapshots, decks, meta decks, and deck cards

Remaining:

- more formal migration file layout

## Milestone 2: Settings and Secret Storage

Goal:

- allow runtime-editable settings with immediate effect

Status:

- done for first usable version

## Milestone 3: Cards Sync

Goal:

- sync and store Hearthstone card data reliably

Status:

- done for first usable version

Completed:

- HearthstoneJSON adapter
- card persistence
- card lookup by localized name now supports external meta deck mapping

Remaining:

- supplementary official card enrichment path
- dedicated cards page

## Milestone 4: Deck Parser

Goal:

- parse deck codes into normalized deck structures

Status:

- done for first usable version

Remaining:

- parser persistence of parsed user decks
- more refined legality rules

## Milestone 5: Rule-Based Analysis Engine

Goal:

- produce useful deterministic analysis without meta dependency

Status:

- in progress, already usable

Completed:

- parse + analyze orchestration
- archetype classification
- strengths / weaknesses
- workbench UI

Remaining:

- stronger heuristics
- richer card metadata normalization beyond the current persisted text-derived functional tags
- finer-grained package taxonomy and package conflict modeling
- structured deterministic recommendation objects on top of current package-aware output

Recently completed:

- confidence scoring
- confidence-reason explanation output in API and UI
- structural tags exposed in analysis results and UI
- first-pass analysis-native suggested adds/cuts in API and UI
- structural tag explanations in API and UI
- package-aware analysis output and package-driven recommendation synthesis
- functional-role summary in API and UI
- persisted card functional tags and parser/analyzer consumption of those tags

## Milestone 6: Scheduler and Job Control

Goal:

- support user-controlled built-in cron jobs inside the app

Status:

- done for first usable version

Completed:

- persisted built-in jobs
- reload on change
- manual run
- execution history
- frontend UI

Remaining:

- retry policy
- retention policy
- observability polish

## Milestone 7: Meta Adapter Framework

Goal:

- support optional meta ingestion without coupling the product to one source

Status:

- in progress, substantially advanced

Completed:

- source abstraction
- fixture/file/fallback sources
- generic remote source
- auth/header support for remote source
- schema normalization for multiple payload aliases
- latest/list/detail meta APIs
- frontend snapshot overview/history/detail UI
- validated `Vicious Syndicate` site profile with:
  - latest report discovery
  - deck-library discovery
  - deck variant discovery
  - card line extraction
- meta persistence into:
  - `meta_snapshots`
  - `decks`
  - `meta_decks`
  - `deck_cards` when external card lines resolve to local cards

Remaining:

- improve remote card-name normalization for remaining edge cases
- add more validated remote source profiles beyond the current two
- source-specific extraction polish beyond the current `Vicious Syndicate` and `hearthstonetopdecks` paths

Recently completed:

- second validated remote source profile:
  - `hearthstonetopdecks`
- real-page and end-to-end `sync_meta` validation for the `hearthstonetopdecks` path

## Milestone 8: Deck Comparison

Goal:

- compare a user deck to stored meta decks when snapshots exist

Status:

- in progress, already usable

Completed:

- `POST /api/decks/compare`
- compare UI in workbench
- candidate ranking
- similarity score
- similarity breakdown
- qualitative summary
- shared / missing card diff
- suggested adds / cuts
- fallback compare using:
  - deckstring-backed meta decks
  - persisted `deck_cards`
  - snapshot card-line parsing when necessary
- explicit tie-break ordering using:
  - similarity
  - tier
  - playrate
  - winrate
  - deck id

Remaining:

- structured compare-aware recommendation objects with explicit source/support/confidence
- dedicated compare page

Recently completed:

- merged compare-aware recommendation output
- first-pass analysis package state + compare candidate merge logic
- weighting across top compare candidates when they agree
- conservative fallback when top compare candidates disagree

## Milestone 9: AI Report Layer

Goal:

- generate AI-assisted reports through an OpenAI-compatible endpoint

Status:

- in progress, usable with structured replay and grounded/schema guardrails

Completed:

- internal report service
- internal provider abstraction for report generation
- OpenAI-compatible chat completion client
- prompt assembly from:
  - analysis output
  - optional compare output
- report generation service
- report generation API:
  - `POST /api/reports/generate`
- report persistence into `analysis_reports`
- recent report list API:
  - `GET /api/reports`
- report detail API:
  - `GET /api/reports/{id}`
- frontend report tab
- frontend recent report history list
- frontend recent report detail replay
- compare-context replay for generated and persisted reports
- structured report payload with JSON-first parsing and plain-text fallback
- generated/persisted/frontend structured payload sanitization
- grounded anti-fabrication guardrails for compare-only claims
- merged compare-aware recommendation context in prompt assembly

Remaining:

- broader near-duplicate / low-information guardrails
- deeper generate -> persist -> replay integration coverage
- structured consumption of deterministic compare-aware recommendation objects once available
- broader provider support if needed beyond the current OpenAI-compatible path

## Milestone 10: Packaging and Hardening

Goal:

- prepare the app for stable single-machine deployment

Status:

- partially done

Completed:

- Docker image builds
- frontend build works
- app runs as single process

Remaining:

- operational logging polish
- automated backup strategy / scheduling

## 3. Cross-Cutting Technical Work

### Testing

- maintain unit tests for parser, analysis, scheduler, meta sync, compare, and settings encryption
- maintain integration-style tests for migrations, card/meta persistence, and APIs
- continue validating real remote source behavior before expanding source-specific code

### Observability

- define structured log fields
- include job IDs, job names, durations, status, and error messages
- expose minimal health/readiness endpoints

### Error Handling

- classify user-visible errors vs operational errors
- make sync and AI failures debuggable from UI and logs

## 4. Recommended Task Order From Here

Current fastest practical sequence:

1. extend card sync/schema so card metadata stores richer role inputs beyond text-derived functional tags
2. refine package taxonomy into more Hearthstone-specific packages and sub-packages
3. replace merged compare summary/add/cut strings with a structured deterministic recommendation object
4. expand AI report content guardrails beyond the current schema/groundedness pass
5. improve remaining remote meta card-name normalization edge cases
6. continue packaging/operations hardening without making it the mainline focus

## 5. Definition of Done Per Task

Each implementation task should be considered done only when:

- code builds cleanly
- at least one test or clear manual verification path exists
- logs or UI expose enough information to debug failure
- the task does not break single-container deployment assumptions
- source-specific behavior is validated against a real reachable page or endpoint when the work depends on external data
