# doug

A task automation CLI built in Go. This is a scaffold — the core commands are stubs today and will be filled in over time.

```
$ doug --help
$ doug run
$ doug init
```

---

## Project structure

```
doug/
├── main.go                        # Entry point
├── go.mod                         # Module definition & dependencies
├── go.sum                         # Dependency checksums (auto-generated)
├── Makefile                       # Common dev tasks
├── .goreleaser.yaml               # Cross-platform release config
├── .gitignore
├── cmd/
│   ├── root.go                    # Root command + --version flag
│   ├── run.go                     # `doug run` subcommand
│   └── init.go                    # `doug init` subcommand
├── internal/                      # Private packages (empty for now)
├── integration/                   # Integration tests (empty for now)
└── .github/
    └── workflows/
        ├── ci.yml                 # Run tests on every push/PR
        └── release.yml            # Publish binaries on version tag
```

---

## File-by-file walkthrough

### `main.go`

The entire file is 7 lines. In Go, every executable must have a `package main` with a `func main()`. This one does exactly one thing: calls `cmd.Execute()` and exits.

Keeping `main.go` this thin is a Go convention — it makes the CLI logic importable and testable without having to run a real process.

### `go.mod`

Declares three things:
- The **module path** (`github.com/robertgumeny/doug`) — this is how Go identifies your project and how other code would import it.
- The **minimum Go version** required.
- The **direct dependency**: [cobra](https://github.com/spf13/cobra), the CLI framework used by kubectl, Hugo, and many others.

You edit this file indirectly via `go get` and `go mod tidy`. You rarely touch it by hand.

### `go.sum`

Auto-generated. Contains cryptographic hashes of every dependency so Go can verify nothing has been tampered with. Commit it, never edit it.

### `cmd/root.go`

Defines the top-level `doug` command. Key concepts here:

- **`cobra.Command`** is a struct that represents one CLI command. You fill in fields like `Use` (the name), `Short` (the help text), and `RunE` (the function to call).
- **`Execute()`** is the public function called by `main.go`. It hands control to cobra, which parses `os.Args` and dispatches to the right subcommand.
- **`func init()`** is a special Go function that runs automatically before `main()`. We use it to register subcommands and set the version string. Every `.go` file can have its own `init()`.
- **`rootCmd.Version = version`** wires up the built-in `--version` / `-v` flag that cobra provides for free.

### `cmd/run.go` and `cmd/init.go`

Each file defines one subcommand as a package-level variable (`var runCmd`, `var initCmd`). They're registered in `root.go`'s `init()`.

`RunE` (vs `Run`) is the variant that returns an `error`. Prefer `RunE` — it lets cobra handle error printing consistently and makes testing easier.

Right now both just print `"not implemented"` and return `nil` (no error).

### `internal/`

Go enforces that anything inside an `internal/` directory **cannot be imported by code outside this module**. This is Go's built-in way to mark packages as private implementation details. Future packages (config loading, task running, etc.) will live here.

### `integration/`

Holds end-to-end tests that run the compiled binary as a subprocess, rather than calling Go functions directly. Empty for now.

### `Makefile`

Shortcuts so you don't have to remember raw `go` commands:

| Target | Command | What it does |
|--------|---------|-------------|
| `make build` | `go build -o doug .` | Compiles a `doug` binary in the current directory |
| `make test` | `go test ./...` | Runs all tests in all packages (`./...` means "recursively") |
| `make lint` | `go vet ./...` | Static analysis — catches common mistakes the compiler misses |
| `make release-dry` | `goreleaser release --snapshot --clean` | Builds all release artifacts locally without publishing |

### `.goreleaser.yaml`

[GoReleaser](https://goreleaser.com) automates building and publishing release binaries. This config:

- Builds for **5 targets**: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
- `CGO_ENABLED=0` — produces fully static binaries with no C dependencies (important for Linux containers)
- `-s -w` linker flags — strips debug symbols to shrink binary size
- Packages Linux/macOS as `.tar.gz`, Windows as `.zip`
- Generates a `checksums.txt` so users can verify downloads

### `.github/workflows/ci.yml`

Runs on every push and pull request. Executes `go test ./...` and `go vet ./...` on both `ubuntu-latest` and `macos-latest` to catch platform-specific issues early.

### `.github/workflows/release.yml`

Triggers when you push a tag matching `v*` (e.g. `git tag v0.1.0 && git push --tags`). Runs GoReleaser, which builds all the binaries and creates a GitHub Release with the artifacts attached automatically.

---

## Development workflow

```bash
# Build and run locally
make build
./doug --help

# Run all tests
make test

# Check for common mistakes
make lint

# Preview what a release would produce (no publish)
make release-dry
```

---

## How subcommands get added

1. Create `cmd/yourcommand.go` with a `var yourCmd = &cobra.Command{...}`
2. Add `rootCmd.AddCommand(yourCmd)` in `cmd/root.go`'s `init()`
3. That's it — cobra handles help text, usage, and argument parsing

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework — commands, flags, help text |
| `github.com/spf13/pflag` | POSIX-style flags (indirect dep of cobra) |
| `github.com/inconshreveable/mousetrap` | Windows-specific cobra helper (indirect) |
