# Procedure Specification: P0.1 - ListInterfaces
## Module: M0 - Platform Adapter

## 1. Purpose

Enumerate host network interfaces that can be considered for SADP IPv4 discovery.

## 2. Go Contract

```go
func ListInterfaces() ([]Interface, error)
```

- On success, return zero or more eligible interfaces and a nil error.
- If enumeration or address inspection fails for the host, return an error with operation context. Do not fabricate adapter data.
- An empty result is valid when no interface meets the eligibility rules.

`Interface` is the typed value defined by M0 and contains the interface index, name, hardware address, IPv4 prefixes, and flags.

## 3. Processing Requirements

1. Enumerate interfaces and addresses using Go's `net` package.
2. Include only interfaces that are up, are not loopback, and have at least one usable IPv4 address.
3. Preserve the interface index and name needed to bind SADP traffic to that interface.
4. Copy address and hardware-address data into the returned values; do not expose mutable internal buffers.
5. Do not assume hostname resolution identifies the correct interface, and do not silently substitute another interface if a requested one is unavailable.

Virtual interfaces are not excluded solely because they are virtual; eligibility is determined by the same flags and address rules.

## 4. Errors and Tests

Return wrapped errors for failures from interface enumeration or address retrieval. Do not use `panic` or return partially populated results as if enumeration had succeeded.

Tests should cover loopback and down interfaces, missing IPv4 addresses, multiple eligible addresses, empty results, and injected enumeration errors where the implementation permits dependency injection.

---

*Level 2 procedure specification.*
