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
    handle PacketHandler,
) error
```

`PacketHandler` is the M1 handler type:

```go
type PacketHandler func(context.Context, []byte, net.Addr) error
```

- Reject nil context, sender, or handler, and empty or oversized datagrams, with an error.
- The Python reference drops datagrams above 1500 bytes. This is a resource/compatibility bound, not a verified SADP maximum; if retained, define it as the named `MaxSADPDatagramSize` shared by M1 and M2. The listener must detect truncation by reading into a buffer larger than that limit before rejecting an oversized datagram.
- On success, return the handler's nil result. Wrap and return handler errors; do not convert them to success.
- The handler must not retain or mutate `data` after returning unless it copies the bytes.

## 3. Processing Requirements

1. Check the datagram length before dispatch. The Python reference drops datagrams above 1500 bytes, but this is an application/resource bound, not a verified SADP maximum. Keep M1 and M2 on the same named `MaxSADPDatagramSize` bound. Do not log packet contents.
2. Pass the original byte slice and sender address to the handler synchronously. Do not launch an unbounded goroutine for each packet.
3. Propagate context cancellation to the handler. The listener remains responsible for making its blocking socket read cancellable and for deciding whether a handler error terminates listening.
4. Do not parse SADP XML or apply device/business rules in this transport procedure.

## 4. Errors and Tests

Errors should identify the failed stage (validation or dispatch) and preserve the cause. Malformed XML is handled by M2 and must not cause a panic or crash in the listener. The application-level handler decides whether the resulting parse error is an expected dropped packet or a reportable failure.

Tests should cover nil inputs, empty and boundary-sized datagrams using the shared named limit, oversized and truncated packets, sender preservation, context cancellation, and handler errors.

---

*Level 3 atomic procedure specification.*
