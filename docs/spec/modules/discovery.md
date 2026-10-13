# Module Specification: M6 - Discovery Coordinator

## 1. Purpose

Compose M0 interface-specific multicast setup, M1 inquiry transmission and packet reception, and M2 advertisement parsing into one cancellable discovery session.

## 2. Responsibilities

- Run one discovery session on one explicitly selected interface.
- Keep one SADP multicast connection open while sending inquiry probes and continuously receiving/parsing advertisements according to a validated continuous or bounded-cycle policy.
- Deliver valid devices and rejected-datagram diagnostics to the caller without exposing packet contents or security fields.
- Make connection ownership, cancellation, callback failures, and cleanup explicit.

M6 does not enumerate/select interfaces, correlate UUIDs, or interpret device-management/security responses. Its probe schedule is bounded application behavior, not protocol retry behavior. Callers can start separate sessions for separate interfaces and own any cross-interface concurrency policy.

## 3. Go API Contract

```go
type EventKind uint8

const (
    DeviceFound EventKind = iota + 1
    DatagramRejected
)

type Event struct {
    Kind       EventKind
    Device     *parser.Device
    Sender     net.Addr
    ObservedAt time.Time
    Err        error
}

type EventHandler func(context.Context, Event) error

type DiscoveryOptions struct {
    ProbeInterval time.Duration
    CycleTimeout  time.Duration
    RepeatCount   int
}

func DiscoverOnInterface(
    ctx context.Context,
    iface adapter.Interface,
    options DiscoveryOptions,
    handle EventHandler,
) error
```

- For `DeviceFound`, `Device` is non-nil and `Err` is nil. For `DatagramRejected`, `Device` is nil and `Err` is non-nil; `Sender` is preserved in either event when available. `ObservedAt` records when M6 accepted the datagram for processing.
- Reject nil context, nil handler, and an ineligible interface with an error before starting a session.
- Use the observed compatibility endpoint `239.255.255.250:37020`. The endpoint is configurable only through a future explicit API/specification change, not by silent fallback.
- Continuous mode requires `ProbeInterval > 0`, `CycleTimeout == 0`, and `RepeatCount == 0`; it sends immediately and then at each interval while the receive loop remains active.
- Bounded mode requires `ProbeInterval == 0`, `CycleTimeout > 0`, and `RepeatCount` in `1..100`; it sends immediately, receives for the full timeout, then starts the next cycle immediately. A device event does not end a cycle.
- Reject mixed, missing, non-positive, or out-of-range timing policies before opening a socket. These settings define application scheduling only and do not imply SADP protocol timing requirements.
- The session context controls scheduled sends; give `Listen` a subordinate context that M6 cancels only after stopping and joining the writer on orderly shutdown. This keeps `Listen` as the sole connection closer without racing a normal close against a scheduled write. If `Listen` fails first, it may close during an in-flight write; join the writer and preserve both errors. A nil return is reserved for normal session cancellation and successful cleanup; socket, send, receive, close, and caller-handler errors are returned.

## 4. Session Lifecycle

1. Validate options and open one multicast connection for the selected interface using M0.
2. Send the first inquiry probe immediately through M1. On failure, close the connection and return the send/cleanup error.
3. Start M1 `Listen` on the same connection and keep it active across scheduled probes. The coordinator owns the overall session; `Listen` owns closing the connection after transfer. M1's listener and probe sender may perform concurrent read/write operations on this connection, with one reader and one serialized writer.
4. In continuous mode, send one further probe every `ProbeInterval` until cancellation. In bounded mode, wait `CycleTimeout` after each probe; if cycles remain, send the next probe and begin its receive window. Stop after the final full receive window.
5. For each datagram, call M2 `ParseAdvertisement`. On parse/validation failure, emit one `DatagramRejected` event and continue if the event handler succeeds. Never log packet bytes or sensitive XML values.
6. Emit `DeviceFound` for each valid advertisement. Callback failure terminates the session and is returned with context.
7. A scheduled probe is a new independent call to M1 `SendInquiryProbe`; do not correlate UUIDs or treat a particular `Hello` as caused by a probe. The caller cancels a continuous session to stop discovery.

The discovery coordinator handles only one interface per invocation. It does not promise a device snapshot, deduplicate repeated advertisements, or aggregate errors from other interfaces.

## 5. Tests

Inject M0/M1/M2 dependencies at the implementation boundary using an unexported dependency bundle or equivalent function seams for open, send, listen, and parse. The exported `DiscoverOnInterface` uses production dependencies; tests invoke the internal orchestration with fakes. Do not add mutable package globals or make test hooks part of the public API.

Test policy validation, immediate and periodic sends, exact bounded cycle count and receive-window duration, no early stop on a device event, concurrent read/write, open/first-send failure cleanup, ownership transfer, normal cancellation, parse-error events followed by continued reception, valid `ProbeMatch` and `Hello` event delivery, callback/send failures, sender preservation, and redaction/no-payload logging. Tests must not require real devices or multicast access.

---

*Level 1 module specification. See [P6.1 DiscoverOnInterface](../procedures/discovery/discover_on_interface.md).*
