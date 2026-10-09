# Module Specification: M2 - Data Parser

## 1. Purpose

Validate untrusted SADP datagrams and transform valid device advertisements into typed Go values for the logic and presentation layers.

## 2. Responsibilities

- Parse the XML advertisement shape used by the Python reference with Go's standard-library `encoding/xml`.
- Validate required fields and normalize values such as MAC and IPv4 addresses.
- Represent device attributes using typed Go fields rather than loosely typed maps.
- Return errors that distinguish malformed XML from missing or invalid required values.

The parser does not perform network I/O, log credentials, or execute device-management workflows.

## 3. Go API Contract

The following signatures and fields describe the module contract; concrete package paths and exported names are finalized with the implementation:

```go
type Device struct {
    MAC          net.HardwareAddr
    IPv4         netip.Addr
    Activated    bool
    Model        string
    SerialNumber string
    SoftwareVersion string
    Sender       net.Addr
}

func ParseAdvertisement(data []byte, sender net.Addr) (Device, error)
func NormalizeMAC(value string) (net.HardwareAddr, error)
```

- `ParseAdvertisement` receives the datagram and sender address from M1, and returns a validated device and a nil error on success. On failure it returns an error and no usable device.
- `MAC` and `IPv4` are required. If `<Activated>` is absent or empty, set `Activated` to `false` to match the Python reference; accept case-insensitive `true`/`false` and reject other non-empty values. `Model` and `SerialNumber` may be empty. If `<SoftwareVersion>` is absent, use `"Unknown"` as in the reference.
- The Python reference accepts non-empty packets up to its `MAX_PACKET_SIZE` of 1500 bytes, decodes UTF-8 and falls back to ISO-8859-1, and parses the XML with `defusedxml`. The 1500-byte value is not a verified SADP protocol limit. A Go implementation may retain it as the named `MaxSADPDatagramSize` application resource bound shared with M1; do not describe it as the protocol maximum or infer its validity from Ethernet MTU.
- The reference reads direct child values for `MAC`, `IPv4Address`, `Activated`, `DeviceModel`, `SerialNumber`, and optionally `SoftwareVersion`; it requires only MAC and IPv4 address to construct a model. Preserve these observed field names and direct-child behavior in fixtures. More restrictive requirements need device evidence and a deliberate spec revision.
- Empty or oversized packets, malformed XML, invalid required values, and incomplete advertisements are errors in the Go API even though the Python callback-oriented parser returns `None` for several invalid cases.
- Unknown optional XML elements may be ignored for firmware forward compatibility. They must not override or weaken validation of required fields.
- `NormalizeMAC` accepts only supported MAC representations and returns a canonical `net.HardwareAddr`; malformed values produce an error rather than a partially normalized value.

## 4. Parsing and Validation

- Decode the XML directly from the bounded byte slice using `encoding/xml`; do not require an intermediate string or a third-party XML package. The Python parser falls back to ISO-8859-1 when UTF-8 decoding fails; preserve that behavior only if a known-good fixture demonstrates that devices emit such payloads. Do not silently reinterpret invalid UTF-8 without documenting and testing the compatibility rule.
- The Python reference displays normalized MAC values in uppercase colon-separated form. Go stores the parsed six-byte value as `net.HardwareAddr`; its conventional `String()` display form is lowercase colon-separated. Treat this as a presentation difference, not a difference in address identity. Protocol serialization casing is specified separately by the relevant XML procedure and remains subject to fixture verification.
- The Python reference marks a device active only when `Activated`, after lowercasing, equals `"true"`; other values become inactive. Go must reject unrecognized non-empty activation values rather than silently treating them as false.
- Validate IPv4 values using `net/netip` and validate hardware addresses using `net`.
- Enforce required-field, string-length, and protocol-value constraints before constructing `Device`. `MaxSADPDatagramSize`, if used, is one shared application resource limit for M1 and M2, not a claimed SADP protocol maximum; the Python reference's 1500-byte cap is a compatibility starting point, not an MTU-derived protocol rule.
- Preserve the sender address supplied by M1 so callers can compare the advertised address with the source of the datagram.
- Do not assume that syntactically valid XML is a valid SADP advertisement.

## 5. Errors and Diagnostics

- Return explicit errors for invalid input; do not log and return a nil/empty success value.
- Callers decide whether a malformed datagram is ignored, counted, or reported. Parser errors must contain enough context for that decision without including sensitive data.
- Do not include packet contents, passwords, or security values in error messages.
- The parser must not panic on arbitrary input. Tests should include malformed XML, missing fields, boundary-sized packets, and fuzzed byte sequences.

---

*Level 1 module specification. The related procedure documents under `../procedures/parser/` are separate from this Go module contract.*
