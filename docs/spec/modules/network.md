# Module Specification: M1 - Network Engine

## 1. Purpose

Manage SADP UDP traffic: send the inquiry probe, receive device responses, and hand received packet data to the parser. The Python implementation has been tested with real devices and successfully discovers them and receives their responses; it is the behavioral compatibility baseline for this module. User-provided HiToolsDelivery captures document observed inquiry, `ProbeMatch`, and `Hello` shapes, but do not fully define universal response correlation or timing. The module uses Go networking APIs and does not depend on an event loop or a particular CLI.

## 2. Responsibilities

- Receive UDP datagrams and preserve sender information for downstream validation.
- Build and send the SADP inquiry probe to the configured multicast endpoint, preserving the working Python implementation's behavior.
- Apply cancellation, deadlines, and bounded packet handling so shutdown is deterministic and malformed traffic cannot exhaust resources.
- Report transport and handler errors to the caller; parsing and device-management decisions remain outside this module.
- Keep receive dispatch synchronous or otherwise bounded, following the Python callback model. Successful real-device discovery is the baseline; the captures alone do not specify universal response parsing, correlation, retransmission, or timing rules.

## 3. Go API Contract

The following signatures describe the module contract; concrete package paths and exported names are finalized with the implementation:

```go
type PacketHandler func(context.Context, []byte, net.Addr) error

const DefaultMaxSADPDatagramSize = 1500

type TransportStage string

const (
    TransportValidation TransportStage = "validation"
    TransportEncode     TransportStage = "encode"
    TransportWrite      TransportStage = "write"
    TransportShortWrite TransportStage = "short_write"
    TransportRead       TransportStage = "read"
    TransportDispatch   TransportStage = "dispatch"
    TransportClose      TransportStage = "close"
)

type TransportError struct {
    Stage TransportStage
    Err   error
}

func (e *TransportError) Error() string
func (e *TransportError) Unwrap() error

func Listen(ctx context.Context, conn net.PacketConn, maxDatagramSize int, handle PacketHandler) error
func SendInquiryProbe(ctx context.Context, conn net.PacketConn, target net.Addr) error
func SendSADPDatagram(
    ctx context.Context,
    conn net.PacketConn,
    target net.Addr,
    payload []byte,
    maxDatagramSize int,
) error
```

- `Listen` rejects a nil context, connection, or handler and a `maxDatagramSize` outside `1..65507`. Ownership of a non-nil connection transfers to `Listen` when called; it closes that connection on every return path so cancellation can unblock a pending read and no socket leaks.
- `Listen` receives packets until the context is canceled or a transport error occurs. It returns `nil` for normal cancellation and a non-nil error for unexpected transport or handler failures.
- `SendInquiryProbe` rejects a nil context, connection, or target and returns an error if the context is already canceled, probe construction fails, or the UDP write fails or is short. It writes one request datagram and does not start a receive loop, retry, or wait for a response.
- `SendInquiryProbe` does not take ownership of the connection; callers may use the same connection for `Listen` after sending.
- `SendSADPDatagram` rejects nil/empty arguments, a limit outside `1..65507`, and payloads larger than `maxDatagramSize`, writes exactly one datagram to the caller-supplied target, and does not own the connection or wait for a response. M3 uses it for SADP management XML such as activation; it does not interpret response semantics.
- During an M6 session, M1 `Listen` may own and read the connection while M6 serially invokes `SendInquiryProbe` or M3 invokes `SendSADPDatagram` to write a management request. The connection implementation must support one concurrent reader and one serialized writer; all writes on a session connection are serialized by its coordinator. On orderly shutdown the coordinator stops and joins the writer before canceling the listener context; if the listener fails first, it may close the connection during an in-flight write, and the coordinator preserves both errors. No other caller may use the connection after ownership transfer.
- The handler receives the packet bytes and sender address. It must not retain or mutate the byte slice after it returns unless it first copies the data.
- The caller owns the connection returned by M0 until passing it to `Listen`; after that transfer, the caller must not reuse it. The listener must stop promptly when the context is canceled by closing the transferred connection to unblock a pending read.

## 4. Concurrency and Lifecycle

- Use goroutines only when they simplify non-blocking I/O or independent work; every goroutine must have a bounded lifetime and a defined shutdown path.
- Do not spawn an unbounded goroutine per datagram. Packet handling must be sequential or use an explicitly bounded worker strategy.
- Preserve packet order if processing is concurrent only when the downstream contract requires it; otherwise document the ordering behavior.
- Do not silently discard handler or socket errors. M2 returns parse errors explicitly; the application-level handler decides whether an expected malformed advertisement is dropped, counted, or reported, and must make that decision observable. Unexpected handler errors propagate through `Listen`.

## 5. Validation and Safety

- Use the configured `maxDatagramSize` consistently for receive and parser validation. M7 supplies the default `DefaultMaxSADPDatagramSize = 1500`; valid configured values are `1..65507`. This is an application resource bound, not a SADP datagram limit. Read one byte beyond the limit so oversized datagrams are detected instead of silently parsing a truncated prefix.
- Treat the received payload as untrusted bytes, preserve its sender, and dispatch synchronously as in the Python reference. Do not log packet contents.
- Avoid logging full packet contents, credentials, or security material.
- Use network deadlines or another supported cancellation mechanism so context cancellation reliably releases blocked reads.
- Do not retry indefinitely. Any retry behavior must be bounded and specified by the caller or the probe contract.
- Do not infer retransmission cadence from the capture's repeated UUID values. A call to `SendInquiryProbe` sends exactly one datagram; retry policy is not part of this contract.
- Return a typed `TransportError` with a stable stage (`validation`, `encode`, `write`, `short_write`, `read`, `dispatch`, or `close`) and preserve the underlying cause. Context cancellation remains detectable with `errors.Is(err, context.Canceled)`; do not classify errors by message text.

---

*Level 1 module specification. See [P1.1 Listen](../procedures/network/listen.md), [P1.2 SendInquiryProbe](../procedures/network/send_probe.md), [P1.4 SendSADPDatagram](../procedures/network/send_datagram.md), and [P1.5 handleDatagram](../procedures/network/datagram_received.md) for atomic procedure contracts. End-to-end orchestration is specified by [M6](./discovery.md).*
