# Module Specification: M3 - Device Logic

## 1. Purpose

Implement device-management rules for activation and network configuration. The module validates requests, builds protocol messages, coordinates operations, and translates device responses into explicit outcomes.

## 2. Responsibilities

- Validate activation credentials and network configuration before constructing commands.
- Construct SADP XML using typed Go structures and `encoding/xml`.
- Coordinate the challenge-response flow and command/result correlation when required by the device protocol.
- Keep workflow rules independent of CLI presentation and operating-system-specific socket setup.
- Return per-device results for bulk operations so failures remain attributable to individual devices.

The SADP activation request is addressed by the device MAC and uses the XML fields specified in P3.1. Practical experiments confirm the challenge-response calculation in P3.5; its exact byte order and encoding are normative compatibility requirements. Password policy validation is a pure procedure; challenge acquisition and interpreting device operation results remain separate workflow boundaries.

M1 defines both inquiry-probe transmission and generic SADP datagram transmission. XML builders produce request payloads; they do not themselves provide end-to-end activation or network-configuration operations. Those workflows must use the context-aware transport API and explicitly interpret per-device outcomes before being enabled.

The selected activation variant is SADP XML with `TargetMAC` identifying the device; it is not ISAPI HTTP activation. P3.1 defines the request shape, with its Python-derived optional fields retained as compatibility requirements pending end-to-end activation outcome testing. For network configuration, the SHA-256 calculation over UTF-8 `password + challenge` is confirmed by practical experiments and is normative. Target MAC text is uppercase, colon-separated EUI-48.

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

type DeviceOperationErrorKind string

const (
    OperationInvalidInput DeviceOperationErrorKind = "invalid_input"
    OperationTransport    DeviceOperationErrorKind = "transport"
    OperationRejected     DeviceOperationErrorKind = "device_rejected"
    OperationCanceled     DeviceOperationErrorKind = "canceled"
)

type DeviceOperationError struct {
    Kind DeviceOperationErrorKind
    Err  error
}

func (e *DeviceOperationError) Error() string
func (e *DeviceOperationError) Unwrap() error

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
- For static configuration, `Address` must contain a valid IPv4 address and prefix and `Gateway` must contain a valid IPv4 address. The gateway is mandatory. For DHCP, static address and gateway fields must be unset. Invalid or contradictory combinations are errors.
- `BuildNetworkXML` receives the target device MAC and security inputs separately from network configuration; it requires the challenge-response behavior specified by the related procedures and must not silently fall back to plaintext credentials.
- Operation-level APIs must accept `context.Context` when they perform network or other blocking I/O and return a result or error for each target device.
- Operation APIs return `DeviceOperationError` for caller-branchable failures. `Kind` is stable and callers use `errors.As`; `Unwrap` preserves the cause. Context cancellation must also remain detectable with `errors.Is(err, context.Canceled)`.

## 4. Security and Error Policy

- Do not log passwords, derived secrets, challenge values, or complete activation payloads.
- Keep credential scope and lifetime limited to the operation. Go strings cannot be reliably erased from memory; do not promise secure zeroization.
- Use P3.5's confirmed challenge-response calculation; do not substitute plaintext credentials if the challenge is missing.
- Return errors with operation and device context while avoiding disclosure of credentials. Preserve wrapped causes for callers.
- Do not use `panic` for invalid requests, device rejections, or transport failures.
- Do not copy the Python reference's missing-challenge fallback, which inserts the plaintext password into the network-configuration XML. A missing challenge must fail closed unless an explicitly named, separately reviewed compatibility mode is required and approved.

## 5. Testing Requirements

- Test password policy boundaries and invalid network configurations.
- Verify XML element names, casing, escaping, encoding, and optional-field behavior against protocol fixtures.
- Test successful and rejected device responses, timeouts, cancellation, and per-device outcomes in bulk operations.
- Use deterministic test inputs; never include production credentials or secrets in fixtures.

---

*Level 1 module specification. See [P3.0 ValidatePassword](../procedures/logic/validate_password.md), [P3.1 BuildActivationXML](../procedures/logic/build_activation.md), [P3.2 BuildNetworkXML](../procedures/logic/build_network.md), and [P3.5 CalculateHash](../procedures/logic/calculate_hash.md). Challenge extraction and device result parsing are not yet specified as executable operations.*
