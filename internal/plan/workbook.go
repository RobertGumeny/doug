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

// IntakeSection is a named planning-intake section whose bullets are already
// prepared by source-specific loaders.
type IntakeSection struct {
	Header  string
	Bullets []string
}

// WorkbookContext captures the Doug-owned context written into PLAN.md.
type WorkbookContext struct {
	PlanningIntent     string
	PlanningMode       string
	TargetEpicHint     string
	LastHandoffAt      string
	LastHandoffArchive string
	LastHandoffEpicIDs []string
	IntakeSections     []IntakeSection
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
		"Do not author final handoff YAML here until the user has explicitly confirmed the alignment summary. Keep this non-final stub in place while the plan is still in draft.\n\n"+
		"When you are ready to finalize after that confirmation, consult the reusable handoff template colocated with the `plan` skill at `.pi/skills/plan/references/handoff-template.yaml`, then replace this stub with the completed schema.\n\n"+
		"```yaml\n"+
		"# Non-final stub \u2014 not handoff-ready.\n"+
		"# Replace with the full schema (see plan skill references/handoff-template.yaml)\n"+
		"# only after the user confirms the alignment summary.\n"+
		"schema_version: 1\n"+
		"project:\n"+
		"  name: \"My Project\"\n"+
		"  mode: \""+projectMode+"\"\n"+
		manifestGuidance+"\n"+
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
		"Follow the `plan` skill for the Doug planning workflow — using this workbook, keeping `## Handoff Data` to the fixed schema, and waiting for explicit alignment confirmation before writing final handoff data.",
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

	for _, section := range ctx.IntakeSections {
		lines = appendIntakeSectionLines(lines, section)
	}

	lines = append(lines,
		"",
		planBriefEndTag,
	)
	return strings.Join(lines, "\n")
}

func appendIntakeSectionLines(lines []string, section IntakeSection) []string {
	if strings.TrimSpace(section.Header) == "" || len(section.Bullets) == 0 {
		return lines
	}

	lines = append(lines, "", section.Header)
	for _, bullet := range section.Bullets {
		lines = append(lines, "- "+bullet)
	}
	return lines
}

func planBriefValue(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return strings.ReplaceAll(trimmed, "\n", "\n  ")
}
