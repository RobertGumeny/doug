---
task_id: "EPIC-4-004"
outcome: "SUCCESS"
timestamp: "2026-02-24T23:20:00Z"
changelog_entry: "Added ParseSessionResult to extract and validate YAML frontmatter from agent session files"
files_modified:
  - internal/agent/parse.go
  - internal/agent/parse_test.go
tests_run: 11
tests_passed: 11
build_successful: true
duration_seconds: 210
estimated_tokens: 25000
---

## Implementation Summary

Created `internal/agent/parse.go` with `ParseSessionResult(filePath string) (*types.SessionResult, error)` that reads a session file, extracts YAML frontmatter between the first and second `---` delimiter lines using pure Go string scanning, unmarshals it into `SessionResult`, and validates the `Outcome` field.

## Files Changed

- `internal/agent/parse.go` — Implements `ParseSessionResult` with typed error types (`ErrNoFrontmatter`, `ErrMissingOutcome`, `*ErrInvalidOutcome`) and CRLF/LF normalisation
- `internal/agent/parse_test.go` — Table-driven tests covering all acceptance criteria cases

## Key Decisions

- CRLF normalised via `strings.ReplaceAll(content, "\r\n", "\n")` before line splitting — handles both Windows and Unix line endings uniformly
- `os.ReadFile` error returned directly so callers use `errors.Is(err, os.ErrNotExist)` — standard Go pattern, no wrapping needed
- Sentinel `errors.New` values for `ErrNoFrontmatter` and `ErrMissingOutcome`; typed struct `*ErrInvalidOutcome` for the invalid value case (carries the bad value for diagnostic messages)
- `yaml.Unmarshal` silently ignores unknown fields by default — extra frontmatter fields require no special handling
- `strings.TrimSpace(line) == "---"` used for delimiter detection to tolerate trailing whitespace

## Test Coverage

- ✅ Valid file with SUCCESS outcome
- ✅ Valid file with BUG outcome
- ✅ Valid file with FAILURE outcome
- ✅ Valid file with EPIC_COMPLETE outcome
- ✅ Extra fields silently ignored
- ✅ CRLF line endings handled correctly
- ✅ Missing `---` delimiters → ErrNoFrontmatter
- ✅ Only one `---` delimiter → ErrNoFrontmatter
- ✅ Empty outcome field → ErrMissingOutcome
- ✅ Unknown outcome value → *ErrInvalidOutcome
- ✅ File not found → os.ErrNotExist
