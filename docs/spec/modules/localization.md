# Module Specification: M5 - Localization

## 1. Purpose

Load, validate, resolve, and atomically switch locale catalogs without depending on terminal rendering or command parsing.

## 2. Responsibilities

- Treat embedded `locales/en.json` as the canonical message-key set and English fallback.
- Validate locale identifiers and flat JSON catalogs.
- Load a requested catalog and merge missing translations from English.
- Resolve startup locale precedence, enumerate available external locales, and export the English template.
- Replace the active locale code and catalog atomically only after a complete candidate has been loaded.

M5 does not parse CLI arguments, render UI text, persist runtime locale changes, or mutate catalogs after publication.

## 3. Go API Contract

```go
var ErrNoExternalLocales = errors.New("no external locales")
var ErrCatalogTooLarge = errors.New("locale catalog exceeds 1 MiB")
```

Callers distinguish the expected embedded-English-only case and oversized catalog input with `errors.Is`. Wrap these sentinels with operation context and preserve them.

Package paths are finalized with implementation. The procedure documents define these contracts:

```go
func ValidateLocaleCode(langCode string) error
func DecodeLocaleCatalog(data []byte) (map[string]string, error)
func MergeLocaleCatalog(english, translated map[string]string) (map[string]string, error)
func LoadLocale(filesystem fs.FS, langCode string, english map[string]string) (map[string]string, error)
func ResolveLocale(settingsLocale, cliLocale string, hasSettings, hasCLI bool) (string, error)
func ListLocales(filesystem fs.FS) ([]string, error)
func ExportLocaleTemplate(english map[string]string, destination string) error
func SwitchLocale(active *atomic.Pointer[LocaleState], candidate LocaleState) error

type LocaleState struct {
    Code     string
    Messages map[string]string
}
```

- `LoadLocale` composes validation, bounded catalog read/decode, and merge; it does not change active application state.
- `SwitchLocale` takes a defensive copy and publishes one complete immutable state through `atomic.Pointer`. Readers load one state pointer and must not mutate its map.
- English is supplied from the canonical embedded catalog; external `en.json` is not required at runtime.
- Locale selection, listing, and template export return explicit errors. A missing/invalid requested catalog is not a successful English fallback.

## 4. Data Flow and Error Policy

1. `ResolveLocale` chooses the effective code from default English, settings, then CLI override. Only the effective value is validated; an overridden invalid lower-priority value does not block startup.
2. `LoadLocale` returns a complete defensive map or an error and no map.
3. Startup initializes active state with validated English. Runtime switch loads the candidate first, then calls `SwitchLocale`.
4. A failed load or switch leaves the active state unchanged.
5. Partial non-English catalogs are valid and inherit missing English keys. Invalid present entries are errors.

Do not log catalog contents or untrusted translation values. Errors identify operation and locale/path without including catalog data. Do not use `panic` for invalid catalogs or ordinary filesystem errors.

## 5. Atomic Procedure Index

- [P5.1 LoadLocale](../procedures/i18n/load.md)
- [P5.2 ValidateLocaleCode](../procedures/i18n/validate_locale_code.md)
- [P5.3 DecodeLocaleCatalog](../procedures/i18n/decode_catalog.md)
- [P5.4 MergeLocaleCatalog](../procedures/i18n/merge_catalog.md)
- [P5.5 ResolveLocale](../procedures/i18n/resolve_locale.md)
- [P5.6 ListLocales](../procedures/i18n/list_locales.md)
- [P5.7 ExportLocaleTemplate](../procedures/i18n/export_template.md)
- [P5.8 SwitchLocale](../procedures/i18n/switch_locale.md)

---

*Level 1 module specification.*
