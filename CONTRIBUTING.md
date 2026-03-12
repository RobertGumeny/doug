# Contributing to Doug

Thanks for contributing to Doug. This guide covers the expected pull request workflow, code quality bar, and the local checks contributors should run before opening or updating a PR.

## Before You Start

- Read [`CLAUDE.md`](CLAUDE.md) for the project's core implementation conventions.
- Read [`AGENTS.md`](AGENTS.md) if you are contributing through an agent workflow or want the repo's task execution boundaries.
- Keep changes focused. Documentation, bug fixes, and features should each stay scoped to a single problem.

## Development Workflow

Use the Make targets at the repository root:

```bash
make build
make test
make lint
```

Those targets currently map to:

- `make build`: builds the `doug` binary with the project version linker flag
- `make test`: runs `go test ./...`
- `make lint`: runs `gofmt`, `golangci-lint`, and `go vet`

If you add or change imports, also run:

```bash
go mod tidy
```

## Code Style Expectations

- Follow the architecture in `CLAUDE.md`: keep business logic in `internal/` and reserve `cmd/` for CLI wiring.
- Do not use `sh -c` or shell string concatenation when invoking subprocesses. Pass explicit arguments to `exec.Command`.
- Use atomic file writes when updating files on disk: write a temporary file first, then rename it into place.
- Avoid new dependencies unless they are necessary. If you add one, update `go.mod` and `go.sum` with `go mod tidy`.
- Keep comments brief and high signal. Prefer clear code over explanatory noise.

## Testing Requirements

Every PR should include the validation needed for the changed behavior:

- Add or update automated tests for new logic, regressions, and edge cases.
- Run `make test` locally before opening the PR.
- Run `make lint` locally before opening the PR.
- Run `make build` if your change affects compilation, packaging, CLI wiring, or dependencies.

If a test is not practical, explain the gap and the manual verification you performed in the PR description.

## Pull Request Expectations

PRs should be small enough to review and should clearly explain:

- what changed
- why the change is needed
- how it was tested
- any follow-up work or known limitations

Link the relevant task, issue, or requirement when one exists. Include screenshots or terminal output only when they materially help review.

Do not include unrelated refactors, drive-by formatting, or speculative cleanup in the same PR unless the change is required to make the main patch correct.

## Commit Conventions

Use short, imperative commit messages. When a task ID exists, include it in the commit subject so the change is traceable.

Preferred patterns:

- `feat: EPIC-10-002 add contributing guide`
- `fix: EPIC-10-002 correct lint command docs`
- `docs: EPIC-10-002 clarify PR expectations`

Keep the first line focused on the user-visible or reviewer-relevant change. Put extra rationale in the commit body if needed.

## AI-Assisted Contributions

AI-assisted or AI-generated contributions are welcome. The same review, testing, and code quality bar applies regardless of how code was produced.

Maintainers may request revisions or decline a pull request that does not meet the project's standards. They may do so without providing detailed justification, though they will do their best to share the reason when a PR is declined.
