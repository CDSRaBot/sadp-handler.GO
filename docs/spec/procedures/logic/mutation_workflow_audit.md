# Mutation workflow contract audit

## Status

Audit completed for Issue #36. Production activation and network mutation services remain disabled until the
device response contract is specified and covered by deterministic fixtures.
Issue #40 defines the semantic contracts; Issue #45 must derive and approve
the executable fixtures before production implementation starts.

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

## Contract update

The missing workflow boundaries are now specified by:

- [P1.6 MutationTransport.Exchange](../../network/exchange_mutation.md);
- [P3.6 AcquireChallenge](./acquire_challenge.md);
- [P3.7 ParseMutationResponse](./parse_mutation_response.md);
- [mutation fixture catalog](./mutation_fixtures.md).

The module specifications [M1](../../modules/network.md) and
[M3](../../modules/logic.md), and the M4 dispatch implementation gate, now
reference these boundaries. The contracts intentionally define semantic
outcomes and correlation rules without inventing undocumented wire fields.
Approved fixtures must establish the operation-specific response grammar.

## Required test-first work

Before production workflow code is enabled, Issue #45 must define and test:

```text
ChallengeProvider:
  acquire(ctx, target) -> challenge or typed failure

MutationTransport:
  exchange(ctx, target, payload) -> raw response + sender metadata

ResponseParser:
  parse(operation, target, sender, response) -> accepted/rejected outcome
```

The final Go signatures above are normative seams. Operation-specific wire
field mappings remain fixture-driven. The implementation must return one
`DeviceOutcome` per requested target, preserve partial results, and map
failures to `DeviceOperationError`.

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
