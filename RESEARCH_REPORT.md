# Research Report: In-Memory Knowledge Graph & Context Injection — Feasibility Analysis

**Generated**: 2026-02-24
**Scope Type**: Feature/Module
**Related Epic**: Post-EPIC-6 (proposed future feature)
**Related Tasks**: None (not yet in tasks.yaml)

---

## Overview

This report analyzes the feasibility of a proposed "In-Memory Knowledge Graph & Context Injection" feature for the `doug` orchestrator. The feature has three phases: building a boot-time adjacency list from `docs/kb/` frontmatter, injecting a "Context Map" section into `ACTIVE_TASK.md`, and exposing a `GetContextNeighborhood(path)` runtime tool for agent graph traversal. The analysis is grounded in the actual codebase state as of EPIC-6.

---

## File Manifest

| File | Purpose |
| --- | --- |
| `cmd/run.go` | Orchestration loop — pre-loop sequence and main loop; the ACTIVE_TASK.md write site |
| `internal/agent/activetask.go` | `WriteActiveTask` — the only function that writes ACTIVE_TASK.md; primary injection point |
| `internal/agent/session.go` | `CreateSessionFile` — pre-creates agent session file; sets up the agent I/O contract |
| `internal/orchestrator/bootstrap.go` | `BootstrapFromTasks`, `IsEpicAlreadyComplete`, `NeedsKBSynthesis` |
| `internal/orchestrator/context.go` | `LoopContext` struct — carries all per-iteration state passed to handlers |
| `internal/templates/templates.go` | Embedded template files via `//go:embed`; two subdirectories: `runtime/` and `init/` |
| `internal/types/types.go` | All shared structs; `SessionResult` 3-field constraint; `TaskType.IsSynthetic()` |
| `docs/kb/README.md` | KB index table — agents' entry point for knowledge discovery |
| `docs/kb/packages/*.md` | 11 package KB articles; each has YAML frontmatter with `related_articles` |
| `docs/kb/infrastructure/*.md` | 1 infrastructure article |
| `docs/kb/patterns/*.md` | 2 pattern articles |
| `docs/kb/dependencies/*.md` | 1 dependency article |

---

## Current ACTIVE_TASK.md Structure

The `WriteActiveTask` function (`activetask.go:113`) currently writes exactly this structure:

```
# Active Task

**Task ID**: {taskID}
**Task Type**: {taskType}
**Session File**: {sessionFilePath}

---

{skillContent — from SKILL.md or hardcoded fallback}

[for bugfix only:]
---

## Bug Context

{content of logs/ACTIVE_BUG.md}
```

`ActiveTaskConfig` has five fields: `TaskID`, `TaskType`, `SessionFilePath`, `LogsDir`, `SkillsConfigPath`. **No KB context is injected today.** Agents discover KB articles on their own by reading `docs/kb/README.md` and navigating from there.

---

## Current KB Frontmatter Convention

Every article in `docs/kb/` uses this frontmatter shape (confirmed across all 15 files):

```yaml
---
title: internal/agent — Session, ActiveTask, Invoke, Parse
updated: 2026-02-24
category: Packages
tags: [agent, session, active-task, invoke, parse, exec, frontmatter, yaml]
related_articles:
  - docs/kb/packages/types.md
  - docs/kb/packages/log.md
  - docs/kb/infrastructure/go.md
  - docs/kb/patterns/pattern-exec-command.md
  - docs/kb/patterns/pattern-atomic-file-writes.md
---
```

**Critical gap**: There is **no `description` field** in the current frontmatter. The spec's `KBNode.Description` field has no source data today. Every KB file would need a new `description:` line added to its frontmatter before the graph can emit useful summaries.

---

## The "Discovery Penalty" — Current Reality

Today's discovery sequence for a typical agent:

1. Read `CLAUDE.md` — references `docs/kb/` as the source of truth
2. Read `docs/kb/README.md` — 34-line index table
3. Read 1–3 specific KB articles based on task type
4. Read relevant source files
5. Implement

For a project with 15 KB articles, this costs approximately **3–5 Read tool calls** before the agent can act. That is a real but **modest** penalty. The KB is small, the README is a well-structured index, and the articles are tightly scoped. The penalty would be meaningfully larger at 50–200 articles.

---

## Phase Analysis

### Phase 1: Boot-time Graph Builder

**Implementation location**: Between `LoadTasks` (Step 4) and `BootstrapFromTasks` (Step 5) in `cmd/run.go`.

**Implementation steps**:
1. `filepath.WalkDir("docs/kb", ...)` — reads only `.md` files
2. For each file: parse YAML frontmatter using `gopkg.in/yaml.v3` (already an approved dependency)
3. Build `map[string]KBNode` keyed on canonical path

**Go type**:
```go
type KBNode struct {
    Title       string
    Description string   // ← NOT in current frontmatter; must be added to all 15 files
    Tags        []string
    Edges       []string // canonical paths from related_articles
}
```

**Complexity**: Low. ~50 lines of new code in a new `internal/kb/` package. YAML parsing is already a solved pattern in this codebase. The operation is O(n) where n=15 files and each file is read for frontmatter only (body skipped).

**Blocking gap**: The `description` field must be backfilled into all 15 KB articles. Without it, injected summaries are empty and the Context Map provides no value beyond file paths.

**PRD alignment issue**: PRD.md states explicitly: *"The Go port is not a feature addition. It is a faithful translation of validated Bash logic into a language that supports the requirements above."* This graph builder is a new feature with no Bash equivalent. It belongs in a **post-v0.4.0 epic**.

---

### Phase 2: Context Injection into ACTIVE_TASK.md

**Implementation location**: `WriteActiveTask` in `internal/agent/activetask.go`. Add a new field to `ActiveTaskConfig`:

```go
type ActiveTaskConfig struct {
    // ... existing fields ...
    KBContext *KBContextBlock  // nil = no injection
}
```

**The mapping problem (critical gap)**: The spec says "Identify the Target File associated with the task." But there is **no current mechanism** to map a task to a KB article. Tasks in `tasks.yaml` have `id`, `type`, `status`, `description`, `acceptance_criteria` — no `kb_target` field. The orchestrator has no way to know that `EPIC-4-002` corresponds to `docs/kb/packages/agent.md` without either:

- (A) Adding a `kb_article` field to every task in `tasks.yaml` — requires schema change and user discipline
- (B) Injecting based on task type only (e.g., all `feature` tasks get the `packages/` subgraph) — imprecise
- (C) Injecting the entire KB README unconditionally — simpler than a graph, nearly equivalent value

**Output if implemented**:
```markdown
## Context Map

### You Are Here
**Target**: docs/kb/packages/agent.md — internal/agent — Session, ActiveTask, Invoke, Parse
**Tags**: agent, session, active-task, invoke, parse

### Direct Dependencies
- `docs/kb/packages/types.md` — internal/types — Shared Structs & Constants
- `docs/kb/packages/log.md` — internal/log — Colored Terminal Output
- `docs/kb/infrastructure/go.md` — Go Infrastructure & Best Practices
- `docs/kb/patterns/pattern-exec-command.md` — Safe subprocess invocation
- `docs/kb/patterns/pattern-atomic-file-writes.md` — write-to-temp-then-rename pattern
```

**Complexity**: Medium. The injection itself is straightforward. The hard part is solving the mapping problem cleanly.

---

### Phase 3: GetContextNeighborhood Runtime Tool

**The process boundary problem (critical gap)**: The spec describes `GetContextNeighborhood` as an "orchestrator lookup in the memory map." But the orchestrator process exits before the agent runs — or more precisely, the agent **is** a subprocess. The `RunAgent` call in `cmd/run.go:236` is:

```go
agent.RunAgent(cfg.AgentCommand, projectRoot)
```

This is `exec.Command("claude", ...)` — a new, separate process. There is **no shared memory** between the orchestrator and the agent at runtime. The "in-memory map" the spec describes does not exist during agent execution.

**Implementation options**:

| Option | Mechanism | Complexity | Fidelity to spec |
|--------|-----------|------------|-----------------|
| A | `doug lookup <path>` CLI subcommand — re-parses frontmatter on each call | Low-Medium | Low (no in-memory map) |
| B | Orchestrator runs a local HTTP server before invoking the agent | High | High (true in-memory) |
| C | MCP server exposing KB graph as a tool | High | High + agent-agnostic |
| D | Pre-dump the entire graph to a JSON file the agent can read | Low | Medium (static, not live) |

**Option A** is the pragmatic path. The "re-parse" cost for 15 files is ~5ms — irrelevant in practice even if called 10 times per session. It can be added as a subcommand (`doug graph <path>`) with no architectural changes to the main loop. The "in-memory efficiency" claim in the spec collapses at the process boundary anyway.

**Option D** (graph dump) is an alternative to static injection: on boot, the orchestrator writes `logs/KB_GRAPH.json` and the agent reads it directly via standard file tools. No new CLI surface required.

---

## Complexity vs Efficiency Gains Assessment

| Phase | Added Complexity | Efficiency Gain | Verdict |
|-------|-----------------|-----------------|---------|
| Phase 1: Graph Builder | Low (50 LOC, zero deps) | Minimal alone — only useful as Phase 2 enabler | Worth it if Phase 2 proceeds |
| Phase 2: Injection | Medium (mapping problem unsolved cleanly) | Saves 2–3 Read calls per task; ~5–10% token reduction | Moderate value; high for large KBs |
| Phase 3: Runtime Tool (Option A CLI) | Low-Medium | Saves 1–3 Read calls per deep-research task | Low value at current KB scale (15 articles) |
| Phase 3: Runtime Tool (Option B HTTP) | High | Same as Option A | Unjustified at 15 articles |

**Bottom line**: For a 15-article KB, the discovery penalty is small enough that a simpler intervention (injecting the KB README table unconditionally into ACTIVE_TASK.md) delivers **~80% of the benefit at ~10% of the complexity**. The full graph infrastructure pays off at scale (50+ articles).

---

## Additional Injection Opportunities for ACTIVE_TASK.md

Beyond the KB graph, the orchestrator has access to substantial context that agents currently rediscover on every run. These are ranked by value-to-effort ratio:

| Injection | Source | Effort | Value |
|-----------|--------|--------|-------|
| **Task description + acceptance criteria** | `tasks.yaml` via already-loaded `tasks` struct | Trivial | High — agents currently read tasks.yaml themselves |
| **Attempt number + "previous attempts: N"** | `projectState.ActiveTask.Attempts` (already in LoopContext) | Trivial | Medium — agents don't know they're on retry #3 |
| **Previous session file path** | Derivable from attempt - 1 | Low | Medium — retry context without reading the whole file |
| **KB README table** | `docs/kb/README.md` read once at startup | Low | High — 34-line index; eliminates most KB discovery |
| **Protected paths list** | Hardcoded in `handlers/success.go` | Trivial | Low — mostly handled by CLAUDE.md |
| **Current epoch branch name** | `projectState.CurrentEpic.BranchName` | Trivial | Low — agents rarely need this |
| **Build system** | `cfg.BuildSystem` | Trivial | Medium — agents sometimes guess build commands |
| **Max retries remaining** | `cfg.MaxRetries - attempts` | Trivial | Low-Medium — useful for retry strategy |

**Highest-value, lowest-effort candidates**:

1. **Task description injection**: `tasks.yaml` is already loaded into memory. Appending the task's full description and acceptance criteria to ACTIVE_TASK.md eliminates agents' first Read call of every session.

2. **KB README injection**: The `docs/kb/README.md` is a 34-line table. Including it in ACTIVE_TASK.md gives agents an immediate navigation map at near-zero cost.

3. **Attempt context**: Appending `**Attempt**: {N} of {MaxRetries}` to the header tells agents to be more conservative on attempt 3 vs attempt 1 — a behavioral nudge that costs nothing.

---

## Revisions & Alternative Approaches

### Recommended Alternative: Incremental Enrichment (No Graph Required)

Instead of building a full graph infrastructure, a simpler two-phase enrichment delivers most of the value:

**Phase A — Unconditional injections (trivial, do now)**:
- Inject `**Attempt**: {N} of {MaxRetries}` into the ACTIVE_TASK.md header
- Inject the task's `description` and `acceptance_criteria` from tasks.yaml (currently agents read this themselves)
- Inject the KB README table (34 lines) as a `## Knowledge Base Index` section

**Phase B — Conditional KB injection (low complexity, do after KB grows)**:
- Add `description:` field to KB frontmatter (one-time backfill)
- When KB grows past ~30 articles, implement Phase 1 graph + Phase 2 injection
- Use a `kb_article` field in tasks.yaml as the task→node mapping (explicit is better than inferred)

**Phase C — CLI tool (defer until agent behavior shows demand)**:
- `doug lookup <kb-path>` subcommand that re-parses frontmatter on each call
- No HTTP server, no daemon, no shared memory required
- Implement only when agents demonstrably exhaust their KB context mid-session

### Revision to Spec: Resolve the Mapping Problem Explicitly

The spec's "Identify the Target File associated with the task" is underspecified. The recommended resolution:

```yaml
# tasks.yaml — proposed schema addition
tasks:
  - id: "EPIC-4-002"
    type: "feature"
    kb_article: "docs/kb/packages/agent.md"   # ← explicit mapping
    description: "..."
```

This makes the graph injection deterministic and requires no heuristics. The orchestrator looks up `task.kb_article` in the graph, injects that node + its 1-hop neighbors. If `kb_article` is absent, the injection is skipped — no fallback heuristics, no silent failures.

---

## Implementation Complexity Summary

| Component | Effort Estimate | New Dependencies | PRD Status |
|-----------|----------------|-----------------|------------|
| `description` backfill in KB frontmatter | Trivial (15 files) | None | OK (docs only) |
| `internal/kb/` graph builder package | Low (~80 LOC) | None (uses `gopkg.in/yaml.v3`) | Post-v0.4.0 |
| `ActiveTaskConfig.KBContext` injection | Low-Medium (~60 LOC) | None | Post-v0.4.0 |
| `kb_article` field in tasks.yaml schema | Trivial (schema + `types.go`) | None | Post-v0.4.0 |
| `doug lookup` CLI subcommand | Medium (~100 LOC) | None | Post-v0.4.0 |
| HTTP server for in-memory tool | High (300+ LOC + lifecycle) | Likely net/http + sync | Not recommended |

**Total for pragmatic implementation** (Phases 1 + 2 + CLI subcommand, no HTTP server): ~240 LOC across 2–3 new files and minor changes to existing files. One new package (`internal/kb/`). No new external dependencies.

---

## PRD Alignment

The PRD is unambiguous: *"No new orchestrator features beyond what v0.3.0 provides."* The knowledge graph feature is explicitly out of scope for v0.4.0. It should be planned as a **post-v0.4.0 feature epic**, likely EPIC-7 or in a separate `v0.5.0` milestone.

The simpler enrichments (attempt count, task description injection) sit in a gray zone — they are not in the Bash v0.3.0 orchestrator but they are purely mechanical additions to ACTIVE_TASK.md with no new data sources or architectural changes. These could be included as quality-of-life improvements within the current port scope without violating the spirit of the PRD.

---

## Patterns Observed

- **All KB articles use a consistent frontmatter schema** — `title`, `updated`, `category`, `tags`, `related_articles` — making a graph builder straightforward to implement.
- **The injection point (`WriteActiveTask`) is already variadic** — it conditionally appends Bug Context for bugfix tasks. Adding KB context as another conditional section follows the same pattern with no architectural change.
- **`LoopContext` is the right carrier** — all per-iteration state flows through `LoopContext`. Adding a `KBGraph map[string]KBNode` to `LoopContext` (built once in pre-loop, passed through) is the natural fit.
- **No in-memory IPC exists** — the agent is a subprocess. Any "runtime tool" must either be a CLI subcommand (re-parses on call) or require a daemon model. The spec's in-memory framing does not survive the process boundary.

---

## Anti-Patterns & Tech Debt

- **`description` field missing from all KB frontmatter** — must be added before any graph-based summary injection has value. This is a one-time documentation debt.
- **No task→KB mapping** — tasks.yaml has no `kb_article` field. Without it, the graph injection either uses imprecise heuristics or requires the user to maintain an external mapping.
- **Spec assumes shared memory between orchestrator and agent** — the Phase 3 "in-memory" framing is architecturally incorrect given the subprocess model. The implementation path must abandon this framing or adopt a daemon model.

---

## Raw Notes

- **KB scale matters**: At 15 articles, the KB is small enough that injecting the full README (34 lines) unconditionally into ACTIVE_TASK.md is nearly equivalent to a graph query. The graph infrastructure starts earning its complexity at ~50 articles.
- **The real win is task description injection**: Agents currently always read `tasks.yaml` themselves (confirmed by observing agent behavior in sessions). This is the single highest-value, zero-risk injection available today with no new infrastructure.
- **Retry context injection is underrated**: Knowing they are on attempt 3 of 5 changes agent behavior meaningfully — they should try a different approach, not the same one. This is a free behavioral improvement.
- **Phase 3 demand signal**: Before building the CLI subcommand, check session logs for agents making multiple KB article reads per session. If agents typically read 2–3 articles and stop, the Explorer Tool adds little value. If agents are reading 6–8+ articles, the tool is justified.
