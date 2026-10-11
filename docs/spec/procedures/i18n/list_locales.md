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
- Return nil and an error if the directory is missing, unreadable, empty, or contains no valid non-English locale file.
- Listing checks names only; catalog contents are validated by P5.1 when a locale is selected.

## 3. Processing Requirements

1. Read the `locales` directory using `fs.ReadDir`.
2. Ignore directories and non-JSON files. For JSON files, remove the `.json` suffix and validate the code; invalid locale filenames produce an error rather than being offered.
3. Include `en` exactly once as the embedded locale, regardless of an external `en.json`.
4. Sort the result lexicographically and return a defensive slice.

## 4. Errors and Tests

Preserve filesystem causes without logging directory contents. Test nil FS, missing/unreadable/empty directories, unrelated files, nested directories, invalid JSON filenames, embedded English, ordering, and that malformed catalog contents are rejected on load rather than silently offered as verified catalogs.

---

*Level 3 atomic procedure specification.*
