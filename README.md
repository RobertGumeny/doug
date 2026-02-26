# doug

**doug** is a CLI orchestrator that drives AI coding agents (Claude Code, Aider, etc.) through a structured task loop. It scaffolds your project, manages task state in YAML, and gives agents the context they need to work reliably across sessions without manual intervention.

---

## Install

### From a release binary

Download the latest binary for your platform from the [releases page](https://github.com/robertgumeny/doug/releases), unzip it, and move it somewhere on your `$PATH`:

```bash
# macOS arm64
curl -L https://github.com/robertgumeny/doug/releases/latest/download/doug_Darwin_arm64.tar.gz | tar xz
sudo mv doug /usr/local/bin/

# Linux amd64
curl -L https://github.com/robertgumeny/doug/releases/latest/download/doug_Linux_amd64.tar.gz | tar xz
sudo mv doug /usr/local/bin/
```

### go install

```bash
go install github.com/robertgumeny/doug@latest
```

### Build from source

Requires Go 1.21+:

```bash
git clone https://github.com/robertgumeny/doug.git
cd doug
go build -o doug .
```

---

## doug init walkthrough

`doug init` scaffolds a new project in the current directory. Run it once, then edit the generated files.

```bash
mkdir my-project && cd my-project
git init
doug init
```

Example output:

```
✓ created doug.yaml
✓ created tasks.yaml
✓ created project-state.yaml
✓ created PRD.md
✓ created CLAUDE.md
✓ created .claude/settings.json
✓ created .claude/skills-config.yaml
✓ created .claude/skills/implement-feature/SKILL.md
✓ created .claude/skills/implement-bugfix/SKILL.md
✓ created .claude/skills/implement-documentation/SKILL.md
✓ created logs/SESSION_RESULTS_TEMPLATE.md
✓ created logs/BUG_REPORT_TEMPLATE.md
✓ created logs/FAILURE_REPORT_TEMPLATE.md
project initialized — edit doug.yaml and tasks.yaml, then run: doug run
```

**Next steps after init:**

1. Edit `PRD.md` — describe your product and architecture
2. Edit `tasks.yaml` — define your epic and tasks
3. Edit `doug.yaml` — set your agent command and build system
4. Run `doug run`

---

## doug run usage

```bash
doug run [flags]
```

**What it does (in order):**

1. Loads `doug.yaml` and applies any CLI flag overrides
2. Verifies that the agent binary, `git`, and your toolchain are on PATH
3. Loads `project-state.yaml` and `tasks.yaml`
4. Bootstraps state on first run (reads epic and task IDs from `tasks.yaml`)
5. Exits immediately if all tasks are already DONE
6. Runs a pre-flight build and test to verify the project compiles
7. Checks out the epic feature branch (creates it if needed)
8. Aligns task pointers with the current task list
9. Enters the main loop (up to `max_iterations`):
   - Creates a session file for the agent to write its result
   - Writes `logs/ACTIVE_TASK.md` with task metadata and skill instructions
   - Invokes the agent
   - Reads the session result and dispatches to a handler (SUCCESS / FAILURE / BUG)
   - On SUCCESS: verifies build+tests, marks task DONE, commits, advances to next task
   - On FAILURE: retries up to `max_retries`; marks BLOCKED after that
   - On BUG: schedules a bugfix task as the next iteration
10. Exits 0 when all work is done or `max_iterations` is reached

**Flags:**

| Flag | Description |
|------|-------------|
| `--agent <cmd>` | Override `agent_command` from `doug.yaml` |
| `--build-system <go\|npm>` | Override `build_system` from `doug.yaml` |
| `--max-retries <n>` | Override `max_retries` from `doug.yaml` |
| `--max-iterations <n>` | Override `max_iterations` from `doug.yaml` |
| `--kb-enabled=<bool>` | Override `kb_enabled` from `doug.yaml` |

---

## doug.yaml reference

```yaml
# doug.yaml — orchestrator configuration

# Command used to invoke the agent.
# For Claude Code: "claude"
# For Aider: "aider --yes"
agent_command: claude

# Build system: "go" or "npm"
# Auto-detected by init based on go.mod / package.json.
build_system: go

# Maximum number of FAILURE outcomes before a task is marked BLOCKED.
# Blocked tasks require human intervention.
max_retries: 5

# Maximum number of orchestration loop iterations before exiting.
# Prevents infinite loops. Exit code is 0 when this limit is hit.
max_iterations: 20

# If true, inject a KB synthesis documentation task after all feature tasks complete.
# The documentation agent synthesizes session logs into docs/kb/.
kb_enabled: true
```

---

## tasks.yaml format

```yaml
epic:
  id: "EPIC-1"           # Unique ID; used as branch prefix and log directory name
  name: "First Epic"     # Human-readable name

  tasks:
    - id: "EPIC-1-001"
      type: "feature"    # feature | bugfix | documentation | manual_review
      status: "TODO"     # TODO | IN_PROGRESS | DONE | BLOCKED
      description: "Implement the first feature of the project."
      acceptance_criteria:
        - "The feature is implemented and all related tests pass"
        - "Code follows the project's conventions and style guidelines"

    - id: "EPIC-1-002"
      type: "feature"
      status: "TODO"
      description: "Implement the second feature of the project."
      acceptance_criteria:
        - "The feature is implemented and all related tests pass"
        - "All acceptance criteria have been verified end-to-end"
```

**Status values:**

| Status | Meaning |
|--------|---------|
| `TODO` | Not yet started |
| `IN_PROGRESS` | Agent is currently working on it (or orchestrator was interrupted) |
| `DONE` | Completed successfully |
| `BLOCKED` | Failed `max_retries` times; requires human intervention |

**Task types:**

| Type | Description |
|------|-------------|
| `feature` | User-defined feature task |
| `bugfix` | Orchestrator-injected when an agent reports a blocking bug |
| `documentation` | Orchestrator-injected KB synthesis task (when `kb_enabled: true`) |
| `manual_review` | Requires human review; orchestrator stops execution |

---

## Agent contract

Agents communicate with the orchestrator through two files: `logs/ACTIVE_TASK.md` (input, written by the orchestrator) and a session result file (output, written by the agent).

### ACTIVE_TASK.md

Written by the orchestrator before each agent invocation. Contains:

```markdown
# Active Task

**Task ID**: EPIC-1-001
**Task Type**: feature
**Session File**: /path/to/logs/sessions/EPIC-1/session-EPIC-1-001_attempt-1.md
**Attempt**: 1 of 5
**Description**: Implement the first feature of the project.

**Acceptance Criteria**:
- The feature is implemented and all related tests pass
- Code follows the project's conventions and style guidelines

---

[Skill instructions follow]
```

### Session result file

The agent writes its result to the path specified in `**Session File**:`. The orchestrator requires exactly three fields in the YAML front-matter:

```yaml
---
outcome: "SUCCESS"
changelog_entry: "Added user authentication with JWT tokens"
dependencies_added: []
---

## Implementation Summary
[Agent notes here — ignored by orchestrator]
```

**Required fields:**

| Field | Type | Description |
|-------|------|-------------|
| `outcome` | string | `SUCCESS` \| `FAILURE` \| `BUG` \| `EPIC_COMPLETE` |
| `changelog_entry` | string | User-facing description of the change (for `CHANGELOG.md`) |
| `dependencies_added` | list | New package dependencies to install before build verification |

**Outcome values:**

| Outcome | What happens next |
|---------|-------------------|
| `SUCCESS` | Orchestrator verifies build+tests, marks task DONE, commits, advances |
| `FAILURE` | Orchestrator retries; marks BLOCKED after `max_retries` |
| `BUG` | Orchestrator schedules a bugfix task; agent must write `logs/ACTIVE_BUG.md` |
| `EPIC_COMPLETE` | Orchestrator finalizes the epic (commits, closes branch) |

---

## Trust boundary

The orchestrator owns:

- All Git operations (branch creation, commit, rollback)
- Updating `project-state.yaml` and `tasks.yaml`
- Updating `CHANGELOG.md`
- Archiving session and bug report files in `logs/`

Agents own:

- Writing source code and tests
- Running build, test, and lint commands
- Writing the session result file
- Writing `logs/ACTIVE_BUG.md` (on bug discovery)
- Writing `logs/ACTIVE_FAILURE.md` (on unresolvable failure)

**Why agents cannot touch YAML or Git:** Agents are stateless processes invoked by the orchestrator. If an agent modified `project-state.yaml` or committed changes, the orchestrator would lose its place and state would diverge. The deny list in `.claude/settings.json` enforces this boundary by blocking reads of the state files (so agents cannot accidentally act on stale state) and all Git write operations.

---

## Platform support

| Platform | Status | Notes |
|----------|--------|-------|
| Linux | Supported | All features available |
| macOS | Supported | All features available |
| Windows (native) | Not supported | Claude Code's Bash tool is unavailable on native Windows |
| Windows (WSL2) | Supported | Recommended path for Windows users |

---

## WSL2 setup guide (Windows)

Claude Code agents require a working Bash environment. On Windows, use WSL2:

1. **Install WSL2** — open PowerShell as Administrator and run:
   ```powershell
   wsl --install
   ```
   Restart when prompted.

2. **Open a WSL2 terminal** — launch Ubuntu (or your chosen distro) from the Start menu.

3. **Install Go** (if using a Go project):
   ```bash
   sudo apt update && sudo apt install -y golang-go
   ```

4. **Install Claude Code**:
   ```bash
   npm install -g @anthropic-ai/claude-code
   ```

5. **Install doug**:
   ```bash
   go install github.com/robertgumeny/doug@latest
   ```

6. **Clone your project inside WSL2** (not on the Windows filesystem) and run:
   ```bash
   cd ~/my-project
   git init
   doug init
   doug run
   ```
