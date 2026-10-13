# Procedure Specification: P4.1 - SelectUIMode
## Module: M4 - Command-Line Interface

## 1. Purpose

Resolve the process interaction mode from the global `--ui` option and terminal availability before application workflows start.

## 2. Go Contract

```go
type InterfaceMode string

const (
    ModeTUI          InterfaceMode = "tui"
    ModeCLIInteractive InterfaceMode = "cli-interactive"
    ModeCLIOnce      InterfaceMode = "cli-once"
)

func SelectUIMode(
    value string,
    once bool,
    stdinIsTerminal bool,
    stdoutIsTerminal bool,
) (InterfaceMode, error)
```

- The command-line parser supplies `"yes"` when `--ui` is omitted. The option accepts exactly `"yes"` or `"no"`; other values, including an empty value, are invalid.
- `"yes"` with `once == false` selects `ModeTUI` only when both input and output are usable terminals. Otherwise return an actionable error recommending `--ui=no`.
- `"yes"` with `once == true` is invalid; `--once` is available only in CLI mode.
- `"no"` with `once == false` selects `ModeCLIInteractive` and requires usable terminal input. Output may be redirected; prompts are sent to stderr so stdout can carry command results. If stdin is not a terminal, return an actionable error recommending a one-shot invocation with `--once`.
- `"no"` with `once == true` selects `ModeCLIOnce` and permits redirected input and output.
- Return an error and no usable mode for invalid option values or unavailable required terminals. Do not silently fall back between modes.

## 3. Processing Requirements

1. Parse and validate `--ui` and `--once` before starting discovery, opening sockets, or invoking any other application workflow.
2. Do not infer the requested mode from terminal detection. Terminal detection only validates the explicitly selected/default mode.
3. Keep the resolved mode fixed for the process lifetime. Commands may invoke application workflows and display their results, but changing between TUI, CLI REPL, and one-shot CLI requires a separate process invocation.
4. Keep terminal detection outside this pure selection procedure; the caller supplies the terminal-capability booleans for the actual input and output streams.

## 4. Errors and Tests

Return distinguishable errors for invalid option combinations and missing required terminals, with no sensitive values. Test omitted-option defaulting at the argument-parser boundary, exact accepted values, invalid/empty values, TUI terminal checks, REPL stdin requirement with redirected stdout, one-shot redirected streams, `--once` invalid with TUI mode, and that no workflow starts after mode-selection failure.

---

*Level 2 atomic procedure specification.*
