# Phase 4: TDD tests for M3 Device Logic

**GitHub Issue:** #18 — https://github.com/CDSRaBot/sadp-handler.GO/issues/18

## Context

Follow `sadp-handler.GO/docs/ru/DEVELOPMENT_GUIDE.md`, section
“Фаза 4: Написание тестов (разработка через тестирование)”, pp. 53–88.
The source contracts are:

- `sadp-handler.GO/docs/spec/modules/logic.md`
- `sadp-handler.GO/docs/spec/procedures/logic/validate_password.md`
- `sadp-handler.GO/docs/spec/procedures/logic/calculate_hash.md`
- `sadp-handler.GO/docs/spec/procedures/logic/build_activation.md`
- `sadp-handler.GO/docs/spec/procedures/logic/build_network.md`

## Scope and scenarios

Test password policy boundaries (0/7/8/16/17 runes), Unicode letters/digits and
combining marks, all class combinations, spaces, controls, and secret-free errors.
Test known-answer SHA-256 vectors, lowercase hex and empty inputs. Test activation XML
contract, EUI-48 validation, password validation, optional email, XML escaping, exact
casing/declaration, and empty payload on failure. Test network XML DHCP/static branches,
IPv4 prefix/gateway rules, contradictory inputs, missing challenge, known challenge
response, escaping, and proof that plaintext password is never used as fallback.

## Deliverables

- `sadp-handler.GO/internal/logic/*_test.go`
- Separate signature stubs under `sadp-handler.GO/internal/logic/` where needed.

## Acceptance criteria

- Contract, logical, negative, security, and serialization tests use `testing`/`t.Run`
  and initially fail with `errors.New("not implemented")`.
- Tests assert typed errors/causes without logging or embedding secrets.
- Fixtures use synthetic credentials and deterministic vectors only.
- Existing P3.1 issue #15 remains the authoritative issue for activation XML; do not
  duplicate it.
- `go test -v -race ./sadp-handler.GO/internal/logic/...` is the validation command.
