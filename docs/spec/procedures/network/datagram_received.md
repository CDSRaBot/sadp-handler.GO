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
- The accepted maximum is the shared `MaxSADPDatagramSize` selected for M1 and M2 after protocol verification. Do not assume a 1500-byte limit based on Ethernet MTU. The listener must detect truncation by reading into a buffer larger than the accepted maximum before rejecting an oversized datagram.
- On success, return the handler's nil result. Wrap and return handler errors; do not convert them to success.
- The handler must not retain or mutate `data` after returning unless it copies the bytes.

## 3. Processing Requirements

1. Check the datagram length before dispatch. Do not log its contents.
2. Pass the original byte slice and sender address to the handler synchronously. Do not launch an unbounded goroutine for each packet.
3. Propagate context cancellation to the handler. The listener remains responsible for making its blocking socket read cancellable and for deciding whether a handler error terminates listening.
4. Do not parse SADP XML or apply device/business rules in this transport procedure.

## 4. Errors and Tests

Errors should identify the failed stage (validation or dispatch) and preserve the cause. Malformed XML is handled by M2 and must not cause a panic or crash in the listener.

Tests should cover nil inputs, empty and boundary-sized datagrams using the verified configured limit, oversized and truncated packets, sender preservation, context cancellation, and handler errors.

---

*Level 3 atomic procedure specification.*
