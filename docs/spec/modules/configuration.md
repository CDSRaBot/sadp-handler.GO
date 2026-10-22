# Module Specification: M7 - Configuration

## 1. Purpose

Load the external `settings.json`, validate every setting, apply documented defaults, and provide one immutable typed configuration to the application. The file is optional: when it is absent, the same defaults as the checked-in template are used. A present but unreadable, malformed, or invalid file is an explicit startup error; it is never silently replaced by defaults.

`settings.json` is the single source for baseline runtime settings. Command-line options are an intentional higher-priority override and must not be written back to the file.

## 2. File Format

The file is valid JSON and contains `_comments` plus the setting groups `ui`, `localization`, `network`, `discovery`, and `logging`. JSON has no native comments, so `_comments` is mandatory documentation metadata: it contains one entry for every setting key and describes all accepted values and boundary conditions. Unknown keys, including unknown comment keys, are errors. `_comments` is ignored after structural validation.

The template is [settings.json](../../../settings.json). Relative paths are resolved from the process working directory and must remain relative; secrets are not accepted in settings.

## 3. Go API Contract

```go
type Settings struct {
    UI            UISettings
    Localization  LocalizationSettings
    Network       NetworkSettings
    Discovery     DiscoverySettings
    Logging       LoggingSettings
}

type ConfigurationErrorKind string

const (
    ConfigurationReadError             ConfigurationErrorKind = "read"
    ConfigurationDecodeError           ConfigurationErrorKind = "decode"
    ConfigurationUnknownKeyError       ConfigurationErrorKind = "unknown_key"
    ConfigurationMissingCommentError   ConfigurationErrorKind = "missing_comment"
    ConfigurationInvalidValueError     ConfigurationErrorKind = "invalid_value"
    ConfigurationInvalidCombinationError ConfigurationErrorKind = "invalid_combination"
)

type ConfigurationError struct {
    Kind  ConfigurationErrorKind
    Field string
    Err   error
}

func (e *ConfigurationError) Error() string
func (e *ConfigurationError) Unwrap() error

func Load(path string) (Settings, error)
func Defaults() Settings
```

`Load` reads one file, validates the complete document, applies defaults only to omitted or explicitly documented zero values, and returns no usable settings on error. `Load("")` uses `settings.json` in the working directory. The caller owns no open file after return.

Defaults and constraints:

| Key | Default | Valid values |
|---|---:|---|
| `ui.mode` | `yes` | `yes` or `no` |
| `ui.format` | `text` | `text` or `json` |
| `ui.language` | `en` | non-empty locale code accepted by M5 |
| `ui.once` | `false` | boolean; `true` requires CLI mode |
| `localization.directory` | `locales` | non-empty relative path |
| `network.multicast_group` | `239.255.255.250` | multicast IPv4 address |
| `network.multicast_port` | `37020` | integer `1..65535` |
| `network.multicast_ttl` | `64` | integer `1..255` |
| `network.max_sadp_datagram_size` | `1500` | integer `1..65507`; zero means default |
| `discovery.probe_interval` | empty | JSON string: empty for bounded, or positive Go duration for continuous |
| `discovery.cycle_timeout` | `5s` | JSON string: empty for continuous, or positive Go duration for bounded |
| `discovery.repeat_count` | `1` | `0` for continuous, or integer `1..100` for bounded |
| `logging.level` | `info` | `debug`, `info`, `warn`, `error` |

The discovery settings are resolved into exactly one M6 policy: continuous mode uses `probe_interval > 0`, `cycle_timeout == 0`, `repeat_count == 0`; bounded mode uses `probe_interval == 0`, `cycle_timeout > 0`, and `repeat_count in 1..100`. The template's `repeat_count=1` is the default bounded policy; the TUI may explicitly select the continuous policy.

Duration values are JSON strings parsed with `time.ParseDuration`. An empty
duration string resolves to zero. Numeric JSON values are invalid.

## 4. Precedence and Security

Startup precedence is defaults, then `settings.json`, then command-line flags, then command-specific arguments. M5 resolves locale precedence as built-in English, settings language, and CLI `--lang`. CLI values are validated after parsing and before workflows start.

Do not log the file contents, passwords, tokens, challenge values, or protocol payloads. Preserve filesystem, JSON, and validation causes in `ConfigurationError`; callers branch on `Kind` with `errors.As` and inspect the cause with `errors.Is`. Do not accept environment variables or undocumented fallback files as another settings source.

`logging.level` is consumed by M8 to configure `slog.HandlerOptions.Level`:
`debug`, `info`, `warn`, and `error` map to the corresponding `slog.Level`.

## 5. Tests

Test missing-file defaults, valid template loading, every documented boundary, malformed JSON, missing `_comments`, unknown keys, invalid durations/paths/addresses, precedence, and secret/payload redaction. Use temporary files and deterministic inputs; no network or terminal is required.
