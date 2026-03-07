---
title: cmd/switch — Agent Switching Subcommand
updated: 2026-03-06
category: Packages
tags: [switch, agent, yaml, config, cobra]
related_articles:
  - docs/kb/packages/config.md
  - docs/kb/packages/init.md
---

# cmd/switch — Agent Switching Subcommand

## Overview

`cmd/switch.go` implements the `doug switch {agent}` subcommand. It reads `.doug/doug.yaml` into `config.OrchestratorConfig`, updates `agent_command` to the chosen agent's command string, then marshals the struct back to YAML and writes it atomically. The testable core is `switchAgent(projectRoot, agentName string) error`.

## Implementation

```go
func switchAgent(projectRoot, agentName string) error {
    configPath := filepath.Join(projectRoot, ".doug", "doug.yaml")
    cfg, err := config.LoadConfig(configPath)
    // ... update fields from agentRegistry ...
    data, err := yaml.Marshal(cfg)
    return os.WriteFile(configPath, data, 0o644)
}
```

**Agent registry** (`cmd/agents.go`): maps agent names to their `agent_command` strings.

| Agent | `agent_command` |
|-------|----------------|
| `claude` | `claude -p "[DOUG_TASK_ID: {{task_id}}] ..."` |
| `codex` | `codex exec "[DOUG_TASK_ID: {{task_id}}] ..."` |
| `gemini` | `gemini --approval-mode auto_edit --output-format json --sandbox "[DOUG_TASK_ID: {{task_id}}] ..."` |

## Key Decisions

- **Typed struct, not `map[string]interface{}`**: `yaml.Marshal` on `config.OrchestratorConfig` always produces correctly-quoted output. A raw map produced unquoted plain scalars that YAML rejected when `agent_command` contained `[DOUG_TASK_ID: ` (colon-space).

- **`agent_command` single-quoted in `dougYAMLContent`**: The init template uses single-quoted YAML scalars for `agent_command` because the value contains embedded double-quotes and colons. `yaml.Marshal` on the typed struct handles quoting automatically on subsequent `doug switch` calls.

- **All other fields preserved**: `LoadConfig` reads the full `doug.yaml` before the switch. `yaml.Marshal` writes all fields back — `build_system`, `max_retries`, `max_iterations`, `kb_enabled` survive the rewrite unchanged.

- **`skills_dir` removed**: The `SkillsDir` field was removed from `OrchestratorConfig` entirely (it was loaded but never consumed at runtime). `doug switch` no longer sets it.

## Usage Example

```bash
doug switch gemini   # updates agent_command in .doug/doug.yaml
doug switch claude   # switches back
```

## Edge Cases & Gotchas

- **Unknown agent**: returns a descriptive error before touching the file.
- **Missing `doug.yaml`**: `LoadConfig` returns defaults rather than an error; the write then creates a `doug.yaml` with defaults + new agent. Run `doug init` first to avoid this.
- **Round-trip stability**: `yaml.Marshal` on `OrchestratorConfig` is stable across consecutive switches (verified by `TestSwitchAgent_SubsequentSwitch`).

## Related Topics

- [internal/config](config.md) — `OrchestratorConfig` struct, `LoadConfig`, default constants
- [cmd/init](init.md) — generates the initial `doug.yaml`; uses the same single-quoting convention for `agent_command`
