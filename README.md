# doug

**doug** is a CLI tool for running AI agent workflows on your codebase. It scaffolds project structure, manages task state, and provides the conventions that Claude Code agents need to work reliably across sessions.

> Full documentation is coming. This README covers the essentials to get started.

---

## Install

### From a release binary

Download the latest binary for your platform from the [releases page](https://github.com/robertgumeny/doug/releases), unzip it, and move it somewhere on your `$PATH`:

```bash
# Example for macOS arm64
curl -L https://github.com/robertgumeny/doug/releases/latest/download/doug_Darwin_arm64.tar.gz | tar xz
mv doug /usr/local/bin/
```

### Build from source

Requires Go 1.21+:

```bash
git clone https://github.com/robertgumeny/doug.git
cd doug
make build
# Puts a `doug` binary in the current directory
```

---

## Usage

```
doug [command]

Available Commands:
  init        Initialize a new project
  run         Run a task
  help        Help about any command

Flags:
  --version   Print the version
  --help      Show help
```

### Initialize a project

```bash
cd my-project
doug init
```

Creates the directory structure, config templates, and `.gitignore` entries that doug agents expect.

### Run a task

```bash
doug run
```

Picks up the current task from `project-state.yaml` and invokes the appropriate agent workflow.

---

## Project files

After `doug init`, your project will have:

| File | Purpose | Committed? |
|------|---------|------------|
| `tasks.yaml` | Your task backlog | Yes |
| `doug.yaml` | Project config | Yes |
| `project-state.yaml` | Runtime state (managed by doug) | No |
| `logs/` | Session logs and agent output | No |

---

## License

MIT
