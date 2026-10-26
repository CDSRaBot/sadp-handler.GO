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

The selected activation variant is SADP XML with `TargetMAC` identifying the device; it is not ISAPI HTTP activation. P3.1 defines the request shape, with its Python-derived optional fields retained as compatibility requirements pending end-to-end activation outcome testing. Activation passwords are restricted to ASCII letters, ASCII digits, and printable ASCII special characters; spaces and all non-ASCII characters are rejected. For network configuration, the SHA-256 calculation over UTF-8 `password + challenge` is confirmed by practical experiments and is normative. Target MAC text is uppercase, colon-separated EUI-48.

M3 supplies operation semantics to M9. SADP remains the default activation and
network-configuration route; ISAPI is selected explicitly or after an eligible
SADP-unavailable result. Rejection, authentication failure, malformed
responses, cancellation, and verification failure are terminal. Every bulk
target retains its own protocol and typed outcome. ISAPI activation implements
the universal SDK RSA/AES/challenge procedure; only additional undocumented
variants are excluded.

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
    OperationMalformed    DeviceOperationErrorKind = "malformed_response"
    OperationCorrelation  DeviceOperationErrorKind = "response_mismatch"
    OperationTimeout      DeviceOperationErrorKind = "timeout"
    OperationMissingData  DeviceOperationErrorKind = "missing_data"
    OperationCanceled     DeviceOperationErrorKind = "canceled"
)

type DeviceOperationError struct {
    Kind DeviceOperationErrorKind
    Err  error
}

func (e *DeviceOperationError) Error() string
func (e *DeviceOperationError) Unwrap() error

type MutationOperation string

const (
    MutationActivation MutationOperation = "activation"
    MutationNetwork    MutationOperation = "network"
)

type MutationTarget struct {
    MAC    net.HardwareAddr
    Sender net.Addr
}

type Challenge struct {
    Value     string
    TargetMAC net.HardwareAddr
    Sender    net.Addr
    ExpiresAt time.Time
}

type ChallengeProvider interface {
    Acquire(context.Context, MutationTarget) (Challenge, error)
}

type MutationDecisionStatus string

const (
    MutationAccepted MutationDecisionStatus = "accepted"
    MutationRejected MutationDecisionStatus = "rejected"
)

type MutationDecision struct {
    Operation MutationOperation
    Target    MutationTarget
    Status    MutationDecisionStatus
    Reason    string
}

type ResponseParser interface {
    Parse(MutationOperation, MutationTarget, net.Addr, []byte) (MutationDecision, error)
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
- For static configuration, `Address` must contain a valid IPv4 address and prefix and `Gateway` must contain a valid IPv4 address. The gateway is mandatory. For DHCP, static address and gateway fields must be unset. Invalid or contradictory combinations are errors.
- `BuildNetworkXML` receives the target device MAC and security inputs separately from network configuration; it requires the challenge-response behavior specified by the related procedures and must not silently fall back to plaintext credentials.
- Operation-level APIs must accept `context.Context` when they perform network or other blocking I/O and return a result or error for each target device.
- Operation APIs return `DeviceOperationError` for caller-branchable failures. `Kind` is stable and callers use `errors.As`; `Unwrap` preserves the cause. Context cancellation must also remain detectable with `errors.Is(err, context.Canceled)`.
- `ChallengeProvider` owns challenge acquisition semantics; it must return a challenge scoped to the requested MAC and sender with a finite expiry. It must never return a challenge for another target, an expired challenge, or a challenge whose source cannot be authenticated by the response-correlation contract.
- `ResponseParser` receives raw bytes only after M1 has preserved the sender. It validates the operation-specific response grammar from approved fixtures, rejects malformed or unexpected responses, and returns either `accepted` or `rejected`; it never turns an unknown response into success.
- `MutationDecision.Reason` is a safe, bounded summary and must not contain raw response bytes, XML, passwords, challenges, hashes, or other secret fields. A rejected decision is a device outcome, not a parser success.

## 4. Security and Error Policy

- Do not log passwords, derived secrets, challenge values, or complete activation payloads.
- Keep credential scope and lifetime limited to the operation. Go strings cannot be reliably erased from memory; do not promise secure zeroization.
- Use P3.5's confirmed challenge-response calculation; do not substitute plaintext credentials if the challenge is missing.
- Return errors with operation and device context while avoiding disclosure of credentials. Preserve wrapped causes for callers.
- Do not use `panic` for invalid requests, device rejections, or transport failures.
- Do not copy the Python reference's missing-challenge fallback, which inserts the plaintext password into the network-configuration XML. A missing challenge must fail closed unless an explicitly named, separately reviewed compatibility mode is required and approved.
- The mutation workflow must acquire a challenge before building a network request, exchange the request through M1, and parse the response before reporting success. A request builder return value alone is never an operation success.

## 5. Testing Requirements

- Test password policy boundaries and invalid network configurations.
- Verify XML element names, casing, escaping, encoding, and optional-field behavior against protocol fixtures.
- Test successful and rejected device responses, timeouts, cancellation, and per-device outcomes in bulk operations.
- Use deterministic test inputs; never include production credentials or secrets in fixtures.
- Test malformed and unexpected responses, expired or target-mismatched challenges, sender/target mismatch, duplicate targets, partial bulk failure, and redaction of all security material.

---

*Level 1 module specification. See [P3.0 ValidatePassword](../procedures/logic/validate_password.md), [P3.1 BuildActivationXML](../procedures/logic/build_activation.md), [P3.2 BuildNetworkXML](../procedures/logic/build_network.md), [P3.5 CalculateHash](../procedures/logic/calculate_hash.md), [P3.6 AcquireChallenge](../procedures/logic/acquire_challenge.md), [P3.7 ParseMutationResponse](../procedures/logic/parse_mutation_response.md), and the [mutation fixture catalog](../procedures/logic/mutation_fixtures.md).*
