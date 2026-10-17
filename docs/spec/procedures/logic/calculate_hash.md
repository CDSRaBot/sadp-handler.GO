# Procedure Specification: P3.5 - CalculateHash
## Module: M3 - Device Logic

## 1. Purpose

Calculate the challenge-response value required by the target device protocol. The algorithm and byte representation are protocol requirements, not interchangeable security choices.

## 2. Go Contract

```go
func CalculateHash(password, challenge string) (string, error)
```

- Reject empty password or challenge with an error.
- Return a lowercase hexadecimal digest and nil error only when the protocol-defined calculation succeeds.
- Do not log or include the password, challenge, or digest in errors.

## 3. Confirmed Algorithm Contract

Practical experiments confirm SHA-256 over the UTF-8 bytes of `password + challenge`, in that order, followed by lowercase hexadecimal encoding. This is the normative M3 compatibility contract. A Go implementation uses `crypto/sha256` and `encoding/hex`. The procedure consumes a challenge provided by its caller; challenge acquisition is a separate workflow responsibility. Never fall back to plaintext credentials when the challenge is missing.

## 4. Errors and Tests

Use typed M3 validation errors for empty inputs without disclosing inputs or digest. Tests must include known-answer vectors confirmed by practical experiments, output casing and length, and invalid inputs. Do not use production credentials in test fixtures.

---

*Level 3 atomic procedure specification.*
