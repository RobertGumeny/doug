package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/testutil"
)

func TestRunHandoff_GeneratesPackages(t *testing.T) {
	dir := t.TempDir()
	originalPlan := "" +
		"# Plan\n\n" +
		"## Handoff Data\n\n" +
		"```yaml\n" +
		"schema_version: 1\n" +
		"project:\n" +
		"  name: \"Acme Planner\"\n" +
		"  mode: \"brownfield\"\n" +
		"epics:\n" +
		"  - id: \"EPIC-17\"\n" +
		"    name: \"Planning Lifecycle\"\n" +
		"    prd: |\n" +
		"      # PRD\n\n" +
		"      Deterministically generate backlog packages.\n" +
		"    tasks:\n" +
		"      - id: \"EPIC-17-003\"\n" +
		"        description: \"Implement deterministic handoff output.\"\n" +
		"        acceptance_criteria:\n" +
		"          - \"Generated tasks.yaml always quotes descriptions.\"\n" +
		"```\n"
	testutil.WriteFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"), originalPlan)

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
	if got := mustReadHandoffFile(t, filepath.Join(dir, ".doug", "plan", "history", "PLAN-20260401T190000.000000000Z.md")); got != originalPlan {
		t.Fatalf("archived plan mismatch:\ngot:\n%s\nwant:\n%s", got, originalPlan)
	}
	reseeded := mustReadHandoffFile(t, filepath.Join(dir, ".doug", "plan", "PLAN.md"))
	if !strings.Contains(reseeded, "**Last handoff:**") {
		t.Fatalf("expected reseeded PLAN.md handoff context, got:\n%s", reseeded)
	}
	if strings.Contains(reseeded, "Planning Lifecycle") {
		t.Fatalf("expected handed-off epic content to be removed from active PLAN.md, got:\n%s", reseeded)
	}
}

func mustReadHandoffFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
