---
title: internal/config — OrchestratorConfig
updated: 2026-05-14
category: Packages
tags: [config, yaml, defaults, build-system, cobra, policy, execution-mode, pi]
related_articles:
  - docs/kb/infrastructure/go.md
  - docs/kb/packages/types.md
  - docs/kb/packages/switch.md
  - docs/kb/features/execution-model.md
  - docs/kb/features/pi-runtime-contract.md
---

# internal/config — OrchestratorConfig

## Overview

`internal/config` loads `.doug/doug.yaml` into an `OrchestratorConfig` struct. A missing file returns sane defaults without error. Partial files overlay only the fields present. CLI flags override all config values by being applied after `LoadConfig` returns.

## API

```go
func LoadConfig(path string) (*OrchestratorConfig, error)
func DetectBuildSystem(dir string) string

// Exported default constants
const (
    DefaultBuildSystem    = "go"
    DefaultMaxRetries     = 5
    DefaultMaxIterations  = 20
    DefaultKBEnabled      = true
    DefaultAgentHeartbeat = 30
)

// Execution mode constants (policy.go)
const (
    ExecutionModeRPC       = "rpc"       // Pi-mediated execution; PiAdapter selected
    ExecutionModeSubprocess = "subprocess" // Direct subprocess execution; DefaultBackend selected
)

func ValidateExecutionMode(mode string) error
```

## OrchestratorConfig Fields

| Field | Default | Source |
|-------|---------|--------|
| `RunAgentCommand` | claude run command | `.doug/doug.yaml` → CLI flag |
| `PlanAgentCommand` | claude plan command | `.doug/doug.yaml` → CLI flag |
| `ScaffoldAgentCommand` | claude scaffold command | `.doug/doug.yaml` → CLI flag |
| `ResearchAgentCommand` | claude research command | `.doug/doug.yaml` → CLI flag |
| `BuildSystem` | `"go"` | `.doug/doug.yaml` → CLI flag |
| `MaxRetries` | `5` | `.doug/doug.yaml` → CLI flag |
| `MaxIterations` | `20` | `.doug/doug.yaml` → CLI flag |
| `KBEnabled` | `true` | `.doug/doug.yaml` → CLI flag |
| `AgentHeartbeatSeconds` | `30` | `.doug/doug.yaml` → CLI flag |
| `Policy` | empty | `.doug/doug.yaml` (canonical execution-policy source) |

`Policy` is a `PolicyConfig` with `phases` and `tasks` sub-maps. It is the canonical execution-policy surface for skill resolution, backend routing, and restriction metadata. `policy.tasks[type].skill` is the highest-precedence skill resolver, overriding the hardcoded defaults.

For normal users, `policy` is usually sparse or absent. `doug init` and `doug switch` generate the mode-specific command fields in `.doug/doug.yaml`, and Doug resolves the active execution contract from the workflow phase, the task type, and any configured `policy` overrides. The `policy:` block is mainly an advanced override surface for custom skills, backend selection (`execution_mode`), routing/tool policies, and additional read/write scope constraints.

## Execution Mode Constants

`policy.go` defines the two valid execution mode strings:

| Constant | Value | Meaning |
|----------|-------|---------|
| `ExecutionModeRPC` | `"rpc"` | Pi-mediated execution. `NewBackend` returns `PiAdapter`. Pi owns model selection, tool enforcement, and agent process lifecycle. |
| `ExecutionModeSubprocess` | `"subprocess"` | Direct subprocess execution. `NewBackend` returns `DefaultBackend`. The agent process runs as a direct child of Doug. |

`ValidateExecutionMode` rejects any string other than `""`, `"rpc"`, or `"subprocess"`. An empty string is valid and means "use backend default" (which is `DefaultBackend`). Call this during config loading or `doug.yaml` writes to catch misconfigured execution modes before backend selection.

```go
if err := config.ValidateExecutionMode(mode); err != nil {
    // "unknown execution_mode %q: valid values are ..."
}
```

Do not hardcode the string literals `"rpc"` or `"subprocess"` in call sites; use the exported constants so a future rename is a single-file change.

## Loading Config

```go
cfg, err := config.LoadConfig(".doug/doug.yaml")
if err != nil {
    log.Fatal("loading config: %v", err)
}
// cfg is always non-nil — missing file returns defaults
```

- **Missing file**: returns defaults, no error
- **Partial file**: fields present in file override defaults; absent fields keep defaults
- **Parse error**: returns nil and the error (only on malformed YAML)

## CLI Flag Override Pattern

Cobra binds flags directly to fields on the returned `*OrchestratorConfig` after `LoadConfig`:

```go
cfg, _ := config.LoadConfig(configPath)

// Cobra flag bindings mutate cfg directly — flags win over config file
cmd.Flags().StringVar(&cfg.RunAgentCommand, "agent", cfg.RunAgentCommand, "agent command")
cmd.Flags().IntVar(&cfg.MaxRetries, "max-retries", cfg.MaxRetries, "max retries")
cmd.Flags().IntVar(&cfg.AgentHeartbeatSeconds, "agent-heartbeat-seconds", cfg.AgentHeartbeatSeconds, "heartbeat seconds")
```

When a flag is provided on the command line, cobra overwrites the field. If the flag is omitted, cobra leaves the field unchanged (already set to the config-file or default value). This gives flags the highest precedence at zero extra cost.

## Partial Config Pattern

The internal `partialConfig` struct uses pointer fields to distinguish "absent" from "zero value":

```go
// yaml:"-" equivalent: only non-nil fields override defaults
type partialConfig struct {
    RunAgentCommand       *string `yaml:"run_agent_command"`
    PlanAgentCommand      *string `yaml:"plan_agent_command"`
    ScaffoldAgentCommand  *string `yaml:"scaffold_agent_command"`
    ResearchAgentCommand  *string `yaml:"research_agent_command"`
    KBEnabled             *bool   `yaml:"kb_enabled"`
    AgentHeartbeatSeconds *int    `yaml:"agent_heartbeat_seconds"`
    // ...
}
```

**Why this matters**: `kb_enabled: false` in the config file is a valid override, not an absent field. If `OrchestratorConfig` were unmarshalled directly, `false` would be indistinguishable from a missing field, and the default `true` would win. The pointer layer preserves intent.

## BuildSystemInfo and BuildSystems registry

```go
// BuildSystemInfo contains all metadata doug needs for a supported build system.
// To add a new build system: add one entry to the BuildSystems map.
type BuildSystemInfo struct {
    Permissions    []string // Claude Code Bash permissions to allow
    InstallCmd     string   // human-readable install command for ACTIVE_TASK.md
    VerifyCommands []string // verification steps for ACTIVE_TASK.md
    InitMarkers    []string // marker files used for detection
    CommonPitfalls []string // guidance injected into ACTIVE_TASK.md
}

// BuildSystems is the registry of all supported build systems.
var BuildSystems = map[string]BuildSystemInfo{
    "go":     { ... },
    "npm":    { ... },
    "pnpm":   { ... },
    "static": { ... }, // no-op; for plain HTML/CSS/JS projects with no build step
}
```

The registry is the single source of truth for:
- Bash permissions injected into `.claude/settings.json` during `doug init`
- The `## Build System` briefing section written into `ACTIVE_TASK.md` by `WriteActiveTask`

**Extending**: to add a new build system, add one entry to `BuildSystems`, add detection logic to `DetectBuildSystem`, and update the `--build-system` flag validation in `cmd/init.go`.

## DetectBuildSystem

```go
// Precedence: go.mod > pnpm-workspace.yaml > package.json > "" (no default)
func DetectBuildSystem(dir string) string
```

| Condition | Returns |
|-----------|---------|
| `go.mod` exists | `"go"` |
| `pnpm-workspace.yaml` exists (no `go.mod`) | `"pnpm"` |
| `package.json` exists (no `go.mod` or `pnpm-workspace.yaml`) | `"npm"` |
| `index.html` exists (no other marker) | `"static"` |
| No marker file exists | `""` |

Returns `""` when no marker files are found. Callers are responsible for the fallback:
- `cmd/init.go` prompts interactively on a TTY, warns + defaults to `"go"` otherwise
- `OrchestratorConfig.BuildSystem` defaults to `"go"` via `LoadConfig` when no config file is present

Used by `doug init` to auto-populate `build_system` in the generated `.doug/doug.yaml`. Not called at runtime — the config file takes precedence once generated.

## Key Decisions

**Missing file is not an error**: `doug` should work out of the box with zero configuration. A missing `.doug/doug.yaml` returns the same defaults as an empty one.

**Pointer-based partial parsing**: Required to handle boolean `false` correctly. Any alternative (e.g. checking if a field equals its zero value) would be fragile and break for `max_retries: 0` or `max_iterations: 0`.

**Exported default constants**: Tests reference `config.DefaultMaxRetries` rather than hardcoding `5`. This prevents tests from silently passing when defaults change.

**`skills_dir` removed**: `OrchestratorConfig` no longer has a `SkillsDir` field. The field was loaded from `.doug/doug.yaml` but never consumed at runtime. See [cmd/switch](switch.md) for how `doug switch` uses `OrchestratorConfig` as the authoritative struct for round-trip YAML writes.

**Four-command model replaced `agent_command`**: `OrchestratorConfig` now has `RunAgentCommand`, `PlanAgentCommand`, `ScaffoldAgentCommand`, and `ResearchAgentCommand` instead of a single `AgentCommand`.

**`Policy` is the canonical execution-policy source**: `PolicyConfig.ResolveSkill` (from `policy.tasks[type].skill`) is the highest-precedence skill resolver, sitting above the hardcoded defaults. `ResolveExecution` resolves all other policy fields in one call. Individual `Resolve*` methods exist for callers that need a single field: `ResolveExecutionMode`, `ResolveRoutingProfile`, `ResolveToolPolicy`, `ResolveRestrictionPolicy`, `ResolveWriteScopes`, `ResolveReadPathAdditions`, `ResolveSessionDefaults`. Task-level settings override phase-level settings for single-value fields; list fields (`WriteScopes`, `ReadPathAdditions`) are merged additively with phase paths first.

**`ValidateExecutionMode` enforces the two-value contract**: Only `""`, `"rpc"`, and `"subprocess"` are valid. The catch-all in `NewBackend` maps unknown values to `DefaultBackend`, which could silently hide misconfiguration. `ValidateExecutionMode` is the enforcement point before backend selection runs.

**Most users should not need to edit `policy:`**: the intended common path is `doug init`, optionally `doug switch`, then run Doug normally. The command being executed selects the mode-specific command template; Doug maps that to a workflow phase and task type, then resolves any policy overrides. Treat `policy:` as an escape hatch for advanced customization, not required day-to-day configuration.

**`go` wins over `npm` in `DetectBuildSystem`**: doug is a Go tool and the Go build system is more common. A project with both files is likely a Go project with a JS toolchain layer on top.

**`DetectBuildSystem` returns `""` on no match**: The empty string signals "unknown" to callers rather than silently defaulting. This allows `cmd/init.go` to prompt the user interactively on a TTY instead of silently writing `build_system: go` for every new project.

## Removed Legacy Policy-Resolution Paths

Both legacy paths were removed in EPIC-25-005.

### 1. `.doug/skills-config.yaml` (removed)

Skill selection used to come from `.doug/skills-config.yaml`, which mapped task types to skill names. That file was retired; skill selection is now the sole responsibility of `PolicyConfig.ResolveSkill`, which reads `policy.tasks[taskType].skill` from `.doug/doug.yaml`, falling back to `agent.DefaultSkillName` for the hardcoded built-in skill names. `GetSkillForTaskType`, `skillsConfigFile`, `DefaultSkillsConfigPath`, and `Paths.SkillsConfigPath` were removed. `doug init` no longer installs a `skills-config.yaml` artifact.

Projects that customized `skills-config.yaml` must migrate those mappings to `policy.tasks` in `.doug/doug.yaml`.

### 2. `agent_command` single-field (removed)

The legacy `agent_command` YAML key (single string) that promoted to the four-command set was removed. Only `run_agent_command`, `plan_agent_command`, `scaffold_agent_command`, and `research_agent_command` are accepted. `InferCommandSetFromLegacyCommand` and `partialConfig.AgentCommand` were removed.

## Edge Cases & Gotchas

**Config path is `.doug/doug.yaml`**: Doug-owned config lives under `.doug/`, not at the repository root. On case-sensitive filesystems, both the directory and filename must match exactly.

**`build_system` is not validated by `LoadConfig`**: Unknown values (e.g. `build_system: python`) are accepted without error. The build system package is responsible for validating the value and returning an actionable error.

**Zero `MaxRetries`**: If `max_retries: 0` is set in `.doug/doug.yaml`, `LoadConfig` correctly returns `MaxRetries: 0`. The orchestrator treats this as "no retries allowed" — a task fails on the first FAILURE outcome. This is a valid configuration for strict environments.

**Zero `AgentHeartbeatSeconds`**: If `agent_heartbeat_seconds: 0`, heartbeat logging is disabled. Useful for very quiet CI logs.

## Related Topics

- [Go Infrastructure](../infrastructure/go.md) — build system and project conventions
- [Types](types.md) — TaskType constants used by the config system
- [Execution Model And Provider Presets](../features/execution-model.md) — how `execution_mode` interacts with `doug switch` and Pi activation
- [Doug-to-Pi Runtime Contract](../features/pi-runtime-contract.md) — full Pi policy-input and interaction contract
- [internal/agent](agent.md) — `NewBackend`, `PiAdapter`, `DefaultBackend`, and `ResolvedExecution` consumers
