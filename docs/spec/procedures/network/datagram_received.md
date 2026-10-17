# Procedure Specification: P1.5 - handleDatagram
## Module: M1 - Network Engine

## 1. Purpose

Validate transport-level bounds for a received UDP datagram and dispatch it to the configured packet handler without introducing unbounded asynchronous work.

## 2. Go Contract

```go
func handleDatagram(
    ctx context.Context,
    data []byte,
    sender net.Addr,
    maxDatagramSize int,
    handle PacketHandler,
) error
```

`PacketHandler` is the M1 handler type:

```go
type PacketHandler func(context.Context, []byte, net.Addr) error
```

- Reject nil context, sender, or handler, a limit outside `1..65507`, and empty or oversized datagrams, with a typed error.
- Use the configured M1/M2 limit, defaulting to 1500 from settings. It is an application resource bound, not a verified SADP maximum. The listener must detect truncation by reading into a buffer one byte larger than that limit before rejecting an oversized datagram.
- On success, return the handler's nil result. Wrap and return handler errors; do not convert them to success.
- The handler must not retain or mutate `data` after returning unless it copies the bytes.

## 3. Processing Requirements

1. Check the datagram length before dispatch using the resolved configured limit. The default is 1500 bytes; the setting is shared by M1 and M2 and is not a SADP maximum. Do not log packet contents.
2. Pass the original byte slice and sender address to the handler synchronously. Do not launch an unbounded goroutine for each packet.
3. Propagate context cancellation to the handler. The listener remains responsible for making its blocking socket read cancellable and for deciding whether a handler error terminates listening.
4. Do not parse SADP XML or apply device/business rules in this transport procedure.

## 4. Errors and Tests

Return `TransportError` for validation/dispatch failure with the cause preserved. Malformed XML is handled by M2 and must not cause a panic or crash in the listener. The application-level handler decides whether the resulting typed parse error is an expected dropped packet or a reportable failure.

Tests should cover nil inputs, invalid configured limits, empty and boundary-sized datagrams using the shared configured limit, oversized and truncated packets, sender preservation, context cancellation, and handler errors.

---

*Level 3 atomic procedure specification.*
