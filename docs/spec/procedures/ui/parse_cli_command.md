# Procedure Specification: P4.4 - ParseCLICommand
## Module: M4 - Command-Line Interface

## 1. Purpose

Parse one CLI command, either from REPL input or the command arguments of a one-shot invocation, into the shared typed command model. Parsing validates syntax only; it does not execute commands or perform I/O.

## 2. Go Contract

```go
func ParseCLICommand(args []string) (Command, error)
func TokenizeCLIInput(line string) ([]string, error)
```

- `ParseCLICommand` receives tokens for exactly one command after executable and process-wide options (`--ui`, `--lang`, `--format`, `--once`) have been handled by the entry point.
- REPL input is tokenized by `TokenizeCLIInput`; one-shot arguments are already tokenized by the operating system and are passed directly to `ParseCLICommand`.
- Both paths produce the same command variant and validations for equivalent arguments.
- Reject empty input, unknown commands/options, duplicate singleton options, missing values, contradictory options, malformed values, and trailing unconsumed arguments. Return no usable command on failure.
- Neither function performs filesystem, network, prompt, logging, or device operations.

## 3. CLI Modes

```text
app [--lang CODE] [--format text|json] --ui=no
app [--lang CODE] [--format text|json] --ui=no --once COMMAND [COMMAND OPTIONS]
```

- `--ui=no` without `--once` starts the interactive CLI REPL. The process-wide options are removed before REPL commands are parsed.
- In REPL startup mode, process arguments may contain only global options; reject positional command arguments and direct the user to add `--once` rather than silently executing or ignoring them.
- `--once` requires exactly one command after it; it is valid only with `--ui=no`. The command is parsed and run once, then the process exits.
- `--lang CODE` and `--format text|json` apply to the whole invocation. `--format` defaults to `text`. `--format` is not repeated per REPL command.
- Interactive REPL prompt/input UI and diagnostic logs are written to stderr. Each completed command emits exactly one JSON result object on stdout in JSON mode, including command-level failures.
- `help`, `version`, and `quit` are available in both modes. In REPL, `quit` cancels owned work and exits with success. In one-shot mode, `quit` is a valid no-op command that exits with success.
- EOF exits the REPL cleanly after owned work is canceled. Ctrl+C cancels the active REPL command and returns to the prompt; interruption of a one-shot command exits with code 130.

## 4. Supported Command Forms

```text
discover --interface INDEX --timeout DURATION [--repeat COUNT]
activate --device MAC [--device MAC ...] --password-stdin [--email EMAIL] [--yes]
network --device MAC [--device MAC ...] --password-stdin (--dhcp | --address IPv4/PREFIX [--gateway IPv4]) [--yes]
language list
language CODE
help
version
quit
--export-locale-template PATH
```

- `discover` requires one eligible interface index and an explicit positive finite `--timeout` for each receive cycle. `--repeat` is an integer in `1..100` and defaults to `1`. Each cycle sends one probe immediately, receives continuously for its timeout, and starts the next cycle immediately after the preceding receive window. Receiving a device does not end a cycle. The total run is bounded by approximately `timeout × repeat` plus setup/cleanup overhead.
- `activate` and `network` accept one or more repeatable `--device MAC` options. Each MAC must be a valid six-byte EUI-48 address; reject duplicates after normalization. Require `--yes` when there are two or more target MACs. With one target, no confirmation prompt is issued in one-shot mode.
- `--password-stdin` marks standard input as the password source. In the REPL, read it with terminal echo disabled; in one-shot mode, read one password line from stdin, including when stdin is redirected. Do not read the password while these workflows are behind the protocol gate; after they become supported, read the secret only after command, target, and confirmation validation. Never accept passwords in an ordinary option value, command argument, environment variable, rendered history, or diagnostic.
- Network configuration requires exactly one of `--dhcp` and `--address`; `--gateway` is allowed only with `--address`. CIDR defines the address and prefix.
- `language list` returns locale identifiers from M5. `language CODE` requests an exact runtime locale switch in the current process and does not persist the selection; in one-shot mode it affects only that invocation.
- `help` returns the documented command catalog. `version` returns the application version/build identifier. `quit` is accepted in both modes as specified above.
- `--export-locale-template PATH` is a standalone command form mutually exclusive with all other commands and uses M5 `ExportLocaleTemplate`.
- `activate` and `network` remain present in the command catalog, but currently return a protocol-gate failure before reading secrets or doing network I/O until challenge acquisition, command transport, response interpretation, and cancellation behavior are verified and specified.
- The user-facing command synopsis and argument descriptions are maintained in [`docs/HELP.md`](../../../HELP.md); keep it synchronized with this grammar and P4.3.

## 5. REPL Tokenization

- Split unquoted tokens on ASCII/Unicode whitespace.
- Double quotes group whitespace into one token. Inside double quotes, backslash escapes only a double quote or another backslash. Outside double quotes, backslash is an ordinary character.
- Reject unmatched quotes and invalid escape sequences with an error that does not echo the complete input line.
- Do not implement shell expansion, variable substitution, command substitution, globbing, pipelines, redirection, or shell execution.

## 6. Errors and Tests

Return explicit parse errors without echoing passwords or complete sensitive input. Test both REPL tokenization and equivalent one-shot tokens; mode/global-option separation; one-shot command cardinality; all command forms; required timeout, default repeat, and repeat bounds; repeated/duplicate targets; `--yes` threshold; network option exclusivity; locale/help/version/quit/export forms; unknown and extra arguments; quoting, escaped quotes/backslashes, unmatched quotes and unsupported shell syntax; and proof that parsing has no side effects.

---

*Level 2 atomic procedure specification. Parsed commands use the common contract in [P4.5 DispatchCommand](./dispatch_command.md).*
