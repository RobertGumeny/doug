---
title: internal/config — OrchestratorConfig
updated: 2026-02-24
category: Packages
tags: [config, yaml, defaults, build-system, cobra]
related_articles:
  - docs/kb/infrastructure/go.md
  - docs/kb/packages/types.md
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
    DefaultAgentCommand  = "claude"
    DefaultBuildSystem   = "go"
    DefaultMaxRetries    = 5
    DefaultMaxIterations = 20
    DefaultKBEnabled     = true
)
```

## OrchestratorConfig Fields

| Field | Default | Source |
|-------|---------|--------|
| `AgentCommand` | `"claude"` | `doug.yaml` → CLI flag |
| `BuildSystem` | `"go"` | `doug.yaml` → CLI flag |
| `MaxRetries` | `5` | `doug.yaml` → CLI flag |
| `MaxIterations` | `20` | `doug.yaml` → CLI flag |
| `KBEnabled` | `true` | `doug.yaml` → CLI flag |

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
```

When a flag is provided on the command line, cobra overwrites the field. If the flag is omitted, cobra leaves the field unchanged (already set to the config-file or default value). This gives flags the highest precedence at zero extra cost.

## Partial Config Pattern

The internal `partialConfig` struct uses pointer fields to distinguish "absent" from "zero value":

```go
// yaml:"-" equivalent: only non-nil fields override defaults
type partialConfig struct {
    AgentCommand  *string `yaml:"agent_command"`
    KBEnabled     *bool   `yaml:"kb_enabled"`
    // ...
}
```

**Why this matters**: `kb_enabled: false` in the config file is a valid override, not an absent field. If `OrchestratorConfig` were unmarshalled directly, `false` would be indistinguishable from a missing field, and the default `true` would win. The pointer layer preserves intent.

## DetectBuildSystem

```go
// Precedence: go.mod > package.json > "go" (safe default)
func DetectBuildSystem(dir string) string
```

| Condition | Returns |
|-----------|---------|
| `go.mod` exists | `"go"` |
| `package.json` exists (no `go.mod`) | `"npm"` |
| Neither exists | `"go"` |

Used by `doug init` to auto-populate `build_system` in the generated `doug.yaml`. Not called at runtime — config file takes precedence once generated.

## Key Decisions

**Missing file is not an error**: `doug` should work out of the box with zero configuration. A missing `doug.yaml` returns the same defaults as an empty one.

**Pointer-based partial parsing**: Required to handle boolean `false` correctly. Any alternative (e.g. checking if a field equals its zero value) would be fragile and break for `max_retries: 0` or `max_iterations: 0`.

**Exported default constants**: Tests reference `config.DefaultMaxRetries` rather than hardcoding `5`. This prevents tests from silently passing when defaults change.

**`go` wins over `npm` in `DetectBuildSystem`**: Doug is a Go tool and the Go build system is more common. A project with both files is likely a Go project with a JS toolchain layer on top.

## Edge Cases & Gotchas

**`doug.yaml` vs `doug.yaml` (case-sensitivity)**: On case-insensitive filesystems (macOS default, Windows), `Doug.yaml` will be found. On Linux (case-sensitive), it won't. Always use lowercase `doug.yaml`.

**`build_system` is not validated by `LoadConfig`**: Unknown values (e.g. `build_system: python`) are accepted without error. The build system package is responsible for validating the value and returning an actionable error.

**Zero `MaxRetries`**: If `max_retries: 0` is set in `doug.yaml`, `LoadConfig` correctly returns `MaxRetries: 0`. The orchestrator treats this as "no retries allowed" — a task fails on the first FAILURE outcome. This is a valid configuration for strict environments.

## Related Topics

- [Go Infrastructure](../infrastructure/go.md) — build system and project conventions
- [Types](types.md) — TaskType constants used by the config system
