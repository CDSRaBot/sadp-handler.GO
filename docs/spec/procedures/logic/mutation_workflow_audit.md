# Mutation workflow contract audit

## Status

Audit completed for Issue #36. Production activation and network mutation
services remain disabled until the device response contract is specified and
covered by deterministic fixtures.

## Existing contract chain

1. M4 parses an `ActivateCommand` or `NetworkCommand` and resolves target
   devices.
2. M3 validates inputs and builds one XML request payload:
   - `BuildActivationXML`;
   - `BuildNetworkXML`.
3. M1 `SendSADPDatagram` sends exactly one payload and does not wait for or
   interpret a response.
4. M4 currently receives results only through injected `ActivationService` and
   `NetworkService` seams.

The chain is intentionally incomplete for real mutation workflows: steps 3
and 4 have no specified response acquisition or correlation boundary.

## Findings

### Activation

- Request XML shape is specified by `build_activation.md`.
- No executable contract specifies activation success, rejection, malformed
  response, timeout, or response-to-target correlation.
- No parser fixture defines the response fields or accepted status values.
- Enabling a production service would require guessing device behavior.

### Network configuration

- `BuildNetworkXML` requires a non-empty challenge and correctly fails closed.
- No executable contract specifies how a challenge is requested, extracted,
  scoped to a target MAC, or expired.
- No parser fixture defines network-update success, rejection, or malformed
  response semantics.
- Plaintext password fallback is explicitly forbidden.

### Transport and lifecycle

- M1 can send a management datagram, but the send API has no response wait or
  correlation semantics.
- Any workflow implementation must use `context.Context`, bounded deadlines,
  explicit socket ownership, and no unbounded retry.
- Passwords, challenges, hashes, XML, and packet payloads must not be logged or
  returned in user-facing errors.

## Required next contract

Before production workflow code is enabled, define and test:

```text
ChallengeProvider:
  acquire(ctx, target) -> challenge or typed failure

MutationTransport:
  exchange(ctx, target, payload) -> raw response + sender metadata

ResponseParser:
  parse(operation, target, sender, response) -> accepted/rejected outcome
```

The final Go signatures must be chosen after the response fixtures are
available. The implementation must return one `DeviceOutcome` per requested
target, preserve partial results, and map failures to
`DeviceOperationError`.

## Acceptance gate

The mutation workflow task is not complete until deterministic tests cover:

- successful activation and network update;
- rejected-device responses;
- malformed and unexpected responses;
- missing challenge;
- timeout and context cancellation;
- sender/target mismatch;
- bulk partial outcomes;
- secret and payload redaction.

