---
title: internal/config — OrchestratorConfig
updated: 2026-05-21
category: Packages
tags: [config, yaml, defaults, build-system, cobra, execution-mode, pi]
related_articles:
  - docs/kb/infrastructure/go.md
  - docs/kb/packages/types.md
  - docs/kb/features/execution-model.md
  - docs/kb/features/pi-runtime-contract.md
---

# internal/config — OrchestratorConfig

## Overview

`internal/config` loads `.doug/doug.yaml` into an `OrchestratorConfig` struct. A missing file returns sane defaults without error. Partial files overlay only the fields present. CLI flags override all config values by being applied after `LoadConfig` returns. Doug-owned prompt text is not stored in config; it is generated in code.

Execution routing (interaction modes, routing profiles, tool policy, skill overrides, write scopes) is no longer read from `.doug/doug.yaml`. Doug source code owns the workflow-to-Pi contract. The `policy:` YAML key is silently ignored if present; `doug upgrade` flags it as a retired field to be removed.

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

// Interaction mode constants (policy.go) — source-owned, not read from config
const (
    InteractionModeInteractive = "interactive" // Pi-mediated interactive session
    InteractionModeRPC         = "rpc"         // Pi-mediated RPC one-shot
)

func ValidateInteractionMode(mode string) error
func ValidatePhaseInteractionMode(phase, mode string) error
func DefaultInteractionModeForPhase(phase string) string
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

`LintEnabled` controls whether a lint step runs after the build/test verification steps succeed (both in `HandleSuccess` and `HandleResume`). When `LintEnabled` is true:
- If `LintCommand` is non-empty, `build.RunLint(projectRoot, LintCommand)` is called (parsed via `strings.Fields`; no `sh -c`).
- If `LintCommand` is empty, the build-system default from `BuildSystemInfo.LintCmd` is used (e.g. `go vet ./...` for Go). If the build system has no default lint command, the step is a no-op.

A lint failure pauses the project (same as a build or test failure).

## Interaction Mode Constants

`policy.go` defines the valid interaction mode strings and phase-default resolution. These are source-owned constants — they are not read from `.doug/doug.yaml`.

| Constant | Value | Meaning |
|----------|-------|---------|
| `InteractionModeInteractive` | `"interactive"` | Pi-mediated interactive session used for planning. |
| `InteractionModeRPC` | `"rpc"` | Pi-mediated JSON-RPC one-shot used for runtime, scaffold, research, and post-epic KB. |

Built-in phase defaults:

| Phase | Default interaction mode |
|-------|--------------------------|
| `planning` | `interactive` |
| `runtime` | `rpc` |
| `scaffold` | `rpc` |
| `research` | `rpc` |
| `post_epic_kb` | `rpc` |

`DefaultInteractionModeForPhase(phase)` returns the built-in default for known phases and `""` for unknown phases.

`ValidateInteractionMode` rejects any string other than `""`, `"interactive"`, or `"rpc"`. `ValidatePhaseInteractionMode` adds a phase-scoped prefix to the error for actionable log messages.

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
- **Unknown YAML keys** (including `policy:`): silently ignored by the YAML parser
- **Parse error**: returns nil and the error (only on malformed YAML)
- **Stale `execution_mode` at top level**: rejected with an error directing the operator to use `interaction_mode`

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
type BuildSystemInfo struct {
    Permissions    []string // Claude Code Bash permissions to allow
    InstallCmd     string   // human-readable install command for ACTIVE_TASK.md
    VerifyCommands []string // verification steps for ACTIVE_TASK.md
    InitMarkers    []string // marker files used for detection
    CommonPitfalls []string // guidance injected into ACTIVE_TASK.md
    LintCmd        string   // default lint command when lint_enabled=true and lint_command is unset
}

var BuildSystems = map[string]BuildSystemInfo{
    "go":     { ... },
    "npm":    { ... },
    "pnpm":   { ... },
    "static": { ... },
}
```

## DetectBuildSystem

```go
// Precedence: go.mod > pnpm-workspace.yaml > package.json > index.html > "" (no default)
func DetectBuildSystem(dir string) string
```

Returns `""` when no marker files are found. Callers are responsible for the fallback.

## Key Decisions

**Missing file is not an error**: `doug` should work out of the box with zero configuration.

**Pointer-based partial parsing**: Required to handle boolean `false` correctly.

**Exported default constants**: Tests reference `config.DefaultMaxRetries` rather than hardcoding `5`.

**Policy removed from config**: `OrchestratorConfig` no longer has a `Policy PolicyConfig` field. Execution routing (interaction mode, routing profiles, tool policy, skill overrides, write scopes) is source-owned by Doug and is not configurable from `.doug/doug.yaml`. The interaction mode constants (`InteractionModeInteractive`, `InteractionModeRPC`) and `DefaultInteractionModeForPhase` remain as source-owned constants in `policy.go`.

**`policy:` block in YAML is silently ignored**: The YAML parser ignores unknown keys, so existing configs with `policy:` blocks will load without error. `doug upgrade` detects the retired field and reports it as drift to be removed manually.

**Prompt text is code-owned, not config-owned**: `OrchestratorConfig` does not store mode-specific command fields. Doug builds initial Pi prompts from code constants via `config.BuildInitialPrompt(...)`.

**`skills_dir` removed**: `OrchestratorConfig` no longer has a `SkillsDir` field.

**`go` wins over `npm` in `DetectBuildSystem`**: A project with both files is likely a Go project with a JS toolchain layer on top.

**`DetectBuildSystem` returns `""` on no match**: Signals "unknown" to callers rather than silently defaulting.

## Removed Legacy Policy-Resolution Paths

### 1. `.doug/skills-config.yaml` (removed in EPIC-25-005)

Skill selection used to come from `.doug/skills-config.yaml`. That file was retired; skill mapping is now hardcoded via `agent.DefaultSkillName`. `doug init` no longer installs a `skills-config.yaml` artifact.

### 2. `agent_command` single-field (removed in EPIC-25-005)

The legacy `agent_command` YAML key was removed. Initial Pi prompt generation now lives in `config.BuildInitialPrompt(...)`.

### 3. `policy:` block (removed in EPIC-41-001)

`PolicyConfig`, `PhasePolicy`, `TaskPolicy`, `ResolvedExecution`, and all `Resolve*` methods were removed. Execution routing is source-owned by Doug. Interaction mode constants and phase defaults remain in `policy.go` as source-owned constants used by `agent.PrepareExecution`.

## Edge Cases & Gotchas

**Config path is `.doug/doug.yaml`**: Doug-owned config lives under `.doug/`, not at the repository root.

**`build_system` is not validated by `LoadConfig`**: Unknown values are accepted without error. The build system package validates the value.

**Zero `MaxRetries`**: If `max_retries: 0` is set, `LoadConfig` correctly returns `MaxRetries: 0`. The orchestrator treats this as "no retries allowed".

**Zero `AgentHeartbeatSeconds`**: If `agent_heartbeat_seconds: 0`, heartbeat logging is disabled.

## Related Topics

- [Go Infrastructure](../infrastructure/go.md) — build system and project conventions
- [Types](types.md) — TaskType constants
- [Interaction Model And Pi Policy Ownership](../features/execution-model.md) — how interaction modes and Pi activation work
- [Doug-to-Pi Runtime Contract](../features/pi-runtime-contract.md) — full Pi policy-input and interaction contract
- [internal/agent](agent.md) — `NewBackend`, `PiAdapter`, and `PrepareExecution`
