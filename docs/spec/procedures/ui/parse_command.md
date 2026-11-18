# Procedure Specification: P4.3 - ParseConsoleCommand
## Module: M4 - Command-Line Interface

## 1. Purpose

Parse one line from the TUI command console into a documented application command. This is a tokenizer/validator only; it does not execute a command or invoke a shell.

## 2. Go Contract

```go
type ConsoleCommand struct {
    Command Command
}

func ParseConsoleCommand(line string) (Command, error)
```

- Reject an empty line, unknown command, invalid argument count, or invalid argument value with an error and no usable command.
- Split on ASCII/Unicode whitespace using `strings.Fields`; quoting and shell expansion are not supported in the TUI command console.
- Return the same typed command variants used by CLI parsing and P4.5 dispatch.
- The parser never invokes an OS shell, performs interpolation, or treats input as executable code.

## 3. Grammar

```text
command        = discover | isapi-status | activate | network | language | help | version | quit | export-locale-template
discover       = "discover" [interface-index]
isapi-status   = "isapi-status" "--address" IP ["--port" PORT] ["--https"]
activate       = "activate" [device-mac]
network        = "network" [device-mac]
language       = "language" ("list" | locale-code)
help           = "help"
version        = "version"
quit           = "quit"
export-locale-template = "--export-locale-template" path
```

- `interface-index` is a non-negative decimal interface index. Omission means use the interface currently selected in the TUI; if none is selected, dispatch reports an actionable error.
- `isapi-status` reads the unauthenticated activation state at the explicitly
  supplied address and displays that identity is unverified.
- `device-mac` is a six-byte EUI-48 MAC accepted by M2 `NormalizeMAC`. If supplied, it is the sole target. If omitted, dispatch uses the current multi-selection, or the focused device when the selection is empty; an empty target set is an actionable error.
- `language list` displays available locales. `language CODE` requests an exact runtime locale switch through M5.
- `--export-locale-template PATH` exports the M5 locale template and is mutually exclusive with every other command.
- `activate` and `network` construct typed intents for the common dispatcher. With activation unavailable, display its explicit disabled/unsupported result before resolving targets, requesting confirmation or secrets, or starting network work. Once supported, collect credentials only through masked UI input; passwords, challenges, and derived secrets are never accepted in the command line.
- `help` lists the shared command catalog. `version` shows the application version/build identifier. `quit` cancels owned workflows and exits the TUI cleanly.
- SADP activation is disabled; the interactive `activate` and `network`
  workflows remain unavailable. Separate one-shot ISAPI paths fail closed
  without verified device profiles.
- After syntax validation, dispatch through P4.5; resolve targets for available mutation workflows. Parsing does not perform target lookup, show forms, collect secrets, or execute operations.

## 4. Errors and Tests

Errors contain the command/argument position and reason but do not echo secret-like values or the full command line. Test whitespace, empty input, every command and arity including locale-template export, invalid MAC/interface index/locale, extra arguments, shell metacharacters and quotes as ordinary non-tokenizing characters, command variant construction, and proof that parsing has no execution side effects.

---

*Level 2 procedure specification.*
