# Agent Onboarding Guide

You will be assigned a specific task. Complete it, then write your session summary.

## Where to Find Context

project-state.yaml + tasks.yaml — your current task and project state
PRD.md — requirements, acceptance criteria, architectural decisions
docs/kb/ — patterns, infrastructure, and lessons learned for this project

What You Can Touch

You may:

Read any of the above files
Modify source and test files
Run build, test, and lint commands

❌ You must never:

Run Git commands
Modify project-state.yaml or tasks.yaml
Move, delete, or create files in logs/

When You're Blocked

Check PRD.md → your skill file → docs/kb/ → existing code patterns
Unresolvable ambiguity → write logs/ACTIVE_FAILURE.md (not logs/failures/), set outcome: FAILURE, stop
Blocking bug unrelated to your task → write logs/ACTIVE_BUG.md (not logs/bugs/), set outcome: BUG, stop

Never guess on architectural or business logic decisions. Escalate instead.

Session Summary
Your activation prompt provides the path to your pre-created session summary file. Fill it out when your task is complete — do not create a new file.
Valid outcomes: SUCCESS | FAILURE | BUG | EPIC_COMPLETE

## Platform Notes

**Windows**: The Bash tool is unavailable when running Claude Code natively on
Windows. Agents cannot run shell commands. Use WSL2 to run doug on Windows:

1. Install WSL2 and a Linux distribution (Ubuntu recommended)
2. Run all doug commands from a WSL2 terminal
3. Ensure `claude`, `git`, and your toolchain are installed inside WSL2
