# Knowledge Base

Agent-oriented reference for the `doug` Go port. Read before writing code.

## Infrastructure

| Article | Description |
|---------|-------------|
| [Go Infrastructure & Best Practices](infrastructure/go.md) | Module path, project structure, approved dependencies, exec/atomic/tier rules |
| [Go 1.26](dependencies/go-1-26.md) | Version pinning, relevant features, upgrade policy |

## Packages

| Article | Description |
|---------|-------------|
| [internal/types](packages/types.md) | All shared structs and typed constants; SessionResult 3-field constraint; UserDefined/Synthetic distinction |
| [internal/state](packages/state.md) | LoadProjectState, SaveProjectState, LoadTasks, SaveTasks; ErrNotFound and ParseError |
| [internal/config](packages/config.md) | OrchestratorConfig, LoadConfig (partial-file pattern), DetectBuildSystem |
| [internal/log](packages/log.md) | Info, Success, Warning, Error, Fatal, Section; OsExit injection for tests |
| [internal/build](packages/build.md) | BuildSystem interface, GoBuildSystem, NpmBuildSystem, NewBuildSystem factory |
| [internal/git](packages/git.md) | EnsureEpicBranch, RollbackChanges (in-memory backup), Commit, ErrNothingToCommit |
| [internal/orchestrator](packages/orchestrator.md) | BootstrapFromTasks, task pointer management (InitializeTaskPointers, AdvanceToNextTask), tiered validation (ValidateYAMLStructure, ValidateStateSync), LoopContext struct, CheckDependencies, EnsureProjectReady |
| [internal/metrics](packages/metrics.md) | RecordTaskMetrics, UpdateMetricTotals, PrintEpicSummary; non-fatal by design |
| [internal/changelog](packages/changelog.md) | UpdateChangelog — idempotent, pure-Go CHANGELOG.md insert; non-fatal errors |
| [internal/agent](packages/agent.md) | CreateSessionFile, WriteActiveTask, GetSkillForTaskType, RunAgent, ParseSessionResult; full agent lifecycle for one iteration |
| [internal/handlers](packages/handlers.md) | HandleSuccess, HandleFailure, HandleBug, HandleEpicComplete; SuccessResultKind; run loop integration and exit code policy |

## Patterns

| Article | Description |
|---------|-------------|
| [Exec Command Pattern](patterns/pattern-exec-command.md) | Safe subprocess invocation; no sh -c; cmd.Dir vs os.Chdir; streaming vs buffering |
| [Atomic File Writes](patterns/pattern-atomic-file-writes.md) | Write to .tmp then os.Rename; same-directory rule; load-mutate-save discipline |
