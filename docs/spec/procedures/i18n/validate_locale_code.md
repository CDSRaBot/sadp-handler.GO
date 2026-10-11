# Procedure Specification: P5.2 - ValidateLocaleCode
## Module: M5 - Localization

## 1. Purpose

Validate one locale identifier before it is used to form a filesystem path or select a catalog.

## 2. Go Contract

```go
func ValidateLocaleCode(langCode string) error
```

- Accept one or more ASCII letters, digits, underscore, or hyphen.
- Reject empty strings, whitespace, periods, path separators, NUL, non-ASCII characters, and every other character.
- Return nil only for a valid locale code; return an error without returning a normalized replacement.
- Codes are case-sensitive and are not canonicalized.

## 3. Errors and Tests

The error identifies an invalid locale identifier without echoing untrusted path-like input. Do not use `panic`.

Test `en`, `en-US`, and `pt_BR`; empty and whitespace values; dot/traversal forms; slash and backslash; NUL; punctuation; and non-ASCII letters.

---

*Level 3 atomic procedure specification.*
