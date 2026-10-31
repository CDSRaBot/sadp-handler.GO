# Logging Specification

## 1. Scope and implementation

Logging is a cross-cutting concern with one internal implementation package:
`internal/logging`. It uses the Go standard library `log/slog`; a third-party
logging framework is not required.

`internal/logging` owns logger construction, level mapping, handler options,
file lifecycle, rotation, run-boundary markers, common attributes, and
redaction. Application modules receive a `*slog.Logger` through typed
dependencies and must not create or replace a global logger.

Modules that emit records import:

```go
import "log/slog"
```

The application composition layer creates the logger before any workflow
starts. Pure transformations may omit the import and must state that their
caller owns logging.

## 2. File destination and run boundaries

Logs are written to the relative directory `.\logs`, resolved from the process
working directory. The directory is created when logging is initialized. A
failure to create or open the directory/file is an explicit startup error.

Each log file is limited to **5 MiB (5 * 1024 * 1024 bytes)**. Rotation occurs
before a write that would exceed the limit. Rotated files remain in `.\logs`
and use a deterministic, non-secret naming scheme. A record must not be split
between files.

At the beginning of every process run, before the first ordinary record, the
handler writes a blank line followed by a header containing the local date and
time at which logging started, for example:

```text

===== logging started: 2026-10-22 21:14:48 +05:00 =====
```

The header is written for every new process run, including when the current
file is reused. The timestamp is generated once during logger initialization.
The header is not emitted by individual modules.

The normal record format is structured text or JSON selected by the
application-level logging policy. Machine-readable command output must never
be written to the diagnostic log as a substitute for command output.

## 3. Levels and common fields

The configured `logging.level` maps to `slog.Level`:

| Configuration | Level |
|---|---|
| `debug` | `slog.LevelDebug` |
| `info` | `slog.LevelInfo` |
| `warn` | `slog.LevelWarn` |
| `error` | `slog.LevelError` |

Records should use stable event names and, where applicable, these safe
attributes:

`component`, `operation`, `stage`, `interface_index`, `interface_name`,
`sender`, `target`, `payload_size`, `datagram_size`, `count`, `duration`,
`error_kind`, `field`, `command`, `locale`, and `mode`.

Errors are recorded with a classified kind and stage. Error strings must not
be used as a substitute for stable fields.

The logging boundary must not discard handler or file errors. `logging.Event`
returns a handler failure to its caller; initialization and cleanup failures
are combined with `errors.Join` when both occur. Callers must either return
that error with context or handle it at the final application boundary.

## 4. Redaction and ownership

The following must never be logged: passwords, password-derived values,
challenges, tokens, complete XML, complete JSON configuration, complete
network payloads, or secret response fields. MAC addresses, serial numbers,
and device identifiers are logged only when required for correlation and must
use the project redaction policy.

The module that detects a low-level event logs its technical context. The
orchestrator logs the workflow result. The UI logs the command boundary.
Callers must not repeat the same event merely while wrapping an error.

## 5. Module logging points

| Module | Required points | Level guidance |
|---|---|---|
| M0 Adapter | interface enumeration, selection, multicast open/close, permission/bind/platform failures | info/debug; failures error |
| M1 Network | probe/datagram write, datagram read, size rejection, dispatch, cancellation, close | debug; rejected input warn/debug; operational failures error |
| M2 Parser | parse start only when diagnostic mode is enabled, accepted message, rejected message, size/XML/field failure | debug; malformed untrusted input warn/debug |
| M3 Logic | workflow start/end, validation rejection, request construction, per-device outcome | info; validation warn; operation failure error |
| M4 UI | mode selection, command parse/dispatch, target count, command completion/failure | info; invalid input warn |
| M5 Localization | catalog load, fallback, locale switch, decode/read failure | info/debug; failures warn/error |
| M6 Discovery | session/cycle start/end, probe, response, parse rejection, timeout, cancellation, cleanup | info/debug; failures error |
| M7 Configuration | defaults fallback, file load, validation, precedence, invalid configuration | info/debug; invalid present file error |
| M8 Application | logger initialization, run header, dependency composition, startup/shutdown | info; startup failure error |

## 6. Procedure logging points

Every procedure specification inherits the common logging contract in
[Common Procedure Policy](./procedures/POLICY.md). The required event boundary
for each procedure is:

| Procedure | Required event boundary |
|---|---|
| `GetInterfaces` | start, eligible count, enumeration failure |
| `GetLinuxMAC` | start, successful read, validation/read failure |
| `OpenMulticast` | validation, socket open, close/ownership failure |
| `LoadSettings` | defaults/file decision, decode/validation failure, success |
| `DiscoverOnInterface` | session start, probe cycle, response/rejection, end/cancel |
| `DecodeCatalog` | decode start/result, validation failure |
| `ExportTemplate` | export start/success, destination/write failure |
| `ListLocales` | enumeration start/result, directory failure |
| `Load` (localization) | locale selection, fallback, load failure |
| `MergeCatalog` | merge validation and result; never log catalog contents |
| `ResolveLocale` | requested/resolved locale, invalid code |
| `SwitchLocale` | switch start/success/failure |
| `ValidateLocaleCode` | rejection only when part of a user/config operation |
| `BuildActivationXML` | validation and build result; never log XML or secrets |
| `BuildNetworkXML` | validation and build result; never log XML or secrets |
| `CalculateHash` | no direct log; caller owns workflow logging |
| `ValidatePassword` | no direct log; caller owns workflow logging |
| `DatagramReceived` | size, sender, parse/dispatch result; never payload |
| `Listen` | listener start, receive loop end, cancellation, close failure |
| `SendDatagram` | target, size, write result; never payload |
| `SendProbe` | target, size, write result; never probe XML |
| `ParseAdvertisement` | size/sender/result; never source XML |
| `SanitizeMAC` | no direct log; caller owns input-operation logging |
| `DispatchCommand` | command start, target count, per-device/result summary |
| `ParseCLICommand` | command parse result and safe validation failure |
| `ParseCommand` | command parse result and safe validation failure |
| `Search` | query operation and result count; do not log raw secrets |
| `SelectMode` | requested/selected mode and terminal validation failure |

If a procedure is pure and has no safe diagnostic event, it emits no records
and its workflow caller owns logging. A procedure document may add a
procedure-specific logging rule only when it is stricter than this table and
the common policy.

## 7. Verification

Tests must use an injected `slog.Handler` or an isolated temporary log
directory. They must verify:

- level filtering;
- creation of `.\logs`;
- the 5 MiB pre-write rotation boundary;
- one blank line and one timestamped header per process initialization;
- no payload, XML, password, challenge, hash, or configuration secret leakage;
- stable event names and required safe attributes;
- separation of diagnostic logs from CLI text/JSON output.
