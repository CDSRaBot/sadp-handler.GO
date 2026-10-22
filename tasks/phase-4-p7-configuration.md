# Phase 4: External settings loader and configuration module

## Scope

Implement M7 according to `docs/spec/modules/configuration.md` and
`docs/spec/procedures/configuration/load_settings.md`.

## Deliverables

- Typed `Settings` groups and `Defaults()`.
- `Load(path)` using `encoding/json`, strict unknown-key and trailing-data checks,
  required `_comments` metadata validation, range validation, and explicit typed
  errors.
- CLI precedence wiring and conversion to M1/M4/M5/M6 options.
- `settings.json` template kept synchronized with the specification.

## Acceptance criteria

- Missing `settings.json` uses defaults; an existing invalid file fails startup.
- Every setting has a comment entry covering all accepted values and boundaries.
- No secrets or protocol payloads are read from or emitted by settings handling.
- `gofmt`, `go vet ./...`, `go test ./...`, and `go build ./...` pass.
