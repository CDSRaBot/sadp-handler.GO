# Procedure Specification: P2.1 - ParseAdvertisement
## Module: M2 - Data Parser

## 1. Purpose

Parse one SADP XML advertisement received over UDP and return a typed, validated device model.

## 2. Go Contract

```go
func ParseAdvertisement(data []byte, sender net.Addr) (Device, error)
```

`Device` is the typed M2 model, containing the validated MAC, IPv4 address, activation state, model, serial number, and sender address.

- Reject an empty datagram, a datagram larger than the shared `MaxSADPDatagramSize`, or a nil sender with an error. The limit must be verified against SADP protocol requirements and must not be inferred from the Ethernet MTU.
- Return a populated `Device` and nil error only when the XML and all required fields are valid.
- Return an error and no usable device for malformed XML, missing required fields, or invalid field values.
- This procedure does not log or suppress errors. M1 or the application layer decides how malformed network traffic is reported.

## 3. Processing Requirements

1. Enforce the verified `MaxSADPDatagramSize` limit before parsing; do not include raw packet contents in diagnostics.
2. Decode the datagram as XML from bytes with Go's `encoding/xml`. Require valid UTF-8 as specified by the SADP payload; do not silently reinterpret invalid bytes using a different character encoding.
3. Reject malformed XML, unexpected trailing document content, and XML directives that are not permitted by the SADP format. Bound parsing by the input-size limit.
4. Extract and validate the required fields:
   - `<MAC>`: parse with `NormalizeMAC` (P2.2).
   - `<IPv4Address>`: parse as an IPv4 address with `net/netip`.
   - `<Activated>`: accept only case-insensitive `true` or `false`.
   - `<DeviceModel>` and `<SerialNumber>`: require non-empty values within documented protocol limits.
5. Preserve `sender` in the returned device model for source/address consistency checks by higher layers.
6. Ignore unknown optional elements when safe to do so; they must not override required values or weaken validation.

## 4. Errors and Tests

Errors should identify the field or parsing stage and preserve the underlying cause without embedding the full datagram or sensitive data. The parser must not panic on arbitrary input.

Tests should include valid advertisements, malformed and truncated XML, invalid UTF-8, directives, trailing content, missing and invalid fields, sender preservation, exact boundaries of the verified size limit, unknown elements, and Go fuzz tests over arbitrary byte slices.

---

*Level 2 procedure specification.*
