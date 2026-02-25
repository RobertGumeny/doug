# Research Report: Windows & Cross-Platform Compatibility — Public Release Readiness

**Generated**: 2026-02-25
**Scope Type**: Feature/Module
**Related Epic**: EPIC-6 (post-completion) — pre-publication planning
**Related Tasks**: None (forward-looking research for README and `doug init` improvements)

---

## Overview

The `doug` Go binary is architecturally cross-platform — all orchestrator operations use `exec.Command` directly with no shell dependencies. The only platform-specific friction is the Claude Code CLI's Bash tool sandbox, which fails on Windows/MSYS2 with `EINVAL` before any command executes. This report documents what works, what doesn't, why, and provides a grounded basis for writing user-facing documentation and implementing a Windows-friendly `doug init` path.

---

## File Manifest

| File | Purpose |
| --- | --- |
| `go.mod` | Go version requirement (`go 1.26`), external dependencies |
| `cmd/init.go` | `doug init` — project scaffolding; the primary onboarding surface |
| `cmd/run.go` | Orchestration loop; all subprocess calls via `exec.Command` |
| `internal/config/config.go` | `DetectBuildSystem`, `OrchestratorConfig`, defaults |
| `internal/build/build.go` | `GoBuildSystem` — `go build ./...`, `go test ./...`, `go mod download` |
| `internal/git/git.go` | All git operations — `exec.Command("git", ...)` throughout |
| `internal/agent/invoke.go` | `RunAgent` — spawns the agent command via `exec.Command` |
| `internal/orchestrator/startup.go` | `CheckDependencies` — verifies `claude`, `git`, `go` on PATH |
| `internal/templates/init/CLAUDE.md` | Template CLAUDE.md copied to new projects by `doug init` |
| `internal/templates/init/skills/` | Skill files copied to `.claude/skills/` by `doug init` |
| `.claude/settings.json` | Current project's Claude Code permission config (not in templates yet) |
| `PRD.md` | States cross-platform distribution as primary motivation for Go port |

---

## The Core Problem: Claude Code's Bash Tool on Windows

### What fails and why

When Claude Code (the `claude` CLI) runs as an agent subprocess, it attempts to capture Bash tool output by opening a temp file:

```
EINVAL: invalid argument, open
'C:\Users\<user>\AppData\Local\Temp\claude\<project>\tasks\<id>.output'
```

This failure occurs **before any shell command runs** — it is the output-capture pipe setup that fails, not the command itself. On Windows/MSYS2, the syscall used to create this temp file handle is incompatible with the MSYS2 runtime. The result is that **all Bash tool calls fail**, consistently and completely, regardless of the command.

### What this means for agents

The Claude Code skill files (`implement-feature/SKILL.md`) instruct agents to "Run build, test, and lint commands" in Phase 4: Verify. On Windows/MSYS2, agents cannot execute any shell commands. They must rely entirely on file-based tools:

- ✅ `Read`, `Write`, `Edit` — native Claude Code tools, unaffected
- ✅ `Glob`, `Grep` — native Claude Code tools, unaffected
- ❌ `Bash(go test ./...)` — fails with EINVAL
- ❌ `Bash(go build ./...)` — fails with EINVAL
- ❌ `Bash(mkdir ...)` — fails with EINVAL

### What this does NOT affect

The `doug` binary itself is fully functional on Windows. All orchestrator operations use `exec.Command` directly from Go — not the Bash tool:

| Operation | Mechanism | Windows status |
|-----------|-----------|---------------|
| `go build ./...` (post-task verify) | `GoBuildSystem.Build()` → `exec.Command("go", "build", "./...")` | ✅ Works |
| `go test ./...` (post-task verify) | `GoBuildSystem.Test()` → `exec.Command("go", "test", "./...")` | ✅ Works |
| `git commit`, `git checkout`, etc. | `internal/git/git.go` → `exec.Command("git", ...)` | ✅ Works |
| Agent invocation | `agent.RunAgent` → `exec.Command("claude", ...)` | ✅ Works |
| `doug init` scaffolding | Pure Go file I/O | ✅ Works |

**Key insight**: The orchestrator's `HandleSuccess` independently runs `Build()` and `Test()` after every agent session, compensating for the agent's inability to self-verify. The workflow is still sound on Windows — agents just can't run their own confirmation step.

---

## Platform Matrix

| Platform | Orchestrator | Agent Bash Tool | Recommended? |
|----------|-------------|-----------------|--------------|
| Linux (native) | ✅ Full | ✅ Full | ✅ Yes |
| macOS (native) | ✅ Full | ✅ Full | ✅ Yes |
| WSL2 (Windows) | ✅ Full | ✅ Full | ✅ Yes — primary Windows path |
| Windows/MSYS2 (Git Bash) | ✅ Full | ❌ EINVAL | ⚠️ Functional with workaround |
| Windows/PowerShell | ✅ Full | ❌ Not supported | ❌ No |
| WSL1 | Unknown | ❌ Not supported by Claude Code | ❌ No |

---

## WSL2 Setup Path

### Prerequisites

From `go.mod`: `go 1.26` is required.
From `internal/orchestrator/startup.go:CheckDependencies`: the orchestrator verifies `claude`, `git`, and `go` are on PATH at startup and exits with a clear error listing anything missing.

### Step-by-step WSL2 setup for a new user

```
1. Enable WSL2
   wsl --install                          # Windows PowerShell (admin)
   # Reboot, then open Ubuntu terminal

2. Install Go 1.26+
   wget https://go.dev/dl/go1.26.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.26.linux-amd64.tar.gz
   echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
   source ~/.bashrc
   go version                             # verify

3. Install Node.js (required for Claude Code CLI)
   curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
   sudo apt-get install -y nodejs

4. Install Claude Code
   npm install -g @anthropic-ai/claude-code
   claude --version                       # verify

5. Install git (usually pre-installed in Ubuntu)
   sudo apt-get install -y git

6. Install doug
   go install github.com/robertgumeny/doug@latest    # once published
   # or build from source:
   git clone https://github.com/robertgumeny/doug
   cd doug && go build -o /usr/local/bin/doug .

7. Initialize a new project
   mkdir my-project && cd my-project
   git init
   doug init
   # Edit doug.yaml and tasks.yaml, then:
   doug run
```

### WSL2 filesystem note

Projects should live **inside the WSL2 filesystem** (`~/projects/my-project`), not on the Windows filesystem (`/mnt/c/Users/...`). The Windows filesystem mount has significantly slower I/O from WSL2, which compounds over many file operations in a long agent session.

### WSL2 PATH note

All four required binaries (`claude`, `go`, `git`, `doug`) must be WSL2-native installations, not Windows-native ones. Using Windows-native `go.exe` from WSL2 PATH can cause subtle cross-environment issues. Install everything fresh inside WSL2.

---

## Native Windows Path (MSYS2/Git Bash)

### What works out of the box

The `doug` binary runs natively on Windows. `go install` works. `doug init` works. `doug run` works — the orchestrator loop runs, agents are invoked, tasks complete, git commits are made. The EINVAL issue reduces agent effectiveness but does not break the workflow because the orchestrator compensates.

### The gap: agent self-verification

On every task, agents attempt to run `go build ./...` and `go test ./...` to confirm their implementation is correct before writing `outcome: SUCCESS`. On Windows/MSYS2, these Bash tool calls fail silently (EINVAL). Agents still report SUCCESS and move on. The orchestrator's `HandleSuccess` catches any actual build/test failures independently.

In practice (observed across EPIC-6): agents adapted well, doing their verification through code reading rather than command execution. The orchestrator's post-success gate held correctly — it detected the one real failure (EPIC-6-003 attempt-1) and triggered a retry.

### Workaround: disable Bash tool via Claude Code settings

The cleanest native Windows experience disables Bash entirely so agents get a clear, actionable error rather than a confusing EINVAL. This also prevents agents from wasting time retrying failed Bash calls.

**Option A — Deny all Bash (simplest):**
```json
{
  "permissions": {
    "deny": ["Bash"]
  }
}
```

**Option B — PreToolUse hook (more informative, recommended):**

The hook intercepts Bash calls before they hit the sandbox and returns a message Claude can act on:

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Bash",
      "hooks": [{
        "type": "command",
        "command": "echo 'Bash tool unavailable: Windows/MSYS2 environment. Use Read, Write, Edit, Glob, or Grep. Build and test verification is handled automatically by the orchestrator after each task.' >&2; exit 2"
      }]
    }]
  }
}
```

Exit code 2 blocks the tool call and feeds the message back to Claude as context, allowing it to pivot immediately rather than failing silently.

---

## `doug init` Windows Mode — Implementation Options

Currently `cmd/init.go` writes `doug.yaml`, `tasks.yaml`, `PRD.md`, `CLAUDE.md`, `AGENTS.md`, template files, and skill files. It does **not** write `.claude/settings.json`.

The existing `.claude/settings.json` in this project (not yet in init templates) shows the fully-developed allow/deny list. Three options for making this available to new users:

### Option 1: Add `settings.json` to init templates (always ships)

Add `internal/templates/init/settings.json` with sensible defaults. `copyInitTemplates` would need a new routing rule to copy it to `.claude/settings.json`. This ships to all platforms — a safe, locked-down default that works on Linux/macOS and degrades gracefully on Windows.

```json
// internal/templates/init/settings.json — proposed
{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "permissions": {
    "allow": [
      "Read", "Write", "Edit", "Glob", "Grep",
      "Bash(go build *)", "Bash(go test *)", "Bash(go vet *)",
      "Bash(go mod tidy)", "Bash(go mod download)",
      "Bash(npm install *)", "Bash(npm run build)", "Bash(npm run test)",
      "Bash(git log *)", "Bash(git diff *)", "Bash(git status)"
    ],
    "deny": [
      "Bash(git add *)", "Bash(git commit*)", "Bash(git push *)",
      "Bash(git checkout *)", "Bash(git branch *)",
      "Bash(rm -rf *)", "Bash(sudo *)",
      "Write(*.yaml)", "Write(.env*)"
    ]
  }
}
```

**Pros**: Every new project gets a security-conscious default. No platform detection needed.
**Cons**: `copyInitTemplates` needs a new routing case for `settings.json` → `.claude/settings.json`.

### Option 2: `doug init --windows` flag

Add a `--windows` flag to `cmd/init.go` that writes a settings.json with the PreToolUse hook included. All other init behavior unchanged.

**Pros**: Explicit opt-in, no surprises on Linux/macOS, clear user intention.
**Cons**: Users must know to pass `--windows`; auto-detection would be friendlier.

### Option 3: Auto-detect platform at init time

```go
import "runtime"

if runtime.GOOS == "windows" {
    writeWindowsSettings(dir)
}
```

**Pros**: Zero user friction — just works on Windows.
**Cons**: MSYS2 users running `doug init` on Windows get the restricted settings even if they later switch to WSL2. Could be confusing.

### Option 4: `--windows` flag with CLAUDE.md note (recommended for v0.4.0)

Ship Option 1 (settings.json in templates with the allow/deny list) for all platforms, and add a note to the template `CLAUDE.md` for Windows users:

```markdown
## Environment Note (Windows/MSYS2 users)

If running on Windows outside WSL2, the Bash tool is unavailable due to a Claude Code
sandbox limitation. Use Read, Write, Edit, Glob, and Grep for all file operations.
Build and test verification is handled automatically by the orchestrator after each task.

For full Bash support, run doug from WSL2.
```

**Pros**: Works cross-platform, educates users, no platform detection complexity. The allow/deny settings.json ships universally and is correct on all platforms.

---

## Cross-Platform Code Audit

Reviewed against common Windows portability issues:

| Concern | Findings | Status |
|---------|----------|--------|
| Path separators | All paths use `filepath.Join`, `filepath.Dir`, `filepath.Abs` throughout | ✅ Safe |
| Hardcoded `/tmp/` | None found — uses `os.TempDir()` and `os.MkdirTemp()` | ✅ Safe |
| Shell string eval | None — all subprocess calls use explicit `[]string` args slice | ✅ Safe |
| `sed`/`awk`/`yq` | None — PRD explicitly forbids them; confirmed absent from all Go files | ✅ Safe |
| File permissions | `0o644` for files, `0o755` for directories — standard, works on Windows | ✅ Safe |
| Line endings | `parse.go` normalises CRLF→LF explicitly: `strings.ReplaceAll(content, "\r\n", "\n")` | ✅ Safe |
| `git` on PATH | `CheckDependencies` verifies at startup; requires Git for Windows or WSL2 git | ✅ Safe |
| `go` on PATH | `CheckDependencies` verifies at startup | ✅ Safe |
| Atomic writes | `os.Rename` used in state saves — works on Windows (same volume) | ✅ Safe |

**One note on `git add -A` in `internal/git/git.go:Commit`**: On Windows with Git for Windows in MSYS2, `git add -A` may behave differently with mixed path separators if the project root contains backslashes. Since `doug` always passes `cmd.Dir = projectRoot` (using Go's `filepath`-normalized path), this should be safe in practice, but worth mentioning in the README for users running on native Windows.

---

## Dependencies

### External (from `go.mod`)
- `github.com/spf13/cobra v1.10.2` — CLI framework; cross-platform ✅
- `gopkg.in/yaml.v3 v3.0.1` — YAML parsing; cross-platform ✅

### Runtime dependencies (verified by `CheckDependencies`)
- `claude` (or configurable `agent_command`) — Claude Code CLI
- `git` — Git for Windows or WSL2 git
- `go` (for Go projects) or `npm` (for Node projects)

### No POSIX-only dependencies in the Go binary
The Go port was explicitly designed to eliminate `sed`, `awk`, `yq`, and `eval`. The PRD's definition of done includes "No `sed`, `awk`, `yq`, or `eval` in the codebase" — confirmed met.

---

## README Recommendations

Based on this research, the public README should include:

### Installation section
```markdown
## Installation

**Requires**: Go 1.26+, Git, Claude Code CLI (`npm install -g @anthropic-ai/claude-code`)

go install github.com/robertgumeny/doug@latest

Or download a pre-built binary from the releases page.
```

### Platform support section
```markdown
## Platform Support

| Platform | Status | Notes |
|----------|--------|-------|
| Linux | ✅ Full support | Recommended |
| macOS | ✅ Full support | Recommended |
| Windows (WSL2) | ✅ Full support | Recommended for Windows users |
| Windows (native/MSYS2) | ⚠️ Partial | Orchestrator works; agent Bash tool unavailable |

### Windows users

Run `doug` from **WSL2** for the best experience. The Claude Code CLI's Bash tool
is not available in native Windows environments (MSYS2/Git Bash), which means agents
cannot run shell commands to self-verify their work. The orchestrator compensates by
running build and test verification independently after each task, so the workflow
remains functional — but WSL2 gives agents full capabilities.

**WSL2 quickstart**: [link to WSL2 setup section]

**Native Windows workaround**: Run `doug init` and add the following to
`.claude/settings.json` to disable the Bash tool and give agents a helpful error
message instead of a confusing failure:

[PreToolUse hook example]
```

### Agent trust boundary section
```markdown
## Agent Trust Boundary

doug invokes the configured agent (`claude` by default) as a subprocess with full
access to the project directory. The agent boundary is enforced by instruction
(CLAUDE.md and skill files), not by sandboxing. Agents are instructed not to run
Git commands, modify YAML state files, or touch the changelog — the orchestrator
owns those operations.

`doug init` generates a `.claude/settings.json` that restricts the most dangerous
operations (force push, rm -rf, sudo) but does not create an isolated sandbox.
Treat `agent_command` as a trusted process with filesystem access to your project.
```

---

## Patterns Observed

- **`exec.Command` throughout**: Every subprocess call — git, go, npm, claude — uses explicit args slices with no shell interpolation. This is both a security win and the reason the orchestrator itself is fully cross-platform.
- **`filepath` package used consistently**: No hardcoded path separators anywhere in the reviewed code.
- **`CheckDependencies` as user-facing error surface**: The startup check catches missing binaries before the loop starts, providing clear guidance. This is the right place to also surface platform-specific warnings (e.g., "Running on Windows — Bash tool may be unavailable to agents").
- **`copyInitTemplates` routing pattern**: The switch in `cmd/init.go:copyInitTemplates` (CLAUDE.md/AGENTS.md → root, `*_TEMPLATE.md` → logs/, `skills/` → `.claude/skills/`) can be extended with a `settings.json` → `.claude/settings.json` case with minimal code change.

---

## Anti-Patterns & Tech Debt

- **No `settings.json` in init templates**: New projects created with `doug init` get skill files and CLAUDE.md but no `.claude/settings.json`. Users must manually create this to get a safe permission profile. The existing `.claude/settings.json` in this repo represents the correct defaults and should be templated.
- **No platform detection or warning at runtime**: `CheckDependencies` verifies binaries exist but does not warn Windows/MSYS2 users that Bash tool calls will fail inside agent sessions. A one-time warning would improve the out-of-box experience significantly.
- **Template CLAUDE.md has no Windows note**: The `internal/templates/init/CLAUDE.md` that ships to new projects says "Run build, test, and lint commands" with no caveat for Windows users. This will confuse agents running on Windows/MSYS2 who try and fail.

---

## PRD Alignment

The PRD's primary motivation is cross-platform distribution: *"Not cross-platform. Windows users cannot run it."* The Go port delivers on this for the orchestrator binary itself. The gap is the agent's Bash tool — an external dependency (Claude Code CLI) that the doug project does not control.

The PRD states: *"No agent sandboxing (document the trust boundary explicitly; enforce by instruction only)"* and *"README documents the agent trust boundary explicitly"*. The platform guidance and Windows-specific settings.json both belong in this documentation layer.

The `doug init` improvements suggested here (settings.json template, CLAUDE.md Windows note) are small, targeted additions that directly serve the public release goal without adding architectural complexity.

---

## Raw Notes

- The `go 1.26` requirement in `go.mod` is notable — Go 1.26 had not been released as of the knowledge cutoff. Verify this is correct or reduce to `go 1.21` (LTS) for broader compatibility before publishing.
- Pre-built binary distribution (GitHub Releases + GoReleaser) would dramatically lower the barrier for non-Go users who don't want to `go install`. A single `curl | sh` install script covering Linux/macOS/Windows is table stakes for a developer tool aiming at wide adoption.
- The `agent_command: claude` default in `doug.yaml` ties the default experience to the Claude Code CLI. The PRD calls out "agent-agnostic" as a guiding principle. The README should clearly document that any command-line program that reads `ACTIVE_TASK.md` and writes a session file is a valid agent — claude is just the default.
- Homebrew tap is the lowest-friction macOS install path for users who don't have Go installed. Worth adding to the roadmap alongside the initial release.
