# Dual SADP/ISAPI Route Contract

**Status:** Phase 1 design baseline  
**Scope:** route selection for discovery, activation, network configuration,
and status verification  
**Normative specifications:** updated in Phase 2; this document records the
decisions that Phase 2 must preserve.

## 1. Route policy

The default policy is `sadp_first`. The caller may explicitly select
`isapi_only`; explicit selection never silently changes to SADP. Under
`sadp_first`, ISAPI fallback is permitted only after a classified
SADP-unavailable result and only when the route provider confirms that the
device address is reachable through an already available local route or
interface.

The route service does not change host addresses, routing tables, interface
state, firewall rules, or network namespaces. It does not promise automatic
subnet switching. An operator may provide an explicit interface or route
selection through configuration or CLI.

## 2. Proposed Go boundary

The route orchestration boundary belongs in a dedicated `internal/route`
package. It depends on protocol-specific transport interfaces and does not
own socket or HTTP implementation details.

```go
type Strategy string

const (
    StrategySADPFirst Strategy = "sadp_first"
    StrategyISAPIOnly Strategy = "isapi_only"
)

type Operation string

const (
    OperationDiscovery Operation = "discovery"
    OperationActivation Operation = "activation"
    OperationNetwork Operation = "network"
    OperationStatus Operation = "status"
)

type SADPUnavailableReason string

const (
    ReasonNoUsableInterface SADPUnavailableReason = "no_usable_interface"
    ReasonMulticastBlocked SADPUnavailableReason = "multicast_blocked"
    ReasonRouteUnavailable SADPUnavailableReason = "route_unavailable"
    ReasonTransportTimeout SADPUnavailableReason = "transport_timeout"
    ReasonTransportCanceled SADPUnavailableReason = "transport_canceled"
    ReasonProtocolUnsupported SADPUnavailableReason = "protocol_unsupported"
)

type FallbackDecision string

const (
    FallbackNotAllowed FallbackDecision = "not_allowed"
    FallbackEligible FallbackDecision = "eligible"
    FallbackRouteUnavailable FallbackDecision = "route_unavailable"
)

type Capability struct {
    Model          string
    Firmware       string
    SADP           bool
    ISAPI          bool
    ISAPIActivation bool
    ISAPINetwork   bool
    ISAPIStatus    bool
}

type RouteTarget struct {
    MAC       net.HardwareAddr
    Address   netip.Addr
    Interface string
}

type RouteResult struct {
    Operation Operation
    Protocol  string
    Target    RouteTarget
    Fallback  FallbackDecision
    Err       error
}

type SADPTransport interface {
    Execute(context.Context, Operation, RouteTarget) (RouteResult, error)
}

type ISAPITransport interface {
    Execute(context.Context, Operation, RouteTarget) (RouteResult, error)
}

type RouteProvider interface {
    CanReach(context.Context, netip.Addr, string) (bool, error)
}

type CapabilityProvider interface {
    Get(context.Context, RouteTarget) (Capability, error)
}

type Selector interface {
    Execute(context.Context, Operation, RouteTarget) (RouteResult, error)
}
```

The final exported names may be adjusted during Phase 2 to match existing
package conventions, but the semantic states and boundaries are fixed here.
Protocol transports must not expose raw credentials, challenges, XML, packet
payloads, HTTP authorization headers, or cryptographic material through
`RouteResult`.

## 3. Fallback rules

Fallback is evaluated per target and per operation, not once for an entire
discovery session. A result is eligible only when all of the following hold:

1. the selected strategy is `sadp_first`;
2. SADP failed before the device accepted or rejected the requested operation;
3. the failure maps to `SADPUnavailableReason`;
4. the target has a usable address;
5. `RouteProvider` confirms an existing route on the selected or default
   interface;
6. `CapabilityProvider` confirms that the requested ISAPI operation is
   supported.

The following are terminal and must never trigger automatic fallback:

- invalid input;
- device rejection;
- invalid credentials or HTTP authentication failure;
- malformed, contradictory, or uncorrelated response;
- target identity mismatch;
- cancellation or caller deadline expiration;
- an ISAPI capability or route check that fails.

An explicitly selected `isapi_only` operation may return a capability or route
error, but it does not attempt SADP.

## 4. Operation scope

- **Discovery:** SADP is the default inventory source. ISAPI discovery is not
  implied; it requires an explicit address or a previously known target.
- **Activation:** SADP activation is preferred. ISAPI activation requires a
  capability-gated documented variant; device-specific challenge, RSA, AES,
  or encrypted fields are not generalized by this contract.
- **Network configuration:** the same selection and fallback rules apply.
  Success requires a subsequent status verification.
- **Status verification:** use the protocol that completed the operation
  unless the caller explicitly requests another protocol. Verification failure
  is not converted into protocol fallback.

Each target produces one typed outcome. Bulk execution continues for
independent targets while preserving each target's protocol, failure class,
and verification result.

## 5. Ownership, deadlines, and errors

The selector owns the operation context and closes transport resources through
the transport interfaces. SADP socket ownership remains with the SADP
transport; HTTP client and response-body ownership remains with the ISAPI
transport. Neither transport transfers live resources in a result.

Every operation receives a caller context. Implementations must propagate
cancellation and deadlines to socket and HTTP operations, stop retries, and
return errors that remain detectable with `errors.Is`. Retry limits and
request/response size limits are bounded and protocol-specific.

Route errors must be typed and distinguish at least:

- invalid input;
- no usable route;
- SADP unavailable;
- unsupported capability;
- device rejection;
- authentication failure;
- malformed or mismatched response;
- timeout;
- cancellation;
- verification failure.

Error text is safe for users and logs. It must not include passwords,
challenges, nonces, hashes, keys, authorization headers, XML, or complete
network payloads.

## 6. Configuration and CLI semantics

The configuration key is `protocol.strategy` with values `sadp_first` and
`isapi_only`; the default is `sadp_first`. CLI overrides are applied after
the settings file and use `--protocol=sadp-first` or
`--protocol=isapi-only`. An optional explicit interface/address selection is
passed to the route provider; it never mutates host routing state.

Invalid strategy values fail configuration validation. There is no hidden
fallback switch and no automatic route manipulation.

## 7. Capability and protocol evidence

Capability detection may use trusted discovery metadata, an explicit device
profile, or a bounded ISAPI capability probe. Model and firmware values are
hints until the device confirms the requested operation. The contract does
not promote undocumented HTTP fields or observed cryptographic sequences to
universal behavior. Unsupported or device-specific activation variants return
an explicit capability error.
