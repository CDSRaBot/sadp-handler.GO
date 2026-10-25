# Common Procedure Policy

## 1. Scope and precedence

This policy applies to every atomic procedure specification in this directory
and its subdirectories. A procedure document defines its operation-specific
contract and inherits the common requirements below. Where a procedure,
module, or system contract differs, the more specific rule applies only when
it does not weaken this policy.

The normative specification order is:

1. [`SYSTEM.md`](../SYSTEM.md) for system-wide architecture, safety, and
   resource rules;
2. the owning module specification for package boundaries and public API;
3. the procedure document for operation-specific behavior;
4. this policy for requirements shared by all procedures;
5. [`logging.md`](../logging.md) for event boundaries, redaction, and log
   lifecycle.

## 2. Architecture and dependencies

- Procedures must remain within the responsibility of their owning module and
  must not perform work assigned to another module.
- A procedure that needs a replaceable dependency, platform seam, clock,
  filesystem, or transport receives it through the package-owned dependency
  boundary. It must not add mutable package-level test hooks or global state.
- Blocking network or I/O work accepts and propagates `context.Context`.
  Pure transformations do not add a context merely for consistency.
- Resources have one documented owner. Every successful acquisition has a
  defined close, transfer, or cleanup path.

## 3. Errors, security, and resource limits

- Validate untrusted input before use. This includes network data, CLI values,
  configuration, filenames, XML, JSON, addresses, ports, credentials, and
  protocol fields relevant to the procedure.
- Return explicit, inspectable errors. Preserve an underlying cause when the
  caller needs to classify it with `errors.Is` or `errors.As`; never use an
  error string as a programmatic contract.
- Do not use `panic` for ordinary invalid input, I/O, network, parsing, or
  device-operation failures.
- A failure returns no partially usable result unless the specific procedure
  explicitly documents a safe partial-result contract.
- Bound input size, concurrency, retries, and blocking operations where
  applicable. Network operations use cancellation and, when needed, a
  deadline or a caller-defined timeout.
- Error messages and results must not disclose passwords, password-derived
  values, challenges, tokens, complete payloads, complete XML/JSON documents,
  or other secret data.

## 4. Logging contract

Every procedure inherits its event boundary from the procedure table in
[`logging.md`](../logging.md). The procedure document need not duplicate
the table's imports, levels, attributes, redaction rule, or ownership.

- A procedure that emits records receives an injected `*slog.Logger` through
  its module's typed dependencies, imports only `log/slog`, and follows its
  table entry.
- A pure procedure emits no records unless its procedure document explicitly
  adds a safe diagnostic event. Its workflow caller owns logging.
- A procedure must never create, replace, or depend on a global logger.
- The detailed logging requirements in [`logging.md`](../logging.md)
  remain normative, including safe attributes, event ownership, and
  redaction.

## 5. Verification baseline

Tests derived from every procedure cover successful behavior, invalid input,
error classification, resource ownership, cancellation or timeout when
applicable, and secret redaction. Procedures that parse untrusted bytes add
boundary and fuzz tests. Network procedures use deterministic fakes and do
not require a live device or multicast network.

When implementation is intentionally deferred, a typed `not implemented`
stub and red tests are acceptable only during Phase 4. Before the procedure
is marked implemented, its tests must pass and its module-level integration
tests must cover the documented cross-procedure behavior.
