# Procedure Specification: P5.5 - ResolveLocale
## Module: M5 - Localization

## 1. Purpose

Select the effective startup locale from the default, optional settings value, and optional command-line override.

## 2. Go Contract

```go
func ResolveLocale(
    settingsLocale string,
    cliLocale string,
    hasSettings bool,
    hasCLI bool,
) (string, error)
```

- Return the selected locale code or an error; return an empty code on failure.
- CLI locale takes precedence over settings. Settings take precedence over the default `en`.
- A supplied but empty or invalid higher-priority value is an error. A lower-priority value overridden by a valid higher-priority value is not validated and cannot block resolution.

## 3. Processing Requirements

1. Choose `cliLocale` when `hasCLI`; otherwise choose `settingsLocale` when `hasSettings`; otherwise choose `en`.
2. Validate only the selected code with P5.2 `ValidateLocaleCode`.
3. Do not read files, decode catalogs, mutate active state, or silently fall back when the effective requested value is invalid or unavailable.

## 4. Errors and Tests

Errors identify startup-locale resolution and do not echo invalid values. Test defaults, each precedence pair, absent versus explicitly empty values, invalid effective values, and invalid lower-priority values overridden by valid higher-priority values.

---

*Level 3 atomic procedure specification.*
