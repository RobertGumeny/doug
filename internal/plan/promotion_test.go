package plan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/robertgumeny/doug/internal/plan"
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

func TestPromoteEpic_CopiesPayloadAndMarksMetadataActive(t *testing.T) {
	dir := t.TempDir()
	paths := plan.NewEpicPackagePaths(dir, "EPIC-17")

	testutil.WriteFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), "{}\n")
	testutil.WriteFile(t, paths.PRDPath, "# PRD\n\nPromoted epic.\n")
	testutil.WriteFile(t, paths.TasksPath, "epic:\n  id: \"EPIC-17\"\n  name: \"Checkout\"\n  tasks: []\n")
	testutil.WriteFile(t, paths.MetadataPath, ""+
		"epic_id: \"EPIC-17\"\n"+
		"status: \"PLANNED\"\n"+
		"created_at: \"2026-04-01T19:00:00Z\"\n"+
		"source_plan_path: \".doug/plan/PLAN.md\"\n")

	now := time.Date(2026, 4, 1, 20, 0, 0, 0, time.UTC)
	if err := plan.PromoteEpic(dir, "EPIC-17", now); err != nil {
		t.Fatalf("PromoteEpic: %v", err)
	}

	prdData, err := os.ReadFile(filepath.Join(dir, ".doug", "PRD.md"))
	if err != nil {
		t.Fatalf("read root PRD: %v", err)
	}
	if got, want := string(prdData), "# PRD\n\nPromoted epic.\n"; got != want {
		t.Fatalf("root PRD mismatch:\n%s", got)
	}

	tasksData, err := os.ReadFile(filepath.Join(dir, ".doug", "tasks.yaml"))
	if err != nil {
		t.Fatalf("read root tasks: %v", err)
	}
	if got, want := string(tasksData), "epic:\n  id: \"EPIC-17\"\n  name: \"Checkout\"\n  tasks: []\n"; got != want {
		t.Fatalf("root tasks mismatch:\n%s", got)
	}

	metadata, err := plan.LoadEpicMetadata(paths.MetadataPath)
	if err != nil {
		t.Fatalf("LoadEpicMetadata: %v", err)
	}
	if got, want := metadata.Status, types.EpicStatusActive; got != want {
		t.Fatalf("metadata status: got %q, want %q", got, want)
	}
	if metadata.ActivatedAt == nil || *metadata.ActivatedAt != "2026-04-01T20:00:00Z" {
		t.Fatalf("activated_at: got %v, want %q", metadata.ActivatedAt, "2026-04-01T20:00:00Z")
	}
}

func TestPromoteEpic_AllowsCompletedRuntimeForRollover(t *testing.T) {
	dir := t.TempDir()
	paths := plan.NewEpicPackagePaths(dir, "EPIC-17")

	testutil.WriteFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), ""+
		"current_epic:\n"+
		"  id: \"EPIC-16\"\n"+
		"  completed_at: \"2026-04-01T19:30:00Z\"\n")
	testutil.WriteFile(t, paths.PRDPath, "# PRD\n")
	testutil.WriteFile(t, paths.TasksPath, "epic:\n  id: \"EPIC-17\"\n  name: \"Checkout\"\n  tasks: []\n")
	testutil.WriteFile(t, paths.MetadataPath, ""+
		"epic_id: \"EPIC-17\"\n"+
		"status: \"PLANNED\"\n"+
		"created_at: \"2026-04-01T19:00:00Z\"\n"+
		"source_plan_path: \".doug/plan/PLAN.md\"\n")

	if err := plan.PromoteEpic(dir, "EPIC-17", time.Date(2026, 4, 1, 20, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("PromoteEpic: %v", err)
	}

	metadata, err := plan.LoadEpicMetadata(paths.MetadataPath)
	if err != nil {
		t.Fatalf("LoadEpicMetadata: %v", err)
	}
	if metadata.Status != types.EpicStatusActive {
		t.Fatalf("metadata status: got %q, want %q", metadata.Status, types.EpicStatusActive)
	}
}

func TestPromoteEpic_FailsWhenRuntimeWorkspaceAlreadyActive(t *testing.T) {
	dir := t.TempDir()
	paths := plan.NewEpicPackagePaths(dir, "EPIC-17")

	testutil.WriteFile(t, filepath.Join(dir, ".doug", "project-state.yaml"), ""+
		"current_epic:\n"+
		"  id: \"EPIC-16\"\n"+
		"  started_at: \"2026-04-01T19:30:00Z\"\n")
	testutil.WriteFile(t, paths.PRDPath, "# PRD\n")
	testutil.WriteFile(t, paths.TasksPath, "epic:\n  id: \"EPIC-17\"\n  name: \"Checkout\"\n  tasks: []\n")
	testutil.WriteFile(t, paths.MetadataPath, ""+
		"epic_id: \"EPIC-17\"\n"+
		"status: \"PLANNED\"\n"+
		"created_at: \"2026-04-01T19:00:00Z\"\n"+
		"source_plan_path: \".doug/plan/PLAN.md\"\n")

	err := plan.PromoteEpic(dir, "EPIC-17", time.Date(2026, 4, 1, 20, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `runtime workspace already has active epic "EPIC-16"`) {
		t.Fatalf("unexpected error: %v", err)
	}

	metadata, loadErr := plan.LoadEpicMetadata(paths.MetadataPath)
	if loadErr != nil {
		t.Fatalf("LoadEpicMetadata: %v", loadErr)
	}
	if metadata.Status != types.EpicStatusPlanned {
		t.Fatalf("metadata status changed: got %q, want %q", metadata.Status, types.EpicStatusPlanned)
	}
}
