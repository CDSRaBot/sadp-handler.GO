# Procedure Specification: P5.1 - LoadLocale
## Module: M4 - CLI Support (Localization)

## 1. Purpose

Load and validate one locale catalog and produce a complete immutable message catalog with English fallback for missing translation keys. A failed load must never replace the currently active locale.

## 2. Go Contract

```go
func LoadLocale(
    filesystem fs.FS,
    langCode string,
    english map[string]string,
) (map[string]string, error)
```

- `locales/en.json` is the canonical source catalog: it defines the complete stable set of message keys and their English text. Embed this file into the Go binary with `embed`, so English remains available if the external locales directory is absent or empty.
- All user-visible application messages must resolve through keys defined by the canonical English catalog. Do not build the key list by scraping rendered text or extracting strings from compiled UI code.
- `english` is the decoded, validated embedded English catalog supplied to the loader. It is always available at runtime independently of external locale files.
- For `langCode == "en"`, return a defensive copy of `english` without requiring `locales/en.json`.
- For another language, return the requested catalog merged over English and nil error only if its catalog exists and is valid.
- If the locales directory is empty, the requested catalog is missing, or it cannot be read, decoded, or validated, return `nil` and an error. Never return English together with an error as a success-shaped fallback.
- The caller changes the active catalog only after receiving a complete catalog and nil error. On failure, it reports the error and keeps the active locale unchanged.
- Returned maps must not alias `english`, the filesystem decoder's map, or mutable internal state. The caller treats a successfully loaded catalog as immutable after activation.

## 3. Canonical Catalog and Template Export

- Keep the canonical English JSON file at `locales/en.json` in the repository and embed it at build time. This one file is both the editable source for developers and the complete key template for translators; do not maintain a second hard-coded default message map.
- JSON must be a flat object of stable message keys to English strings, for example:

  ```json
  {
    "language.name": "English",
    "scan.start": "Starting network scan...",
    "devices.found": "Found {count} devices."
  }
  ```

- Provide a non-interactive CLI operation such as `--export-locale-template <path>`. Since TUI mode is the default, invoke this and other one-shot CLI operations with `--ui=no` (for example, `app --ui=no --export-locale-template <path>`). It writes a JSON copy of the embedded English catalog to the requested path, preserving every key and English value so a translator can edit only the values for a target locale.
- Export must work even when the external locales directory is missing or empty. It must not change the active locale or normal application settings.
- Export is handled by standard CLI mode and does not open or change the TUI.
- Do not overwrite an existing destination. Return an explicit error identifying the destination when it already exists or cannot be created/written; preserve the existing file unchanged.
- The exported template is a starting point, not a requirement that every translation file contain every key. Partial locale files are valid and missing keys resolve to English as specified below.
- After changing the English catalog, tests must detect message keys referenced by application code but missing from the catalog. Template export must include all canonical keys and must not add or drop keys.

## 4. Locale Selection and Precedence

The effective locale is resolved in this order, with later sources taking precedence:

1. Start with built-in English (`en`).
2. Use the locale from the settings file when present.
3. Override it with the locale specified by command-line arguments when present.
4. While the application is running, an explicit successful locale selection through the built-in command overrides the current locale for the rest of the process.

At startup, resolve settings and command-line precedence first, then load only the effective locale. An invalid or unavailable lower-priority value that is overridden by a higher-priority value does not block startup. If the effective requested locale cannot be loaded, report an error and retain the current locale (English during startup); do not silently fall through to another requested locale. Runtime selection is in-memory only and does not rewrite settings unless a separate persistence behavior is specified.

The application enumerates locales to build the runtime selection list. Built-in English is always an available base locale. If the external locales directory is empty or cannot be enumerated, report an error and do not open/offer a runtime switch; keep English active during startup or preserve the last successfully selected locale at runtime. A scan that discovers no valid non-English catalogs is an error, not an empty successful selection list.

## 5. Catalog Format and Fallback

1. Validate `langCode` as a locale identifier containing only ASCII letters, digits, underscore, or hyphen; reject empty values, path separators, and traversal.
2. Read `locales/{langCode}.json` using `fs.FS` and slash-separated paths. `en` is supplied by the built-in English catalog and does not require an external file.
3. Decode exactly one flat JSON object into `map[string]string`. Reject nested values, non-string values, malformed JSON, and trailing JSON content.
4. Validate the English catalog and every provided locale entry: keys and values must not be empty or whitespace-only. An absent locale key is allowed; an invalid present entry is a catalog error.
5. Build the resolved catalog by starting with a defensive copy of English and overlaying the selected locale's entries. Thus, if a message key has no translation in the selected locale (the intended `en`/selected-language pair is incomplete), that message renders in English.
6. If a key is absent from both catalogs, the message resolver renders the key itself and emits a diagnostic suitable for development/support logs. It must not render a blank string.
7. Do not log catalog contents or user-provided translation values.

## 6. Atomic Runtime Switching

1. Keep the currently active locale code and resolved catalog together as one application state.
2. For a requested switch, load and validate the complete candidate catalog without mutating active state.
3. On failure, report the error using the currently active catalog and leave both the active code and catalog unchanged.
4. On success, replace the active locale state atomically; subsequent messages use the new catalog. Report success in the newly active locale.
5. Locale selection is exact: an unavailable locale is an error, not a request to silently switch to English or to another locale.
6. Prefer the settings key `language`, the command-line flag `--lang`, and a built-in `language` selection command; use the same normalized locale code in all three sources.

## 7. Errors and Tests

Return errors that identify the operation and requested locale without exposing catalog contents. Preserve filesystem and JSON decoding causes for inspection. Do not use a fallback-error result that callers could accidentally install as if the requested locale had loaded.

Tests should cover:

- Embedded English loaded from the canonical `locales/en.json` with no external locale directory or file.
- Template export copies all canonical keys and values, works without external locale files, leaves active locale/settings unchanged, and refuses to overwrite an existing file.
- Application message keys are checked against the canonical English catalog.
- Resolving config, command-line, and runtime selection precedence.
- Valid full and partial catalogs; missing keys use English values.
- Unknown message keys do not produce blank output.
- Empty locales directory, missing requested file, unreadable file, malformed/trailing JSON, nested/non-string values, and invalid entries.
- Invalid requested locale leaves the active locale code and catalog unchanged.
- A successful runtime switch replaces code and catalog together.
- Defensive copying and concurrent readers observing either the old or new complete catalog, never a partially merged catalog.

---

*Level 3 atomic procedure specification.*
