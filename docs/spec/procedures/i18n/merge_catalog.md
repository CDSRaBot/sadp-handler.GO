# Procedure Specification: P5.4 - MergeLocaleCatalog
## Module: M5 - Localization

## 1. Purpose

Create a defensive, complete message catalog by overlaying a valid partial translation on the canonical English catalog.

## 2. Go Contract

```go
func MergeLocaleCatalog(
    english map[string]string,
    translated map[string]string,
) (map[string]string, error)
```

- Reject nil or empty English catalogs, invalid keys/values, nil translations, invalid translated entries, and translation keys absent from English.
- Return a new non-nil map containing all English keys and translated values where present.
- Return nil and an error on failure; never return a partially merged catalog.

## 3. Processing Requirements

1. Validate all keys and values in both maps as non-empty and non-whitespace.
2. Reject translated keys not present in the canonical English catalog to surface spelling/key drift.
3. Copy every English entry into a new map, then overlay translated entries.
4. Do not mutate either input. The returned map aliases neither input and is treated as immutable after publication.
5. `translated` may be empty; English is then returned as a defensive copy.

## 4. Errors and Tests

Errors identify which catalog failed validation without including its contents. Test partial and complete translation, English fallback, unknown keys, invalid/nil maps, defensive copying, and input non-mutation.

---

*Level 3 atomic procedure specification.*
