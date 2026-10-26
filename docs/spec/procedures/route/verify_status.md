# Procedure: VerifyStatus

## Contract

`VerifyStatus` receives a context, completed operation, target, and selected
protocol transport. It performs a bounded status query and returns the
validated device state or a typed verification error.

Activation and network configuration are not successful until verification
confirms the expected state and target identity. Verification failure is
terminal and does not trigger protocol fallback. Cancellation, deadline,
malformed data, identity mismatch, and authentication failure remain
detectable through typed errors and wrapped context causes.

Status results are bounded and redacted. Passwords, challenges, HTTP
authorization data, XML, and complete payloads are never returned or logged.
