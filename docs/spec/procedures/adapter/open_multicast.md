# Procedure Specification: P0.2 - OpenMulticast
## Module: M0 - Platform Adapter

## 1. Purpose

Open a UDP packet connection for SADP multicast traffic on one explicitly selected host interface.

## 2. Go Contract

```go
func OpenMulticast(iface Interface, group netip.AddrPort) (net.PacketConn, error)
```

- Reject an ineligible or unavailable interface, an invalid/non-multicast IPv4 group, or an invalid/zero UDP port.
- Return a connection configured for the requested interface and group, or an error with no usable connection.
- The returned connection supports one concurrent reader and one serialized writer on the same interface, as required by M6 scheduled probes during continuous reception.
- The caller owns and must close every successfully returned connection unless it transfers ownership to M1 `Listen`, which closes it on all return paths.

## 3. Processing Requirements

1. Validate the selected interface and multicast endpoint before opening sockets. Do not silently select a different interface or endpoint.
2. Join the requested IPv4 multicast group on the selected interface and configure send/receive behavior as supported by the target OS. A user-provided HiToolsDelivery capture observes inquiry traffic to `239.255.255.250:37020`; treat it as the current compatibility target, not an authoritative protocol constant for all devices.
3. Bind the receive socket to the requested group port (`37020` for the observed endpoint) where supported. Do not copy the Python Windows branch that binds an ephemeral port without joining the group; implement and verify Windows behavior explicitly.
4. Ensure one `ReadFrom` and a serialized `WriteTo` can be active concurrently on the returned connection. Keep both directions tied to the selected interface.
5. Do not set an unverified multicast TTL or infer cross-subnet discovery requirements. Any such option must be justified by a verified requirement.
6. Close any partially initialized socket on setup failure and return the original cause wrapped with operation/interface context. Do not expose a partially configured connection as success.
7. Leave cancellation-aware reads and receive-loop lifecycle to M1; the returned connection remains owned by the caller.

## 4. Errors and Tests

Return explicit, wrapped errors for invalid arguments, interface selection, permissions, bind/join failures, and unsupported platform behavior. Test invalid endpoints, interface/group binding, resource cleanup after partial setup, supported-platform behavior, and Windows multicast membership. Tests must not depend on external devices or a live multicast network where socket behavior can be injected.

---

*Level 2 atomic procedure specification.*
