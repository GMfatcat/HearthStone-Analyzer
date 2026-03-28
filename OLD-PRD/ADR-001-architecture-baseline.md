# ADR-001: Single-App, Single-Container MVP Baseline

## Status

Accepted

## Context

The original PRD assumed PostgreSQL, Redis, and Docker Compose-oriented deployment. The project has since been narrowed to a personal, single-machine MVP with a strong preference for simple deployment and manageable local development on Windows.

## Decision

The MVP will use:

- one Go application process
- one SQLite database
- no Redis
- no PostgreSQL
- one Docker container for deployment
- optional Dev Container support for development

The same Go application will host:

- HTTP API
- in-process scheduler
- sync jobs
- settings management
- static frontend asset serving

## Consequences

### Positive

- deployment is simpler and closer to `docker run`
- fewer moving parts for a solo project
- lower infrastructure overhead
- easier backup and restore story

### Negative

- reduced horizontal scalability
- scheduler and API share the same runtime
- SQLite requires careful concurrency handling

### Follow-Up

- schema and repository design should remain modular enough to allow a future DB swap if needed

