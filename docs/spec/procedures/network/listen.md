# Procedure Specification: P1.1 - Listen
## Module: M1 - Network Engine

## 1. Purpose

Receive UDP datagrams on an M0-provided packet connection and dispatch each datagram synchronously to a caller-supplied handler until cancellation or failure.

## 2. Go Contract

```go
func Listen(ctx context.Context, conn net.PacketConn, handle PacketHandler) error
```

- Reject a nil context, connection, or handler with an error.
- Ownership of a non-nil connection transfers to `Listen` when called. Close it on every return path, including invalid arguments, cancellation, transport errors, and handler errors.
- Preserve connection-close failures in the returned error, joined with the primary failure where necessary; normal cancellation returns nil only if cleanup succeeds.
- Return nil for normal context cancellation. Return an error for unexpected read or handler failures; do not report failure as success.
- The handler must not retain or mutate the received byte slice after returning unless it copies it.
- In an M6 session, one M1 `SendInquiryProbe` writer may use the transferred connection concurrently with this reader. Writes are serialized by M6; no other reader/writer is permitted. On orderly shutdown M6 stops and joins its writer before canceling the listener context, allowing `Listen` to close the connection afterward. On a listener/handler failure, `Listen` may close while one write is completing; M6 must join that writer and preserve any resulting write error alongside the listener error.

## 3. Processing Requirements

1. Read into a buffer of `MaxSADPDatagramSize + 1` bytes so a packet larger than the shared M1/M2 application limit is detectable rather than silently passed as a truncated valid prefix.
2. Dispatch through `handleDatagram` sequentially; do not launch an unbounded goroutine per packet. Preserve each packet's sender address.
3. Check cancellation before each read. Arrange for cancellation to close the owned connection exactly once and unblock a pending read. Normalize a read error caused by cancellation to normal cancellation; preserve other read errors. Include a close error in the returned error rather than discarding it.
4. On unexpected handler errors, stop receiving and return a wrapped error. The application handler is responsible for consuming expected M2 validation errors when policy calls for dropping an invalid advertisement.
5. Do not parse XML or implement discovery-probe generation in this procedure.

## 4. Errors and Tests

Return errors with stage context while preserving causes; never include packet bytes or secrets. Test nil arguments, connection closure on every return path, close-error reporting, normal cancellation during a blocked read, unexpected read errors, sequential dispatch and sender preservation, handler errors, and packets at/over the shared datagram limit.

---

*Level 2 atomic procedure specification.*
