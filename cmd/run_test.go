package cmd

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/robertgumeny/doug/internal/config"
	"github.com/robertgumeny/doug/internal/orchestrator"
	"github.com/robertgumeny/doug/internal/plan"
	"github.com/robertgumeny/doug/internal/testutil"
)

type stubRunExecutor struct {
	run func(context.Context) error
}

func (s stubRunExecutor) Run(ctx context.Context) error {
	return s.run(ctx)
}

func TestRunCommandAcceptsAtMostOneEpicID(t *testing.T) {
	if err := runCmd.Args(runCmd, nil); err != nil {
		t.Fatalf("Args(nil): %v", err)
	}
	if err := runCmd.Args(runCmd, []string{"EPIC-17"}); err != nil {
		t.Fatalf("Args(one): %v", err)
	}
	if err := runCmd.Args(runCmd, []string{"EPIC-17", "EXTRA"}); err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestRunOrchestrate_PromotesEpicBeforeStartingRuntime(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), ""+
		"agent_command: git\n"+
		"build_system: go\n"+
		"max_retries: 3\n"+
		"max_iterations: 10\n")

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s): %v", dir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	restore := stubRunDeps()
	defer restore()

	callOrder := make([]string, 0, 3)
	runPromoteEpic = func(projectRoot, epicID string, now time.Time) error {
		callOrder = append(callOrder, "promote")
		if projectRoot != dir {
			t.Fatalf("projectRoot = %q, want %q", projectRoot, dir)
		}
		if epicID != "EPIC-17" {
			t.Fatalf("epicID = %q, want %q", epicID, "EPIC-17")
		}
		return nil
	}
	newRunExecutor = func(cfg *config.OrchestratorConfig, paths orchestrator.Paths) (runExecutor, error) {
		callOrder = append(callOrder, "new")
		if cfg.BuildSystem != "go" {
			t.Fatalf("BuildSystem = %q, want %q", cfg.BuildSystem, "go")
		}
		if paths.ProjectRoot != dir {
			t.Fatalf("paths.ProjectRoot = %q, want %q", paths.ProjectRoot, dir)
		}
		return stubRunExecutor{run: func(context.Context) error {
			callOrder = append(callOrder, "run")
			return nil
		}}, nil
	}

	cmd := &cobra.Command{}
	if err := runOrchestrate(cmd, []string{"EPIC-17"}); err != nil {
		t.Fatalf("runOrchestrate: %v", err)
	}

	if want := []string{"promote", "new", "run"}; !reflect.DeepEqual(callOrder, want) {
		t.Fatalf("call order = %#v, want %#v", callOrder, want)
	}
}

func TestRunOrchestrate_LeavesExistingFlowUnchangedWithoutEpicID(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), ""+
		"agent_command: git\n"+
		"build_system: go\n"+
		"max_retries: 3\n"+
		"max_iterations: 10\n")

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%s): %v", dir, err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
	}()

	restore := stubRunDeps()
	defer restore()

	promoted := false
	runPromoteEpic = func(projectRoot, epicID string, now time.Time) error {
		promoted = true
		return nil
	}
	newRunExecutor = func(cfg *config.OrchestratorConfig, paths orchestrator.Paths) (runExecutor, error) {
		return stubRunExecutor{run: func(context.Context) error { return nil }}, nil
	}

	cmd := &cobra.Command{}
	if err := runOrchestrate(cmd, nil); err != nil {
		t.Fatalf("runOrchestrate: %v", err)
	}

	if promoted {
		t.Fatal("promotion should not run when no epic ID is provided")
	}
}

func stubRunDeps() func() {
	oldNow := runNow
	oldPromoteEpic := runPromoteEpic
	oldNewRunExecutor := newRunExecutor

	runNow = time.Now
	runPromoteEpic = plan.PromoteEpic
	newRunExecutor = func(cfg *config.OrchestratorConfig, paths orchestrator.Paths) (runExecutor, error) {
		return orchestrator.New(cfg, paths)
	}

	return func() {
		runNow = oldNow
		runPromoteEpic = oldPromoteEpic
		newRunExecutor = oldNewRunExecutor
	}
}
