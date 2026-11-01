# SADP wire profiles and optional workflows

SADP operations use explicit wire profiles. The existing `legacy-sha256`
profile preserves the current `SecurityResponse` contract. The `salted-aes`
profile requires an injected, verified sensitive-field encoder.

The implementation does not guess the SADP AES key length, KDF, IV, `Salt`
semantics, or `bSalt` encoding. Without a reproducible SADP vector, the
operation fails closed and no payload is sent.

Security questions are an independent opt-in operation. An empty list is not
serialized; three question/answer pairs are supported as a common shape, not
as a universal requirement. Hik-Connect is also a separate operation.

The concrete transport can use an operation-aware matcher to discard
unrelated datagrams until the shared deadline expires. Existing `Exchange`
transports remain compatible.

## Evidence status for `salted-aes` (Phase 1–2)

| Element | Observation | Source | Status |
|---|---|---|---|
| XML root | `<Probe>` | `SdkLog_20261028_214108754_w.log`, lines 188 and 200 | confirmed |
| Operation | `<Types>update</Types>` | same SDK log | confirmed |
| Error policy | `<PWErrorParse>true</PWErrorParse>` | same SDK log | confirmed |
| Identity | `<Uuid>` and `<MAC>` | same SDK log | confirmed |
| Secret field | `<Password bSalt="true">` contains opaque Base64-like text | same SDK log | shape confirmed, algorithm unknown |
| Device salt | `<Salt>` occurs in advertisements/responses | `SADP-Activation-Net-DHCP-HC.txt` | relationship to `Password` unproven |
| AES, key, KDF, IV, padding | not defined by available material | capture and SDK logs | implementation blocker |

The two SDK requests show that ciphertext changes between DHCP and static
operations, but do not identify why: network fields and UUID also change, and
there is no controlled pair with identical plaintext and all but one input held
constant. Passwords, salts, and ciphertext are not copied into the repository.

### Contract before a vector exists

1. `legacy-sha256` is explicitly selected and is not evidence for the `bSalt`
   format.
2. `salted-aes` requires an externally verified encoder and fails closed when
   none is available.
3. ISAPI `security`/`iv` parameters and its documented AES-CBC pipeline must
   not be transferred to raw SADP.
4. A production codec requires an approved vector distinguishing algorithm/key
   length, KDF, IV, padding, and encoding.

### Required Phase 3 input

A repeatable SDK/capture set is required for one device and firmware: repeat
the same password and request, then change exactly one input at a time
(network field, challenge/exchange code, or Salt), retaining XML, UUID, device
result, and timestamps. Secrets stay outside the repository; only a redacted
fixture and provenance are committed.
