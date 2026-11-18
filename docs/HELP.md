# SADP-handler — Command Help

[Русская версия](ru/HELP.md)

This document describes the user commands and options defined by the project specifications. Discovery is implemented. The executable does not yet wire end-to-end `activate` and `network` services, so these commands fail explicitly without reading passwords or performing network operations. SADP activation is disabled; the planned ISAPI replacement is not yet available in the executable.

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

### `isapi-discover`

```text
sadp-handler --ui=no --once isapi-discover --address IP --device MAC --username USER --password-stdin [--port PORT] [--https] [--basic]
```

Read device information from one explicitly specified IP address and verify its
MAC using authenticated ISAPI. This read-only command is available only in
one-shot mode; it neither changes the device nor discovers unknown addresses.
The address must be a non-link-local unicast IP literal, and the MAC an EUI-48.
The default port is 80, or 443 with `--https`. TLS certificates are verified
using the system trust store. `--basic` explicitly enables Basic authentication
and requires `--https`; Digest is preferred when offered.

Redirect one UTF-8 password line to stdin (maximum 4096 bytes); terminal stdin
is rejected. Do not put passwords in arguments, shell history, or logs. For
example, use a securely managed secret file with input redirection:

```text
sadp-handler --ui=no --once isapi-discover --address 192.0.2.10 --device 00:01:02:03:04:05 --username admin --password-stdin --https < password-file
```

The JSON result contains `isapi_device` with the verified MAC, address, model,
serial number and firmware version when provided. Physical-device compatibility
has not yet been established.

### `isapi-status`

```text
sadp-handler --ui=no --once isapi-status --address IP [--port PORT] [--https]
# TUI console or CLI REPL:
isapi-status --address IP [--port PORT] [--https]
```

Read the activation state of an explicitly specified non-link-local unicast IP
using `GET /SDK/activateStatus`. This read-only command does not read
a password or activate a device. Ports default to 80 for HTTP and 443 for
HTTPS; HTTPS uses the system trust store.

The result reports `active` or `inactive` and explicitly marks the device
identity as unverified. JSON uses `isapi_status` with `address`, `state`, and
`identity_verified: false`. This unauthenticated status is only a preflight
signal: it must not be used as evidence of device identity or authorization
to perform a mutation. Physical-device compatibility is not yet established.

### `isapi-activate`

```text
sadp-handler --ui=no --once isapi-activate --address IP --device MAC --username USER --password-stdin [--port PORT] [--https] [--basic]
```

This one-shot ISAPI activation path is **disabled in the executable**: no
trusted device-specific identity and activation profile has been established.
It fails before reading the password or contacting the device. It is not a
claim of physical-device compatibility.

The path is exercised with synthetic profiles only. If a verified profile is
provided in a future version, the JSON `isapi_activation` result reports
`attempted` and `verified`. An attempted PUT with `verified: false` means the
device state may be unknown; inspect the device before retrying. Basic
authentication requires explicit `--basic` and `--https`, and password input
is redirected, as for `isapi-discover`.

### `isapi-network`

```text
sadp-handler --ui=no --once isapi-network --address IP --device MAC --username USER --password-stdin --interface-id ID --yes --new-address IPv4/PREFIX --gateway IPv4 [--port PORT] [--https] [--basic]
sadp-handler --ui=no --once isapi-network --address IP --device MAC --username USER --password-stdin --interface-id ID --yes --dhcp --discovery-interface INDEX [--port PORT] [--https] [--basic]
```

This one-shot ISAPI network path is **disabled in the executable** until a
device-specific network profile and authenticated model/firmware gate are
verified. It rejects the command before password input or network access.
`--yes` explicitly confirms the operation; DHCP requires an eligible host
interface for SADP-assisted reappearance discovery. This does not change host
network settings.

The workflow has synthetic tests only. If enabled with a verified profile in a
future version, `isapi_network` reports `attempted`, `verified`,
`restart_required`, and, when found, `observed_address`. An attempted but
unverified change may have succeeded; inspect the device before retrying. A
required restart is reported but never performed automatically. The returned
address is checked against authenticated device identity and configuration.

### `activate`

```text
activate --device MAC [--device MAC ...] --password-stdin [--email EMAIL] [--yes]
```

Request activation of one or more devices.

- SADP activation is disabled. The separate `isapi-activate` path is wired but
  remains unavailable without a verified device profile.
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
- While activation is unavailable, the TUI and CLI REPL report disabled SADP
  activation (or the unavailable ISAPI workflow in `isapi_only` mode) without
  requiring a selected device, confirmation, password input, or network access.
  One-shot commands still require the documented CLI syntax.
- Quoting and shell syntax are not supported; input is split on whitespace. The console never invokes a shell.

## Process exit and errors

- CLI exit codes: `0` — success (including `quit`), `1` — command or operational failure, `2` — invalid invocation/arguments, `130` — one-shot interrupted with Ctrl+C.
- In the REPL, a command error does not terminate the process. EOF exits the REPL cleanly; Ctrl+C cancels the active command and returns to the prompt.
- Commands must not store or display passwords, challenges, derived secrets, or raw XML data.
