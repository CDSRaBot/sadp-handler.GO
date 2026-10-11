# Procedure Specification: P5.1 - LoadLocale
## Module: M5 - Localization

## 1. Purpose

Load a requested locale catalog and return an independent, complete map with English fallback. This procedure does not mutate active locale state.

## 2. Go Contract

```go
func LoadLocale(
    filesystem fs.FS,
    langCode string,
    english map[string]string,
) (map[string]string, error)
```

- Reject nil filesystem and invalid/empty English catalogs with an error and no usable result.
- For `langCode == "en"`, return a defensive copy of validated `english` without reading an external file.
- For another code, return the selected catalog merged over English only if the file exists, can be read, and passes all validation.
- On any failure, return nil and an error; never return a success-shaped English fallback for an unavailable requested locale.
- The returned map must not alias `english` or decoder-owned mutable state. The caller treats it as immutable after publication.

## 3. Processing Requirements

1. Call P5.2 `ValidateLocaleCode`.
2. Validate the supplied canonical English catalog by applying the same entry rules as P5.3.
3. If the code is `en`, call P5.4 with English as both base and translation and return its defensive copy.
4. Otherwise read `locales/{langCode}.json` with slash-separated `fs.FS` paths. Wrap and return filesystem errors.
5. Call P5.3 `DecodeLocaleCatalog`, then P5.4 `MergeLocaleCatalog`.
6. Return the complete candidate only after every stage succeeds. Leave active locale changes to P5.8 `SwitchLocale`.

## 4. Errors and Tests

Errors identify validation, read, decode, or merge stage and requested locale without including catalog contents. Preserve wrapped causes. Do not use `panic`.

Test embedded-English loading without an external file, valid full/partial translated catalogs, missing/unreadable files, invalid locale identifiers, malformed catalogs, defensive copying, and that failures return no candidate catalog and do not change active state.

---

*Level 3 atomic procedure specification. Catalog validation/decoding is P5.3; locale-code validation is P5.2; merge/copy is P5.4.*
