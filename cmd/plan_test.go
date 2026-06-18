package cmd

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/agent"
	runstats "github.com/robertgumeny/doug/internal/stats"
	"github.com/robertgumeny/doug/internal/testutil"
)

func TestRunPlan_CommandIntentModes(t *testing.T) {
	t.Run("uses explicit flag intent and writes it into PLAN context", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "")

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
		planRunPiInteractive = piInteractiveLauncherFunc(func(ctx context.Context, req agent.PiInteractiveLaunchRequest) (agent.RunResponse, error) {
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
			"- Intent: Plan a safer runtime/archive boundary",
			"- Mode: definition",
			"- Target epic: EPIC-29",
		} {
			if !strings.Contains(content, want) {
				t.Fatalf("expected %q in PLAN.md, got:\n%s", want, content)
			}
		}
	})

	t.Run("captures intent interactively before launching plan agent", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "")

		restoreDeps := stubPlanDeps()
		restoreFlags := stubPlanFlags()
		restoreInteractive := stubPlanInteractive()
		defer restoreDeps()
		defer restoreFlags()
		defer restoreInteractive()

		p := &planStubPrompter{textValue: "  Shape the next plan around archived bug follow-up.  "}
		planIsInteractive = func() bool { return true }
		planNewPrompter = func() planningIntentPrompter { return p }
		planRunPiInteractive = piInteractiveLauncherFunc(func(ctx context.Context, req agent.PiInteractiveLaunchRequest) (agent.RunResponse, error) {
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
		if !strings.Contains(string(data), "- Intent: Shape the next plan around archived bug follow-up.") {
			t.Fatalf("expected interactive planning intent in PLAN.md, got:\n%s", string(data))
		}
	})

	t.Run("fails in non-interactive mode when no intent is supplied", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "")

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

func TestRunPlan_AutoDetectsGreenfieldModeForNearEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "")
	testutil.WriteFile(t, filepath.Join(dir, "README.md"), "# New Project\n")

	restoreDeps := stubPlanDeps()
	restoreFlags := stubPlanFlags()
	restoreInteractive := stubPlanInteractive()
	defer restoreDeps()
	defer restoreFlags()
	defer restoreInteractive()

	planFlags.intent = "Plan the initial project scaffold"
	planIsInteractive = func() bool { return false }
	planRunPiInteractive = piInteractiveLauncherFunc(func(ctx context.Context, req agent.PiInteractiveLaunchRequest) (agent.RunResponse, error) {
		return agent.RunResponse{Duration: time.Second}, nil
	})

	stderr := capturePlanStderr(t, func() {
		if err := runPlan(&cobra.Command{}, nil); err != nil {
			t.Fatalf("runPlan: %v", err)
		}
	})

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "plan", "PLAN.md"))
	if err != nil {
		t.Fatalf("read PLAN.md: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"- Mode: greenfield",
		"the `manifest` block is required output in `## Handoff Data`",
		`  mode: "greenfield"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected auto-detected greenfield phrase %q in PLAN.md, got:\n%s", want, content)
		}
	}
	if strings.Contains(content, `  mode: "brownfield"`) {
		t.Fatalf("auto-detected greenfield PLAN.md seed must not default project.mode to brownfield, got:\n%s", content)
	}
	if !strings.Contains(stderr, "auto-detected greenfield planning mode") {
		t.Fatalf("expected auto-detection log line on stderr, got %q", stderr)
	}
}

func TestGreenfieldPlanningRepoHeuristic(t *testing.T) {
	t.Run("rejects recognized build files", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteFile(t, filepath.Join(dir, "go.mod"), "module example.com/project\n")
		if isGreenfieldPlanningRepo(dir) {
			t.Fatal("expected repo with recognized build file not to be greenfield")
		}
	})

	t.Run("rejects repos with more than a few non-Doug files", func(t *testing.T) {
		dir := t.TempDir()
		testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "")
		for _, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt"} {
			testutil.WriteFile(t, filepath.Join(dir, name), name)
		}
		if isGreenfieldPlanningRepo(dir) {
			t.Fatal("expected repo with many non-Doug files not to be greenfield")
		}
	})
}

func TestPlanProject_CreatesPlanAndInvokesAgent(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "build_system: go\n")

	restore := stubPlanDeps()
	defer restore()

	var runCalls int
	planRunPiInteractive = piInteractiveLauncherFunc(func(ctx context.Context, req agent.PiInteractiveLaunchRequest) (agent.RunResponse, error) {
		runCalls++
		if req.ProjectRoot != dir {
			t.Fatalf("projectRoot = %q, want %q", req.ProjectRoot, dir)
		}
		if req.Phase != agent.RunPhasePlanning {
			t.Fatalf("phase = %q, want %q", req.Phase, agent.RunPhasePlanning)
		}
		if req.Task.ID != planTaskID || req.Task.Type != "plan" {
			t.Fatalf("unexpected task context: %+v", req.Task)
		}
		if req.Task.Attempt != 1 || req.Task.MaxRetries != 1 {
			t.Fatalf("unexpected task attempt context: %+v", req.Task)
		}
		wantSessionDir := agent.PiInteractiveSessionDir(dir, agent.RunPhasePlanning, req.Task)
		if req.SessionDir != wantSessionDir {
			t.Fatalf("sessionDir = %q, want %q", req.SessionDir, wantSessionDir)
		}
		if !strings.Contains(req.InitialPrompt, ".doug/ACTIVE_TASK.md") {
			t.Fatalf("expected bootstrap prompt to reference ACTIVE_TASK.md, got %q", req.InitialPrompt)
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
		"# Planning Session",
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
		"Extra fields will cause `doug handoff` to reject the payload.",
		"use the `manifest` block rather than `epics` alone",
		"single working artifact",
		"the user has confirmed it",
		"**This session:**",
		"- Intent: Plan a safer backlog handoff flow",
		"- Mode: definition",
		"- Target epic: EPIC-19",
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
		"source of truth for this planning cycle",
		"alignment summary",
		"explicitly confirmed the summary",
		"downstream working artifacts rather than competing canonical briefs",
	} {
		if !strings.Contains(activeTaskContent, want) {
			t.Fatalf("expected %q in ACTIVE_TASK.md, got:\n%s", want, activeTaskContent)
		}
	}
}

func TestPlanProject_RefreshesOwnedBriefAndPreservesWorkbookBody(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "")
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), "<!-- DOUG-PLAN-BRIEF:START -->\nold brief\n<!-- DOUG-PLAN-BRIEF:END -->\n\n# Existing Plan\n\nKeep me.\n")

	restore := stubPlanDeps()
	defer restore()

	planRunPiInteractive = piInteractiveLauncherFunc(func(ctx context.Context, req agent.PiInteractiveLaunchRequest) (agent.RunResponse, error) {
		if !strings.Contains(req.InitialPrompt, ".doug/ACTIVE_TASK.md") {
			t.Fatalf("expected bootstrap prompt to reference ACTIVE_TASK.md, got %q", req.InitialPrompt)
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
	if !strings.Contains(content, "# Planning Session") {
		t.Fatalf("expected refreshed Doug briefing, got:\n%s", content)
	}
	if strings.Contains(content, "old brief") {
		t.Fatalf("expected old briefing to be replaced, got:\n%s", content)
	}
	for _, want := range []string{
		"- Intent: Retarget the plan around epic activation",
		"- Mode: roadmapping",
		"- Target epic: EPIC-7",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected refreshed context %q, got:\n%s", want, content)
		}
	}
	if !strings.Contains(content, "# Existing Plan\n\nKeep me.\n") {
		t.Fatalf("expected existing workbook body to be preserved, got:\n%s", content)
	}

	statsPath := filepath.Join(dir, ".doug", "logs", "stats", "EPIC-7", "stats-PLAN_attempt-1.json")
	statsData, err := os.ReadFile(statsPath)
	if err != nil {
		t.Fatalf("read stats file: %v", err)
	}
	var record runstats.RunStats
	if err := json.Unmarshal(statsData, &record); err != nil {
		t.Fatalf("parse stats file: %v", err)
	}
	if record.Phase != "planning" || record.TaskID != planTaskID || record.DurationMs != 1000 {
		t.Fatalf("unexpected stats record: %+v", record)
	}
}

func TestPlanProject_SurfacesArchivedBugPlanningContext(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "")
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

	planRunPiInteractive = piInteractiveLauncherFunc(func(ctx context.Context, req agent.PiInteractiveLaunchRequest) (agent.RunResponse, error) {
		if !strings.Contains(req.InitialPrompt, ".doug/ACTIVE_TASK.md") {
			t.Fatalf("expected bootstrap prompt to reference ACTIVE_TASK.md, got %q", req.InitialPrompt)
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
		"**Unresolved bugs**",
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

		p := &planStubPrompter{textValue: "  Plan the next release around backlog cleanup  "}
		planIsInteractive = func() bool { return true }
		planNewPrompter = func() planningIntentPrompter { return p }

		got, err := resolvePlanRunContext(&cobra.Command{}, nil)
		if err != nil {
			t.Fatalf("resolvePlanRunContext: %v", err)
		}
		if !p.composeCalled {
			t.Fatal("expected Compose to be used for interactive planning intent capture")
		}
		if got.Intent != "Plan the next release around backlog cleanup" {
			t.Fatalf("Intent = %q, want trimmed text value", got.Intent)
		}
	})

	t.Run("does not prompt when explicit flag intent is provided", func(t *testing.T) {
		reset := stubPlanFlags()
		restore := stubPlanInteractive()
		defer reset()
		defer restore()

		p := &planStubPrompter{textValue: "should not be used"}
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

		p := &planStubPrompter{textValue: " \n\t "}
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

func capturePlanStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	os.Stderr = writer
	defer func() { os.Stderr = oldStderr }()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read captured stderr: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return string(data)
}

func stubPlanDeps() func() {
	oldRunPiInteractive := planRunPiInteractive
	oldNewPiInteractiveLauncher := planNewPiInteractiveLauncher

	planRunPiInteractive = piInteractiveLauncherFunc(func(context.Context, agent.PiInteractiveLaunchRequest) (agent.RunResponse, error) {
		return agent.RunResponse{}, nil
	})
	planNewPiInteractiveLauncher = func() piInteractiveLauncher { return agent.NewPiInteractiveLauncher() }

	return func() {
		planRunPiInteractive = oldRunPiInteractive
		planNewPiInteractiveLauncher = oldNewPiInteractiveLauncher
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

type piInteractiveLauncherFunc func(context.Context, agent.PiInteractiveLaunchRequest) (agent.RunResponse, error)

func (f piInteractiveLauncherFunc) Run(ctx context.Context, req agent.PiInteractiveLaunchRequest) (agent.RunResponse, error) {
	return f(ctx, req)
}

type planStubPrompter struct {
	textValue     string
	textErr       error
	composeCalled bool
}

func (p *planStubPrompter) Compose(_ string, _ string) (string, error) {
	p.composeCalled = true
	return p.textValue, p.textErr
}
