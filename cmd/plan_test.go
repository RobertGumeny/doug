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

func TestPlanProject_CreatesPlanAndInvokesAgent(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "plan_agent_command: mock-agent {{skill_name}} {{task_id}}\nbuild_system: go\n")

	restore := stubPlanDeps()
	defer restore()

	var runCalls int
	planRunAgent = func(ctx context.Context, agentCommand, projectRoot string, heartbeatInterval time.Duration, heartbeatFn func(time.Duration), output io.Writer) (time.Duration, error) {
		runCalls++
		if !strings.Contains(agentCommand, "plan") {
			t.Fatalf("expected plan skill in command, got %q", agentCommand)
		}
		if !strings.Contains(agentCommand, planTaskID) {
			t.Fatalf("expected plan task id in command, got %q", agentCommand)
		}
		if projectRoot != dir {
			t.Fatalf("projectRoot = %q, want %q", projectRoot, dir)
		}
		if heartbeatInterval != 0 {
			t.Fatalf("plan run should suppress heartbeat: heartbeatInterval = %v, want 0", heartbeatInterval)
		}
		if heartbeatFn != nil {
			t.Fatalf("plan run should suppress heartbeat: heartbeatFn should be nil")
		}
		return time.Second, nil
	}

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

	if _, err := os.Stat(filepath.Join(dir, ".doug", "ACTIVE_TASK.md")); !os.IsNotExist(err) {
		t.Fatalf("expected plan command not to create root ACTIVE_TASK.md, stat err=%v", err)
	}
}

func TestPlanProject_RefreshesOwnedBriefAndPreservesWorkbookBody(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "plan_agent_command: mock-agent {{skill_name}} {{task_id}}\n")
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), "<!-- DOUG-PLAN-BRIEF:START -->\nold brief\n<!-- DOUG-PLAN-BRIEF:END -->\n\n# Existing Plan\n\nKeep me.\n")

	restore := stubPlanDeps()
	defer restore()

	planRunAgent = func(ctx context.Context, agentCommand, projectRoot string, heartbeatInterval time.Duration, heartbeatFn func(time.Duration), output io.Writer) (time.Duration, error) {
		return time.Second, nil
	}

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

	planRunAgent = func(ctx context.Context, agentCommand, projectRoot string, heartbeatInterval time.Duration, heartbeatFn func(time.Duration), output io.Writer) (time.Duration, error) {
		return time.Second, nil
	}

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
		defer reset()

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

func TestResolvePlanAgentCommand_RoutesPlanningThroughPlanWorkbook(t *testing.T) {
	command := `codex exec "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. This is a doug-orchestrated run: use .doug/ACTIVE_TASK.md as the task brief and complete the task described there."`

	got := resolvePlanAgentCommand(command, "plan", "PLAN")
	if !strings.Contains(got, ".doug/plan/PLAN.md as the planning workbook") {
		t.Fatalf("expected plan workbook prompt, got %q", got)
	}
	if !strings.Contains(got, "If the repository is empty or near-empty and the user has explicit day-0 or bootstrap intent, prefer scaffold-oriented handoff data under `manifest` instead of defaulting to an implementation epic.") {
		t.Fatalf("expected bootstrap/scaffold guidance in plan prompt, got %q", got)
	}
	if strings.Contains(got, ".doug/ACTIVE_TASK.md") {
		t.Fatalf("expected runtime ACTIVE_TASK prompt to be removed, got %q", got)
	}
}

func stubPlanDeps() func() {
	oldLoadConfig := planLoadConfig
	oldRunAgent := planRunAgent

	planLoadConfig = config.LoadConfig
	planRunAgent = agent.RunAgent

	return func() {
		planLoadConfig = oldLoadConfig
		planRunAgent = oldRunAgent
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
