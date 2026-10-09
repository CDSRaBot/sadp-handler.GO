# System Specification: SADP-handler (v0.1.0)

## 1. Purpose and Status

**SADP-handler** is a terminal-based Go utility for discovering, activating, and configuring Hikvision cameras and NVRs over the Search Active Device Protocol (SADP). Discovery targets devices on the local Layer 2 network, including devices that do not yet have an address usable by the host.

The project has completed its architecture and specification phase. This document defines the system-level boundaries; package APIs and atomic procedures are specified in the linked module and procedure documents.

## 2. Functional Scope

The system is designed to provide:

1. **Device discovery** — receive SADP advertisements on supported local network interfaces. Probe transmission and the complete request/response flow remain unspecified until verified against device evidence; the current network contract must not imply that discovery probes are already implemented.
2. **Device activation** — activate inactive devices, applying password-complexity rules and the required security exchange.
3. **Network configuration** — configure device IP address, subnet mask, gateway, and DHCP state.
4. **CLI operation** — expose these workflows to users and automation with structured configuration, output, and logging. The default user-facing mode is a full-screen TUI; `--ui=no` selects standard command-line operation for one-shot use and automation.

These are target capabilities, not a claim that end-to-end device operations are currently specified or implemented. Bulk activation and network configuration must report per-device outcomes so a failure for one device does not conceal the result for others. The current module contracts do not yet define transmission of these requests or a complete response/correlation workflow.

## 3. Module Boundaries

Modules communicate through Go package APIs and explicitly defined Go types. A package owns its data and errors; callers must not depend on another package's private implementation.

### [M0: Platform Adapter](./modules/adapter.md)

**Role:** isolate operating-system and network-interface differences.

- Enumerate eligible interfaces and expose the address and hardware information required by SADP.
- Configure and own the sockets needed for SADP traffic on the selected interface.
- Keep platform-specific behavior behind a small interface or package boundary where it improves portability and testability.

### [M1: Network Engine](./modules/network.md)

**Role:** manage SADP UDP receive lifecycle; add probe transmission once its payload and response flow are established.

- Receive advertisements using Go networking APIs from `net`. The local Python reference does not contain probe transmission, so the probe payload is not defined by this baseline.
- Use goroutines and channels only where concurrency is needed; make listener shutdown and cancellation explicit.
- Accept `context.Context` for operations that can block, and ensure sockets, goroutines, and other resources are released on completion or cancellation.
- Pass received payloads to the parser without embedding business rules in the transport layer.

### [M2: Data Parser](./modules/parser.md)

**Role:** validate and transform untrusted device data.

- Parse SADP XML with the standard library's `encoding/xml`.
- Convert valid advertisements into typed Go models used by the rest of the application.
- Validate required fields and enforce input-size and value limits before data reaches business logic.
- Return explicit errors for malformed or incomplete data; unknown optional fields may be ignored when that preserves forward compatibility.

### [M3: Logic Engine](./modules/logic.md)

**Role:** implement device-management rules and workflows.

- Coordinate activation, security challenge-response, and network-configuration operations.
- Validate passwords and configuration before sending commands.
- Construct protocol payloads from validated typed inputs and interpret device responses.
- Keep workflow rules independent of CLI presentation and operating-system details.

### [M4: CLI](./modules/ui.md)

**Role:** provide the command-line user experience and connect user input to application workflows.

- Parse and validate command-line arguments and configuration.
- Present discovery results and operation outcomes, including errors and partial results.
- Start in TUI mode by default and select standard command-line mode only when `--ui=no` is supplied; report an actionable error if the default TUI is requested without usable terminal input and output.
- Keep terminal interaction separate from business logic; do not make a particular CLI library mandatory at the system level.

## 4. Data and API Contracts

- Public package APIs use typed Go values rather than unstructured maps.
- The specification for each package must define its purpose, public API, input and output types, possible errors, and behavior for nil or empty inputs, timeouts, and cancellation where applicable.
- Data passed between modules must be validated at the boundary where it enters the system. Raw network payloads must not be treated as trusted application models.
- Errors are part of each API contract. Callers must be able to distinguish and wrap failures with context; invalid data or failed operations must not be represented as success-shaped zero values.
- Use interfaces at dependency boundaries when substitution is needed for testing or portability. Avoid interfaces and global state without a concrete need.

## 5. Go Architecture and Runtime Constraints

- Target **Go 1.22 or later**, consistent with the project README.
- The Go module path is `github.com/CDSRaBot/sadp-handler.GO`, matching the planned GitHub repository. Go imports are rooted at this module path.
- Follow the project layout: `cmd/` for executable entry points, `internal/` for application and private implementation packages, and `pkg/` only for libraries intentionally reusable outside this module. Add directories such as `config/`, `models/`, or `utils/` only when they represent a clear package responsibility.
- Repository filesystem paths used in documentation, configuration, and build instructions are relative to the project root. Do not hard-code machine-specific absolute paths.
- Keep interface enumeration and socket setup in the platform adapter; transport in the network engine; parsing in the parser; device workflows in the logic engine; and presentation in the CLI.
- Use `context.Context` for cancellable network and I/O operations. Concurrent work must have a defined owner, shutdown path, and error-reporting strategy; avoid goroutine leaks and unbounded resource use.
- Keep the interface mode fixed for one process invocation. TUI application commands run workflows within the TUI; changing to standard CLI mode requires a separate invocation.
- Prefer the Go standard library, including `net`, `encoding/xml`, `context`, and `log/slog`. A CLI, configuration, or logging dependency may be selected by implementation, but no optional third-party library is required by this system specification.
- Keep source comments and public Go documentation in English. Document exported types and functions, and add implementation comments only when they clarify non-obvious behavior.

## 6. Error Handling, Security, and Resource Policy

- Return and handle errors explicitly. Do not ignore errors or use `panic` in production workflows.
- Apply timeouts or cancellation to network operations and bound input sizes, concurrency, and other resource consumption.
- Treat network data, CLI input, and configuration as untrusted. Validate addresses, ports, device fields, passwords, and protocol values before use.
- Parse XML with `encoding/xml`; do not build XML or JSON by string formatting when a serializer is appropriate.
- Do not log passwords, password-derived secrets, or other sensitive activation material.
- Close files, response bodies, sockets, and other owned resources on all relevant paths.

## 7. Specification and Verification Requirements

Changes follow the development lifecycle in [`DEVELOPMENT_GUIDE.md`](../DEVELOPMENT_GUIDE.md):

1. Define the behavior, package boundaries, Go types, and error contract before implementation.
2. Update this system document when module responsibilities or dependencies change; document package APIs and atomic procedures at their corresponding levels.
3. Audit data flow, error handling, platform support, serialization, context use, and concurrency before coding.
4. Derive tests from the specification, including contract, success, negative, timeout, and fuzz tests where appropriate. Place Go tests in `*_test.go` files and use the standard `testing` package.
5. Validate the implementation with the applicable project checks, including `gofmt`, `go vet ./...`, `go test ./...`, and `go build ./...`. Run configured tools such as `staticcheck` or `golangci-lint` when available.
6. If implementation changes specified behavior, update the specification and tests together.

## 8. Python Reference and Protocol Compatibility Gates

The user identified `../examples/SADP-app` as a working Python implementation and directed that its processing approach be used as the local behavioral reference. The implementation uses interface-aware UDP sockets, packages received datagrams with their sender and timestamp, parses XML into a typed device model, and calculates `SHA-256(UTF-8(password + challenge))` as lowercase hexadecimal.

This is an implementation reference, not an authoritative SADP protocol specification. The Python files currently present establish only part of the wire behavior:

- `core/contracts.py` sets `239.255.255.250:37020` and `MAX_PACKET_SIZE = 1500`. The size is labelled for fuzzing/MTU purposes in that code; it is not evidence of a SADP protocol maximum.
- `core/adapter.py` joins the multicast group on a selected interface on Unix-like systems, binds UDP/37020 there, and sets multicast TTL to 64. Its Windows branch binds an interface address on an ephemeral port and does not join the multicast group. A Go adapter must implement and test the intended interface-specific behavior per OS rather than copy this discrepancy.
- `core/protocol.py` bounds received datagrams and dispatches them synchronously to a callback. The files do not define or send a discovery probe, nor do they implement a full request/response exchange or correlate replies.
- `services/parser.py` expects direct XML child fields `MAC`, `IPv4Address`, `Activated`, `DeviceModel`, `SerialNumber`, and optionally `SoftwareVersion`. It accepts UTF-8 with an ISO-8859-1 fallback, requires MAC and IPv4 address, and uses `defusedxml`.
- `services/logic/crypto.py` implements SHA-256 over the UTF-8 bytes of `password + challenge`. The helper extracting `Challenge`/`Salt` fields does not itself implement challenge negotiation.
- `services/logic/__init__.py` emits `Activate` and `NetworkParameter` XML requests. Its missing-challenge branch sends the plaintext password; this fallback is explicitly not adopted by this Go specification.

Accordingly, the reference behavior above may guide implementation and fixture design, but must not be presented as an independently verified protocol guarantee. Keep these gates open before claiming compatibility beyond the locally referenced behavior:

- Establish the actual probe/response flow and supported SADP variants from runnable end-to-end behavior or reproducible device captures.
- Define and verify the transport contract for activation and network-configuration payloads: destination, send behavior, reply flow/correlation, timeout, cancellation, and per-device outcome mapping. M1 currently specifies receiving only; M3 XML builders alone do not constitute an executable operation.
- Confirm activation and network-configuration XML fields, challenge acquisition/correlation, and the SHA-256 input/encoding against known-good device exchanges. Do not assume the Python helper's SHA-256 expression is universally valid across models and firmware.
- Use 1500 bytes only as an explicitly named compatibility/resource bound if chosen; it is not a verified protocol limit. M1 and M2 must use one shared bound and the receiver must detect/reject truncation and oversize datagrams.
- Verify per-interface UDP multicast behavior and cancellation/cleanup on each supported OS. In particular, do not reproduce the Python Windows branch without evidence that it receives the intended SADP replies.
- Finalize package paths before implementation. The module path is set in `go.mod`; API signatures in module documents are design contracts, not implemented symbols.

The Go implementation has not started. It may follow the Python reference's processing boundaries, but protocol compatibility remains **unverified** until the gates above are resolved.

---

*Root system specification for the Go implementation of SADP-handler.*
