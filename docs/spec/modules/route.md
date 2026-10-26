# Module Specification: M9 - Protocol Route

## 1. Purpose

M9 selects the protocol used for each target and operation. It keeps SADP and
ISAPI transport implementations independent, applies the configured strategy,
and produces one typed outcome per target. M9 never changes host routing,
interface addresses, firewall rules, or network namespaces.

## 2. Strategy and API contract

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

type RouteTarget struct {
    MAC net.HardwareAddr
    Address netip.Addr
    Interface string
}

type RouteResult struct {
    Operation Operation
    Protocol string
    Target RouteTarget
    Fallback FallbackDecision
    Err error
}

type Selector interface {
    Execute(context.Context, Operation, RouteTarget) (RouteResult, error)
}
```

The selector depends on injected SADP and ISAPI transports, a route provider,
and a capability provider. Transports own their sockets or HTTP response
bodies and return no live resources. M9 owns only selection and outcome
composition.

## 3. Selection rules

`sadp_first` is the default. `isapi_only` never attempts SADP. In
`sadp_first`, fallback is per target and operation and is allowed only after a
classified SADP-unavailable result, an existing route to the target, and
capability confirmation for the requested ISAPI operation.

Device rejection, invalid credentials, authentication failure, malformed or
uncorrelated responses, identity mismatch, cancellation, deadline expiration,
and verification failure are terminal outcomes. They cannot become an ISAPI
fallback.

Discovery through ISAPI requires an explicit known address; ISAPI is not an
implicit replacement for SADP inventory. Activation and network configuration
must pass a post-operation status verification gate.

## 4. Errors, limits, and security

Stable error kinds distinguish invalid input, no route, SADP unavailable,
unsupported capability, device rejection, authentication failure, malformed
response, timeout, cancellation, and verification failure. Context deadlines
and cancellation stop retries and close owned resources. Response sizes,
request counts, retries, and concurrency are bounded.

Results and diagnostics must not contain passwords, challenges, nonces,
hashes, keys, authorization headers, XML, or complete packet payloads.
Observed device-specific ISAPI cryptography is not a universal contract.
