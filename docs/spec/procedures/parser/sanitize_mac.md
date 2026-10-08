# Procedure Specification: P2.2 - NormalizeMAC
## Module: M2 - Data Parser

## 1. Purpose

Parse a device MAC address into a validated, canonical six-byte Go hardware address for consistent comparisons and protocol serialization.

## 2. Go Contract

```go
func NormalizeMAC(value string) (net.HardwareAddr, error)
```

- Accept MAC representations supported by `net.ParseMAC` only when they represent a six-byte EUI-48 address.
- Return the parsed `net.HardwareAddr` and nil on success.
- Return an error for empty, malformed, or non-EUI-48 values. Never return the raw input as a successful fallback.
- `net.HardwareAddr.String()` provides the canonical lowercase colon-separated display form.

## 3. Processing Requirements

1. Parse with `net.ParseMAC`; do not strip arbitrary non-hexadecimal characters, since doing so could turn invalid input into a different address.
2. Require exactly six bytes.
3. Do not log malformed input verbatim when it may contain untrusted control characters.

## 4. Errors and Tests

Return an error that callers can wrap with XML field or device context. The caller decides whether an invalid advertisement is discarded or reported.

Tests should cover canonical colon form, other forms explicitly accepted by `net.ParseMAC`, invalid characters, wrong byte counts, empty input, and canonical formatting.

---

*Level 3 atomic procedure specification.*
