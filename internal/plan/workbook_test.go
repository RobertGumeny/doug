package plan

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/types"
)

// TestPlanBriefBlock_PreservesDynamicSessionContextAndDefersWorkflowToSkill
// verifies the refreshed brief still carries the dynamic session context (intent,
// mode, target) and points at the `plan` skill for the generic workflow rules,
// while no longer embedding the generic tutorial/process prose that now lives in
// the skill. It asserts structure, not exact tutorial wording.
func TestPlanBriefBlock_PreservesDynamicSessionContextAndDefersWorkflowToSkill(t *testing.T) {
	doc := RefreshPlanDocument("# Project Plan\n", WorkbookContext{
		PlanningIntent: "Shape the next epic",
		PlanningMode:   "definition",
		TargetEpicHint: "EPIC-19",
	})

	for _, want := range []string{
		"**This session:**",
		"- Intent: Shape the next epic",
		"- Mode: definition",
		"- Target epic: EPIC-19",
		"`plan` skill",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("expected dynamic-context phrase %q in plan brief block, got:\n%s", want, doc)
		}
	}

	// The generic workflow tutorial prose now belongs to the plan skill and must
	// not be re-embedded in the refreshed PLAN brief.
	for _, unwanted := range []string{
		"single working artifact",
		"the user has confirmed it",
		"Don't fill in `## Handoff Data`",
		"Extra fields will cause `doug handoff` to reject the payload.",
		"use the `manifest` block rather than `epics` alone",
	} {
		if strings.Contains(doc, unwanted) {
			t.Fatalf("did not expect generic workflow prose %q in plan brief block, got:\n%s", unwanted, doc)
		}
	}
}

// TestInitialPlanDocument_SeedDistinguishesDraftFromConfirmedHandoff ensures the
// seeded PLAN.md workbook includes a Handoff Readiness section that makes the
// draft-versus-confirmed-handoff boundary explicit from day one. This prevents
// the seed from silently losing the exploratory / review / deterministic language
// that signals planning state to both humans and agents.
func TestInitialPlanDocument_SeedDistinguishesDraftFromConfirmedHandoff(t *testing.T) {
	doc := InitialPlanDocument(WorkbookContext{PlanningIntent: "Bootstrap the project"})

	for _, want := range []string{
		"## Handoff Readiness",
		"exploratory",
		"ready for deterministic handoff",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("expected draft-vs-confirmed phrase %q in initial PLAN.md seed, got:\n%s", want, doc)
		}
	}
}

func TestInitialPlanDocument_GreenfieldSeedDoesNotDefaultToBrownfield(t *testing.T) {
	doc := InitialPlanDocument(WorkbookContext{
		PlanningIntent: "Bootstrap the project",
		PlanningMode:   "greenfield",
	})

	for _, want := range []string{
		`  mode: "greenfield"`,
		"Greenfield planning mode: `manifest` is required in handoff-ready output.",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("expected greenfield seed phrase %q in initial PLAN.md seed, got:\n%s", want, doc)
		}
	}
	if strings.Contains(doc, `  mode: "brownfield"`) {
		t.Fatalf("greenfield PLAN.md seed must not default project.mode to brownfield, got:\n%s", doc)
	}
}

func TestPlanBriefBlock_GreenfieldRequiresManifestWithoutRemovingLifecycleNote(t *testing.T) {
	doc := RefreshPlanDocument("# Project Plan\n", WorkbookContext{
		PlanningIntent: "Bootstrap the project",
		PlanningMode:   "greenfield",
	})

	for _, want := range []string{
		"**Greenfield handoff directive:**",
		"the `manifest` block is required output in `## Handoff Data`",
		"**Downstream awareness:**",
		"post-epic KB pass",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("expected greenfield brief phrase %q in plan brief block, got:\n%s", want, doc)
		}
	}
}

func TestPlanBriefBlock_IncludesDownstreamKBAwarenessInDougOwnedBlock(t *testing.T) {
	doc := RefreshPlanDocument("# Project Plan\n", WorkbookContext{
		PlanningIntent: "Shape the next epic",
	})

	start := strings.Index(doc, planBriefStartTag)
	end := strings.Index(doc, planBriefEndTag)
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("expected Doug-owned plan brief block in document, got:\n%s", doc)
	}
	brief := doc[start:end]
	body := doc[end+len(planBriefEndTag):]

	for _, want := range []string{
		"After each epic completes, Doug automatically runs a post-epic KB pass",
		"reads archived session logs and `PLAN.md`",
		"writes knowledge-base updates only under `docs/kb/`",
	} {
		if !strings.Contains(brief, want) {
			t.Fatalf("expected downstream KB phrase %q in Doug-owned plan brief block, got:\n%s", want, doc)
		}
		if strings.Contains(body, want) {
			t.Fatalf("downstream KB phrase %q must not be injected into editable narrative body, got:\n%s", want, doc)
		}
	}
}

func TestRefreshPlanDocument_RendersMultilinePlanningIntent(t *testing.T) {
	doc := RefreshPlanDocument("# Existing Plan\n", WorkbookContext{
		PlanningIntent: "Plan a safer composer\nInclude plan intent capture",
	})

	want := "- Intent: Plan a safer composer\n  Include plan intent capture"
	if !strings.Contains(doc, want) {
		t.Fatalf("expected multiline planning intent to be indented as one bullet, got:\n%s", doc)
	}
}

func TestRefreshPlanDocument_RendersArchivedBugContext(t *testing.T) {
	status := types.EpicStatusActive
	doc := RefreshPlanDocument("# Existing Plan\n", WorkbookContext{
		PlanningIntent: "Revisit deferred bug follow-up",
		ArchivedBugs: []ArchivedBugContext{
			{
				BugID:          "bug-epic-2-open",
				SourceEpicID:   "EPIC-2",
				SourcePath:     ".doug/logs/bugs/EPIC-2/bug-epic-2-open.md",
				Status:         "open",
				Severity:       "blocking",
				Summary:        "Active epic bug summary.",
				EpicStatus:     &status,
				PlanningAction: "treat follow-up as new planning work; do not reopen or mutate the `ACTIVE` backlog package",
			},
		},
	})

	for _, want := range []string{
		"**Unresolved bugs**",
		"`bug-epic-2-open` from epic `EPIC-2`",
		"source epic lifecycle `ACTIVE`",
		"do not reopen or mutate the `ACTIVE` backlog package",
		"# Existing Plan",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("expected %q in refreshed plan document, got:\n%s", want, doc)
		}
	}
}
