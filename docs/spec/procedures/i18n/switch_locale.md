# Procedure Specification: P5.8 - SwitchLocale
## Module: M5 - Localization

## 1. Purpose

Atomically publish a fully loaded locale code and catalog as one immutable active state.

## 2. Go Contract

```go
type LocaleState struct {
    Code     string
    Messages map[string]string
}

func SwitchLocale(
    active *atomic.Pointer[LocaleState],
    candidate LocaleState,
) error
```

- Reject a nil pointer, invalid code, nil/empty catalog, or invalid catalog entries with an error and leave active state unchanged.
- Return nil only after the complete candidate has been defensively copied and atomically published.
- The candidate catalog must already be complete (normally returned by P5.1); this procedure does not perform I/O or fallback merging.

## 3. Processing Requirements

1. Validate `candidate.Code` with P5.2 and validate every message key/value.
2. Copy the code and map into a new `LocaleState`; do not retain mutable aliases to the caller's map.
3. Perform one atomic pointer store so readers observe either the previous complete state or the new complete state.
4. Published state and its map are immutable. Readers must load the pointer once per rendering operation and must not mutate the pointed-to map.
5. On failure, preserve the previous state exactly. Runtime switching does not persist settings.

## 4. Errors and Tests

Return validation errors without catalog contents and do not use `panic`. Test nil pointer, invalid candidate, defensive copy, concurrent readers observing only complete code/catalog pairs, and unchanged previous state on failure.

---

*Level 3 atomic procedure specification.*
