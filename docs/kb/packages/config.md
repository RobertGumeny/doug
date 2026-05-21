---
title: internal/config — OrchestratorConfig
updated: 2026-05-20
category: Packages
tags: [config, yaml, defaults, build-system, cobra, policy, execution-mode, pi]
related_articles:
  - docs/kb/infrastructure/go.md
  - docs/kb/packages/types.md
  - docs/kb/features/execution-model.md
  - docs/kb/features/pi-runtime-contract.md
---

# internal/config — OrchestratorConfig

## Overview

`internal/config` loads `.doug/doug.yaml` into an `OrchestratorConfig` struct. A missing file returns sane defaults without error. Partial files overlay only the fields present. CLI flags override all config values by being applied after `LoadConfig` returns. Doug-owned prompt text is not stored in config; it is generated in code and then paired with the resolved policy at runtime.

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
    DefaultLintEnabled    = false
)

// Interaction mode constants (policy.go)
const (
    InteractionModeInteractive = "interactive" // Pi-mediated interactive session; PiAdapter selected
    InteractionModeRPC         = "rpc"         // Pi-mediated RPC one-shot; PiAdapter selected
)

func ValidateInteractionMode(mode string) error
func (PolicyConfig) RequiresPi() bool  // true for interactive or rpc
func (PolicyConfig) RequiresRPC() bool // true only for rpc transport
```

## OrchestratorConfig Fields

| Field | Default | Source |
|-------|---------|--------|
| `BuildSystem` | `"go"` | `.doug/doug.yaml` → CLI flag |
| `MaxRetries` | `5` | `.doug/doug.yaml` → CLI flag |
| `MaxIterations` | `20` | `.doug/doug.yaml` → CLI flag |
| `KBEnabled` | `true` | `.doug/doug.yaml` → CLI flag |
| `AgentHeartbeatSeconds` | `30` | `.doug/doug.yaml` → CLI flag |
| `LintEnabled` | `false` | `.doug/doug.yaml` |
| `LintCommand` | `""` | `.doug/doug.yaml` |
| `Policy` | empty | `.doug/doug.yaml` (canonical interaction-policy source) |

`LintEnabled` controls whether a lint step runs after the build/test verification steps succeed (both in `HandleSuccess` and `HandleResume`). When `LintEnabled` is true:
- If `LintCommand` is non-empty, `build.RunLint(projectRoot, LintCommand)` is called (parsed via `strings.Fields`; no `sh -c`).
- If `LintCommand` is empty, the build-system default from `BuildSystemInfo.LintCmd` is used (e.g. `go vet ./...` for Go). If the build system has no default lint command, the step is a no-op.

A lint failure pauses the project (same as a build or test failure).

`Policy` is a `PolicyConfig` with `phases` and `tasks` sub-maps. It is the canonical interaction-policy surface for skill resolution and restriction metadata. Pi routing is source-owned by workflow phase, so `interaction_mode` entries in config cannot change the execution harness or Pi mode. `policy.tasks[type].skill` is the highest-precedence skill resolver, overriding the hardcoded defaults.

For normal users, `policy` is usually narrow. `doug init` generates a `policy.phases` block that makes phase defaults explicit: planning uses `interaction_mode: interactive`, while runtime, scaffold, research, and post-epic KB use `interaction_mode: rpc`. Doug resolves the active execution contract from the workflow phase, the task type, and any configured non-mode `policy` overrides. The `policy:` block is the interaction-policy surface for custom skills, routing/tool policies, and additional read/write scope constraints; Pi mode remains source-owned by phase.

## Interaction Mode Constants

`policy.go` defines the valid interaction mode strings:

| Constant | Value | Meaning |
|----------|-------|---------|
| `InteractionModeInteractive` | `"interactive"` | Pi-mediated interactive session. `NewBackend` returns `PiAdapter`, and the Pi RPC request uses Pi's interactive execution pattern. |
| `InteractionModeRPC` | `"rpc"` | Pi-mediated JSON-RPC one-shot execution. `NewBackend` returns `PiAdapter`; Doug supervises Pi over RPC while Pi owns model selection, tool enforcement, and agent process lifecycle. |

Built-in phase defaults when no task or phase override is configured:

| Phase | Default interaction mode |
|-------|--------------------------|
| `planning` | `interactive` |
| `runtime` | `rpc` |
| `scaffold` | `rpc` |
| `research` | `rpc` |
| `post_epic_kb` | `rpc` |

The older `execution_mode` YAML field was intentionally removed before wide release. `LoadConfig` rejects stale policy entries that contain `execution_mode` with an error telling the operator to use `interaction_mode` instead; there is no aliasing or precedence between the two names. A stale `policy.phases.planning.execution_mode: rpc` entry is reported as needing migration to `policy.phases.planning.interaction_mode: interactive`, because planning's implemented default is an interactive Pi session rather than an RPC one-shot.

`ValidateInteractionMode` rejects any string other than `""`, `"interactive"`, or `"rpc"`. Phase-policy parsing reports unsupported values with the phase path, for example `policy.phases.runtime.interaction_mode`, and lists the accepted implemented modes. Call this during config loading or `doug.yaml` writes to catch misconfigured interaction modes before backend selection.

`PolicyConfig.RequiresPi()` is the generic dependency predicate for Pi CLI availability. It returns true for configured `interactive` and `rpc` modes. Empty values return false until phase defaults are resolved. `PolicyConfig.RequiresRPC()` remains available for checks that specifically require the RPC transport; do not use it for generic Pi dependency/preflight checks because interactive Pi sessions also need the `pi` binary.

```go
if err := config.ValidateInteractionMode(mode); err != nil {
    // "unknown interaction_mode %q: valid values are ..."
}
```

Do not hardcode the string literals `"interactive"` or `"rpc"` in call sites; use the exported constants so a future rename is a single-file change. `subprocess` is a stale unsupported value.

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
cmd.Flags().IntVar(&cfg.MaxRetries, "max-retries", cfg.MaxRetries, "max retries")
cmd.Flags().IntVar(&cfg.AgentHeartbeatSeconds, "agent-heartbeat-seconds", cfg.AgentHeartbeatSeconds, "heartbeat seconds")
```

When a flag is provided on the command line, cobra overwrites the field. If the flag is omitted, cobra leaves the field unchanged (already set to the config-file or default value). This gives flags the highest precedence at zero extra cost.

## Partial Config Pattern

The internal `partialConfig` struct uses pointer fields to distinguish "absent" from "zero value":

```go
// yaml:"-" equivalent: only non-nil fields override defaults
type partialConfig struct {
    BuildSystem           *string `yaml:"build_system"`
    MaxRetries            *int    `yaml:"max_retries"`
    MaxIterations         *int    `yaml:"max_iterations"`
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
    LintCmd        string   // default lint command when lint_enabled=true and lint_command is unset; empty = no-op
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
- The `## Build System` briefing section written into `ACTIVE_TASK.md` by `WriteActiveTask`
- Build-system detection defaults used by `doug init` and scaffold/runtime verification flows

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

**`skills_dir` removed**: `OrchestratorConfig` no longer has a `SkillsDir` field. The field was loaded from `.doug/doug.yaml` but never consumed at runtime.

**Prompt text is code-owned, not config-owned**: `OrchestratorConfig` no longer stores mode-specific command fields. Doug builds initial Pi prompts from code constants via `config.BuildInitialPrompt(...)`, while `.doug/doug.yaml` owns only policy and top-level runtime settings.

**`Policy` is the canonical interaction-policy source for non-mode settings**: `PolicyConfig.ResolveSkill` (from `policy.tasks[type].skill`) is the highest-precedence skill resolver, sitting above the hardcoded defaults. `ResolveExecution` resolves all execution fields in one call, but `InteractionMode` is always derived from the built-in phase default. Individual `Resolve*` methods exist for callers that need a single field: `ResolveInteractionMode`, `ResolveRoutingProfile`, `ResolveToolPolicy`, `ResolveRestrictionPolicy`, `ResolveWriteScopes`, `ResolveReadPathAdditions`, `ResolveSessionDefaults`. For non-mode single-value fields, task-level settings override phase-level settings; list fields (`WriteScopes`, `ReadPathAdditions`) are merged additively with phase paths first.

**Phase interaction-mode routing is source-owned**: Doug resolves planning to `interactive` and runtime, scaffold, research, and post-epic KB to `rpc` regardless of task type or `.doug/doug.yaml`. Unknown phases resolve to `""` and execution preparation turns that into a clear Doug error.

**`ValidateInteractionMode` enforces the implemented mode contract**: Only `""`, `"interactive"`, and `"rpc"` are valid. Phase-policy validation names the failing phase and lists the accepted implemented modes. `NewBackend` always returns the Pi adapter, so validation is the enforcement point for rejecting stale direct-subprocess configuration before execution runs.

**Pi dependency checks are interaction-mode aware**: `RequiresPi` covers both Pi-backed CLI modes (`interactive` and `rpc`), while `RequiresRPC` is intentionally narrow for code paths that require Pi's JSON-RPC transport specifically. Preflight dependency checks should use `RequiresPi` so interactive Pi sessions do not skip the `pi` binary check.

**Most users should not need to edit `policy:` heavily**: the intended common path is `doug init`, then run Doug normally. Doug maps each workflow to a built-in prompt plus a workflow phase and task type, then resolves any policy overrides. Treat `policy:` as an advanced customization surface, not something most users hand-maintain day to day.

**`go` wins over `npm` in `DetectBuildSystem`**: doug is a Go tool and the Go build system is more common. A project with both files is likely a Go project with a JS toolchain layer on top.

**`DetectBuildSystem` returns `""` on no match**: The empty string signals "unknown" to callers rather than silently defaulting. This allows `cmd/init.go` to prompt the user interactively on a TTY instead of silently writing `build_system: go` for every new project.

## Removed Legacy Policy-Resolution Paths

Both legacy paths were removed in EPIC-25-005.

### 1. `.doug/skills-config.yaml` (removed)

Skill selection used to come from `.doug/skills-config.yaml`, which mapped task types to skill names. That file was retired; skill selection is now the sole responsibility of `PolicyConfig.ResolveSkill`, which reads `policy.tasks[taskType].skill` from `.doug/doug.yaml`, falling back to `agent.DefaultSkillName` for the hardcoded built-in skill names. `GetSkillForTaskType`, `skillsConfigFile`, `DefaultSkillsConfigPath`, and `Paths.SkillsConfigPath` were removed. `doug init` no longer installs a `skills-config.yaml` artifact.

Projects that customized `skills-config.yaml` must migrate those mappings to `policy.tasks` in `.doug/doug.yaml`.

### 2. `agent_command` single-field (removed)

The legacy `agent_command` YAML key (single string) was removed. Doug no longer reads mode-specific command templates from config at all; initial Pi prompt generation now lives in `config.BuildInitialPrompt(...)`. `InferCommandSetFromLegacyCommand` and `partialConfig.AgentCommand` were removed.

## Edge Cases & Gotchas

**Config path is `.doug/doug.yaml`**: Doug-owned config lives under `.doug/`, not at the repository root. On case-sensitive filesystems, both the directory and filename must match exactly.

**`build_system` is not validated by `LoadConfig`**: Unknown values (e.g. `build_system: python`) are accepted without error. The build system package is responsible for validating the value and returning an actionable error.

**Zero `MaxRetries`**: If `max_retries: 0` is set in `.doug/doug.yaml`, `LoadConfig` correctly returns `MaxRetries: 0`. The orchestrator treats this as "no retries allowed" — a task fails on the first FAILURE outcome. This is a valid configuration for strict environments.

**Zero `AgentHeartbeatSeconds`**: If `agent_heartbeat_seconds: 0`, heartbeat logging is disabled. Useful for very quiet CI logs.

## Related Topics

- [Go Infrastructure](../infrastructure/go.md) — build system and project conventions
- [Types](types.md) — TaskType constants used by the config system
- [Interaction Model And Pi Policy Ownership](../features/execution-model.md) — how `interaction_mode`, Doug-owned prompts, and Pi activation work
- [Doug-to-Pi Runtime Contract](../features/pi-runtime-contract.md) — full Pi policy-input and interaction contract
- [internal/agent](agent.md) — `NewBackend`, `PiAdapter`, and `ResolvedExecution` consumers
