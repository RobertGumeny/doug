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

	got, err := LoadArchivedBugContext(dir, nil)
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

func TestLoadArchivedBugContext_SkipsMalformedFilesWithWarning(t *testing.T) {
	dir := t.TempDir()

	// Malformed file: missing YAML frontmatter entirely.
	writeArchivedBug(t, dir, "EPIC-M", "bug-malformed.md", "no frontmatter here\n\nJust prose.\n")

	// Valid open file in a different epic sub-directory.
	writeArchivedBug(t, dir, "EPIC-V", "bug-valid-open.md", ""+
		"---\n"+
		"bug_id: \"bug-valid-open\"\n"+
		"status: \"open\"\n"+
		"severity: \"non-blocking\"\n"+
		"---\n\n"+
		"## Summary\n\nValid open bug.\n")

	var warnings []string
	warnFn := func(msg string) { warnings = append(warnings, msg) }

	got, err := LoadArchivedBugContext(dir, warnFn)
	if err != nil {
		t.Fatalf("LoadArchivedBugContext returned unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].BugID != "bug-valid-open" {
		t.Fatalf("got[0].BugID = %q, want %q", got[0].BugID, "bug-valid-open")
	}
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want 1; warnings: %v", len(warnings), warnings)
	}
	malformedPath := filepath.Join(dir, ".doug", "logs", "bugs", "EPIC-M", "bug-malformed.md")
	if !strings.Contains(warnings[0], malformedPath) {
		t.Fatalf("warning does not name malformed path %q; got: %q", malformedPath, warnings[0])
	}
}

func TestLoadArchivedBugContext_FiltersTerminalStatuses(t *testing.T) {
	dir := t.TempDir()

	terminalStatuses := []struct {
		status   string
		fileName string
		bugID    string
	}{
		{"resolved", "bug-resolved.md", "bug-resolved"},
		{"done", "bug-done.md", "bug-done"},
		{"closed", "bug-closed.md", "bug-closed"},
		{"fixed", "bug-fixed.md", "bug-fixed"},
	}

	for _, tc := range terminalStatuses {
		writeArchivedBug(t, dir, "EPIC-T", tc.fileName, ""+
			"---\n"+
			"bug_id: \""+tc.bugID+"\"\n"+
			"status: \""+tc.status+"\"\n"+
			"severity: \"non-blocking\"\n"+
			"---\n\n"+
			"## Summary\n\nShould be filtered out.\n")
	}

	// One open bug that should pass through.
	writeArchivedBug(t, dir, "EPIC-T", "bug-open.md", ""+
		"---\n"+
		"bug_id: \"bug-open\"\n"+
		"status: \"open\"\n"+
		"severity: \"blocking\"\n"+
		"---\n\n"+
		"## Summary\n\nOpen bug that should surface.\n")

	var warnings []string
	got, err := LoadArchivedBugContext(dir, func(msg string) { warnings = append(warnings, msg) })
	if err != nil {
		t.Fatalf("LoadArchivedBugContext: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings for well-formed terminal-status files: %v", warnings)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (only the open bug); got: %+v", len(got), got)
	}
	if got[0].BugID != "bug-open" {
		t.Fatalf("got[0].BugID = %q, want %q", got[0].BugID, "bug-open")
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
