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

The current SADP procedure proposal is SHA-256 over the UTF-8 bytes of `password + challenge`, in that order, followed by lowercase hexadecimal encoding using Go's `crypto/sha256` and `encoding/hex`.

This proposal is not considered verified merely because it appears here. Before implementation is treated as device-compatible, compare it with an authoritative Hikvision protocol description or known-good test vectors. If the verified protocol specifies a different order, encoding, normalization, or algorithm, update this procedure and its dependent XML procedure together. Do not silently fall back to plaintext credentials when verification fails or the challenge is missing.

## 4. Errors and Tests

Use explicit validation errors for empty inputs. Tests must include known-answer vectors from a verified protocol source, output casing and length, and invalid inputs. Do not use production credentials in test fixtures.

---

*Level 3 atomic procedure specification.*
