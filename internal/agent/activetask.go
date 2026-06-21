package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	buildconfig "github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/types"
)

// ActiveTaskConfig holds the parameters for writing .doug/ACTIVE_TASK.md.
type ActiveTaskConfig struct {
	TaskID   string
	TaskType types.TaskType
	// DougDir is the path to the .doug/ directory. ACTIVE_TASK.md is written
	// to {DougDir}/ACTIVE_TASK.md.
	DougDir string
	// ProjectRoot is the absolute path to the project root. When set, PRD and
	// KB context references are rendered as repo-relative paths (e.g. .doug/PRD.md)
	// to improve prompt-cache friendliness across projects.
	ProjectRoot string
	// Description is the task description from tasks.yaml. Empty for synthetic tasks.
	Description string
	// AcceptanceCriteria is the list of acceptance criteria from tasks.yaml.
	// Empty for synthetic tasks (bugfix, documentation).
	AcceptanceCriteria []string
	// Attempts is the current attempt number (already incremented before WriteActiveTask is called).
	Attempts int
	// MaxRetries is the configured maximum number of retries from doug.yaml.
	MaxRetries int
	// BuildSystem is the build system identifier (e.g. "go", "npm", "pnpm").
	// If set and found in the BuildSystems registry, a "## Build System" briefing
	// section is injected into ACTIVE_TASK.md. If empty or unknown, the section is omitted.
	BuildSystem string
	// TestFailureOutput holds the captured output from a failed test run on the
	// previous attempt. When non-empty, it is injected into ACTIVE_TASK.md so
	// the agent can see what tests are failing and fix them.
	TestFailureOutput string
	// ContextSections appends structured context blocks to ACTIVE_TASK.md.
	// This is used for synthetic tasks like scaffold that need extra agent-facing
	// context beyond the standard task description and criteria.
	ContextSections []ActiveTaskSection

	// Bug payload fields (used for synthetic BUG-<taskID> bugfix tasks).
	// When BugID is non-empty, the bug context section is rendered directly from
	// these fields without reading any separate file. BugBody is the raw markdown
	// written by the agent that discovered the bug (summary + reproduction steps).
	// BugArchivePath is the relative path to the durable archive for reference.
	BugID          string
	BugSeverity    string
	BugSourceTask  string
	BugBody        string
	BugArchivePath string
}

// ActiveTaskSection is an extra markdown section appended to ACTIVE_TASK.md.
type ActiveTaskSection struct {
	Heading string
	Body    string
}

// hardcodedSkillNames maps known task types to their built-in workflow skill names.
// Repository-specific operating rules live in AGENTS.md; this map only selects the
// task workflow.
var hardcodedSkillNames = map[string]string{
	string(types.TaskTypeFeature):       "implement-feature",
	string(types.TaskTypeBugfix):        "implement-bugfix",
	string(types.TaskTypeDocumentation): "implement-documentation",
	string(types.TaskTypeScaffold):      "scaffold",
	string(types.TaskTypePlan):          "plan",
	string(types.TaskTypeResearch):      "research",
}

// DefaultSkillName returns the built-in skill name for taskType from hardcodedSkillNames.
// Returns ("", false) for unknown task types.
func DefaultSkillName(taskType string) (string, bool) {
	name, ok := hardcodedSkillNames[taskType]
	return name, ok
}

// bugOutcomeValidFor returns true for task types where BUG is a recognized outcome
// routed through HandleBug in the main runtime orchestration loop. Plan, scaffold,
// and research tasks are dispatched through separate command flows that do not call
// HandleBug, so BUG is not a valid or useful outcome for those types.
func bugOutcomeValidFor(taskType types.TaskType) bool {
	return taskType == types.TaskTypeFeature || taskType == types.TaskTypeDocumentation
}

// prdPath returns a repo-relative path to PRD.md suitable for agent-facing
// display. When projectRoot is non-empty, a relative path is computed from
// the project root; otherwise the .doug/PRD.md literal is returned.
func prdPath(projectRoot, dougDir string) string {
	if projectRoot != "" {
		if rel, err := filepath.Rel(projectRoot, filepath.Join(dougDir, "PRD.md")); err == nil {
			return rel
		}
	}
	return ".doug/PRD.md"
}

// WriteActiveTask writes .doug/ACTIVE_TASK.md with task metadata and a briefing
// header. The file is archived and cleaned up after the corresponding outcome is processed.
//
// For bugfix tasks, the bug context section is rendered directly from the
// BugID/BugSeverity/BugSourceTask/BugBody fields in config. No separate file
// is read; all bug context is carried on the active task state.
func WriteActiveTask(config ActiveTaskConfig, l log.Logger) error {
	var sb strings.Builder
	sb.WriteString("# Task Brief\n\n")
	sb.WriteString("Fill in the **## Result** section at the bottom of this file when you're done.\n\n")
	// Stable context pointers first — these are consistent across tasks of the same
	// type and improve prompt-cache hits when variable task details follow below.
	sb.WriteString("**Context**:\n")
	fmt.Fprintf(&sb, "- PRD: `%s` — product requirements and constraints (read when relevant to the task)\n", prdPath(config.ProjectRoot, config.DougDir))
	sb.WriteString("- Knowledge base: `docs/kb/README.md` — read the index first, then only the articles relevant to your task\n")
	if bugOutcomeValidFor(config.TaskType) {
		sb.WriteString("- Blocking bug: set `outcome: BUG` with `bugs: [{severity: blocking, body: \"...\"}]` in `## Result` only when you must stop — i.e., the bug makes this task's acceptance criteria impossible to verify, requires committing a change that violates the acceptance criteria, or would directly introduce a regression. For all other bugs found during this task, use `bugs: [{severity: non-blocking, body: \"...\"}]` and finish the task.\n")
	} else if config.TaskType == types.TaskTypeBugfix {
		sb.WriteString("`BUG` outcome is not available for bugfix tasks — reporting it would create a nested-bug death spiral. If you discover an unrelated issue, record it as `bugs: [{severity: non-blocking, body: \"...\"}]` in the result and complete this task.\n")
	}
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "**Task ID**: %s\n", config.TaskID)
	fmt.Fprintf(&sb, "**Task Type**: %s\n", string(config.TaskType))
	fmt.Fprintf(&sb, "**Attempt**: %d of %d\n", config.Attempts, config.MaxRetries)
	if config.Description != "" {
		fmt.Fprintf(&sb, "**Description**: %s\n", config.Description)
	}
	if len(config.AcceptanceCriteria) > 0 {
		sb.WriteString("\n**Acceptance Criteria**:\n")
		for _, criterion := range config.AcceptanceCriteria {
			fmt.Fprintf(&sb, "- %s\n", criterion)
		}
	}

	if info, ok := buildconfig.BuildSystems[config.BuildSystem]; ok {
		sb.WriteString("\n\n---\n\n## Build System\n\n")
		fmt.Fprintf(&sb, "**System**: %s\n", config.BuildSystem)
		fmt.Fprintf(&sb, "**Install**: `%s`\n", info.InstallCmd)
		if len(info.VerifyCommands) > 0 {
			sb.WriteString("**Verify**:\n")
			for _, cmd := range info.VerifyCommands {
				fmt.Fprintf(&sb, "- `%s`\n", cmd)
			}
		}
		if len(info.CommonPitfalls) > 0 {
			sb.WriteString("**Common Pitfalls**:\n")
			for _, p := range info.CommonPitfalls {
				fmt.Fprintf(&sb, "- %s\n", p)
			}
		}
	}

	if config.TaskType == types.TaskTypeBugfix && config.BugID != "" {
		sb.WriteString("\n\n---\n\n## Bug Context\n\n")
		if config.BugID != "" {
			fmt.Fprintf(&sb, "**Bug ID**: %s\n", config.BugID)
		}
		if config.BugSeverity != "" {
			fmt.Fprintf(&sb, "**Severity**: %s\n", config.BugSeverity)
		}
		if config.BugSourceTask != "" {
			fmt.Fprintf(&sb, "**Source Task**: %s\n", config.BugSourceTask)
		}
		if config.BugBody != "" {
			sb.WriteString("\n")
			sb.WriteString(config.BugBody)
			if !strings.HasSuffix(config.BugBody, "\n") {
				sb.WriteString("\n")
			}
		}
		if config.BugArchivePath != "" {
			fmt.Fprintf(&sb, "\n**Archive** (durable reference): `%s`\n", config.BugArchivePath)
		}
	}

	if config.TestFailureOutput != "" {
		sb.WriteString("\n\n---\n\n## Previous Test Failure Output\n\n")
		sb.WriteString("The previous attempt reported SUCCESS but the following tests failed during orchestrator verification.\n")
		sb.WriteString("Fix the failing tests before reporting SUCCESS again.\n\n")
		sb.WriteString("```\n")
		sb.WriteString(config.TestFailureOutput)
		sb.WriteString("\n```\n")
	}

	for _, section := range config.ContextSections {
		if strings.TrimSpace(section.Heading) == "" || strings.TrimSpace(section.Body) == "" {
			continue
		}
		sb.WriteString("\n\n---\n\n## ")
		sb.WriteString(section.Heading)
		sb.WriteString("\n\n")
		sb.WriteString(section.Body)
		if !strings.HasSuffix(section.Body, "\n") {
			sb.WriteString("\n")
		}
	}

	// Append the result block that the agent fills in.
	sb.WriteString("\n\n---\n\n## Result\n\n")
	if bugOutcomeValidFor(config.TaskType) {
		sb.WriteString("Set `outcome` to one of: `SUCCESS`, `FAILURE`, `BUG`, `EPIC_COMPLETE`.\n")
		sb.WriteString("The `bugs` field reports discovered issues: `severity: blocking` requires `outcome: BUG` and interrupts the task; `severity: non-blocking` is archived without interrupting — finish the task and report the normal outcome.\n\n")
	} else if config.TaskType == types.TaskTypeBugfix {
		sb.WriteString("Set `outcome` to one of: `SUCCESS`, `FAILURE`, `EPIC_COMPLETE`.\n")
		sb.WriteString("The `bugs` field accepts `severity: non-blocking` entries — archived by Doug without interrupting the task.\n\n")
	} else {
		sb.WriteString("Set `outcome` to one of: `SUCCESS`, `FAILURE`.\n")
		sb.WriteString("The `bugs` field accepts `severity: non-blocking` entries — archived by Doug without interrupting the task.\n\n")
	}
	sb.WriteString("---\n")
	sb.WriteString("outcome: \"\"\n")
	sb.WriteString("changelog_entry: \"\"\n")
	sb.WriteString("dependencies_added: []\n")
	sb.WriteString("bugs: []\n")
	sb.WriteString("---\n\n")
	sb.WriteString("## Summary\n\n")
	sb.WriteString("## Files Changed\n\n")
	sb.WriteString("## Key Decisions\n\n")
	sb.WriteString("## Verification\n")

	outPath := filepath.Join(config.DougDir, "ACTIVE_TASK.md")
	if err := os.MkdirAll(config.DougDir, 0o755); err != nil {
		return fmt.Errorf("create .doug directory %s: %w", config.DougDir, err)
	}
	if err := os.WriteFile(outPath, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("write ACTIVE_TASK.md: %w", err)
	}

	return nil
}


