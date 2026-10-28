# Procedure Specification: P3.7 - ParseMutationResponse
## Module: M3 - Device Logic

## 1. Purpose

Interpret one activation or network mutation response after M1 has acquired
the raw bytes and sender metadata. Return an explicit accepted/rejected
decision or a typed failure.

This procedure defines the semantic response contract. Wire-level element
names, status values, and required fields are normative only when present in
the approved operation fixtures. Unknown or undocumented values never imply
success.

## 1.1 SDK-backed ISAPI grammar

The Hikvision Device Network SDK Information Release guide defines an ISAPI
terminal SADP-management surface distinct from the raw SADP UDP mutation
protocol:

| Operation | Method and URI | Success response | Failure response |
| --- | --- | --- | --- |
| Read terminal SADP capability | `GET /ISAPI/Publish/TerminalMgr/terminals/discovery/capabilities` | `XML_Cap_DiscoveryMode` | `XML_ResponseStatus` |
| Read terminal SADP parameters | `GET /ISAPI/Publish/TerminalMgr/terminals/discovery/<ID>` | `XML_DiscoveryMode` | `XML_ResponseStatus` |
| Set terminal SADP parameters | `PUT /ISAPI/Publish/TerminalMgr/terminals/discovery` | `XML_ResponseStatus` | `XML_ResponseStatus` |

The corresponding request and response schemas are:

- `DiscoveryMode` with namespace
  `http://www.isapi.org/ver20/XMLSchema` and version `2.0`;
- `DiscoveryModeCfg` with the same namespace and version;
- `ResponseStatus` with `requestURL`, `statusCode`, `statusString`,
  `subStatusCode`, optional `MErrCode`, and optional `MErrDevSelfEx`.

The documented discovery configuration fields are `ONVIFDiscoveryMode`,
`ISDiscoveryMode`, `PcapMode`, and `SADP`, with values
`discoverable` or `nonDiscoverable`. `DiscoveryModeCfg` additionally carries
the operation selector and terminal/organization lists.

The functional error table documents these activation-related mappings:

| `MErrCode` | `subStatusCode` | Meaning |
| --- | --- | --- |
| `0x01400004` | `notActivated` | Device is not activated |
| `0x01400005` | `hasActivated` | Device has been activated |
| `0x01400006` | `forbiddenIP` | IP address is banned |
| `0x01400007` | `bondMacAddressNotMatch` | MAC address does not match the user |

These mappings are normative for the documented ISAPI/Text Protocol response
grammar. They must not be used as evidence that the same fields or codes are
present in a raw SADP UDP response.

**Source:** Hikvision *Device Network SDK (Information Release) Developer
Guide V6.1.7.X*, topics `/ISAPI/Publish/TerminalMgr/terminals/discovery`,
`XML_DiscoveryMode`, `XML_DiscoveryModeCfg`, `XML_ResponseStatus`, and
*Error Codes Categorized by Functional Modules*.

## 2. Go Contract

```go
type MutationOperation string

const (
    MutationActivation MutationOperation = "activation"
    MutationNetwork    MutationOperation = "network"
)

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
```

## 3. Semantic Contract

- Validate the operation, target, sender, and response size before parsing.
- Parse only the grammar approved for the selected operation. Do not accept
  activation fields in a network response or vice versa.
- Verify that the response identifies the requested target and that the sender
  matches the target correlation rule.
- Return `MutationAccepted` only for an explicitly documented success status
  with all required fields present and valid.
- Return `MutationRejected` for an explicitly documented device rejection.
- Return an error for unknown status, missing required fields, malformed bytes,
  duplicate conflicting status fields, unexpected operation, or correlation
  mismatch.
- Preserve the target in every decision. `Reason` is a bounded safe summary,
  not a copy of the response.

## 4. Failure Contract

Return `DeviceOperationError` with stable kinds:

- `invalid_input`;
- `malformed_response`;
- `response_mismatch`;
- `device_rejected` only when the response is syntactically valid and carries
  the documented rejection status;
- `missing_data` when required semantic fields are absent;
- `timeout` or `canceled` only when the parser is part of a context-aware
  workflow and that workflow reports the transport cause.

The parser is a pure transformation and does not log, perform network I/O,
retry, or access global state. It never returns a successful zero-value
decision.

## 5. Bulk and Security Rules

The owning workflow invokes the parser independently for each requested target.
It preserves one `DeviceOutcome` per input target and keeps input order. A
parser failure for one target must not erase already attributable outcomes for
other targets. The workflow may aggregate failures only after recording the
per-device result.

Raw response bytes, XML, passwords, challenges, hashes, and complete security
fields are excluded from errors, logs, CLI output, TUI output, and test
failure messages.

## 6. Verification Requirements

Tests derived from this procedure cover accepted activation and network
responses, explicit rejection, malformed and unexpected responses, missing
fields, conflicting fields, sender/target mismatch, duplicate target
handling, partial bulk outcomes, and secret/payload redaction. Every fixture
uses synthetic secrets and identifies the expected semantic decision.

---

*Level 2 procedure specification.*
