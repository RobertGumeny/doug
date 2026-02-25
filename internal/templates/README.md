# Orchestrator Templates

This directory contains template files for bootstrapping new projects with the autonomous coding orchestrator.

## Quick Start

### One-Liner Setup (Recommended)

```bash
# Clone orchestrator into your project, run setup, auto-cleanup
git clone --depth 1 https://github.com/RobertGumeny/agent-orchestrator.git && agent-orchestrator/setup.sh
```

That's it! The setup script will:

- Copy orchestrator framework to `./orchestrator/`
- Copy all template files to project root
- Install `agent_loop` executable
- Configure Claude Code settings
- Auto-delete orchestrator source for clean result

### Manual Target Directory

If you want to set up in a different location:

```bash
git clone --depth 1 https://github.com/RobertGumeny/agent-orchestrator.git
agent-orchestrator/setup.sh /path/to/myproject
```

## Template Files

### Core Configuration

- **`project-state.yaml`** - Tracks orchestrator state (epic, active task, metrics). Auto-managed by the orchestrator.
- **`tasks.yaml`** - Defines your epic and tasks to implement. **This is where you define your work.**
- **`CLAUDE.md`** - Project-specific instructions for Claude agents. Include coding standards, architecture notes, patterns.
- **`PRD.md`** - Product Requirements Document. Defines product vision, requirements, and technical specifications.
- **`CHANGELOG.md`** - Auto-generated changelog following Keep a Changelog format.
- **`.gitignore`** - Comprehensive gitignore with common patterns for meta-projects.
- **`README.md`** - Project documentation template.

### Claude Code Integration

- **`.claude/settings.json`** - Claude Code permission settings configured for orchestrator workflow.

### Logs Structure

- **`logs/sessions/`** - Session logs from agent runs
- **`logs/bugs/`** - Bug reports with BUG_REPORT_TEMPLATE.md
- **`logs/failures/`** - Failure reports with FAILURE_REPORT_TEMPLATE.md

## Project Structure After Setup

```
/myproject/
├── agent_loop               # Main executable - run this!
├── project-state.yaml       # Orchestrator state (auto-managed)
├── tasks.yaml               # Your epic definition (you edit this)
├── CLAUDE.md               # Project instructions for agents
├── PRD.md                  # Product requirements document
├── CHANGELOG.md            # Auto-generated changelog
├── README.md               # Project documentation
├── .gitignore              # Git ignore patterns
├── .claude/
│   └── settings.json       # Claude Code permissions
├── logs/
│   ├── sessions/           # Agent session logs
│   ├── bugs/               # Bug reports
│   └── failures/           # Failure reports
├── docs/
│   └── kb/                 # Knowledge base (auto-generated)
├── orchestrator/           # Framework (don't modify)
│   ├── lib/
│   ├── core/
│   ├── agent/
│   ├── handlers/
│   └── skills/
└── [your project files]
```

## Usage

### Running the Agent Loop

```bash
./agent_loop 5   # Process up to 5 tasks
./agent_loop 1   # Process 1 task (good for testing)
./agent_loop 10  # Process up to 10 tasks
```

### Workflow

1. **First Run**: The orchestrator bootstraps state from `tasks.yaml`
2. **Epic Branch**: Creates a feature branch (e.g., `feature/EPIC-1`)
3. **Task Execution**: Works through tasks sequentially with verification
4. **Atomic Commits**: Each task creates a commit with CHANGELOG entry
5. **KB Synthesis**: After tasks complete, generates knowledge base docs (optional)

### Editing Your Epic

Edit `tasks.yaml` to define your epic and tasks:

```yaml
epic:
  id: EPIC-1
  title: "My Feature"
  description: "Description of what we're building"
  status: IN_PROGRESS

  tasks:
    - id: TASK-1
      type: feature
      title: "Implement core logic"
      description: "Detailed task description"
      acceptance_criteria:
        - "Criterion 1"
        - "Criterion 2"
      status: TODO
```

## Configuration

### Disable Knowledge Base Synthesis

If you don't want automatic documentation generation, set in `project-state.yaml`:

```yaml
kb_enabled: false
```

### Max Iterations

Control how many tasks to process per run via command-line argument:

```bash
./agent_loop 5   # Process up to 5 tasks
```

## Requirements

- `yq` (YAML processor)
- `claude` CLI (Anthropic's Claude command-line tool)
- `git`
- Project-specific tools (`npm`, `go`, `python`, etc.)

## Distribution Model

The orchestrator uses **GitHub + git clone** as its distribution method:

- ✓ Standard, familiar workflow
- ✓ Version controlled releases
- ✓ No .git contamination (setup only copies specific directories)
- ✓ Always get latest when cloning fresh

## Setting Up Multiple Projects

For each new project, simply clone fresh:

```bash
cd /path/to/new-project
git clone --depth 1 https://github.com/RobertGumeny/agent-orchestrator.git && agent-orchestrator/setup.sh
```

The orchestrator source is auto-deleted after setup, keeping your project clean.
