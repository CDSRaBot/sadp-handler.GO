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
