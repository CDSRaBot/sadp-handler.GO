# Phase 4: TDD tests for M6 Discovery Coordinator

**GitHub Issue:** #21 — https://github.com/CDSRaBot/sadp-handler.GO/issues/21

## Context

Follow `sadp-handler.GO/docs/ru/DEVELOPMENT_GUIDE.md`, section
“Фаза 4: Написание тестов (разработка через тестирование)”, pp. 53–88.
The source contracts are:

- `sadp-handler.GO/docs/spec/modules/discovery.md`
- `sadp-handler.GO/docs/spec/procedures/discovery/discover_on_interface.md`
- Related M0/M1/M2 module contracts for dependency seams and ownership.

## Scope and scenarios

Test option validation for continuous and bounded modes, timing and repeat-count
boundaries, default/max datagram size, nil context/handler/ineligible interface,
immediate and periodic probes, exact bounded cycle count and full receive windows,
no early stop on device events, one reader plus serialized writer, open/first-send
cleanup, ownership transfer, orderly cancellation, listener/send/handler errors,
parse-error events followed by continued reception, valid `ProbeMatch` and `Hello`
events, sender/timestamp preservation, and redaction/no-payload logging.

## Deliverables

- `sadp-handler.GO/internal/discovery/*_test.go`
- An unexported dependency bundle or equivalent seams for open/send/listen/parse;
  no mutable package globals or public test hooks.

## Acceptance criteria

- Tests use fakes and controllable clocks/signals; no real devices, multicast, or
  sleep-based race assumptions.
- Error joining, connection ownership, goroutine shutdown, event contracts, and
  cancellation detection are asserted.
- Tests initially fail against explicit `not implemented` stubs.
- `go test -v -race ./sadp-handler.GO/internal/discovery/...` is the validation command.
