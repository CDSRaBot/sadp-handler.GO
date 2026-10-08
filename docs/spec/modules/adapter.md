# Module Specification: M0 - Platform Adapter

## 1. Purpose

Provide a cross-platform boundary for discovering local network interfaces and opening the UDP connection used by the SADP network engine. SADP discovery uses UDP multicast on the local network; it does not require a raw Ethernet socket.

## 2. Responsibilities

- Enumerate network interfaces and expose the attributes needed to select an interface for discovery.
- Open a UDP packet connection configured for the selected interface and SADP multicast settings.
- Isolate operating-system-specific socket options and report unsupported configurations explicitly.
- Return resources to the caller with a clear ownership contract; the caller closes every successfully opened connection.

The adapter does not send probes, parse advertisements, or implement device-management rules.

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
- The caller owns the returned `net.PacketConn` and must close it, including on cancellation and error paths.
- Returned data uses Go networking types from `net` and `net/netip`; callers must not rely on OS-specific adapter names or socket internals for portable behavior.

## 4. Interface Selection and Portability

- Exclude loopback and interfaces that are down or lack a usable IPv4 address for the requested operation.
- Preserve enough interface identity (index and name) to bind traffic to the selected adapter; do not assume a hostname resolves to the correct interface.
- Use Go's `net` package for interface enumeration and packet connections. Add platform-specific code only where required by OS socket behavior.
- Support Windows, Linux (including Termux where Go and the required network permissions are available), and macOS where the necessary multicast socket behavior is supported.
- Report permission, bind, and unsupported-platform failures as errors with operation context. Do not silently fall back to a different interface.

## 5. Error and Resource Policy

- Validate the interface and multicast group before opening a socket.
- Wrap system errors with enough context to identify the operation and interface while preserving the underlying error for inspection.
- Close partially initialized sockets if setup fails.
- Do not use `panic` for ordinary enumeration or socket failures.

---

*Level 1 module specification. The related procedure documents under `../procedures/adapter/` are separate from this Go module contract.*
