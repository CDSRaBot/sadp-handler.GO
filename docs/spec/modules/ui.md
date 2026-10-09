# Module Specification: M4 - Command-Line Interface

## 1. Purpose

Provide the user-facing command-line entry point and connect validated user requests to the application workflows. The CLI is an adapter, not a home for device protocol or business rules.

## 2. Responsibilities

- Define commands and flags for discovery, activation, and network configuration.
- Start the TUI by default; allow standard CLI mode only through `--ui=no`.
- Load and validate configuration, command-line arguments, and user-provided values.
- Resolve the initial language from built-in English, then settings, then command-line override; allow a built-in runtime command to switch locale for the current process.
- Keep the active locale catalog complete by falling back to embedded English for missing translation keys.
- Invoke application workflows with a cancellable `context.Context`.
- Present device data and operation outcomes, including errors and partial results, in a form suitable for both interactive use and automation.
- Return documented process exit codes for success, invalid input, and operational failure.

The CLI must not perform socket setup, parse SADP XML, or construct device protocol payloads directly.

## 3. Go Structure and Dependencies

- Executable entry points belong under `cmd/`; shared orchestration remains in an application package rather than being duplicated across commands.
- Use a hybrid CLI: standard-library commands and output for scripts and one-shot operations, plus a full-screen interactive device-management TUI enabled by default.
- Prefer the standard-library `flag` package (with a `flag.FlagSet` per subcommand where useful), `fmt`, and `text/tabwriter` for command parsing and plain tabular output. Use `encoding/json` for an optional machine-readable output mode; keep it free of terminal decoration.
- The standard library has no terminal UI framework, raw-key input abstraction, terminal resize handling, mouse-event support, or portable screen management. Use a focused TUI dependency such as Bubble Tea with Bubbles and Lip Gloss for the full-screen interface; keep it inside the presentation layer. TUI mouse handling is optional and must degrade cleanly when the terminal does not support it.
- Keep output and terminal formatting separate from application logic so core workflows can be tested without a terminal.
- Use `log/slog` or the logging mechanism selected for the application; do not mix diagnostic logs with machine-readable command output.

## 4. Interaction and Presentation

- Select the interaction mode with the global `--ui=yes|no` option, which accepts only `yes` or `no` and defaults to `yes`. With the default or `--ui=yes`, start the TUI. With `--ui=no`, use the standard command-line mode, where commands and flags are supplied to the process and output remains suitable for scripting. Reject invalid values before starting application workflows.
- The functional areas currently identified are discovery, activation, network configuration, and runtime locale selection (`language`). The exact command names, grammar, arguments, and mappings for both standard CLI and the TUI command console are not yet defined; specify them in a follow-up CLI command contract before implementation. The command console must dispatch only that application command set and must never invoke an OS shell or evaluate arbitrary input.
- The default TUI requires both terminal input and output. If either is redirected or is not a usable terminal, return an actionable error recommending `--ui=no`; never silently switch modes based on terminal detection. Standard CLI mode selected with `--ui=no` may use redirected streams.
- After terminal validation, TUI mode enters the device-management screen and starts the discovery workflow supported by the application. It must not invent probe messages; probe transmission remains gated by the M1 protocol verification requirements.
- Select the mode once per process invocation. Do not switch between TUI and standard CLI modes in the same process: exit the current invocation and start another with the desired `--ui` value. Commands entered in the TUI command area invoke application workflows and return results to the TUI; they do not switch the process to standard CLI mode. Runtime `language` selection changes locale only, not interaction mode.
- Divide the full-screen TUI into three persistent zones: a top menu/status bar; a central area with a scrollable, filterable device list on the left and contextual information on the right; and a full-width command console at the bottom. Keep the focused device's details and device-reported data in the right panel. The bottom console accepts only documented application commands (never OS shell commands), retains command history and command-level results, and returns device/workflow details to the right panel.
- Keep command history in memory for the current process only. Never retain a password, challenge, derived secret, or other credential in history or rendered command text; collect secrets through a masked prompt and redact them from command results and diagnostics.
- Make keyboard operation complete and primary. With focus in the device list, Up/Down moves between rows and PageUp/PageDown scrolls; Tab/Shift+Tab changes the focused zone; Enter activates the focused menu item or device action; Space toggles a device's multi-selection; Esc cancels the current prompt/action or closes the current menu before exiting. In the top menu, Left/Right changes menu items and Up/Down navigates an open menu. In the detail panel, PageUp/PageDown scrolls. With focus in the command console, Up/Down navigates command history and Enter submits a command. Show available key bindings in the menu or status bar; no action may require a mouse or an undiscoverable shortcut.
- In terminals that support mouse reporting, provide secondary mouse interaction to focus a zone, select a device/menu item, and scroll a pane. Every mouse action must have a keyboard equivalent, and lack of mouse support must not impair operation.
- Present discovered devices as a scrollable list with a compact table-like row containing the identifying address, activation state, model, and other available summary fields. Support incremental local filtering, keyboard navigation, multi-selection, and actions on the selected device or selection.
- Keep multi-selection independent of the active filter: filtering hides rows but does not silently clear selected devices. Show the selection count and make the exact target set clear before a bulk action.
- Before a bulk activation or network change, validate the complete request and show the exact target devices and requested operation for explicit confirmation. Report each device's progress and outcome independently.
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

Mode-selection details are specified in [P4.1](../procedures/ui/select_mode.md); list filtering is specified in [P4.2](../procedures/ui/search.md).

---

*Level 1 module specification for the CLI layer.*
