# .doug/

This directory is managed by [doug](https://github.com/robertgumeny/doug), an orchestrator for coding agents.

## What's here

| File / Directory | Purpose |
|---|---|
| `doug.yaml` | Project configuration — agent command, build system, retry limits |
| `tasks.yaml` | Task backlog generated from a planning handoff |
| `project-state.yaml` | Runtime state tracked by the orchestrator — do not edit manually |
| `PRD.md` | Product requirements, read by agents for context during runs |
| `ACTIVE_TASK.md` | The current task brief, written fresh before each agent run |
| `plan/PLAN.md` | Planning workbook, used during `doug plan` sessions |
| `logs/` | Archived task results, bug reports, and session history |

## Common commands

```
doug run          # run the next pending task
doug plan         # start a planning session
doug handoff      # convert a finished plan into a task backlog
doug research     # run a read-only research session
doug revert       # revert a completed task
```

## What you can edit

- `doug.yaml` — adjust config as needed
- `PRD.md` — keep product requirements up to date
- `plan/PLAN.md` — your working artifact during a `doug plan` session

## What doug manages

Don't edit `project-state.yaml` or `ACTIVE_TASK.md` by hand — doug rewrites them on each run.
