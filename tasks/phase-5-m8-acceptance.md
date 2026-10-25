# Phase 5: M8 application composition acceptance

**GitHub Issue:** #37 — https://github.com/CDSRaBot/sadp-handler.GO/issues/37

## Context

Follow `docs/ru/DEVELOPMENT_GUIDE.md`, Phases 1–8, and
`docs/spec/modules/application.md`.

## Scope and scenarios

- Verify M7 settings to typed runtime conversion.
- Verify duration, multicast, logging-level, and discovery-policy mappings.
- Verify configured localization directories are safely mapped to M5 `fs.FS`.
- Verify path escape rejection and explicit startup logging failures.
- Verify CLI overrides are applied after settings without mutating the settings
  file or introducing global configuration state.
- Verify the executable startup path wires M4, M5, M6, and M8 services.

## Deliverables

- Acceptance tests under `internal/app` and `cmd/` where appropriate.
- Updated Issue #37 report and status.
- Committed implementation and verification results.

## Acceptance criteria

- `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go build ./...`
  pass.
- Logging remains redacted, bounded, and rooted at `.\logs`.
- Issue #37 is closed independently of unfinished M4 mutation protocol work.

