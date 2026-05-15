package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/agent"
	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/testutil"
)

func TestRunPlan_CommandIntentModes(t *testing.T) {
	t.Run("uses explicit flag intent and writes it into PLAN context", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "plan_agent_command: mock-agent {{skill_name}} {{task_id}}\n")

		restoreDeps := stubPlanDeps()
		restoreFlags := stubPlanFlags()
		restoreInteractive := stubPlanInteractive()
		defer restoreDeps()
		defer restoreFlags()
		defer restoreInteractive()

		planFlags.intent = "Plan a safer runtime/archive boundary"
		planFlags.mode = "definition"
		planFlags.epic = "EPIC-29"
		planIsInteractive = func() bool { return false }
		planNewPrompter = func() planningIntentPrompter { return &planStubPrompter{} }
		planRunAgent = backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
			return agent.RunResponse{Duration: time.Second}, nil
		})

		if err := runPlan(&cobra.Command{}, nil); err != nil {
			t.Fatalf("runPlan: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(dir, ".doug", "plan", "PLAN.md"))
		if err != nil {
			t.Fatalf("read PLAN.md: %v", err)
		}
		content := string(data)
		for _, want := range []string{
			"- Planning intent: Plan a safer runtime/archive boundary",
			"- Planning mode: definition",
			"- Target epic hint: EPIC-29",
		} {
			if !strings.Contains(content, want) {
				t.Fatalf("expected %q in PLAN.md, got:\n%s", want, content)
			}
		}
	})

	t.Run("captures intent interactively before launching plan agent", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "plan_agent_command: mock-agent {{skill_name}} {{task_id}}\n")

		restoreDeps := stubPlanDeps()
		restoreFlags := stubPlanFlags()
		restoreInteractive := stubPlanInteractive()
		defer restoreDeps()
		defer restoreFlags()
		defer restoreInteractive()

		p := &planStubPrompter{composeValue: "  Shape the next plan around archived bug follow-up.\nKeep backlog mutations intentional.  "}
		planIsInteractive = func() bool { return true }
		planNewPrompter = func() planningIntentPrompter { return p }
		planRunAgent = backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
			return agent.RunResponse{Duration: time.Second}, nil
		})

		if err := runPlan(&cobra.Command{}, nil); err != nil {
			t.Fatalf("runPlan: %v", err)
		}
		if !p.composeCalled {
			t.Fatal("expected interactive planning-intent capture before plan launch")
		}

		data, err := os.ReadFile(filepath.Join(dir, ".doug", "plan", "PLAN.md"))
		if err != nil {
			t.Fatalf("read PLAN.md: %v", err)
		}
		if !strings.Contains(string(data), "- Planning intent: Shape the next plan around archived bug follow-up.\nKeep backlog mutations intentional.") {
			t.Fatalf("expected interactive planning intent in PLAN.md, got:\n%s", string(data))
		}
	})

	t.Run("fails in non-interactive mode when no intent is supplied", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "plan_agent_command: mock-agent {{skill_name}} {{task_id}}\n")

		restoreDeps := stubPlanDeps()
		restoreFlags := stubPlanFlags()
		restoreInteractive := stubPlanInteractive()
		defer restoreDeps()
		defer restoreFlags()
		defer restoreInteractive()

		planIsInteractive = func() bool { return false }
		planNewPrompter = func() planningIntentPrompter { return &planStubPrompter{} }

		err := runPlan(&cobra.Command{}, nil)
		if err == nil {
			t.Fatal("expected missing intent error, got nil")
		}
		if !strings.Contains(err.Error(), "planning intent required in non-interactive mode") {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(dir, ".doug", "plan", "PLAN.md")); !os.IsNotExist(statErr) {
			t.Fatalf("PLAN.md should not be created on non-interactive missing-intent failure; stat err = %v", statErr)
		}
	})
}

func TestPlanProject_CreatesPlanAndInvokesAgent(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "plan_agent_command: mock-agent {{skill_name}} {{task_id}}\nbuild_system: go\n")

	restore := stubPlanDeps()
	defer restore()

	var runCalls int
	planRunAgent = backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		runCalls++
		planPath := filepath.Join(dir, ".doug", "plan", "PLAN.md")
		activeTaskPath := filepath.Join(dir, ".doug", "ACTIVE_TASK.md")
		if req.Phase != agent.RunPhasePlanning {
			t.Fatalf("phase = %q, want %q", req.Phase, agent.RunPhasePlanning)
		}
		if req.Task.ID != planTaskID || req.Task.Type != "plan" {
			t.Fatalf("unexpected task context: %+v", req.Task)
		}
		if req.Task.Attempt != 1 || req.Task.MaxRetries != 1 {
			t.Fatalf("unexpected task attempt context: %+v", req.Task)
		}
		if req.Brief.Path != activeTaskPath || req.Brief.Format != agent.BriefFormatMarkdown || req.Brief.Authority != agent.ArtifactAuthorityDoug {
			t.Fatalf("unexpected brief: %+v", req.Brief)
		}
		if len(req.ContextLoadOrder) != 4 {
			t.Fatalf("contextLoadOrder length = %d, want 4", len(req.ContextLoadOrder))
		}
		if req.ContextLoadOrder[2].Kind != agent.ContextInputCanonicalBrief || req.ContextLoadOrder[2].Path != activeTaskPath || !req.ContextLoadOrder[2].Required || req.ContextLoadOrder[2].Authority != agent.ArtifactAuthorityDoug {
			t.Fatalf("unexpected canonical brief context: %+v", req.ContextLoadOrder[2])
		}
		if req.ContextLoadOrder[3].Kind != agent.ContextInputWorkingArtifact || req.ContextLoadOrder[3].Path != planPath || !req.ContextLoadOrder[3].Required || req.ContextLoadOrder[3].Authority != agent.ArtifactAuthorityDoug {
			t.Fatalf("unexpected working artifact context: %+v", req.ContextLoadOrder[3])
		}
		if len(req.Artifacts.Write) != 2 || req.Artifacts.Write[0].Path != activeTaskPath || req.Artifacts.Write[1].Path != planPath {
			t.Fatalf("unexpected write artifacts: %+v", req.Artifacts.Write)
		}
		if len(req.Artifacts.Read) != 5 {
			t.Fatalf("read artifact count = %d, want 5", len(req.Artifacts.Read))
		}
		if req.Artifacts.Read[0].Path != dir || req.Artifacts.Read[0].Purpose != agent.ArtifactPurposeProjectWorkspace {
			t.Fatalf("unexpected project workspace read artifact: %+v", req.Artifacts.Read[0])
		}
		if req.Artifacts.Write[1].Purpose != agent.ArtifactPurposeWorkingArtifact {
			t.Fatalf("unexpected working artifact purpose: %+v", req.Artifacts.Write[1])
		}
		if req.Routing.Workflow != "plan" || req.Routing.SkillName != "plan" {
			t.Fatalf("unexpected routing: %+v", req.Routing)
		}
		if req.Restrictions.Read.Mode != agent.RestrictionModeInherit {
			t.Fatalf("unexpected read restriction: %+v", req.Restrictions.Read)
		}
		if len(req.Restrictions.Read.Paths) != 5 || req.Restrictions.Read.Paths[0] != dir {
			t.Fatalf("unexpected read restriction paths: %+v", req.Restrictions.Read.Paths)
		}
		if req.Restrictions.Write.Mode != agent.RestrictionModeAllowList {
			t.Fatalf("unexpected write restriction mode: %+v", req.Restrictions.Write)
		}
		if len(req.Restrictions.Write.Paths) != 2 || req.Restrictions.Write.Paths[0] != activeTaskPath || req.Restrictions.Write.Paths[1] != planPath {
			t.Fatalf("unexpected write restriction paths: %+v", req.Restrictions.Write.Paths)
		}
		if !strings.Contains(req.Command, "plan") {
			t.Fatalf("expected plan skill in command, got %q", req.Command)
		}
		if !strings.Contains(req.Command, planTaskID) {
			t.Fatalf("expected plan task id in command, got %q", req.Command)
		}
		if req.ProjectRoot != dir {
			t.Fatalf("projectRoot = %q, want %q", req.ProjectRoot, dir)
		}
		if req.HeartbeatInterval != 0 {
			t.Fatalf("plan run should suppress heartbeat: heartbeatInterval = %v, want 0", req.HeartbeatInterval)
		}
		if req.HeartbeatFn != nil {
			t.Fatalf("plan run should suppress heartbeat: heartbeatFn should be nil")
		}
		return agent.RunResponse{Duration: time.Second}, nil
	})

	runCtx := planRunContext{
		Intent: "Plan a safer backlog handoff flow",
		Mode:   "definition",
		Epic:   "EPIC-19",
	}

	if err := planProjectContext(context.Background(), dir, io.Discard, runCtx); err != nil {
		t.Fatalf("planProjectContext: %v", err)
	}

	if runCalls != 1 {
		t.Fatalf("runCalls = %d, want 1", runCalls)
	}

	planPath := filepath.Join(dir, ".doug", "plan", "PLAN.md")
	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read PLAN.md: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"<!-- DOUG-PLAN-BRIEF:START -->",
		"# Doug Planning Brief",
		"# Project Plan",
		"## Planning Objective",
		"## Handoff Data",
		"Fill in this schema exactly. Do not add extra fields.",
		"Unknown fields cause `doug handoff` to fail.",
		`project:`,
		`  name: "My Project"`,
		`  mode: "brownfield"`,
		"# Include `manifest` only when the plan needs greenfield scaffold output.",
		`# manifest:`,
		`#   scaffold:`,
		`#     package_manager: "pnpm"`,
		`#   dependencies:`,
		`#     runtime:`,
		`epics:`,
		`  - id: "EPIC-1"`,
		`        description: "Describe the task here."`,
		`        acceptance_criteria:`,
		"do not add fields beyond the ones shown in the template",
		"Put greenfield scaffold data under `manifest`, not under `project`.",
		"repository is empty or near-empty and the user explicitly wants day-0 bootstrap work",
		"prefer scaffold-oriented handoff data under `manifest` instead of defaulting to an implementation epic",
		"make the handoff scaffold-ready",
		"Planning Run Context:",
		"- Planning intent: Plan a safer backlog handoff flow",
		"- Planning mode: definition",
		"- Target epic hint: EPIC-19",
		"do not diverge",
		"blank or contains only placeholder text",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q in PLAN.md, got:\n%s", want, content)
		}
	}

	activeTaskData, err := os.ReadFile(filepath.Join(dir, ".doug", "ACTIVE_TASK.md"))
	if err != nil {
		t.Fatalf("read ACTIVE_TASK.md: %v", err)
	}
	activeTaskContent := string(activeTaskData)
	for _, want := range []string{
		"**Task ID**: PLAN",
		"**Task Type**: plan",
		"Refine .doug/plan/PLAN.md as the planning workbook for this Doug-managed run.",
		"## Planning Workbook",
		"Canonical brief for this run: `.doug/ACTIVE_TASK.md`",
		"Editable planning workbook: `.doug/plan/PLAN.md`",
		"downstream working artifacts rather than competing canonical briefs",
	} {
		if !strings.Contains(activeTaskContent, want) {
			t.Fatalf("expected %q in ACTIVE_TASK.md, got:\n%s", want, activeTaskContent)
		}
	}
}

func TestPlanProject_RefreshesOwnedBriefAndPreservesWorkbookBody(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "plan_agent_command: mock-agent {{skill_name}} {{task_id}}\n")
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), "<!-- DOUG-PLAN-BRIEF:START -->\nold brief\n<!-- DOUG-PLAN-BRIEF:END -->\n\n# Existing Plan\n\nKeep me.\n")

	restore := stubPlanDeps()
	defer restore()

	planRunAgent = backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		if req.HeartbeatInterval != 0 {
			t.Fatalf("plan run should suppress heartbeat: heartbeatInterval = %v, want 0", req.HeartbeatInterval)
		}
		if req.HeartbeatFn != nil {
			t.Fatalf("plan run should suppress heartbeat: heartbeatFn should be nil")
		}
		return agent.RunResponse{Duration: time.Second}, nil
	})

	runCtx := planRunContext{
		Intent: "Retarget the plan around epic activation",
		Mode:   "roadmapping",
		Epic:   "EPIC-7",
	}

	if err := planProjectContext(context.Background(), dir, io.Discard, runCtx); err != nil {
		t.Fatalf("planProjectContext: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "plan", "PLAN.md"))
	if err != nil {
		t.Fatalf("read PLAN.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Doug Planning Brief") {
		t.Fatalf("expected refreshed Doug briefing, got:\n%s", content)
	}
	if strings.Contains(content, "old brief") {
		t.Fatalf("expected old briefing to be replaced, got:\n%s", content)
	}
	for _, want := range []string{
		"- Planning intent: Retarget the plan around epic activation",
		"- Planning mode: roadmapping",
		"- Target epic hint: EPIC-7",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected refreshed context %q, got:\n%s", want, content)
		}
	}
	if !strings.Contains(content, "# Existing Plan\n\nKeep me.\n") {
		t.Fatalf("expected existing workbook body to be preserved, got:\n%s", content)
	}
}

func TestPlanProject_SurfacesArchivedBugPlanningContext(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "plan_agent_command: mock-agent {{skill_name}} {{task_id}}\n")
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "logs", "bugs", "EPIC-9", "bug-epic-9-open.md"), ""+
		"---\n"+
		"bug_id: \"bug-epic-9-open\"\n"+
		"status: \"open\"\n"+
		"severity: \"non-blocking\"\n"+
		"---\n\n"+
		"## Summary\n\n"+
		"Completed epic bug summary.\n")
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "epics", "EPIC-9", "metadata.yaml"), ""+
		"epic_id: \"EPIC-9\"\n"+
		"status: \"COMPLETED\"\n"+
		"created_at: \"2026-04-01T00:00:00Z\"\n"+
		"source_plan_path: \".doug/plan/PLAN.md\"\n")

	restore := stubPlanDeps()
	defer restore()

	planRunAgent = backendFunc(func(ctx context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		if req.HeartbeatInterval != 0 {
			t.Fatalf("plan run should suppress heartbeat: heartbeatInterval = %v, want 0", req.HeartbeatInterval)
		}
		if req.HeartbeatFn != nil {
			t.Fatalf("plan run should suppress heartbeat: heartbeatFn should be nil")
		}
		return agent.RunResponse{Duration: time.Second}, nil
	})

	if err := planProjectContext(context.Background(), dir, io.Discard, planRunContext{}); err != nil {
		t.Fatalf("planProjectContext: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "plan", "PLAN.md"))
	if err != nil {
		t.Fatalf("read PLAN.md: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"Unresolved Archived Bugs:",
		"`bug-epic-9-open` from epic `EPIC-9`",
		"Completed epic bug summary.",
		"source epic lifecycle `COMPLETED`",
		"do not reopen the `COMPLETED` historical package",
		"archive: `.doug/logs/bugs/EPIC-9/bug-epic-9-open.md`",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q in PLAN.md, got:\n%s", want, content)
		}
	}
}

func TestResolvePlanRunContext(t *testing.T) {
	t.Run("uses positional intent when flags are empty", func(t *testing.T) {
		reset := stubPlanFlags()
		defer reset()

		cmd := &cobra.Command{}
		got, err := resolvePlanRunContext(cmd, []string{"Plan", "the", "next", "epic"})
		if err != nil {
			t.Fatalf("resolvePlanRunContext: %v", err)
		}
		if got.Intent != "Plan the next epic" {
			t.Fatalf("Intent = %q, want %q", got.Intent, "Plan the next epic")
		}
	})

	t.Run("rejects conflicting flag and positional intent", func(t *testing.T) {
		reset := stubPlanFlags()
		defer reset()

		planFlags.intent = "Intent from flag"
		cmd := &cobra.Command{}
		_, err := resolvePlanRunContext(cmd, []string{"Different", "intent"})
		if err == nil {
			t.Fatal("expected conflict error, got nil")
		}
		if !strings.Contains(err.Error(), "provided twice") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("validates and normalizes planning mode", func(t *testing.T) {
		reset := stubPlanFlags()
		restore := stubPlanInteractive()
		defer reset()
		defer restore()

		planIsInteractive = func() bool { return true }
		planNewPrompter = func() planningIntentPrompter { return nil }
		planFlags.intent = "Plan the next epic"
		planFlags.mode = "Greenfield"
		planFlags.epic = "EPIC-22"
		cmd := &cobra.Command{}
		got, err := resolvePlanRunContext(cmd, nil)
		if err != nil {
			t.Fatalf("resolvePlanRunContext: %v", err)
		}
		if got.Mode != "greenfield" {
			t.Fatalf("Mode = %q, want %q", got.Mode, "greenfield")
		}
		if got.Epic != "EPIC-22" {
			t.Fatalf("Epic = %q, want %q", got.Epic, "EPIC-22")
		}
	})

	t.Run("captures intent interactively when none was provided", func(t *testing.T) {
		reset := stubPlanFlags()
		restore := stubPlanInteractive()
		defer reset()
		defer restore()

		p := &planStubPrompter{
			composeValue: "  Plan the next release around backlog cleanup\nand safer handoff sequencing.  ",
		}
		planIsInteractive = func() bool { return true }
		planNewPrompter = func() planningIntentPrompter { return p }

		got, err := resolvePlanRunContext(&cobra.Command{}, nil)
		if err != nil {
			t.Fatalf("resolvePlanRunContext: %v", err)
		}
		if !p.composeCalled {
			t.Fatal("expected Compose to be used for interactive planning intent capture")
		}
		if got.Intent != "Plan the next release around backlog cleanup\nand safer handoff sequencing." {
			t.Fatalf("Intent = %q, want trimmed composed value", got.Intent)
		}
	})

	t.Run("does not prompt when explicit flag intent is provided", func(t *testing.T) {
		reset := stubPlanFlags()
		restore := stubPlanInteractive()
		defer reset()
		defer restore()

		p := &planStubPrompter{composeValue: "should not be used"}
		planIsInteractive = func() bool { return true }
		planNewPrompter = func() planningIntentPrompter { return p }
		planFlags.intent = "Intent from flag"

		got, err := resolvePlanRunContext(&cobra.Command{}, nil)
		if err != nil {
			t.Fatalf("resolvePlanRunContext: %v", err)
		}
		if p.composeCalled {
			t.Fatal("did not expect Compose when explicit intent is already provided")
		}
		if got.Intent != "Intent from flag" {
			t.Fatalf("Intent = %q, want %q", got.Intent, "Intent from flag")
		}
	})

	t.Run("fails fast when no intent is supplied in non-interactive mode", func(t *testing.T) {
		reset := stubPlanFlags()
		restore := stubPlanInteractive()
		defer reset()
		defer restore()

		planIsInteractive = func() bool { return false }
		planNewPrompter = func() planningIntentPrompter { return &planStubPrompter{} }

		_, err := resolvePlanRunContext(&cobra.Command{}, nil)
		if err == nil {
			t.Fatal("expected missing intent error, got nil")
		}
		if !strings.Contains(err.Error(), "non-interactive mode") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects blank interactive planning intent", func(t *testing.T) {
		reset := stubPlanFlags()
		restore := stubPlanInteractive()
		defer reset()
		defer restore()

		p := &planStubPrompter{composeValue: " \n\t "}
		planIsInteractive = func() bool { return true }
		planNewPrompter = func() planningIntentPrompter { return p }

		_, err := resolvePlanRunContext(&cobra.Command{}, nil)
		if err == nil {
			t.Fatal("expected blank planning intent error, got nil")
		}
		if !strings.Contains(err.Error(), "planning intent is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects invalid planning mode", func(t *testing.T) {
		reset := stubPlanFlags()
		defer reset()

		planFlags.mode = "launch"
		cmd := &cobra.Command{}
		_, err := resolvePlanRunContext(cmd, nil)
		if err == nil {
			t.Fatal("expected invalid mode error, got nil")
		}
		if !strings.Contains(err.Error(), "invalid planning mode") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// TestPlanProject_PropagatesExecutionModeToRoutingWhenRPC verifies that when
// the policy configures execution_mode: rpc for the plan task type, the resolved
// mode propagates to req.Routing.ExecutionMode in the RunRequest sent to the backend.
func TestPlanProject_PropagatesExecutionModeToRoutingWhenRPC(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"),
		"plan_agent_command: mock-agent {{skill_name}} {{task_id}}\npolicy:\n  tasks:\n    plan:\n      execution_mode: rpc\n")

	restore := stubPlanDeps()
	restoreFlags := stubPlanFlags()
	restoreInteractive := stubPlanInteractive()
	defer restore()
	defer restoreFlags()
	defer restoreInteractive()

	planIsInteractive = func() bool { return false }
	planNewPrompter = func() planningIntentPrompter { return &planStubPrompter{} }
	planRunAgent = backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
		if req.Routing.ExecutionMode != "rpc" {
			t.Errorf("execution mode = %q, want rpc", req.Routing.ExecutionMode)
		}
		return agent.RunResponse{}, nil
	})

	runCtx := planRunContext{Intent: "validate RPC backend wiring", Mode: "definition"}
	if err := planProjectContext(context.Background(), dir, io.Discard, runCtx); err != nil {
		t.Fatalf("planProjectContext: %v", err)
	}
}

// TestPlanProject_SelectsPiAdapterForRPCModeViaProductionPath verifies that
// when planRunAgent is nil (the production path) and execution_mode: rpc is
// configured in policy, planNewBackend is called with an exec whose
// ExecutionMode is "rpc" and returns a PiAdapter — not DefaultBackend.
func TestPlanProject_SelectsPiAdapterForRPCModeViaProductionPath(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"),
		"plan_agent_command: mock-agent {{skill_name}} {{task_id}}\npolicy:\n  tasks:\n    plan:\n      execution_mode: rpc\n")

	restore := stubPlanDeps()
	defer restore()

	// Leave planRunAgent nil so the production path calls planNewBackend.
	planRunAgent = nil

	var selectedBackend agent.Backend
	planNewBackend = func(exec config.ResolvedExecution) agent.Backend {
		b := agent.NewBackend(exec)
		selectedBackend = b
		return backendFunc(func(_ context.Context, req agent.RunRequest) (agent.RunResponse, error) {
			return agent.RunResponse{}, nil
		})
	}

	runCtx := planRunContext{Intent: "validate Pi adapter selection for planning", Mode: "definition"}
	if err := planProjectContext(context.Background(), dir, io.Discard, runCtx); err != nil {
		t.Fatalf("planProjectContext: %v", err)
	}

	if _, ok := selectedBackend.(agent.PiAdapter); !ok {
		t.Fatalf("expected PiAdapter for rpc execution mode, got %T", selectedBackend)
	}
}

func stubPlanDeps() func() {
	oldLoadConfig := planLoadConfig
	oldRunAgent := planRunAgent
	oldNewBackend := planNewBackend

	planLoadConfig = config.LoadConfig
	planRunAgent = agent.DefaultBackend{}
	planNewBackend = agent.NewBackend

	return func() {
		planLoadConfig = oldLoadConfig
		planRunAgent = oldRunAgent
		planNewBackend = oldNewBackend
	}
}

func stubPlanFlags() func() {
	oldFlags := planFlags
	planFlags = struct {
		intent string
		mode   string
		epic   string
	}{}

	return func() {
		planFlags = oldFlags
	}
}

func stubPlanInteractive() func() {
	oldIsInteractive := planIsInteractive
	oldNewPrompter := planNewPrompter

	return func() {
		planIsInteractive = oldIsInteractive
		planNewPrompter = oldNewPrompter
	}
}

type planStubPrompter struct {
	composeValue  string
	composeErr    error
	composeCalled bool
}

func (p *planStubPrompter) Compose(_ string, _ string) (string, error) {
	p.composeCalled = true
	return p.composeValue, p.composeErr
}
