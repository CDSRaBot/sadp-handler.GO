# Module Specification: M4 - Command-Line Interface

## 1. Purpose

Provide the user-facing command-line entry point and connect validated user requests to the application workflows. The CLI is an adapter, not a home for device protocol or business rules.

## 2. Responsibilities

- Define one command catalog and shared typed command/result contracts for discovery, activation, network configuration, locale operations, help, version, and quit; retain separate TUI and CLI parsers.
- Start the TUI by default; `--ui=no` starts an interactive CLI REPL, while `--once` selects one-shot command execution from process arguments.
- Load and validate configuration, command-line arguments, and user-provided values.
- Resolve the initial language from built-in English, then settings, then command-line override; allow a built-in runtime command to switch locale for the current process.
- Keep the active locale catalog complete by falling back to embedded English for missing translation keys.
- Invoke application workflows with a cancellable `context.Context`.
- Present device data and operation outcomes, including errors and partial results, in a form suitable for both interactive use and automation.
- Return documented process exit codes for success, invalid input, and operational failure.

The CLI must not perform socket setup, parse SADP XML, or construct device protocol payloads directly.

## 3. Go Structure and Dependencies

- Executable entry points belong under `cmd/`; shared orchestration remains in an application package rather than being duplicated across commands.
- Use a hybrid interface: a full-screen interactive TUI enabled by default, an interactive standard CLI REPL selected by `--ui=no`, and one-shot CLI execution selected by `--ui=no --once`.
- Prefer the standard-library `flag` package for one-shot arguments, a dedicated safe tokenizer for REPL input, `fmt`, and `text/tabwriter` for plain tabular output. Use `encoding/json` for the shared machine-readable result mode; keep it free of terminal decoration.
- Use Bubble Tea (`github.com/charmbracelet/bubbletea`) as the sole TUI framework. Use Bubbles (`github.com/charmbracelet/bubbles`) for the device list, text input, and scrollable viewports, and Lip Gloss (`github.com/charmbracelet/lipgloss`) for layout and styling of the three screen zones described below. Keep these dependencies inside the presentation layer and do not add a second TUI or CLI framework. Mouse handling is optional and must degrade cleanly when the terminal does not support it.
- Keep output and terminal formatting separate from application logic so core workflows can be tested without a terminal.
- Use `log/slog` or the logging mechanism selected for the application; do not mix diagnostic logs with machine-readable command output.

## 4. Interaction and Presentation

- Select the interaction mode with the global `--ui=yes|no` option, which accepts only `yes` or `no` and defaults to `yes`. With the default or `--ui=yes`, start the TUI. With `--ui=no` and no `--once`, start the interactive CLI REPL. With `--ui=no --once`, parse and execute exactly one command supplied in process arguments, then exit. Reject `--once` with TUI mode, malformed global flags, and invalid values before starting application workflows.
- Treat `--lang CODE` and `--format text|json` as CLI-wide options, valid in both REPL and one-shot modes. `--format` defaults to `text` and selects the representation of every command result for that process. The REPL prompt/input UI is written to stderr; each command result, including command-level failures, is one JSON object on stdout in JSON mode. Diagnostic logs go to stderr and never contaminate result output.
- The command grammar is defined by [P4.3 ParseConsoleCommand](../procedures/ui/parse_command.md) and [P4.4 ParseCLICommand](../procedures/ui/parse_cli_command.md). The TUI console accepts only the documented application command set and must never invoke an OS shell or evaluate arbitrary input.
- P4.3 and P4.4 adapt their distinct input syntaxes to the shared typed command catalog in [P4.5 DispatchCommand](../procedures/ui/dispatch_command.md). The common dispatcher owns operation routing and result construction; parsers do not execute commands or perform I/O.
- In the CLI REPL, a command error is reported and the REPL continues. EOF exits cleanly after canceling owned work; Ctrl+C cancels the active command and returns to the prompt. `quit` cancels owned work and exits cleanly. In one-shot mode, the process exits after its sole command. Use stable exit codes: `0` success (including `quit`), `1` operational/command failure, `2` invalid invocation, and `130` interrupted one-shot execution.
- In TUI mode, enumerate eligible interfaces through M0 and let the user explicitly select one for discovery through M6. Report an actionable error when no eligible interface is available; do not silently choose an interface or fan out across all interfaces.
- The default TUI requires both terminal input and output. If either is redirected or is not a usable terminal, return an actionable error recommending `--ui=no` for REPL use or `--ui=no --once COMMAND` for non-interactive use; never silently switch modes based on terminal detection. The REPL requires terminal input but may redirect output; one-shot CLI permits redirected streams.
- After terminal validation, TUI mode enters the device-management screen. Discovery starts immediately for the selected interface through M6; receive listening remains active and one new inquiry probe is sent every 60 seconds until canceled. The TUI owns at most one such session: reselecting its interface does not start a duplicate, and changing interfaces cancels and joins the prior session before starting another. This cadence is application behavior, not an assertion about SADP requirements. Do not assume unverified response or UUID-correlation behavior.
- Select the mode once per process invocation. Do not switch between TUI and standard CLI modes in the same process: exit the current invocation and start another with the desired `--ui` value. Commands entered in the TUI command area invoke application workflows and return results to the TUI; they do not switch the process to standard CLI mode. Runtime `language` selection changes locale only, not interaction mode.
- Divide the full-screen TUI into three persistent zones: a top menu/status bar; a central area with a scrollable, filterable device list on the left and contextual information on the right; and a full-width command console at the bottom. Keep the focused device's details and device-reported data in the right panel. The bottom console accepts only documented application commands (never OS shell commands), retains command history and command-level results, and returns device/workflow details to the right panel.
- Keep command history in memory for the current process only. Never retain a password, challenge, derived secret, or other credential in history or rendered command text; collect secrets through a masked prompt and redact them from command results and diagnostics.
- Make keyboard operation complete and primary. With focus in the device list, Up/Down moves between rows and PageUp/PageDown scrolls; Tab/Shift+Tab changes the focused zone; Enter activates the focused menu item or device action; Space toggles a device's multi-selection; Esc cancels the current prompt/action or closes the current menu before exiting. In the top menu, Left/Right changes menu items and Up/Down navigates an open menu. In the detail panel, PageUp/PageDown scrolls. With focus in the command console, Up/Down navigates command history and Enter submits a command. Show available key bindings in the menu or status bar; no action may require a mouse or an undiscoverable shortcut.
- In terminals that support mouse reporting, provide secondary mouse interaction to focus a zone, select a device/menu item, and scroll a pane. Every mouse action must have a keyboard equivalent, and lack of mouse support must not impair operation.
- Present discovered devices as a scrollable list with a compact table-like row containing the identifying address, activation state, model, and other available summary fields. Support incremental local filtering, keyboard navigation, multi-selection, and actions on the selected device or selection.
- Keep multi-selection independent of the active filter: filtering hides rows but does not silently clear selected devices. Show the selection count and make the exact target set clear before a bulk action.
- Before a bulk activation or network change, validate the complete request and show the exact target devices and requested operation for explicit confirmation. Report each device's progress and outcome independently.
- In the TUI, resolve an operation target from the current multi-selection when it is non-empty; otherwise use the focused device. An explicit MAC in the command targets that device. Reject an empty or unresolved target set before collecting secrets or dispatching the operation.
- Show a contextual detail panel for the focused device or current operation, including available validated network/device fields, sanitized device response summaries, and per-device progress, success, or failure. Treat all device-supplied text as untrusted: escape terminal control/ANSI sequences, do not render raw protocol payloads, and never display passwords, challenges, or derived secrets. Reflow zones when terminal width or height is constrained; do not hide the command console or device details without an accessible way to focus them. Do not rely on color alone to convey state.
- Keep selection attached to a stable identity exposed by the device model rather than a row number when discovery updates or filtering reorder the list. Use the device MAC as the primary identity and disambiguate duplicate MAC values with sender information; preserve selection when possible and clearly handle a device that disappears from the latest snapshot.
- Use color only in interactive terminal presentation, detect terminal capability, respect `NO_COLOR` and `TERM=dumb`, and retain readable plain-text status labels. Never add ANSI styling to standard CLI JSON or other machine-readable output.
- On interactive cancellation or exit, restore terminal state and stop or cancel owned discovery/workflow activity.

## 5. Input, Output, and Error Contract

- Validate required values and mutually exclusive options before starting network operations.
- Support cancellation and propagate it to the application layer; do not report canceled work as successful.
- Report outcomes per device for bulk operations. A failure for one device must not hide successful or failed results for other devices.
- Write diagnostics to the designated error stream and command results to the designated output stream. Avoid leaking passwords, derived secrets, or activation payloads.
- Use stable, documented exit-code semantics. Return errors from command handlers to the executable boundary, where they are logged and translated into process status.
- Apply locale changes atomically: report a missing, empty, unreadable, or invalid catalog and leave the current locale unchanged. During startup, the current locale is built-in English; at runtime, it is the last successfully selected locale.
- Resolve startup precedence before loading: a command-line locale overrides a settings-file locale; an unavailable effective locale is an error and does not silently fall through to a lower-priority request.
- Treat embedded English as always available. When enumerating locales at startup or opening the runtime selection command, report an empty or unavailable external locales directory and do not offer a switch; keep English active at startup or preserve the last successfully selected locale at runtime.
- Use one locale identifier across the settings key `language`, command-line flag `--lang`, and built-in `language` selection command.

## 6. Testing Requirements

- Test argument and configuration validation, output formatting, exit-code mapping, and cancellation behavior.
- Use injected application dependencies or interfaces where needed to test CLI behavior without real devices or network access.
- Keep tests deterministic and avoid requiring an actual interactive terminal; test mode selection and non-TTY errors, command-console parsing/history, list filtering, selection and refresh behavior, detail rendering, plain and machine-readable output, keyboard behavior, optional mouse degradation, confirmation and per-device results, and terminal cleanup through injected inputs, outputs, and workflow dependencies.

Mode selection is specified in [P4.1](../procedures/ui/select_mode.md); list filtering in [P4.2](../procedures/ui/search.md); TUI command parsing in [P4.3](../procedures/ui/parse_command.md); and non-interactive command parsing in [P4.4](../procedures/ui/parse_cli_command.md).

---

*Level 1 module specification for the CLI layer. Localization is specified separately by [M5](./localization.md); discovery orchestration is specified separately by [M6](./discovery.md).*
