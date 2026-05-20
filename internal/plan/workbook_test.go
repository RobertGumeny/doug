package plan

import (
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/types"
)

func TestRefreshPlanDocument_RendersMultilinePlanningIntent(t *testing.T) {
	doc := RefreshPlanDocument("# Existing Plan\n", WorkbookContext{
		PlanningIntent: "Plan a safer composer\nInclude plan intent capture",
	})

	want := "- Planning intent: Plan a safer composer\n  Include plan intent capture"
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
		"Unresolved Archived Bugs:",
		"Review these archived bug reports as planning intake sourced from `.doug/logs/bugs/{epic}/`.",
		"Re-enter deferred bug work by creating new planning work or updating `PLANNED` backlog work here; do not maintain a second manual intake artifact.",
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
