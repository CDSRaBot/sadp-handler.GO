# ISAPI Encrypted User Fields — Protocol Note

**Status:** Trusted design reference; device- and firmware-capability gated  
**Scope:** Challenge handling and AES-CBC protection of sensitive user XML fields  
**Related procedure:** [SADP/ISAPI trusted pipelines](../procedures/SADP_ISAPI_TRUSTED_PIPELINES.md), Pipelines F–G

## Summary

For a device that advertises encrypted user-management support, sensitive user
fields (for example, `<password>`) are transformed before being sent to
`/ISAPI/Security/users` or `/ISAPI/Security/users/{id}`. The documented
transformation is:

```text
UTF8(value)
  -> Base64(UTF8(value))
  -> PKCS#7 padding to the AES block size (16 bytes)
  -> AES-CBC encryption
  -> Base64(ciphertext) in the XML field
```

The generic documented request form carries the selected scheme and a
Base64-encoded 16-byte IV in its query:

```text
?security=1&iv={url_encoded_base64_iv}  # AES-128-CBC, 16-byte key
?security=2&iv={url_encoded_base64_iv}  # AES-256-CBC, 32-byte key
```

Query values must be encoded with a URI/query builder. Do not concatenate a
standard Base64 value containing `+`, `/`, or `=` into a raw URI.
The analyzed recovery-question request uses a 32-character hex IV instead;
the cached web UI code for that endpoint also uses hex key, IV, and ciphertext
representations. This is a device/endpoint-specific wire format, not a
contradiction of the generic representation above.

## Challenge and request flow

When the device advertises the security-challenge procedure, obtain and
validate the short-lived challenge before the protected operation:

```text
POST /ISAPI/Security/challenge
```

The response may contain a challenge, salt, validity indicator, and other
device-specific metadata. Its semantics and binding are device-specific; do
not assume this endpoint is required for every encrypted user operation. In
the captured browser flow, the session-login challenge feeds a separate login
proof; it is not an input to the recovery-question AES key derivation. Bind a
challenge to an operation only as specified by the device or SDK.

An illustrative response shape is:

```xml
<SecurityChallenge version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <challenge>{device-challenge}</challenge>
  <salt>{device-salt}</salt>
  <isSessionValid>true</isSessionValid>
</SecurityChallenge>
```

The concrete response schema, authentication requirements, challenge lifetime,
and binding mechanism are device/firmware-specific.

## HAR and cached web UI analysis

Sources: user-provided `192.168.1.64.har` (104 exchanges),
`192.168.1.64-loading.har` (44 exchanges), and
`192.168.1.64-non-activated.har` (43 exchanges), analyzed locally. The
non-activated HAR captures the login page and startup checks; its API calls
include `GET /SDK/activateStatus` and
`GET /ISAPI/Security/extern/capabilities`, but no activation challenge or
activation `PUT`. Cached script bodies are present despite `304 Not Modified`
statuses. The overlapping crypto-related scripts are byte-identical between
the non-activated and activated loading captures, so their client crypto logic
does not change with activation state. Secret values are omitted.

Read-only checks against the camera returned `Activated=false` and
`isSupportWithSecurityQuestion=true`. An unauthenticated
`GET /ISAPI/Security/sessionLogin/capabilities?username=admin` returned
`iterations=100` and `isIrreversible=true`; sensitive challenge, salt, and
session identifiers were not retained. An unauthenticated
`GET /ISAPI/Security/capabilities?username=admin` returned `403`, which does
not establish whether the endpoint is available after authentication. The
activation state remained false after these reads.

### Recovery-question AES key

The captured `PUT /ISAPI/Security/questionConfiguration?security=1&iv=...`
contains a 32-character hex IV and hex ciphertext for the `password` and
`answer` fields. The UI's `websdk.js`, `utils.js`, and `base64.js` code defines
the following flow:

```text
irreversible = security-capabilities.isIrreversible
salt         = security-capabilities.salt
iterations   = security-capabilities.keyIterateNum

base = password
if irreversible:
    base = SHA256(username || salt || password)

h = SHA256(base || "AaBbCcDd1234!@#$")
repeat iterations - 1 times:
    h = SHA256(h)

aes_key_hex = first 32 hexadecimal characters of h
iv_hex      = MD5(decimal_current_time_in_milliseconds)
```

For the captured `security=1` path, the first 32 hex characters are parsed as
a 16-byte AES-128 key. The AES key uses the salt from
`/ISAPI/Security/capabilities`; the login challenge and the salt returned by
`/ISAPI/Security/sessionLogin/capabilities` are not inputs to this KDF.

For each field marked `security1` by the UI (here, `answer` and `password`),
the helper XML-escapes `&`, `<`, and `>`, UTF-8/Base64-encodes that text, then
applies AES-CBC with PKCS#7 padding. It parses the key and IV from hex and
places the ciphertext bytes' hex representation in the XML field:

```text
escaped = XML_escape(value)
plaintext = Base64(UTF8(escaped))
ciphertext_hex = Hex(AES_128_CBC_PKCS7(aes_key_hex, iv_hex, plaintext))
```

This is a confirmed implementation in the captured web UI, not a universal
ISAPI KDF. The UI's IV is an MD5 digest of the current millisecond timestamp:
it is predictable and is recorded here only for compatibility analysis, not
as a recommended IV generator. Use a cryptographically random 16-byte IV in a
new client where the device accepts it.

### Session-login proof is separate

The browser first calls
`GET /ISAPI/Security/sessionLogin/capabilities`, then submits a 64-character
hex `password` field to `POST /ISAPI/Security/sessionLogin`. The cached
`utils.js` defines a distinct proof using that endpoint's `challenge`, `salt`,
and `iterations`:

```text
if isIrreversible:
    proof = SHA256(username || session_salt || password)
    proof = SHA256(proof || challenge)
    repeat iterations - 2 times:
        proof = SHA256(proof)
else:
    proof = SHA256(password) || challenge
    repeat iterations - 1 times:
        proof = SHA256(proof)
```

The returned proof is hex text. This login proof must not be confused with the
AES key derivation above. In the capture, the session challenge and salt are
hex strings of 16 and 32 bytes respectively; the security capabilities
advertise `keyIterateNum=100`.

### Activation challenge and encryption

The activation HAR shows `POST /ISAPI/Security/challenge` with `PublicKey/key`,
a `Challenge/key` response, then `PUT /ISAPI/System/activate` with
`ActivateInfo/password`. Cached `login.js`, `utils.js`, `encryption.js`,
`cryptico.min.js`, and `AES.js` provide the call chain:

1. The activation UI calls `utils.getRSABits()`, which selects 256 bits only
   for Internet Explorer versions below 9 and 1024 bits otherwise. The
   captured Edge 155 user agent therefore selects 1024 bits.
2. `cryptico.publicKeyString(privateKey)` returns the RSA modulus as a
   hexadecimal string, which the UI Base64-encodes. In the activation HAR,
   Base64 decoding `PublicKey/key` yields 256 ASCII hex characters: this is a
   1024-bit modulus represented as hex text, not 256 raw key bytes or a
   2048-bit key. The response `Challenge/key` likewise decodes to 256 ASCII
   hex characters, matching a 1024-bit RSA ciphertext in this implementation.
3. `cryptico.decrypt` consumes that response using the local RSA private key.
   For the 1024-bit branch, the decrypted plaintext is used as a 32-hex-
   character AES key. The separate `aes_encrypt` helper encrypts two single
   16-character blocks: the first 16 plaintext characters and the supplied
   activation password. Each block produces 32 hex characters; the UI
   concatenates them and Base64-encodes the resulting 64-character ASCII hex
   string for `ActivateInfo/password`. The HAR's decoded activation field
   matches this representation.

The helper is a separate single-block AES routine, not the AES-CBC/PKCS#7
routine used for recovery-question fields. Its block parser accepts 16
characters, truncates longer input to the first 16 characters after a UI
alert, and zero-pads shorter input. `cryptico.generateRSAKey` seeds
`Math.seedrandom` from `sha256.hex(new Date()+"")`; this is deterministic,
time-derived key generation and is not cryptographically secure. These are
compatibility findings about this captured web UI, not safe implementation
guidance or a universal ISAPI activation specification. The activation
challenge exchange was observed in the earlier HAR; it was not replayed
against the live camera.

### Assessment of the transport and key-source hypothesis

The evidence supports part of the hypothesis, with important limits:

- The browser executes UI code locally, but the captures load the HTML and
  crypto scripts from `http://192.168.1.64/...`, the camera's own origin.
  Matching script bodies in the activated and non-activated captures show the
  same UI build behavior across those two states; they do not prove that the
  UI is portable across models, firmware builds, or vendors. The UI is not
  independent of the device that serves it.
- For recovery-question fields, the AES key is derived client-side from the
  entered username/password plus salt and iteration/capability values supplied
  by `/ISAPI/Security/capabilities`. Thus the camera contributes public
  parameters, while the user credential is also essential. The key is not
  fetched as a standalone secret from the camera. The separate login proof
  uses the session-login challenge and salt.
- For activation, the browser creates the RSA key pair and sends its public
  modulus. The camera's challenge response contains key material encrypted
  for that supplied public key; the browser decrypts it and derives the AES
  block inputs. The camera contributes challenge key material, but the result
  also depends on client-generated RSA state and the user-entered activation
  password.
- This exchange may protect the challenge key from a passive observer if the
  RSA private key remains secret. The captures do not establish
  authentication of the camera's challenge response, integrity/authenticity
  of the complete activation request, or protection against an active
  intermediary. In particular, the observed page and API use plain HTTP, the
  activation public key is supplied by the client without an evidenced
  signature/pinning step, the RSA generator is seeded deterministically from
  the current date string, and the custom AES block encryption has no
  evidenced MAC. An intermediary able to alter HTTP traffic could potentially
  replace page code or the supplied public key. The traces therefore do not
  support a guarantee that actions cannot be substituted between browser and
  camera.

The requested activation was then performed through the camera-served UI with
the supplied credentials. The UI accepted the password and advanced to the
recovery-question configuration dialog. A subsequent read-only
`GET /SDK/activateStatus` returned `Activated=true`; no recovery questions
were submitted. This validates the observed activation path on this
camera/build, not universal support across firmware versions.

### Direct ISAPI activation validation

The same camera was subsequently activated without using the web UI. The
following exchange was reproduced and executed directly against
`192.168.1.64`:

```text
1. Generate a local RSA-1024 key pair with public exponent 65537.
2. Base64-encode the 256-character hexadecimal RSA modulus.
3. POST /ISAPI/Security/challenge
   body: <PublicKey><key>{base64(modulus_hex)}</key></PublicKey>
4. Base64-decode Challenge/key to ASCII hexadecimal RSA ciphertext.
5. RSA PKCS#1 v1.5-decrypt locally; require a 32-character hexadecimal result.
6. Interpret that result as the 16-byte AES-128 key.
7. AES-128-ECB-encrypt two 16-byte ASCII blocks:
   - the first 16 characters of the decrypted challenge plaintext;
   - the activation password, truncated or zero-padded to 16 bytes.
8. Concatenate the two hexadecimal ciphertext strings and Base64-encode the
   resulting 64-character ASCII hexadecimal string.
9. PUT /ISAPI/System/activate
   body: <ActivateInfo><password>{base64(ciphertext_hex)}</password></ActivateInfo>
10. GET /SDK/activateStatus and require Activated=true.
```

The device returned HTTP 200 with `statusCode=1` and `subStatusCode=ok` for
the activation request. The verification request returned `Activated=true`.
No browser page, UI event, recovery-question request, or secret value was
used in this validation. This confirms that the observed challenge/RSA/AES
format is accepted by this camera/build through direct ISAPI. It does not
establish that every firmware accepts arbitrary RSA key generation or the
same XML namespace, padding, block mode, or key size.

This is a protocol-evidence assessment, not a claim that every firmware has
the same properties. A stronger guarantee would require an authenticated
transport (for example, correctly validated TLS) and an authenticated,
integrity-protected application protocol; neither is evidenced by these
captures.

## User operation shape

The collection endpoint is used with the method and body required by the
device's advertised schema (`PUT` or `POST` may apply). Updating a specific
user uses the identified endpoint when supported:

```text
{device-supported-method} /ISAPI/Security/users?security=1&iv={encoded-iv}
Content-Type: application/xml

<UserList version="2.0" xmlns="http://www.isapi.org/ver20/XMLSchema">
  <User>
    <userName>{validated-user-name}</userName>
    <password>{device-supported ciphertext text for this endpoint}</password>
  </User>
</UserList>
```

This is a schematic example, not a universal XML schema. Preserve the
device-required namespace, version, fields, method, and user identifier.

## Key-source boundary

The Device Network SDK stream-encryption API documents
`NET_DVR_GetDVRConfig` with `NET_DVR_GET_AES_KEY` (command 6113), output
structure `NET_DVR_AES_KEY_INFO`, and key configuration calls
`NET_DVR_InquestSetSecretKey` and `NET_DVR_SetSDKSecretKey`. This path concerns
SDK stream encryption. It is not established as the AES key source for ISAPI
user management. The ISAPI key source and its relationship, if any, to SDK
stream keys must be confirmed by the device-specific ISAPI procedure.

## Operational constraints

- Use only schemes, key sources, XML schemas, and endpoint methods supported by
  the target device.
- Generate a fresh IV for each request; validate decoded IV length (16 bytes)
  and selected AES key length. Treat the UI's timestamp-derived IV as a
  legacy implementation detail, not a recommendation.
- Do not submit the operation if the device-specific key derivation or
  challenge binding required by that operation is unknown.
- Never log or persist passwords, keys, IVs, challenge values, derived
  material, ciphertext, or complete credential-bearing XML.
- Confirm operation success from the device response and a safe state check;
  HTTP success alone is not sufficient.
