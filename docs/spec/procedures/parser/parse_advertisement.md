# Procedure Specification: P2.1 - ParseAdvertisement
## Module: M2 - Data Parser

## 1. Purpose

Parse one SADP XML advertisement received over UDP and return a typed, validated device model.

## 2. Go Contract

```go
func ParseAdvertisement(data []byte, sender net.Addr) (Device, error)
```

`Device` is the typed M2 model, containing the validated MAC, IPv4 address, activation state, model, serial number, and sender address.

- Reject an empty datagram, a datagram larger than the shared `MaxSADPDatagramSize`, or a nil sender with an error. This is an application resource bound, not a claimed protocol maximum; do not infer it from Ethernet MTU.
- Return a populated `Device` and nil error only when the XML and all required fields are valid.
- Return an error and no usable device for malformed XML, missing required fields, or invalid field values.
- This procedure does not log or suppress errors. M1 or the application layer decides how malformed network traffic is reported.

## 3. Processing Requirements

1. Enforce the single shared `MaxSADPDatagramSize` application bound before parsing; M1 must use the same limit and detect truncation before dispatch. The Python reference uses 1500 bytes, which may be retained as a named resource/compatibility setting but is not a verified SADP protocol maximum.
2. Decode the datagram as XML from bytes with Go's `encoding/xml`. The Python reference decodes UTF-8 and falls back to ISO-8859-1; only enable this fallback if it is needed for known-good compatibility fixtures.
3. Reject malformed XML, unexpected trailing document content, and XML directives that are not permitted by the SADP format. Bound parsing by the input-size limit.
4. Extract direct child fields following the Python reference:
   - `<MAC>`: parse with `NormalizeMAC` (P2.2). The reference uppercases valid MAC values and inserts colon separators; Go rejects malformed values instead of returning the raw string.
   - `<IPv4Address>`: parse as an IPv4 address with `net/netip`.
   - `<Activated>`: the reference treats a case-insensitive `"true"` as active and any other value as inactive. If absent or empty, Go sets `Activated` to `false`; it accepts case-insensitive `true`/`false` and rejects unknown non-empty values.
   - `<DeviceModel>`, `<SerialNumber>`, and optional `<SoftwareVersion>`: preserve values as supplied; model and serial number need not be non-empty. If software version is absent, set it to `"Unknown"` as in the Python reference.
5. Preserve `sender` in the returned device model for source/address consistency checks by higher layers.
6. Ignore unknown optional elements when safe to do so; they must not override required values or weaken validation. The Python parser uses safe XML parsing (`defusedxml`) and checks root-level children; Go's standard `encoding/xml` does not resolve external entities, and should likewise avoid expanding the accepted structure without fixtures.

## 4. Errors and Tests

Errors should identify the field or parsing stage and preserve the underlying cause without embedding the full datagram or sensitive data. The parser must not panic on arbitrary input.

Tests should include valid advertisements, malformed and truncated XML, invalid UTF-8, directives, trailing content, missing and invalid fields, sender preservation, exact boundaries of the verified size limit, unknown elements, and Go fuzz tests over arbitrary byte slices.

---

*Level 2 procedure specification.*
