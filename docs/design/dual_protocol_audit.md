# Dual SADP/ISAPI Recursive Audit

**Phase:** 3  
**Baseline:** `26fa29b` and `a351ba5`  
**Result:** specification is internally consistent and ready for Phase 4
contract tests. Runtime implementation remains intentionally out of scope.

## 1. Audit matrix

| Chain or concern | Contract checked | Result | Phase 4 evidence |
|---|---|---|---|
| Discovery -> target | M0/M1/M2/M6 preserve interface, sender, MAC and bounded payload | PASS | synthetic discovery target and identity mismatch |
| Target -> route | M9 receives operation, strategy and target | PASS | SADP-first and ISAPI-only selector tests |
| SADP -> fallback | only classified unavailable results can enter eligibility check | PASS | table-driven reason matrix |
| Route -> ISAPI | existing route and operation capability are required | PASS | route-provider and capability-provider fakes |
| Operation -> verification | activation/network success requires status verification | PASS | accepted transport plus failed verification |
| Per-device outcomes | selection is per target and per operation | PASS | mixed bulk results with independent failures |
| Rejection/authentication | terminal and never fallback | PASS | rejection and HTTP auth-failure assertions |
| Cancellation/deadline | context reaches transports and stops retries/cleanup | PASS | cancellation cause, deadline and close assertions |
| Resource ownership | M0/M1 own SADP sockets; ISAPI owns client/body | PASS | close-on-success, error and cancellation |
| Payload limits | SADP datagram and HTTP response limits are bounded | PASS | oversized and truncated synthetic responses |
| Serialization | existing SADP XML remains; ISAPI XML is endpoint/capability-gated | PASS | approved fixtures only |
| Configuration/CLI | M7 -> M8 -> M4 precedence; default is SADP-first | PASS | file/default/CLI precedence tests |
| Diagnostics | secrets, payloads and auth headers are redacted | PASS | negative redaction assertions |
| OS seams | platform behavior remains in M0 and injected route provider | DEFERRED | Windows/Linux/Termux/macOS acceptance matrix |

## 2. State-machine audit

The only automatic transition is:

`SADP attempt -> SADP unavailable -> existing route -> ISAPI capability ->
ISAPI operation -> verification`

`SADP rejection`, `SADP authentication/credential failure`, malformed or
uncorrelated data, cancellation, deadline, route failure, capability failure,
ISAPI authentication failure, and verification failure terminate the target
operation. Explicit `isapi_only` starts at route/capability validation and
never enters the SADP state.

No transition changes an address, route table, interface state, firewall, or
network namespace.

## 3. Deferred and non-universal protocol claims

- ISAPI discovery requires a known address; it is not an implicit inventory
  replacement for SADP.
- The HTTP scheme, port, Digest/Basic behavior, endpoint availability, and
  activation variant are capability and device-profile data, not universal
  assumptions.
- Challenge/RSA/AES/encrypted user-field observations remain device-specific.
  No undocumented field or cryptographic sequence is required by Phase 4.
- Cross-OS acceptance and real-device interoperability are Phase 5/acceptance
  gates, not claims established by these documents.

## 4. Corrections and implementation prerequisites

No contradiction was found in the route policy or fallback state machine.
Before runtime implementation, M7/M8 must add `protocol.strategy` to the
typed settings and template, M4 must parse the explicit CLI override, and M9
must provide the injected transport/route/capability seams. These are
implementation prerequisites, not hidden specification behavior.

Phase 4 must use synthetic fixtures, fake transports, deterministic route and
capability providers, and must keep all sensitive fields out of assertions and
diagnostics.
