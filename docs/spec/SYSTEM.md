# System Specification: SADP-handler (v0.1.0)

## 1. Purpose and Status

**SADP-handler** is a terminal-based Go utility for discovering, activating, and configuring Hikvision cameras and NVRs over the Search Active Device Protocol (SADP). Discovery targets devices on the local Layer 2 network, including devices that do not yet have an address usable by the host.

The architecture baseline is established and the specifications define system boundaries, module APIs, and atomic procedures. The Python implementation is the verified behavioral baseline for discovery: it has been tested with real devices, finds them, and receives their responses. The Go implementation must preserve this behavior and be validated against the same real-device scenario. SADP activation identifies its target by MAC address and uses the SADP XML request defined by M3. The `SHA-256(UTF-8(password + challenge))` challenge-response calculation is accepted as verified by practical experiments. End-to-end Go operations still require implementation and integration tests; details not covered by those experiments remain explicitly open.

## 2. Functional Scope

The system is designed to provide:

1. **Device discovery** — send the SADP inquiry probe and receive device responses on supported local network interfaces. The Python implementation has been verified on real devices to find devices and receive their responses; its behavior is the compatibility baseline for the Go implementation. HiToolsDelivery captures document compatible traffic, including UDP multicast destination `239.255.255.250:37020`, an XML `<Probe>` request containing `<Uuid>`, `<Types>inquiry</Types>`, and `<ResponseMode>2</ResponseMode>`, device `<ProbeMatch>` messages, and a separate multicast `<Hello>` message containing `<Types>hello</Types>`. The captures do not establish universal response correlation, retransmission policy, timing, or behavior across all models and firmware.
2. **Device activation** — send a SADP activation XML request identified by target MAC, applying password-complexity rules. The request format established by practical experiments is the compatibility baseline; activation outcome handling remains an application workflow.
3. **Network configuration** — configure device IP address, subnet mask, gateway, and DHCP state.
4. **CLI operation** — expose these workflows to users and automation with a shared command catalog and typed results. The default user-facing mode is a full-screen TUI; `--ui=no` starts an interactive CLI REPL, and `--ui=no --once` runs one command from process arguments and exits.

These are target capabilities, not a claim that end-to-end device operations are currently implemented. Bulk activation and network configuration must report per-device outcomes so a failure for one device does not conceal the result for others. M3 defines the SADP activation request, while its end-to-end sender/result workflow and network-configuration transport/result workflow must be completed before those commands are enabled.

## 3. Module Boundaries

Modules communicate through Go package APIs and explicitly defined Go types. A package owns its data and errors; callers must not depend on another package's private implementation.

### [M0: Platform Adapter](./modules/adapter.md)

**Role:** isolate operating-system and network-interface differences.

- Enumerate eligible interfaces and expose the address and hardware information required by SADP.
- Configure and own the sockets needed for SADP traffic on the selected interface.
- Keep platform-specific behavior behind a small interface or package boundary where it improves portability and testability.

### [M1: Network Engine](./modules/network.md)

**Role:** manage SADP UDP probe transmission and receive lifecycle.

- Build and send the observed inquiry probe over the M0-configured multicast connection, and receive advertisements using Go networking APIs from `net`.
- Keep the confirmed probe request contract separate from unverified response, correlation, retransmission, and timing behavior.
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
- Construct SADP activation payloads from validated typed inputs and interpret device responses.
- Keep workflow rules independent of CLI presentation and operating-system details.

### [M4: CLI](./modules/ui.md)

**Role:** provide the command-line user experience and connect user input to application workflows.

- Parse and validate command-line arguments and configuration.
- Present discovery results and operation outcomes, including errors and partial results.
- Start in TUI mode by default and select standard command-line mode only when `--ui=no` is supplied; report an actionable error if the default TUI is requested without usable terminal input and output.
- Keep terminal interaction separate from business logic; do not make a particular CLI library mandatory at the system level.
- Use separate TUI/CLI parsers that map into a shared typed command model and dispatcher; `--ui=no` selects the REPL and `--once` selects one-shot execution.

### [M5: Localization](./modules/localization.md)

**Role:** validate, load, resolve, and switch immutable locale catalogs independently of terminal rendering.

- Use the embedded English catalog as the canonical key set and fallback.
- Enumerate external locale files and export the English translation template.
- Install a fully loaded locale as one atomic state change; a failed load leaves the current locale unchanged.

M5 supplies locale services to M4 and does not own command-line parsing or terminal presentation.

### [M6: Discovery Coordinator](./modules/discovery.md)

**Role:** compose interface-specific socket setup, scheduled sends of the observed inquiry probe, datagram listening, and advertisement parsing into a cancellable discovery session.

- Operate on one explicitly selected interface per call and report parsed devices or rejected datagrams through a caller handler.
- Keep socket ownership, cancellation, callback errors, and partial results explicit.
- Do not infer response correlation or protocol retry requirements from the application-scheduled probes, or assign a schedule to unsolicited `Hello` messages.

M6 depends on M0, M1, and M2; M4 invokes M6 and presents its events.

### [M7: Configuration](./modules/configuration.md)

**Role:** load and validate the external `settings.json` and provide the
typed baseline configuration used by M0-M6.

- Use `settings.json` as the only external baseline settings source.
- Apply documented defaults only when the file is absent or a setting is
  explicitly documented as omitted/zero.
- Require a `_comments` entry for every setting; comments document all choices
  and value boundaries because JSON has no native comment syntax.
- Apply CLI overrides after file settings and before starting workflows.

### [M8: Application Composition](./modules/application.md)

**Role:** convert M7 settings into one typed `RuntimeConfig` and compose
dependencies for M4, M5, and M6.

- Own startup precedence and ensure JSON settings are decoded only once.
- Construct M5's `fs.FS`, M0's `MulticastOptions`, M6's `DiscoveryOptions`,
  and the configured `slog` level.
- Keep configuration out of package globals and prevent downstream modules from
  depending on the `settings.json` schema.

### Logging policy

Logging is a cross-cutting concern implemented by the internal
[`logging` specification](./logging.md) and Go's standard `log/slog`
package. M8 initializes one logger before workflows start and injects it into
modules through typed dependencies. The logger writes to `.\logs`, rotates
before a file exceeds 5 MiB, and writes a blank line plus a timestamped
start-of-run header before the first record of every process run. Each module
and atomic procedure inherits its logging contract from the
[Common Procedure Policy](./procedures/POLICY.md) and the
[logging specification](./logging.md). Procedure documents declare only
operation-specific additions or stricter rules.

## 4. Data and API Contracts

- Public package APIs use typed Go values rather than unstructured maps.
- The specification for each package must define its purpose, public API, input and output types, possible errors, and behavior for nil or empty inputs, timeouts, and cancellation where applicable.
- Data passed between modules must be validated at the boundary where it enters the system. Raw network payloads must not be treated as trusted application models.
- Errors are part of each API contract. Callers must be able to distinguish and wrap failures with context; invalid data or failed operations must not be represented as success-shaped zero values.
- When callers branch on failure category, expose a package-owned typed error or sentinel and preserve it through wrapping with `Unwrap`; callers must use `errors.Is`/`errors.As`, never parse error strings. Each such error contract specifies its stable categories and whether the underlying cause is retained.
- Use interfaces at dependency boundaries when substitution is needed for testing or portability. Avoid interfaces and global state without a concrete need.

## 5. Go Architecture and Runtime Constraints

- Target **Go 1.22 or later**, consistent with the project README.
- The Go module path is `github.com/CDSRaBot/sadp-handler.GO`, matching the planned GitHub repository. Go imports are rooted at this module path.
- Follow the project layout: `cmd/` for executable entry points, `internal/` for application and private implementation packages, and `pkg/` only for libraries intentionally reusable outside this module. Add directories such as `config/`, `models/`, or `utils/` only when they represent a clear package responsibility.
- Repository filesystem paths used in documentation, configuration, and build instructions are relative to the project root. Do not hard-code machine-specific absolute paths.
- Keep interface enumeration and socket setup in the platform adapter; transport in the network engine; parsing in the parser; device workflows in the logic engine; and presentation in the CLI.
- Keep discovery orchestration in M6 and locale catalog/state management in M5; neither belongs in the CLI presentation module.
- Use `context.Context` for cancellable network and I/O operations. Concurrent work must have a defined owner, shutdown path, and error-reporting strategy; avoid goroutine leaks and unbounded resource use.
- Keep the interface mode fixed for one process invocation. TUI application commands run workflows within the TUI; changing to standard CLI mode requires a separate invocation.
- In CLI mode, use the REPL unless `--once` is explicitly supplied; never infer one-shot execution merely because command arguments are present.
- The TUI owns at most one continuous discovery session. Interface changes stop and join the prior session before starting another.
- Prefer the Go standard library, including `net`, `encoding/xml`, `context`,
  and `log/slog`. The approved TUI stack is Bubble Tea, Bubbles, and Lip
  Gloss; M4 must use it when the TUI is implemented and declare the
  dependencies in `go.mod`. No other CLI, TUI, configuration, or logging
  framework is permitted without a system-specification change.
- Keep source comments and public Go documentation in English. Document exported types and functions, and add implementation comments only when they clarify non-obvious behavior.

## 6. Error Handling, Security, and Resource Policy

- Return and handle errors explicitly. Do not ignore errors or use `panic` in production workflows.
- Apply timeouts or cancellation to network operations and bound input sizes, concurrency, and other resource consumption.
- Treat network data, CLI input, and configuration as untrusted. Validate addresses, ports, device fields, passwords, and protocol values before use.
- All baseline runtime settings are read from `settings.json` through M7 and
  composed by M8. The
  complete key set, defaults, comments, and boundaries are defined in
  [M7](./modules/configuration.md) and the checked-in [template](../../settings.json).
  In particular, `network.max_sadp_datagram_size` defaults to 1500 (zero also
  means default) and accepts 1 through 65507 bytes. The resolved value is
  passed unchanged through discovery to M1 and M2. This is an application
  resource limit, not a SADP protocol maximum.
- Parse XML with `encoding/xml`; do not build XML or JSON by string formatting when a serializer is appropriate.
- Do not log passwords, password-derived secrets, or other sensitive activation material.
- Do not log complete network payloads, XML, or complete configuration
  documents. Diagnostic logs must be separate from text and JSON CLI output.
- Close files, response bodies, sockets, and other owned resources on all relevant paths.

## 7. Specification and Verification Requirements

Changes follow the development lifecycle in [`DEVELOPMENT_GUIDE.md`](../DEVELOPMENT_GUIDE.md):

1. Define the behavior, package boundaries, Go types, and error contract before implementation.
2. Update this system document when module responsibilities or dependencies change; document package APIs and atomic procedures at their corresponding levels.
3. When a Russian document links to an English contract, the English document is normative; the Russian copy is a guide and must not contradict it.
4. Audit data flow, error handling, platform support, serialization, context use, and concurrency before coding.
5. Derive tests from the specification, including contract, success, negative, timeout, and fuzz tests where appropriate. Place Go tests in `*_test.go` files and use the standard `testing` package.
6. Validate the implementation with the applicable project checks, including `gofmt`, `go vet ./...`, `go test ./...`, and `go build ./...`. Run configured tools such as `staticcheck` or `golangci-lint` when available.
7. If implementation changes specified behavior, update the specification and tests together.

## 8. Python Reference and Protocol Compatibility Gates

The user identified `../examples/SADP-app` as a working Python implementation and directed that its processing approach be used as the local behavioral reference. The implementation uses interface-aware UDP sockets, packages received datagrams with their sender and timestamp, parses XML into a typed device model, and calculates `SHA-256(UTF-8(password + challenge))` as lowercase hexadecimal.

This is an implementation reference, not an authoritative SADP protocol specification. Although individual Python components describe only parts of the wire behavior, the complete discovery implementation has been verified with real devices as stated above. The component notes below are implementation details, not limits on that end-to-end evidence:

- `core/contracts.py` sets `239.255.255.250:37020` and `MAX_PACKET_SIZE = 1500`. The size is labelled for fuzzing/MTU purposes in that code; it is not evidence of a SADP protocol maximum.
- `core/adapter.py` joins the multicast group on a selected interface on Unix-like systems, binds UDP/37020 there, and sets multicast TTL to 64. Its Windows branch binds an interface address on an ephemeral port and does not join the multicast group. A Go adapter must implement and test the intended interface-specific behavior per OS rather than copy this discrepancy.
- `core/protocol.py` bounds received datagrams and dispatches them synchronously to a callback. Treat that file's scope separately from the verified end-to-end discovery behavior; successful discovery does not establish universal response correlation or retry semantics.
- `services/parser.py` expects direct XML child fields `MAC`, `IPv4Address`, `Activated`, `DeviceModel`, `SerialNumber`, and optionally `SoftwareVersion`. It accepts UTF-8 with an ISO-8859-1 fallback, requires MAC and IPv4 address, and uses `defusedxml`.
- `services/logic/crypto.py` implements SHA-256 over the UTF-8 bytes of `password + challenge`. The helper extracting `Challenge`/`Salt` fields does not itself implement challenge negotiation.
- `services/logic/__init__.py` emits `Activate` and `NetworkParameter` XML requests. Its missing-challenge branch sends the plaintext password; this fallback is explicitly not adopted by this Go specification.

The Python implementation's successful real-device discovery is verified behavior and is the baseline for this feature; it is not by itself a universal protocol guarantee. Keep the following protocol details open unless separately verified:

- The user-provided HiToolsDelivery captures show requests sent to UDP multicast `239.255.255.250:37020` with XML `Probe` elements `Uuid`, `Types=inquiry`, and `ResponseMode=2`. Twelve request entries contain six UUID values, each appearing twice; this repetition is an observation, not a retry requirement.
- A subsequent capture contains a 1,382-byte XML `ProbeMatch` response for a DS-2CD1021-I. Its direct-child fields include `Uuid`, `Types=inquiry`, device description and serial fields, command/HTTP ports, MAC and IPv4/IPv6 network fields, DHCP and channel counts, software/DSP versions, boot time, activation/security capability fields, a `Salt`, and lock/reset-related fields. The response UUID agrees between the displayed response records. The corresponding request payload is not included with this response capture, so equality with the probe UUID is not verified.
- The response capture shows an exchange entry between `192.168.0.128` and `192.168.0.160` on UDP port `37020`, and a device-originated datagram from `192.168.0.160:37020` to `239.255.255.250:37020`, with the same displayed `ProbeMatch` payload. Preserve these as observed records without inferring universal unicast/multicast delivery behavior.
- Another capture contains fourteen `Probe` XML documents: seven distinct UUID values, each appearing twice, in multicast traffic to `239.255.255.250:37020`. This is consistent with repeated transmissions in this capture but does not establish required retry behavior, pairing semantics, cadence, or whether UUID reuse is required.
- The same later capture contains a 1,319-byte device-originated UDP datagram from `192.168.0.160:37020` to `239.255.255.250:37020`, with root `<Hello>`, `<Types>hello</Types>`, and device-information fields overlapping the observed `ProbeMatch`. It has no displayed UUID and appears after the displayed probe burst; the capture does not establish that it is a reply to that burst or define its announcement schedule.
- Do not store sample MAC addresses, serial numbers, UUIDs, or salt values in specifications or fixtures. Treat security-related response fields as sensitive and never log full response XML.
- Verify `ProbeMatch`/`Hello` variants and correlation behavior across additional devices/firmware before making fields mandatory or defining retries, timing, or a universal response-delivery contract.
- Define and verify the transport contract for activation and network-configuration payloads: destination, send behavior, reply flow/correlation, timeout, cancellation, and per-device outcome mapping. M1 currently specifies inquiry-probe sending and receiving only; it does not define transmission of activation or network-configuration commands. M3 XML builders alone do not constitute an executable operation.
- The selected activation protocol is SADP XML identified by target MAC, as specified in P3.1; do not substitute ISAPI HTTP activation. The `SHA-256(UTF-8(password + challenge))` calculation used by network configuration is confirmed by practical experiments and is a normative M3 compatibility requirement.
- The default `MaxSADPDatagramSize` is 1500 bytes and is configurable through settings as specified above. M1 and M2 must use the same resolved setting and the receiver must detect/reject truncation and oversize datagrams.
- Continue to verify activation response/result behavior and the network-configuration request/response flow separately; request/hash confirmation alone does not establish all device outcome semantics or cross-model behavior.
- Verify per-interface UDP multicast behavior and cancellation/cleanup on each supported OS. In particular, do not reproduce the Python Windows branch without evidence that it receives the intended SADP replies.
- Finalize package paths before implementation. The module path is set in `go.mod`; API signatures in module documents are design contracts, not implemented symbols.

Encrypted ISAPI user-field handling is specified in the [protocol report](./reports/ISAPI_ENCRYPTED_USER_FIELDS.md) and detailed SADP/ISAPI pipelines. This documents the protocol contract; it does not claim that the workflow is implemented.

The Go implementation has not started, so Go parity with the verified Python discovery baseline remains to be implemented and tested. The unresolved items above do not invalidate the confirmed Python discovery behavior; they limit claims about unverified protocol details, other devices/platforms, and activation or network configuration.

### M9: Protocol Route

M9 selects SADP or ISAPI per target and operation. `sadp_first` is the
default; `isapi_only` is explicit. Fallback is allowed only after a
classified SADP-unavailable result, an existing route to the target, and
capability confirmation for the requested ISAPI operation. Device rejection,
authentication failure, malformed or mismatched responses, cancellation,
deadlines, and verification failures are terminal and never trigger fallback.

M9 depends on M0/M1 for SADP interface and transport behavior, M3 for
operation semantics, M7/M8 for configuration and dependency injection, and M4
for user-facing selection. It never changes host routing or interface state.
The [M9 module specification](./modules/route.md) and
[route procedures](./procedures/route/select_route.md) define the typed
outcomes, cleanup, limits, and redaction rules.

---

*Root system specification for the Go implementation of SADP-handler.*
