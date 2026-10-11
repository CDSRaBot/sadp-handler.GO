# Module Specification: M0 - Platform Adapter

## 1. Purpose

Provide a cross-platform boundary for discovering local network interfaces and opening the UDP connection used by the SADP network engine. The Python reference uses UDP multicast at `239.255.255.250:37020`; this is the compatibility target for the initial Go implementation, not an authoritative protocol guarantee. Raw Ethernet SADP is outside this adapter contract unless device evidence requires it.

## 2. Responsibilities

- Enumerate network interfaces and expose the attributes needed to select an interface for discovery.
- Open a UDP packet connection configured for the selected interface and SADP multicast settings. The user-provided HiToolsDelivery capture observes inquiry traffic to `239.255.255.250:37020`; treat that endpoint as the current compatibility target, not a universal protocol guarantee.
- Isolate operating-system-specific socket options and report unsupported configurations explicitly.
- Return resources to the caller with a clear ownership contract; the caller closes every successfully opened connection.

The adapter does not send probes, parse advertisements, orchestrate discovery sessions, or implement device-management rules.

## 3. Go API Contract

The following signatures describe the module contract; concrete package paths and exported names are finalized with the implementation:

```go
type Interface struct {
    Index        int
    Name         string
    HardwareAddr net.HardwareAddr
    IPv4Addrs    []netip.Prefix
    Flags        net.Flags
}

func ListInterfaces() ([]Interface, error)
func OpenMulticast(iface Interface, group netip.AddrPort) (net.PacketConn, error)
```

- `ListInterfaces` returns eligible interfaces and an error if enumeration fails. An empty result is valid when no eligible interface exists.
- `OpenMulticast` validates its arguments and returns a connection configured to send and receive SADP UDP traffic on the requested interface. On error, it returns no usable connection.
- The caller owns the returned `net.PacketConn` until it transfers ownership to M1 `Listen`; if it does not pass the connection to `Listen`, it must close it. `Listen` closes a transferred connection on all return paths.
- Returned data uses Go networking types from `net` and `net/netip`; callers must not rely on OS-specific adapter names or socket internals for portable behavior.

## 4. Interface Selection and Portability

- Exclude loopback and interfaces that are down or lack a usable IPv4 address for the requested operation.
- Preserve enough interface identity (index and name) to bind traffic to the selected adapter; do not assume a hostname resolves to the correct interface.
- Enumerate interfaces through Go's `net` package and join `239.255.255.250` on the selected interface, binding the receive socket to UDP port `37020` where supported. The same endpoint is observed in a HiToolsDelivery capture; it remains a compatibility target pending broader device verification.
- The Python reference sets multicast TTL to 64. Preserve that value only if required by observed behavior; it does not establish SADP's required TTL or imply that discovery should cross subnets.
- The Python reference's Unix-like branch binds the group port and joins on the selected interface. Its Windows branch binds an ephemeral local port and omits group membership. This inconsistency is not a target behavior: implement and verify Windows multicast send/receive explicitly, and return an error rather than silently claiming that an unverified socket is configured correctly.
- Use Go's `net` package for interface enumeration and packet connections. Add platform-specific code only where required by OS socket behavior.
- Support Windows, Linux (including Termux where Go and the required network permissions are available), and macOS where the necessary multicast socket behavior is supported.
- Report permission, bind, and unsupported-platform failures as errors with operation context. Do not silently fall back to a different interface.

## 5. Error and Resource Policy

- Validate the interface and multicast group before opening a socket.
- Wrap system errors with enough context to identify the operation and interface while preserving the underlying error for inspection.
- Close partially initialized sockets if setup fails.
- Do not use `panic` for ordinary enumeration or socket failures.

Atomic contracts are specified in [P0.1 ListInterfaces](../procedures/adapter/get_interfaces.md), [P0.2 OpenMulticast](../procedures/adapter/open_multicast.md), and [P0.3 getLinuxMAC](../procedures/adapter/get_linux_mac.md).

---

*Level 1 module specification. The related procedure documents under `../procedures/adapter/` are separate from this Go module contract. End-to-end discovery is composed by [M6](./discovery.md).*
