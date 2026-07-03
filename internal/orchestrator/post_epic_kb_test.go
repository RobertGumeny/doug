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
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/log"
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

func setupPostEpicKBRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test Agent")

	testutil.WriteFile(t, filepath.Join(dir, ".gitignore"), ".doug/\n")
	testutil.WriteFile(t, filepath.Join(dir, "README.md"), "# test repo\n")
	testutil.WriteFile(t, filepath.Join(dir, "CHANGELOG.md"), "# Changelog\n\n## [Unreleased]\n\n### Changed\n\n## [0.1.0]\n\n### Changed\n- Released note\n")
	testutil.WriteFile(t, filepath.Join(dir, "docs", "kb", "README.md"), "# KB Index\n")

	runGit("add", ".")
	runGit("commit", "-m", "initial")

	return dir
}

func postEpicState() *types.ProjectState {
	return &types.ProjectState{
		CurrentEpic: types.EpicState{
			ID:         "EPIC-20",
			Name:       "KB",
			BranchName: "feature/EPIC-20",
			StartedAt:  "2026-04-20T00:00:00Z",
		},
	}
}

func hasPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

func hasArtifact(artifacts []agent.ArtifactSurface, wantPath string, wantPurpose agent.ArtifactPurpose) bool {
	for _, artifact := range artifacts {
		if artifact.Path == wantPath && artifact.Purpose == wantPurpose {
			return true
		}
	}
	return false
}

func TestRunPostEpicKB_WritesConstrainedDocumentationBriefing(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)

	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		taskPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
		data, err := os.ReadFile(taskPath)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "SUCCESS"`, 1)
		if err := os.WriteFile(taskPath, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			KBEnabled:             true,
			BuildSystem:           "go",
			AgentHeartbeatSeconds: 0,
		},
		paths:   paths,
		logger:  log.Discard(),
		backend: stub,
	}

	if err := o.runPostEpicKB(context.Background(), postEpicState()); err != nil {
		t.Fatalf("runPostEpicKB: %v", err)
	}

	sessionPath := filepath.Join(paths.LogsDir, "epics", "EPIC-20", "POST_EPIC_KB", "attempt-1", "session.md")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read archived post-epic KB session: %v", err)
	}
	content := string(data)

	for _, want := range []string{
		"**Task Type**: documentation",
		"Use the documentation workflow",
		"`docs/kb/README.md`",
		".doug/plan/PLAN.md",
		"planning rationale, scope decisions, and non-goals",
		"Write KB output only under `docs/kb/` and changelog polish only in `CHANGELOG.md`.",
		"edit only `[Unreleased]`",
		"preserve all factual meaning",
		"invent nothing",
		"never touch released sections",
		"Do not reopen or modify epic runtime state",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected archived session to contain %q, got:\n%s", want, content)
		}
	}
}

// TestRunPostEpicKB_UsesInjectedBackend verifies that runPostEpicKB routes
// agent invocation through the Orchestrator's backend seam rather than calling
// the backend selection path directly. If the seam is bypassed this test fails because the stub
// never receives control and the ACTIVE_TASK.md outcome is never written.
func TestRunPostEpicKB_UsesInjectedBackend(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)

	var backendCalled bool
	stub := backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		backendCalled = true
		taskPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
		kbRoot := filepath.Join(paths.ProjectRoot, "docs", "kb")
		if req.Phase != agent.RunPhasePostEpicKB {
			return agent.RunResponse{}, fmt.Errorf("phase = %q, want %q", req.Phase, agent.RunPhasePostEpicKB)
		}
		if req.Task.ID != postEpicKBTaskID || req.Task.Type != string(types.TaskTypeDocumentation) {
			return agent.RunResponse{}, fmt.Errorf("unexpected task context: %+v", req.Task)
		}
		if req.Task.Attempt != 1 || req.Task.MaxRetries != 1 {
			return agent.RunResponse{}, fmt.Errorf("unexpected task attempt context: %+v", req.Task)
		}
		if req.Brief.Path != taskPath || req.Brief.Format != agent.BriefFormatMarkdown || req.Brief.Authority != agent.ArtifactAuthorityDoug {
			return agent.RunResponse{}, fmt.Errorf("unexpected brief: %+v", req.Brief)
		}
		if !hasContextInput(req.ContextLoadOrder, agent.ContextInput{
			Kind:      agent.ContextInputCanonicalBrief,
			Path:      taskPath,
			Required:  true,
			Authority: agent.ArtifactAuthorityDoug,
		}) {
			return agent.RunResponse{}, fmt.Errorf("missing canonical brief context entry in %+v", req.ContextLoadOrder)
		}
		planPath := filepath.Join(paths.DougDir, "plan", "PLAN.md")
		if !hasContextInput(req.ContextLoadOrder, agent.ContextInput{
			Kind:      agent.ContextInputWorkingArtifact,
			Path:      planPath,
			Required:  false,
			Authority: agent.ArtifactAuthorityDoug,
		}) {
			return agent.RunResponse{}, fmt.Errorf("missing PLAN.md context entry in %+v", req.ContextLoadOrder)
		}
		if req.Routing.Workflow != "post_epic_kb" || req.Routing.SkillName != "implement-documentation" {
			return agent.RunResponse{}, fmt.Errorf("unexpected routing: %+v", req.Routing)
		}
		if req.Restrictions.Read.Mode != agent.RestrictionModeInherit || req.Restrictions.Write.Mode != agent.RestrictionModeAllowList {
			return agent.RunResponse{}, fmt.Errorf("unexpected restrictions: %+v", req.Restrictions)
		}
		changelogPath := filepath.Join(paths.ProjectRoot, "CHANGELOG.md")
		if !hasPath(req.Restrictions.Write.Paths, kbRoot) || !hasPath(req.Restrictions.Write.Paths, changelogPath) || !hasPath(req.Restrictions.Write.Paths, taskPath) {
			return agent.RunResponse{}, fmt.Errorf("expected kb root, changelog, and task path in write restriction paths, got %+v", req.Restrictions.Write.Paths)
		}
		if !hasPath(req.Restrictions.Read.Paths, planPath) {
			return agent.RunResponse{}, fmt.Errorf("expected PLAN.md in read restriction paths, got %+v", req.Restrictions.Read.Paths)
		}
		if !hasArtifact(req.Artifacts.Read, planPath, agent.ArtifactPurposeWorkingArtifact) {
			return agent.RunResponse{}, fmt.Errorf("missing PLAN.md read artifact in %+v", req.Artifacts.Read)
		}
		if !hasArtifact(req.Artifacts.Read, kbRoot, agent.ArtifactPurposeKnowledgeBase) {
			return agent.RunResponse{}, fmt.Errorf("missing kb read artifact in %+v", req.Artifacts.Read)
		}
		if !hasArtifact(req.Artifacts.Read, changelogPath, agent.ArtifactPurposeChangelog) {
			return agent.RunResponse{}, fmt.Errorf("missing changelog read artifact in %+v", req.Artifacts.Read)
		}
		if !hasArtifact(req.Artifacts.Write, kbRoot, agent.ArtifactPurposeKnowledgeBase) {
			return agent.RunResponse{}, fmt.Errorf("missing kb write artifact in %+v", req.Artifacts.Write)
		}
		if !hasArtifact(req.Artifacts.Write, changelogPath, agent.ArtifactPurposeChangelog) {
			return agent.RunResponse{}, fmt.Errorf("missing changelog write artifact in %+v", req.Artifacts.Write)
		}
		data, err := os.ReadFile(taskPath)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "SUCCESS"`, 1)
		if err := os.WriteFile(taskPath, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}
		code := 0
		return agent.RunResponse{
			Status:              agent.RunStatusCompleted,
			Duration:            time.Millisecond,
			ExitCode:            &code,
			SessionID:           "pi-session-123",
			AvailableSessionIDs: []string{"pi-session-123", "pi-session-456"},
		}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			KBEnabled:             true,
			BuildSystem:           "go",
			AgentHeartbeatSeconds: 0,
		},
		paths:   paths,
		logger:  log.Discard(),
		backend: stub,
	}

	if err := o.runPostEpicKB(context.Background(), postEpicState()); err != nil {
		t.Fatalf("runPostEpicKB: %v", err)
	}
	if !backendCalled {
		t.Fatal("expected injected backend to be called, but it was not — seam may be bypassed")
	}
}

// lastCommitFiles returns the file paths touched by HEAD in dir.
func lastCommitFiles(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "show", "--name-only", "--format=", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git show: %v", err)
	}
	return string(out)
}

// TestRunPostEpicKB_MissingOutcomeWithKBChangesSoftSucceeds verifies that a
// missing outcome (e.g. a provider transport issue) is tolerated as a
// best-effort success when in-scope docs/kb/ files changed, committing only the
// docs/kb/ paths.
func TestRunPostEpicKB_MissingOutcomeWithKBChangesSoftSucceeds(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)
	logger := &recordingLogger{}

	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		// Leave ACTIVE_TASK.md outcome empty, but write an in-scope KB edit.
		if err := os.WriteFile(filepath.Join(dir, "docs", "kb", "new-article.md"), []byte("# Synthesized KB article\n"), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write kb article: %w", err)
		}
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			KBEnabled:             true,
			BuildSystem:           "go",
			AgentHeartbeatSeconds: 0,
		},
		paths:   paths,
		logger:  logger,
		backend: stub,
	}

	if err := o.runPostEpicKB(context.Background(), postEpicState()); err != nil {
		t.Fatalf("runPostEpicKB soft-success: %v", err)
	}

	files := lastCommitFiles(t, dir)
	if !strings.Contains(files, "docs/kb/new-article.md") {
		t.Fatalf("expected docs/kb/new-article.md committed, got: %s", files)
	}
	if loggerContains(logger.warnings, "outcome") || loggerContains(logger.warnings, "incomplete") {
		t.Fatalf("missing outcome with in-scope KB output should not produce a false warning, got %+v", logger.warnings)
	}
}

// TestRunPostEpicKB_MissingOutcomeWithoutKBChangesReturnsParseError verifies
// that a missing outcome with no in-scope docs/kb/ changes is still a hard
// parse error rather than a soft success.
func TestRunPostEpicKB_MissingOutcomeWithoutKBChangesReturnsParseError(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)

	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		// Leave the outcome empty and make no docs/kb/ changes.
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			KBEnabled:             true,
			BuildSystem:           "go",
			AgentHeartbeatSeconds: 0,
		},
		paths:   paths,
		logger:  log.Discard(),
		backend: stub,
	}

	err := o.runPostEpicKB(context.Background(), postEpicState())
	if err == nil {
		t.Fatal("expected parse error for missing outcome without KB changes, got nil")
	}
	if !errors.Is(err, agent.ErrMissingOutcome) {
		t.Fatalf("expected ErrMissingOutcome, got: %v", err)
	}
}

func TestRunPostEpicKB_RejectsChangesOutsideAllowedOutputs(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)

	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		taskPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
		data, err := os.ReadFile(taskPath)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "SUCCESS"`, 1)
		if err := os.WriteFile(taskPath, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}
		// Simulate agent writing inside and outside docs/kb/.
		if err := os.WriteFile(filepath.Join(dir, "docs", "kb", "new-article.md"), []byte("# Allowed KB article\n"), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write kb article: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "rogue-kb-note.md"), []byte("outside docs/kb\n"), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write rogue file: %w", err)
		}
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})

	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			KBEnabled:             true,
			BuildSystem:           "go",
			AgentHeartbeatSeconds: 0,
		},
		paths:   paths,
		logger:  log.Discard(),
		backend: stub,
	}

	err := o.runPostEpicKB(context.Background(), postEpicState())
	if err == nil {
		t.Fatal("expected post-epic KB path validation error, got nil")
	}
	if !strings.Contains(err.Error(), `post-epic KB produced changes outside docs/kb/ or CHANGELOG.md: "rogue-kb-note.md"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPostEpicKB_ChangelogOnlyChangesCommit(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)

	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		taskPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
		data, err := os.ReadFile(taskPath)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "SUCCESS"`, 1)
		if err := os.WriteFile(taskPath, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}
		changelog := filepath.Join(dir, "CHANGELOG.md")
		content, err := os.ReadFile(changelog)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read changelog: %w", err)
		}
		polished := strings.Replace(string(content), "### Changed\n", "### Changed\n- Polished unreleased note\n", 1)
		if err := os.WriteFile(changelog, []byte(polished), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write changelog: %w", err)
		}
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})

	o := &Orchestrator{cfg: &config.OrchestratorConfig{KBEnabled: true, BuildSystem: "go"}, paths: paths, logger: log.Discard(), backend: stub}

	if err := o.runPostEpicKB(context.Background(), postEpicState()); err != nil {
		t.Fatalf("runPostEpicKB: %v", err)
	}
	files := lastCommitFiles(t, dir)
	if strings.TrimSpace(files) != "CHANGELOG.md" {
		t.Fatalf("expected only CHANGELOG.md committed, got: %q", files)
	}
}

func TestRunPostEpicKB_MissingOutcomeWithChangelogChangesSoftSucceeds(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)

	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		changelog := filepath.Join(dir, "CHANGELOG.md")
		content, err := os.ReadFile(changelog)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read changelog: %w", err)
		}
		polished := strings.Replace(string(content), "### Changed\n", "### Changed\n- Soft-success changelog note\n", 1)
		if err := os.WriteFile(changelog, []byte(polished), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write changelog: %w", err)
		}
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})

	o := &Orchestrator{cfg: &config.OrchestratorConfig{KBEnabled: true, BuildSystem: "go"}, paths: paths, logger: log.Discard(), backend: stub}

	if err := o.runPostEpicKB(context.Background(), postEpicState()); err != nil {
		t.Fatalf("runPostEpicKB soft-success: %v", err)
	}
	files := lastCommitFiles(t, dir)
	if strings.TrimSpace(files) != "CHANGELOG.md" {
		t.Fatalf("expected CHANGELOG.md committed, got: %q", files)
	}
}

func TestRunPostEpicKB_CombinedChangesCreateSeparateScopedCommits(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	paths := NewPaths(dir)

	stub := backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		taskPath := filepath.Join(paths.DougDir, "ACTIVE_TASK.md")
		data, err := os.ReadFile(taskPath)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "SUCCESS"`, 1)
		if err := os.WriteFile(taskPath, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "docs", "kb", "combined.md"), []byte("# Combined KB\n"), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write kb: %w", err)
		}
		changelog := filepath.Join(dir, "CHANGELOG.md")
		content, err := os.ReadFile(changelog)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read changelog: %w", err)
		}
		polished := strings.Replace(string(content), "### Changed\n", "### Changed\n- Combined changelog note\n", 1)
		if err := os.WriteFile(changelog, []byte(polished), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write changelog: %w", err)
		}
		code := 0
		return agent.RunResponse{Status: agent.RunStatusCompleted, ExitCode: &code}, nil
	})

	o := &Orchestrator{cfg: &config.OrchestratorConfig{KBEnabled: true, BuildSystem: "go"}, paths: paths, logger: log.Discard(), backend: stub}

	if err := o.runPostEpicKB(context.Background(), postEpicState()); err != nil {
		t.Fatalf("runPostEpicKB: %v", err)
	}

	cmd := exec.Command("git", "show", "--name-only", "--format=", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git show HEAD: %v", err)
	}
	if strings.TrimSpace(string(out)) != "CHANGELOG.md" {
		t.Fatalf("expected HEAD to be scoped changelog commit, got: %q", out)
	}
	cmd = exec.Command("git", "show", "--name-only", "--format=", "HEAD~1")
	cmd.Dir = dir
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("git show HEAD~1: %v", err)
	}
	if strings.TrimSpace(string(out)) != "docs/kb/combined.md" {
		t.Fatalf("expected previous commit to be scoped KB commit, got: %q", out)
	}
}
