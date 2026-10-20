# Phase 4: TDD tests for M4 CLI and TUI adapters

**GitHub Issue:** #19 — https://github.com/CDSRaBot/sadp-handler.GO/issues/19

## Context

Follow `sadp-handler.GO/docs/ru/DEVELOPMENT_GUIDE.md`, section
“Фаза 4: Написание тестов (разработка через тестирование)”, pp. 53–88.
The source contracts are:

- `sadp-handler.GO/docs/spec/modules/ui.md`
- `sadp-handler.GO/docs/spec/procedures/ui/parse_command.md`
- `sadp-handler.GO/docs/spec/procedures/ui/parse_cli_command.md`
- `sadp-handler.GO/docs/spec/procedures/ui/dispatch_command.md`
- `sadp-handler.GO/docs/spec/procedures/ui/search.md`
- `sadp-handler.GO/docs/spec/procedures/ui/select_mode.md`

## Scope and scenarios

Test the shared typed command/result model and separate REPL/one-shot/TUI parsers.
Cover valid commands, quoting/tokenization, `--ui`, `--once`, `--lang`, `--format`,
malformed flags, unsupported commands, mode conflicts, exit-code mapping, dispatch
routing, cancellation, partial results, search/filter behavior, and interface-selection
errors. Assert that parsers perform no I/O, TUI never invokes a shell, JSON output stays
on stdout, prompts/logs stay on stderr, and passwords/challenges/derived values never
enter history, output, or diagnostics.

## Deliverables

- `sadp-handler.GO/internal/ui/*_test.go` (or the package path chosen by the implementation)
- Test doubles and signature stubs as required.

## Acceptance criteria

- Tests use deterministic input/output buffers and fake application services.
- Positive, negative, cancellation, security-redaction, and exit-code cases are covered.
- Tests initially fail against explicit `not implemented` stubs and do not require a TTY.
- `go test -v -race ./sadp-handler.GO/internal/ui/...` is the validation command.
