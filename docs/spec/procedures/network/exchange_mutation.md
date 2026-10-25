# Procedure Specification: P1.6 - MutationTransport.Exchange
## Module: M1 - Network Engine

## 1. Purpose

Acquire a bounded response for one already-built SADP management payload while
preserving sender metadata and transport ownership. This procedure does not
interpret the response.

## 2. Go Contract

```go
type MutationTarget struct {
    MAC    net.HardwareAddr
    Sender net.Addr
}

type MutationExchange struct {
    Sender net.Addr
    Data   []byte
}

type MutationTransport interface {
    Exchange(context.Context, MutationTarget, []byte) (MutationExchange, error)
}
```

- Reject a nil context, invalid target, nil sender where a sender is required,
  empty payload, and payloads over the configured datagram limit before opening
  a socket.
- Require a caller-provided finite deadline or a transport default explicitly
  defined by M1 configuration. Cancellation must interrupt every blocking
  operation.
- Send exactly one payload for one exchange. Retries are not part of this
  procedure and may not be introduced implicitly.
- Return the received bytes and actual sender address without parsing or
  normalizing either value. The caller owns response validation.
- Bound the response size. An oversized response is a typed transport failure,
  not a truncated successful response.

## 3. Ownership and Failure Contract

The transport owns connections it creates and closes them on every path. A
caller-owned connection remains caller-owned; the transport serializes its
exchange use and documents the required reader/writer ownership. Close
failures are returned without hiding an earlier transport error.

Failures use the M1 transport error contract and preserve causes:

- `validation` for invalid arguments;
- `write` or `short_write` for request transmission;
- `read` for receive failure;
- `timeout` for a transport deadline;
- `canceled` when the context is canceled;
- `response_too_large` for a bounded-size violation;
- `close` when cleanup fails without an earlier failure.

Error messages contain only operation stage, sizes, and safe endpoint
metadata. They never contain payload bytes, XML, passwords, challenges, or
hashes.

## 4. Processing Requirements

1. Validate the target, payload, limit, and context.
2. Establish the documented deadline and ownership.
3. Send one datagram.
4. Read one bounded response while preserving its sender.
5. Return the response to M3 for challenge extraction or mutation-result
   parsing.
6. Close owned resources and return any cleanup failure according to the
   transport error contract.

The response sender is deliberately not required to match the target here.
M1 preserves it; M3 performs operation-specific correlation.

## 5. Verification Requirements

Tests derived from this procedure use deterministic fake connections and cover
validation, one-write behavior, short writes, bounded reads, oversized
responses, sender preservation, deadline, cancellation, and close failures.
No live device or multicast network is required.

---

*Level 2 procedure specification.*
