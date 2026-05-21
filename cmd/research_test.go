package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/testutil"
)

func TestResearchProject_InvokesAgentWithResearchContract(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "build_system: go\n")

	restore := stubResearchDeps()
	defer restore()

	var runCalls int
	researchRunAgent = backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		runCalls++
		activeTaskPath := filepath.Join(dir, ".doug", "ACTIVE_TASK.md")
		researchLogsPath := filepath.Join(dir, ".doug", "logs", "research")

		if req.Phase != agent.RunPhaseResearch {
			t.Fatalf("phase = %q, want %q", req.Phase, agent.RunPhaseResearch)
		}
		if req.Task.ID != researchTaskID || req.Task.Type != "research" {
			t.Fatalf("unexpected task context: %+v", req.Task)
		}
		if req.Task.Attempt != 1 || req.Task.MaxRetries != 1 {
			t.Fatalf("unexpected task attempt context: %+v", req.Task)
		}
		if req.Brief.Path != activeTaskPath || req.Brief.Format != agent.BriefFormatMarkdown || req.Brief.Authority != agent.ArtifactAuthorityDoug {
			t.Fatalf("unexpected brief: %+v", req.Brief)
		}
		if len(req.ContextLoadOrder) != 3 {
			t.Fatalf("contextLoadOrder length = %d, want 3", len(req.ContextLoadOrder))
		}
		if req.ContextLoadOrder[2].Kind != agent.ContextInputCanonicalBrief || req.ContextLoadOrder[2].Path != activeTaskPath || !req.ContextLoadOrder[2].Required {
			t.Fatalf("unexpected canonical brief context: %+v", req.ContextLoadOrder[2])
		}
		if len(req.Artifacts.Write) != 2 {
			t.Fatalf("write artifact count = %d, want 2", len(req.Artifacts.Write))
		}
		if req.Artifacts.Write[0].Path != activeTaskPath {
			t.Fatalf("unexpected first write artifact: %+v", req.Artifacts.Write[0])
		}
		if req.Artifacts.Write[1].Path != researchLogsPath {
			t.Fatalf("unexpected second write artifact path: got %q want %q", req.Artifacts.Write[1].Path, researchLogsPath)
		}
		if req.Artifacts.Write[1].Authority != agent.ArtifactAuthorityDoug {
			t.Fatalf("unexpected write artifact authority: %+v", req.Artifacts.Write[1])
		}
		if len(req.Artifacts.Read) != 4 {
			t.Fatalf("read artifact count = %d, want 4", len(req.Artifacts.Read))
		}
		if req.Artifacts.Read[0].Path != dir || req.Artifacts.Read[0].Purpose != agent.ArtifactPurposeProjectWorkspace {
			t.Fatalf("unexpected project workspace read artifact: %+v", req.Artifacts.Read[0])
		}
		if req.Routing.Workflow != "research" || req.Routing.SkillName != "research" {
			t.Fatalf("unexpected routing: %+v", req.Routing)
		}
		if req.Restrictions.Read.Mode != agent.RestrictionModeInherit {
			t.Fatalf("unexpected read restriction mode: %+v", req.Restrictions.Read)
		}
		if req.Restrictions.Write.Mode != agent.RestrictionModeAllowList {
			t.Fatalf("unexpected write restriction mode: %+v", req.Restrictions.Write)
		}
		if len(req.Restrictions.Write.Paths) != 2 || req.Restrictions.Write.Paths[0] != activeTaskPath || req.Restrictions.Write.Paths[1] != researchLogsPath {
			t.Fatalf("unexpected write restriction paths: %+v", req.Restrictions.Write.Paths)
		}
		if !strings.Contains(req.InitialPrompt, "research") {
			t.Fatalf("expected research skill in prompt, got %q", req.InitialPrompt)
		}
		if !strings.Contains(req.InitialPrompt, researchTaskID) {
			t.Fatalf("expected research task id in prompt, got %q", req.InitialPrompt)
		}
		if req.ProjectRoot != dir {
			t.Fatalf("projectRoot = %q, want %q", req.ProjectRoot, dir)
		}
		if req.HeartbeatInterval != 0 {
			t.Fatalf("research run should suppress heartbeat: heartbeatInterval = %v, want 0", req.HeartbeatInterval)
		}
		if req.HeartbeatFn != nil {
			t.Fatalf("research run should suppress heartbeat: heartbeatFn should be nil")
		}
		if req.Output != nil {
			t.Fatalf("research run should use interactive terminal: Output should be nil")
		}
		return agent.RunResponse{Duration: time.Second}, nil
	})

	runCtx := researchRunContext{
		Topic: "agent execution architecture",
		Scope: "feature",
	}

	if err := researchProjectContext(context.Background(), dir, io.Discard, runCtx); err != nil {
		t.Fatalf("researchProjectContext: %v", err)
	}

	if runCalls != 1 {
		t.Fatalf("runCalls = %d, want 1", runCalls)
	}
}

func TestResearchProject_WritesActiveTaskBrief(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "")

	restore := stubResearchDeps()
	defer restore()

	researchRunAgent = backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		return agent.RunResponse{Duration: time.Second}, nil
	})

	runCtx := researchRunContext{
		Topic: "config loading pipeline",
		Scope: "feature",
	}

	if err := researchProjectContext(context.Background(), dir, io.Discard, runCtx); err != nil {
		t.Fatalf("researchProjectContext: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "ACTIVE_TASK.md"))
	if err != nil {
		t.Fatalf("read ACTIVE_TASK.md: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"**Task ID**: RESEARCH",
		"**Task Type**: research",
		"config loading pipeline",
		"## Research Output",
		".doug/logs/research/",
		"Do not create `RESEARCH_REPORT.md` in the project root",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q in ACTIVE_TASK.md, got:\n%s", want, content)
		}
	}
}

func TestResolveResearchRunContext(t *testing.T) {
	t.Run("uses positional topic when flags are empty", func(t *testing.T) {
		reset := stubResearchFlags()
		defer reset()

		got, err := resolveResearchRunContext(researchCmd, []string{"agent", "backend", "flow"})
		if err != nil {
			t.Fatalf("resolveResearchRunContext: %v", err)
		}
		if got.Topic != "agent backend flow" {
			t.Fatalf("Topic = %q, want %q", got.Topic, "agent backend flow")
		}
	})

	t.Run("rejects conflicting flag and positional topic", func(t *testing.T) {
		reset := stubResearchFlags()
		defer reset()

		researchFlags.topic = "topic from flag"
		_, err := resolveResearchRunContext(researchCmd, []string{"different topic"})
		if err == nil {
			t.Fatal("expected conflict error, got nil")
		}
		if !strings.Contains(err.Error(), "provided twice") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("validates and normalizes scope", func(t *testing.T) {
		reset := stubResearchFlags()
		defer reset()

		researchFlags.scope = "Feature"
		got, err := resolveResearchRunContext(researchCmd, nil)
		if err != nil {
			t.Fatalf("resolveResearchRunContext: %v", err)
		}
		if got.Scope != "feature" {
			t.Fatalf("Scope = %q, want %q", got.Scope, "feature")
		}
	})

	t.Run("rejects invalid scope", func(t *testing.T) {
		reset := stubResearchFlags()
		defer reset()

		researchFlags.scope = "module"
		_, err := resolveResearchRunContext(researchCmd, nil)
		if err == nil {
			t.Fatal("expected invalid scope error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid scope") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("allows empty topic", func(t *testing.T) {
		reset := stubResearchFlags()
		defer reset()

		got, err := resolveResearchRunContext(researchCmd, nil)
		if err != nil {
			t.Fatalf("resolveResearchRunContext: %v", err)
		}
		if got.Topic != "" {
			t.Fatalf("Topic = %q, want empty", got.Topic)
		}
	})
}

// TestResearchProject_PropagatesInteractionModeToRoutingWhenRPC verifies that when
// the policy configures interaction_mode: rpc for the research task type, the resolved
// mode propagates to req.Routing.InteractionMode in the RunRequest sent to the backend.
func TestResearchProject_PropagatesInteractionModeToRoutingWhenRPC(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"),
		"policy:\n  tasks:\n    research:\n      interaction_mode: rpc\n")

	restore := stubResearchDeps()
	defer restore()

	researchRunAgent = backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		if req.Routing.InteractionMode != "rpc" {
			t.Errorf("interaction mode = %q, want rpc", req.Routing.InteractionMode)
		}
		return agent.RunResponse{}, nil
	})

	runCtx := researchRunContext{Topic: "execution backend selection", Scope: "feature"}
	if err := researchProjectContext(context.Background(), dir, io.Discard, runCtx); err != nil {
		t.Fatalf("researchProjectContext: %v", err)
	}
}

// TestResearchProject_SelectsPiAdapterForRPCModeViaProductionPath verifies that
// when researchRunAgent is nil (the production path) and interaction_mode: rpc is
// configured in policy, researchNewBackend is called with an exec whose
// InteractionMode is "rpc" and returns a PiAdapter.
func TestResearchProject_SelectsPiAdapterForRPCModeViaProductionPath(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"),
		"policy:\n  tasks:\n    research:\n      interaction_mode: rpc\n")

	restore := stubResearchDeps()
	defer restore()

	// Leave researchRunAgent nil so the production path calls researchNewBackend.
	researchRunAgent = nil

	var selectedBackend agent.Backend
	researchNewBackend = func(exec config.ResolvedExecution) agent.Backend {
		b := agent.NewBackend(exec)
		selectedBackend = b
		return backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
			return agent.RunResponse{}, nil
		})
	}

	runCtx := researchRunContext{Topic: "validate Pi adapter selection for research", Scope: "feature"}
	if err := researchProjectContext(context.Background(), dir, io.Discard, runCtx); err != nil {
		t.Fatalf("researchProjectContext: %v", err)
	}

	if _, ok := selectedBackend.(agent.PiAdapter); !ok {
		t.Fatalf("expected PiAdapter for rpc interaction mode, got %T", selectedBackend)
	}
}

func stubResearchDeps() func() {
	oldLoadConfig := researchLoadConfig
	oldRunAgent := researchRunAgent
	oldNewBackend := researchNewBackend

	researchLoadConfig = config.LoadConfig
	researchRunAgent = nil
	researchNewBackend = agent.NewBackend

	return func() {
		researchLoadConfig = oldLoadConfig
		researchRunAgent = oldRunAgent
		researchNewBackend = oldNewBackend
	}
}

func stubResearchFlags() func() {
	oldFlags := researchFlags
	researchFlags = struct {
		topic string
		scope string
	}{}

	return func() {
		researchFlags = oldFlags
	}
}
