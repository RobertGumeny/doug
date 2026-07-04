package plan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/testutil"
)

func TestLoadResearchReports_LoadsTopLevelMarkdownDeterministically(t *testing.T) {
	dir := t.TempDir()

	writeResearchFile(t, dir, "zeta.md", "# Zeta\n\nBody is not inlined.\n")
	writeResearchFile(t, dir, "alpha.md", "# Alpha\n")
	writeResearchFile(t, dir, "README.md", "# Index\n")
	writeResearchFile(t, dir, "notes.txt", "not markdown\n")
	writeResearchFile(t, dir, filepath.Join("history", "old.md"), "# Archived\n")

	got, err := LoadResearchReports(dir, nil)
	if err != nil {
		t.Fatalf("LoadResearchReports: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2; got: %+v", len(got), got)
	}
	assertResearchReportContext(t, got[0], "alpha", filepath.ToSlash(filepath.Join(".doug", "intake", "research", "alpha.md")))
	assertResearchReportContext(t, got[1], "zeta", filepath.ToSlash(filepath.Join(".doug", "intake", "research", "zeta.md")))
}

func TestResearchReportContextPlanningBullet(t *testing.T) {
	report := ResearchReportContext{
		ReportID:   "research-to-plan-intake",
		SourcePath: ".doug/intake/research/research-to-plan-intake.md",
	}

	got := report.PlanningBullet()
	for _, want := range []string{
		"research report `research-to-plan-intake`",
		"source: `.doug/intake/research/research-to-plan-intake.md`",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in planning bullet, got %q", want, got)
		}
	}
}

func TestLoadResearchReports_MissingDirectoryReturnsEmpty(t *testing.T) {
	got, err := LoadResearchReports(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("LoadResearchReports returned unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0; got: %+v", len(got), got)
	}
}

func assertResearchReportContext(t *testing.T, got ResearchReportContext, wantReportID, wantSourcePath string) {
	t.Helper()

	if got.ReportID != wantReportID {
		t.Fatalf("ReportID = %q, want %q", got.ReportID, wantReportID)
	}
	if got.SourcePath != wantSourcePath {
		t.Fatalf("SourcePath = %q, want %q", got.SourcePath, wantSourcePath)
	}
	bullet := got.PlanningBullet()
	if !strings.Contains(bullet, "`"+wantReportID+"`") || !strings.Contains(bullet, "`"+wantSourcePath+"`") {
		t.Fatalf("PlanningBullet() = %q; want report ID and source path", bullet)
	}
}

func TestLoadResearchReports_ReadsLegacyLogsResearchForCompatibility(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "logs", "research", "legacy-report.md"), "# Legacy\n")

	got, err := LoadResearchReports(dir, nil)
	if err != nil {
		t.Fatalf("LoadResearchReports: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1; got: %+v", len(got), got)
	}
	assertResearchReportContext(t, got[0], "legacy-report", filepath.ToSlash(filepath.Join(".doug", "logs", "research", "legacy-report.md")))
}

func writeResearchFile(t *testing.T, root, name, content string) {
	t.Helper()
	testutil.WriteFile(t, filepath.Join(root, ".doug", "intake", "research", name), content)
}
