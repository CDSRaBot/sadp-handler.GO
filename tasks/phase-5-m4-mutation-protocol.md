# Phase 5: M4 mutation protocol workflows

**GitHub Issue:** #36 — https://github.com/CDSRaBot/sadp-handler.GO/issues/36

**Audit artifact:** `docs/spec/procedures/logic/mutation_workflow_audit.md`

## Context

Follow `docs/ru/DEVELOPMENT_GUIDE.md`, Phases 1–8. The source contracts are:

- `docs/spec/modules/ui.md`
- `docs/spec/modules/logic.md`
- `docs/spec/modules/network.md`
- `docs/spec/procedures/ui/dispatch_command.md`
- `docs/spec/procedures/logic/build_activation.md`
- `docs/spec/procedures/logic/build_network.md`

The existing M4 mutation seams must remain typed and injected. No workflow may
be enabled by guessing an undocumented device response format.

## Scope and scenarios

- Define and review challenge acquisition and management-response contracts.
- Implement context-aware activation and network services only after the
  response shape and correlation rules are specified.
- Preserve password-stdin policy and fail closed when a challenge is missing.
- Map invalid input, cancellation, transport, rejection, and malformed response
  failures to stable typed outcomes.
- Return an attributable `DeviceOutcome` for every requested device, including
  partial bulk failures.
- Cover successful, rejected, timed-out, canceled, malformed, duplicate-target,
  and secret-redaction cases with deterministic fakes.

## Deliverables

- Contract and procedure updates under `docs/spec/`.
- Production services under the appropriate `internal/` package.
- M4 dependency wiring from `internal/app` and `cmd/`.
- Tests in `internal/logic`, `internal/network`, and `internal/ui`.
- Updated change report and Issue #36 status.

## Current audit result

The request builders and M1 send primitive exist, but challenge acquisition,
management-response parsing, and response correlation are not specified by the
repository. Production mutation implementation is therefore blocked until
fixtures and these contracts are approved.

## Acceptance criteria

- Tests are written before implementation for each new protocol contract.
- No password, challenge, hash, XML payload, or device secret is logged or
  emitted in CLI/TUI output.
- All blocking operations accept `context.Context` and close resources.
- `go test -race ./...`, `go vet ./...`, and `go build ./...` pass.
- Issue #36 is closed only after the end-to-end mutation acceptance checks pass.
