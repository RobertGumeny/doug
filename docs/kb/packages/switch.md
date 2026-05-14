---
title: cmd/switch — Agent Switching Subcommand
updated: 2026-05-13
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

The supported operator contract is narrow on purpose: `doug switch` manages preset command templates only. It does not install provider files, it does not select skills, and it does not change the backend transport. Those are handled elsewhere (`doug init` scaffolding and `policy.*.execution_mode`).

## Implementation

```go
func switchAgent(projectRoot, agentName string) error {
    configPath := filepath.Join(projectRoot, ".doug", "doug.yaml")
    data, err := os.ReadFile(configPath)
    var cfg config.OrchestratorConfig
    yaml.Unmarshal(data, &cfg)
    // update all four mode-specific commands from agentRegistry
    cfg.RunAgentCommand = set.Run
    cfg.PlanAgentCommand = set.Plan
    cfg.ScaffoldAgentCommand = set.Scaffold
    cfg.ResearchAgentCommand = set.Research
    out, _ := yaml.Marshal(&cfg)
    return state.AtomicWrite(configPath, out)
}
```

**Agent registry** (`internal/config/agent_commands.go`): the single authoritative source for all supported agents and their four mode-specific command templates. To add a new agent, add one entry to `AgentCommandSets` — no other files need updating for registration.

| Agent | Modes updated | Command style |
|-------|--------------|---------------|
| `claude` | `run_agent_command`, `plan_agent_command`, `scaffold_agent_command`, `research_agent_command` | CLI subprocess (`claude -p "..."`) |
| `codex` | same four fields | CLI subprocess (`codex exec "..."`) |
| `gemini` | same four fields | CLI subprocess (`gemini --approval-mode ... "..."`) |
| `pi` | same four fields | Prompt-only (no CLI prefix; sent as RPC message payload) |

Each command template contains `{{task_id}}` and `{{skill_name}}` placeholders resolved by `agent.PrepareExecution` before dispatch. The run, scaffold, and research commands use the runtime prompt; the plan command uses a planning-specific prompt; the research command uses a read-only research-focused prompt.

**Pi commands are prompt-only**: Unlike other agents whose commands include a CLI binary prefix, Pi's commands contain only the prompt text (no `pi ...` prefix). `piCLILauncher` handles the `pi --mode rpc` invocation itself and sends the resolved command string as the RPC message payload. When switching to Pi, users should also configure `execution_mode: rpc` in their `doug.yaml` policy — this is generated automatically by `doug init --agents pi` but must be added manually when using `doug switch pi`.

**Preset selection is not backend selection**: `doug switch pi` makes Pi's prompt payloads the active command templates, but Doug still uses the subprocess backend unless `policy.phases.*.execution_mode` or `policy.tasks.*.execution_mode` resolves to `rpc`. This separation is the supported product model today.

## Key Decisions

- **Typed struct, not `map[string]interface{}`**: `yaml.Marshal` on `config.OrchestratorConfig` always produces correctly-quoted output. A raw map produced unquoted plain scalars that YAML rejected when command strings contained `[DOUG_TASK_ID: ` (colon-space).

- **Four-command model**: `switchAgent` updates all four mode-specific fields (`RunAgentCommand`, `PlanAgentCommand`, `ScaffoldAgentCommand`, `ResearchAgentCommand`) in one atomic write. Adding a new agent requires only adding one entry to `AgentCommandSets` in `internal/config/agent_commands.go`.

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
- **No `.pi` extension coupling**: `doug switch` does not inspect or modify `.pi/extensions/*`. Pi extension surfaces are scaffold-time conveniences, not part of the switch contract.

## Follow-Up Notes

- If Doug later offers a single command that both rewrites Pi presets and enables `execution_mode: rpc`, that should be treated as an additive UX feature. The current split is intentional and documented.
- If future Pi integration adds extension-driven runtime artifact ownership, document that in `internal/agent` and `cmd/init` KB pages first. It is not part of `doug switch` today.

## Related Topics

- [internal/config](config.md) — `OrchestratorConfig` struct, `LoadConfig`, default constants
- [cmd/init](init.md) — generates the initial `doug.yaml`; uses the same single-quoting convention for the mode-specific `*_agent_command` fields
