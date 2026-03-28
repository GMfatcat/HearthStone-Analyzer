# ADR-005: HearthstoneJSON as Primary Card Source, Meta as Optional Adapter

## Status

Accepted

## Context

The project needs reliable card data and useful meta comparison, but third-party meta sources may be unstable or unavailable.

## Decision

- HearthstoneJSON is the primary card data source
- official Hearthstone card information is supplementary
- meta ingestion is adapter-based
- HSReplay is only a candidate source until proven workable
- meta sync is optional and failure-tolerant
- core deck parsing and analysis must function without meta data

## Consequences

### Positive

- stable baseline for core product functionality
- reduced coupling to one meta source
- graceful degradation when meta data is unavailable

### Negative

- compare features may be incomplete early on
- meta functionality depends on source validation work

### Follow-Up

- validate HSReplay feasibility separately
- preserve raw snapshots where possible for debugging and future remapping

