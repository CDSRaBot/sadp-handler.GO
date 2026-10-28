# Mutation protocol fixture catalog
## Module: M3 - Device Logic

This catalog defines the semantic cases that Issue #45 must encode as
deterministic test fixtures. It is normative for expected outcomes. It
contains separate fixture tracks for SDK-documented ISAPI terminal
SADP-management and raw SADP UDP mutation. A fixture may use a captured
device response only after its source, target correlation, and field mapping
are approved.

The Hikvision SDK confirms the following ISAPI track:

- `GET /ISAPI/Publish/TerminalMgr/terminals/discovery/capabilities`;
- `GET /ISAPI/Publish/TerminalMgr/terminals/discovery/<ID>`;
- `PUT /ISAPI/Publish/TerminalMgr/terminals/discovery`;
- `DiscoveryMode` and `DiscoveryModeCfg` roots in
  `http://www.isapi.org/ver20/XMLSchema`;
- `XML_ResponseStatus` for failures and for the `PUT` operation;
- functional codes `0x01400004` through `0x01400007` listed in
  `parse_mutation_response.md`.

These facts approve ISAPI fixture grammar only. They do not approve raw SADP
UDP `TargetMAC`, `SecurityResponse`, challenge, sender-correlation, or
mutation-response fields.

All values in test data are synthetic. Fixture names are stable identifiers;
they are not wire-level protocol values.

## 1. Common fixture fields

Each fixture records:

- protocol track: `isapi_terminal_sadp` or `sadp_udp`;
- operation: `activation` or `network`;
- requested target MAC and expected sender;
- request payload bytes or a builder input;
- response bytes and actual sender;
- expected decision or typed failure;
- expected per-device outcome;
- values that must be absent from logs, errors, and rendered output.

The fixture harness must preserve response bytes as untrusted input and must
not normalize them before the parser receives them.

## 2. Required fixture matrix

| Fixture ID | Operation | Input/response condition | Expected result |
| --- | --- | --- | --- |
| `activation-accepted` | activation | Approved success response for the requested MAC and sender | `accepted`, one successful device outcome |
| `activation-rejected` | activation | Approved explicit device rejection | `device_rejected`, one failed device outcome |
| `activation-malformed` | activation | Truncated, invalid, or structurally unexpected response | `malformed_response`, no success |
| `activation-mismatch` | activation | Valid response from a different sender or target | `response_mismatch`, no success |
| `network-challenge-accepted` | network | Approved challenge response scoped to the requested target | finite target-scoped challenge |
| `network-challenge-missing` | network | Response has no usable challenge | `missing_data`, no request payload |
| `network-challenge-expired` | network | Challenge expiry is not after acquisition time | `missing_data`, no request payload |
| `network-accepted` | network | Approved success response for the requested MAC and sender | `accepted`, one successful device outcome |
| `network-rejected` | network | Approved explicit device rejection | `device_rejected`, one failed device outcome |
| `network-malformed` | network | Truncated, invalid, or structurally unexpected response | `malformed_response`, no success |
| `network-mismatch` | network | Valid response from a different sender or target | `response_mismatch`, no success |
| `transport-timeout` | either | Exchange exceeds the configured deadline | `timeout`, context cause preserved |
| `transport-canceled` | either | Context is canceled during exchange | `canceled`, context cause preserved |
| `bulk-partial` | either | One target succeeds while another has a typed failure | one outcome per input target, failure preserved |
| `duplicate-target` | either | The same normalized target is supplied more than once | explicit validation failure before secret input |
| `secret-redaction` | either | Synthetic password, challenge, hash, XML, and payload canary values flow through failures | none of the canaries appear in errors, logs, CLI, or TUI |

## 2.1 SDK-backed ISAPI terminal SADP fixtures

| Fixture ID | Request | Response grammar | Expected result |
| --- | --- | --- | --- |
| `isapi-sadp-capabilities-accepted` | `GET .../discovery/capabilities` | `XML_Cap_DiscoveryMode` | parsed capability |
| `isapi-sadp-read-accepted` | `GET .../discovery/<ID>` | `DiscoveryMode` | parsed SADP discovery configuration |
| `isapi-sadp-write-accepted` | `PUT .../discovery` | `ResponseStatus` with documented OK status | `accepted` |
| `isapi-sadp-not-activated` | any supported request | `MErrCode=0x01400004`, `subStatusCode=notActivated` | terminal device state error |
| `isapi-sadp-has-activated` | activation/state operation | `MErrCode=0x01400005`, `subStatusCode=hasActivated` | terminal already-active error |
| `isapi-sadp-forbidden-ip` | any supported request | `MErrCode=0x01400006`, `subStatusCode=forbiddenIP` | terminal authentication/access error |
| `isapi-sadp-mac-mismatch` | any supported request | `MErrCode=0x01400007`, `subStatusCode=bondMacAddressNotMatch` | terminal correlation/authentication error |

The exact `statusCode`/`statusString` success pair must be taken from the
response fixture for the endpoint. The fixture must not infer success from an
HTTP status alone.

## 3. Fixture approval rules

- A success fixture must identify the exact fields that prove acceptance.
- A rejection fixture must identify the exact fields that prove rejection.
- A malformed fixture must state which grammar rule is violated without
  relying on an error string.
- A correlation fixture must distinguish target identity from sender address.
- A challenge fixture must state acquisition time, expiry, target MAC, and
  sender metadata.
- A fixture cannot be marked approved when its expected result depends on an
  undocumented response field or guessed status value.
- An ISAPI fixture must record its URI, HTTP method, XML root, namespace,
  response-status fields, and SDK source topic.
- An `isapi_terminal_sadp` fixture must not be reused as a `sadp_udp` fixture.
- A `sadp_udp` fixture remains unapproved until the raw datagram grammar,
  target identity, sender correlation, and response status are sourced from a
  SADP protocol specification or an approved device capture.

## 4. Test ordering

Issue #45 derives red tests from this catalog. Issues #41 and #42 may add
implementation-specific fixtures only when they refine, rather than weaken,
these semantic cases and update the owning specification first.

---

*Specification artifact for the M4 mutation workflow contract. No production
implementation is defined by this catalog.*
