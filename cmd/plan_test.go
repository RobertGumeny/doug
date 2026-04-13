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

	if err := planProjectContext(context.Background(), dir, io.Discard); err != nil {
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

	if err := planProjectContext(context.Background(), dir, io.Discard); err != nil {
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
	if !strings.Contains(content, "# Existing Plan\n\nKeep me.\n") {
		t.Fatalf("expected existing workbook body to be preserved, got:\n%s", content)
	}
}

func TestResolvePlanAgentCommand_RoutesPlanningThroughPlanWorkbook(t *testing.T) {
	command := `codex exec "[DOUG_TASK_ID: {{task_id}}] Please activate {{skill_name}}. This is a doug-orchestrated run: use .doug/ACTIVE_TASK.md as the task brief and complete the task described there."`

	got := resolvePlanAgentCommand(command, "plan", "PLAN")
	if !strings.Contains(got, ".doug/plan/PLAN.md as the planning workbook") {
		t.Fatalf("expected plan workbook prompt, got %q", got)
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
