package plan_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertgumeny/doug/internal/plan"
	"github.com/robertgumeny/doug/internal/state"
	"github.com/robertgumeny/doug/internal/testutil"
	"github.com/robertgumeny/doug/internal/types"
)

func TestNewEpicPackagePaths(t *testing.T) {
	root := "/tmp/project"

	paths := plan.NewEpicPackagePaths(root, "EPIC-17")

	if got, want := paths.PlanDir, filepath.Join(root, ".doug", "plan"); got != want {
		t.Fatalf("PlanDir: got %q, want %q", got, want)
	}
	if got, want := paths.EpicsDir, filepath.Join(root, ".doug", "plan", "epics"); got != want {
		t.Fatalf("EpicsDir: got %q, want %q", got, want)
	}
	if got, want := paths.EpicDir, filepath.Join(root, ".doug", "plan", "epics", "EPIC-17"); got != want {
		t.Fatalf("EpicDir: got %q, want %q", got, want)
	}
	if got, want := paths.PRDPath, filepath.Join(root, ".doug", "plan", "epics", "EPIC-17", "PRD.md"); got != want {
		t.Fatalf("PRDPath: got %q, want %q", got, want)
	}
	if got, want := paths.TasksPath, filepath.Join(root, ".doug", "plan", "epics", "EPIC-17", "tasks.yaml"); got != want {
		t.Fatalf("TasksPath: got %q, want %q", got, want)
	}
	if got, want := paths.MetadataPath, filepath.Join(root, ".doug", "plan", "epics", "EPIC-17", "metadata.yaml"); got != want {
		t.Fatalf("MetadataPath: got %q, want %q", got, want)
	}
}

func TestLoadEpicMetadata_Valid(t *testing.T) {
	dir := t.TempDir()
	paths := plan.NewEpicPackagePaths(dir, "EPIC-17")
	testutil.WriteFile(t, paths.MetadataPath, `
epic_id: "EPIC-17"
status: "ACTIVE"
created_at: "2026-04-01T17:00:00Z"
source_plan_path: ".doug/plan/PLAN.md"
activated_at: "2026-04-01T18:00:00Z"
completed_at: "2026-04-01T19:00:00Z"
`)

	metadata, err := plan.LoadEpicMetadata(paths.MetadataPath)
	if err != nil {
		t.Fatalf("LoadEpicMetadata: %v", err)
	}

	if metadata.EpicID != "EPIC-17" {
		t.Errorf("EpicID: got %q, want %q", metadata.EpicID, "EPIC-17")
	}
	if metadata.Status != types.EpicStatusActive {
		t.Errorf("Status: got %q, want %q", metadata.Status, types.EpicStatusActive)
	}
	if metadata.SourcePlanPath != ".doug/plan/PLAN.md" {
		t.Errorf("SourcePlanPath: got %q, want %q", metadata.SourcePlanPath, ".doug/plan/PLAN.md")
	}
	if metadata.ActivatedAt == nil || *metadata.ActivatedAt != "2026-04-01T18:00:00Z" {
		t.Errorf("ActivatedAt: got %v, want %q", metadata.ActivatedAt, "2026-04-01T18:00:00Z")
	}
	if metadata.CompletedAt == nil || *metadata.CompletedAt != "2026-04-01T19:00:00Z" {
		t.Errorf("CompletedAt: got %v, want %q", metadata.CompletedAt, "2026-04-01T19:00:00Z")
	}
}

func TestLoadEpicMetadata_InvalidStatus(t *testing.T) {
	dir := t.TempDir()
	paths := plan.NewEpicPackagePaths(dir, "EPIC-17")
	testutil.WriteFile(t, paths.MetadataPath, `
epic_id: "EPIC-17"
status: "CANCELLED"
created_at: "2026-04-01T17:00:00Z"
source_plan_path: ".doug/plan/PLAN.md"
`)

	_, err := plan.LoadEpicMetadata(paths.MetadataPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unsupported status "CANCELLED"`) {
		t.Fatalf("expected invalid status error, got: %v", err)
	}
}

func TestLoadEpicMetadata_MalformedFile(t *testing.T) {
	dir := t.TempDir()
	paths := plan.NewEpicPackagePaths(dir, "EPIC-17")
	testutil.WriteFile(t, paths.MetadataPath, "epic_id: [unclosed\n")

	_, err := plan.LoadEpicMetadata(paths.MetadataPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse epic metadata") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestLoadEpicMetadata_MissingField(t *testing.T) {
	dir := t.TempDir()
	paths := plan.NewEpicPackagePaths(dir, "EPIC-17")
	testutil.WriteFile(t, paths.MetadataPath, `
epic_id: "EPIC-17"
status: "PLANNED"
created_at: "2026-04-01T17:00:00Z"
`)

	_, err := plan.LoadEpicMetadata(paths.MetadataPath)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `missing required field "source_plan_path"`) {
		t.Fatalf("expected missing field error, got: %v", err)
	}
}

func TestLoadEpicMetadata_NotFound(t *testing.T) {
	dir := t.TempDir()
	paths := plan.NewEpicPackagePaths(dir, "EPIC-17")

	_, err := plan.LoadEpicMetadata(paths.MetadataPath)
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSaveEpicMetadata_ValidatesAndWrites(t *testing.T) {
	dir := t.TempDir()
	paths := plan.NewEpicPackagePaths(dir, "EPIC-17")
	if err := os.MkdirAll(paths.EpicDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	metadata := &types.EpicMetadata{
		EpicID:         "EPIC-17",
		Status:         types.EpicStatusPlanned,
		CreatedAt:      "2026-04-01T17:00:00Z",
		SourcePlanPath: ".doug/plan/PLAN.md",
	}

	if err := plan.SaveEpicMetadata(paths.MetadataPath, metadata); err != nil {
		t.Fatalf("SaveEpicMetadata: %v", err)
	}

	if _, err := os.Stat(paths.MetadataPath + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temp file cleanup, got stat err %v", err)
	}

	reloaded, err := plan.LoadEpicMetadata(paths.MetadataPath)
	if err != nil {
		t.Fatalf("LoadEpicMetadata after save: %v", err)
	}
	if reloaded.Status != types.EpicStatusPlanned {
		t.Fatalf("Status: got %q, want %q", reloaded.Status, types.EpicStatusPlanned)
	}
}

func TestSaveEpicMetadata_RejectsInvalidStatus(t *testing.T) {
	dir := t.TempDir()
	paths := plan.NewEpicPackagePaths(dir, "EPIC-17")

	err := plan.SaveEpicMetadata(paths.MetadataPath, &types.EpicMetadata{
		EpicID:         "EPIC-17",
		Status:         types.EpicLifecycleStatus("CANCELLED"),
		CreatedAt:      "2026-04-01T17:00:00Z",
		SourcePlanPath: ".doug/plan/PLAN.md",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unsupported status "CANCELLED"`) {
		t.Fatalf("expected invalid status error, got: %v", err)
	}
}
