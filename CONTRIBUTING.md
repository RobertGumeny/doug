# Contributing to doug

This guide is for humans contributing to doug. Start with the knowledge base at [`docs/kb/README.md`](docs/kb/README.md), then use this document for the expected workflow and review bar.

`AGENTS.md` and `CLAUDE.md` are operational instructions for coding agents. They are useful when running doug against this repo or working in a manual agent session, but they are not the primary contributor guide.

## Where to Start

- Read [`docs/kb/README.md`](docs/kb/README.md) for the codebase index and package-level reference.
- Read [`README.md`](README.md) for product context, CLI usage, and repository layout.
- Keep each change focused on one problem. Documentation, bug fixes, and features should not be mixed unless they are tightly coupled.

## Using doug Effectively

If you are using doug to build doug, treat task design as part of the implementation work. Well-scoped epics and tasks produce more reliable agent output and make review easier.

- Use the `research` skill to help analyze the codebase. It can be pointed at a specific function, file, feature, or the entire codebase as well as knowledge base articles.
- The `research` skill will generate a `RESEARCH_REPORT.md` which can be used to draft and edit `PRD.md` and `tasks.yaml` files to set up a doug agent loop.
- Keep epics to roughly 6-8 tasks. Larger epics tend to accumulate too much context and are harder for humans to validate cleanly.
- Keep each task narrow enough that it can be completed without spanning multiple unrelated concerns.
- Aim for about 3-4 acceptance criteria per task. That is usually enough to make the target concrete without overloading the agent.
- Prefer tasks with a single primary outcome, such as one bug fix, one feature slice, or one documentation update.
- Split follow-up cleanup, refactors, and stretch goals into separate tasks instead of bundling them into the first implementation pass.
- Write acceptance criteria in observable terms. Favor outcomes like CLI behavior, file changes, validation rules, or tests over vague implementation instructions.
- Use the KB and existing code patterns to shape tasks around how the project already works instead of describing an entirely new approach from scratch.
- Review agent-produced changes with the same scrutiny you would apply to manual contributions. Verify behavior, tests, and scope before treating the task as done.
- When a task starts needing long exception lists or many conditional requirements, that is usually a sign it should be split into smaller tasks.
- Treat this guidance as iterative. If you find better patterns for task sizing or epic structure, update this document so future contributors can reuse them.

## Local Checks

Run the standard project checks before opening or updating a pull request:

```bash
make build
make test
make lint
```

Those targets currently map to:

- `make build`: builds the `doug` binary at `bin/doug` with the project version linker flag
- `make test`: runs `go test ./...`
- `make lint`: runs `gofmt`, `golangci-lint`, and `go vet`

If you add or change imports, also run:

```bash
go mod tidy
```

## Code Expectations

- Keep business logic in `internal/`. Use `cmd/` only for CLI wiring.
- Do not use `sh -c` or shell string concatenation when invoking subprocesses. Pass explicit arguments to `exec.Command`.
- Use atomic file writes when updating files on disk: write a temporary file first, then rename it into place.
- Avoid new dependencies unless they are necessary. If you add one, update `go.mod` and `go.sum` with `go mod tidy`.
- Keep comments brief and high signal. Prefer clear code over explanatory noise.

The KB articles in [`docs/kb/README.md`](docs/kb/README.md) document package behavior, project patterns, and implementation constraints in more detail.

## Testing Expectations

Every PR should include validation for the changed behavior:

- Add or update automated tests for new logic, regressions, and edge cases.
- Run `make test` locally before opening the PR.
- Run `make lint` locally before opening the PR.
- Run `make build` if your change affects compilation, packaging, CLI wiring, or dependencies.

If a test is not practical, explain the gap and the manual verification you performed in the PR description.

## Pull Requests

PRs should be small enough to review and should clearly explain:

- what changed
- why the change is needed
- how it was tested
- any follow-up work or known limitations

Link the relevant task, issue, or requirement when one exists. Include screenshots or terminal output only when they materially help review.

Do not include unrelated refactors, drive-by formatting, or speculative cleanup in the same PR unless the change is required to make the main patch correct.

## Commit Messages

Use short, imperative commit messages. When a task ID exists, include it in the commit subject so the change is traceable.

Preferred patterns:

- `feat: EPIC-10-002 add contributing guide`
- `fix: EPIC-10-002 correct lint command docs`
- `docs: EPIC-10-002 clarify contributor entry points`

Keep the first line focused on the user-visible or reviewer-relevant change. Put extra rationale in the commit body if needed.

## AI-Assisted Contributions

AI-assisted or AI-generated contributions are welcome. The same review, testing, and code quality bar applies regardless of how code was produced.

Maintainers may request revisions or decline a pull request that does not meet the project's standards. They may do so without providing detailed justification, though they will do their best to share the reason when a PR is declined.
