package orchestrator

import (
	"context"
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
	testutil.WriteFile(t, filepath.Join(dir, "docs", "kb", "README.md"), "# KB Index\n")

	runGit("add", ".")
	runGit("commit", "-m", "initial")

	return dir
}

func writePostEpicAgent(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, ".doug", "post_epic_agent.go")
	testutil.WriteFile(t, path, body)
	return path
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

func TestRunPostEpicKB_WritesConstrainedDocumentationBriefing(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	writePostEpicAgent(t, dir, `package main

import (
	"os"
	"strings"
)

func main() {
	path := ".doug/ACTIVE_TASK.md"
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	content := strings.Replace(string(data), "outcome: \"\"", "outcome: \"SUCCESS\"", 1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		panic(err)
	}
}`)

	paths := NewPaths(dir)
	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			KBEnabled:             true,
			BuildSystem:           "go",
			RunAgentCommand:       "go run ./.doug/post_epic_agent.go",
			AgentHeartbeatSeconds: 60,
		},
		paths:  paths,
		logger: log.Discard(),
	}

	if err := o.runPostEpicKB(context.Background(), postEpicState()); err != nil {
		t.Fatalf("runPostEpicKB: %v", err)
	}

	sessionPath := filepath.Join(paths.LogsDir, "sessions", "EPIC-20", "session-POST_EPIC_KB_attempt-1.md")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatalf("read archived post-epic KB session: %v", err)
	}
	content := string(data)

	for _, want := range []string{
		"**Task Type**: documentation",
		"Use the documentation workflow",
		"`docs/kb/README.md`",
		"Write KB output only under `docs/kb/`.",
		"Do not reopen or modify epic runtime state",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected archived session to contain %q, got:\n%s", want, content)
		}
	}
}

// TestRunPostEpicKB_UsesInjectedBackend verifies that runPostEpicKB routes
// agent invocation through the Orchestrator's backend seam rather than calling
// RunAgent directly. If the seam is bypassed this test fails because the stub
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
		if len(req.ContextLoadOrder) != 3 {
			return agent.RunResponse{}, fmt.Errorf("contextLoadOrder length = %d, want 3", len(req.ContextLoadOrder))
		}
		if req.ContextLoadOrder[2].Kind != agent.ContextInputCanonicalBrief || req.ContextLoadOrder[2].Path != taskPath || !req.ContextLoadOrder[2].Required || req.ContextLoadOrder[2].Authority != agent.ArtifactAuthorityDoug {
			return agent.RunResponse{}, fmt.Errorf("unexpected canonical brief context: %+v", req.ContextLoadOrder[2])
		}
		if req.Routing.Workflow != "post_epic_kb" || req.Routing.SkillName != "implement-documentation" {
			return agent.RunResponse{}, fmt.Errorf("unexpected routing: %+v", req.Routing)
		}
		if req.Restrictions.Read.Mode != agent.RestrictionModeInherit || req.Restrictions.Write.Mode != agent.RestrictionModeAllowList {
			return agent.RunResponse{}, fmt.Errorf("unexpected restrictions: %+v", req.Restrictions)
		}
		if len(req.Restrictions.Write.Paths) != 2 || req.Restrictions.Write.Paths[0] != kbRoot || req.Restrictions.Write.Paths[1] != taskPath {
			return agent.RunResponse{}, fmt.Errorf("unexpected write restriction paths: %+v", req.Restrictions.Write.Paths)
		}
		if len(req.Artifacts.Read) != 6 {
			return agent.RunResponse{}, fmt.Errorf("read artifact count = %d, want 6", len(req.Artifacts.Read))
		}
		if req.Artifacts.Write[0].Path != kbRoot || req.Artifacts.Write[0].Purpose != agent.ArtifactPurposeKnowledgeBase {
			return agent.RunResponse{}, fmt.Errorf("unexpected kb write artifact: %+v", req.Artifacts.Write[0])
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
			RunAgentCommand:       "stub-never-executed",
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
	metadataPath := agent.RunMetadataPath(filepath.Join(paths.LogsDir, "output", "EPIC-20", "output-post_epic_kb.log"))
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read post-epic KB run metadata: %v", err)
	}
	if !strings.Contains(string(metadata), `"pi-session-456"`) {
		t.Fatalf("expected post-epic KB run metadata to capture session ids, got:\n%s", metadata)
	}
}

func TestRunPostEpicKB_RejectsChangesOutsideDocsKB(t *testing.T) {
	dir := setupPostEpicKBRepo(t)
	writePostEpicAgent(t, dir, `package main

import (
	"os"
	"strings"
)

func main() {
	path := ".doug/ACTIVE_TASK.md"
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	content := strings.Replace(string(data), "outcome: \"\"", "outcome: \"SUCCESS\"", 1)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile("docs/kb/new-article.md", []byte("# Allowed KB article\n"), 0o644); err != nil {
		panic(err)
	}
	if err := os.WriteFile("rogue-kb-note.md", []byte("outside docs/kb\n"), 0o644); err != nil {
		panic(err)
	}
}`)

	paths := NewPaths(dir)
	o := &Orchestrator{
		cfg: &config.OrchestratorConfig{
			KBEnabled:             true,
			BuildSystem:           "go",
			RunAgentCommand:       "go run ./.doug/post_epic_agent.go",
			AgentHeartbeatSeconds: 60,
		},
		paths:  paths,
		logger: log.Discard(),
	}

	err := o.runPostEpicKB(context.Background(), postEpicState())
	if err == nil {
		t.Fatal("expected post-epic KB path validation error, got nil")
	}
	if !strings.Contains(err.Error(), `post-epic KB produced changes outside docs/kb/: "rogue-kb-note.md"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
