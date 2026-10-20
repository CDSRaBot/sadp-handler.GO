# Phase 4: TDD tests for M1 Network Engine

**GitHub Issue:** #16 — https://github.com/CDSRaBot/sadp-handler.GO/issues/16

## Context

Follow `sadp-handler.GO/docs/ru/DEVELOPMENT_GUIDE.md`, section
“Фаза 4: Написание тестов (разработка через тестирование)”, pp. 53–88.
The source contracts are:

- `sadp-handler.GO/docs/spec/modules/network.md`
- `sadp-handler.GO/docs/spec/procedures/network/listen.md`
- `sadp-handler.GO/docs/spec/procedures/network/send_probe.md`
- `sadp-handler.GO/docs/spec/procedures/network/send_datagram.md`
- `sadp-handler.GO/docs/spec/procedures/network/datagram_received.md`

## Scope and scenarios

Test `Listen`, `SendInquiryProbe`, and `SendSADPDatagram` with fake
`net.PacketConn` implementations. Cover typed validation/read/write/dispatch/close and
short-write errors, `errors.Is` cancellation, nil arguments, limits `1..65507`,
oversized packets and payloads, exact one-write behavior, target and sender preservation,
probe XML declaration/elements/UUID format, sequential handler dispatch, blocked-read
 cancellation, close-on-every-return-path, handler/read failures, and payload redaction.
Include race-safe concurrent reader/serialized writer behavior without live multicast.

## Deliverables

- `sadp-handler.GO/internal/network/*_test.go`
- Signature stubs under `sadp-handler.GO/internal/network/` as needed.

## Acceptance criteria

- Tests are table-driven with `testing`/`t.Run` and initially fail against
  `errors.New("not implemented")`.
- No test depends on a real socket, device, timing race, or production secret.
- Close errors are checked rather than discarded; error kinds and causes are inspectable.
- `go test -v -race ./sadp-handler.GO/internal/network/...` is the validation command.
