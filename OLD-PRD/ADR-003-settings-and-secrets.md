# ADR-003: Settings in SQLite with Lightweight Secret Encryption

## Status

Accepted

## Context

The app requires a settings page where users can update configuration at runtime, including sensitive values such as an API key for an OpenAI-compatible endpoint. The deployment target is a single machine, so operational simplicity is more important than enterprise-grade secret infrastructure.

## Decision

- application settings are stored in SQLite
- settings edited through the UI take effect immediately
- sensitive values are encrypted before persistence
- non-sensitive values may remain plain text

Initial encrypted field:

- LLM API key

## Consequences

### Positive

- simple and self-contained runtime configuration
- no separate secret store required for MVP
- settings remain editable from the product UI

### Negative

- security posture is limited by local-machine assumptions
- encryption key management still needs a concrete implementation

### Follow-Up

- define the encryption key source
- define validation and audit behavior for settings changes

