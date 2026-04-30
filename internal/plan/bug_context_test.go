package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

func TestLoadArchivedBugContext_FiltersAndAnnotatesByEpicLifecycle(t *testing.T) {
	dir := t.TempDir()

	writeArchivedBug(t, dir, "EPIC-1", "bug-epic-1-open.md", ""+
		"---\n"+
		"bug_id: \"bug-epic-1-open\"\n"+
		"status: \"open\"\n"+
		"severity: \"non-blocking\"\n"+
		"---\n\n"+
		"## Summary\n\n"+
		"Planned epic bug summary.\n")
	writeArchivedBug(t, dir, "EPIC-2", "bug-epic-2-open.md", ""+
		"---\n"+
		"bug_id: \"bug-epic-2-open\"\n"+
		"status: \"in_progress\"\n"+
		"severity: \"blocking\"\n"+
		"---\n\n"+
		"## Summary\n\n"+
		"Active epic bug summary.\n")
	writeArchivedBug(t, dir, "EPIC-3", "bug-epic-3-open.md", ""+
		"---\n"+
		"bug_id: \"bug-epic-3-open\"\n"+
		"status: \"open\"\n"+
		"severity: \"non-blocking\"\n"+
		"---\n\n"+
		"## Summary\n\n"+
		"Completed epic bug summary.\n")
	writeArchivedBug(t, dir, "EPIC-4", "bug-epic-4-fixed.md", ""+
		"---\n"+
		"bug_id: \"bug-epic-4-fixed\"\n"+
		"status: \"fixed\"\n"+
		"severity: \"non-blocking\"\n"+
		"---\n\n"+
		"## Summary\n\n"+
		"Should be filtered.\n")
	writeArchivedBug(t, dir, "EPIC-5", "bug-epic-5-open.md", ""+
		"---\n"+
		"bug_id: \"bug-epic-5-open\"\n"+
		"status: \"open\"\n"+
		"severity: \"non-blocking\"\n"+
		"---\n\n"+
		"## Summary\n\n"+
		"No metadata bug summary.\n")

	writeEpicMetadata(t, dir, "EPIC-1", types.EpicStatusPlanned)
	writeEpicMetadata(t, dir, "EPIC-2", types.EpicStatusActive)
	writeEpicMetadata(t, dir, "EPIC-3", types.EpicStatusCompleted)
	writeEpicMetadata(t, dir, "EPIC-4", types.EpicStatusCompleted)

	got, err := LoadArchivedBugContext(dir)
	if err != nil {
		t.Fatalf("LoadArchivedBugContext: %v", err)
	}

	if len(got) != 4 {
		t.Fatalf("len(got) = %d, want 4", len(got))
	}

	assertArchivedBugContext(t, got[0], "bug-epic-1-open", "EPIC-1", types.EpicStatusPlanned, "update the existing `PLANNED` backlog work")
	assertArchivedBugContext(t, got[1], "bug-epic-2-open", "EPIC-2", types.EpicStatusActive, "do not reopen or mutate the `ACTIVE` backlog package")
	assertArchivedBugContext(t, got[2], "bug-epic-3-open", "EPIC-3", types.EpicStatusCompleted, "do not reopen the `COMPLETED` historical package")
	if got[3].BugID != "bug-epic-5-open" {
		t.Fatalf("got[3].BugID = %q, want bug-epic-5-open", got[3].BugID)
	}
	if got[3].EpicStatus != nil {
		t.Fatalf("got[3].EpicStatus = %v, want nil", *got[3].EpicStatus)
	}
	if !strings.Contains(got[3].PlanningAction, "new or updated `PLANNED` work") {
		t.Fatalf("unexpected planning action for missing metadata: %q", got[3].PlanningAction)
	}
	if got[3].Summary != "No metadata bug summary." {
		t.Fatalf("got[3].Summary = %q", got[3].Summary)
	}
}

func TestArchivedBugContextPlanningBullet(t *testing.T) {
	status := types.EpicStatusCompleted
	bug := ArchivedBugContext{
		BugID:          "bug-1",
		SourceEpicID:   "EPIC-42",
		SourcePath:     ".doug/logs/bugs/EPIC-42/bug-1.md",
		Status:         "open",
		Severity:       "non-blocking",
		Summary:        "Summary text.",
		EpicStatus:     &status,
		PlanningAction: "treat follow-up as new planning work; do not reopen the `COMPLETED` historical package",
	}

	got := bug.PlanningBullet()
	for _, want := range []string{
		"`bug-1` from epic `EPIC-42`",
		"status `open`",
		"severity `non-blocking`",
		"summary: Summary text.",
		"source epic lifecycle `COMPLETED`",
		"do not reopen the `COMPLETED` historical package",
		"archive: `.doug/logs/bugs/EPIC-42/bug-1.md`",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in planning bullet, got %q", want, got)
		}
	}
}

func assertArchivedBugContext(t *testing.T, got ArchivedBugContext, wantBugID, wantEpicID string, wantStatus types.EpicLifecycleStatus, wantPlanningSubstring string) {
	t.Helper()

	if got.BugID != wantBugID {
		t.Fatalf("BugID = %q, want %q", got.BugID, wantBugID)
	}
	if got.SourceEpicID != wantEpicID {
		t.Fatalf("SourceEpicID = %q, want %q", got.SourceEpicID, wantEpicID)
	}
	if got.EpicStatus == nil || *got.EpicStatus != wantStatus {
		t.Fatalf("EpicStatus = %v, want %q", got.EpicStatus, wantStatus)
	}
	if !strings.Contains(got.PlanningAction, wantPlanningSubstring) {
		t.Fatalf("PlanningAction = %q, want substring %q", got.PlanningAction, wantPlanningSubstring)
	}
	if got.SourcePath != filepath.ToSlash(filepath.Join(".doug", "logs", "bugs", wantEpicID, filepath.Base(got.SourcePath))) {
		t.Fatalf("SourcePath = %q", got.SourcePath)
	}
}

func writeArchivedBug(t *testing.T, root, epicID, fileName, content string) {
	t.Helper()
	testutil.WriteFile(t, filepath.Join(root, ".doug", "logs", "bugs", epicID, fileName), content)
}

func writeEpicMetadata(t *testing.T, root, epicID string, status types.EpicLifecycleStatus) {
	t.Helper()
	paths := NewEpicPackagePaths(root, epicID)
	if err := os.MkdirAll(paths.EpicDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", paths.EpicDir, err)
	}
	if err := SaveEpicMetadata(paths.MetadataPath, &types.EpicMetadata{
		EpicID:         epicID,
		Status:         status,
		CreatedAt:      "2026-04-01T00:00:00Z",
		SourcePlanPath: ".doug/plan/PLAN.md",
	}); err != nil {
		t.Fatalf("SaveEpicMetadata(%s): %v", epicID, err)
	}
}
