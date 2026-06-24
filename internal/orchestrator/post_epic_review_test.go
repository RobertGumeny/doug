package orchestrator

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

func writeReviewSession(t *testing.T, logsDir, epicID, taskID string, attempt int, outcome types.Outcome, changelog string) {
	t.Helper()
	path := filepath.Join(logsDir, "sessions", epicID, fmt.Sprintf("session-%s_attempt-%d.md", taskID, attempt))
	content := fmt.Sprintf(`# Task Brief

---
outcome: %q
changelog_entry: %q
dependencies_added: []
bugs: []
---
`, outcome, changelog)
	testutil.WriteFile(t, path, content)
}

func TestAssemblePostEpicReviewInput_CompleteTaskData(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, ".doug", "logs")
	epicID := "EPIC-50"
	writeReviewSession(t, logsDir, epicID, "EPIC-50-001", 2, types.OutcomeSuccess, "Added review input assembly")

	tasks := []types.Task{{
		ID:          "EPIC-50-001",
		Type:        types.TaskTypeFeature,
		Description: "Build deterministic review input.",
		AcceptanceCriteria: []string{
			"Pair each task with metadata.",
			"Include committed diffs.",
		},
	}}
	metrics := []types.TaskMetric{{
		TaskID:    "EPIC-50-001",
		Outcome:   string(types.OutcomeSuccess),
		CommitSHA: "abc123",
		Attempts:  2,
	}}

	input := assemblePostEpicReviewInput(dir, logsDir, epicID, tasks, metrics, func(sha, projectRoot string) (string, error) {
		if sha != "abc123" || projectRoot != dir {
			t.Fatalf("diff lookup got sha=%q projectRoot=%q", sha, projectRoot)
		}
		return "diff --git a/review.go b/review.go\n+assembled\n", nil
	})
	brief := renderPostEpicReviewBrief(input)

	for _, want := range []string{
		"- acceptance-criteria faithfulness",
		"- likely regressions",
		"- implementation coherence",
		"- release-readiness",
		"### EPIC-50-001",
		"**Description:** Build deterministic review input.",
		"- Pair each task with metadata.",
		"- Include committed diffs.",
		"**Outcome:** SUCCESS",
		"**Changelog Entry:** Added review input assembly",
		"**Recorded Commit SHA:** abc123",
		"diff --git a/review.go b/review.go",
		"+assembled",
	} {
		if !strings.Contains(brief, want) {
			t.Fatalf("expected brief to contain %q, got:\n%s", want, brief)
		}
	}
	if strings.Contains(brief, "Assembly Warnings") {
		t.Fatalf("did not expect assembly warnings, got:\n%s", brief)
	}
}

func TestAssemblePostEpicReviewInput_MissingCommitSHAUsesWarningPlaceholder(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, ".doug", "logs")
	epicID := "EPIC-50"
	writeReviewSession(t, logsDir, epicID, "EPIC-50-002", 1, types.OutcomeSuccess, "Recorded task without commit SHA")

	diffCalled := false
	input := assemblePostEpicReviewInput(dir, logsDir, epicID, []types.Task{{
		ID:                 "EPIC-50-002",
		Type:               types.TaskTypeFeature,
		Description:        "Handle missing SHA.",
		AcceptanceCriteria: []string{"Warn instead of failing."},
	}}, []types.TaskMetric{{
		TaskID:   "EPIC-50-002",
		Outcome:  string(types.OutcomeSuccess),
		Attempts: 1,
	}}, func(string, string) (string, error) {
		diffCalled = true
		return "", nil
	})
	brief := renderPostEpicReviewBrief(input)

	if diffCalled {
		t.Fatal("diff lookup should not be called when commit SHA is missing")
	}
	for _, want := range []string{
		"EPIC-50-002: missing commit SHA",
		"**Recorded Commit SHA:** ⚠️ missing commit SHA",
		"⚠️ diff unavailable: missing commit SHA",
		"missing commit SHA; committed diff unavailable",
	} {
		if !strings.Contains(brief, want) {
			t.Fatalf("expected brief to contain %q, got:\n%s", want, brief)
		}
	}
}

func TestAssemblePostEpicReviewInput_FailedDiffLookupUsesWarningPlaceholder(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, ".doug", "logs")
	epicID := "EPIC-50"
	writeReviewSession(t, logsDir, epicID, "EPIC-50-003", 1, types.OutcomeSuccess, "Recorded task with unreachable diff")

	input := assemblePostEpicReviewInput(dir, logsDir, epicID, []types.Task{{
		ID:                 "EPIC-50-003",
		Type:               types.TaskTypeFeature,
		Description:        "Handle failed diff lookup.",
		AcceptanceCriteria: []string{"Warn instead of failing."},
	}}, []types.TaskMetric{{
		TaskID:    "EPIC-50-003",
		Outcome:   string(types.OutcomeSuccess),
		CommitSHA: "deadbeef",
		Attempts:  1,
	}}, func(string, string) (string, error) {
		return "", fmt.Errorf("object not found")
	})
	brief := renderPostEpicReviewBrief(input)

	for _, want := range []string{
		"EPIC-50-003: committed diff lookup failed for deadbeef",
		"**Recorded Commit SHA:** deadbeef",
		"⚠️ diff unavailable for deadbeef: object not found",
		"committed diff lookup failed for deadbeef: object not found",
	} {
		if !strings.Contains(brief, want) {
			t.Fatalf("expected brief to contain %q, got:\n%s", want, brief)
		}
	}
}

func TestAssemblePostEpicReviewBrief_UsesCommittedDiffHelperWithoutBackend(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	logsDir := filepath.Join(dir, ".doug", "logs")
	epicID := "EPIC-50"

	testutil.WriteFile(t, filepath.Join(dir, "feature.txt"), "done\n")
	runGitForReview(t, dir, "add", "feature.txt")
	runGitForReview(t, dir, "commit", "-m", "feat: EPIC-50-004")
	sha := strings.TrimSpace(runGitOutputForReview(t, dir, "rev-parse", "HEAD"))

	writeReviewSession(t, logsDir, epicID, "EPIC-50-004", 1, types.OutcomeSuccess, "Added committed diff evidence")
	brief := assemblePostEpicReviewBrief(dir, logsDir, epicID, []types.Task{{
		ID:                 "EPIC-50-004",
		Type:               types.TaskTypeFeature,
		Description:        "Use committed git diff.",
		AcceptanceCriteria: []string{"Include the committed patch."},
	}}, []types.TaskMetric{{
		TaskID:    "EPIC-50-004",
		Outcome:   string(types.OutcomeSuccess),
		CommitSHA: sha,
		Attempts:  1,
	}})

	for _, want := range []string{"diff --git a/feature.txt b/feature.txt", "+done", "Added committed diff evidence"} {
		if !strings.Contains(brief, want) {
			t.Fatalf("expected brief to contain %q, got:\n%s", want, brief)
		}
	}
}

func runGitForReview(t *testing.T, dir string, args ...string) {
	t.Helper()
	_ = runGitOutputForReview(t, dir, args...)
}

func runGitOutputForReview(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
