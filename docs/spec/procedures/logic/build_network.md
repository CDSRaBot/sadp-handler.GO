# Procedure Specification: P3.2 - BuildNetworkXML
## Module: M3 - Device Logic

## 1. Purpose

Build the SADP XML request for a device network update, including the verified security response required by the target protocol.

## 2. Go Contract

```go
func BuildNetworkXML(
    mac net.HardwareAddr,
    config NetworkConfig,
    challenge string,
    password string,
) ([]byte, error)
```

`NetworkConfig` is the typed M3 configuration with an IPv4 prefix/address, optional IPv4 gateway, and DHCP state.

- Validate the MAC, configuration, challenge, and password before constructing the payload.
- A missing challenge is an error. Never silently place a plaintext password in the request as a legacy fallback.
- Return complete UTF-8 XML bytes and nil error on success; return an error and no usable payload on failure.
- Do not log the password, challenge, derived response, or serialized payload.

## 3. Processing Requirements

1. Validate the target MAC as EUI-48 and apply the M3 network-configuration rules. For DHCP, static address and gateway fields must be unset; for static configuration, the address must be IPv4 and any specified gateway must be valid IPv4.
2. Require a non-empty challenge and calculate the reference response with P3.5 `CalculateHash`. Its algorithm remains subject to the protocol verification gate in that procedure.
3. Match the local Python reference's current `<NetworkParameter>` XML shape using `encoding/xml`:
   - `<TargetMAC>`: uppercase, colon-separated EUI-48 text for the Python-reference compatibility fixture. This casing is not an independently verified wire-protocol requirement.
   - `<DHCP>`: lowercase `true` or `false`.
   - For static configuration, include non-empty `<IPv4Address>`, `<SubnetMask>`, and `<DefaultGateway>` values.
   - Include `<SecurityResponse>` with the calculated response.
4. Serialize with `encoding/xml`; prepend `xml.Header` because the encoder does not add a declaration automatically. Do not build XML by string formatting.
5. Keep the result as a payload for the future transport integration; this procedure does not perform network I/O. M1 currently specifies receiving only and has no send API, so network-configuration transmission and response handling remain open protocol gates. Do not describe this builder alone as a completed network-configuration operation.

The Python reference conditionally emits each non-empty static-address field. Its missing-challenge branch emits `<Password>` in plaintext; Go must not copy this fallback. The exact XML fields, optional-field rules, and security-response algorithm must be verified against known-good device fixtures before claiming device compatibility.

## 4. Errors and Tests

Return explicit errors for invalid input, missing challenge, hashing failure, or XML serialization failure. Test DHCP/static branches, invalid and contradictory configuration, MAC validation, verified challenge-response vectors, XML escaping, element casing, and absence of plaintext fallback.

---

*Level 2 procedure specification.*
