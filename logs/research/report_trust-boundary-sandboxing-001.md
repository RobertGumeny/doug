# Research Report: Trust Boundary & Sandboxing

**Generated**: 2026-02-25
**Scope Type**: Feature/Module
**Related Epic**: Post-EPIC-6 (production readiness)
**Related Tasks**: PRD Open Item — "Agent trust boundary: document and explore sandboxing"

---

## Overview

`doug` invokes a coding agent (Claude Code, Aider, or any CLI) via `exec.Command` with the project root as the working directory and no filesystem restrictions. The agent has full read/write access to everything the OS user can access. This report catalogs every practical sandboxing option for the `RunAgent` attachment point (`internal/agent/invoke.go:17`), assesses what Claude Code provides natively, surveys how comparable tools solve this, and delivers a concrete, prioritized implementation roadmap.

---

## File Manifest

| File | Relevance |
|------|-----------|
| `internal/agent/invoke.go` | **Attachment point** — `RunAgent` is where all sandboxing hooks |
| `internal/config/config.go` | Config struct — any `sandbox_mode` field would be added here |
| `cmd/run.go` | CLI flags — `--sandbox` override would be wired here |
| `.claude/settings.json` | Claude Code deny rules — the current enforcement layer |
| `internal/templates/init/CLAUDE.md` | Instructional enforcement — copied to user projects by `doug init` |
| `internal/templates/init/AGENTS.md` | Agent contract documentation |

---

## The Trust Boundary — Current State

### What the agent can do today

The current `RunAgent` invocation:

```go
// internal/agent/invoke.go:24-27
cmd := exec.Command(parts[0], parts[1:]...)
cmd.Dir = projectRoot
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
```

- Full read/write access to the **entire filesystem** accessible to the OS user
- Can write to `~/.ssh`, `~/.config`, `~/.aws`, `/etc` (if user is root), etc.
- Can spawn child processes, open network connections, install packages globally
- No CPU, memory, or time limit beyond `max_iterations`

### Current enforcement layers

| Layer | Mechanism | Enforced by | Bypassed by |
|-------|-----------|-------------|-------------|
| Instructional | `CLAUDE.md` tells agent what not to do | Agent following instructions | Agent ignoring/misunderstanding |
| Skill files | Per-task reinforcement of constraints | Agent following instructions | Same |
| `settings.json` deny rules | Claude Code blocks matching tool calls | Claude Code tool dispatch (userspace) | Alternative tool use (e.g., reading `.env` via `Bash(cat)` instead of `Read(.env)`) |
| Orchestrator rollback | `git reset --hard HEAD` on FAILURE/BUG | Orchestrator (reliable) | Does not prevent writes during the session |
| Protected state files | State YAML backed up before rollback | Orchestrator (reliable) | Not related to agent overreach |

### `settings.json` permission enforcement — important clarification

The deny rules in `.claude/settings.json` are **userspace intercepts** at the Claude Code tool-dispatch layer, not kernel-level controls. When Claude attempts a denied tool call, Claude Code blocks it before execution and returns an error to the model. There is no seccomp filter, chroot, or namespace backing these rules.

Implications:
- Deny rules can be bypassed by equivalent tool calls (e.g., `"Bash(cat .env)"` instead of `"Read(.env)"` when `cat` is allowed)
- The rule `"Bash(rm -rf *)"` is in the deny list but `"Bash(rm -rf ./dist)"` would match `"Bash(rm *)"` in the allow list — pattern matching is glob-style and order-dependent (deny takes precedence over allow)
- There is also a confirmed typo: `"Bash(git commit:*)"` uses a colon and will never match `git commit` commands
- `--dangerously-skip-permissions` **does not bypass deny rules** — it only suppresses the interactive approval prompt for tool calls that aren't pre-allowed. Deny rules still apply.

### The enforcement gap for end users

The `settings.json` file in this repository controls Claude Code's behavior **when developing doug itself**. When a user installs `doug` and runs it on their own project, the `settings.json` that applies is the one in their project directory — which `doug init` does not create. Users get the `CLAUDE.md` and skill file instructional constraints, but no mechanical deny-rule layer unless they configure it themselves.

This is the primary gap: the developer experience of doug (with carefully tuned deny rules) is not reproduced for end users by default.

---

## Sandboxing Options — Platform by Platform

The single insertion point for all sandboxing is `RunAgent` in `internal/agent/invoke.go`. The pattern in every case is the same: wrap the agent's command + args with a sandbox harness.

### Linux — Best Platform, Multiple Strong Options

#### Option L1: Landlock (recommended for Linux)

**What it is:** Linux Security Module (kernel 5.13+) that restricts filesystem access paths without root. Unprivileged. Self-applied by the process. All descendants inherit the restriction. Irreversible once applied.

**Key properties:**
- No external tools required — pure kernel syscalls
- `github.com/landlock-lsm/go-landlock` provides a high-level Go API
- `BestEffort()` mode silently does nothing on older kernels — zero breakage risk
- V1 (kernel 5.13): filesystem path restrictions
- V4 (kernel 6.7): adds TCP bind/connect restrictions

**The child-process problem:** Landlock restricts the calling process. `RunAgent` cannot directly apply Landlock to the child before exec. The solution is a thin wrapper binary (`doug-sandbox`) that the orchestrator launches instead of the agent directly:

```
doug run → launches doug-sandbox → doug-sandbox applies Landlock to itself → exec's claude
```

```go
// RunAgent with Landlock enabled:
sandboxArgs := []string{
    "--allow-write", projectRoot,
    "--allow-read", "/",
    "--allow-exec", "/usr/local/bin",
    "--",
}
sandboxArgs = append(sandboxArgs, parts...)
cmd := exec.Command("doug-sandbox", sandboxArgs...)
```

The `doug-sandbox` binary (a separate ~100-line Go program, distributed alongside `doug`):

```go
// cmd/sandbox/main.go (Linux only, build tag: //go:build linux)
package main

import (
    "os"
    "os/exec"
    "github.com/landlock-lsm/go-landlock/landlock"
)

func main() {
    projectRoot, agentArgs := parseArgs(os.Args[1:])

    // Apply Landlock to this process (and all future descendants)
    err := landlock.V1.BestEffort().RestrictPaths(
        landlock.RODirs("/"),           // read-only access to entire FS
        landlock.RWDirs(projectRoot),   // read-write to project only
    )
    if err != nil {
        fmt.Fprintf(os.Stderr, "landlock: %v\n", err)
        // Non-fatal: continue without restriction if not supported
    }

    // exec replaces this process — restrictions are inherited
    cmd := exec.Command(agentArgs[0], agentArgs[1:]...)
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()
}
```

**Coverage:** Read access to `/`, write access restricted to `projectRoot` only. The agent cannot write to `~/.ssh`, `~/.config`, `.env` files outside the project, etc.

**What it does NOT cover:** Network access (requires V4 kernel 6.7+), syscall filtering.

#### Option L2: Network Namespace via `syscall.SysProcAttr` (zero deps)

Network isolation with no external tools, available on all kernels since 2.6.24:

```go
// internal/agent/invoke.go — add after cmd construction:
import "syscall"

cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: syscall.CLONE_NEWNET,  // isolated network namespace
    // CLONE_NEWUSER enables unprivileged use:
    // Cloneflags: syscall.CLONE_NEWNET | syscall.CLONE_NEWUSER,
    // UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
    // GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
}
```

**Effect:** Agent process runs in a separate network namespace with no interfaces configured — no outbound connections possible. The agent cannot call LLM APIs, exfiltrate data over HTTP, etc.

**Caveats:**
- `CLONE_NEWNET` alone requires `CAP_SYS_ADMIN` (root)
- Combined with `CLONE_NEWUSER`, it can be done unprivileged on kernels that allow it (`kernel.unprivileged_userns_clone=1`) — Ubuntu 23.10+ restricts this by default with AppArmor
- Does not restrict filesystem access

**This is the easiest low-level improvement** to add to `RunAgent` with zero new dependencies on Linux.

#### Option L3: bubblewrap (bwrap) — strongest, requires install

When `bwrap` is installed, it provides full namespace + filesystem restriction:

```go
// In RunAgent, if sandbox_mode = "bwrap":
bwrapArgs := []string{
    "--unshare-all",
    "--ro-bind", "/usr", "/usr",
    "--ro-bind", "/lib", "/lib",
    "--ro-bind", "/lib64", "/lib64",
    "--ro-bind", "/bin", "/bin",
    "--ro-bind", "/etc", "/etc",
    "--ro-bind", "/home/" + os.Getenv("USER") + "/.config/claude", "/root/.config/claude",
    "--bind", projectRoot, projectRoot,
    "--dev", "/dev",
    "--proc", "/proc",
    "--tmpfs", "/tmp",
    "--setenv", "HOME", "/tmp",
    "--die-with-parent",
    "--new-session",
    "--",
}
bwrapArgs = append(bwrapArgs, parts...)
cmd := exec.Command("bwrap", bwrapArgs...)
```

**Coverage:** Complete filesystem isolation (only explicitly bound paths visible), network isolation (`--unshare-all` includes `CLONE_NEWNET`), PID isolation.

**Requirement:** `bwrap` binary must be installed (`apt install bubblewrap` / `brew install bubblewrap`).

---

### macOS — Limited, Best-Effort Only

#### Option M1: `sandbox-exec` with Seatbelt profile

The only native option for macOS CLI tools. **Officially deprecated since macOS 12** but still functional through macOS 15:

```go
// internal/agent/invoke.go — darwin build tag
sbProfile := fmt.Sprintf(`(version 1)
(allow default)
(deny network*)
(deny file-write* (subpath "/"))
(allow file-write*
    (subpath "%s")
    (subpath "/tmp")
    (subpath "/private/tmp")
    (subpath "/var/folders"))
`, projectRoot)

cmd := exec.Command("sandbox-exec", "-p", sbProfile, "--")
cmd.Args = append(cmd.Args, parts...)
```

**Coverage:** Filesystem writes restricted to project dir + temp dirs. Network blocked.

**Important caveats:**
- Apple deprecated Seatbelt. Could be removed in a future macOS version.
- Profile language is undocumented. Must be tuned empirically.
- Claude Code uses Mach IPC and XPC extensively — `(allow mach*)` may be needed.
- Ship a tuned `.sb` profile file rather than an inline string for maintainability.

#### Option M2: Docker Desktop (macOS)

Same Docker command as the cross-platform container approach below. Most reliable macOS option but requires Docker Desktop installation.

---

### Windows — Minimal Kernel Options, Container is the Story

#### Option W1: Job Object — orphan prevention only

Job Objects can prevent orphan agent processes but cannot restrict filesystem or network:

```go
// Useful but not a security sandbox:
cmd.SysProcAttr = &syscall.SysProcAttr{
    CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
}
// Assign to Job Object post-Start for kill-on-close semantics
```

**Coverage:** Orphan-process prevention, CPU/memory limits. **Does not restrict filesystem or network.**

#### Option W2: Docker Desktop / WSL2

The realistic Windows sandboxing story. Check with `exec.LookPath("docker")` and fall through to Docker mode.

---

### Cross-Platform — Container Mode (Strongest Available Everywhere)

When Docker or Podman is available, this provides the strongest cross-platform isolation:

```go
// In RunAgent, if sandbox_mode = "container":
containerRuntime := detectContainerRuntime()  // docker | podman | nerdctl

containerArgs := []string{
    "run", "--rm",
    "--read-only",
    "--tmpfs", "/tmp:rw,size=512m",
    "--network", "none",
    "--cap-drop", "ALL",
    "--security-opt", "no-new-privileges",
    "--volume", projectRoot + ":/workspace:rw",
    "--volume", claudeConfigDir() + ":/root/.config/claude:ro",
    "--workdir", "/workspace",
    cfg.SandboxImage,  // e.g. "ghcr.io/robertgumeny/doug-agent:latest"
    "--",
}
containerArgs = append(containerArgs, parts...)
cmd := exec.Command(containerRuntime, containerArgs...)
```

**Coverage:**
- Filesystem: only `projectRoot` is mounted read-write; container FS is read-only
- Network: completely blocked (`--network none`)
- Capabilities: all Linux capabilities dropped
- Privilege escalation: prevented (`--security-opt no-new-privileges`)

**Requirement:** Docker/Podman installed and daemon running.

**What the sandbox image needs:**
- The agent binary (Claude Code, Aider, etc.) installed
- Go toolchain (if `build_system: go`)
- Node.js/npm (if `build_system: npm`)
- Git (for the orchestrator to commit after the session)

The `doug-agent` image can be a Dockerfile that users build from:
```dockerfile
FROM ubuntu:24.04
RUN apt-get update && apt-get install -y git golang nodejs npm
# Install Claude Code
RUN npm install -g @anthropic-ai/claude-code
WORKDIR /workspace
```

---

## What Claude Code Provides Natively

### `settings.json` permissions — userspace only

As confirmed by codebase analysis, the `permissions` block in `.claude/settings.json` is enforced at the Claude Code **tool-dispatch layer in userspace**. It is not backed by OS-level restrictions.

**Enforce semantics:**
- `"deny": ["Bash(git commit -m *)"]` → Claude Code blocks the tool call before calling the OS, feeds an error back to the model
- Does not prevent: alternative spellings, piped bash commands, using lower-level tools
- Deny takes precedence over allow in all cases

**`--dangerously-skip-permissions`** — Suppresses interactive approval prompts. **Does not bypass deny rules.** Essential for non-interactive orchestrator use.

### Claude Code `hooks` — more expressive than deny rules

The `hooks` system (via `.claude/settings.json`) allows running a shell command before or after any tool call. Exit code 2 from a `PreToolUse` hook blocks the call and returns stderr to the model as context:

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Write",
      "hooks": [{
        "type": "command",
        "command": "python3 -c \"import sys,json; call=json.load(sys.stdin); path=call.get('file_path',''); sys.exit(2 if '.env' in path or 'project-state.yaml' in path else 0)\""
      }]
    }]
  }
}
```

This is **more precise than glob patterns** because the hook receives the full tool call as JSON on stdin and can apply arbitrary logic. It can check the actual file path rather than relying on glob matching.

**Hooks are a significant improvement over deny rules** for the `doug` use case:
- Can validate that writes stay within `projectRoot`
- Can block any Write/Edit to `*.yaml` files more reliably than glob patterns
- Can enforce the git-operations boundary more precisely
- Hooks fire even for tools not explicitly listed in `allow`

`doug init` should generate a `settings.json` that uses hooks for the most critical restrictions rather than (or in addition to) glob-based deny rules.

### No native sandboxing in Claude Code

Claude Code does not ship with Docker mode, chroot, namespace isolation, or any OS-level sandboxing as of early 2026. The `settings.json` system is the entirety of its built-in access control.

---

## Prior Art — How Comparable Tools Handle This

| Tool | Approach | Isolation Level |
|------|----------|-----------------|
| **Aider** | None — full filesystem access by convention | None |
| **SWE-agent** | Docker container per session, project mounted as volume | Container |
| **OpenHands** | Docker sandbox container + REST API bridge | Container |
| **AutoCodeRover** | Docker for test execution, agent process less restricted | Partial |
| **Devin** | Dedicated Ubuntu VM per session (cloud-hosted) | VM |
| **E2B** | Firecracker microVM per session (cloud API) | microVM |
| **Modal** | Cloud container per function call | Container |
| **doug** (today) | Instruction + `settings.json` deny rules | Userspace only |

**Community consensus as of early 2026:**

1. **Instruction-first + deny rules** is standard for local developer tools (Tier 1 — all tools)
2. **Docker containers** are the production standard for unattended/CI agent runners (Tier 2 — SWE-agent, OpenHands)
3. **VMs/microVMs** are for multi-tenant or public code execution services (Tier 3 — Devin, E2B)

`doug` is explicitly designed for the developer-on-their-own-machine use case, where Tier 1 is appropriate. **Tier 2 (container mode) should be an opt-in flag for CI/CD and shared-team use cases.**

---

## Patterns Observed

- **Separation of enforcement tiers**: Every production agent runner separates "instructional" (prompt-level) from "mechanical" (syscall-level) enforcement. The former is the first line; the latter is the backstop.
- **Docker as the cross-platform answer**: Both SWE-agent and OpenHands settled on Docker as the cross-platform sandbox. It is the only mechanism that provides consistent strong isolation on Linux, macOS, and Windows.
- **Landlock as the future of no-daemon Linux sandboxing**: As kernels have crossed 5.13+ in mainstream distros (Ubuntu 22.04 LTS = kernel 5.15), Landlock has become the recommended approach for unprivileged filesystem restriction on Linux without external tools.
- **Hooks > deny rules for precision**: The Claude Code hooks system provides more expressive per-tool-call filtering than glob-pattern deny rules. The two should be used together.

---

## Anti-Patterns & Risks

- **Relying solely on `settings.json` deny rules as a security boundary**: They are useful but not robust. A sophisticated agent (or one that receives confusing instructions) can bypass them via equivalent tool combinations.
- **`--dangerously-skip-permissions` as a security control**: It is not one. Users should understand this flag suppresses UX friction, not security enforcement.
- **macOS `sandbox-exec` as a long-term strategy**: It is deprecated and Apple provides no replacement. Any `sandbox-exec` integration should be clearly labeled as best-effort.
- **Attempting to sandbox on Windows at the OS level without containers**: Job Objects provide no filesystem restriction. AppContainer is not applicable to arbitrary CLI tools. Windows sandboxing without Docker is not meaningful.
- **seccomp allowlists for arbitrary agent binaries**: Seccomp allowlists must enumerate every syscall the binary may call. Claude Code's internal syscall set is not documented and will change between versions. This approach would break on every Claude Code update.

---

## Recommended Implementation Roadmap

### Immediate (no code changes — docs + config)

**Step 1: Generate `settings.json` in `doug init`**

`doug init` should create `.claude/settings.json` in the user's project with a curated deny list. Currently, this file is not created by init — users get no mechanical enforcement layer.

The generated file should use hooks for path-based restrictions and deny rules for operation-based restrictions:

```json
{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "permissions": {
    "allow": [
      "Read", "Write", "Edit", "Glob", "Grep",
      "Bash(git log *)", "Bash(git diff *)", "Bash(git status)",
      "Bash(go build *)", "Bash(go test *)", "Bash(go vet *)",
      "Bash(go mod tidy)", "Bash(go mod download)",
      "Bash(npm install)", "Bash(npm run build)", "Bash(npm run test)",
      "Bash(npm ci)", "Bash(make *)",
      "Bash(cat *)", "Bash(ls *)", "Bash(find *)",
      "Bash(mkdir *)", "Bash(cp *)", "Bash(mv *)", "Bash(touch *)",
      "Bash(head *)", "Bash(tail *)", "Bash(echo *)", "Bash(grep *)",
      "Bash(wc *)", "Bash(sed *)", "Bash(awk *)"
    ],
    "deny": [
      "Bash(git add *)",
      "Bash(git commit *)",
      "Bash(git checkout *)",
      "Bash(git branch *)",
      "Bash(git push *)",
      "Bash(git pull *)",
      "Bash(git stash *)",
      "Bash(git rebase *)",
      "Bash(git reset *)",
      "Bash(gh pr create *)",
      "Bash(gh pr merge *)",
      "Bash(rm -rf *)",
      "Bash(sudo *)",
      "Bash(npm run dev)",
      "Bash(npm start)",
      "Read(.env)",
      "Read(.env.*)",
      "Write(.env)",
      "Write(.env.*)",
      "Write(project-state.yaml)",
      "Write(tasks.yaml)"
    ]
  }
}
```

Fix the typo (`"Bash(git commit:*)"` → `"Bash(git commit *)"`) and add `"Bash(git reset *)"` which was missing.

**Step 2: Add `README.md` trust boundary section**

Document prominently:
- What the agent can and cannot do
- How `settings.json` enforcement works (and its limits)
- The recommended `settings.json` pattern
- Instructions for enabling container mode (once implemented)
- The explicit statement: "doug does not sandbox the agent at the OS level by default"

---

### Short-Term (small code changes — Linux network isolation)

**Step 3: Add `CLONE_NEWNET` network isolation on Linux**

This is the highest-value, lowest-effort OS-level improvement. Add to `internal/agent/invoke.go` with a build tag:

```go
// internal/agent/invoke_linux.go
//go:build linux

package agent

import "syscall"

func setSandboxAttrs(cmd *exec.Cmd, cfg SandboxConfig) {
    if !cfg.NetworkIsolation {
        return
    }
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Cloneflags: syscall.CLONE_NEWNET | syscall.CLONE_NEWUSER,
        UidMappings: []syscall.SysProcIDMap{
            {ContainerID: 0, HostID: os.Getuid(), Size: 1},
        },
        GidMappings: []syscall.SysProcIDMap{
            {ContainerID: 0, HostID: os.Getgid(), Size: 1},
        },
    }
}
```

Add `network_isolation: false` to `OrchestratorConfig` and `doug.yaml`. Default off; users opt in. Document the Ubuntu AppArmor restriction caveat.

---

### Medium-Term (new binary — filesystem isolation via Landlock on Linux)

**Step 4: Implement `doug-sandbox` wrapper binary**

Create `cmd/sandbox/main.go` (Linux only, distributed alongside `doug` in releases):

```go
//go:build linux

package main

// Thin wrapper: applies Landlock to itself, then exec's the agent binary.
// Usage: doug-sandbox --project-root /path/to/project -- claude args...
```

Update `RunAgent` to detect `doug-sandbox` on PATH and use it when `sandbox_mode: landlock` is configured.

Update `.goreleaser.yaml` to build and package both `doug` and `doug-sandbox`.

Add `sandbox_mode: ""  # landlock | bwrap | container | none` to `doug.yaml`.

**Graceful degradation:**
- `landlock` mode: falls back to unrestricted if kernel < 5.13 (via `BestEffort()`)
- `bwrap` mode: fails fast with a clear error if `bwrap` not installed
- `container` mode: fails fast with a clear error if Docker/Podman not installed
- `none` (default): current behavior, unchanged

---

### Long-Term (optional, for CI/CD teams)

**Step 5: Container mode (`--sandbox=container`)**

Add `sandbox_mode: container` support to `RunAgent`. Detect `docker`/`podman`/`nerdctl`. Build and publish a `ghcr.io/robertgumeny/doug-agent` Docker image.

Configuration in `doug.yaml`:
```yaml
sandbox_mode: container
sandbox_image: ghcr.io/robertgumeny/doug-agent:latest
```

This is the recommended path for **shared team environments, CI/CD, or any case where the project directory is not on a developer's personal machine.**

---

## Summary Table — Options vs. doug's Constraints

| Option | Platform | Filesystem | Network | Root needed | External tool | Effort |
|--------|----------|-----------|---------|-------------|---------------|--------|
| `settings.json` (today) | All | Pattern deny (userspace) | None | No | No | Done |
| `CLAUDE.md` hooks | All | Path-based (userspace) | None | No | No | Low |
| `CLONE_NEWNET` namespaces | Linux | None | Yes | Maybe* | No | Low |
| Landlock V1 via `go-landlock` | Linux 5.13+ | Yes (path-based) | No | No | No | Medium |
| Landlock V4 | Linux 6.7+ | Yes | Yes (TCP) | No | No | Medium |
| bubblewrap (`bwrap`) | Linux | Yes (bind mounts) | Yes | No | Yes (bwrap) | Medium |
| `sandbox-exec` | macOS (deprecated) | Yes (best-effort) | Yes | No | No (built-in) | Medium |
| Job Objects | Windows | None | None | No | No | Low |
| Docker/Podman container | All | Yes (volume mounts) | Yes | Yes (daemon) | Yes (daemon) | High |
| E2B / Modal | All (cloud) | Yes (VM) | Yes | No | Yes (account) | High |

*`CLONE_NEWUSER` + `CLONE_NEWNET` = unprivileged; `CLONE_NEWNET` alone = needs CAP_SYS_ADMIN

---

## PRD Alignment

The PRD explicitly deferred sandboxing: "No agent sandboxing (document the trust boundary explicitly; enforce by instruction only)." That was the correct v0.4.0 decision.

For v0.5.0+, the following is the minimum viable improvement path that stays true to the "lightweight CLI tool" design principle:

1. **`settings.json` generated by `doug init`** — closes the end-user enforcement gap with zero runtime overhead
2. **`CLONE_NEWNET` Linux network isolation** — two stdlib imports, eight lines of code, no external deps
3. **Landlock filesystem isolation via `doug-sandbox`** — meaningful filesystem boundary on all modern Linux distros, graceful degradation on older kernels
4. **Container mode (opt-in)** — for teams that need it; Docker is the only cross-platform answer for strong isolation

The explicit documentation in README that the boundary is instructional-first is required before any of the above. Sandboxing that users don't know about provides false security; sandboxing they understand and configured intentionally is effective defense.

---

## Raw Notes

### On `--dangerously-skip-permissions` documentation

This flag is commonly misunderstood. It should be documented as "required for non-interactive use" with a clear explanation that deny rules still apply. Many users enabling this flag think they are disabling all safety checks — they are only disabling the interactive approval prompt.

### On `settings.json` being copied by `doug init`

The highest-impact single change is adding `settings.json` creation to `doug init`. This gives every user who uses Claude Code the same mechanical deny-rule layer that the doug development environment has. It does not require any code changes to `internal/` — only a new file in `internal/templates/init/` and a new entry in the `copyInitTemplates` switch in `cmd/init.go`.

### On the skills-config.yaml gap

`doug init` also does not copy `.claude/skills-config.yaml`. Users who want to add custom task types must create this file manually. The init command should copy a default `skills-config.yaml` from the init template directory, similar to how CLAUDE.md and AGENTS.md are handled.

### On the macOS deprecation risk

`sandbox-exec` deprecation is real and should be tracked. The Apple Developer Forums have noted it works through macOS 15 but Apple has consistently refused to document it as a supported API. For a v1.0 release, the macOS story should be: "For strong isolation on macOS, use container mode. `sandbox-exec` support is best-effort and may be removed in a future macOS release."

### On Landlock and the coding agent's requirements

A Landlock ruleset for a coding agent needs to allow:
- Read access to `/` (the agent reads system libraries, Go stdlib, etc.)
- Read-write access to `projectRoot` (the agent's primary workspace)
- Read-write access to `/tmp` (many tools write temp files here)
- Read-write access to `~/.config/claude` (Claude Code's config/auth directory)
- Read-write access to `~/.local/share/claude` (Claude Code's data directory)
- Execute access to system binaries in `/usr/bin`, `/usr/local/bin`, etc.
- Read access to `/etc/ssl/certs` (TLS certificates for HTTPS)

Without these, the agent will fail in confusing ways. The `doug-sandbox` binary needs a well-tested default ruleset, not just `RODirs("/")` + `RWDirs(projectRoot)`.
