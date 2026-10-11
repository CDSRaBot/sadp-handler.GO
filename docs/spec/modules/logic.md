# Module Specification: M3 - Device Logic

## 1. Purpose

Implement device-management rules for activation and network configuration. The module validates requests, builds protocol messages, coordinates operations, and translates device responses into explicit outcomes.

## 2. Responsibilities

- Validate activation credentials and network configuration before constructing commands.
- Construct SADP XML using typed Go structures and `encoding/xml`.
- Coordinate the challenge-response flow and command/result correlation when required by the device protocol.
- Keep workflow rules independent of CLI presentation and operating-system-specific socket setup.
- Return per-device results for bulk operations so failures remain attributable to individual devices.

The exact challenge-response algorithm and wire-level fields must be specified by the corresponding protocol procedures; this module specification does not prescribe an unverified hash algorithm. Password policy validation is a pure procedure; parsing a device challenge, submitting commands, and interpreting operation results remain separate workflow boundaries.

M1 defines inquiry-probe sending and receive APIs. The XML builders in this module do not provide end-to-end activation or network-configuration operations until a verified send/response transport contract and the corresponding context-aware workflow APIs are specified.

The local Python reference provides a behavioral proposal: it builds `Activate` XML with `TargetMAC`, `Password`, and optional `OptionData/SecurityEmail`; it builds `NetworkParameter` XML with `TargetMAC`, `DHCP`, optional static-address fields, and `SecurityResponse` when a challenge is supplied. It calculates that response as SHA-256 of UTF-8 `password + challenge`. These shapes and calculation are useful for compatibility fixtures but are not authoritative protocol facts until compared with a known-good device exchange.

## 3. Go API Contract

The following signatures describe the module contract; concrete package paths and exported names are finalized with the implementation:

```go
type ActivationRequest struct {
    MAC      net.HardwareAddr
    Password string
    Email    string
}

type NetworkConfig struct {
    Address netip.Prefix
    Gateway netip.Addr
    UseDHCP bool
}

func ValidatePassword(password string) error
func BuildActivationXML(request ActivationRequest) ([]byte, error)
func BuildNetworkXML(
    mac net.HardwareAddr,
    config NetworkConfig,
    challenge string,
    password string,
) ([]byte, error)
```

- Validation functions return `nil` only when the input satisfies the documented policy; invalid values return descriptive, inspectable errors.
- XML builders validate their complete input and return serialized UTF-8 XML bytes or an error. They must not return a partially usable payload on failure.
- For static configuration, `Address` must contain a valid IPv4 address and prefix; a specified `Gateway` must be a valid IPv4 address. For DHCP, static address and gateway fields must be unset. Invalid or contradictory combinations are errors.
- `BuildNetworkXML` receives the target device MAC and security inputs separately from network configuration; it requires the challenge-response behavior specified by the related procedures and must not silently fall back to plaintext credentials.
- Operation-level APIs must accept `context.Context` when they perform network or other blocking I/O and return a result or error for each target device.

## 4. Security and Error Policy

- Do not log passwords, derived secrets, challenge values, or complete activation payloads.
- Keep credential scope and lifetime limited to the operation. Go strings cannot be reliably erased from memory; do not promise secure zeroization.
- Do not embed protocol secrets or assume a hash/response algorithm unless it is confirmed by the protocol specification.
- Return errors with operation and device context while avoiding disclosure of credentials. Preserve wrapped causes for callers.
- Do not use `panic` for invalid requests, device rejections, or transport failures.
- Do not copy the Python reference's missing-challenge fallback, which inserts the plaintext password into the network-configuration XML. A missing challenge must fail closed unless an explicitly named, separately reviewed compatibility mode is required and approved.

## 5. Testing Requirements

- Test password policy boundaries and invalid network configurations.
- Verify XML element names, casing, escaping, encoding, and optional-field behavior against protocol fixtures.
- Test successful and rejected device responses, timeouts, cancellation, and per-device outcomes in bulk operations.
- Use deterministic test inputs; never include production credentials or secrets in fixtures.

---

*Level 1 module specification. See [P3.0 ValidatePassword](../procedures/logic/validate_password.md), [P3.1 BuildActivationXML](../procedures/logic/build_activation.md), [P3.2 BuildNetworkXML](../procedures/logic/build_network.md), and [P3.5 CalculateHash](../procedures/logic/calculate_hash.md). Challenge extraction, activation execution/result parsing, and network-update execution/result parsing are intentionally not specified as executable operations until protocol evidence is available.*
