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
		data, err := os.ReadFile(taskPath)
		if err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: read ACTIVE_TASK.md: %w", err)
		}
		updated := strings.Replace(string(data), `outcome: ""`, `outcome: "SUCCESS"`, 1)
		if err := os.WriteFile(taskPath, []byte(updated), 0o644); err != nil {
			return agent.RunResponse{}, fmt.Errorf("stub: write ACTIVE_TASK.md: %w", err)
		}
		return agent.RunResponse{Duration: time.Millisecond}, nil
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
