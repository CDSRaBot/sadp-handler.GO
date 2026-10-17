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

`NetworkConfig` is the typed M3 configuration with an IPv4 prefix/address, required IPv4 gateway for static configuration, and DHCP state.

- Validate the MAC, configuration, challenge, and password before constructing the payload.
- A missing challenge is an error. Never silently place a plaintext password in the request as a legacy fallback.
- Return complete UTF-8 XML bytes and nil error on success; return an error and no usable payload on failure.
- Do not log the password, challenge, derived response, or serialized payload.

## 3. Processing Requirements

1. Validate the target MAC as EUI-48 and apply the M3 network-configuration rules. For DHCP, static address and gateway fields must be unset; for static configuration, the address and mandatory gateway must be valid IPv4 values.
2. Require a non-empty challenge and calculate the response with P3.5 `CalculateHash`, using SHA-256 over UTF-8 `password + challenge` and lowercase hexadecimal output.
3. Match the local Python reference's current `<NetworkParameter>` XML shape using `encoding/xml`:
   - `<TargetMAC>`: uppercase, colon-separated EUI-48 text for the Python-reference compatibility fixture. This casing is not an independently verified wire-protocol requirement.
   - `<DHCP>`: lowercase `true` or `false`.
   - For static configuration, include non-empty `<IPv4Address>`, `<SubnetMask>`, and `<DefaultGateway>` values. Reject a missing gateway; it must never serialize as an empty element.
   - Include `<SecurityResponse>` with the calculated response.
4. Serialize with `encoding/xml`; prepend `xml.Header` because the encoder does not add a declaration automatically. Do not build XML by string formatting.
5. Keep the result as a payload for transport integration; this procedure does not perform network I/O. M1 P1.4 can send the SADP datagram, but this builder does not acquire the challenge or interpret the device's network-update result. Do not describe this builder alone as a completed network-configuration operation.

The request shape and challenge-response calculation are accepted based on practical experiments. Go must not copy the Python reference's missing-challenge fallback, which emits `<Password>` in plaintext.

## 4. Errors and Tests

Return typed M3 validation errors for invalid input or a missing challenge; preserve hashing and XML serialization causes. Test DHCP/static branches, missing gateway, invalid and contradictory configuration, MAC validation, confirmed challenge-response vectors, XML escaping, element casing, and absence of plaintext fallback.

---

*Level 2 procedure specification.*
