# Procedure Specification: P3.0 - ValidatePassword
## Module: M3 - Device Logic

## 1. Purpose

Validate one activation password against the local Python-reference compatibility policy without logging or retaining it.

## 2. Go Contract

```go
func ValidatePassword(password string) error
```

- Return nil only when the password satisfies the policy below.
- Return a validation error for an invalid password; do not return a normalized or replacement value.
- Empty input is invalid. This pure validation function does not perform I/O and does not need a context.

## 3. Compatibility Policy

The Python reference requires 8 through 16 characters and at least two of these four classes: lowercase letters, uppercase letters, digits, and non-alphanumeric characters. For a deterministic Go contract, count Unicode code points (runes), classify letters with `unicode.IsLower`/`unicode.IsUpper`, digits with `unicode.IsDigit`, and special characters as runes that are neither a Unicode letter nor a Unicode digit. Each rune contributes to at most one class. Reject Unicode control characters; spaces and other non-letter/non-digit runes count as special characters.

This is a compatibility policy derived from the local reference, not a device-wide protocol guarantee. Confirm it against the supported-device policy before claiming universal activation compatibility.

## 4. Errors, Safety, and Tests

- Error messages identify password-policy failure but never include the supplied password.
- Do not log, persist, or return the password from this function.
- Do not use `panic`.
- Test lengths 0, 7, 8, 16, and 17 code points; each single class; all six pairs of classes; Unicode letters/digits; combining marks; spaces; rejected control characters; and synthetic values only.

---

*Level 3 atomic procedure specification.*
