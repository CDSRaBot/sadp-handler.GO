# Procedure Specification: P3.1 - BuildActivationXML
## Module: M3 - Device Logic

## 1. Purpose

Build the SADP XML request for activating one device using validated credentials and optional recovery data.

## 2. Go Contract

```go
func BuildActivationXML(request ActivationRequest) ([]byte, error)
```

`ActivationRequest` is the typed M3 request containing a six-byte MAC address, password, and optional recovery email.

- Validate all required fields, including the password policy defined by M3, before serialization.
- Return complete UTF-8 XML bytes and nil error on success.
- Return an error and no usable payload for invalid input or serialization failure.
- Do not log the password, recovery email, or serialized payload.

## 3. Processing Requirements

1. Validate the target MAC as an EUI-48 address and validate the password using M3's password policy. The Python reference requires 8–16 characters and at least two of lowercase, uppercase, digit, or non-alphanumeric classes.
2. Construct XML with `encoding/xml`; do not concatenate untrusted values into XML strings.
3. Match the local Python reference's current element names and casing:
   - Root: `<Activate>`.
   - Target: `<TargetMAC>` using uppercase, colon-separated EUI-48 text for the Python-reference compatibility fixture. This casing is not an independently verified wire-protocol requirement.
   - Credential: `<Password>`.
   - If recovery email is present, include `<OptionData><SecurityEmail>…</SecurityEmail></OptionData>`.
4. Let the XML encoder escape text content. Since `encoding/xml` does not add a declaration automatically, prepend `xml.Header` to the marshaled XML bytes.
5. Keep the resulting payload scoped to the operation. M1 P1.4 `SendSADPDatagram` sends the payload as one SADP datagram to the multicast endpoint on the selected interface. The request is identified by `TargetMAC`; do not substitute an ISAPI HTTP request or endpoint. This builder does not interpret the device's activation result and alone is not a completed activation operation.

SADP XML activation with `TargetMAC` is the selected protocol variant, not ISAPI HTTP activation. The element names and optional recovery field above are the request shape adopted from the Python implementation for this contract; activation-result semantics still require end-to-end testing.

## 4. Errors and Tests

Return a typed M3 validation error for invalid input and preserve serialization causes; messages must not disclose secret values. Test required-field validation, password-policy rejection, optional email behavior, XML escaping, exact element names and casing, and serialization failures.

---

*Level 2 procedure specification.*
