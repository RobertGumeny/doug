---
title: internal/config — OrchestratorConfig
updated: 2026-05-04
category: Packages
tags: [config, yaml, defaults, build-system, cobra]
related_articles:
  - docs/kb/infrastructure/go.md
  - docs/kb/packages/types.md
  - docs/kb/packages/switch.md
---

# internal/config — OrchestratorConfig

## Overview

`internal/config` loads `doug.yaml` from the project root into an `OrchestratorConfig` struct. A missing file returns sane defaults without error. Partial files overlay only the fields present. CLI flags override all config values by being applied after `LoadConfig` returns.

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
```

## OrchestratorConfig Fields

| Field | Default | Source |
|-------|---------|--------|
| `RunAgentCommand` | claude run command | `doug.yaml` → CLI flag |
| `PlanAgentCommand` | claude plan command | `doug.yaml` → CLI flag |
| `ScaffoldAgentCommand` | claude scaffold command | `doug.yaml` → CLI flag |
| `ResearchAgentCommand` | claude research command | `doug.yaml` → CLI flag |
| `BuildSystem` | `"go"` | `doug.yaml` → CLI flag |
| `MaxRetries` | `5` | `doug.yaml` → CLI flag |
| `MaxIterations` | `20` | `doug.yaml` → CLI flag |
| `KBEnabled` | `true` | `doug.yaml` → CLI flag |
| `AgentHeartbeatSeconds` | `30` | `doug.yaml` → CLI flag |
| `Policy` | empty | `doug.yaml` (canonical policy source) |

`Policy` is a `PolicyConfig` with `phases` and `tasks` sub-maps. `policy.tasks[type].skill` is the highest-precedence skill resolver, overriding both `skills-config.yaml` and the hardcoded defaults.

## Loading Config

```go
cfg, err := config.LoadConfig("doug.yaml")
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
cmd.Flags().StringVar(&cfg.AgentCommand, "agent", cfg.AgentCommand, "agent command")
cmd.Flags().IntVar(&cfg.MaxRetries, "max-retries", cfg.MaxRetries, "max retries")
cmd.Flags().IntVar(&cfg.AgentHeartbeatSeconds, "agent-heartbeat-seconds", cfg.AgentHeartbeatSeconds, "heartbeat seconds")
```

When a flag is provided on the command line, cobra overwrites the field. If the flag is omitted, cobra leaves the field unchanged (already set to the config-file or default value). This gives flags the highest precedence at zero extra cost.

## Partial Config Pattern

The internal `partialConfig` struct uses pointer fields to distinguish "absent" from "zero value":

```go
// yaml:"-" equivalent: only non-nil fields override defaults
type partialConfig struct {
    AgentCommand  *string `yaml:"agent_command"`
    KBEnabled     *bool   `yaml:"kb_enabled"`
    AgentHeartbeatSeconds *int `yaml:"agent_heartbeat_seconds"`
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

Used by `doug init` to auto-populate `build_system` in the generated `doug.yaml`. Not called at runtime — config file takes precedence once generated.

## Key Decisions

**Missing file is not an error**: `doug` should work out of the box with zero configuration. A missing `doug.yaml` returns the same defaults as an empty one.

**Pointer-based partial parsing**: Required to handle boolean `false` correctly. Any alternative (e.g. checking if a field equals its zero value) would be fragile and break for `max_retries: 0` or `max_iterations: 0`.

**Exported default constants**: Tests reference `config.DefaultMaxRetries` rather than hardcoding `5`. This prevents tests from silently passing when defaults change.

**`skills_dir` removed**: `OrchestratorConfig` no longer has a `SkillsDir` field. The field was loaded from `doug.yaml` but never consumed at runtime. See [cmd/switch](switch.md) for how `doug switch` uses `OrchestratorConfig` as the authoritative struct for round-trip YAML writes.

**Three-command model replaced `agent_command`**: `OrchestratorConfig` now has `RunAgentCommand`, `PlanAgentCommand`, and `ScaffoldAgentCommand` instead of a single `AgentCommand`. The legacy `agent_command` YAML key is still accepted as a backward-compatible migration path (see *Legacy Policy-Resolution Paths* below).

**`Policy` is the canonical execution-policy source**: `PolicyConfig.ResolveSkill` (from `policy.tasks[type].skill`) is the highest-precedence skill resolver, sitting above `skills-config.yaml` and the hardcoded defaults. `ResolveExecution` resolves all other policy fields in one call.

**`go` wins over `npm` in `DetectBuildSystem`**: doug is a Go tool and the Go build system is more common. A project with both files is likely a Go project with a JS toolchain layer on top.

**`DetectBuildSystem` returns `""` on no match**: The empty string signals "unknown" to callers rather than silently defaulting. This allows `cmd/init.go` to prompt the user interactively on a TTY instead of silently writing `build_system: go` for every new project.

## Removed Legacy Policy-Resolution Paths

Both legacy paths were removed in EPIC-25-005.

### 1. `skills-config.yaml` (removed)

Skill selection came from `.doug/skills-config.yaml` mapped task types to skill names. This file was retired; skill selection is now the sole responsibility of `PolicyConfig.ResolveSkill`, which reads `policy.tasks[taskType].skill` from `doug.yaml`, falling back to `agent.DefaultSkillName` for the hardcoded built-in skill names. `GetSkillForTaskType`, `skillsConfigFile`, `DefaultSkillsConfigPath`, and `Paths.SkillsConfigPath` were removed. The `skills-config.yaml` template is silently skipped by `doug init` (the file remains in the embedded FS for compatibility but produces no output).

Projects that customized `skills-config.yaml` must migrate those mappings to `policy.tasks` in `doug.yaml`.

### 2. `agent_command` single-field (removed)

The legacy `agent_command` YAML key (single string) that promoted to the three-command set was removed. Only `run_agent_command`, `plan_agent_command`, and `scaffold_agent_command` are accepted. `InferCommandSetFromLegacyCommand` and `partialConfig.AgentCommand` were removed.

## Edge Cases & Gotchas

**`doug.yaml` vs `doug.yaml` (case-sensitivity)**: On case-insensitive filesystems (macOS default, Windows), `doug.yaml` will be found. On Linux (case-sensitive), it won't. Always use lowercase `doug.yaml`.

**`build_system` is not validated by `LoadConfig`**: Unknown values (e.g. `build_system: python`) are accepted without error. The build system package is responsible for validating the value and returning an actionable error.

**Zero `MaxRetries`**: If `max_retries: 0` is set in `doug.yaml`, `LoadConfig` correctly returns `MaxRetries: 0`. The orchestrator treats this as "no retries allowed" — a task fails on the first FAILURE outcome. This is a valid configuration for strict environments.

**Zero `AgentHeartbeatSeconds`**: If `agent_heartbeat_seconds: 0`, heartbeat logging is disabled. Useful for very quiet CI logs.

## Related Topics

- [Go Infrastructure](../infrastructure/go.md) — build system and project conventions
- [Types](types.md) — TaskType constants used by the config system
