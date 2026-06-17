# Knowledge Base Index

The knowledge base is the shared reference for doug's codebase. It is intended to be readable by both humans and coding agents: use it to understand package responsibilities, project conventions, and implementation constraints before changing code.

If you are contributing as a human, start here. If you are running doug or using a manual agent session, this is also the right high-level map before dropping into package docs.

## How to Use This Index

- Start with the section closest to your task: infrastructure, packages, features, or patterns.
- Use package articles for code ownership and behavior details.
- Use pattern articles for project-wide implementation rules that apply across packages.
- Use feature articles for cross-cutting flows and repository-facing policies.

## Infrastructure

| Article | Description |
|---------|-------------|
| [Go Infrastructure & Best Practices](infrastructure/go.md) | Module path, project structure, approved dependencies, exec/atomic/tier rules |
| [Go 1.26](dependencies/go-1-26.md) | Version pinning, relevant features, upgrade policy |

## Packages

| Article | Description |
|---------|-------------|
| [internal/types](packages/types.md) | Shared structs and typed constants; SessionResult 3-field constraint; UserDefined/Synthetic distinction |
| [internal/types — LoopContext & Task Ops](packages/types-loop-context.md) | LoopContext struct (per-iteration state), UpdateTaskStatus, AdvanceToNextTask, AreAllUserTasksComplete |
| [internal/state](packages/state.md) | LoadProjectState, SaveProjectState, LoadTasks, SaveTasks; ErrNotFound and ParseError |
| [internal/config](packages/config.md) | OrchestratorConfig, LoadConfig (partial-file pattern), DetectBuildSystem |
| [internal/log](packages/log.md) | Info, Success, Warning, Error, Fatal, Section; OsExit injection for tests |
| [internal/build](packages/build.md) | BuildSystem interface, GoBuildSystem, NpmBuildSystem, NewBuildSystem factory |
| [internal/git](packages/git.md) | EnsureEpicBranch, RollbackChanges (in-memory backup), Commit, ErrNothingToCommit; CurrentSHA, ResetHard, SHA/branch introspection helpers |
| [internal/orchestrator](packages/orchestrator.md) | BootstrapFromTasks, task pointer management (InitializeTaskPointers, AdvanceToNextTask), tiered validation (ValidateYAMLStructure, ValidateStateSync), LoopContext struct, CheckDependencies, EnsureProjectReady |
| [internal/metrics](packages/metrics.md) | RecordTaskMetrics, UpdateMetricTotals, PrintEpicSummary; non-fatal by design |
| [internal/changelog](packages/changelog.md) | UpdateChangelog — idempotent, pure-Go CHANGELOG.md insert; non-fatal errors |
| [internal/agent](packages/agent.md) | Pi-only Backend interface and PiAdapter; reusable true-interactive Pi launcher; PrepareExecution + ExecutionPrep; WriteActiveTask, ParseSessionResult, ArchiveActiveTask |
| [internal/templates](packages/templates.md) | Embedded init-template inventory, explicit `//go:embed` coverage, and Pi-first scaffold boundaries |
| [internal/handlers](packages/handlers.md) | HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete; SuccessResultKind; run loop integration and exit code policy |
| [cmd/init](packages/init.md) | `doug init` subcommand; runInitWorkflow + doInitProject entrypoint chain; Pi-first config and scaffolding flow; install plan model and merge algorithms |
| [cmd/plan](packages/plan.md) | `doug plan` subcommand; planning-intent resolution, interactive prompt capture, PLAN.md refresh, ACTIVE_TASK.md planning brief contract |
| [internal/testutil](packages/testutil.md) | Shared test helpers (`WriteFile`); eliminates duplicate helpers across packages |
| [internal/prompt](packages/prompt.md) | Reusable interactive prompt helpers (`SelectOne`, `Confirm`, `Text`, `IsTTY`); `io.Writer`/`io.Reader`-injected for testability |
| [internal/interactive](packages/interactive.md) | Shared interactive command UX (`Prompter` interface); Bubble Tea-backed on TTY, plain fallback in CI/tests; `SelectOne`, `Confirm`, `Text`, `Compose` |

## Features

| Article | Description |
|---------|-------------|
| [Interaction Model And Pi Policy Ownership](features/execution-model.md) | Cross-cutting operator contract for Doug-owned prompts, Pi-only execution, phase-owned Pi modes, and `.pi/` scaffolding boundaries |
| [Doug-to-Pi Runtime Contract](features/pi-runtime-contract.md) | Pi's mandatory role as Doug's execution boundary; run inputs, workflow interaction semantics, and Doug/Pi compatibility boundaries |
| [Transport Failure Recovery](features/transport-failure-recovery.md) | Pi RPC transport failure classification, infra retries, durable failure records, and attempt-start markers |
| [Build-System Module Root](features/module-root.md) | Optional `module_root` config, subdirectory build roots, Go `go.mod` initialization sentinel, and missing-module warning |
| [Planning And Execution Lifecycle Contract](features/planning-lifecycle.md) | Canonical planning/backlog/runtime ownership model, epic statuses, transition rules, and command responsibilities |
| [doug revert](features/revert.md) | `doug revert <task_id>`; ten-step validation, git reset --hard, session log cleanup, SHA fallback via grep |
| [doug scaffold](features/scaffold.md) | `doug scaffold`; manifest v1 contract, precondition guards, single-invocation agent model, statelessness |
| [OSS Beta Repository Readiness](features/oss-beta-readiness.md) | License, community policy docs, GitHub issue/PR templates, README badges, and repository-facing contributor expectations |
| [doug research](features/research.md) | `doug research`; read-only analysis contract, write restriction to `.doug/logs/research/`, one-shot invocation model |
| [doug upgrade](features/upgrade.md) | `doug upgrade`; three-stage workflow (inspect, report, apply); retired execution-config stripping, surface ownership model, retired artifact detection, managed surface reinstall |

## Patterns

| Article | Description |
|---------|-------------|
| [Exec Command Pattern](patterns/pattern-exec-command.md) | Safe subprocess invocation; no sh -c; cmd.Dir vs os.Chdir; streaming vs buffering |
| [Atomic File Writes](patterns/pattern-atomic-file-writes.md) | Write to .tmp then os.Rename; same-directory rule; load-mutate-save discipline |
| [Best-Effort Terminal & Writer Output](patterns/pattern-best-effort-writes.md) | Use local `writef` helpers for non-fatal stdout/stderr and `io.Writer` output |
| [Drain Subprocess Pipes Before Wait](patterns/pattern-pipe-drain.md) | Drain stdout to EOF on every reader exit path or `cmd.Wait()` deadlocks against a blocked child write |

## Related Entry Points

- [`README.md`](../../README.md) for product overview, installation, and CLI usage
- [`CONTRIBUTING.md`](../../CONTRIBUTING.md) for the human contribution workflow
- [`AGENTS.md`](../../AGENTS.md) for repository-specific agent operating instructions; `CLAUDE.md` includes it via `@AGENTS.md`
- [`docs/testing-strategy.md`](../testing-strategy.md) for the project testing strategy, package coverage status, and shared test utilities
