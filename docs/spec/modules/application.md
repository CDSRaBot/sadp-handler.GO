# Module Specification: M8 - Application Composition

## 1. Purpose

Compose the validated M7 settings into runtime dependencies without exposing
the JSON file schema to protocol or presentation packages. M8 owns startup
precedence, construction of M5's filesystem, logging setup, and typed
dependency injection into M4 and M6.

## 2. Runtime Configuration

```go
type RuntimeConfig struct {
    UI           ui.RuntimeUIConfig
    Localization LocalizationRuntimeConfig
    Network      adapter.MulticastOptions
    Discovery    discovery.DiscoveryOptions
    Protocol     route.Strategy
    LogLevel     slog.Level
}

type LocalizationRuntimeConfig struct {
    FS fs.FS
}

type ApplicationDependencies struct {
    Config   RuntimeConfig
    Services ServiceSet
}

type ServiceSet struct {
    // Package-owned injected interfaces are defined by the consuming module.
    // Application-owned resources are released in reverse construction order.
    Closers []io.Closer
}

func Compose(settings config.Settings, baseDir string) (RuntimeConfig, error)
func BuildDependencies(runtime RuntimeConfig, services ServiceSet) ApplicationDependencies
```

The concrete fields of `ServiceSet` are package-owned injected interfaces
defined by the consuming modules; M8 must not invent duplicate protocol
contracts. The composition boundary and data flow are normative. M8 converts validated
M7 values into typed runtime values once; downstream modules do not decode JSON
or read `settings.json`.

`Compose` resolves the validated relative `localization.directory` against
`baseDir`, constructs an `fs.FS` for M5, converts duration strings to
`time.Duration`, builds `adapter.MulticastOptions`, maps
`protocol.strategy` to M9, and maps `logging.level` to `slog.Level`. It
rejects a path that escapes `baseDir` and returns a typed configuration error.

## 3. Precedence and Lifecycle

Startup applies defaults, then M7 file settings, then global CLI overrides, then
command arguments. M8 creates one immutable runtime configuration and passes it
through typed dependencies. No package-level configuration provider or mutable
global state is permitted.

The executable parses global UI and protocol options before composing the
runtime configuration. A supplied `--protocol=sadp_first|isapi_only` overrides
the validated file value; command parsing is repeated only at the UI boundary
using the already-resolved runtime defaults. The process context is canceled
on interrupt or termination so owned discovery and workflow operations can
release their resources.

M8 configures logging before workflows start. The configured logger must not
write secrets, packet payloads, passwords, challenges, or complete XML.

Logging is implemented by `internal/logging` using the standard-library
`log/slog`. M8 creates the logger once and injects `*slog.Logger` into the
modules that need it. The logger writes to `.\logs`; the directory is created
at startup and a failure to create or open it aborts startup. Each file is
limited to 5 MiB and is rotated before an oversized write. At initialization
the handler writes a blank line and a timestamped `logging started` header
before the first ordinary record. The header is emitted once per process run,
not by individual modules. See [Logging Specification](../logging.md) for the
module and procedure event contract.

## 4. Cross-Platform Composition

M8 supplies the same typed `adapter.MulticastOptions` on Windows, Linux
(including Termux where supported), and macOS. Platform-specific socket
behavior remains behind M0 seams. Unit tests use injected platform openers;
the acceptance gate additionally requires multicast setup and discovery
verification on each supported OS where a test device/network is available.
M8 injects M9's strategy and protocol dependencies without decoding settings in
M4 or mutating host routes.

Mutation services remain injectable at this boundary. When activation or
network services are supplied, M4 dispatches commands through those services.
When they are absent, M4 returns an explicit safe failure and does not
construct protocol payloads. Application-owned closers registered in
`ServiceSet.Closers` are released in reverse order after the UI exits.
