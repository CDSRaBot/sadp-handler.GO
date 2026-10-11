# Procedure Specification: P4.3 - ParseConsoleCommand
## Module: M4 - Command-Line Interface

## 1. Purpose

Parse one line from the TUI command console into a documented application command. This is a tokenizer/validator only; it does not execute a command or invoke a shell.

## 2. Go Contract

```go
type ConsoleCommand struct {
    Name string
    Args []string
}

func ParseConsoleCommand(line string) (ConsoleCommand, error)
```

- Reject an empty line, unknown command, invalid argument count, or invalid argument value with an error and no usable command.
- Split on ASCII/Unicode whitespace using `strings.Fields`; quoting and shell expansion are not supported.
- The parser never invokes an OS shell, performs interpolation, or treats input as executable code.

## 3. Grammar

```text
command        = discover | activate | network | language | help | quit
discover       = "discover" [interface-index]
activate       = "activate" [device-mac]
network        = "network" [device-mac]
language       = "language" ("list" | locale-code)
help           = "help"
quit           = "quit"
```

- `interface-index` is a non-negative decimal interface index. Omission means use the interface currently selected in the TUI; if none is selected, dispatch reports an actionable error.
- `device-mac` is a six-byte EUI-48 MAC accepted by M2 `NormalizeMAC`. Omission means use the focused device; if none is focused, dispatch reports an actionable error.
- `language list` displays available locales. `language CODE` requests an exact runtime locale switch through M5.
- `activate` starts a masked password prompt; `network` starts a configuration form. Passwords, challenges, and derived secrets are never accepted in the command line.
- `help` lists this grammar. `quit` requests TUI shutdown after owned workflows are canceled.
- Commands requiring activation or network protocol behavior must return an explicit unsupported/protocol-gate error until M3 defines and implements that operation.

## 4. Errors and Tests

Errors contain the command/argument position and reason but do not echo secret-like values or the full command line. Test whitespace, empty input, every command and arity, invalid MAC/interface index/locale, extra arguments, shell metacharacters, and proof that parsing has no execution side effects.

---

*Level 2 procedure specification.*
