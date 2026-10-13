# Module Specification: M1 - Network Engine

## 1. Purpose

Manage SADP UDP traffic: send the observed inquiry probe, receive datagrams, and hand received packet data to the parser. The Python reference uses an asynchronous UDP protocol callback with synchronous packet dispatch; user-provided HiToolsDelivery captures supply observed inquiry, `ProbeMatch`, and `Hello` shapes, but do not fully define the response/correlation contract. The module uses Go networking APIs and does not depend on an event loop or a particular CLI.

## 2. Responsibilities

- Receive UDP datagrams and preserve sender information for downstream validation.
- Build and send the observed SADP inquiry probe to the configured multicast endpoint.
- Apply cancellation, deadlines, and bounded packet handling so shutdown is deterministic and malformed traffic cannot exhaust resources.
- Report transport and handler errors to the caller; parsing and device-management decisions remain outside this module.
- Keep receive dispatch synchronous or otherwise bounded, following the Python callback model. The captured probe payload is established as an observed request; response parsing, correlation, retransmission, and timing are not specified by that capture.

## 3. Go API Contract

The following signatures describe the module contract; concrete package paths and exported names are finalized with the implementation:

```go
type PacketHandler func(context.Context, []byte, net.Addr) error

func Listen(ctx context.Context, conn net.PacketConn, handle PacketHandler) error
func SendInquiryProbe(ctx context.Context, conn net.PacketConn, target net.Addr) error
```

- `Listen` rejects a nil context, connection, or handler. Ownership of a non-nil connection transfers to `Listen` when called; it closes that connection on every return path so cancellation can unblock a pending read and no socket leaks.
- `Listen` receives packets until the context is canceled or a transport error occurs. It returns `nil` for normal cancellation and a non-nil error for unexpected transport or handler failures.
- `SendInquiryProbe` rejects a nil context, connection, or target and returns an error if the context is already canceled, probe construction fails, or the UDP write fails or is short. It writes one request datagram and does not start a receive loop, retry, or wait for a response.
- `SendInquiryProbe` does not take ownership of the connection; callers may use the same connection for `Listen` after sending.
- During an M6 discovery session, M1 `Listen` may own and read the connection while M6 serially invokes `SendInquiryProbe` to write scheduled probes. The connection implementation must support one concurrent reader and one serialized writer. On orderly shutdown M6 stops and joins the writer before canceling the listener context; if the listener fails first, it may close the connection during an in-flight write, and M6 preserves both errors. No other caller may use the connection after ownership transfer.
- The handler receives the packet bytes and sender address. It must not retain or mutate the byte slice after it returns unless it first copies the data.
- The caller owns the connection returned by M0 until passing it to `Listen`; after that transfer, the caller must not reuse it. The listener must stop promptly when the context is canceled by closing the transferred connection to unblock a pending read.

## 4. Concurrency and Lifecycle

- Use goroutines only when they simplify non-blocking I/O or independent work; every goroutine must have a bounded lifetime and a defined shutdown path.
- Do not spawn an unbounded goroutine per datagram. Packet handling must be sequential or use an explicitly bounded worker strategy.
- Preserve packet order if processing is concurrent only when the downstream contract requires it; otherwise document the ordering behavior.
- Do not silently discard handler or socket errors. M2 returns parse errors explicitly; the application-level handler decides whether an expected malformed advertisement is dropped, counted, or reported, and must make that decision observable. Unexpected handler errors propagate through `Listen`.

## 5. Validation and Safety

- The Python reference sets a 1500-byte maximum and drops larger received datagrams. This is a local resource/compatibility choice, not a verified SADP datagram limit. If retained in Go, define one named `MaxSADPDatagramSize` application bound shared with M2, and read enough to detect datagrams larger than the bound instead of silently parsing a truncated prefix.
- Treat the received payload as untrusted bytes, preserve its sender, and dispatch synchronously as in the Python reference. Do not log packet contents.
- Avoid logging full packet contents, credentials, or security material.
- Use network deadlines or another supported cancellation mechanism so context cancellation reliably releases blocked reads.
- Do not retry indefinitely. Any retry behavior must be bounded and specified by the caller or the probe contract.
- Do not infer retransmission cadence from the capture's repeated UUID values. A call to `SendInquiryProbe` sends exactly one datagram; retry policy is not part of this contract.

---

*Level 1 module specification. See [P1.1 Listen](../procedures/network/listen.md), [P1.2 SendInquiryProbe](../procedures/network/send_probe.md), and [P1.5 handleDatagram](../procedures/network/datagram_received.md) for atomic procedure contracts. End-to-end orchestration is specified by [M6](./discovery.md).*
