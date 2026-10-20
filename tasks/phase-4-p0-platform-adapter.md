# Phase 4: TDD tests for M0 Platform Adapter

**GitHub Issue:** #14 — https://github.com/CDSRaBot/sadp-handler.GO/issues/14  
**Status:** already created

## Context

Follow `sadp-handler.GO/docs/ru/DEVELOPMENT_GUIDE.md`, section
“Фаза 4: Написание тестов (разработка через тестирование)”, pp. 53–88.
The source contracts are:

- `sadp-handler.GO/docs/spec/modules/adapter.md`
- `sadp-handler.GO/docs/spec/procedures/adapter/get_interfaces.md`
- `sadp-handler.GO/docs/spec/procedures/adapter/get_linux_mac.md`
- `sadp-handler.GO/docs/spec/procedures/adapter/open_multicast.md`

## Scope and scenarios

Write contract, positive, negative, portability, resource-ownership, and security tests
for interface enumeration, Linux MAC lookup, and multicast socket setup. Cover eligible
and empty interface lists, loopback/down/no-IPv4 filtering, invalid interfaces, bind/join/
permission failures, platform-specific behavior, endpoint/interface preservation, cleanup,
and the absence of silent fallback. Tests must use deterministic seams/fakes and must not
require a real multicast device.

## Deliverables

- `sadp-handler.GO/internal/adapter/*_test.go`
- Signature stubs under `sadp-handler.GO/internal/adapter/` when required.

## Acceptance criteria

- Tests use `testing` and `t.Run`, compile with Go 1.22+, and initially fail with
  `errors.New("not implemented")`.
- Typed values, fields, errors, platform branches, and resource ownership are asserted.
- No production MACs, credentials, packet dumps, or OS-dependent flaky assumptions are used.
- `go test -v -race ./sadp-handler.GO/internal/adapter/...` is the validation command.
