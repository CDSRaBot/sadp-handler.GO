# Module Specification: M2 - Data Parser

## 1. Purpose

Validate untrusted SADP datagrams and transform valid device advertisements into typed Go values for the logic and presentation layers.

## 2. Responsibilities

- Parse SADP XML with Go's standard-library `encoding/xml`.
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
    Sender       net.Addr
}

func ParseAdvertisement(data []byte, sender net.Addr) (Device, error)
func NormalizeMAC(value string) (net.HardwareAddr, error)
```

- `ParseAdvertisement` receives the datagram and sender address from M1, and returns a validated device and a nil error on success. On failure it returns an error and no usable device.
- Empty or oversized packets, malformed XML, invalid required values, and incomplete advertisements are errors. The maximum accepted packet size must be one explicitly defined limit shared with M1 and verified against SADP protocol requirements; do not assume it is 1500 bytes based on Ethernet MTU.
- Unknown optional XML elements may be ignored for firmware forward compatibility. They must not override or weaken validation of required fields.
- `NormalizeMAC` accepts only supported MAC representations and returns a canonical `net.HardwareAddr`; malformed values produce an error rather than a partially normalized value.

## 4. Parsing and Validation

- Decode the XML directly from the bounded byte slice using `encoding/xml`; do not require an intermediate string or a third-party XML package.
- Validate IPv4 values using `net/netip` and validate hardware addresses using `net`.
- Enforce required-field, string-length, and protocol-value constraints before constructing `Device`.
- Preserve the sender address supplied by M1 so callers can compare the advertised address with the source of the datagram.
- Do not assume that syntactically valid XML is a valid SADP advertisement.

## 5. Errors and Diagnostics

- Return explicit errors for invalid input; do not log and return a nil/empty success value.
- Callers decide whether a malformed datagram is ignored, counted, or reported. Parser errors must contain enough context for that decision without including sensitive data.
- Do not include packet contents, passwords, or security values in error messages.
- The parser must not panic on arbitrary input. Tests should include malformed XML, missing fields, boundary-sized packets, and fuzzed byte sequences.

---

*Level 1 module specification. The related procedure documents under `../procedures/parser/` are separate from this Go module contract.*
