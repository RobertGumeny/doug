# Doug

Go CLI tool that orchestrates Claude Code agents across multi-task projects.

## Build & Test

```bash
go build ./...
go test ./...
go vet ./...
go mod tidy   # after adding imports
```

## Key Conventions

- All logic lives in `internal/`. `cmd/` only wires subcommands to `internal/`.
- No `sh -c` or shell string concatenation — always pass explicit args to `exec.Command`.
- Atomic file writes: write to `.tmp`, then `os.Rename` to final path.
- No new dependencies without updating `go.mod` via `go mod tidy`.
- Approved deps: `cobra`, `gopkg.in/yaml.v3`. Everything else is stdlib.

## Docs

- `docs/kb/` — package-by-package reference and patterns
- `PRD.md` — requirements and architecture
