package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

func writeReviewSession(t *testing.T, logsDir, epicID, taskID string, attempt int, outcome types.Outcome, changelog string) {
	t.Helper()
	path := filepath.Join(logsDir, "epics", epicID, taskID, fmt.Sprintf("attempt-%d", attempt), "session.md")
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

func TestRunPostEpicReview_DisabledSkipsBackend(t *testing.T) {
	dir := t.TempDir()
	paths := NewPaths(dir)
	called := false
	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			ReviewEnabled:         false,
			BuildSystem:           "go",
			AgentHeartbeatSeconds: 0,
		},
		paths:  paths,
		logger: &recordingLogger{},
		backend: backendFunc(func(context.Context, agent.RunRequest) (agent.RunResponse, error) {
			called = true
			return agent.RunResponse{}, nil
		}),
	}

	if err := o.runPostEpicReview(context.Background(), postEpicReviewState(), postEpicReviewTasks()); err != nil {
		t.Fatalf("runPostEpicReview disabled: %v", err)
	}
	if called {
		t.Fatal("expected backend not to be called when review is disabled")
	}
	if _, err := os.Stat(filepath.Join(paths.LogsDir, "epics", "EPIC-50")); !os.IsNotExist(err) {
		t.Fatalf("expected no review directory on skipped review, stat err=%v", err)
	}
}

func TestRunPostEpicReview_WritesSkeletonBriefAndInvokesContract(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)
	logger := &recordingLogger{}
	writeReviewSession(t, paths.LogsDir, "EPIC-50", "EPIC-50-001", 1, types.OutcomeSuccess, "Implemented reviewed feature")

	var backendCalled bool
	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		backendCalled = true
		if req.Phase != agent.RunPhasePostEpicReview {
			return agent.RunResponse{}, fmt.Errorf("phase = %q", req.Phase)
		}
		if req.Task.ID != postEpicReviewTaskID || req.Task.Attempt != 1 || req.Task.MaxRetries != 1 {
			return agent.RunResponse{}, fmt.Errorf("unexpected task context: %+v", req.Task)
		}
		if req.Routing.Workflow != "post_epic_review" || req.Routing.SkillName != "implement-documentation" || req.Routing.InteractionMode != "rpc" {
			return agent.RunResponse{}, fmt.Errorf("unexpected routing: %+v", req.Routing)
		}
		reviewRoot := filepath.Join(paths.LogsDir, "epics", "EPIC-50")
		if !hasPath(req.Restrictions.Write.Paths, reviewRoot) || !hasPath(req.Restrictions.Write.Paths, filepath.Join(paths.DougDir, "ACTIVE_TASK.md")) {
			return agent.RunResponse{}, fmt.Errorf("missing review write restrictions: %+v", req.Restrictions.Write.Paths)
		}
		if !hasArtifact(req.Artifacts.Write, reviewRoot, agent.ArtifactPurposeReviewArtifact) {
			return agent.RunResponse{}, fmt.Errorf("missing review artifact write surface: %+v", req.Artifacts.Write)
		}

		reviewPath := filepath.Join(reviewRoot, "epic-review.md")
		skeleton, err := os.ReadFile(reviewPath)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("read skeleton: %w", err)
		}
		for _, heading := range []string{"# Epic Review", "## Faithfulness To Acceptance Criteria", "## Likely Regressions", "## Implementation Coherence", "## Release Readiness", "## Evidence Reviewed"} {
			if !strings.Contains(string(skeleton), heading) {
				return agent.RunResponse{}, fmt.Errorf("skeleton missing %q", heading)
			}
		}
		if err := os.WriteFile(reviewPath, []byte(string(skeleton)+"\nReviewed.\n"), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("fill review artifact: %w", err)
		}

		activeTaskPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
		activeTask, err := os.ReadFile(activeTaskPath)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("read ACTIVE_TASK.md: %w", err)
		}
		for _, want := range []string{"POST_EPIC_REVIEW", "Structured Review Input", "Implemented reviewed feature", "committed diff lookup failed", "Review artifact:"} {
			if !strings.Contains(string(activeTask), want) {
				return agent.RunResponse{}, fmt.Errorf("active task missing %q", want)
			}
		}
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			ReviewEnabled:         true,
			BuildSystem:           "go",
			AgentHeartbeatSeconds: 0,
		},
		paths:   paths,
		logger:  logger,
		backend: stub,
	}

	if err := o.runPostEpicReview(context.Background(), postEpicReviewState(), postEpicReviewTasks()); err != nil {
		t.Fatalf("runPostEpicReview: %v", err)
	}
	if !backendCalled {
		t.Fatal("expected backend to be called")
	}
	if _, err := os.Stat(filepath.Join(paths.LogsDir, "epics", "EPIC-50", "POST_EPIC_REVIEW", "attempt-1", "session.md")); err != nil {
		t.Fatalf("expected archived post-epic review session: %v", err)
	}
	if !loggerContains(logger.successes, filepath.Join(paths.LogsDir, "epics", "EPIC-50", "epic-review.md")) {
		t.Fatalf("expected success log to print artifact path, got %+v", logger.successes)
	}
	if loggerContains(logger.warnings, "result was not parseable") {
		t.Fatalf("filled review artifact with empty outcome should not warn about parseability, got %+v", logger.warnings)
	}
}

func TestRunPostEpicReview_ArtifactsAreVersioned(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)
	writeReviewSession(t, paths.LogsDir, "EPIC-50", "EPIC-50-001", 1, types.OutcomeSuccess, "Implemented reviewed feature")

	stub := backendFunc(func(_ context.Context, _ agent.RunRequest) (agent.RunResponse, error) {
		activeTaskPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
		data, err := os.ReadFile(activeTaskPath)
		if err != nil {
			return agent.RunResponse{}, err
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "SUCCESS"`, 1)
		if err := os.WriteFile(activeTaskPath, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, err
		}
		return agent.RunResponse{Status: agent.RunStatusCompleted}, nil
	})
	o := &Orchestrator{
		cfg:     &config.OrchestratorConfig{ReviewEnabled: true, BuildSystem: "go"},
		paths:   paths,
		logger:  &recordingLogger{},
		backend: stub,
	}

	if err := o.runPostEpicReview(context.Background(), postEpicReviewState(), postEpicReviewTasks()); err != nil {
		t.Fatalf("first review: %v", err)
	}
	if err := o.runPostEpicReview(context.Background(), postEpicReviewState(), postEpicReviewTasks()); err != nil {
		t.Fatalf("second review: %v", err)
	}
	for _, name := range []string{"epic-review.md", "epic-review-v2.md"} {
		if _, err := os.Stat(filepath.Join(paths.LogsDir, "epics", "EPIC-50", name)); err != nil {
			t.Fatalf("expected versioned review artifact %s: %v", name, err)
		}
	}
}

func TestReviewCompletedEpic_RequiresCompletedRuntimeArchive(t *testing.T) {
	dir := t.TempDir()
	paths := NewPaths(dir)
	o := &Orchestrator{cfg: &config.OrchestratorConfig{BuildSystem: "go"}, paths: paths, logger: &recordingLogger{}}

	_, err := o.ReviewCompletedEpic(context.Background(), "EPIC-MISSING")
	if err == nil {
		t.Fatal("expected missing archive error")
	}
	if !strings.Contains(err.Error(), "completed runtime archive") || !strings.Contains(err.Error(), "run the epic to completion") {
		t.Fatalf("expected actionable missing archive error, got %v", err)
	}
}

func TestReviewCompletedEpic_RejectsIncompleteArchive(t *testing.T) {
	dir := t.TempDir()
	paths := NewPaths(dir)
	writeCompletedReviewArchive(t, paths, postEpicReviewState(), postEpicReviewTasks())
	writeReviewSession(t, paths.LogsDir, "EPIC-50", "EPIC-50-001", 1, types.OutcomeSuccess, "Implemented reviewed feature")
	archivedState, err := state.LoadProjectState(filepath.Join(paths.LogsDir, "epics", "EPIC-50", "project-state.yaml"))
	if err != nil {
		t.Fatalf("load archive state: %v", err)
	}
	archivedState.CurrentEpic.CompletedAt = nil
	if err := state.SaveProjectState(filepath.Join(paths.LogsDir, "epics", "EPIC-50", "project-state.yaml"), archivedState); err != nil {
		t.Fatalf("save incomplete archive state: %v", err)
	}

	o := &Orchestrator{cfg: &config.OrchestratorConfig{BuildSystem: "go"}, paths: paths, logger: &recordingLogger{}}
	_, err = o.ReviewCompletedEpic(context.Background(), "EPIC-50")
	if err == nil || !strings.Contains(err.Error(), "not completed") {
		t.Fatalf("expected incomplete archive rejection, got %v", err)
	}
}

func TestReviewCompletedEpic_IgnoresDisabledConfigAndUsesSharedRunner(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)
	writeCompletedReviewArchive(t, paths, postEpicReviewState(), postEpicReviewTasks())
	writeReviewSession(t, paths.LogsDir, "EPIC-50", "EPIC-50-001", 1, types.OutcomeSuccess, "Implemented reviewed feature")

	backendCalled := false
	stub := backendFunc(func(_ context.Context, _ agent.RunRequest) (agent.RunResponse, error) {
		backendCalled = true
		activeTaskPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
		data, err := os.ReadFile(activeTaskPath)
		if err != nil {
			return agent.RunResponse{}, err
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "SUCCESS"`, 1)
		if err := os.WriteFile(activeTaskPath, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, err
		}
		return agent.RunResponse{Status: agent.RunStatusCompleted}, nil
	})
	o := &Orchestrator{
		cfg:     &config.OrchestratorConfig{ReviewEnabled: false, BuildSystem: "go"},
		paths:   paths,
		logger:  &recordingLogger{},
		backend: stub,
	}

	artifactPath, err := o.ReviewCompletedEpic(context.Background(), "EPIC-50")
	if err != nil {
		t.Fatalf("ReviewCompletedEpic: %v", err)
	}
	if !backendCalled {
		t.Fatal("expected explicit review to invoke backend despite review_enabled=false")
	}
	if !strings.HasSuffix(artifactPath, filepath.Join("epics", "EPIC-50", "epic-review.md")) {
		t.Fatalf("unexpected artifact path %q", artifactPath)
	}
}

func TestRunPostEpicReview_WarningOnlyOnAgentErrorAndMissingOutcome(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)
	logger := &recordingLogger{}
	stubErr := errors.New("provider unavailable")
	stub := backendFunc(func(_ context.Context, _ agent.RunRequest) (agent.RunResponse, error) {
		return agent.RunResponse{Status: agent.RunStatusTransportFailure}, stubErr
	})
	o := &Orchestrator{
		cfg:     &config.OrchestratorConfig{ReviewEnabled: true, BuildSystem: "go"},
		paths:   paths,
		logger:  logger,
		backend: stub,
	}

	if err := o.runPostEpicReview(context.Background(), postEpicReviewState(), postEpicReviewTasks()); err != nil {
		t.Fatalf("expected warning-only review failure, got error: %v", err)
	}
	if !loggerContains(logger.warnings, "advisory post-epic review did not complete") {
		t.Fatalf("expected incomplete review warning, got %+v", logger.warnings)
	}
	if !loggerContains(logger.warnings, "inspect the completed epic more carefully") {
		t.Fatalf("expected inspect warning, got %+v", logger.warnings)
	}
	if !loggerContains(logger.warnings, "doug review EPIC-50") {
		t.Fatalf("expected retry command warning, got %+v", logger.warnings)
	}
}

func TestRunPostEpicReview_PristineArtifactWithEmptyOutcomeWarnsIncomplete(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)
	logger := &recordingLogger{}
	stub := backendFunc(func(_ context.Context, _ agent.RunRequest) (agent.RunResponse, error) {
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})
	o := &Orchestrator{
		cfg:     &config.OrchestratorConfig{ReviewEnabled: true, BuildSystem: "go"},
		paths:   paths,
		logger:  logger,
		backend: stub,
	}

	if err := o.runPostEpicReview(context.Background(), postEpicReviewState(), postEpicReviewTasks()); err != nil {
		t.Fatalf("expected warning-only pristine review failure, got error: %v", err)
	}
	if !loggerContains(logger.warnings, "advisory post-epic review did not complete") || !loggerContains(logger.warnings, "result was not parseable") {
		t.Fatalf("expected incomplete pristine-review warning, got %+v", logger.warnings)
	}
}

func writeCompletedReviewArchive(t *testing.T, paths Paths, projectState *types.ProjectState, tasks *types.Tasks) {
	t.Helper()
	completedAt := "2026-06-24T00:00:00Z"
	projectState.CurrentEpic.CompletedAt = &completedAt
	archiveDir := filepath.Join(paths.LogsDir, "epics", projectState.CurrentEpic.ID)
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatalf("create archive dir: %v", err)
	}
	if err := state.SaveProjectState(filepath.Join(archiveDir, "project-state.yaml"), projectState); err != nil {
		t.Fatalf("save archived project state: %v", err)
	}
	if err := state.SaveTasks(filepath.Join(archiveDir, "tasks.yaml"), tasks); err != nil {
		t.Fatalf("save archived tasks: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(paths.LogsDir, "epics", projectState.CurrentEpic.ID, "EPIC-50-001", "attempt-1"), 0o755); err != nil {
		t.Fatalf("create sessions archive: %v", err)
	}
}

func postEpicReviewState() *types.ProjectState {
	return &types.ProjectState{
		CurrentEpic: types.EpicState{ID: "EPIC-50", Name: "Review", BranchName: "feature/EPIC-50"},
		Metrics: types.Metrics{Tasks: []types.TaskMetric{{
			TaskID:    "EPIC-50-001",
			Outcome:   string(types.OutcomeSuccess),
			CommitSHA: "abc123",
			Attempts:  1,
		}}},
	}
}

func postEpicReviewTasks() *types.Tasks {
	return &types.Tasks{Epic: types.EpicDefinition{ID: "EPIC-50", Name: "Review", Tasks: []types.Task{{
		ID:                 "EPIC-50-001",
		Type:               types.TaskTypeFeature,
		Description:        "Implement reviewed feature.",
		AcceptanceCriteria: []string{"Review runner works."},
	}}}}
}

func loggerContains(messages []string, want string) bool {
	for _, message := range messages {
		if strings.Contains(message, want) {
			return true
		}
	}
	return false
}
