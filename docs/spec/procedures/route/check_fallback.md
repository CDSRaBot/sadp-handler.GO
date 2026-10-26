# Procedure: CheckFallbackEligibility

The procedure receives a SADP result, strategy, target, operation, route
provider, and capability provider. It returns `eligible` only for
`sadp_first` after a classified SADP-unavailable reason, a reachable existing
route, and capability support for the requested ISAPI operation.

Invalid input, device rejection, invalid credentials, authentication failure,
malformed or mismatched response, cancellation, deadline, and verification
failure return terminal `not_allowed`. A failed route or capability check is
reported explicitly and is not retried as another protocol.

No host address, route table, interface, firewall, or namespace is modified.
