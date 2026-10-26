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

The request carries the selected scheme and a Base64-encoded 16-byte IV in its
query:

```text
?security=1&iv={url_encoded_base64_iv}  # AES-128-CBC, 16-byte key
?security=2&iv={url_encoded_base64_iv}  # AES-256-CBC, 32-byte key
```

Query values must be encoded with a URI/query builder. Do not concatenate a
standard Base64 value containing `+`, `/`, or `=` into a raw URI.

## Challenge and request flow

When the device advertises the security-challenge procedure, obtain and
validate the short-lived challenge before the protected operation:

```text
POST /ISAPI/Security/challenge
```

The response may contain a challenge, salt, validity indicator, and other
device-specific metadata. The challenge protects session/operation freshness
against replay. It is not, by itself, a documented AES key derivation recipe.
Bind the subsequent critical request to the challenge only as specified by the
device or SDK. Do not infer a SHA-256, MD5, or HMAC derivation from the presence
of challenge and salt.

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
    <password>{Base64(AES-CBC(PKCS7(Base64(UTF8(password)))))}</password>
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
  and selected AES key length (16 or 32 bytes).
- Do not submit the operation if the key source or challenge binding required
  by the device is unknown.
- Never log or persist passwords, keys, IVs, challenge values, derived
  material, ciphertext, or complete credential-bearing XML.
- Confirm operation success from the device response and a safe state check;
  HTTP success alone is not sufficient.
