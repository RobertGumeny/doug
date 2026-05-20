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
		"  mode: \"brownfield\"\n"+
		"# Include `manifest` only when the plan needs greenfield scaffold output.\n"+
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
		"#       - \"next\"\n"+
		"#     development:\n"+
		"#       - \"typescript\"\n"+
		"#   constraints:\n"+
		"#     - \"Describe a scaffold constraint here.\"\n"+
		"epics:\n"+
		"  - id: \"EPIC-1\"\n"+
		"    name: \"Example Epic\"\n"+
		"    prd: |\n"+
		"      # PRD\n"+
		"\n"+
		"      Describe the epic's product requirements here.\n"+
		"    tasks:\n"+
		"      - id: \"EPIC-1-001\"\n"+
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
		"# Doug Planning Brief",
		"",
		"This is the editable planning workbook for this run.",
		"Use it to refine scope, risks, epic sequencing, and executable tasks from the canonical brief in `.doug/ACTIVE_TASK.md`.",
		"",
		"Rules:",
		"- Treat `.doug/ACTIVE_TASK.md` as the canonical Doug-managed brief for this planning run.",
		"- Update `.doug/plan/PLAN.md` directly as the editable planning workbook.",
		"- Use the narrative sections for collaborative planning notes and rationale.",
		"- Keep the plan coherent as you refine it; do not create alternate planning files or stage documents.",
		"- Keep the deterministic payload under `## Handoff Data` aligned with the surrounding narrative plan.",
		"- Fill in the seeded YAML schema exactly; do not add fields beyond the ones shown in the template.",
		"- Put greenfield scaffold data under `manifest`, not under `project`.",
		"- When the repository is empty or near-empty and the user explicitly wants day-0 bootstrap work, prefer scaffold-oriented handoff data under `manifest` instead of defaulting to an implementation epic.",
		"- `doug handoff` owns generated derivatives such as `.doug/plan/epics/<EPIC-ID>/` and `.doug/plan/manifest.yaml`; they are downstream artifacts, not competing briefs.",
		"",
		"`.doug/plan/PLAN.md` is the source of truth for this planning cycle; do not fill `## Handoff Data` until you have produced an alignment summary and the user has explicitly confirmed it.",
		"Once confirmed, fill the `## Handoff Data` YAML payload exactly as seeded so `doug handoff` can parse it without guesswork. Unknown fields are rejected.",
		"For explicit bootstrap intent, make the handoff scaffold-ready: capture the stack, runtime, framework, package-manager, dependencies, and constraints Doug will need to derive `.doug/plan/manifest.yaml`.",
		"",
		"Planning Run Context:",
		"- Planning intent: " + planBriefValue(ctx.PlanningIntent, "not provided on this CLI run"),
		"- Planning mode: " + planBriefValue(ctx.PlanningMode, "auto / not specified"),
		"- Target epic hint: " + planBriefValue(ctx.TargetEpicHint, "not specified"),
		"- If the existing workbook narrative disagrees with this run context, reconcile the workbook to match before continuing so CLI intent and PLAN.md do not diverge.",
	}

	if ctx.LastHandoffArchive != "" || len(ctx.LastHandoffEpicIDs) > 0 || ctx.LastHandoffAt != "" {
		lines = append(lines,
			"",
			"Latest Handoff Context:",
			"- Last handoff completed at: "+planBriefValue(ctx.LastHandoffAt, "not recorded"),
			"- Archived workbook: "+planBriefValue(ctx.LastHandoffArchive, "not recorded"),
			"- Handed-off epics: "+planBriefValue(strings.Join(ctx.LastHandoffEpicIDs, ", "), "not recorded"),
			"- Start the next planning cycle here instead of reusing handed-off epic definitions as active intake content.",
		)
	}

	if len(ctx.ArchivedBugs) > 0 {
		lines = append(lines,
			"",
			"Unresolved Archived Bugs:",
			"- Review these archived bug reports as planning intake sourced from `.doug/logs/bugs/{epic}/`.",
			"- Re-enter deferred bug work by creating new planning work or updating `PLANNED` backlog work here; do not maintain a second manual intake artifact.",
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
