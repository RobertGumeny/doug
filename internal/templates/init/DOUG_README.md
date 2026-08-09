# .doug/

This directory is managed by [doug](https://github.com/robertgumeny/doug), an orchestrator for coding agents.

## What's here

| File / Directory | Purpose |
|---|---|
| `doug.yaml` | Project configuration — build-system and phase policy |
| `tasks.yaml` | Active runtime task backlog generated from a planning handoff |
| `project-state.yaml` | Runtime state tracked by the orchestrator — do not edit manually |
| `PRD.md` | Product requirements, read by agents for context during runs |
| `ACTIVE_TASK.md` | The current task brief, written fresh before each agent run |
| `intake/` | Doug-managed planning intake such as reported bugs and `research/` reports |
| `logs/epics/` | Completed epic snapshots, attempt archives, review artifacts, and stats |
| `templates/` | Optional project-local templates or reference material |
| `run.lock` | Advisory lock used while mutating Doug lifecycle state |
| `plan/PLAN.md` | Planning workbook, used during `doug plan` sessions |
| `plan/epics/` | Handoff output packaged for execution by epic |
| `plan/history/` | Archived planning workbooks from completed handoffs |

## Common commands

```
doug run          # run the next pending task
doug plan         # start a planning session
doug handoff      # convert a finished plan into a task backlog
doug research     # run a read-only research session
doug stats        # summarize local run statistics
doug revert       # revert a completed task
```

## What you can edit

- `doug.yaml` — adjust config as needed
- `PRD.md` — keep product requirements up to date
- `plan/PLAN.md` — your working artifact during a `doug plan` session
- `intake/bugs/` — reported-bug template plus durable bug reports that Doug-managed sessions archive for later planning
- `intake/research/` — research reports that should appear as planning candidates

For bugs found during a Doug-managed task, report them in `.doug/ACTIVE_TASK.md` through the structured `bugs:` result field instead of writing ad-hoc ledger files. For bugs found outside a scheduled run, use `doug research` for a focused investigation report and then `doug plan` to decide whether the finding becomes planned work; Doug intentionally does not provide a separate `doug bug` command.

## What doug manages

Don't edit `project-state.yaml` or `ACTIVE_TASK.md` by hand — doug rewrites them on each run.
