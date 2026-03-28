# ADR-004: In-Process Built-In Job Scheduler with UI Control

## Status

Accepted

## Context

The product needs scheduled data sync while remaining easy to deploy as a single container. Users also need to control scheduling from the UI instead of editing external cron configuration.

## Decision

- the scheduler runs inside the same Go application process as the API
- only a fixed set of built-in jobs may be scheduled
- users can edit cron expressions in the UI
- users can enable or disable jobs in the UI
- users can manually trigger jobs in the UI
- schedule changes take effect immediately
- manual runs do not affect the next scheduled run time
- job definitions and execution history are stored in SQLite

Initial built-in jobs:

- `sync_cards`
- `sync_meta`
- `rebuild_features`

## Consequences

### Positive

- no external scheduler dependency
- user-friendly control model
- operational state is visible in one place

### Negative

- scheduler complexity becomes part of the app
- duplicate-run prevention and locking must be handled carefully

### Follow-Up

- define job concurrency policy
- define retry behavior
- define job history retention policy

