---
title: cmd/switch — Agent Switching Subcommand
updated: 2026-05-04
category: Packages
tags: [switch, agent, yaml, config, cobra]
related_articles:
  - docs/kb/packages/config.md
  - docs/kb/packages/init.md
  - docs/kb/patterns/pattern-best-effort-writes.md
---

# cmd/switch — Agent Switching Subcommand

## Overview

`cmd/switch.go` implements the `doug switch {agent}` subcommand. It reads `.doug/doug.yaml` into `config.OrchestratorConfig`, updates all four mode-specific command fields (`run_agent_command`, `plan_agent_command`, `scaffold_agent_command`, `research_agent_command`) to the chosen agent's command strings, then marshals the struct back to YAML and writes it atomically. The testable core is `switchAgent(projectRoot, agentName string) error`.

## Implementation

```go
func switchAgent(projectRoot, agentName string) error {
    configPath := filepath.Join(projectRoot, ".doug", "doug.yaml")
    data, err := os.ReadFile(configPath)
    var cfg config.OrchestratorConfig
    yaml.Unmarshal(data, &cfg)
    // update all four mode-specific commands from agentRegistry
    cfg.RunAgentCommand = info.runCommand
    cfg.PlanAgentCommand = info.planCommand
    cfg.ScaffoldAgentCommand = info.scaffoldCommand
    cfg.ResearchAgentCommand = info.researchCommand
    out, _ := yaml.Marshal(&cfg)
    return state.AtomicWrite(configPath, out)
}
```

**Agent registry** (`cmd/agents.go`): maps agent names to four mode-specific command strings.

| Agent | Modes updated |
|-------|--------------|
| `claude` | `run_agent_command`, `plan_agent_command`, `scaffold_agent_command`, `research_agent_command` |
| `codex` | same four fields |
| `gemini` | same four fields |

Each command template contains `{{task_id}}` and `{{skill_name}}` placeholders resolved by `agent.PrepareExecution` before dispatch. The run, scaffold, and research commands use the runtime prompt; the plan command uses a planning-specific prompt; the research command uses a read-only research-focused prompt.

## Key Decisions

- **Typed struct, not `map[string]interface{}`**: `yaml.Marshal` on `config.OrchestratorConfig` always produces correctly-quoted output. A raw map produced unquoted plain scalars that YAML rejected when command strings contained `[DOUG_TASK_ID: ` (colon-space).

- **Four-command model**: `switchAgent` updates all four mode-specific fields (`RunAgentCommand`, `PlanAgentCommand`, `ScaffoldAgentCommand`, `ResearchAgentCommand`) in one atomic write. Adding a new Doug workflow command requires updating `agentInfo` in `cmd/agents.go`, `AgentCommandSet` in `internal/config/agent_commands.go`, and `OrchestratorConfig` in `internal/config/config.go`.

- **All other fields preserved**: `yaml.Unmarshal` then `yaml.Marshal` on the typed struct round-trips the full `doug.yaml` — `build_system`, `max_retries`, `max_iterations`, `kb_enabled`, `policy` survive the rewrite unchanged.

- **`skills_dir` removed**: The `SkillsDir` field was removed from `OrchestratorConfig` entirely (it was loaded but never consumed at runtime). `doug switch` no longer sets it.

## Usage Example

```bash
doug switch gemini   # updates all four *_agent_command fields in .doug/doug.yaml
doug switch claude   # switches back
```

## Edge Cases & Gotchas

- **Unknown agent**: returns a descriptive error before touching the file.
- **`--list` output is best-effort**: supported-agent listing goes through the shared `cmd` output helper and intentionally ignores write errors. See [Best-Effort Terminal & Writer Output](../patterns/pattern-best-effort-writes.md).
- **Planning command defaults**: generated plan commands now route the agent through `.doug/ACTIVE_TASK.md` and describe `.doug/plan/PLAN.md` as the editable planning workbook, so switch-driven rewrites preserve the universal canonical brief contract.
- **Round-trip stability**: `yaml.Marshal` on `OrchestratorConfig` is stable across consecutive switches (verified by `TestSwitchAgent_SubsequentSwitch`).

## Related Topics

- [internal/config](config.md) — `OrchestratorConfig` struct, `LoadConfig`, default constants
- [cmd/init](init.md) — generates the initial `doug.yaml`; uses the same single-quoting convention for the mode-specific `*_agent_command` fields
