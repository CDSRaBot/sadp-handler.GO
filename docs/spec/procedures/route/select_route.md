# Procedure: SelectRoute

## Inputs

- caller `context.Context`;
- M9 strategy (`sadp_first` or `isapi_only`);
- operation and target;
- injected SADP/ISAPI transports, route provider, and capability provider.

## Outputs

Return one `RouteResult` for the target. It identifies the selected protocol,
operation, target, fallback decision, and a safe typed error when the
operation fails.

## Rules

1. Reject invalid strategy, operation, or target before network I/O.
2. For `isapi_only`, check route and capability, then execute ISAPI only.
3. For `sadp_first`, execute SADP first.
4. Consider ISAPI only when SADP returns a classified unavailable reason,
   route reachability is confirmed, and the requested capability is enabled.
5. Never fallback after rejection, authentication failure, malformed or
   mismatched response, cancellation, deadline, or verification failure.
6. Preserve `errors.Is` for context cancellation/deadline and close resources
   through the owning transport.

## Security

The result and error text are redacted; raw protocol payloads and credentials
are never returned or logged.
