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
