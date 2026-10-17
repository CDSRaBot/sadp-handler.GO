# Procedure Specification: P5.6 - ListLocales
## Module: M5 - Localization

## 1. Purpose

Enumerate locale identifiers available for runtime selection.

## 2. Go Contract

```go
func ListLocales(filesystem fs.FS) ([]string, error)
```

- Reject a nil filesystem with an error.
- Return a sorted list containing built-in `en` and each external `locales/{code}.json` filename whose code passes P5.2.
- If the directory is absent or contains no external locale, return `["en"]` and the sentinel `ErrNoExternalLocales`; callers treat this as expected and continue with embedded English. Return nil and a wrapped filesystem error if the directory exists but cannot be read.
- Listing checks names only; catalog contents are validated by P5.1 when a locale is selected.

## 3. Processing Requirements

1. Read the `locales` directory using `fs.ReadDir`.
2. Ignore directories and non-JSON files. For JSON files other than `en.json`, remove the `.json` suffix and validate the code; invalid locale filenames produce an error rather than being offered.
3. Include `en` exactly once as the embedded locale. Never open or validate external `en.json`; the embedded catalog is canonical and shadows it.
4. Sort the result lexicographically and return a defensive slice.

## 4. Errors and Tests

`ErrNoExternalLocales` is a sentinel detected with `errors.Is`; its accompanying `["en"]` result is usable. Preserve other filesystem causes without logging directory contents. Test nil FS, missing/unreadable/empty directories, unrelated files, nested directories, invalid JSON filenames, external `en.json` being ignored, embedded English, ordering, and that malformed catalog contents are rejected on load rather than silently offered as verified catalogs.

---

*Level 3 atomic procedure specification.*
