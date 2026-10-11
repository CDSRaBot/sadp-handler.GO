# Procedure Specification: P4.4 - ParseCLICommand
## Module: M4 - Command-Line Interface

## 1. Purpose

Parse the non-interactive `--ui=no` command form into a typed request before any workflow or network activity starts.

## 2. Go Contract

```go
type CLIRequest struct {
    Command string
    Options map[string]string
    Args    []string
}

func ParseCLICommand(args []string) (CLIRequest, error)
```

- `args` contains arguments after the executable name and after global `--ui`/`--lang` options have been removed by the process entry point.
- Reject empty arguments, unknown commands/options, duplicate options, missing values, contradictory options, and invalid values with an error and no usable request.
- Parsing performs no filesystem, network, prompt, or device operation.

## 3. Supported Command Forms

```text
app --ui=no discover --interface INDEX --timeout DURATION [--format text|json]
app --ui=no activate --device MAC --password-stdin [--email EMAIL]
app --ui=no network --device MAC --password-stdin (--dhcp | --address IPv4/PREFIX [--gateway IPv4])
app --ui=no language list
app --ui=no --export-locale-template PATH
```

- Discovery requires an explicit eligible interface index and a positive finite timeout in non-interactive mode. The timeout is a caller-selected session bound, not a protocol retry/timing claim. `--format` defaults to `text`.
- `activate` and `network` require one six-byte MAC and a password read as one line from standard input. Passwords are never accepted in options, arguments, environment variables, or diagnostics.
- Network configuration requires exactly one of `--dhcp` and `--address`; `--gateway` is allowed only with `--address`. CIDR defines the address and prefix.
- `language list` reports available locales. The startup locale is selected through global `--lang CODE`; non-interactive locale changes are not persisted.
- `--export-locale-template PATH` is mutually exclusive with subcommands and uses M5 `ExportLocaleTemplate`.
- `activate` and `network` may be parsed before their workflows are implemented, but execution must return an explicit protocol-gate error until challenge acquisition, command transport, response interpretation, and cancellation behavior are verified and specified.

## 4. Errors and Tests

Return explicit argument errors without echoing passwords or complete sensitive input. Test each grammar branch, defaults, required/duplicate/unknown options, mutually exclusive network options, finite positive timeout, malformed MAC/address/interface values, stdin password selection, command/export exclusivity, and rejection before side effects.

---

*Level 2 procedure specification.*
