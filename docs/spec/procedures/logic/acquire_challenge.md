# Procedure Specification: P3.6 - AcquireChallenge
## Module: M3 - Device Logic

## 1. Purpose

Acquire a challenge for one mutation target and return a finite, target-scoped
value that can be used by a subsequent network-configuration request.

Challenge acquisition is a workflow boundary, not a hash helper. The exact
wire fields and accepted encodings must come from approved device fixtures.
This procedure must not infer a field name, status value, or success response
from an undocumented capture.

## 2. Go Contract

```go
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
```

- Reject a nil context and invalid or incomplete target before transport use.
- Acquire through the injected M1 mutation transport; do not open sockets or
  create global clients inside the procedure.
- Validate the response using the approved challenge fixture grammar.
- Require a non-empty challenge, the requested target MAC, the correlated
  sender, and an expiry strictly after acquisition time.
- Never return an expired, unscoped, ambiguous, or target-mismatched challenge.
- Do not expose raw response data in the returned error or challenge summary.

## 3. Failure Contract

Return `DeviceOperationError` with these stable kinds:

- `invalid_input` for an invalid target;
- `transport` for exchange failure not otherwise classified;
- `timeout` for deadline expiry;
- `canceled` when the context is canceled;
- `malformed_response` for bytes that do not match the approved fixture
  grammar;
- `missing_data` for an accepted response without a usable challenge;
- `response_mismatch` for sender or target mismatch;
- `device_rejected` when the device explicitly rejects challenge acquisition.

Preserve the underlying cause with `Unwrap`. Error messages contain only the
operation, target identity, and safe failure category; never the challenge,
password, hash, XML, or raw response.

## 4. Processing Requirements

1. Validate the target and context.
2. Request one challenge through M1 P1.6.
3. Parse the response according to the approved fixture grammar.
4. Verify sender and target correlation.
5. Verify non-empty value and finite expiry.
6. Return the challenge only to the owning workflow.

Challenge values are operation-scoped and must not be cached globally or
reused for another target. A failed or canceled acquisition returns no usable
challenge.

## 5. Verification Requirements

Tests derived from this procedure cover success, explicit rejection, missing
challenge, malformed response, expired challenge, sender mismatch, target
mismatch, timeout, cancellation, duplicate response, and redaction. Fixtures
contain synthetic values only.

---

*Level 2 procedure specification.*
