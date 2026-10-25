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

The activation policy requires 8 through 16 ASCII characters and at least two of these four classes: lowercase letters, uppercase letters, digits, and special characters. Lowercase and uppercase letters are restricted to `a-z` and `A-Z`; digits are restricted to `0-9`. Special characters are printable ASCII punctuation and symbols in byte ranges 33-47, 58-64, 91-96, and 123-126. Every input byte must belong to one of these sets. Spaces, control characters, Unicode letters/digits/symbols, combining marks, and invalid UTF-8 are rejected.

This is a compatibility policy derived from the local reference, not a device-wide protocol guarantee. Confirm it against the supported-device policy before claiming universal activation compatibility.

## 4. Errors, Safety, and Tests

- Error messages identify password-policy failure but never include the supplied password.
- Do not log, persist, or return the password from this function.
- Do not use `panic`.
- Test lengths 0, 7, 8, 16, and 17 ASCII characters; each single class; all six pairs of classes; accepted printable ASCII specials; Unicode letters/digits/symbols; whitespace; rejected control characters; invalid UTF-8; and synthetic values only.

---

*Level 3 atomic procedure specification.*
