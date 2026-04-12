package cmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/testutil"
)

func TestRunHandoff_GeneratesPackages(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), ""+
		"# Plan\n\n"+
		"## Handoff Data\n\n"+
		"```yaml\n"+
		"schema_version: 1\n"+
		"project:\n"+
		"  name: \"Acme Planner\"\n"+
		"  mode: \"brownfield\"\n"+
		"epics:\n"+
		"  - id: \"EPIC-17\"\n"+
		"    name: \"Planning Lifecycle\"\n"+
		"    prd: |\n"+
		"      # PRD\n\n"+
		"      Deterministically generate backlog packages.\n"+
		"    tasks:\n"+
		"      - id: \"EPIC-17-003\"\n"+
		"        description: \"Implement deterministic handoff output.\"\n"+
		"        acceptance_criteria:\n"+
		"          - \"Generated tasks.yaml always quotes descriptions.\"\n"+
		"```\n")

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

	oldNow := handoffNow
	handoffNow = func() time.Time {
		return time.Date(2026, 4, 1, 19, 0, 0, 0, time.UTC)
	}
	defer func() {
		handoffNow = oldNow
	}()

	handoffCmd.SetOut(io.Discard)
	handoffCmd.SetErr(io.Discard)
	handoffCmd.SetArgs(nil)

	if err := runHandoff(handoffCmd, nil); err != nil {
		t.Fatalf("runHandoff: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".doug", "plan", "epics", "EPIC-17", "tasks.yaml")); err != nil {
		t.Fatalf("expected generated tasks.yaml, stat err: %v", err)
	}
}
