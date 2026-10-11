# Procedure Specification: P5.7 - ExportLocaleTemplate
## Module: M5 - Localization

## 1. Purpose

Write the canonical embedded English catalog to a new destination for translation.

## 2. Go Contract

```go
func ExportLocaleTemplate(
    english map[string]string,
    destination string,
) error
```

- Reject an invalid English catalog or empty destination with an error.
- Return nil only when the complete JSON template is written and closed successfully.
- Never overwrite an existing destination.

## 3. Processing Requirements

1. Validate entries with P5.3's catalog entry rules.
2. Encode the map as one UTF-8 JSON object with stable key ordering and a trailing newline.
3. Create the destination exclusively (`O_CREATE|O_EXCL`) with permissions no broader than `0644`; do not truncate or replace existing files.
4. If write or close fails after this call created the file, remove only that newly created partial file and include cleanup failure in the returned error.
5. Do not change active locale or settings, and do not require external locale files.

## 4. Errors and Tests

Preserve filesystem errors and include the destination path but never catalog contents. Test successful exact key/value export, deterministic ordering, empty/invalid input, missing parent, write/close failure, existing destination remains byte-for-byte unchanged, and cleanup of a partial file created by the operation.

---

*Level 3 atomic procedure specification.*
