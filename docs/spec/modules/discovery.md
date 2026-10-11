# Module Specification: M6 - Discovery Coordinator

## 1. Purpose

Compose M0 interface-specific multicast setup, M1 inquiry transmission and packet reception, and M2 advertisement parsing into one cancellable discovery session.

## 2. Responsibilities

- Run one discovery session on one explicitly selected interface.
- Open the SADP multicast connection, send one inquiry probe, and receive/parse advertisements until cancellation or failure.
- Deliver valid devices and rejected-datagram diagnostics to the caller without exposing packet contents or security fields.
- Make connection ownership, cancellation, callback failures, and cleanup explicit.

M6 does not enumerate/select interfaces, implement retries, correlate UUIDs, or interpret device-management/security responses. Callers can start separate sessions for separate interfaces and own any cross-interface concurrency policy.

## 3. Go API Contract

```go
type EventKind uint8

const (
    DeviceFound EventKind = iota + 1
    DatagramRejected
)

type Event struct {
    Kind   EventKind
    Device *parser.Device
    Sender net.Addr
    Err    error
}

type EventHandler func(context.Context, Event) error

func DiscoverOnInterface(
    ctx context.Context,
    iface adapter.Interface,
    handle EventHandler,
) error
```

- For `DeviceFound`, `Device` is non-nil and `Err` is nil. For `DatagramRejected`, `Device` is nil and `Err` is non-nil; `Sender` is preserved in either event when available.
- Reject nil context, nil handler, and an ineligible interface with an error before starting a session.
- Use the observed compatibility endpoint `239.255.255.250:37020`. The endpoint is configurable only through a future explicit API/specification change, not by silent fallback.
- The context bounds the session lifetime. A nil return is reserved for normal context cancellation and successful cleanup; socket, send, receive, close, and caller-handler errors are returned.

## 4. Session Lifecycle

1. Open the multicast connection for the selected interface using M0.
2. Send exactly one inquiry probe through M1. On failure, close the connection and return the send/cleanup error.
3. Transfer connection ownership to M1 `Listen`, which receives until context cancellation or fatal transport/callback error.
4. For each datagram, call M2 `ParseAdvertisement`. On parse/validation failure, emit one `DatagramRejected` event and continue if the event handler succeeds. Never log packet bytes or sensitive XML values.
5. Emit `DeviceFound` for each valid advertisement. Callback failure terminates the session and is returned with context.
6. Do not retransmit, correlate UUIDs, or treat a particular `Hello` as caused by the probe. The caller cancels the context to stop discovery.

The discovery coordinator handles only one interface per invocation. It does not promise a device snapshot, deduplicate repeated advertisements, or aggregate errors from other interfaces.

## 5. Tests

Inject M0/M1/M2 dependencies at the implementation boundary using an unexported dependency bundle or equivalent function seams for open, send, listen, and parse. The exported `DiscoverOnInterface` uses production dependencies; tests invoke the internal orchestration with fakes. Do not add mutable package globals or make test hooks part of the public API.

Test open/send/listen ordering, connection cleanup on send failure, ownership transfer, normal cancellation, parse-error events followed by continued reception, valid `ProbeMatch` and `Hello` event delivery, callback failure, sender preservation, and redaction/no-payload logging. Tests must not require real devices or multicast access.

---

*Level 1 module specification. See [P6.1 DiscoverOnInterface](../procedures/discovery/discover_on_interface.md).*
