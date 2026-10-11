# Procedure Specification: P6.1 - DiscoverOnInterface
## Module: M6 - Discovery Coordinator

## 1. Purpose

Run one discovery session on a selected interface by composing socket setup, one inquiry probe, receive dispatch, and advertisement parsing.

## 2. Go Contract

```go
func DiscoverOnInterface(
    ctx context.Context,
    iface adapter.Interface,
    handle EventHandler,
) error
```

- Reject nil context or handler and an ineligible interface before network activity.
- Run until the context is canceled or an unexpected operation/callback error occurs.
- Return nil only for normal context cancellation when all owned resources close successfully. Preserve close failures with the primary error.
- The handler receives either a valid `DeviceFound` event or a `DatagramRejected` event. It must not retain mutable data owned by the coordinator.

## 3. Processing Requirements

1. Open one connection with M0 `OpenMulticast` for `239.255.255.250:37020`. If opening fails, return a wrapped error.
2. Send one M1 `SendInquiryProbe` datagram. If sending fails, close the caller-owned connection and return the send error joined with any close error.
3. After a successful send, transfer connection ownership to M1 `Listen`; do not close or reuse the connection independently after transfer.
4. For each received datagram, call M2 `ParseAdvertisement` with its sender. If parsing fails, send a `DatagramRejected` event containing sender and a sanitized parse error, then continue if the callback succeeds.
5. For each parsed advertisement, send a `DeviceFound` event. A callback error is fatal and is returned by the session.
6. Do not deduplicate advertisements, correlate UUIDs, retransmit the probe, stop on a `Hello`, or infer an announcement schedule. The caller controls the session lifetime by canceling `ctx`.
7. Do not log raw datagrams, full XML, device identifiers, passwords, or values such as `Salt`.

## 4. Errors and Tests

Errors identify the stage (validation, socket open, probe send, receive, parsing callback, device callback, cleanup) while preserving wrapped causes. A malformed device packet is an event, not a fatal session error, unless the event handler returns an error.

Test dependency-injected open/send/listen/parser behavior using an unexported dependency bundle or equivalent function seams; the exported API must not expose test hooks and implementation must not rely on mutable package globals. Verify that send occurs once, successful send precedes ownership transfer, open/send failures clean up, cancellation closes once, rejected datagrams do not stop subsequent valid events, both observed root variants are emitted as devices, handler failures terminate, and close errors are preserved. Do not require live devices or a multicast-capable test host.

---

*Level 2 procedure specification.*
