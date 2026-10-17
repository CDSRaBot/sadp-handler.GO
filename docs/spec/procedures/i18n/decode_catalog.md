# Procedure Specification: P5.3 - DecodeLocaleCatalog
## Module: M5 - Localization

## 1. Purpose

Decode and validate one bounded flat JSON locale catalog.

## 2. Go Contract

```go
func DecodeLocaleCatalog(data []byte) (map[string]string, error)
```

- Reject empty input and input larger than 1 MiB with `ErrCatalogTooLarge` for the oversized case.
- Return a non-nil map and nil error only for one complete valid catalog.
- On failure return nil and an error; do not return a partial map.

## 3. Processing Requirements

1. Decode exactly one JSON object mapping string keys to string values. Reject arrays, scalars, nested objects/arrays, non-string values, malformed JSON, and trailing JSON values.
2. Reject duplicate keys rather than silently accepting the last occurrence.
3. Require every key and value to be non-empty and non-whitespace.
4. Do not require every canonical message key in a translated catalog; P5.4 applies fallback.
5. Do not log catalog contents or user-provided translation values.

## 4. Errors and Tests

Return the typed `ErrCatalogTooLarge` for size violations and preserve JSON parsing causes without including catalog contents. Test size boundaries, valid empty-object rejection, duplicate keys, nested/non-string values, malformed/trailing JSON, whitespace keys/values, and no partial result on failure.

---

*Level 3 atomic procedure specification.*
