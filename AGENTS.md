# Agents

This project uses [doug](https://github.com/robertgumeny/doug) for agentic task orchestration.

## Available Skills

| Skill | Trigger | Description |
|-------|---------|-------------|
| `implement-feature` | `active_task.type: feature` | Full feature implementation: research, plan, code, test, report |
| `implement-bugfix` | `active_task.type: bugfix` | Root cause analysis, fix, regression test, report |
| `implement-documentation` | `active_task.type: documentation` | Synthesize session logs into `docs/kb/` knowledge base |

Skill files live in `.claude/skills/`. The orchestrator passes the correct skill to the agent via `logs/ACTIVE_TASK.md`.

## Agent Contract

The orchestrator requires exactly three things from an agent:

1. A command to invoke (`agent_command` in `doug.yaml`)
2. A pre-created briefing file to read before starting (`logs/ACTIVE_TASK.md`)
3. A session result file to write after completing (path given inside `ACTIVE_TASK.md`)

Agents are stateless — they read YAML and code, write code and session results, and never touch Git or YAML state files.

## Failure Escalation

| Outcome | When to use | What to write |
|---------|-------------|---------------|
| `SUCCESS` | Task complete, build and tests pass | Session result only |
| `BUG` | Blocking bug found in unrelated code | `logs/ACTIVE_BUG.md` + session result |
| `FAILURE` | Cannot complete after 5 attempts | `logs/ACTIVE_FAILURE.md` + session result |
| `EPIC_COMPLETE` | All tasks in epic are DONE | Session result only |
