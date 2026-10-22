# Phase 4: Refactor existing tests for external settings

## Dependency

Depends on `phase-4-p7-configuration.md`.

## Scope

Update existing tests affected by the settings contract rather than retaining
hard-coded runtime defaults. The affected packages already have tests:

- `internal/ui`: settings/CLI precedence, mode, format, locale, and once policy.
- `internal/network`: configured multicast target and datagram limit.
- `internal/parser`: the same resolved datagram limit and boundary behavior.
- `internal/adapter`: configured group, port, and TTL passed to socket setup.
- `internal/logic`: ensure device payload tests do not gain configuration secrets.

Add a dedicated configuration test package for missing files, template loading,
strict JSON/comment metadata, all documented boundaries, invalid combinations,
and explicit errors. Use temporary files and injected dependencies; do not use
the real working directory, network, TTY, or sleep-based timing.

## Acceptance criteria

- Tests assert settings-derived values and CLI override precedence.
- No test duplicates undocumented defaults as an implicit contract.
- Existing redaction, cancellation, ownership, and transport assertions remain.
- Targeted tests pass with `go test -race ./...`.
