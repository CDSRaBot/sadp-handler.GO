# Phase 4: TDD tests for M5 Localization

**GitHub Issue:** #20 — https://github.com/CDSRaBot/sadp-handler.GO/issues/20

## Context

Follow `sadp-handler.GO/docs/ru/DEVELOPMENT_GUIDE.md`, section
“Фаза 4: Написание тестов (разработка через тестирование)”, pp. 53–88.
The source contracts are:

- `sadp-handler.GO/docs/spec/modules/localization.md`
- `sadp-handler.GO/docs/spec/procedures/i18n/validate_locale_code.md`
- `sadp-handler.GO/docs/spec/procedures/i18n/decode_catalog.md`
- `sadp-handler.GO/docs/spec/procedures/i18n/merge_catalog.md`
- `sadp-handler.GO/docs/spec/procedures/i18n/load.md`
- `sadp-handler.GO/docs/spec/procedures/i18n/resolve_locale.md`
- `sadp-handler.GO/docs/spec/procedures/i18n/list_locales.md`
- `sadp-handler.GO/docs/spec/procedures/i18n/export_template.md`
- `sadp-handler.GO/docs/spec/procedures/i18n/switch_locale.md`

## Scope and scenarios

Test locale-code validation, bounded JSON decode, flat string catalogs, malformed JSON,
wrong value types, duplicate/unknown keys, missing English keys, merge precedence and
defensive copies, load from `fs.FS`, missing/oversized catalogs, precedence
default/settings/CLI, locale listing, template export, atomic switch, immutable
published state, and unchanged active state after every failed load/switch. Assert
`errors.Is` for `ErrNoExternalLocales` and `ErrCatalogTooLarge`, and no catalog contents
in errors/logs.

## Deliverables

- `sadp-handler.GO/internal/i18n/*_test.go` (or implementation package path)
- In-memory `fs.FS` fixtures and signature stubs as required.

## Acceptance criteria

- Tests are isolated from the host filesystem and run deterministically under `-race`.
- Both success and every specified negative/error-preservation path are asserted.
- Tests initially fail against explicit `not implemented` stubs.
- `go test -v -race ./sadp-handler.GO/internal/i18n/...` is the validation command.
