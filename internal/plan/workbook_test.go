package plan

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/types"
)

// TestPlanBriefBlock_ContainsAlignmentCheckpoint ensures the alignment-summary-
// and-confirmation requirement is present in the Doug planning brief injected into
// every PLAN.md. This is a narrow regression guard: if the checkpoint phrase is
// accidentally removed from planBriefBlock(), this test fails before any integration
// test has a chance to catch it.
func TestPlanBriefBlock_ContainsAlignmentCheckpoint(t *testing.T) {
	doc := RefreshPlanDocument("# Project Plan\n", WorkbookContext{
		PlanningIntent: "Shape the next epic",
	})

	for _, want := range []string{
		"alignment summary",
		"the user has confirmed it",
		"single working artifact",
		"Don't fill in `## Handoff Data`",
	} {
		if !strings.Contains(doc, want) {
			t.Fatalf("expected alignment-checkpoint phrase %q in plan brief block, got:\n%s", want, doc)
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
