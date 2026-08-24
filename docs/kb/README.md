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
| [internal/types](packages/types.md) | Shared structs and typed constants; SessionResult 5-field constraint (incl. structured `bugs`); SessionBug/BugPayload bug types plus resolver metadata; UserDefined/Synthetic distinction (scaffold + bugfix); provider observability metric structs |
| [internal/types — LoopContext & Task Ops](packages/types-loop-context.md) | LoopContext struct (per-iteration state), UpdateTaskStatus, AdvanceToNextTask, AreAllUserTasksComplete |
| [internal/state](packages/state.md) | LoadProjectState, SaveProjectState, LoadTasks, SaveTasks; ErrNotFound and ParseError with YAML field/hint diagnostics |
| [internal/config](packages/config.md) | OrchestratorConfig, LoadConfig (partial-file pattern), DetectBuildSystem, parse diagnostics, stale `execution_mode` rejection, `review_enabled`/`kb_enabled` finalization toggles |
| [internal/dougpath](packages/dougpath.md) | Centralized `.doug/intake/` and `.doug/logs/epics/` path helpers; stable attempt directory contract for Pi-native JSONL transcripts |
| [internal/log](packages/log.md) | Info, Success, Warning, Error, Fatal, Section; Lipgloss-backed style palette; OsExit injection for tests |
| [internal/style](packages/style.md) | Shared Lipgloss palette for log badges, section headers, hints, selected rows, and status text |
| [internal/status](packages/status.md) | TTY-gated live status indicator for long Pi-backed waits with non-TTY heartbeat fallback |
| [internal/build](packages/build.md) | BuildSystem interface, GoBuildSystem, NpmBuildSystem, NewBuildSystem factory |
| [internal/git](packages/git.md) | EnsureEpicBranch, RollbackChanges (in-memory backup), Commit/CommitPaths, ErrNothingToCommit; CurrentSHA, CommittedDiff, ResetHard, SHA/branch introspection helpers |
| [internal/lifecycle](packages/lifecycle.md) | Shared lifecycle core for read-only status discovery, interactive claims, verified completion, failure/blockage, and epic finalization invariants |
| [internal/mcp](packages/mcp.md) | Interactive Implement tool handlers for status, claiming, completion, blockage, post-epic lifecycle work, and dispatcher/worker hygiene |
| [internal/runlock](packages/runlock.md) | Shared `.doug/run.lock` OS file lock for headless run and mutating MCP lifecycle tools |
| [internal/orchestrator](packages/orchestrator.md) | BootstrapFromTasks, task pointer management, tiered validation, CheckDependencies, EnsureProjectReady, runtime attempt UX logging, advisory post-epic review, post-epic KB/changelog synthesis |
| [internal/metrics](packages/metrics.md) | RecordTaskMetrics with provider wait/failure diagnostics, UpdateMetricTotals, PrintEpicSummary; non-fatal by design |
| [internal/stats](packages/stats.md) | RunStats schema, write-time Pi stats capture, phase-aware summary loading, and attempt-scoped `.doug/logs/epics/` persistence |
| [internal/changelog](packages/changelog.md) | UpdateChangelog — idempotent, pure-Go CHANGELOG.md insert; non-fatal errors |
| [internal/agent](packages/agent.md) | Pi-only Backend interface and PiAdapter; first-response/tool/provider observability; reusable true-interactive Pi launcher; PrepareExecution + ExecutionPrep; lifecycle-aware WriteActiveTask, ParseSessionResult (structured `bugs`), ArchiveActiveTask; shared bug archive writer/resolution updater; post-epic review and KB/changelog contracts |
| [internal/templates](packages/templates.md) | Embedded init-template inventory, explicit `//go:embed` coverage, and Pi-first scaffold boundaries |
| [internal/handlers](packages/handlers.md) | HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete; blocking/non-blocking bug routing; conservative BUG-* archive writeback; SuccessResultKind; run loop integration and exit code policy |
| [cmd/init](packages/init.md) | `doug init` subcommand; runInitWorkflow + doInitProject entrypoint chain; discoverable config generation, first-run prompts/epilogue, install plan model and merge algorithms |
| [cmd/mcp](packages/cmd-mcp.md) | `doug mcp` local stdio MCP server; early config validation, JSON-RPC framing, tool listing/calling, and thin command-to-handler dispatch |
| [cmd/plan](packages/plan.md) | `doug plan` subcommand; planning-intent resolution, greenfield auto-detection, PLAN.md refresh, ACTIVE_TASK.md planning brief contract, generic intake sections for reported bugs and recent research, downstream post-epic review and KB/changelog awareness |
| [internal/testutil](packages/testutil.md) | Shared test helpers (`WriteFile`); eliminates duplicate helpers across packages |
| [internal/prompt](packages/prompt.md) | Reusable interactive prompt helpers (`SelectOne`, `Confirm`, `Text`, `IsTTY`); `io.Writer`/`io.Reader`-injected for testability |
| [internal/interactive](packages/interactive.md) | Shared interactive command UX (`Prompter` interface); Bubble Tea-backed on TTY, plain fallback in CI/tests; `SelectOne`, `Confirm`, `Text`, `Compose` |

## Features

| Article | Description |
|---------|-------------|
| [Interaction Model And Pi Policy Ownership](features/execution-model.md) | Cross-cutting operator contract for Doug-owned prompts, Pi-only execution, phase-owned Pi modes, and `.pi/` scaffolding boundaries |
| [Interactive Implement MCP Surface](features/interactive-implement.md) | MCP-first interactive Implement, recovery for interrupted sessions (`get_status`, diagnostics, explicit repair), supported headless/interactive terminology, handshake-surface contract, lifecycle authority, locking, and dispatcher/worker context hygiene |
| [Doug-to-Pi Runtime Contract](features/pi-runtime-contract.md) | Pi's mandatory role as Doug's execution boundary; run inputs, lifecycle-aware briefs, workflow interaction semantics, and Doug/Pi compatibility boundaries |
| [Transport Failure Recovery](features/transport-failure-recovery.md) | Pi RPC transport failure classification, infra retries, durable failure records, and attempt-start markers |
| [Run UX + Provider Stall Visibility](features/run-ux-provider-visibility.md) | Attempt headers, live heartbeat activity, first-response callouts, stall warnings, end-of-turn summaries, and provider metrics |
| [Build-System Module Root](features/module-root.md) | Optional `module_root` config, subdirectory build roots, Go `go.mod` initialization sentinel, and missing-module warning |
| [CLI Discoverability And Config Diagnostics](features/cli-discoverability.md) | First-run init guidance, command help expectations, generated config comments, parse diagnostics, and MCP startup validation |
| [Planning And Execution Lifecycle Contract](features/planning-lifecycle.md) | Canonical planning/backlog/runtime ownership model, deterministic reported-bug intake, greenfield handoff contract, epic statuses, transition rules, and command responsibilities |
| [Post-Epic Review, KB Synthesis, And Changelog Polish](features/post-epic-finalization.md) | Shared finalization ordering, advisory review artifacts, explicit `doug review`, KB/changelog contract, scoped commits, and non-gating semantics |
| [doug revert](features/revert.md) | `doug revert <task_id>`; ten-step validation, git reset --hard, session log cleanup, SHA fallback via grep |
| [doug scaffold](features/scaffold.md) | `doug scaffold`; manifest v1 contract, current-stable dependency lookup, precondition guards, single-invocation agent model, statelessness |
| [OSS Beta Repository Readiness](features/oss-beta-readiness.md) | License, community policy docs, GitHub issue/PR templates, README badges, and repository-facing contributor expectations |
| [doug research](features/research.md) | `doug research`; read-only analysis contract, write restriction to `.doug/intake/research/`, one-shot invocation model, top-level report intake into `doug plan` |
| [doug stats](features/stats.md) | `doug stats [epic_id]`; local `.doug/logs/epics/` stats reader, per-task table, phase-aware records, and aggregate totals |
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
