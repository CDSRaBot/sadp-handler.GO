# Procedure Specification: P6.1 - DiscoverOnInterface
## Module: M6 - Discovery Coordinator

## 1. Purpose

Run one cancellable discovery session on one selected interface, with either continuous scheduled probes or a bounded number of timeout-limited receive cycles.

The Python implementation's successful discovery and response reception on real devices is the behavioral acceptance baseline. Validate Go parity against the same device scenario; this does not make the application schedule a protocol retry requirement.

## 2. Go Contract

```go
type DiscoveryOptions struct {
    Multicast          adapter.MulticastOptions
    ProbeInterval       time.Duration
    CycleTimeout        time.Duration
    RepeatCount         int
    MaxSADPDatagramSize int
}

func DiscoverOnInterface(
    ctx context.Context,
    iface adapter.Interface,
    options DiscoveryOptions,
    handle EventHandler,
) error
```

- Reject nil context/handler, an ineligible interface, invalid multicast
  options, invalid or mixed timing options, and `MaxSADPDatagramSize` outside
  `1..65507` before network activity. M8 resolves settings before calling M6.
- Continuous mode is valid only when `ProbeInterval > 0`, `CycleTimeout == 0`, and `RepeatCount == 0`.
- Bounded mode is valid only when `ProbeInterval == 0`, `CycleTimeout > 0`, and `RepeatCount` is in `1..100`.
- Return nil on normal external cancellation after successful cleanup, or on completion of all bounded cycles and successful cleanup. Return socket, sender, listener, handler, or cleanup failures with stage context; preserve underlying causes.
- Each received packet produces either a validated `DeviceFound` event or a `DatagramRejected` event. Preserve its sender and record an `ObservedAt` timestamp. Keep raw event delivery distinct from presentation deduplication.

## 3. Session Lifecycle

1. Open one connection through M0 using `options.Multicast`.
2. Send the first probe immediately with M1 `SendInquiryProbe`. If it fails, close the connection and return the send error joined with any close error.
3. Start M1 `Listen` on that connection with a subordinate listener context and the resolved `MaxSADPDatagramSize`. After transfer, `Listen` owns closing it; M6 owns session coordination and scheduled writes. Require exactly one reader and one serialized writer on the `net.PacketConn`; no other component may use it.
4. In continuous mode, schedule another probe every `ProbeInterval`, measured from completion of the preceding scheduled send. Keep receiving between and during sends until external cancellation.
5. In bounded mode, start a receive timer after each successful probe. Do not stop a cycle early when a device is received. After the full `CycleTimeout`, send the next probe immediately if cycles remain; after the last full receive window, cancel the listener and return.
6. On every datagram, invoke M2 `ParseAdvertisement` with its sender and the same `MaxSADPDatagramSize`. On parse/validation error, emit a sanitized `DatagramRejected` event and continue if the handler succeeds. Emit `DeviceFound` for every valid packet.
7. A scheduled send failure cancels the session and is returned. On external cancellation or normal bounded completion, stop and join the scheduler before canceling the listener context; then wait for `Listen` to close the connection and finish cleanup. If a listener/handler failure occurs first, `Listen` may close the connection while one serialized write is completing; stop and join the scheduler, preserve both primary and concurrent write/close failures, and do not hide the original failure. Classify only errors caused by session cancellation as shutdown, never as a successful sent probe.

The probe cadence, receive-window length, and repeat count are application policy, not asserted SADP timing/retry requirements. Do not correlate UUIDs, infer reply ownership, or treat `Hello` as caused by a probe.

## 4. Discovery Result Aggregation

P6.1 emits every validated advertisement event and does not deduplicate at the protocol-event boundary. P4.5's application collector maintains the visible inventory keyed by normalized `(MAC, sender network address)`, updates the latest validated fields and last-seen time on repeats, and returns CLI snapshots sorted by MAC then sender. This key is a UI/application identity only.

## 5. Errors and Tests

Inject open/send/listen/parser dependencies through an unexported dependency bundle or equivalent function seams; do not expose test hooks or use mutable package globals. Test option validation, send/listen ordering, connection cleanup on open/first-send failure, immediate and periodic schedules, exact bounded cycle count and window length, receive continuity during scheduled writes, cancellation and goroutine joining, concurrent write/close failure propagation, parse-error events followed by continued reception, valid `ProbeMatch` and `Hello` event delivery, callback failure, sender/timestamp preservation, and redaction/no-payload logging. Unit tests must not require live devices or multicast access. Before claiming Go parity with the verified baseline, validate discovery and response reception against the same real-device scenario used for the Python implementation.

---

*Level 2 procedure specification.*
