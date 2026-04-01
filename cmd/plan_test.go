package cmd

import (
	"bytes"
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

func TestRootHelp_IncludesPlanCommand(t *testing.T) {
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--help"})
	defer rootCmd.SetArgs(nil)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd.Execute(): %v", err)
	}

	if !strings.Contains(buf.String(), "plan") {
		t.Fatalf("expected help output to include plan command; got:\n%s", buf.String())
	}
}

func TestPlanProject_CreatesPlanAndInvokesAgent(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "agent_command: mock-agent {{skill_name}} {{task_id}}\nbuild_system: go\n")

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
		return time.Second, nil
	}

	buf := &bytes.Buffer{}
	if err := planProjectContext(context.Background(), dir, buf); err != nil {
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
		"# Project Plan",
		"## Handoff Data",
		"epics: []",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q in PLAN.md, got:\n%s", want, content)
		}
	}

	assertActiveTaskContains(t, dir, []string{
		"**Task ID**: PLAN",
		"**Task Type**: plan",
		"## Planning Artifact Contract",
		"single primary planning artifact",
		"`doug handoff` owns deterministic derivatives",
		"## Plan File Path",
	})

	if !strings.Contains(buf.String(), "Created .doug/plan/PLAN.md") {
		t.Fatalf("expected create message, got:\n%s", buf.String())
	}
}

func TestPlanProject_PreservesExistingPlanDocument(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "doug.yaml"), "agent_command: mock-agent {{skill_name}} {{task_id}}\n")
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), "# Existing Plan\n\nKeep me.\n")

	restore := stubPlanDeps()
	defer restore()

	planRunAgent = func(ctx context.Context, agentCommand, projectRoot string, heartbeatInterval time.Duration, heartbeatFn func(time.Duration), output io.Writer) (time.Duration, error) {
		return time.Second, nil
	}

	buf := &bytes.Buffer{}
	if err := planProjectContext(context.Background(), dir, buf); err != nil {
		t.Fatalf("planProjectContext: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".doug", "plan", "PLAN.md"))
	if err != nil {
		t.Fatalf("read PLAN.md: %v", err)
	}
	if string(data) != "# Existing Plan\n\nKeep me.\n" {
		t.Fatalf("PLAN.md was overwritten:\n%s", data)
	}
	if !strings.Contains(buf.String(), "Using existing .doug/plan/PLAN.md") {
		t.Fatalf("expected existing message, got:\n%s", buf.String())
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
