package plan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/plan"
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

func TestFinalizeEpicCompletion_UpdatesBacklogMetadataAndArchivesRuntime(t *testing.T) {
	root := t.TempDir()
	paths := plan.NewEpicPackagePaths(root, "EPIC-17")

	testutil.WriteFile(t, filepath.Join(root, ".doug", "PRD.md"), "# Runtime PRD\nupdated working copy\n")
	testutil.WriteFile(t, filepath.Join(root, ".doug", "tasks.yaml"), "epic:\n  id: EPIC-17\n  name: Runtime\n  tasks: []\n")
	testutil.WriteFile(t, filepath.Join(root, ".doug", "project-state.yaml"), "current_epic:\n  id: EPIC-17\n")
	testutil.WriteFile(t, filepath.Join(root, ".doug", "ACTIVE_TASK.md"), "# Active Task\n")
	testutil.WriteFile(t, paths.PRDPath, "# Planned PRD\noriginal payload\n")
	testutil.WriteFile(t, paths.TasksPath, "epic:\n  id: EPIC-17\n  name: Planned\n  tasks: []\n")
	if err := os.MkdirAll(paths.EpicDir, 0o755); err != nil {
		t.Fatalf("MkdirAll epic dir: %v", err)
	}
	if err := plan.SaveEpicMetadata(paths.MetadataPath, &types.EpicMetadata{
		EpicID:         "EPIC-17",
		Status:         types.EpicStatusActive,
		CreatedAt:      "2026-04-01T00:00:00Z",
		SourcePlanPath: ".doug/plan/PLAN.md",
	}); err != nil {
		t.Fatalf("SaveEpicMetadata: %v", err)
	}

	archiveDir, err := plan.FinalizeEpicCompletion(root, types.EpicState{ID: "EPIC-17"}, "2026-04-01T01:23:45Z")
	if err != nil {
		t.Fatalf("FinalizeEpicCompletion: %v", err)
	}

	metadata, err := plan.LoadEpicMetadata(paths.MetadataPath)
	if err != nil {
		t.Fatalf("LoadEpicMetadata: %v", err)
	}
	if metadata.Status != types.EpicStatusCompleted {
		t.Fatalf("metadata status = %q, want %q", metadata.Status, types.EpicStatusCompleted)
	}
	if metadata.CompletedAt == nil || *metadata.CompletedAt != "2026-04-01T01:23:45Z" {
		t.Fatalf("metadata completed_at = %v, want %q", metadata.CompletedAt, "2026-04-01T01:23:45Z")
	}

	archivedPRD, err := os.ReadFile(filepath.Join(archiveDir, "PRD.md"))
	if err != nil {
		t.Fatalf("read archived PRD: %v", err)
	}
	if got, want := string(archivedPRD), "# Runtime PRD\nupdated working copy\n"; got != want {
		t.Fatalf("archived PRD mismatch:\ngot:  %q\nwant: %q", got, want)
	}

	plannedPRD, err := os.ReadFile(paths.PRDPath)
	if err != nil {
		t.Fatalf("read planned PRD: %v", err)
	}
	if got, want := string(plannedPRD), "# Planned PRD\noriginal payload\n"; got != want {
		t.Fatalf("planned PRD was mutated:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFinalizeEpicCompletion_RejectsReverseSyncWhenBacklogStatusIsNotActive(t *testing.T) {
	root := t.TempDir()
	paths := plan.NewEpicPackagePaths(root, "EPIC-17")

	testutil.WriteFile(t, filepath.Join(root, ".doug", "PRD.md"), "# Runtime PRD\n")
	testutil.WriteFile(t, filepath.Join(root, ".doug", "tasks.yaml"), "epic:\n  id: EPIC-17\n  name: Runtime\n  tasks: []\n")
	testutil.WriteFile(t, filepath.Join(root, ".doug", "project-state.yaml"), "current_epic:\n  id: EPIC-17\n")
	if err := os.MkdirAll(paths.EpicDir, 0o755); err != nil {
		t.Fatalf("MkdirAll epic dir: %v", err)
	}
	if err := plan.SaveEpicMetadata(paths.MetadataPath, &types.EpicMetadata{
		EpicID:         "EPIC-17",
		Status:         types.EpicStatusPlanned,
		CreatedAt:      "2026-04-01T00:00:00Z",
		SourcePlanPath: ".doug/plan/PLAN.md",
	}); err != nil {
		t.Fatalf("SaveEpicMetadata: %v", err)
	}

	archiveDir, err := plan.FinalizeEpicCompletion(root, types.EpicState{ID: "EPIC-17"}, "2026-04-01T01:23:45Z")
	if err == nil {
		t.Fatal("expected reverse-sync guard error, got nil")
	}
	if !strings.Contains(err.Error(), `metadata status is "PLANNED"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(archiveDir, "PRD.md")); statErr != nil {
		t.Fatalf("runtime snapshot archive missing after guarded failure: %v", statErr)
	}

	metadata, err := plan.LoadEpicMetadata(paths.MetadataPath)
	if err != nil {
		t.Fatalf("LoadEpicMetadata: %v", err)
	}
	if metadata.Status != types.EpicStatusPlanned {
		t.Fatalf("metadata status = %q, want %q", metadata.Status, types.EpicStatusPlanned)
	}
}

func TestFinalizeEpicCompletion_NoBacklogMetadataStillArchivesRuntime(t *testing.T) {
	root := t.TempDir()

	testutil.WriteFile(t, filepath.Join(root, ".doug", "PRD.md"), "# Runtime PRD\n")
	testutil.WriteFile(t, filepath.Join(root, ".doug", "tasks.yaml"), "epic:\n  id: EPIC-42\n  name: Runtime\n  tasks: []\n")
	testutil.WriteFile(t, filepath.Join(root, ".doug", "project-state.yaml"), "current_epic:\n  id: EPIC-42\n")

	archiveDir, err := plan.FinalizeEpicCompletion(root, types.EpicState{ID: "EPIC-42"}, "2026-04-01T01:23:45Z")
	if err != nil {
		t.Fatalf("FinalizeEpicCompletion: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(archiveDir, "tasks.yaml")); statErr != nil {
		t.Fatalf("runtime snapshot archive missing: %v", statErr)
	}
}
