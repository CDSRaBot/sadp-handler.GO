# Module Specification: M4 - Command-Line Interface

## 1. Purpose

Provide the user-facing command-line entry point and connect validated user requests to the application workflows. The CLI is an adapter, not a home for device protocol or business rules.

## 2. Responsibilities

- Define commands and flags for discovery, activation, and network configuration.
- Load and validate configuration, command-line arguments, and user-provided values.
- Invoke application workflows with a cancellable `context.Context`.
- Present device data and operation outcomes, including errors and partial results, in a form suitable for both interactive use and automation.
- Return documented process exit codes for success, invalid input, and operational failure.

The CLI must not perform socket setup, parse SADP XML, or construct device protocol payloads directly.

## 3. Go Structure and Dependencies

- Executable entry points belong under `cmd/`; shared orchestration remains in an application package rather than being duplicated across commands.
- Prefer the standard-library `flag` package unless the implementation selects a CLI dependency for a documented need.
- Keep output and terminal formatting separate from application logic so core workflows can be tested without a terminal.
- Use `log/slog` or the logging mechanism selected for the application; do not mix diagnostic logs with machine-readable command output.

## 4. Input, Output, and Error Contract

- Validate required values and mutually exclusive options before starting network operations.
- Support cancellation and propagate it to the application layer; do not report canceled work as successful.
- Report outcomes per device for bulk operations. A failure for one device must not hide successful or failed results for other devices.
- Write diagnostics to the designated error stream and command results to the designated output stream. Avoid leaking passwords, derived secrets, or activation payloads.
- Use stable, documented exit-code semantics. Return errors from command handlers to the executable boundary, where they are logged and translated into process status.

## 5. Testing Requirements

- Test argument and configuration validation, output formatting, exit-code mapping, and cancellation behavior.
- Use injected application dependencies or interfaces where needed to test CLI behavior without real devices or network access.
- Keep tests deterministic and avoid requiring interactive input.

---

*Level 1 module specification for the CLI layer.*
