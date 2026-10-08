# Module Specification: M1 - Network Engine

## 1. Purpose

Manage SADP UDP discovery traffic: send probes, receive datagrams, and hand raw packet data to the parser. The module uses Go networking APIs and does not depend on an event loop or a particular CLI.

## 2. Responsibilities

- Send the SADP discovery probe through a connection configured by M0.
- Receive UDP datagrams and preserve sender information for downstream validation.
- Apply cancellation, deadlines, and bounded packet handling so shutdown is deterministic and malformed traffic cannot exhaust resources.
- Report transport and handler errors to the caller; parsing and device-management decisions remain outside this module.

## 3. Go API Contract

The following signatures describe the module contract; concrete package paths and exported names are finalized with the implementation:

```go
type PacketHandler func(context.Context, []byte, net.Addr) error

func SendProbe(ctx context.Context, conn net.PacketConn, target net.Addr) error
func Listen(ctx context.Context, conn net.PacketConn, handle PacketHandler) error
```

- `SendProbe` rejects a nil context, connection, or target, sends one valid SADP probe, and returns an error if it cannot complete.
- `Listen` receives packets until the context is canceled or a transport error occurs. It returns `nil` for normal cancellation and a non-nil error for unexpected transport or handler failures.
- The handler receives the packet bytes and sender address. It must not retain or mutate the byte slice after it returns unless it first copies the data.
- The caller owns `conn` and is responsible for closing it. The listener must stop promptly when the context is canceled; the implementation must use a cancellation-aware read strategy rather than rely on an unbounded blocking read.

## 4. Concurrency and Lifecycle

- Use goroutines only when they simplify non-blocking I/O or independent work; every goroutine must have a bounded lifetime and a defined shutdown path.
- Do not spawn an unbounded goroutine per datagram. Packet handling must be sequential or use an explicitly bounded worker strategy.
- Preserve packet order if processing is concurrent only when the downstream contract requires it; otherwise document the ordering behavior.
- Do not silently discard handler or socket errors. If malformed packets are intentionally dropped, that decision belongs to the parser and should be observable through an explicit result or appropriate diagnostic.

## 5. Validation and Safety

- Bound the receive buffer and reject datagrams exceeding a shared SADP packet-size limit defined by M2 after protocol verification. Do not infer the protocol limit from the Ethernet MTU; size the receive buffer so oversize datagrams can be detected rather than silently truncated.
- Avoid logging full packet contents, credentials, or security material.
- Use network deadlines or another supported cancellation mechanism so context cancellation reliably releases blocked reads.
- Do not retry indefinitely. Any retry behavior must be bounded and specified by the caller or the probe contract.

---

*Level 1 module specification. The related procedure documents under `../procedures/network/` are separate from this Go module contract.*
