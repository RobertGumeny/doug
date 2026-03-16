# internal/testutil

## Purpose

Shared test helpers used across package-level test files. Eliminates duplicate `writeFile` helper definitions that previously existed independently in multiple packages.

## API

### `WriteFile(t *testing.T, path, content string)`

Creates a file at `path` with the given `content`, creating all parent directories as needed. Calls `t.Fatalf` if any filesystem operation fails. Always call `t.Helper()` before delegating to this function from your own helper.

```go
testutil.WriteFile(t, filepath.Join(dir, "go.mod"), "module example\n\ngo 1.21\n")
```

## Usage

Import as a test-only dependency:

```go
import "github.com/RobertGumeny/doug/internal/testutil"
```

Used by: `internal/agent`, `internal/build`, `internal/config`, `internal/handlers`.

## Constraints

- This package is for test helpers only. Do not add production code here.
- Keep helpers general and reusable across packages. Package-specific setup belongs in the package's own `_test.go` files.
