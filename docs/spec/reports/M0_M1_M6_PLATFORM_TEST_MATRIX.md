# M0/M1/M6 Platform Test Matrix

This matrix records the deterministic test boundary for Issue #48. The tests
use injected openers and packet connections; they do not require a live camera
or multicast network.

| Platform | Deterministic contract tests | Live multicast acceptance | Status |
|---|---|---|---|
| Windows | `internal/adapter`, `internal/network`, `internal/discovery` | Requires an eligible interface and permitted multicast socket | Host-gated; live socket not required for the unit gate |
| Linux/Termux | `internal/adapter`, `internal/network`, `internal/discovery` | Requires Go networking permissions and an eligible interface | Environment-gated; live socket not required for the unit gate |
| macOS | `internal/adapter`, `internal/network`, `internal/discovery` | Requires an eligible interface and permitted multicast socket | Environment-gated; live socket not required for the unit gate |

The deterministic suite verifies interface-specific opener arguments, multicast
validation, configured TTL propagation, packet-size bounds, sender
preservation, synchronous dispatch, one-reader/serialized-writer behavior,
cancellation that unblocks reads, exactly-once close ownership, initial-send
cleanup, listener/handler failures, joined cleanup errors, and redacted error
surfaces.

On the current Windows host, `go test -race` is environment-gated: Go 1.27.1
is installed, but `CGO_ENABLED=1` cannot start because no `gcc` compiler is
available. The non-race deterministic suite, `go test ./...`, `go vet ./...`,
and `go build ./...` pass. The race command must be rerun after a supported C
compiler is installed; this is an environment limitation, not a skipped test
case.

Platform-specific socket behavior is exercised only through the production
`OpenMulticast` seam. No test silently substitutes another interface, changes
host routing, or claims successful live multicast behavior when the host cannot
provide it.
