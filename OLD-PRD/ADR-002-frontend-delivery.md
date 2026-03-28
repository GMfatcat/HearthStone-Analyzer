# ADR-002: Vue Frontend Compiled and Served by Go

## Status

Accepted

## Context

The product needs a modern, professional UI, but the project should remain manageable for a solo developer and align with single-container deployment.

## Decision

- the frontend framework is Vue
- frontend assets are compiled ahead of time
- the Go application serves the compiled static assets
- the deployment model does not require a separate frontend container

## Consequences

### Positive

- modern UI without introducing a separate deployed runtime
- simplified packaging and hosting
- clear boundary between API and frontend assets

### Negative

- frontend rebuild is required for UI changes
- dev workflow needs build integration between Go and Vue

### Follow-Up

- define how local frontend development proxies to the Go API
- define the build pipeline for embedding or copying assets into the container image

