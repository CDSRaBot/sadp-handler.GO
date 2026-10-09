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

## 3. Algorithm Contract and Verification Gate

The local Python reference computes SHA-256 over the UTF-8 bytes of `password + challenge`, in that order, followed by lowercase hexadecimal encoding. A Go implementation intended to match that reference would use `crypto/sha256` and `encoding/hex`.

This is Python-reference behavior, not independent verification of the device protocol. The reference does not negotiate a challenge; it accepts one from its caller. Before claiming compatibility, verify the challenge source and calculation against a known-good device exchange or authoritative description. Update this procedure and dependent XML requirements if the evidence differs. Never fall back to plaintext credentials when the challenge is missing.

## 4. Errors and Tests

Use explicit validation errors for empty inputs. Tests must include known-answer vectors from a verified protocol source, output casing and length, and invalid inputs. Do not use production credentials in test fixtures.

---

*Level 3 atomic procedure specification.*
