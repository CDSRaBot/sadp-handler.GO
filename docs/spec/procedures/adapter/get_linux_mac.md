# Procedure Specification: P0.3 - getLinuxMAC
## Module: M0 - Platform Adapter

## 1. Purpose

Read and validate the hardware address for a named Linux or Android (Termux) network interface. This is an internal platform-specific helper; portable interface enumeration should prefer Go's `net.Interface` APIs.

## 2. Go Contract

```go
func getLinuxMAC(interfaceName string) (net.HardwareAddr, error)
```

- Reject an empty interface name or a name containing path separators or a NUL byte.
- Return a validated six-byte EUI-48 hardware address on success.
- Return an error if the interface address cannot be read or is invalid. Do not return an empty string, placeholder, or guessed address.

## 3. Processing Requirements

1. Build the sysfs path with `filepath.Join("/sys/class/net", interfaceName, "address")`.
2. Read the file with `os.ReadFile`.
3. Trim surrounding whitespace and parse the value with `net.ParseMAC`.
4. Reject parsed addresses that are not six bytes long.

Do not use a host-wide fallback such as a UUID-derived node value: it may identify a different adapter and is not a valid substitute for the requested interface.

## 4. Errors and Tests

Wrap file and parse errors with the interface name and operation while preserving the underlying error. Do not expose file contents in errors.

Tests should cover valid and malformed addresses, absent and unreadable files, invalid interface names, and rejection of non-EUI-48 addresses.

---

*Level 3 atomic procedure specification.*
