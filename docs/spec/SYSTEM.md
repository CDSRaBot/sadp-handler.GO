# System Specification: SADP-handler (v0.1.0)

## 1. Purpose and Status

**SADP-handler** is a CLI-first Go utility for discovering, activating, and configuring Hikvision cameras and NVRs over the Search Active Device Protocol (SADP). Discovery targets devices on the local Layer 2 network, including devices that do not yet have an address usable by the host.

The project has completed its architecture and specification phase. This document defines the system-level boundaries; package APIs and atomic procedures are specified in the linked module and procedure documents.

## 2. Functional Scope

The system provides:

1. **Device discovery** — send SADP probes and receive device advertisements on supported local network interfaces.
2. **Device activation** — activate inactive devices, applying password-complexity rules and the required security exchange.
3. **Network configuration** — configure device IP address, subnet mask, gateway, and DHCP state.
4. **CLI operation** — expose these workflows to users and automation with structured configuration, output, and logging.

Bulk activation and network configuration must report per-device outcomes so a failure for one device does not conceal the result for others.

## 3. Module Boundaries

Modules communicate through Go package APIs and explicitly defined Go types. A package owns its data and errors; callers must not depend on another package's private implementation.

### [M0: Platform Adapter](./modules/adapter.md)

**Role:** isolate operating-system and network-interface differences.

- Enumerate eligible interfaces and expose the address and hardware information required by SADP.
- Configure and own the sockets needed for SADP traffic on the selected interface.
- Keep platform-specific behavior behind a small interface or package boundary where it improves portability and testability.

### [M1: Network Engine](./modules/network.md)

**Role:** manage SADP transport and discovery lifecycle.

- Send SADP probes and receive advertisements using Go networking APIs from `net`.
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

## 8. Phase 3 Audit Outcome and Implementation Gates

The specification audit has identified the following decisions that must be resolved before implementation can claim SADP protocol compatibility:

- Confirm the SADP multicast address, ports, interface-binding requirements, probe/response formats, and supported discovery behavior from authoritative protocol documentation or reproducible device captures.
- Confirm the activation and network-configuration XML schemas, required fields, encoding rules, response correlation, and challenge-response algorithm. The proposed SHA-256 calculation is not verified and must not be treated as a protocol fact.
- Determine the accepted datagram-size limit from protocol evidence. Do not use the Ethernet MTU as a substitute; M1 and M2 must share one verified limit and detect truncation.
- Verify OS-specific socket and multicast behavior on each supported platform, including required permissions and cancellation/cleanup behavior.
- Finalize package paths before implementation. The module path is set in `go.mod`; API signatures in module documents are design contracts, not implemented symbols.

The previous generic HTTP/JSON starter files and their Makefile were removed because they did not implement this system specification. The repository is specification-only until these gates are resolved and implementation begins. Phase 3 is therefore **in progress, not closed**; do not proceed to implementation on the basis of unverified protocol assumptions.

---

*Root system specification for the Go implementation of SADP-handler.*
