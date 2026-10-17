# SADP-handler — Command Help

[Русская версия](ru/HELP.md)

This document describes the user commands and options defined by the project specifications. The Go implementation has not started yet. The `activate` and `network` end-to-end workflows are not implemented and must fail explicitly without reading passwords or performing network operations until their result handling and cancellation contracts are implemented. Activation uses the accepted SADP XML request addressed by MAC; it is not an ISAPI HTTP operation.

## Starting the application

```text
sadp-handler [--ui=yes|no] [--lang CODE] [--format text|json]
sadp-handler --ui=no [--lang CODE] [--format text|json]
sadp-handler --ui=no [--lang CODE] [--format text|json] --once COMMAND [ARGS...]
```

- `--ui=yes|no` selects the interface. The default is `yes`, which starts the full-screen TUI. `no` starts the CLI REPL.
- `--once` runs exactly one command from process arguments and exits. It is valid only with `--ui=no`. Without `--once`, command arguments supplied at startup are not executed: CLI starts the REPL, and extra positional startup arguments are rejected.
- `--lang CODE` selects the application language for this invocation; `language list` shows available locale codes. This option overrides the language from settings.
- `--format text|json` selects the output format for all CLI results in the process. The default is `text`. In JSON mode, each completed command writes one JSON object to stdout; the REPL prompt and diagnostic messages go to stderr. This option applies to both REPL and one-shot CLI.

The TUI requires terminal input and output. The CLI REPL requires terminal stdin but permits redirected stdout. One-shot mode permits redirected streams.

## CLI commands

Enter commands at the CLI REPL prompt. To execute a command from process arguments, use `--ui=no --once`.

### `discover`

```text
discover --interface INDEX --timeout DURATION [--repeat COUNT]
```

Discover SADP devices through the selected network interface.

- `--interface INDEX` is the required index of an eligible network interface.
- `--timeout DURATION` is the required positive receive-window duration for each cycle, for example `5s`.
- `--repeat COUNT` is the number of cycles, from `1` to `100`; the default is `1`.

Each cycle sends one inquiry probe immediately, then receives advertisements for the full `--timeout`. Receiving a device does not end a cycle early. The result is a device list; repeated advertisements are aggregated by normalized MAC address and sender address. An empty list is a successful result.

```text
discover --interface 4 --timeout 5s
discover --interface 4 --timeout 3s --repeat 5
```

### `activate`

```text
activate --device MAC [--device MAC ...] --password-stdin [--email EMAIL] [--yes]
```

Request activation of one or more devices.

- Activation uses the selected SADP XML request addressed by target MAC; it is not an ISAPI HTTP request.
- `--device MAC` specifies a target EUI-48 MAC address. This option may be repeated; duplicates after normalization are rejected.
- `--password-stdin` selects stdin as the password source. In the REPL, input must not echo characters; in one-shot mode, one line is read. Do not pass the password as a command argument.
- `--email EMAIL` is an optional email address for the activation operation.
- `--yes` is required when two or more devices are specified.

Until the end-to-end workflow is implemented, this command returns an `unsupported` error before reading stdin, prompting for a secret, or performing network operations.

### `network`

```text
network --device MAC [--device MAC ...] --password-stdin (--dhcp | --address IPv4/PREFIX --gateway IPv4) [--yes]
```

Request network configuration for one or more devices.

- `--device MAC` specifies a target EUI-48 MAC address. The option may be repeated, but normalized duplicates are rejected.
- `--password-stdin` selects the password source; never pass the password in command arguments.
- Exactly one of `--dhcp` and `--address` is required.
- `--dhcp` enables DHCP and cannot be combined with `--address` or `--gateway`.
- `--address IPv4/PREFIX` specifies a static IPv4 address and prefix length, for example `192.168.1.20/24`.
- `--gateway IPv4` is required for a static configuration and is allowed only with `--address`.
- `--yes` is required for two or more target devices.

Until the end-to-end workflow is implemented, this command returns an `unsupported` error before reading stdin, prompting for a secret, or performing network operations.

### `language`

```text
language list
language CODE
```

- `language list` displays available locales.
- `language CODE` switches the current process to the exact locale. The change is not saved to settings; in one-shot mode it lasts only for that invocation.

### `help`

Display brief help for the command catalog.

### `version`

Display the application version and build identifier.

### `quit`

Exit the CLI REPL and cancel background work owned by it. In one-shot mode, this is a valid successful command with no additional action.

### `--export-locale-template PATH`

```text
--export-locale-template PATH
```

Export a locale template from the embedded English catalog. This is a standalone command and cannot be combined with other commands. An existing file is not overwritten.

## TUI commands

The TUI starts by default. Its command console accepts the same application commands but uses a separate, shorter grammar:

```text
discover [INTERFACE-INDEX]
activate [MAC]
network [MAC]
language list
language CODE
help
version
quit
--export-locale-template PATH
```

- If `discover` has no index, it uses the interface explicitly selected in the TUI; otherwise it uses the specified interface. The TUI sends the first probe immediately, then sends another every 60 seconds while continuing to receive advertisements until canceled.
- For `activate` and `network`, MAC is optional: without it, the selected devices are targeted, or the focused device if none are selected. Before a bulk operation, the TUI must show the targets and request confirmation.
- Quoting and shell syntax are not supported; input is split on whitespace. The console never invokes a shell.

## Process exit and errors

- CLI exit codes: `0` — success (including `quit`), `1` — command or operational failure, `2` — invalid invocation/arguments, `130` — one-shot interrupted with Ctrl+C.
- In the REPL, a command error does not terminate the process. EOF exits the REPL cleanly; Ctrl+C cancels the active command and returns to the prompt.
- Commands must not store or display passwords, challenges, derived secrets, or raw XML data.
