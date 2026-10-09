# Procedure Specification: P4.2 - SearchDevices
## Module: M4 - Command-Line Interface

## 1. Purpose

Filter a snapshot of discovered devices by a user-provided query for display in the CLI device list. This procedure is presentation logic only; it does not perform network discovery, selection, or device operations.

## 2. Go Contract

```go
func SearchDevices(devices []parser.Device, query string) []parser.Device
```

- Match case-insensitively against each device's MAC, IPv4 address, serial number, and model.
- Trim leading and trailing whitespace from the query.
- Return a new slice and do not modify the input slice or its elements.
- Return all devices when the normalized query is empty.
- Return an empty slice when there are no matches; nil input produces an empty result.
- Support filtering the complete device list, including when the result is subsequently used for single-device focus or multi-device selection. Selection state and operations are managed separately and must not be inferred from this result slice.

## 3. Processing Requirements

1. Normalize the query using `strings.TrimSpace` and `strings.ToLower`.
2. Convert searchable values to their display forms and apply case-insensitive substring matching.
3. Sort results stably with inactive devices (`Activated == false`) before active devices; preserve the original order within each status group.
4. Do not mutate or reorder the discovery snapshot.
5. Apply filtering before display without losing the underlying discovery snapshot; clearing the query restores the full current snapshot.

## 4. Errors and Tests

This in-memory filtering procedure has no expected operational errors. Test empty and whitespace-only queries, case-insensitive matching, every searchable field, no matches, nil and empty input, filtering and clearing a query over a device list, stable inactive-first ordering, and preservation of the original slice.

---

*Level 2 procedure specification.*
