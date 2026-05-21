---
title: internal/config — OrchestratorConfig
updated: 2026-05-21
category: Packages
tags: [config, yaml, defaults, build-system, cobra, lint]
related_articles:
  - docs/kb/infrastructure/go.md
  - docs/kb/packages/init.md
  - docs/kb/features/execution-model.md
  - docs/kb/features/pi-runtime-contract.md
---

# internal/config — OrchestratorConfig

## Overview

`internal/config` loads `.doug/doug.yaml` into an `OrchestratorConfig` struct.

The supported config surface is intentionally small. `.doug/doug.yaml` stores ordinary orchestrator settings such as build system, retry limits, KB enablement, heartbeat cadence, and optional lint settings. It does **not** store backend selection, provider commands, phase routing, skill overrides, or prompt templates. Doug source owns the Pi execution contract.

A missing config file returns defaults without error. A partial file overlays only the fields present. CLI flags override loaded values after `LoadConfig` returns.

## API

```go
func LoadConfig(path string) (*OrchestratorConfig, error)
func DetectBuildSystem(dir string) string

const (
    DefaultBuildSystem    = "go"
    DefaultMaxRetries     = 5
    DefaultMaxIterations  = 20
    DefaultKBEnabled      = true
    DefaultAgentHeartbeat = 30
    DefaultLintEnabled    = false
)
```

## Supported Config Fields

| Field | Default | Meaning |
|-------|---------|---------|
| `build_system` | `go` | Which build-system adapter Doug should use |
| `max_retries` | `5` | Max `FAILURE` outcomes before a task becomes blocked |
| `max_iterations` | `20` | Max orchestration loop iterations before `doug run` exits |
| `kb_enabled` | `true` | Whether post-epic KB synthesis should run |
| `agent_heartbeat_seconds` | `30` | Liveness log cadence while Pi is running (`0` disables) |
| `lint_enabled` | `false` | Whether lint should run after successful build/test verification |
| `lint_command` | `""` | Optional explicit lint command override |

When `lint_enabled` is true:
- if `lint_command` is non-empty, Doug runs it via `build.RunLint(projectRoot, lintCommand)`
- otherwise Doug uses the build-system default lint command from `BuildSystemInfo.LintCmd`
- if the chosen build system has no default lint command, the lint step is a no-op

## Loading Behavior

```go
cfg, err := config.LoadConfig(".doug/doug.yaml")
if err != nil {
    log.Fatal("loading config: %v", err)
}
```

`LoadConfig` behavior:

- **missing file**: returns defaults, no error
- **partial file**: present fields override defaults
- **unknown YAML keys**: ignored by the YAML parser
- **malformed YAML**: returns an error
- **top-level `execution_mode`**: rejected as a stale field

## Partial-Config Pattern

`LoadConfig` unmarshals into an internal `partialConfig` that uses pointer fields so Doug can distinguish “absent” from “explicit zero value”:

```go
type partialConfig struct {
    BuildSystem           *string `yaml:"build_system"`
    MaxRetries            *int    `yaml:"max_retries"`
    MaxIterations         *int    `yaml:"max_iterations"`
    KBEnabled             *bool   `yaml:"kb_enabled"`
    AgentHeartbeatSeconds *int    `yaml:"agent_heartbeat_seconds"`
    LintEnabled           *bool   `yaml:"lint_enabled"`
    LintCommand           *string `yaml:"lint_command"`
}
```

This matters most for booleans like `kb_enabled: false`, which must override the default `true`.

## CLI Flag Override Pattern

Cobra binds flags onto the returned config struct after loading, so flags win automatically:

```go
cfg, _ := config.LoadConfig(configPath)
cmd.Flags().IntVar(&cfg.MaxRetries, "max-retries", cfg.MaxRetries, "max retries")
```

If a flag is omitted, the loaded value remains unchanged.

## BuildSystems Registry

```go
type BuildSystemInfo struct {
    Permissions    []string
    InstallCmd     string
    VerifyCommands []string
    InitMarkers    []string
    CommonPitfalls []string
    LintCmd        string
}

var BuildSystems = map[string]BuildSystemInfo{
    "go":     { ... },
    "npm":    { ... },
    "pnpm":   { ... },
    "static": { ... },
}
```

The registry provides the human-facing install/verify guidance injected into `.doug/ACTIVE_TASK.md`, build-system detection markers, and default lint commands.

## DetectBuildSystem

```go
// Precedence: go.mod > pnpm-workspace.yaml > package.json > index.html > ""
func DetectBuildSystem(dir string) string
```

Returns `""` when no marker file is found.

## Key Decisions

- **Missing config is not an error**: Doug should work with zero setup.
- **Pointer-based partial parsing**: required for correct boolean and zero-value overrides.
- **Small config schema**: `.doug/doug.yaml` stores project settings only; Pi routing is source-owned.
- **`execution_mode` is rejected**: catches a stale field with an actionable error.
- **Other retired execution keys are ignored here**: `policy:`, `interaction_mode:`, and `*_agent_command` are not part of `OrchestratorConfig`; `doug upgrade` is the cleanup path.
- **`DetectBuildSystem` returns `""` on no match**: callers choose the fallback.

## Edge Cases & Gotchas

- Config lives at `.doug/doug.yaml`, not the repo root.
- `LoadConfig` does not validate `build_system`; call `(*OrchestratorConfig).Validate()` after CLI overrides.
- `max_retries: 0` is valid and means no retries.
- `agent_heartbeat_seconds: 0` disables heartbeat logging.

## Related Topics

- [cmd/init](init.md) — how new `.doug/doug.yaml` files are generated
- [Interaction Model And Pi Policy Ownership](../features/execution-model.md) — source-owned Pi routing
- [Doug-to-Pi Runtime Contract](../features/pi-runtime-contract.md) — Doug/Pi execution boundary
