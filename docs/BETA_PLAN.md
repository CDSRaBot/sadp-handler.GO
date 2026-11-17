# Beta implementation and release plan

## Goal and scope

Keep SADP discovery available while disabling SADP activation. When SADP
activation is selected, inform the operator and use ISAPI only when the target
address, route and operation capability are established. Never change host
network settings automatically.

Implement the ISAPI operations described in the specifications, including
known-address discovery, authentication, activation, verification, network
configuration, user management, security questions and documented terminal
management. Device-specific, unverified procedures must fail explicitly.
Retain SADP network configuration only for profiles actually verified on the
target device; the observed salted/AES profile is not yet established.

## Work items

1. **Specification and inventory:** reconcile English and Russian system,
   route, UI, procedure and help documents. Define operation inputs, capability
   and identity checks, failure outcomes, strategy and CLI compatibility,
   confirmation, security and per-device results.
2. **SADP activation gate:** enforce the ban at the service and routing
   boundaries, not only in the UI. Show a localized message and attempt ISAPI
   only when prerequisites are met. Preserve SADP discovery and verified
   network profiles; reject unverified profiles before sending a mutation.
3. **ISAPI transport and authentication:** use each target's actual address,
   bounded and cancellable HTTP(S), safe TLS policy, Digest authentication and
   explicitly enabled Basic only. Classify HTTP and ISAPI errors and never log
   credentials or sensitive payloads.
4. **ISAPI operations:** complete the documented activation challenge and
   encryption, verify activated state and authenticated device identity, and
   implement DHCP/static network changes with reappearance and identity
   verification. Implement capability-gated encrypted user, recovery-question
   and terminal-management procedures where their device-specific schema and
   key source are established; otherwise fail explicitly.
5. **Application integration:** connect discovery, target resolution,
   credentials, protocol selection and per-device outcomes to CLI and TUI.
   Maintain cancellation, confirmation, bounded concurrency and localization.
6. **Verification:** add synthetic protocol and integration tests covering
   positive and negative paths, authentication, encryption, response status,
   identity, cancellation, multi-device isolation and secret redaction. Run
   `go test ./...`, `go test -race ./...`, `go vet ./...`, `go build ./...`
   and applicable documentation checks.
7. **Real-device acceptance:** test the claimed operations on real devices,
   record model and firmware compatibility, and verify Windows 10/11, Ubuntu
   and macOS x64. A reachable ISAPI address is required; discovery alone does
   not establish HTTP reachability.
8. **Release:** after all acceptance gates pass, build Windows, Linux and macOS
   x64 artifacts, verify packaged configuration and localization, publish
   SHA-256 checksums, release notes and compatibility matrix, and create a
   prerelease in the CDSRaBot repository.

## Release decisions and constraints

- Choose signing before publication. An unsigned beta needs explicit warnings;
  macOS Gatekeeper may require manual authorization. Signed artifacts require
  securely managed Windows and Apple signing credentials and macOS
  notarization.
- Do not publish a beta or claim interoperability without real-device
  acceptance. List unverified models and features as unsupported.
- Research captures and local SDK logs can contain credentials and must never
  be included in commits, fixtures, logs or release artifacts.
- Record compatibility results without passwords, keys, nonces, ciphertext or
  credential-bearing XML. Use synthetic values in automated tests.
