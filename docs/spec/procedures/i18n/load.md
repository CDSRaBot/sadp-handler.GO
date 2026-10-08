# Procedure Specification: P5.1 - LoadLocale
## Module: M4 - CLI Support (Localization)

## 1. Purpose

Load a flat JSON translation catalog for the CLI from a supplied filesystem, with an explicit English fallback when the requested locale cannot be loaded.

## 2. Go Contract

```go
type FallbackError struct {
    Language string
    Err      error
}

func LoadLocale(
    filesystem fs.FS,
    langCode string,
    english map[string]string,
) (map[string]string, error)
```

- On success, return the requested locale's validated messages and nil error.
- If the requested catalog cannot be resolved, read, decoded, or validated, return a copy of the validated English catalog and a `FallbackError` that wraps the cause. This makes fallback behavior explicit to callers.
- If the English fallback itself is invalid, return an error and no usable catalog.
- Returned maps must not alias the caller's `english` map or internal mutable state.
- `FallbackError` must support error inspection through `Unwrap`.

## 3. Processing Requirements

1. Validate `langCode` as a locale identifier containing only ASCII letters, digits, underscore, or hyphen; reject separators, traversal, and empty values.
2. Read `locales/{langCode}.json` using `fs.FS` and slash-separated paths.
3. Decode exactly one JSON object into `map[string]string`. Reject nested values, non-string values, malformed JSON, and trailing JSON content.
4. Validate fallback catalog entries as non-empty keys and string values.
5. On requested-catalog failure, return a defensive copy of the English catalog with an explicit `FallbackError`; do not silently claim the requested language loaded successfully.
6. If a requested translation key is absent, the CLI renders the key itself, as specified by the localization consumer.

## 4. Errors and Tests

Do not log catalog contents. Wrap filesystem and decoding errors while preserving the cause.

Tests should cover valid catalogs, unsupported locale names, missing files, malformed and trailing JSON, nested/non-string values, fallback copying, invalid fallback data, and `errors.As`/`errors.Unwrap` behavior for `FallbackError`.

---

*Level 3 atomic procedure specification.*
