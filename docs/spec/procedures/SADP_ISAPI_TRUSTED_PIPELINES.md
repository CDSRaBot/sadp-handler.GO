# Trusted SADP and ISAPI Procedure Pipelines

**Status:** Trusted normative reference  
**Scope:** Device discovery, activation, ISAPI authentication, verification, and
network configuration for Hikvision-compatible devices  
**Audience:** Protocol, logic, and integration implementers

## 1. Authority and trust status

This document is the trusted procedural baseline for the system. It is an
implementation-neutral synthesis of:

- the vendor ISAPI and SADP protocol model;
- publicly documented HTTP Digest behavior;
- repeated practical observations with real devices and firmware variants;
- independent field implementations that use the same observable protocol
  boundaries.

The document defines workflows and decision gates, not source code. It does
not copy implementation text from any external project. An individual device
or firmware may expose a narrower endpoint set or require a vendor-specific
activation variant; such a response is a compatibility result, not permission
to silently change the workflow.

## 2. Protocol separation

SADP and ISAPI are separate protocol layers:

| Layer | Purpose | Transport | Authentication |
|---|---|---|---|
| SADP | Local discovery and device-management commands | UDP multicast/broadcast, normally port 37020 | Protocol command semantics; not HTTP Digest |
| ISAPI | Per-device control and status | HTTP or HTTPS | Usually HTTP Digest; Basic may exist on legacy/configured devices |

The `Activated` state reported by SADP is the primary pre-activation signal.
ISAPI access to an inactive device is not assumed to be available.

## 3. Shared invariants

Every pipeline MUST:

1. operate only on an explicitly selected local interface or explicitly
   supplied device address;
2. bound datagram size, XML size, request count, concurrency, retries, and
   timeouts;
3. validate all network addresses, identifiers, XML fields, and credentials
   before transmission;
4. preserve the target identity by MAC address whenever SADP provides it;
5. correlate a response only with the intended device and operation;
6. treat malformed, incomplete, or contradictory responses as explicit errors;
7. avoid logging passwords, challenges, nonces, Digest responses, or complete
   credential-bearing payloads;
8. return a typed outcome for every target in a bulk operation;
9. make cancellation close sockets, stop listeners, and terminate pending
   retries;
10. never report activation or configuration success until a verification gate
    has passed.

## 4. Pipeline A — SADP discovery

**Goal:** Build a validated inventory of visible devices on the selected local
network.

### Inputs

- selected network interface;
- discovery context and deadline;
- bounded receive buffer;
- optional target MAC or address filter.

### Stages

| Stage | Action | Required output | Failure gate |
|---|---|---|---|
| A1 | Enumerate and validate the interface | Usable interface address and hardware identity | Stop if the interface cannot send and receive local multicast/broadcast traffic |
| A2 | Create the SADP socket | Socket bound to the selected interface with explicit cleanup ownership | Stop and return transport error |
| A3 | Build the inquiry request | Valid request identifier and inquiry message | Stop if required fields cannot be generated |
| A4 | Send the request to the SADP multicast/broadcast destinations | Transmission result | Record send failure; do not claim discovery success |
| A5 | Receive datagrams until deadline or cancellation | Bounded raw responses | Finish with partial results only if the caller explicitly permits partial inventory |
| A6 | Validate and parse each response | Typed device candidate | Reject malformed, oversized, or identity-incomplete responses |
| A7 | Normalize and merge candidates | One record per device identity, preferably MAC | Resolve duplicate observations deterministically |
| A8 | Close socket and publish inventory | Complete inventory plus rejected-response diagnostics | Always release socket and listener resources |

### Required device fields

The parser should retain, when present:

- MAC address;
- IPv4 and IPv6 addresses;
- device serial number;
- model or description;
- firmware version;
- HTTP/HTTPS ports;
- `Activated` state;
- source and receive interface.

Unknown optional fields are preserved only when bounded and safe. A missing
required identity field prevents the response from becoming an actionable
device record.

### SADP request and response markers

SADP is not an HTTP API. The discovery request is a UDP XML inquiry sent to
the selected local multicast/broadcast transport, normally on port `37020`.
The request MUST carry a fresh correlation identifier. A response is
actionable only when it contains a matching or otherwise safely correlated
inquiry result and a stable device identity.

The discovery parser MUST recognize the following response markers when
present:

| XML field | Procedure use |
|---|---|
| `MAC` | Primary identity and response-correlation key |
| `IPv4Address` | Candidate management address; not an identity |
| `DeviceSN` | Secondary identity evidence |
| `DeviceDescription` | Model/description for compatibility selection |
| `Activated` | Pre-activation state gate |
| `HttpPort` / `HttpPortNo` | Candidate ISAPI port where supplied |

## 5. Pipeline B — SADP activation

**Goal:** Set the initial administrative password on an inactive device.

### Inputs

- discovery record;
- target MAC address;
- candidate device address if required by the transport;
- new password supplied through a secret-safe input path;
- activation context and deadline.

### Stages

| Stage | Action | Required output | Failure gate |
|---|---|---|---|
| B1 | Confirm target identity and inactive state | Stable target identity and `Activated=false` or equivalent | Stop on identity ambiguity or already-active state unless the caller requested revalidation |
| B2 | Validate password policy locally | Validated password metadata without retaining or logging the secret | Stop before network transmission if policy fails |
| B3 | Obtain any required device challenge/capability data | Challenge or capability result, if the model requires it | Stop with a classified compatibility error when the required capability is unavailable |
| B4 | Construct the activation operation | Schema-valid activation request bound to the target MAC/device | Never substitute a different target after request construction |
| B5 | Send the SADP activation command or supported ISAPI activation request | Device response | Classify timeout, rejection, unsupported variant, and transport failure separately |
| B6 | Parse the response | Explicit success or typed failure | Do not infer success from a transport-level response alone |
| B7 | Re-discover or query the device | Updated `Activated=true` state and stable identity | Treat missing or contradictory state as verification failure |
| B8 | Publish the result | Activation result with no secret material | Preserve the failure reason and target identity |

### Activation variants

The implementation may select only a documented variant supported by the
device:

1. SADP activation command using the target MAC and device challenge;
2. plain XML activation through the device's ISAPI activation endpoint;
3. challenge/RSA/encrypted activation required by a particular firmware;
4. vendor web UI or official tool handoff when the device does not expose a
   supported programmatic variant.

A plain XML attempt MUST NOT be retried indefinitely when the device returns a
security-capability or encrypted-activation indication. The result must state
that a different activation variant is required.

### ISAPI activation request shape

For devices that expose the plain ISAPI activation variant, the operation is:

| Property | Required value |
|---|---|
| Method | `PUT` |
| Path | `/ISAPI/System/activate` |
| Content type | `application/xml` |
| Authentication | No existing HTTP account is assumed for initial activation |
| Success condition | A successful device response followed by Pipeline D verification |

The body is an `ActivateInfo` XML document containing only the new password.
The exact namespace and version MUST match the device capability or the
vendor's model-specific schema. The following is pseudocode, not a byte-for-
byte universal payload:

```text
PUT /ISAPI/System/activate
Content-Type: application/xml

<ActivateInfo [device-required version and namespace]>
  <password>{new-secret}</password>
</ActivateInfo>
```

An inactive device has no established administrative account; therefore the
first plain activation request MUST NOT require invented Digest credentials.
If the endpoint replies with `400`, `401`, `403`, a security-capability
response, or an encryption/challenge requirement, the client MUST stop the
plain variant and classify the model as requiring a supported alternate
activation flow. A firmware-specific challenge endpoint, when advertised by
the device, is an input to that alternate flow and not a substitute for
target identity verification.

## 6. Pipeline C — ISAPI authentication

**Goal:** Execute one authenticated request against an already activated
device.

### Inputs

- validated HTTP(S) endpoint;
- username and password from a secret-safe source;
- HTTP method, path, bounded body, and request context;
- TLS policy, including explicit handling of self-signed certificates.

### Stages

| Stage | Action | Required output | Failure gate |
|---|---|---|---|
| C1 | Validate endpoint and request | Canonical scheme, host, port, path, method, and bounded body | Stop on invalid or ambiguous endpoint |
| C2 | Send the initial request without credentials | HTTP response and authentication challenge, if any | Return non-authentication response directly when valid |
| C3 | Parse `WWW-Authenticate` | Digest realm, nonce, algorithm, qop, opaque, and supported parameters | Stop on missing, malformed, or unsupported challenge |
| C4 | Compute the Digest proof | Authorization metadata for this exact method and URI | Never log or persist the proof, nonce, or password-derived values |
| C5 | Retry once with the Digest Authorization header | Authenticated HTTP response | Do not loop on repeated `401` responses |
| C6 | Parse HTTP and ISAPI response status | Typed success or typed device/application error | Preserve status and sub-status classification |
| C7 | Close response body and release resources | Completed request outcome | Always close body and honor cancellation |

### Authentication rules

- Digest is the default and preferred mechanism.
- Basic authentication is permitted only when explicitly enabled and
  supported by the device policy.
- A `401` must distinguish bad credentials, locked account, missing user,
  unsupported authentication, and expired/invalid challenge when the response
  provides those details.
- Failed authentication must not trigger unbounded retries.
- Clock synchronization should be checked when Digest failures are
  intermittent or device-side timestamps indicate drift.

### Authentication probe and Digest exchange

The default authenticated probe is:

| Property | Required value |
|---|---|
| Method | `GET` |
| Path | `/ISAPI/System/deviceInfo` |
| Initial request | No `Authorization` header |
| Expected challenge | `401 Unauthorized` with `WWW-Authenticate: Digest ...` |
| Retried request | Same method and URI with one `Authorization: Digest ...` header |
| Verified response | `2xx` and a parseable `DeviceInfo` document |

The implementation MUST bind the Digest proof to the exact HTTP method and
request URI. It MUST preserve a replayable bounded request body for a
state-changing request, so that the single authenticated retry is identical
to the challenge request.

```text
response = HTTP(GET, "/ISAPI/System/deviceInfo", no_authorization)
if response.status != 401:
    return classify_unexpected_or_unauthenticated_response(response)

challenge = parse_digest(response.headers["WWW-Authenticate"])
require challenge.realm, challenge.nonce, supported_qop(challenge.qop)

proof = digest(username, password, "GET", "/ISAPI/System/deviceInfo", challenge)
response = HTTP(GET, "/ISAPI/System/deviceInfo",
                Authorization = digest_header(proof))
return classify_http_and_isapi_response(response)
```

The following paths may be used only for the stated purpose and only after
the same authenticated transport has been established:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/ISAPI/System/deviceInfo` | Identity, model, serial, firmware, and post-change reachability verification |
| `GET` | `/ISAPI/System/status` | Runtime system status where implemented |
| `GET` | `/ISAPI/System/capabilities` | Optional system capability discovery where implemented |

## 7. Pipeline D — Activation and credential verification

**Goal:** Prove that activation produced a usable administrative account.

### Stages

| Stage | Action | Acceptance condition |
|---|---|---|
| D1 | Re-run SADP discovery or query the device state | Device identity matches the activation target |
| D2 | Confirm activation state | `Activated=true` or equivalent positive device state |
| D3 | Request basic device information through ISAPI | Endpoint responds with a valid device information document |
| D4 | Authenticate using the newly established credentials | Digest exchange completes successfully |
| D5 | Classify the result | Verified, active-but-authentication-failed, unreachable, or incompatible |

Activation is not verified by HTTP `200` alone. The verified result requires
both positive device state and a successful authenticated information request.

### Verification request sequence

```text
sadp = discover_by_target_mac(target_mac)
require sadp.activated == true

device = digest_get("/ISAPI/System/deviceInfo", new_credentials)
require device.http_status is 2xx
require parse_device_info(device.body).identity matches target_mac or target_serial
return verified
```

If `deviceInfo` is unavailable on a supported device, a model-specific
authenticated read endpoint may be used only when it supplies equally strong
identity evidence. A successful TCP connection, an HTTP `200` with an
unrelated body, or a SADP state change alone is not verification.

## 8. Pipeline E — ISAPI network configuration

**Goal:** Apply a validated IPv4/DHCP configuration to an active device.

### Inputs

- verified active device;
- authenticated ISAPI session/transport;
- requested DHCP or static IPv4 configuration;
- explicit gateway and subnet values where static mode is selected.

### Stages

| Stage | Action | Required output | Failure gate |
|---|---|---|---|
| E1 | Validate the requested network plan | Internally consistent address, mask, gateway, and mode | Stop before sending |
| E2 | Read current capabilities/configuration | Supported interface identifier and writable fields | Stop if the interface or mode is unsupported |
| E3 | Submit the configuration | Device response classified by HTTP and ISAPI status | Do not assume the old address remains reachable |
| E4 | Handle reboot/restart indication | Explicit restart-required state | Do not issue duplicate restart commands |
| E5 | Wait for bounded reappearance | Device reachable at the expected address or DHCP-discovery path | Return a recoverable reachability result on timeout |
| E6 | Re-authenticate and verify | Device info and network state match the requested plan | Mark configuration unverified if state differs |

Network changes are disruptive. The result must preserve the old identity,
requested plan, observed new address, and verification state without exposing
credentials.

### Network configuration request sequence

The implementation MUST read the interface collection before it writes a
specific interface. The standard request family is:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/ISAPI/System/Network/interfaces` | Enumerate network interfaces |
| `GET` | `/ISAPI/System/Network/interfaces/{interface-id}` | Read the selected interface and its writable fields |
| `PUT` | `/ISAPI/System/Network/interfaces/{interface-id}` | Apply DHCP or static IPv4 configuration |
| `PUT` | `/ISAPI/System/reboot` | Reboot only when the device explicitly requires it and the endpoint is supported |

The write body MUST preserve required, device-returned fields and change only
the validated address mode and requested address values. The following
pseudocode shows the required decision structure, not a universal XML schema:

```text
interfaces = digest_get("/ISAPI/System/Network/interfaces")
interface = select_explicit_interface(interfaces, interface_id)
current = digest_get("/ISAPI/System/Network/interfaces/{interface_id}")
desired = build_from(current)

if mode == "dhcp":
    desired.addressingType = "dhcp"
else:
    desired.addressingType = "static"
    desired.ipAddress = validated_ipv4
    desired.subnetMask = validated_mask
    desired.defaultGateway = validated_gateway

result = digest_put("/ISAPI/System/Network/interfaces/{interface_id}", desired)
require result.http_status is 2xx and result.isapi_status is success
if result.indicates_reboot_required:
    digest_put("/ISAPI/System/reboot", empty_or_device_required_body)
```

The caller MUST retain the target MAC/serial before the write, because a
changed address cannot be used as proof that the same device reappeared.

## 9. Pipeline F — ISAPI security challenge exchange

**Goal:** Obtain and validate device-specific challenge material required by
a security operation, without assuming its semantics or key-binding rules.

The security challenge and its salt are inputs to some device-specific flows, but
they are not automatically the AES key or AES-key derivation inputs for
sensitive XML nodes. The captured recovery-question UI uses a separate key
derivation from the account credentials and `/ISAPI/Security/capabilities`
salt; its session-login challenge feeds a different login proof. Do not apply
this pipeline to user-management encryption unless that operation explicitly
requires `/ISAPI/Security/challenge`.

This pipeline describes operations that explicitly use the security
challenge, including the captured activation flow. It is separate from the
HTTP Digest challenge in Pipeline C: the two challenge values have different
protocol roles and MUST NOT be substituted for one another.

### Stages

| Stage | Action | Required output | Failure gate |
|---|---|---|---|
| F1 | Confirm that the target operation requires and supports the security challenge procedure | Capability-confirmed operation and supported security parameters | Stop if the device does not advertise a compatible procedure |
| F2 | Send the security-challenge request | Bounded device response | Classify authentication, transport, and device rejection separately |
| F3 | Validate the challenge response | Fresh challenge plus any declared salt, algorithm, key size, and lifetime | Stop on missing, malformed, expired, or unsupported values |
| F4 | Resolve the key source required by this operation | Key material from the operation's documented or capability-confirmed source | If the source is not declared, stop with a classified compatibility error; never infer a KDF from a challenge by guessing a SHA-256, MD5, or HMAC variant |
| F5 | Bind the critical request to the challenge result | Request sent before challenge expiry with its required security parameters | Do not retry an expired or rejected challenge; acquire a new one when the device procedure permits |

### Challenge request

The device-issued challenge is requested with the following shape when this
procedure is advertised:

```text
POST /ISAPI/Security/challenge
# Include authentication only when the device procedure requires it.
```

Illustrative response (placeholders; field set and values are device-specific):

```text
<SecurityChallenge version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <challenge>{device-random-challenge}</challenge>
  <salt>{device-salt}</salt>
  <isSessionValid>true</isSessionValid>
</SecurityChallenge>
```

Flow pseudocode:

```text
c = parse_challenge(response)              # validate fields and lifetime if declared
require valid(c)
operation_key = resolve_operation_key(c)    # only if the operation defines one
send_critical_request(operation_key, challenge_binding = c)
```

`challenge_binding` is whatever the device or SDK procedure requires to tie
the request to the challenge (for example session authorization); it is not
specified by the reviewed material and MUST be taken from the device's
advertised procedure.

The response supplies a unique challenge and may supply a salt and lifetime.
Challenge text, salt, lifetime-bound tokens, AES keys, and password-derived
intermediate values are secret material. They MUST NOT be logged, persisted,
returned in diagnostics, or reused after expiry.

### HAR-observed activation exchange

One captured web activation flow used a different challenge envelope from the
illustrative `SecurityChallenge` XML above:

```text
POST /ISAPI/Security/challenge
<PublicKey><key>{Base64; decodes to 256 ASCII hexadecimal characters}</key></PublicKey>

Response:
<Challenge version="{device-version}">
  <key>{Base64; decodes to 256 ASCII hexadecimal characters}</key>
</Challenge>

PUT /ISAPI/System/activate
<ActivateInfo><password>{Base64; decodes to 64 ASCII hexadecimal characters}</password></ActivateInfo>
```

The new non-activated HAR contains the login UI and startup checks, but no
activation challenge or activation `PUT`. Its cached crypto-related script
bodies are byte-identical to those in the activated loading capture. The
activation exchange itself is present in the earlier activation HAR, and the
cached scripts explain its representation:

- `login.js` calls `utils.getRSABits()`. The helper selects 256 bits only for
  Internet Explorer versions below 9 and 1024 bits otherwise; the captured
  Edge 155 user agent selects 1024 bits.
- `cryptico.publicKeyString` returns the RSA modulus as hexadecimal text;
  Base64 decoding the captured `PublicKey/key` yields 256 ASCII hex characters,
  i.e. a 1024-bit modulus, not 256 raw key bytes or a 2048-bit modulus. The
  decoded `Challenge/key` is also 256 ASCII hex characters, as expected for a
  1024-bit RSA ciphertext in this helper.
- The response is RSA-decrypted locally. For the 1024-bit branch, the
  plaintext is used as a 32-hex-character AES key. The separate `aes_encrypt`
  helper encrypts two 16-character blocks (the first 16 plaintext characters
  and the activation password), concatenates their two 32-character hex
  outputs, and Base64-encodes that 64-character ASCII hex string for
  `ActivateInfo/password`.

This is separate from Pipeline G's AES-CBC/PKCS#7 flow. The block helper
truncates input longer than 16 characters and zero-pads shorter input.
`cryptico.generateRSAKey` seeds `Math.seedrandom` from
`sha256.hex(new Date()+"")`, making this legacy UI key generation
deterministic and time-derived, not cryptographically secure. Document this
only for compatibility analysis; do not reproduce it in a new client.

The exchange does not by itself authenticate the camera or provide
end-to-end request integrity. The captured page and API use HTTP, the client
public key has no evidenced authentication, and the activation ciphertext
contains no evidenced MAC. Do not treat this legacy field encryption as
protection against an active intermediary; see the
[transport and key-source assessment](../reports/ISAPI_ENCRYPTED_USER_FIELDS.md).

Before activation, read-only checks against the currently connected camera
`/SDK/activateStatus` = `Activated=false` and
`/ISAPI/Security/extern/capabilities` =
`isSupportWithSecurityQuestion=true`. The unauthenticated session-login
capabilities request returned `iterations=100` and `isIrreversible=true`;
challenge, salt, and session identifiers were not retained. An unauthenticated
request to `/ISAPI/Security/capabilities?username=admin` returned `403`; this
does not determine its authenticated behavior.

The requested activation was then performed through the camera-served UI with
the supplied credentials. The UI accepted the password and advanced to the
recovery-question configuration dialog. A subsequent read-only
`/SDK/activateStatus` returned `Activated=true`; no recovery questions were
submitted. This validates the observed activation path on this
camera/build, not universal support across firmware versions.

### Device Network SDK stream-encryption key

The Device Network SDK documents a separate stream-encryption key path:

```text
NET_DVR_GetDVRConfig(user, NET_DVR_GET_AES_KEY /* 6113 */,
                    ..., lpOutBuffer /* NET_DVR_AES_KEY_INFO */, ...)
NET_DVR_InquestSetSecretKey(...)
NET_DVR_SetSDKSecretKey(...)
```

This obtains/configures a key for the SDK's stream-encryption functionality.
It MUST NOT be treated as the key source for ISAPI user-management encryption
unless a device-specific ISAPI specification explicitly establishes that
relationship. The command number, output structure, and SDK key-setting calls
do not by themselves define the key, KDF, or authorization binding for
`/ISAPI/Security/users`.

## 10. Pipeline G — encrypted ISAPI user management

**Goal:** Create or update an ISAPI user without transmitting sensitive XML
nodes, such as a password, in plaintext.

### Inputs

- verified active device and authenticated ISAPI transport;
- a capability-confirmed user-management operation and user identifier, where
  the operation updates an existing user;
- sensitive field values from a secret-safe source;
- encryption scheme selected by the device: `security=1` or `security=2`;
- a fresh initialization vector and, for the captured recovery-question
  `security=1` flow, the username, current password, and security-capabilities
  salt and iteration count needed by the browser-observed KDF.

The client MUST establish the authenticated transport through Pipeline C and
MUST use the key derivation documented for the specific operation. For the
captured recovery-question UI, the KDF is defined below and does not use the
session-login challenge. It MUST NOT assume that every device supports
encrypted user management, AES-256, or the same XML user schema.

### Stages

| Stage | Action | Required output | Failure gate |
|---|---|---|---|
| G1 | Discover capability and select the user operation | Supported endpoint, XML schema, sensitive nodes, and security scheme | Stop if the device does not advertise a compatible encrypted-user procedure |
| G2 | Validate operation inputs | Valid user identifier, non-secret XML fields, scheme, key length, and IV length | Stop before any request when values are invalid or incomplete |
| G3 | Derive the operation key and generate/encode request IV | Operation-specific key and fresh 16-byte IV in a device-supported query representation | Stop on missing KDF inputs, entropy, or encoding failure; never reuse an IV with the same key |
| G4 | Transform every sensitive node | Device-required textual ciphertext for each sensitive value | Stop without sending if encoding or encryption fails |
| G5 | Construct bounded XML and query | Schema-valid request with `security` and URL-encoded `iv` | Do not concatenate an unescaped query string or log the body |
| G6 | Send one authenticated mutation | HTTP and ISAPI response | Classify rejection, authentication failure, incompatible capability, and transport failure separately |
| G7 | Verify the intended user state using a safe read or device result | Confirmed operation result without recovering a password | Do not treat HTTP success alone as proof of the requested state |

### Request family and security parameters

Use the collection endpoint to create a user and the identified endpoint to
update one:

| Operation | Method | Path |
|---|---|---|
| Create user | Device-supported `PUT` or `POST` | `/ISAPI/Security/users` |
| Update user | `PUT` | `/ISAPI/Security/users/{id}` |

For either operation, the following query parameters are mandatory when the
selected encrypted-user capability requires them:

| Parameter | Value | Meaning |
|---|---|---|
| `security` | `1` | AES-128-CBC; the secret key is exactly 16 bytes |
| `security` | `2` | AES-256-CBC; the secret key is exactly 32 bytes |
| `iv` | Device-supported encoding of a 16-byte IV (Base64 or hex where observed) | Initialization vector for the request |

The URI builder MUST encode query values. In particular, standard Base64
characters such as `+`, `/`, and `=` in `iv` MUST NOT be concatenated into a
raw URI. The captured question-configuration request uses a 32-character
hexadecimal IV and does not include a `key` query parameter. Do not assume
that every device uses the same IV or ciphertext text encoding.

### Captured recovery-question key derivation

The cached web UI uses `keyIterateNum`, the `isIrreversible` flag, and `salt`
from `/ISAPI/Security/capabilities`; the account password and username are
inputs. For `security=1`, its exact key derivation is:

```text
base = password
if isIrreversible:
    base = SHA256(username || security_capabilities.salt || password)

h = SHA256(base || "AaBbCcDd1234!@#$")
repeat keyIterateNum - 1 times:
    h = SHA256(h)

aes_key = first 32 hex characters of h  # 16 bytes, security=1
```

The recovered key is derived locally; the observed
`questionConfiguration?security=1&iv=...` request transmits only the
`security` and `iv` query parameters. This is observed web UI behavior for the
captured device/build, not a universal ISAPI rule. The key derivation uses the
security-capabilities salt, not the separate session-login challenge or salt.
See the [encrypted-field report](../reports/ISAPI_ENCRYPTED_USER_FIELDS.md)
for the capture evidence and the separate session-login proof.

### Sensitive-node transformation

For the captured question-configuration endpoint, the UI XML-escapes `&`,
`<`, and `>` in each marked sensitive value, UTF-8/Base64-encodes the result,
and uses the resulting Base64 text as AES input:

```text
escaped_value    = XML_escape(sensitive_value)
utf8_bytes       = UTF8(escaped_value)
base64_plaintext = Base64(utf8_bytes)
padded_plaintext = PKCS7Pad(base64_plaintext, 16)
ciphertext       = AES_CBC_Encrypt(aes_key, request_iv, padded_plaintext)
xml_node_value   = Hex(ciphertext)
```

AES-CBC uses PKCS#7 padding with a 16-byte block. The resulting ciphertext
is hex text in the observed endpoint; raw ciphertext bytes MUST NOT be
inserted into XML. Other device/endpoint schemas may require Base64 or another
representation. The same request IV is used for the marked fields in the
captured request; do not generalize that behavior without device support.

The captured UI generates the IV as `MD5(decimal_current_time_in_milliseconds)`.
This timestamp-derived value is predictable, so it is documented only as an
observed compatibility detail. New clients SHOULD use a cryptographically
random 16-byte IV whenever the device accepts one.

### HAR-observed recovery-question configuration

In the captured browser flow, recovery questions were configured after a
separate session-login exchange:

| Order | Method and path | Observed role |
|---|---|---|
| 1 | `GET /ISAPI/Security/sessionLogin/capabilities?username=...` | Returned a session ID, 16-byte hex challenge, 32-byte hex salt, and `iterations=100` |
| 2 | `POST /ISAPI/Security/sessionLogin?timeStamp=...` | Submitted a 64-character hex `password` field and session ID; response indicated success and returned a session ID |
| 3 | `GET /ISAPI/Security/capabilities?username=...` | Advertised security-question support and `keyIterateNum=100` |
| 4 | `PUT /ISAPI/Security/questionConfiguration?security=1&iv={32-hex-characters}` | Submitted three question IDs, three 32-character hex `answer` fields, and a 32-character hex `password` field |

The cached `utils.js` session-login helper defines a separate proof:

```text
if isIrreversible:
    proof = SHA256(username || session_login_salt || password)
    proof = SHA256(proof || session_login_challenge)
    repeat iterations - 2 times: proof = SHA256(proof)
else:
    proof = SHA256(password) || session_login_challenge
    repeat iterations - 1 times: proof = SHA256(proof)
```

The 64-character login field is the hex proof. The session-login challenge and
salt are separate from the security-capabilities salt used in the AES KDF.
The capture and cached UI code together establish the observed recovery-
question key derivation and wire encoding; they do not establish universal
support on other devices or firmware.

The captured response was an ISAPI success response. This confirms the shape
of that exchange only; it does not prove that the same endpoint or encoding is
supported on other devices/firmware. Never retain the captured credentials,
answers, challenge, salt, session IDs, IV, or encrypted values in reports or
fixtures.

This sequence is pseudocode, not a universal XML payload:

```text
PUT /ISAPI/Security/users/{id}?security=1&iv={url_encoded_base64_iv}
Content-Type: application/xml
Authorization: Digest ...

<User [device-required namespace and version]>
  <id>{validated-id}</id>
  <userName>{validated-non-secret-name}</userName>
  <password>{xml_node_value}</password>
</User>
```

Variant using the collection endpoint and a `UserList` body, where the
device schema requires it:

```text
{device-supported-method} /ISAPI/Security/users?security=2&iv={url_encoded_base64_iv}
Content-Type: application/xml

<UserList version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <User>
    <userName>{validated-name}</userName>
    <password>{xml_node_value}</password>
  </User>
</UserList>
```

The HTTP method for creating a user MUST follow the device capability; do not
assume it.

Keys, IV bytes, IV Base64 values, Base64 plaintext, ciphertext, complete XML
bodies, Digest proofs, and passwords are secret material. They MUST NOT be
logged, returned in diagnostics, persisted, or included in test fixtures.

## 11. Error classification

All pipelines should expose at least these classes:

- invalid input;
- interface or socket failure;
- timeout or cancellation;
- malformed protocol data;
- target identity mismatch;
- authentication rejected;
- account locked;
- unsupported activation/authentication variant;
- device rejected request;
- device unreachable after address change;
- verification failed;
- partial bulk-operation result.

The classification is part of the public procedure contract. Human-readable
messages may be localized, but the machine-readable class must remain stable.

## 12. Trusted acceptance criteria

An implementation conforms to this document only when it can demonstrate:

1. discovery on a selected interface with bounded lifecycle;
2. correct parsing and identity-preserving merge of SADP responses;
3. activation result verification after the activation operation;
4. one-challenge/one-retry Digest behavior without authentication loops;
5. explicit handling of unsupported activation variants;
6. secret redaction in errors, logs, and returned diagnostics;
7. cancellation and timeout cleanup;
8. per-device outcomes for bulk operations;
9. post-change verification for network configuration.
10. encrypted user-management requests that select the correct AES key size,
    URL-encode the IV, encrypt Base64 UTF-8 sensitive values with CBC and
    PKCS#7 padding, Base64-encode the ciphertext for XML, and redact all
    secret material;
11. security-challenge requests that validate freshness, never reuse an
    expired challenge, and never derive an AES key from the challenge without
    a device-declared procedure.

Live-device tests may extend coverage, but they must not replace deterministic
tests for parsing, validation, challenge handling, error classification, and
resource ownership.
