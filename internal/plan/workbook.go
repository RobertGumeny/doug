package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/robertgumeny/doug/internal/state"
)

const (
	planBriefStartTag = "<!-- DOUG-PLAN-BRIEF:START -->"
	planBriefEndTag   = "<!-- DOUG-PLAN-BRIEF:END -->"
)

var planBriefPattern = regexp.MustCompile("(?s)^" + regexp.QuoteMeta(planBriefStartTag) + "\\n.*?\\n" + regexp.QuoteMeta(planBriefEndTag) + "\\n*")

// WorkbookContext captures the Doug-owned context written into PLAN.md.
type WorkbookContext struct {
	PlanningIntent     string
	PlanningMode       string
	TargetEpicHint     string
	LastHandoffAt      string
	LastHandoffArchive string
	LastHandoffEpicIDs []string
	ArchivedBugs       []ArchivedBugContext
}

// EnsurePlanDocument creates or refreshes the active PLAN.md workbook.
func EnsurePlanDocument(dougDir string, ctx WorkbookContext) (string, bool, error) {
	planDir := filepath.Join(dougDir, backlogPlanDirName)
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return "", false, fmt.Errorf("create plan directory %s: %w", planDir, err)
	}

	planPath := filepath.Join(planDir, planFileName)
	data, err := os.ReadFile(planPath)
	if err == nil {
		refreshed := RefreshPlanDocument(string(data), ctx)
		if err := state.AtomicWrite(planPath, []byte(refreshed)); err != nil {
			return "", false, fmt.Errorf("write %s: %w", planPath, err)
		}
		return planPath, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, fmt.Errorf("stat %s: %w", planPath, err)
	}

	if err := state.AtomicWrite(planPath, []byte(InitialPlanDocument(ctx))); err != nil {
		return "", false, fmt.Errorf("write %s: %w", planPath, err)
	}
	return planPath, true, nil
}

// InitialPlanDocument returns the seeded planning workbook for a new cycle.
func InitialPlanDocument(ctx WorkbookContext) string {
	projectMode := "brownfield"
	manifestGuidance := "# Include `manifest` only when the plan needs greenfield scaffold output."
	if strings.EqualFold(strings.TrimSpace(ctx.PlanningMode), "greenfield") {
		projectMode = "greenfield"
		manifestGuidance = "# Greenfield planning mode: `manifest` is required in handoff-ready output."
	}

	return RefreshPlanDocument(""+
		"# Project Plan\n\n"+
		"## Planning Objective\n\n"+
		"Describe the planning request, intended outcome, and why it matters now.\n\n"+
		"## Current Context\n\n"+
		"Capture the relevant codebase, product, architectural, or backlog context here.\n\n"+
		"## Scope And Non-Goals\n\n"+
		"- In scope:\n"+
		"- Out of scope:\n\n"+
		"## Risks, Assumptions, And Open Questions\n\n"+
		"- Risks:\n"+
		"- Assumptions:\n"+
		"- Open questions:\n\n"+
		"## Proposed Epics\n\n"+
		"Document the intended epics, sequence, and rationale here.\n\n"+
		"## Handoff Readiness\n\n"+
		"State whether the plan is exploratory, ready for review, or ready for deterministic handoff.\n\n"+
		"## Handoff Data\n\n"+
		"```yaml\n"+
		"# Fill in this schema exactly. Do not add extra fields.\n"+
		"# Unknown fields cause `doug handoff` to fail.\n"+
		"schema_version: 1\n"+
		"project:\n"+
		"  name: \"My Project\"\n"+
		"  mode: \""+projectMode+"\"\n"+
		manifestGuidance+"\n"+
		"# When included, use this exact schema.\n"+
		"# manifest:\n"+
		"#   schema_version: 1\n"+
		"#   project:\n"+
		"#     name: \"My Project\"\n"+
		"#     mode: \"greenfield\"\n"+
		"#   scaffold:\n"+
		"#     language: \"typescript\"\n"+
		"#     runtime: \"node\"\n"+
		"#     framework: \"nextjs\"\n"+
		"#     package_manager: \"pnpm\"\n"+
		"#     build_system: \"npm-scripts\"\n"+
		"#   dependencies:\n"+
		"#     runtime:\n"+
		"#       - \"next@current-stable-version\"\n"+
		"#     development:\n"+
		"#       - \"typescript@current-stable-version\"\n"+
		"#   constraints:\n"+
		"#     - \"Describe a scaffold constraint here.\"\n"+
		"epics:\n"+
		"  # Use EPIC-<X> placeholders for new epics. Doug allocates the concrete\n"+
		"  # EPIC-<N> identifiers at handoff, so you don't hand-author absolute numbers.\n"+
		"  - id: \"EPIC-<X>\"\n"+
		"    name: \"Example Epic\"\n"+
		"    prd: |\n"+
		"      # PRD\n"+
		"\n"+
		"      Describe the epic's product requirements here.\n"+
		"    tasks:\n"+
		"      - id: \"EPIC-<X>-001\"\n"+
		"        type: \"feature\"\n"+
		"        status: \"TODO\"\n"+
		"        description: \"Describe the task here.\"\n"+
		"        acceptance_criteria:\n"+
		"          - \"First acceptance criterion.\"\n"+
		"          - \"Second acceptance criterion.\"\n"+
		"```\n", ctx)
}

// RefreshPlanDocument rewrites the Doug-owned brief and preserves the workbook body.
func RefreshPlanDocument(existing string, ctx WorkbookContext) string {
	body := strings.ReplaceAll(existing, "\r\n", "\n")
	body = planBriefPattern.ReplaceAllString(body, "")
	body = strings.TrimLeft(body, "\n")
	if body == "" {
		body = "# Project Plan\n"
	}
	return planBriefBlock(ctx) + "\n\n" + strings.TrimRight(body, "\n") + "\n"
}

func planBriefBlock(ctx WorkbookContext) string {
	lines := []string{
		planBriefStartTag,
		"# Planning Session",
		"",
		"This is your planning workbook. Work directly in this file — update the narrative sections as you go and keep `## Handoff Data` consistent with them.",
		"",
		"Your goal is to turn the request into a plan clear enough that another agent can implement it without guesswork. Use `.doug/ACTIVE_TASK.md` as the brief for this session.",
		"",
		"A few things to keep in mind:",
		"- Don't fill in `## Handoff Data` until you've produced an alignment summary and the user has confirmed it.",
		"- The YAML schema in `## Handoff Data` is fixed — use only the fields shown in the template. Extra fields will cause `doug handoff` to reject the payload.",
		"- For greenfield/bootstrap work, use the `manifest` block rather than `epics` alone.",
		"- This file is the single working artifact — don't create alternate planning files.",
		"",
		"**This session:**",
		"- Intent: " + planBriefValue(ctx.PlanningIntent, "not specified"),
		"- Mode: " + planBriefValue(ctx.PlanningMode, "auto"),
		"- Target epic: " + planBriefValue(ctx.TargetEpicHint, "not specified"),
	}

	if strings.EqualFold(strings.TrimSpace(ctx.PlanningMode), "greenfield") {
		lines = append(lines,
			"",
			"**Greenfield handoff directive:** Because this planning session is in greenfield mode, the `manifest` block is required output in `## Handoff Data` before handoff-ready completion.",
		)
	}

	lines = append(lines,
		"",
		"**Downstream awareness:** After each epic completes, Doug automatically runs a post-epic KB pass. That pass reads archived session logs and `PLAN.md`, then writes knowledge-base updates only under `docs/kb/`.",
	)

	if ctx.LastHandoffArchive != "" || len(ctx.LastHandoffEpicIDs) > 0 || ctx.LastHandoffAt != "" {
		lines = append(lines,
			"",
			"**Last handoff:** "+planBriefValue(ctx.LastHandoffAt, "not recorded")+" — epics "+planBriefValue(strings.Join(ctx.LastHandoffEpicIDs, ", "), "none")+" were handed off and are now tracked in the backlog. Start the next cycle fresh: plan new work as EPIC-<X> placeholders and let handoff allocate concrete IDs, rather than re-submitting these already-handed-off epic definitions as new intake.",
		)
	}

	if len(ctx.ArchivedBugs) > 0 {
		lines = append(lines,
			"",
			"**Unresolved bugs** (from `.doug/logs/bugs/`) — treat these as planning intake:",
		)
		for _, bug := range ctx.ArchivedBugs {
			lines = append(lines, "- "+bug.PlanningBullet())
		}
	}

	lines = append(lines,
		"",
		planBriefEndTag,
	)
	return strings.Join(lines, "\n")
}

func planBriefValue(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return strings.ReplaceAll(trimmed, "\n", "\n  ")
}
