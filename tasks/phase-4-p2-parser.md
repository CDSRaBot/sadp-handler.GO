# Phase 4: TDD tests for M2 Data Parser

**GitHub Issue:** #17 — https://github.com/CDSRaBot/sadp-handler.GO/issues/17

## Context

Follow `sadp-handler.GO/docs/ru/DEVELOPMENT_GUIDE.md`, section
“Фаза 4: Написание тестов (разработка через тестирование)”, pp. 53–88.
The source contracts are:

- `sadp-handler.GO/docs/spec/modules/parser.md`
- `sadp-handler.GO/docs/spec/procedures/parser/parse_advertisement.md`
- `sadp-handler.GO/docs/spec/procedures/parser/sanitize_mac.md`

## Scope and scenarios

Test `NormalizeMAC` and `ParseAdvertisement` against sanitized deterministic fixtures for
valid Python-reference `ProbeMatch`, observed `ProbeMatch` aliases, and `Hello` without UUID.
Cover root/type validation, direct-child extraction, MAC and IPv4 normalization, activation
boolean boundaries, default software version, sender preservation, conflicting aliases,
unknown optional fields, empty/oversized/boundary datagrams, limits outside `1..65507`,
malformed/truncated XML, invalid UTF-8/directives/trailing content, missing/invalid fields,
and arbitrary-byte fuzzing. Assert `ParseError` kinds, fields, wrapped causes, no usable
device on failure, no panic, and no packet/security data in errors.

## Deliverables

- `sadp-handler.GO/internal/parser/*_test.go`
- Sanitized XML fixtures and signature stubs under `sadp-handler.GO/internal/parser/`.

## Acceptance criteria

- Tests use `testing`, `t.Run`, and Go fuzz tests where appropriate; they initially fail
  against `errors.New("not implemented")`.
- Fixtures contain no real identifiers, salts, credentials, or complete captured payloads.
- Exact size boundaries and both supported advertisement shapes are asserted.
- `go test -v -race ./sadp-handler.GO/internal/parser/...` is the validation command.
