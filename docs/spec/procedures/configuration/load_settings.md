# Procedure Specification: P7.1 - LoadSettings

## 1. Contract

```go
func Load(path string) (Settings, error)
```

Load `settings.json`, validate its documented comment metadata and all setting values, apply defaults, and return one typed configuration.

## 2. Processing

1. If `path` is empty, use `settings.json` in the current working directory.
2. If the file does not exist, return `Defaults()` and no error.
3. Decode one JSON document with `encoding/json`; reject trailing data.
4. Require `_comments` and exactly one comment for every leaf setting listed in M7. Reject unknown setting or comment keys.
5. Validate types and ranges from M7. Resolve zero/omitted values only where M7 explicitly permits it.
6. Return a typed error retaining the filesystem, decoding, or validation cause. Never return partially populated settings.

## 3. Verification

Cover absent file, complete template, each lower/upper boundary, zero/default behavior, malformed and trailing JSON, missing or extra comments, unknown settings, invalid relative paths, and invalid discovery-policy combinations.
